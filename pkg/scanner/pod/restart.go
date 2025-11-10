package pod

// CheckRestartSeverity checks if restart count exceeds threshold
func CheckRestartSeverity(count int32, threshold int32) string {
	if count > threshold {
		return "high"
	}
	return "low"
}

