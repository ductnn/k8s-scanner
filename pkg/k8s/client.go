package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewK8sClient creates a Kubernetes client with the following priority:
// 1. kubeconfigPath parameter (if provided)
// 2. KUBECONFIG environment variable
// 3. Default ~/.kube/config (or %USERPROFILE%\.kube\config on Windows)
func NewK8sClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	// Detect running inside or outside cluster
	config, err := rest.InClusterConfig()
	if err != nil {
		// Running locally → use kubeconfig
		var kubeconfig string

		// Priority: flag > env var > default
		if kubeconfigPath != "" {
			kubeconfig = kubeconfigPath
		} else if kubeconfig = os.Getenv("KUBECONFIG"); kubeconfig == "" {
			// Default to ~/.kube/config (works on Windows, Linux, macOS)
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}
