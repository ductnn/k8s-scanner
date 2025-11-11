package pod

// SeverityFromReason maps pod reason to severity level
func SeverityFromReason(reason string) string {
	switch reason {
	case "ImagePullBackOff", "ErrImagePull":
		return "critical"
	case "CrashLoopBackOff", "Pending":
		return "high"
	case "Evicted", "OOMKilled":
		return "medium"
	default:
		return "low"
	}
}
