package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func main() {
	metricsURL := "http://localhost:8080/metrics"
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	log.Println("Starting metrics client...")
	log.Printf("Querying %s every 3 seconds\n", metricsURL)
	log.Println("Press Ctrl+C to stop\n")

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Query immediately, then on ticker
	queryMetrics(client, metricsURL)

	for range ticker.C {
		queryMetrics(client, metricsURL)
	}
}

func queryMetrics(client *http.Client, url string) {
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Error querying metrics: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected status code: %d\n", resp.StatusCode)
		return
	}

	// Use the Decoder approach which is the modern best practice
	decoder := expfmt.NewDecoder(resp.Body, expfmt.NewFormat(expfmt.TypeTextPlain))

	var lastUpdateTimestamp int64
	var requestCount float64

	// Decode metrics one by one
	for {
		mf := &dto.MetricFamily{}
		err := decoder.Decode(mf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error decoding metrics: %v\n", err)
			return
		}

		metricName := mf.GetName()

		if metricName == "last_metric_update_timestamp_seconds" {
			for _, m := range mf.Metric {
				if m.Gauge != nil {
					lastUpdateTimestamp = int64(m.Gauge.GetValue())
				}
			}
		}

		if metricName == "promhttp_metric_handler_requests_total" {
			for _, m := range mf.Metric {
				if m.Counter != nil {
					// Sum up all the counter values with different labels
					requestCount += m.Counter.GetValue()
				}
			}
		}
	}

	fmt.Printf("=== Metrics Query [%s] ===\n", time.Now().Format("15:04:05"))
	if lastUpdateTimestamp > 0 {
		fmt.Printf("Last Update Timestamp: %d (%s)\n",
			lastUpdateTimestamp,
			time.Unix(lastUpdateTimestamp, 0).Format("15:04:05"))
	}
	if requestCount > 0 {
		fmt.Printf("Total HTTP Requests: %.0f\n", requestCount)
	}
	fmt.Println()
}
