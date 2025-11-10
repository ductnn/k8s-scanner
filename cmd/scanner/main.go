package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/k8s"
	"github.com/ductnn/k8s-scanner/pkg/report"
	"github.com/ductnn/k8s-scanner/pkg/scanner"
	"github.com/ductnn/k8s-scanner/pkg/scanner/pod"
	"github.com/ductnn/k8s-scanner/pkg/types"
)

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `k8s-scanner - Kubernetes cluster issues scanner

USAGE:
  k8s-scanner [OPTIONS]

OPTIONS:
`)
	flag.PrintDefaults()
	fmt.Fprintf(flag.CommandLine.Output(), `
EXAMPLES:
  # Scan all namespaces and display results
  k8s-scanner

  # Scan specific namespace
  k8s-scanner --namespace default

  # Export reports in multiple formats
  k8s-scanner --export json,html,csv

  # Show history of all reports
  k8s-scanner --history

  # Compare two reports
  k8s-scanner --diff "daily-20251109-210646,daily-20251109-210704"

  # Use custom kubeconfig
  k8s-scanner --kubeconfig /path/to/config

  # Set custom restart threshold
  k8s-scanner --restart-threshold 10

  # Output in JSON format
  k8s-scanner --format json

`)
}

func main() {
	// Customize help output
	flag.Usage = printUsage
	var (
		namespace        string
		format           string // json|table  (console output)
		exportOpt        string // csv,md,html,json  (comma-separated)
		outdir           string // output directory for exported files
		restartThreshold int    // threshold for restart count to be considered high severity
		kubeconfig       string // path to kubeconfig file
		history          bool   // show history of reports
		diff             string // compare two reports (format: "old,new" or directory names)
	)
	flag.StringVar(&namespace, "namespace", "", "Namespace to scan (empty = all)")
	flag.StringVar(&format, "format", "table", "Console output format: json|table")
	flag.StringVar(&exportOpt, "export", "", "Export report file(s): csv,md,html,json (comma-separated)")
	flag.StringVar(&outdir, "outdir", ".reports", "Directory to write exported reports")
	flag.IntVar(&restartThreshold, "restart-threshold", 5, "Restart count threshold for high severity (default: 5)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	flag.BoolVar(&history, "history", false, "Show history of all reports")
	flag.StringVar(&diff, "diff", "", "Compare two reports (format: 'old,new' directory names or 'old,new' paths)")

	// Check for help flags in arguments before parsing
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "-help" {
			flag.Usage()
			return
		}
	}

	flag.Parse()

	// Handle history flag
	if history {
		reports, err := report.ListHistory(outdir)
		if err != nil {
			log.Fatalf("failed to list history: %v", err)
		}
		report.PrintHistory(reports)
		return
	}

	// Handle diff flag
	if diff != "" {
		handleDiff(diff, outdir)
		return
	}

	clientset, err := k8s.NewK8sClient(kubeconfig)
	if err != nil {
		log.Fatalf("cannot init k8s client: %v", err)
	}

	// Scan
	var issues []types.Issue

	pods, _ := pod.ScanPods(clientset, namespace, int32(restartThreshold))
	// deploys, _ := scanner.ScanDeploymentsNS(clientset, namespace)
	// jobs, _ := scanner.ScanJobsNS(clientset, namespace)
	// crons, _ := scanner.ScanCronJobsNS(clientset, namespace)

	issues = append(issues, pods...)
	// issues = append(issues, deploys...)
	// issues = append(issues, jobs...)
	// issues = append(issues, crons...)

	// Summary
	sum := scanner.SummarizeByNamespace(issues)

	// Console output
	switch strings.ToLower(format) {
	case "json":
		obj := map[string]any{"issues": issues, "summary": sum}
		b, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(b))
	default:
		fmt.Println("\n=== Issues (table) ===")
		printIssuesTable(issues)
		fmt.Println("\n=== Summary by Namespace ===")
		printSummaryTable(sum)
	}

	// Export files
	if exportOpt != "" {
		kinds := parseExports(exportOpt)
		base := "k8s-report"

		// Add timestamped subdirectory: daily-YYYYMMDD-HHMMSS
		now := time.Now()
		timestampDir := fmt.Sprintf("daily-%s-%s",
			now.Format("20060102"), // YYYYMMDD
			now.Format("150405"))   // HHMMSS
		finalOutdir := filepath.Join(outdir, timestampDir)

		if err := report.WriteAll(finalOutdir, base, issues, sum, kinds); err != nil {
			log.Fatalf("export failed: %v", err)
		}
		fmt.Printf("\nExported to %s: %s.%s\n", finalOutdir, base, strings.Join(stringify(kinds), ","))
	}
}

func parseExports(s string) []report.ExportKind {
	var out []report.ExportKind
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		switch strings.ToLower(p) {
		case "json":
			out = append(out, report.ExportJSON)
		case "csv":
			out = append(out, report.ExportCSV)
		case "md", "markdown":
			out = append(out, report.ExportMD)
		case "html":
			out = append(out, report.ExportHTML)
		}
	}
	return out
}

func stringify(k []report.ExportKind) []string {
	ss := make([]string, len(k))
	for i, v := range k {
		ss[i] = string(v)
	}
	return ss
}

func printIssuesTable(issues []types.Issue) {
	fmt.Println("TIME                | NAMESPACE | KIND | NAME | SEV | STATUS | REASON | NODE | RESTARTS")
	fmt.Println(strings.Repeat("-", 120))
	for _, is := range issues {
		fmt.Printf("%-19s | %-9s | %-4s | %-20s | %-4s | %-12s | %-18s | %-10s | %-3d\n",
			trunc(is.Timestamp, 19), trunc(is.Namespace, 9), trunc(is.Kind, 4), trunc(is.Name, 20),
			strings.ToUpper(trunc(is.Severity, 4)), trunc(is.PodStatus, 12), trunc(is.Reason, 18),
			trunc(is.NodeName, 10), is.RestartCount)
	}
}

func printSummaryTable(sum map[string]types.SeveritySummary) {
	fmt.Println("NAMESPACE | CRITICAL | HIGH | MEDIUM | LOW")
	fmt.Println("-------------------------------------------")
	for ns, s := range sum {
		fmt.Printf("%-9s | %-8d | %-4d | %-6d | %-3d\n", ns, s.Critical, s.High, s.Medium, s.Low)
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func handleDiff(diffArg string, outdir string) {
	parts := strings.Split(diffArg, ",")
	if len(parts) != 2 {
		log.Fatalf("diff requires exactly 2 arguments separated by comma (e.g., 'old,new' or 'daily-20251109-210646,daily-20251109-210704')")
	}

	oldPath := strings.TrimSpace(parts[0])
	newPath := strings.TrimSpace(parts[1])

	// If paths don't contain slashes, assume they're directory names
	if !strings.Contains(oldPath, string(filepath.Separator)) && !strings.Contains(oldPath, "/") {
		oldPath = filepath.Join(outdir, oldPath, "k8s-report.json")
	} else if !filepath.IsAbs(oldPath) {
		oldPath = filepath.Join(outdir, oldPath)
	}

	if !strings.Contains(newPath, string(filepath.Separator)) && !strings.Contains(newPath, "/") {
		newPath = filepath.Join(outdir, newPath, "k8s-report.json")
	} else if !filepath.IsAbs(newPath) {
		newPath = filepath.Join(outdir, newPath)
	}

	// Load reports
	oldReport, err := report.LoadReport(oldPath)
	if err != nil {
		log.Fatalf("failed to load old report from %s: %v", oldPath, err)
	}

	newReport, err := report.LoadReport(newPath)
	if err != nil {
		log.Fatalf("failed to load new report from %s: %v", newPath, err)
	}

	// Compare and display
	result := report.DiffReports(oldReport, newReport)
	report.PrintDiff(result, oldReport, newReport)
}
