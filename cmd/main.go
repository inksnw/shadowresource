package main

import (
	"github.com/inksnw/shadowresource/cmd/options"
	v1 "github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1"
	"github.com/inksnw/shadowresource/pkg/informer"
	"github.com/inksnw/shadowresource/pkg/store"
	"github.com/inksnw/shadowresource/pkg/utils"
	"github.com/phuslu/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

func main() {
	utils.InitMapper()
	log.Info().Msgf("载入restMapper 完成")

	server := generateServer()
	informer.ReloadInformer()

	err := server.PrepareRun().Run(genericapiserver.SetupSignalHandler())
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}
}

func generateServer() *genericapiserver.GenericAPIServer {
	metav1.AddToGroupVersion(options.Scheme, schema.GroupVersion{Version: "v1"})
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	options.Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	err := v1.AddToScheme(options.Scheme)
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}
	gvi := v1.SchemeGroupVersion
	gvi.Version = runtime.APIVersionInternal
	options.Scheme.AddKnownTypes(gvi, &v1.ShadowResource{}, &v1.ShadowResourceList{})

	agi := genericapiserver.NewDefaultAPIGroupInfo(
		v1.SchemeGroupVersion.Group,
		options.Scheme,
		metav1.ParameterCodec, options.Codecs)

	config := genericapiserver.NewRecommendedConfig(options.Codecs)
	//config.ClientConfig = pkgconfig.K8sRestConfig()
	//config.SharedInformerFactory = informers.NewSharedInformerFactory(pkgconfig.K8sClient, 0)
	err = options.GetRcOpt().ApplyTo(config)
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}

	config.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		v1.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(options.Scheme))

	if utilfeature.DefaultFeatureGate.Enabled(features.OpenAPIV3) {
		config.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(v1.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(options.Scheme))

	}
	completeConfig := config.Complete()
	resources := map[string]rest.Storage{
		"shadowresources": store.NewMyStore(v1.SchemeGroupResource, true,
			rest.NewDefaultTableConvertor(v1.SchemeGroupResource)),
	}
	agi.VersionedResourcesStorageMap[v1.SchemeGroupVersion.Version] = resources
	server, err := completeConfig.New("myapi", genericapiserver.NewEmptyDelegate())
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}
	err = server.InstallAPIGroup(&agi)
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}
	return server
}
