package scanner

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetLatestPodEvent(client *kubernetes.Clientset, namespace string, podName string) string {
	events, err := client.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return ""
	}

	var (
		latestMsg string
		latestTs  time.Time
	)

	for _, ev := range events.Items {
		if ev.InvolvedObject.Kind == "Pod" && ev.InvolvedObject.Name == podName {

			if ev.LastTimestamp.Time.After(latestTs) {
				latestTs = ev.LastTimestamp.Time
				latestMsg = ev.Message
			}
		}
	}

	return latestMsg
}
