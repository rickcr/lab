package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Define custom metrics using Prometheus library
	lastUpdateTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "last_metric_update_timestamp_seconds",
		Help: "Unix timestamp of when metrics were last updated",
	})

	requestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests received",
		},
		[]string{"endpoint"},
	)

	appInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_info",
			Help: "Application information",
		},
		[]string{"version", "app"},
	)
)

func main() {
	// Set app info metric
	appInfo.WithLabelValues("1.0", "metrics_demo").Set(1)

	// Start the metrics update goroutine
	go updateMetrics()

	// Register HTTP handlers
	http.HandleFunc("/", showAppHandler)
	// Use the Prometheus HTTP handler for metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	log.Println("Starting metrics server on http://localhost:8080")
	log.Println("Access /metrics for Prometheus metrics")
	log.Println("Access / for just to hit server")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func showAppHandler(w http.ResponseWriter, r *http.Request) {
	requestCounter.WithLabelValues("root").Inc()
	fmt.Fprintf(w, "Mock Metrics Info\n")
	fmt.Fprintf(w, "This is a basic Go metrics demo server using Prometheus.\n")
	fmt.Fprintf(w, "Check /metrics for Prometheus metrics.\n")
}

// updateMetrics periodically updates the last update timestamp
func updateMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lastUpdateTimestamp.Set(float64(time.Now().Unix()))
		log.Println("Metrics updated at", time.Now().Format("15:04:05"))
	}
}
