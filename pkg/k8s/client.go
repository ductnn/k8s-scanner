package k8s

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewK8sClient() (*kubernetes.Clientset, error) {
	// Detect running inside or outside cluster
	config, err := rest.InClusterConfig()
	if err != nil {
		// Running locally → use kubeconfig
		// Check KUBECONFIG environment variable first (cross-platform)
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			// Default to ~/.kube/config (works on Windows, Linux, macOS)
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		flag.Parse()
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}
