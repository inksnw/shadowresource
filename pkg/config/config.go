package config

import (
	"github.com/phuslu/log"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func init() {
	if !log.IsTerminal(os.Stderr.Fd()) {
		return
	}
	log.DefaultLogger = log.Logger{
		TimeFormat: "15:04:05",
		Caller:     1,
		Writer: &log.ConsoleWriter{
			ColorOutput:    true,
			QuoteString:    true,
			EndWithMessage: true,
		},
	}
}

var DynamicClient dynamic.Interface
var K8sClient *kubernetes.Clientset
var KindStatusKeyMap map[string]string

func init() {
	var err error
	DynamicClient, err = dynamic.NewForConfig(K8sRestConfig())
	if err != nil {
		log.Fatal()
	}
	K8sClient, err = kubernetes.NewForConfig(K8sRestConfig())
	if err != nil {
		log.Fatal()
	}
	KindStatusKeyMap = make(map[string]string)
	KindStatusKeyMap["Pod"] = "status.phase"
}

func GetStatusKey(kind string) string {
	key, ok := KindStatusKeyMap[kind]
	if !ok {
		log.Warn().Msgf("未定义资源 %s 的状态字段来源", kind)
	}
	return key
}

func K8sRestConfig() *rest.Config {
	if exists(clientcmd.RecommendedHomeFile) {
		config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			log.Fatal()
		}
		return config
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal()
	}
	return config
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
