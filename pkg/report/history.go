package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"
)

// ReportData represents the structure of a saved JSON report
type ReportData struct {
	GeneratedAt string                           `json:"generated_at"`
	Issues      []types.Issue                    `json:"issues"`
	Summary     map[string]types.SeveritySummary `json:"summary"`
}

// ReportInfo contains metadata about a historical report
type ReportInfo struct {
	Path        string
	DirName     string
	GeneratedAt time.Time
	IssueCount  int
	Summary     map[string]types.SeveritySummary
}

// ListHistory scans the reports directory and returns all historical reports
func ListHistory(outdir string) ([]ReportInfo, error) {
	entries, err := os.ReadDir(outdir)
	if err != nil {
		return nil, fmt.Errorf("failed to read reports directory: %w", err)
	}

	var reports []ReportInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		// Check if it matches the pattern: daily-YYYYMMDD-HHMMSS
		if !strings.HasPrefix(dirName, "daily-") {
			continue
		}

		reportPath := filepath.Join(outdir, dirName, "k8s-report.json")
		if _, err := os.Stat(reportPath); err != nil {
			// Skip if JSON report doesn't exist
			continue
		}

		// Load report to get metadata
		reportData, err := LoadReport(reportPath)
		if err != nil {
			continue
		}

		generatedAt, _ := time.Parse(time.RFC3339, reportData.GeneratedAt)

		reports = append(reports, ReportInfo{
			Path:        reportPath,
			DirName:     dirName,
			GeneratedAt: generatedAt,
			IssueCount:  len(reportData.Issues),
			Summary:     reportData.Summary,
		})
	}

	// Sort by generated time (newest first)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})

	return reports, nil
}

// LoadReport loads a JSON report from the given path
func LoadReport(path string) (*ReportData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read report file: %w", err)
	}

	var report ReportData
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report JSON: %w", err)
	}

	return &report, nil
}

// PrintHistory displays the list of historical reports in a table format
func PrintHistory(reports []ReportInfo) {
	if len(reports) == 0 {
		fmt.Println("No historical reports found.")
		return
	}

	fmt.Println("\n=== Historical Reports ===")
	fmt.Printf("%-30s | %-20s | %-8s | %-10s\n", "DIRECTORY", "GENERATED AT", "ISSUES", "SUMMARY")
	fmt.Println(strings.Repeat("-", 100))

	for _, r := range reports {
		// Calculate total issues by severity
		totalCritical := 0
		totalHigh := 0
		totalMedium := 0
		totalLow := 0
		for _, s := range r.Summary {
			totalCritical += s.Critical
			totalHigh += s.High
			totalMedium += s.Medium
			totalLow += s.Low
		}
		summaryStr := fmt.Sprintf("C:%d H:%d M:%d L:%d", totalCritical, totalHigh, totalMedium, totalLow)

		fmt.Printf("%-30s | %-20s | %-8d | %-10s\n",
			r.DirName,
			r.GeneratedAt.Format("2006-01-02 15:04:05"),
			r.IssueCount,
			summaryStr)
	}
	fmt.Println()
}
