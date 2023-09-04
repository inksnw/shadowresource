package crd

import (
	"encoding/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	StoreApiVersion = "kubesphere.io/v1"
	StoreKind       = "shim"
	StoreStatusKey  = "spec.status"
)

var StoreGVR = schema.GroupVersionResource{
	Group:    "kubesphere.io",
	Version:  "v1",
	Resource: "shims",
}

type CrdStoreSpec struct {
	CrInfoList []CrInfo `json:"CrInfoList"`
	Status     string   `json:"status"`
	ShadowUid  string   `json:"shadowUid"`
}

type CrInfo struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type CrdStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CrdStoreSpec `json:"spec"`
}

func (u *CrdStore) FromUnstructured(utd *unstructured.Unstructured) (err error) {
	marshalJSON, err := utd.MarshalJSON()
	if err != nil {
		return err
	}
	err = json.Unmarshal(marshalJSON, u)
	return err
}

type Metadata struct {
	Name      string
	Namespace string
}
