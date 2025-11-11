package pod

import (
	"context"
	"sync"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ScanPods scans pods in the specified namespace and returns issues
func ScanPods(client *kubernetes.Clientset, ns string, restartThreshold int32, ignoredNamespaces map[string]bool) ([]types.Issue, error) {
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

	// Filter out pods from ignored namespaces
	if len(ignoredNamespaces) > 0 {
		filteredPods := make([]v1.Pod, 0, len(pods.Items))
		for _, pod := range pods.Items {
			if !ignoredNamespaces[pod.Namespace] {
				filteredPods = append(filteredPods, pod)
			}
		}
		pods.Items = filteredPods
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
	issues := make([]types.Issue, 0, 3)
	podStatus := GetPodStatus(pod)
	timestamp := time.Now().Format(time.RFC3339)
	lastEvent := GetLatestPodEvent(eventMap, pod.Namespace, pod.Name)

	// Check pod-level issues
	if pod.Status.Phase == v1.PodFailed && pod.Status.Reason == "Evicted" {
		issues = append(issues, createIssue(pod, "Evicted", podStatus, timestamp, lastEvent, getMaxRestartCount(pod)))
	}

	// Check container-level issues
	for _, cs := range pod.Status.ContainerStatuses {
		// Check waiting state
		if cs.State.Waiting != nil {
			issues = append(issues, createIssue(pod, cs.State.Waiting.Reason, podStatus, timestamp, lastEvent, cs.RestartCount))
		}

		// Check terminated state
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			issues = append(issues, createIssue(pod, cs.State.Terminated.Reason, podStatus, timestamp, lastEvent, cs.RestartCount))
		}

		// Check high restart count
		if CheckRestartSeverity(cs.RestartCount, restartThreshold) == "high" {
			issues = append(issues, createIssue(pod, "HighRestartCount", podStatus, timestamp, lastEvent, cs.RestartCount))
		}
	}

	return issues
}

// getMaxRestartCount returns the maximum restart count from all containers
func getMaxRestartCount(pod v1.Pod) int32 {
	maxCount := int32(0)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount > maxCount {
			maxCount = cs.RestartCount
		}
	}
	return maxCount
}

// createIssue creates an Issue struct with common fields
func createIssue(pod v1.Pod, reason string, podStatus string, timestamp string, lastEvent string, restartCount int32) types.Issue {
	severity := SeverityFromReason(reason)
	rootCause := DetectPodRootCause(reason)

	// Special handling for HighRestartCount
	if reason == "HighRestartCount" {
		severity = "high"
		rootCause = "Container bị restart quá nhiều lần (unstable)."
	}

	return types.Issue{
		Kind:         "Pod",
		Namespace:    pod.Namespace,
		Name:         pod.Name,
		Severity:     severity,
		Reason:       reason,
		RootCause:    rootCause,
		PodStatus:    podStatus,
		NodeName:     pod.Spec.NodeName,
		Timestamp:    timestamp,
		RestartCount: restartCount,
		LastEvent:    lastEvent,
	}
}
