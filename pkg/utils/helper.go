package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/inksnw/shadowresource/pkg/apis/crd"
	"github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1"
	"github.com/inksnw/shadowresource/pkg/config"
	"github.com/phuslu/log"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/restmapper"

	"os"
)

var (
	Mapper meta.RESTMapper
)

func InitMapper() {
	gr, err := restmapper.GetAPIGroupResources(config.K8sClient.Discovery())
	if err != nil {
		log.Fatal().Msgf("初始化mapper失败,请检查k8s连接 %s", err)
		os.Exit(1)
	}
	Mapper = restmapper.NewDiscoveryRESTMapper(gr)
}
func ForList(ns string) (rv runtime.Object, err error) {

	obj, err := config.DynamicClient.Resource(crd.StoreGVR).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := &v1.ShadowResourceList{}
	result.APIVersion = v1.ShadowAPIVersion
	result.Kind = v1.ShadowKind
	for _, i := range obj.Items {
		js, _ := i.MarshalJSON()
		var item v1.ShadowResource
		item.Name = i.GetName()
		item.Namespace = i.GetNamespace()
		item.CreationTimestamp = i.GetCreationTimestamp()
		item.Status.State = gjson.GetBytes(js, crd.StoreStatusKey).String()
		result.Items = append(result.Items, item)
	}
	return result, err
}

func ForDelete(name, ns string) (runtime.Object, error) {
	ins := &crd.CrdStore{}

	obj, err := config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info().Msgf("未找到 %s", name)
		return &metav1.Status{Reason: "NotFound", Code: 404}, err
	}
	if err = ins.FromUnstructured(obj); err != nil {
		return nil, err
	}

	for idx, i := range ins.Spec.CrInfoList {
		subGvr := schema.GroupVersionResource{
			Group:    i.Group,
			Version:  i.Version,
			Resource: i.Resource,
		}
		msg := fmt.Sprintf("[%d/%d]", idx+1, len(ins.Spec.CrInfoList))
		log.Info().Msgf("%s 删除资源 %s: %s", msg, subGvr.Resource, i.Name)
		err = config.DynamicClient.Resource(subGvr).
			Namespace(i.Namespace).
			Delete(context.TODO(), i.Name, metav1.DeleteOptions{})
		if err != nil {
			return nil, err
		}
	}

	err = config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}

	shadow := v1.ShadowResource{}
	shadow.APIVersion = v1.ShadowAPIVersion
	shadow.Kind = v1.ShadowKind
	shadow.Name = name
	shadow.Namespace = ns

	return &shadow, nil
}

func ForGet(name, ns string) (runtime.Object, error) {
	ins := &crd.CrdStore{}

	obj, err := config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Info().Msgf("未找到 %s/%s", ns, name)
		return &metav1.Status{Reason: "NotFound", Code: 404}, err
	}
	if err = ins.FromUnstructured(obj); err != nil {
		return nil, err
	}

	shadow := v1.ShadowResource{}
	shadow.APIVersion = v1.ShadowAPIVersion
	shadow.Kind = v1.ShadowKind
	shadow.Name = name
	shadow.Namespace = ns
	shadow.CreationTimestamp = obj.GetCreationTimestamp()
	shadow.Status.State = ins.Spec.Status
	shadow.UID = types.UID(ins.Spec.ShadowUid)

	var list []any
	for _, i := range ins.Spec.CrInfoList {
		gvr := schema.GroupVersionResource{
			Group:    i.Group,
			Version:  i.Version,
			Resource: i.Resource,
		}

		opt := metav1.GetOptions{}
		utd, err := config.DynamicClient.Resource(gvr).
			Namespace(i.Namespace).Get(context.TODO(), i.Name, opt)
		if err != nil {
			return nil, err
		}
		utd.SetManagedFields(nil)
		list = append(list, utd)
	}
	shadow.Spec.FlowList = list
	shadow.UID = types.UID(ins.Spec.ShadowUid)

	opt := k8sjson.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: false,
	}
	serializer := k8sjson.NewSerializerWithOptions(k8sjson.DefaultMetaFactory, nil, nil, opt)
	buf := &bytes.Buffer{}
	err = serializer.Encode(&shadow, buf)
	if err != nil {
		return nil, err
	}
	decode, _, err := Decode(buf.Bytes())

	return decode, err
}

func GvkToGvr(gvk *schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) || err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

func setAnnotation(obj runtime.Object, annotation, key string) error {
	ants, err := meta.NewAccessor().Annotations(obj)
	if err != nil {
		return err
	}
	if ants == nil {
		ants = map[string]string{}
	}
	ants[key] = annotation
	return meta.NewAccessor().SetAnnotations(obj, ants)
}

func ForApply(tasks []json.RawMessage, metaAnnotations string) (err error) {

	for idx, js := range tasks {
		msg := fmt.Sprintf("[%d/%d]", idx+1, len(tasks))

		gvr, utd, err := GetInfoFromBytes(js)
		if err != nil {
			return err
		}
		log.Info().Msgf("%s 提交资源 %s: %s", msg, gvr.Resource, utd.GetName())
		if idx == 0 {
			if err = setAnnotation(utd, metaAnnotations, v1.ShadowKind); err != nil {
				return err
			}
		}
		opt := metav1.PatchOptions{FieldManager: v1.FieldManager}

		marshalJSON, err := utd.MarshalJSON()
		if err != nil {
			return err
		}
		_, err = config.DynamicClient.Resource(gvr).
			Namespace(utd.GetNamespace()).
			Patch(context.TODO(), utd.GetName(), types.ApplyPatchType, marshalJSON, opt)

		if err != nil {
			return err
		}
	}
	return nil
}

func GetInfoFromBytes(bytes json.RawMessage) (gvr schema.GroupVersionResource, utd *unstructured.Unstructured, err error) {
	obj, gvk, err := Decode(bytes)
	if err != nil {
		return gvr, utd, err
	}
	gvr, err = GvkToGvr(gvk)
	if err != nil {
		return gvr, utd, err
	}
	utd, err = ConvertToUnstructured(obj)
	return gvr, utd, err
}

func Decode(data []byte) (obj runtime.Object, gvk *schema.GroupVersionKind, err error) {
	decoder := unstructured.UnstructuredJSONScheme
	obj, gvk, err = decoder.Decode(data, nil, nil)
	return obj, gvk, err
}

func ConvertToUnstructured(obj any) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	return &unstructured.Unstructured{Object: objMap}, err
}
