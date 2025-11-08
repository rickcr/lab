package main

/*
this example is for a case where we can't hit the metrics REST endpoint directly,
so we write metrics.prom files to a directory that Alloy can scan and push to Mimir.
NOTE: in this case the cadence of when metrics are pushed, has to be done within this
application, so there would be routine that periodically would push metrics.
*/
import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
)

var (
	// Define custom metrics using Prometheus library
	lastUpdateTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "last_metric_update_timestamp_seconds",
		Help: "Unix timestamp of when metrics were last updated",
	})

	myCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "demo_counter",
			Help: "Helping debug",
		},
	)

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

// Fixed name for the final metrics file that the Prometheus Exporter will read.
const metricFilename = "app_metrics.prom"

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
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		myCounter.Inc()
		lastUpdateTimestamp.Set(float64(time.Now().Unix()))
		log.Println("Metrics updated at", time.Now().Format("15:04:05"))
		//This app we aren't scraping from endpoint but scraping from files written out
		metricsString, err := getMetricsAsString()
		if err != nil {
			log.Println("Error gathering metrics to string:", err)
		} else {
			log.Println("Metric data we will push to prom file:\n", metricsString)
		}
		//THIS would be an env provided dir - for testing just using temp in the project
		WriteMetricToPromFile("temp", metricsString)
	}
}

func getMetricsAsString() (string, error) {
	log.Println("Gathering metrics from default registry...")
	// Gather all registered metrics
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return "", fmt.Errorf("could not gather metrics: %w", err)
	}

	// Create a strings.Builder to write the encoded metrics to
	var sb strings.Builder

	// Create a new text encoder and write the metrics to the string builder
	encoder := expfmt.NewEncoder(&sb, expfmt.FmtText)
	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			return "", fmt.Errorf("could not encode metric family %s: %w", mf.GetName(), err)
		}
	}

	// Return the built string
	return sb.String(), nil
}

// 1. Writes content to a temporary file.
// 2. Renames the temporary file to the final metricFilename.
func WriteMetricToPromFile(directory string, content string) {
	// 1. Ensure the directory exists.
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		log.Fatalf("Fatal error: Could not create directory '%s': %v", directory, err)
	}

	if content == "" {
		log.Println("WARNING: Metric content is empty, skipping file write.")
		return
	}

	// Define final target path and temporary path
	finalPath := filepath.Join(directory, metricFilename)

	// Create a unique temporary file path (e.g., app_metrics.prom.tmp.20060102...)
	// This ensures we aren't writing directly to the file the exporter might be reading.
	tempPath := fmt.Sprintf("%s.tmp.%d", finalPath, time.Now().UnixNano())

	// 2. Write the content to the temporary file.
	err = os.WriteFile(tempPath, []byte(content), 0644)
	if err != nil {
		log.Fatalf("Fatal error: Could not write metrics to temporary file '%s': %v", tempPath, err)
	}

	// 3. Atomically rename the temporary file to the final path, replacing the old file.
	err = os.Rename(tempPath, finalPath)
	if err != nil {
		// Attempt to clean up the orphaned temp file
		os.Remove(tempPath)
		log.Fatalf("Fatal error: Could not rename temporary file to '%s': %v", finalPath, err)
	}

	log.Printf("Successfully updated atomic metrics file: %s", finalPath)
}
