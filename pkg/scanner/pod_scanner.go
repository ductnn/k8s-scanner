package scanner

import (
	"context"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const RestartThreshold int32 = 5 // restart > 5 coi như bất thường

func ScanPods(client *kubernetes.Clientset, ns string) ([]types.Issue, error) {
	opts := metav1.ListOptions{}
	issues := []types.Issue{}

	var pods *v1.PodList
	var err error

	if ns == "" {
		pods, err = client.CoreV1().Pods("").List(context.Background(), opts)
	} else {
		pods, err = client.CoreV1().Pods(ns).List(context.Background(), opts)
	}
	if err != nil {
		return issues, err
	}

	for _, pod := range pods.Items {
		podStatus := GetPodStatus(pod)

		for _, cs := range pod.Status.ContainerStatuses {

			// CASE 1: Container đang waiting → ghi lại reason
			if cs.State.Waiting != nil {

				reason := cs.State.Waiting.Reason
				sev := SeverityFromReason(reason)
				rootCause := DetectPodRootCause(reason)

				issues = append(issues, types.Issue{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Severity:  sev,
					Reason:    reason,
					RootCause: rootCause,
					// Suggestion:   SuggestionFromReason(reason),
					PodStatus:    podStatus,
					NodeName:     pod.Spec.NodeName,
					Timestamp:    time.Now().Format(time.RFC3339),
					RestartCount: cs.RestartCount,
					LastEvent:    GetLatestPodEvent(client, pod.Namespace, pod.Name),
				})
			}

			// CASE 2: Restart count quá cao → tạo issue riêng
			if sev := CheckRestartSeverity(cs.RestartCount); sev == "high" {

				issues = append(issues, types.Issue{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Severity:  sev,
					Reason:    "HighRestartCount",
					RootCause: "Container bị restart quá nhiều lần (unstable).",
					// Suggestion:   "Kiểm tra logs, readiness/liveness probes và resource limits.",
					PodStatus:    podStatus,
					NodeName:     pod.Spec.NodeName,
					Timestamp:    time.Now().Format(time.RFC3339),
					RestartCount: cs.RestartCount,
					LastEvent:    GetLatestPodEvent(client, pod.Namespace, pod.Name),
				})
			}
		}
	}

	return issues, nil
}
