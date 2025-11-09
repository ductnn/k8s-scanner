package scanner

func SeverityFromReason(reason string) string {
	switch reason {
	case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff":
		return "critical"
	case "Evicted", "OOMKilled":
		return "high"
	case "Pending":
		return "medium"
	default:
		return "low"
	}
}
