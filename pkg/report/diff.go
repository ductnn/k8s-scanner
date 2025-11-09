package report

import (
	"fmt"
	"strings"

	"github.com/ductnn/k8s-scanner/pkg/types"
)

// IssueKey creates a unique key for an issue for comparison
func issueKey(issue types.Issue) string {
	return fmt.Sprintf("%s/%s/%s", issue.Namespace, issue.Kind, issue.Name)
}

// DiffResult contains the differences between two reports
type DiffResult struct {
	NewIssues      []types.Issue
	ResolvedIssues []types.Issue
	ChangedIssues  []IssueChange
}

// IssueChange represents a change in an issue between two reports
type IssueChange struct {
	OldIssue types.Issue
	NewIssue types.Issue
	Changes  []string // List of what changed
}

// DiffReports compares two reports and returns the differences
func DiffReports(oldReport, newReport *ReportData) *DiffResult {
	result := &DiffResult{
		NewIssues:      []types.Issue{},
		ResolvedIssues: []types.Issue{},
		ChangedIssues:  []IssueChange{},
	}

	// Build maps for quick lookup
	oldIssuesMap := make(map[string]types.Issue)
	for _, issue := range oldReport.Issues {
		key := issueKey(issue)
		oldIssuesMap[key] = issue
	}

	newIssuesMap := make(map[string]types.Issue)
	for _, issue := range newReport.Issues {
		key := issueKey(issue)
		newIssuesMap[key] = issue
	}

	// Find new issues (in new but not in old)
	for key, newIssue := range newIssuesMap {
		if _, exists := oldIssuesMap[key]; !exists {
			result.NewIssues = append(result.NewIssues, newIssue)
		}
	}

	// Find resolved issues (in old but not in new)
	for key, oldIssue := range oldIssuesMap {
		if _, exists := newIssuesMap[key]; !exists {
			result.ResolvedIssues = append(result.ResolvedIssues, oldIssue)
		}
	}

	// Find changed issues (in both but different)
	for key, newIssue := range newIssuesMap {
		if oldIssue, exists := oldIssuesMap[key]; exists {
			changes := compareIssues(oldIssue, newIssue)
			if len(changes) > 0 {
				result.ChangedIssues = append(result.ChangedIssues, IssueChange{
					OldIssue: oldIssue,
					NewIssue: newIssue,
					Changes:  changes,
				})
			}
		}
	}

	return result
}

// compareIssues compares two issues and returns a list of what changed
func compareIssues(old, new types.Issue) []string {
	var changes []string

	if old.Severity != new.Severity {
		changes = append(changes, fmt.Sprintf("Severity: %s → %s", old.Severity, new.Severity))
	}
	if old.Reason != new.Reason {
		changes = append(changes, fmt.Sprintf("Reason: %s → %s", old.Reason, new.Reason))
	}
	if old.PodStatus != new.PodStatus {
		changes = append(changes, fmt.Sprintf("Status: %s → %s", old.PodStatus, new.PodStatus))
	}
	if old.RestartCount != new.RestartCount {
		changes = append(changes, fmt.Sprintf("RestartCount: %d → %d", old.RestartCount, new.RestartCount))
	}
	if old.RootCause != new.RootCause {
		changes = append(changes, fmt.Sprintf("RootCause: %s → %s", old.RootCause, new.RootCause))
	}
	if old.NodeName != new.NodeName {
		changes = append(changes, fmt.Sprintf("NodeName: %s → %s", old.NodeName, new.NodeName))
	}

	return changes
}

// PrintDiff displays the diff results in a readable format
func PrintDiff(result *DiffResult, oldReport, newReport *ReportData) {
	fmt.Println("\n=== Report Comparison ===")
	fmt.Printf("Old Report: %s (%d issues)\n", oldReport.GeneratedAt, len(oldReport.Issues))
	fmt.Printf("New Report: %s (%d issues)\n", newReport.GeneratedAt, len(newReport.Issues))
	fmt.Println()

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Printf("New Issues:      %d\n", len(result.NewIssues))
	fmt.Printf("Resolved Issues: %d\n", len(result.ResolvedIssues))
	fmt.Printf("Changed Issues:  %d\n", len(result.ChangedIssues))
	fmt.Println()

	// New Issues
	if len(result.NewIssues) > 0 {
		fmt.Println("=== New Issues ===")
		for _, issue := range result.NewIssues {
			fmt.Printf("  [%s] %s/%s/%s - %s: %s\n",
				strings.ToUpper(issue.Severity),
				issue.Namespace,
				issue.Kind,
				issue.Name,
				issue.Reason,
				issue.RootCause)
		}
		fmt.Println()
	}

	// Resolved Issues
	if len(result.ResolvedIssues) > 0 {
		fmt.Println("=== Resolved Issues ===")
		for _, issue := range result.ResolvedIssues {
			fmt.Printf("  [%s] %s/%s/%s - %s\n",
				strings.ToUpper(issue.Severity),
				issue.Namespace,
				issue.Kind,
				issue.Name,
				issue.Reason)
		}
		fmt.Println()
	}

	// Changed Issues
	if len(result.ChangedIssues) > 0 {
		fmt.Println("=== Changed Issues ===")
		for _, change := range result.ChangedIssues {
			fmt.Printf("  %s/%s/%s:\n",
				change.NewIssue.Namespace,
				change.NewIssue.Kind,
				change.NewIssue.Name)
			for _, ch := range change.Changes {
				fmt.Printf("    - %s\n", ch)
			}
		}
		fmt.Println()
	}

	if len(result.NewIssues) == 0 && len(result.ResolvedIssues) == 0 && len(result.ChangedIssues) == 0 {
		fmt.Println("No differences found between reports.")
	}
}

