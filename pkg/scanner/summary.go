package scanner

import "github.com/ductnn/k8s-scanner/pkg/types"

func SummarizeByNamespace(issues []types.Issue) map[string]types.SeveritySummary {
	result := map[string]types.SeveritySummary{}

	for _, iss := range issues {
		ns := iss.Namespace

		if _, exists := result[ns]; !exists {
			result[ns] = types.SeveritySummary{}
		}

		summary := result[ns]

		switch iss.Severity {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		default:
			summary.Low++
		}

		result[ns] = summary
	}

	return result
}
