package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"
)

type ExportKind string

const (
	ExportJSON ExportKind = "json"
	ExportCSV  ExportKind = "csv"
	ExportMD   ExportKind = "md"
	ExportHTML ExportKind = "html"
)

func EnsureDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func WriteAll(outdir string, basename string, issues []types.Issue, summary map[string]types.SeveritySummary, kinds []ExportKind) error {
	if err := EnsureDir(outdir); err != nil {
		return err
	}
	for _, k := range kinds {
		filename := filepath.Join(outdir, fmt.Sprintf("%s.%s", basename, string(k)))
		var b []byte
		var err error

		switch k {
		case ExportJSON:
			obj := map[string]any{
				"generated_at": time.Now().Format(time.RFC3339),
				"issues":       issues,
				"summary":      summary,
			}
			b, err = json.MarshalIndent(obj, "", "  ")
		case ExportCSV:
			b, err = csvReport(issues)
		case ExportMD:
			b = []byte(mdReport(issues, summary))
		case ExportHTML:
			b = []byte(htmlReport(issues, summary))
		default:
			err = fmt.Errorf("unsupported export: %s", k)
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(filename, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func csvReport(issues []types.Issue) ([]byte, error) {
	buf := &bytes.Buffer{}

	// Add UTF-8 BOM for proper encoding in Excel and other tools
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(buf)
	_ = w.Write([]string{
		"timestamp", "namespace", "kind", "name", "severity", "pod_status",
		"reason", "root_cause", "node_name", "restart_count", "last_event",
	})
	for _, is := range issues {
		_ = w.Write([]string{
			is.Timestamp, is.Namespace, is.Kind, is.Name, is.Severity, is.PodStatus,
			is.Reason, is.RootCause, is.NodeName, fmt.Sprint(is.RestartCount), is.LastEvent,
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func mdReport(issues []types.Issue, summary map[string]types.SeveritySummary) string {
	var sb strings.Builder
	sb.WriteString("# Kubernetes Issues Report\n\n")
	sb.WriteString(fmt.Sprintf("_Generated: %s_\n\n", time.Now().Format(time.RFC3339)))

	// Summary
	sb.WriteString("## Summary by Namespace\n\n")
	sb.WriteString("| Namespace | Critical | High | Medium | Low |\n|---|---:|---:|---:|---:|\n")
	ns := make([]string, 0, len(summary))
	for k := range summary {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	for _, n := range ns {
		s := summary[n]
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d |\n", n, s.Critical, s.High, s.Medium, s.Low))
	}
	sb.WriteString("\n")

	// Issues
	sb.WriteString("## Issues\n\n")
	sb.WriteString("| Time | Namespace | Kind | Name | Severity | PodStatus | Reason | RootCause | Node |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, is := range issues {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			is.Timestamp, is.Namespace, is.Kind, is.Name, strings.ToUpper(is.Severity), is.PodStatus,
			escapeMD(is.Reason), escapeMD(is.RootCause), is.NodeName))
	}
	return sb.String()
}

func htmlReport(issues []types.Issue, summary map[string]types.SeveritySummary) string {
	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head><meta charset='utf-8'><title>K8s Report</title>")
	sb.WriteString(`<style>
body{font-family:system-ui,Arial,sans-serif;padding:24px}
h1,h2{margin:0 0 12px}
table{border-collapse:collapse;width:100%;margin:12px 0}
th,td{border:1px solid #ddd;padding:8px;font-size:14px}
th{background:#f5f5f5;text-align:left}
.badge{padding:4px 10px;border-radius:4px;display:inline-block;font-weight:bold;font-size:12px}
.badge.CRITICAL{background:#dc2626;color:#fff}
.badge.HIGH{background:#ea580c;color:#fff}
.badge.MEDIUM{background:#ca8a04;color:#fff}
.badge.LOW{background:#0284c7;color:#fff}
.small{color:#666;font-size:12px}
</style></head><body>`)
	sb.WriteString("<h1>Kubernetes Issues Report</h1>")
	sb.WriteString(fmt.Sprintf("<div class='small'>Generated: %s</div>", html.EscapeString(time.Now().Format(time.RFC3339))))

	// Summary
	sb.WriteString("<h2>Summary by Namespace</h2><table><thead><tr><th>Namespace</th><th>Critical</th><th>High</th><th>Medium</th><th>Low</th></tr></thead><tbody>")
	ns := make([]string, 0, len(summary))
	for k := range summary {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	for _, n := range ns {
		s := summary[n]
		sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>",
			html.EscapeString(n), s.Critical, s.High, s.Medium, s.Low))
	}
	sb.WriteString("</tbody></table>")

	// Issues
	sb.WriteString("<h2>Issues</h2><table><thead><tr>")
	cols := []string{"Time", "Namespace", "Kind", "Name", "Severity", "PodStatus", "Reason", "RootCause", "Node", "RestartCount", "LastEvent"}
	for _, c := range cols {
		sb.WriteString("<th>" + c + "</th>")
	}
	sb.WriteString("</tr></thead><tbody>")
	for _, is := range issues {
		sb.WriteString("<tr>")
		severityBadge := fmt.Sprintf("<span class='badge %s'>%s</span>", strings.ToUpper(is.Severity), strings.ToUpper(is.Severity))
		sb.WriteString("<td>" + html.EscapeString(is.Timestamp) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.Namespace) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.Kind) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.Name) + "</td>")
		sb.WriteString("<td>" + severityBadge + "</td>") // Don't escape HTML badge
		sb.WriteString("<td>" + html.EscapeString(is.PodStatus) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.Reason) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.RootCause) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.NodeName) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(fmt.Sprint(is.RestartCount)) + "</td>")
		sb.WriteString("<td>" + html.EscapeString(is.LastEvent) + "</td>")
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table></body></html>")
	return sb.String()
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
