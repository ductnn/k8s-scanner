package pod

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CleanResult contains information about pods that were cleaned
type CleanResult struct {
	DeletedPods []PodInfo
	DryRun      bool
	Errors      []error
}

// PodInfo contains basic information about a pod
type PodInfo struct {
	Namespace string
	Name      string
	Reason    string
	Severity  string
}

// CleanPods identifies and optionally deletes evicted pods and completed jobs
// If dryRun is true, it only reports what would be deleted without actually deleting
func CleanPods(client *kubernetes.Clientset, namespaces []string, ignoredNamespaces map[string]bool, dryRun bool) (*CleanResult, error) {
	result := &CleanResult{
		DeletedPods: make([]PodInfo, 0),
		DryRun:      dryRun,
		Errors:      make([]error, 0),
	}

	opts := metav1.ListOptions{}

	var allPods []v1.Pod

	// If no namespaces specified, scan all namespaces
	if len(namespaces) == 0 {
		pods, err := client.CoreV1().Pods("").List(context.Background(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %w", err)
		}
		allPods = pods.Items
	} else {
		// Scan each specified namespace
		for _, ns := range namespaces {
			ns = strings.TrimSpace(ns)
			if ns == "" {
				continue
			}
			pods, err := client.CoreV1().Pods(ns).List(context.Background(), opts)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to list pods in namespace %s: %w", ns, err))
				continue
			}
			allPods = append(allPods, pods.Items...)
		}
	}

	// Filter out pods from ignored namespaces
	if len(ignoredNamespaces) > 0 {
		filteredPods := make([]v1.Pod, 0, len(allPods))
		for _, pod := range allPods {
			if !ignoredNamespaces[pod.Namespace] {
				filteredPods = append(filteredPods, pod)
			}
		}
		allPods = filteredPods
	}

	// Identify pods to clean
	podsToClean := identifyPodsToClean(allPods)

	// Delete or report pods
	for _, podInfo := range podsToClean {
		if dryRun {
			result.DeletedPods = append(result.DeletedPods, podInfo)
		} else {
			err := client.CoreV1().Pods(podInfo.Namespace).Delete(context.Background(), podInfo.Name, metav1.DeleteOptions{})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to delete pod %s/%s: %w", podInfo.Namespace, podInfo.Name, err))
				continue
			}
			result.DeletedPods = append(result.DeletedPods, podInfo)
		}
	}

	return result, nil
}

// identifyPodsToClean identifies pods that should be cleaned:
// 1. Evicted pods (Phase == PodFailed && Reason contains "evicted")
// 2. Completed jobs (Phase == PodSucceeded)
func identifyPodsToClean(pods []v1.Pod) []PodInfo {
	podsToClean := make([]PodInfo, 0)

	for _, pod := range pods {
		phase := pod.Status.Phase
		reason := pod.Status.Reason

		// Check for evicted pods
		if phase == v1.PodFailed && strings.Contains(strings.ToLower(reason), "evicted") {
			podsToClean = append(podsToClean, PodInfo{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Reason:    reason,
				Severity:  "medium",
			})
			continue
		}

		// Check for completed jobs
		if phase == v1.PodSucceeded {
			podsToClean = append(podsToClean, PodInfo{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Reason:    "Completed",
				Severity:  "low",
			})
			continue
		}
	}

	return podsToClean
}
