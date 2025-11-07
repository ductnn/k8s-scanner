package scanner

import (
	"context"
	"sync"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ScanPods(client *kubernetes.Clientset, ns string, restartThreshold int32) ([]types.Issue, error) {
	opts := metav1.ListOptions{}

	var pods *v1.PodList
	var err error

	if ns == "" {
		pods, err = client.CoreV1().Pods("").List(context.Background(), opts)
	} else {
		pods, err = client.CoreV1().Pods(ns).List(context.Background(), opts)
	}
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return []types.Issue{}, nil
	}

	// Collect unique namespaces for event fetching
	namespaceSet := make(map[string]bool)
	for _, pod := range pods.Items {
		namespaceSet[pod.Namespace] = true
	}
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	// Build event map once for all pods (major performance improvement)
	eventMap := BuildEventMap(client, namespaces)

	// Pre-allocate issues slice with estimated capacity
	estimatedIssues := len(pods.Items) * 2 // rough estimate: 2 issues per pod
	issues := make([]types.Issue, 0, estimatedIssues)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process pods concurrently
	semaphore := make(chan struct{}, 50) // Limit concurrent goroutines to 50

	for i := range pods.Items {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(pod v1.Pod) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			podIssues := processPod(pod, restartThreshold, eventMap)

			// Thread-safe append
			if len(podIssues) > 0 {
				mu.Lock()
				issues = append(issues, podIssues...)
				mu.Unlock()
			}
		}(pods.Items[i])
	}

	wg.Wait()
	return issues, nil
}

// processPod processes a single pod and returns its issues
func processPod(pod v1.Pod, restartThreshold int32, eventMap EventMap) []types.Issue {
	podStatus := GetPodStatus(pod)
	issues := make([]types.Issue, 0, 2) // Pre-allocate for 2 potential issues
	timestamp := time.Now().Format(time.RFC3339)
	lastEvent := GetLatestPodEvent(eventMap, pod.Namespace, pod.Name)

	for _, cs := range pod.Status.ContainerStatuses {
		// CASE 1: Container đang waiting → ghi lại reason
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			sev := SeverityFromReason(reason)
			rootCause := DetectPodRootCause(reason)

			issues = append(issues, types.Issue{
				Kind:         "Pod",
				Namespace:    pod.Namespace,
				Name:         pod.Name,
				Severity:     sev,
				Reason:       reason,
				RootCause:    rootCause,
				PodStatus:    podStatus,
				NodeName:     pod.Spec.NodeName,
				Timestamp:    timestamp,
				RestartCount: cs.RestartCount,
				LastEvent:    lastEvent,
			})
		}

		// CASE 2: Restart count quá cao → tạo issue riêng
		if sev := CheckRestartSeverity(cs.RestartCount, restartThreshold); sev == "high" {
			issues = append(issues, types.Issue{
				Kind:         "Pod",
				Namespace:    pod.Namespace,
				Name:         pod.Name,
				Severity:     sev,
				Reason:       "HighRestartCount",
				RootCause:    "Container bị restart quá nhiều lần (unstable).",
				PodStatus:    podStatus,
				NodeName:     pod.Spec.NodeName,
				Timestamp:    timestamp,
				RestartCount: cs.RestartCount,
				LastEvent:    lastEvent,
			})
		}
	}

	return issues
}
