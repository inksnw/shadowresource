package informer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/inksnw/shadowresource/pkg/apis/crd"
	shadowresourcev1 "github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1"
	"github.com/inksnw/shadowresource/pkg/config"
	"github.com/inksnw/shadowresource/pkg/utils"
	"github.com/phuslu/log"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

var shardInformer dynamicinformer.DynamicSharedInformerFactory
var informerMap map[schema.GroupVersionResource]bool

func init() {
	shardInformer = dynamicinformer.NewDynamicSharedInformerFactory(config.DynamicClient, 0)
	informerMap = make(map[schema.GroupVersionResource]bool)
}

type Event struct {
}

func (e Event) OnAdd(obj interface{}) {
	//fmt.Println("OnAdd")
}

func getMetaInfoStatus(obj any) (metaInfo crd.Metadata, status string, err error) {
	utd, err := utils.ConvertToUnstructured(obj)
	if err != nil {
		return metaInfo, status, err
	}
	js, _ := utd.MarshalJSON()
	status = gjson.GetBytes(js, config.GetStatusKey(utd.GetKind())).String()
	str := utd.GetAnnotations()[shadowresourcev1.ShadowKind]
	if str != "" {
		json.Unmarshal([]byte(str), &metaInfo)
	}
	return metaInfo, status, err
}

func (e Event) OnUpdate(oldObj, newObj interface{}) {
	oldInfo, oldStatus, err := getMetaInfoStatus(oldObj)
	if err != nil {
		log.Error().Msgf("更新状态失败 %s", err)
		return
	}
	_, newStatus, err := getMetaInfoStatus(newObj)
	if err != nil {
		log.Error().Msgf("更新状态失败 %s", err)
		return
	}
	if oldInfo.Name != "" && oldStatus != newStatus {
		log.Info().Msgf(" %s/%s 状态变更 %s --> %s", oldInfo.Namespace, oldInfo.Name, oldStatus, newStatus)
		err := updateStoreStatus(oldInfo, newStatus)
		if err != nil {
			log.Error().Msgf("更新状态失败 %s", err)
			return
		}
	}
}

func updateStoreStatus(metaInfo crd.Metadata, status string) (err error) {

	opt := metav1.PatchOptions{FieldManager: shadowresourcev1.FieldManager}
	data := []byte(fmt.Sprintf(`{"spec":{"status":"%s"}}`, status))
	_, err = config.DynamicClient.Resource(crd.StoreGVR).
		Namespace(metaInfo.Namespace).
		Patch(context.TODO(), metaInfo.Name, types.MergePatchType, data, opt)
	if err != nil {
		return err
	}
	log.Info().Msgf(" %s/%s 状态变更  %s 成功", metaInfo.Namespace, metaInfo.Name, status)
	return nil
}

func (e Event) OnDelete(obj interface{}) {

	info, _, err := getMetaInfoStatus(obj)
	if err != nil {
		log.Error().Msgf("解析主资源失败 %s", err)
	}
	if info.Name != "" {
		err = updateStoreStatus(info, "deleted")
		if err != nil {
			log.Error().Msgf("更新状态失败 %s", err)
			return
		}
	}
}

func NewEvent() *Event {
	return &Event{}
}
func CreateInformer(gvr schema.GroupVersionResource) {
	if _, ok := informerMap[gvr]; ok {
		return
	}
	event := NewEvent()

	info := shardInformer.ForResource(gvr).Informer()
	info.AddEventHandler(event)
	stopCh := make(chan struct{})
	go info.Run(stopCh)
	informerMap[gvr] = true
	log.Info().Msgf("创建informer: %s 成功", gvr.Resource)
}

func ReloadInformer() {
	list, err := config.DynamicClient.Resource(crd.StoreGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error().Msgf("重启载入informer失败 %s", err)
		return
	}
	for _, i := range list.Items {
		ins := &crd.CrdStore{}
		err = ins.FromUnstructured(&i)
		if err != nil {
			log.Error().Msgf("重启载入informer失败 %s", err)
			return
		}
		first := ins.Spec.CrInfoList[0]
		gvr := schema.GroupVersionResource{
			Group:    first.Group,
			Version:  first.Version,
			Resource: first.Resource,
		}
		CreateInformer(gvr)
	}
	log.Info().Msgf("重启载入informer成功, 载入 %d 条", len(list.Items))
}
