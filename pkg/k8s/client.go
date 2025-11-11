package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetCurrentContext returns the current context name from kubeconfig
func GetCurrentContext(kubeconfigPath string) (string, error) {
	var kubeconfig string

	// Priority: flag > env var > default
	if kubeconfigPath != "" {
		kubeconfig = kubeconfigPath
	} else if kubeconfig = os.Getenv("KUBECONFIG"); kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return "", err
	}

	if config.CurrentContext == "" {
		return "", nil
	}

	// Try to get cluster name from current context
	context, exists := config.Contexts[config.CurrentContext]
	if !exists || context.Cluster == "" {
		return config.CurrentContext, nil
	}

	return context.Cluster, nil
}

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
