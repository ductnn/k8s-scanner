package scanner

func SeverityFromReason(reason string) string {
	switch reason {
	case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff":
		return "critical"
	case "OOMKilled":
		return "high"
	case "Pending":
		return "medium"
	default:
		return "low"
	}
}
