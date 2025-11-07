package scanner

const RestartThreshold int32 = 5 // restart > 5 coi như bất thường

func CheckRestartSeverity(count int32, threshold int32) string {
	if count > threshold {
		return "high"
	}
	return "low"
}
