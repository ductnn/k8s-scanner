package scanner

import (
	v1 "k8s.io/api/core/v1"
)

func GetPodStatus(pod v1.Pod) string {
	if pod.Status.Phase != "" {
		return string(pod.Status.Phase)
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			return cs.State.Terminated.Reason
		}
	}

	return "Unknown"
}
