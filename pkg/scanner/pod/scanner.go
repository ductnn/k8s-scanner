package pod

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ScanPods scans pods in the specified namespaces and returns issues
// If namespaces is empty or nil, scans all namespaces
func ScanPods(client *kubernetes.Clientset, namespaces []string, restartThreshold int32, ignoredNamespaces map[string]bool) ([]types.Issue, error) {
	opts := metav1.ListOptions{}

	var allPods []v1.Pod

	// If no namespaces specified, scan all namespaces
	if len(namespaces) == 0 {
		pods, err := client.CoreV1().Pods("").List(context.Background(), opts)
		if err != nil {
			return nil, err
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
				// Log error but continue with other namespaces
				continue
			}
			allPods = append(allPods, pods.Items...)
		}
	}

	if len(allPods) == 0 {
		return []types.Issue{}, nil
	}

	// Create a PodList-like structure for compatibility with existing code
	pods := &v1.PodList{Items: allPods}

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
	uniqueNamespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		uniqueNamespaces = append(uniqueNamespaces, ns)
	}

	// Build event map once for all pods (major performance improvement)
	eventMap := BuildEventMap(client, uniqueNamespaces)

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

	// Deduplicate issues: keep only the highest priority issue per pod
	deduplicatedIssues := deduplicateIssues(issues)

	return deduplicatedIssues, nil
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

// getSeverityPriority returns a numeric priority for severity (higher = more important)
func getSeverityPriority(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// getReasonPriority returns a numeric priority for reason specificity (higher = more specific)
// This helps prioritize specific errors (like CrashLoopBackOff) over generic ones (like HighRestartCount)
func getReasonPriority(reason string) int {
	// Specific error reasons have higher priority
	specificReasons := map[string]int{
		"ImagePullBackOff": 10,
		"ErrImagePull":     10,
		"CrashLoopBackOff": 9,
		"OOMKilled":        8,
		"Evicted":          7,
		"Pending":          6,
	}
	if priority, ok := specificReasons[reason]; ok {
		return priority
	}
	// HighRestartCount is generic, so lower priority
	if reason == "HighRestartCount" {
		return 1
	}
	// Other reasons default to medium priority
	return 5
}

// deduplicateIssues keeps only the highest priority issue per pod
// Priority is determined by: severity (critical > high > medium > low) > reason specificity
func deduplicateIssues(issues []types.Issue) []types.Issue {
	if len(issues) == 0 {
		return issues
	}

	// Map to store the best issue for each pod (key: namespace/name)
	podIssues := make(map[string]types.Issue)

	for _, issue := range issues {
		key := issue.Namespace + "/" + issue.Name
		existing, exists := podIssues[key]

		if !exists {
			// First issue for this pod
			podIssues[key] = issue
			continue
		}

		// Compare priorities
		existingSeverityPriority := getSeverityPriority(existing.Severity)
		newSeverityPriority := getSeverityPriority(issue.Severity)

		// If new issue has higher severity, replace
		if newSeverityPriority > existingSeverityPriority {
			podIssues[key] = issue
			continue
		}

		// If same severity, compare reason specificity
		if newSeverityPriority == existingSeverityPriority {
			existingReasonPriority := getReasonPriority(existing.Reason)
			newReasonPriority := getReasonPriority(issue.Reason)

			// If new issue has more specific reason, replace
			if newReasonPriority > existingReasonPriority {
				podIssues[key] = issue
			}
		}
	}

	// Convert map back to slice
	result := make([]types.Issue, 0, len(podIssues))
	for _, issue := range podIssues {
		result = append(result, issue)
	}

	return result
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
