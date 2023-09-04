package options

import (
	v1 "github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
)

func GetRcOpt() *genericoptions.RecommendedOptions {
	rc := genericoptions.NewRecommendedOptions("", Codecs.LegacyCodec(v1.SchemeGroupVersion))
	rc.SecureServing.BindPort = 443
	rc.CoreAPI = nil
	rc.Admission = nil
	rc.Authorization = nil
	rc.Authentication = nil

	return rc
}
