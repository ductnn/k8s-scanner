package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/ductnn/k8s-scanner/pkg/k8s"
	"github.com/ductnn/k8s-scanner/pkg/report"
	"github.com/ductnn/k8s-scanner/pkg/scanner"
	"github.com/ductnn/k8s-scanner/pkg/types"
)

func main() {
	var (
		namespace        string
		format           string // json|table  (console output)
		exportOpt        string // csv,md,html,json  (comma-separated)
		outdir           string // output directory for exported files
		restartThreshold int    // threshold for restart count to be considered high severity
		kubeconfig       string // path to kubeconfig file
	)
	flag.StringVar(&namespace, "namespace", "", "Namespace to scan (empty = all)")
	flag.StringVar(&format, "format", "table", "Console output format: json|table")
	flag.StringVar(&exportOpt, "export", "", "Export report file(s): csv,md,html,json (comma-separated)")
	flag.StringVar(&outdir, "outdir", ".reports", "Directory to write exported reports")
	flag.IntVar(&restartThreshold, "restart-threshold", 5, "Restart count threshold for high severity (default: 5)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	flag.Parse()

	clientset, err := k8s.NewK8sClient(kubeconfig)
	if err != nil {
		log.Fatalf("cannot init k8s client: %v", err)
	}

	// Scan
	var issues []types.Issue

	pods, _ := scanner.ScanPods(clientset, namespace, int32(restartThreshold))
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
		if err := report.WriteAll(outdir, base, issues, sum, kinds); err != nil {
			log.Fatalf("export failed: %v", err)
		}
		fmt.Printf("\nExported to %s: %s.%s\n", outdir, base, strings.Join(stringify(kinds), ","))
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
