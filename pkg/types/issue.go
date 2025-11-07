package types

type Issue struct {
	Kind         string `json:"kind"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Severity     string `json:"severity"`
	Reason       string `json:"reason"`
	RootCause    string `json:"root_cause"`
	PodStatus    string `json:"pod_status"`
	Timestamp    string `json:"timestamp"`
	NodeName     string `json:"node_name"`
	RestartCount int32  `json:"restart_count"`
	LastEvent    string `json:"last_event"`
	// Suggestion is not used for now
}
