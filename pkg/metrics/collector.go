package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ductnn/k8s-scanner/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	IssuesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "k8s_issues_total",
			Help: "Number of Kubernetes issues by namespace and severity.",
		},
		[]string{"namespace", "severity"},
	)

	NamespaceCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_scanner_namespace_count",
			Help: "Number of namespaces that have issues.",
		},
	)

	LastRunTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_scanner_last_run_timestamp",
			Help: "Unix timestamp of last scanner run.",
		},
	)
)

func Init() {
	prometheus.MustRegister(IssuesTotal)
	prometheus.MustRegister(NamespaceCount)
	prometheus.MustRegister(LastRunTimestamp)
}

func ExportSummary(sum map[string]types.SeveritySummary) {
	// Clear old metrics
	IssuesTotal.Reset()

	// Export new
	for ns, s := range sum {
		IssuesTotal.WithLabelValues(ns, "critical").Set(float64(s.Critical))
		IssuesTotal.WithLabelValues(ns, "high").Set(float64(s.High))
		IssuesTotal.WithLabelValues(ns, "medium").Set(float64(s.Medium))
		IssuesTotal.WithLabelValues(ns, "low").Set(float64(s.Low))
	}

	NamespaceCount.Set(float64(len(sum)))
	LastRunTimestamp.Set(float64(time.Now().Unix()))
}

// StartServer starts the Prometheus metrics HTTP server
func StartServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Prometheus metrics server running at http://localhost%s/metrics\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Metrics server error: %v\n", err)
	}
}
