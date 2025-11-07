package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventMap stores the latest event message for each pod
// Key format: "namespace/podname"
type EventMap map[string]string

// BuildEventMap fetches all events for given namespaces and builds a lookup map
// This is much more efficient than fetching events per pod
func BuildEventMap(client *kubernetes.Clientset, namespaces []string) EventMap {
	eventMap := make(EventMap)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process each namespace concurrently
	for _, ns := range namespaces {
		wg.Add(1)
		go func(namespace string) {
			defer wg.Done()
			events, err := client.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return
			}

			// Build a map of pod -> latest event message for this namespace
			nsEventMap := make(map[string]struct {
				msg string
				ts  time.Time
			})

			for _, ev := range events.Items {
				if ev.InvolvedObject.Kind == "Pod" {
					key := fmt.Sprintf("%s/%s", namespace, ev.InvolvedObject.Name)
					existing, exists := nsEventMap[key]
					if !exists || ev.LastTimestamp.Time.After(existing.ts) {
						nsEventMap[key] = struct {
							msg string
							ts  time.Time
						}{msg: ev.Message, ts: ev.LastTimestamp.Time}
					}
				}
			}

			// Merge into main map (thread-safe)
			mu.Lock()
			for k, v := range nsEventMap {
				eventMap[k] = v.msg
			}
			mu.Unlock()
		}(ns)
	}

	wg.Wait()
	return eventMap
}

// GetLatestPodEvent retrieves the latest event message from the pre-built map
func GetLatestPodEvent(eventMap EventMap, namespace string, podName string) string {
	key := fmt.Sprintf("%s/%s", namespace, podName)
	return eventMap[key]
}
