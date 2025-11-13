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
	"github.com/ductnn/k8s-scanner/pkg/metrics"
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

  # Scan specific namespace(s)
  k8s-scanner --namespace default
  k8s-scanner --namespace "ns-1,ns-2,ns-3"

  # Export reports in multiple formats
  k8s-scanner --export json,html,csv

  # Show history of all reports
  k8s-scanner --history

  # Compare two reports (by timestamp or filename)
  k8s-scanner --diff "20251109-210646,20251109-210704"
  k8s-scanner --diff "k8s-report-20251109-210646.json,k8s-report-20251109-210704.json"

  # Use custom kubeconfig
  k8s-scanner --kubeconfig /path/to/config

  # Set custom restart threshold
  k8s-scanner --restart-threshold 10

  # Output in JSON format
  k8s-scanner --format json

  # Enable Prometheus metrics server
  k8s-scanner --metrics --metrics-port 9090

  # Ignore specific namespaces
  k8s-scanner --ignore-ns "kube-system,kube-public"

  # Use custom cluster name for output files
  k8s-scanner --cluster-name "production" --export json,html

  # Output only the count of issues
  k8s-scanner --count

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
		metricsPort      int    // port for Prometheus metrics server
		enableMetrics    bool   // enable Prometheus metrics server
		ignoreNS         string // comma-separated list of namespaces to ignore
		clusterName      string // cluster name for output files (auto-detected if not provided)
		count            bool   // output only the count of issues
	)
	flag.StringVar(&namespace, "namespace", "", "Namespace(s) to scan: comma-separated list (e.g., 'ns-1,ns-2') or empty for all")
	flag.StringVar(&format, "format", "table", "Console output format: json|table")
	flag.StringVar(&exportOpt, "export", "", "Export report file(s): csv,md,html,json (comma-separated)")
	flag.StringVar(&outdir, "outdir", ".reports", "Directory to write exported reports")
	flag.IntVar(&restartThreshold, "restart-threshold", 10, "Restart count threshold for high severity (default: 10)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	flag.BoolVar(&history, "history", false, "Show history of all reports")
	flag.StringVar(&diff, "diff", "", "Compare two reports (format: 'old,new' directory names or 'old,new' paths)")
	flag.BoolVar(&enableMetrics, "metrics", false, "Enable Prometheus metrics server")
	flag.IntVar(&metricsPort, "metrics-port", 9090, "Port for Prometheus metrics server (default: 9090)")
	flag.StringVar(&ignoreNS, "ignore-ns", "", "Comma-separated list of namespaces to ignore (e.g., 'kube-system,kube-public')")
	flag.StringVar(&clusterName, "cluster-name", "", "Cluster name for output files (auto-detected from kubeconfig if not provided)")
	flag.BoolVar(&count, "count", false, "Output only the count of issues found")
	// Check for help flags in arguments before parsing
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "-help" {
			flag.Usage()
			return
		}
	}

	flag.Parse()

	// Initialize and start metrics server if enabled
	if enableMetrics {
		metrics.Init()
		go metrics.StartServer(metricsPort)
	}

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

	// Auto-detect cluster name if not provided
	if clusterName == "" {
		detected, err := k8s.GetCurrentContext(kubeconfig)
		if err == nil && detected != "" {
			clusterName = detected
		}
	}

	var issues []types.Issue

	// Parse ignored namespaces
	ignoredNamespaces := parseIgnoredNamespaces(ignoreNS)

	// Parse namespace flag (comma-separated list)
	var namespacesToScan []string
	if namespace != "" {
		for _, ns := range strings.Split(namespace, ",") {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				namespacesToScan = append(namespacesToScan, ns)
			}
		}
	}

	pods, _ := pod.ScanPods(clientset, namespacesToScan, int32(restartThreshold), ignoredNamespaces)
	// deploys, _ := scanner.ScanDeploymentsNS(clientset, namespace)
	// jobs, _ := scanner.ScanJobsNS(clientset, namespace)
	// crons, _ := scanner.ScanCronJobsNS(clientset, namespace)

	issues = append(issues, pods...)
	// issues = append(issues, deploys...)
	// issues = append(issues, jobs...)
	// issues = append(issues, crons...)

	// Summary
	sum := scanner.SummarizeByNamespace(issues)

	// Export metrics if enabled
	if enableMetrics {
		metrics.ExportSummary(sum)
	}

	// If count flag is set, output only the count and exit
	if count {
		fmt.Printf("%d issues\n", len(issues))
		// Still allow export if specified
		if exportOpt != "" {
			kinds := parseExports(exportOpt)

			// Add timestamp to filename: [cluster-name]-k8s-report-YYYYMMDD-HHMMSS
			now := time.Now()
			timestamp := fmt.Sprintf("%s-%s",
				now.Format("20060102"), // YYYYMMDD
				now.Format("150405"))   // HHMMSS

			// Build base filename with optional cluster name prefix
			var base string
			if clusterName != "" {
				// Sanitize cluster name for filename (remove invalid characters)
				sanitized := sanitizeClusterName(clusterName)
				base = fmt.Sprintf("%s-k8s-report-%s", sanitized, timestamp)
			} else {
				base = fmt.Sprintf("k8s-report-%s", timestamp)
			}

			if err := report.WriteAll(outdir, base, issues, sum, kinds); err != nil {
				log.Fatalf("export failed: %v", err)
			}
		}
		// Keep program running if metrics server is enabled
		if enableMetrics {
			fmt.Println("\nMetrics server is running. Press Ctrl+C to stop.")
			select {} // Block forever to keep metrics server running
		}
		// Exit immediately when count flag is used (unless metrics are enabled)
		return
	}

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

		// Add timestamp to filename: [cluster-name]-k8s-report-YYYYMMDD-HHMMSS
		now := time.Now()
		timestamp := fmt.Sprintf("%s-%s",
			now.Format("20060102"), // YYYYMMDD
			now.Format("150405"))   // HHMMSS

		// Build base filename with optional cluster name prefix
		var base string
		if clusterName != "" {
			// Sanitize cluster name for filename (remove invalid characters)
			sanitized := sanitizeClusterName(clusterName)
			base = fmt.Sprintf("%s-k8s-report-%s", sanitized, timestamp)
		} else {
			base = fmt.Sprintf("k8s-report-%s", timestamp)
		}

		if err := report.WriteAll(outdir, base, issues, sum, kinds); err != nil {
			log.Fatalf("export failed: %v", err)
		}
		fmt.Printf("\nExported to %s: %s.%s\n", outdir, base, strings.Join(stringify(kinds), ","))
	}

	// Keep program running if metrics server is enabled
	if enableMetrics {
		fmt.Println("\nMetrics server is running. Press Ctrl+C to stop.")
		select {} // Block forever to keep metrics server running
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

func parseIgnoredNamespaces(ignoreNS string) map[string]bool {
	ignored := make(map[string]bool)
	if ignoreNS == "" {
		return ignored
	}
	for _, ns := range strings.Split(ignoreNS, ",") {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			ignored[ns] = true
		}
	}
	return ignored
}

func sanitizeClusterName(name string) string {
	// Replace invalid filename characters with hyphens
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	sanitized := name
	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "-")
	}
	// Remove consecutive hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

func findReportFile(outdir, timestamp string) string {
	// Look for files matching the timestamp pattern
	// Pattern: [cluster-name]-k8s-report-YYYYMMDD-HHMMSS.json or k8s-report-YYYYMMDD-HHMMSS.json
	entries, err := os.ReadDir(outdir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		if !strings.HasSuffix(fileName, ".json") {
			continue
		}
		// Check if filename ends with the timestamp pattern
		if strings.HasSuffix(fileName, fmt.Sprintf("k8s-report-%s.json", timestamp)) {
			return filepath.Join(outdir, fileName)
		}
	}
	return ""
}

func handleDiff(diffArg string, outdir string) {
	parts := strings.Split(diffArg, ",")
	if len(parts) != 2 {
		log.Fatalf("diff requires exactly 2 arguments separated by comma (e.g., '20251109-210646,20251109-210704' or 'k8s-report-20251109-210646.json,k8s-report-20251109-210704.json')")
	}

	oldPath := strings.TrimSpace(parts[0])
	newPath := strings.TrimSpace(parts[1])

	// If paths don't contain slashes, assume they're timestamp identifiers or filenames
	if !strings.Contains(oldPath, string(filepath.Separator)) && !strings.Contains(oldPath, "/") {
		// Check if it's just a timestamp (e.g., "20251109-143022") or full filename
		if !strings.HasSuffix(oldPath, ".json") {
			// Try to find matching report file (could be with or without cluster name prefix)
			// First try with cluster prefix pattern, then without
			matched := findReportFile(outdir, oldPath)
			if matched != "" {
				oldPath = matched
			} else {
				oldPath = filepath.Join(outdir, fmt.Sprintf("k8s-report-%s.json", oldPath))
			}
		} else {
			oldPath = filepath.Join(outdir, oldPath)
		}
	} else if !filepath.IsAbs(oldPath) {
		oldPath = filepath.Join(outdir, oldPath)
	}

	if !strings.Contains(newPath, string(filepath.Separator)) && !strings.Contains(newPath, "/") {
		// Check if it's just a timestamp (e.g., "20251109-143022") or full filename
		if !strings.HasSuffix(newPath, ".json") {
			// Try to find matching report file (could be with or without cluster name prefix)
			matched := findReportFile(outdir, newPath)
			if matched != "" {
				newPath = matched
			} else {
				newPath = filepath.Join(outdir, fmt.Sprintf("k8s-report-%s.json", newPath))
			}
		} else {
			newPath = filepath.Join(outdir, newPath)
		}
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
