package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/inksnw/shadowresource/pkg/apis/crd"
	"github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1"
	"github.com/inksnw/shadowresource/pkg/config"
	"github.com/inksnw/shadowresource/pkg/informer"
	"github.com/inksnw/shadowresource/pkg/utils"
	"github.com/phuslu/log"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"reflect"
	"sync"
	"time"
)

var _ rest.StandardStorage = &store{}
var _ rest.Scoper = &store{}
var _ rest.Storage = &store{}
var _ rest.Watcher = &store{}
var _ watch.Interface = fakeWatch{}

//var _ rest.SingularNameProvider = &crd{}

func NewMyStore(groupResource schema.GroupResource, isNamespaced bool, tc rest.TableConvertor) rest.Storage {

	return &store{
		defaultQualifiedResource: groupResource,
		TableConvertor:           tc,
		isNamespaced:             isNamespaced,
		newFunc: func() runtime.Object {
			return &v1.ShadowResource{}
		},
		newListFunc: func() runtime.Object {
			return &v1.ShadowResourceList{}
		},
	}

}

type store struct {
	rest.TableConvertor
	isNamespaced             bool
	muWatchers               sync.RWMutex
	newFunc                  func() runtime.Object
	newListFunc              func() runtime.Object
	defaultQualifiedResource schema.GroupResource
}

func (f *store) GetSingularName() string {
	return "ShadowResource"
}

func (f *store) Destroy() {
	log.Info().Msgf("Destroy!!")
}

func (f *store) notifyWatchers(ev watch.Event) {

}

func (f *store) New() runtime.Object {
	return f.newFunc()
}

func (f *store) NewList() runtime.Object {
	return f.newListFunc()
}

func (f *store) NamespaceScoped() bool {
	return f.isNamespaced
}
func (f *store) ShortNames() []string {
	return []string{"mi"}
}

func (f *store) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	requestInfo, _ := request.RequestInfoFrom(ctx)
	rv, err := utils.ForGet(name, requestInfo.Namespace)

	return rv, err

}

func (f *store) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	info, _ := request.RequestInfoFrom(ctx)
	log.Info().Msgf("查询列表 %s", info.Path)
	list, err := utils.ForList(info.Namespace)
	return list, err
}

func (f *store) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions) (runtime.Object, error) {
	ma, _ := obj.(*v1.ShadowResource)
	if len(ma.Spec.FlowList) == 0 {
		return obj, errors.New("you must set spec.flowList")
	}

	var in []json.RawMessage
	for _, i := range ma.Spec.FlowList {
		bytes, err := json.Marshal(i)
		if err != nil {
			return nil, err
		}
		in = append(in, bytes)
	}

	shadowInfo := crd.Metadata{
		Name:      ma.Name,
		Namespace: ma.Namespace,
	}
	marshal, _ := json.Marshal(shadowInfo)

	if err := utils.ForApply(in, string(marshal)); err != nil {
		return nil, err
	}

	if err := saveCrdStore(ma); err != nil {
		return nil, err
	}

	first := ma.Spec.FlowList[0]
	firstJs, err := json.Marshal(first)
	if err != nil {
		return nil, err
	}
	gvr, _, err := utils.GetInfoFromBytes(firstJs)
	if err != nil {
		return nil, err
	}
	informer.CreateInformer(gvr)

	return obj, err
}

func saveCrdStore(sr *v1.ShadowResource) (err error) {
	var exist bool
	oldStore := &crd.CrdStore{}
	utdStore, err := config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(sr.Namespace).Get(context.TODO(), sr.Name, metav1.GetOptions{})
	if err == nil {
		exist = true
		if err = oldStore.FromUnstructured(utdStore); err != nil {
			return err
		}
	}

	newStore := crd.CrdStore{}
	newStore.Kind = crd.StoreKind
	newStore.APIVersion = crd.StoreApiVersion
	newStore.Namespace = sr.Namespace
	newStore.Name = sr.Name
	newUUID, _ := uuid.NewUUID()
	newStore.Spec.ShadowUid = newUUID.String()

	for _, i := range sr.Spec.FlowList {
		b, _ := json.Marshal(i)
		gvr, utd, err := utils.GetInfoFromBytes(b)
		if err != nil {
			return err
		}
		info := crd.CrInfo{
			Group:     gvr.Group,
			Version:   gvr.Version,
			Kind:      utd.GetKind(),
			Resource:  gvr.Resource,
			Namespace: utd.GetNamespace(),
			Name:      utd.GetName(),
		}
		newStore.Spec.CrInfoList = append(newStore.Spec.CrInfoList, info)
	}
	if exist && reflect.DeepEqual(oldStore.Spec.CrInfoList, newStore.Spec.CrInfoList) {
		log.Info().Msgf("crd store已经存在 %s/%s", sr.Namespace, sr.Name)
		return nil
	}

	js, _ := json.Marshal(newStore)
	opt := metav1.PatchOptions{FieldManager: v1.FieldManager}
	ctx := context.TODO()
	_, err = config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(sr.Namespace).Patch(ctx, sr.Name, types.ApplyPatchType, js, opt)

	return err
}

func (f *store) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	info, _ := request.RequestInfoFrom(ctx)
	log.Info().Msgf("收到了更新请求: %s/%s", info.Namespace, info.Name)
	oldObj, _ := f.Get(ctx, name, nil)

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)

	create, err := f.Create(ctx, newObj, nil, &metav1.CreateOptions{})

	return create, false, err
}

func (f *store) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	info, _ := request.RequestInfoFrom(ctx)
	log.Info().Msgf("执行删除: %s", info.Namespace)
	obj, err := utils.ForDelete(name, info.Namespace)

	return obj, false, err
}

func (f *store) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	newListObj := f.NewList()

	return newListObj, nil
}

type fakeWatch struct {
}

func (f fakeWatch) Stop() {
}

func (f fakeWatch) ResultChan() <-chan watch.Event {
	return make(chan watch.Event)
}

func (f *store) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	info, _ := request.RequestInfoFrom(ctx)
	log.Info().Msgf("接到watch请求: %s", info.Path)

	return fakeWatch{}, nil
}

func (f *store) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table
	fn := func(obj runtime.Object) error {
		m, err := meta.Accessor(obj)
		utd, err := utils.ConvertToUnstructured(obj)
		if err != nil {
			return err
		}
		marshalJSON, err := utd.MarshalJSON()
		if err != nil {
			return err
		}
		status := gjson.GetBytes(marshalJSON, "status.State").String()
		if err != nil {
			resource := v1.SchemeGroupResource
			if info, ok := request.RequestInfoFrom(ctx); ok {
				resource = schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
			}
			return errNotAcceptable{resource: resource}
		}
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{m.GetName(), status, m.GetCreationTimestamp().Time.UTC().Format(time.RFC3339)},
			Object: runtime.RawExtension{Object: obj},
		})
		return nil
	}
	switch {
	case meta.IsListType(object):
		if err := meta.EachListItem(object, fn); err != nil {
			return nil, err
		}
	default:
		if err := fn(object); err != nil {
			return nil, err
		}
	}
	if m, err := meta.ListAccessor(object); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
		table.RemainingItemCount = m.GetRemainingItemCount()
	} else {
		if m, err := meta.CommonAccessor(object); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}
	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Status", Type: "string", Format: "string"},
			{Name: "Created At", Type: "date"},
		}
	}
	return &table, nil
}

type errNotAcceptable struct {
	resource schema.GroupResource
}

func (e errNotAcceptable) Error() string {
	return fmt.Sprintf("the resource %s does not support being converted to a Table", e.resource)
}

func (f *store) qualifiedResourceFromContext(ctx context.Context) schema.GroupResource {
	if info, ok := request.RequestInfoFrom(ctx); ok {
		return schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
	}
	// some implementations access storage directly and thus the context has no RequestInfo
	return f.defaultQualifiedResource
}
