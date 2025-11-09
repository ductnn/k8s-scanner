package scanner

const RestartThreshold int32 = 10 // restart > 10 coi như bất thường

func CheckRestartSeverity(count int32, threshold int32) string {
	if count > threshold {
		return "high"
	}
	return "low"
}
