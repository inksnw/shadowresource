package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ShadowApiGroup     = "apis.abc.com"
	ShadowApiVersion   = "v1"
	ShadowResourceName = "shadowresources"
	ShadowAPIVersion   = "apis.abc.com/v1"
	ShadowKind         = "ShadowResource"
	FieldManager       = "shadow"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: ShadowApiGroup, Version: ShadowApiVersion}
var SchemeGroupResource = schema.GroupResource{Group: ShadowApiGroup, Resource: ShadowResourceName}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ShadowResource{},
		&ShadowResourceList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
