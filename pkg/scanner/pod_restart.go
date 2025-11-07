package scanner

func CheckRestartSeverity(count int32) string {
	if count > 0 {
		return "high"
	}
	return "low"
}
