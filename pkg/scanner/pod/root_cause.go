package pod

// DetectPodRootCause returns a human-readable root cause for pod issues
func DetectPodRootCause(reason string) string {
	switch reason {
	case "ImagePullBackOff", "ErrImagePull":
		return "Không pull được image — có thể sai tag, private registry hoặc thiếu quyền."
	case "CrashLoopBackOff":
		return "Container start xong rồi crash liên tục — thường do lỗi app hoặc config sai."
	case "Evicted":
		return "Pod bị evict do node thiếu tài nguyên (disk pressure, memory pressure) — cần kiểm tra node resources."
	case "OOMKilled":
		return "Container bị kill do thiếu bộ nhớ (Out-of-Memory)."
	case "Pending":
		return "Không đủ tài nguyên (CPU/RAM) hoặc không match node selector/taints."
	default:
		return "Chưa xác định — cần kiểm tra logs container."
	}
}
