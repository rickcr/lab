package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/prometheus/prompb"
)

const (
	metricsURL     = "http://localhost:8080/metrics"
	mimirWriteURL  = "http://localhost:9009/api/v1/push"
	scrapeInterval = 5 * time.Second
)

func main() {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	log.Println("Starting Prometheus -> Mimir Bridge")
	log.Printf("Scraping from: %s\n", metricsURL)
	log.Printf("Pushing to: %s\n", mimirWriteURL)
	log.Printf("Interval: %v\n", scrapeInterval)
	log.Println("Press Ctrl+C to stop\n")

	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()

	// Scrape and push immediately, then on ticker
	scrapeAndPush(client)

	for range ticker.C {
		scrapeAndPush(client)
	}
}

func scrapeAndPush(client *http.Client) {
	// Scrape metrics
	metrics, err := scrapeMetrics(client, metricsURL)
	if err != nil {
		log.Printf("Error scraping metrics: %v\n", err)
		return
	}

	log.Printf("Scraped %d metric families\n", len(metrics))

	// Convert to Prometheus remote write format
	timeseries, err := convertToTimeseries(metrics)
	if err != nil {
		log.Printf("Error converting metrics: %v\n", err)
		return
	}

	log.Printf("Converted to %d timeseries\n", len(timeseries))

	// Push to Mimir
	err = pushToMimir(client, mimirWriteURL, timeseries)
	if err != nil {
		log.Printf("Error pushing to Mimir: %v\n", err)
		return
	}

	log.Printf("✓ Successfully pushed metrics to Mimir at %s\n\n", time.Now().Format("15:04:05"))
}

func scrapeMetrics(client *http.Client, url string) (map[string]*io_prometheus_client.MetricFamily, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	decoder := expfmt.NewDecoder(resp.Body, expfmt.NewFormat(expfmt.TypeTextPlain))
	
	metrics := make(map[string]*io_prometheus_client.MetricFamily)
	
	for {
		mf := &io_prometheus_client.MetricFamily{}
		err := decoder.Decode(mf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode: %w", err)
		}
		
		metrics[mf.GetName()] = mf
	}

	return metrics, nil
}

func convertToTimeseries(metricFamilies map[string]*io_prometheus_client.MetricFamily) ([]prompb.TimeSeries, error) {
	timestamp := time.Now().UnixMilli()
	var timeseries []prompb.TimeSeries

	for _, mf := range metricFamilies {
		metricName := mf.GetName()
		metricType := mf.GetType()

		for _, metric := range mf.GetMetric() {
			// Build labels
			labels := []prompb.Label{
				{Name: "__name__", Value: metricName},
			}

			// Add metric labels
			for _, label := range metric.GetLabel() {
				labels = append(labels, prompb.Label{
					Name:  label.GetName(),
					Value: label.GetValue(),
				})
			}

			// Extract value based on metric type
			var value float64
			var found bool

			switch metricType {
			case io_prometheus_client.MetricType_COUNTER:
				if metric.Counter != nil {
					value = metric.Counter.GetValue()
					found = true
				}
			case io_prometheus_client.MetricType_GAUGE:
				if metric.Gauge != nil {
					value = metric.Gauge.GetValue()
					found = true
				}
			case io_prometheus_client.MetricType_UNTYPED:
				if metric.Untyped != nil {
					value = metric.Untyped.GetValue()
					found = true
				}
			case io_prometheus_client.MetricType_SUMMARY:
				// For summaries, we export count and sum
				if metric.Summary != nil {
					// Sum
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: append(labels, prompb.Label{Name: "quantile", Value: "sum"}),
						Samples: []prompb.Sample{
							{Value: metric.Summary.GetSampleSum(), Timestamp: timestamp},
						},
					})
					// Count
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: append(labels, prompb.Label{Name: "quantile", Value: "count"}),
						Samples: []prompb.Sample{
							{Value: float64(metric.Summary.GetSampleCount()), Timestamp: timestamp},
						},
					})
				}
				continue
			case io_prometheus_client.MetricType_HISTOGRAM:
				// For histograms, we export buckets, sum, and count
				if metric.Histogram != nil {
					for _, bucket := range metric.Histogram.GetBucket() {
						bucketLabels := append(labels, prompb.Label{
							Name:  "le",
							Value: fmt.Sprintf("%v", bucket.GetUpperBound()),
						})
						timeseries = append(timeseries, prompb.TimeSeries{
							Labels: bucketLabels,
							Samples: []prompb.Sample{
								{Value: float64(bucket.GetCumulativeCount()), Timestamp: timestamp},
							},
						})
					}
					// Sum
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: append(labels, prompb.Label{Name: "le", Value: "sum"}),
						Samples: []prompb.Sample{
							{Value: metric.Histogram.GetSampleSum(), Timestamp: timestamp},
						},
					})
					// Count
					timeseries = append(timeseries, prompb.TimeSeries{
						Labels: append(labels, prompb.Label{Name: "le", Value: "count"}),
						Samples: []prompb.Sample{
							{Value: float64(metric.Histogram.GetSampleCount()), Timestamp: timestamp},
						},
					})
				}
				continue
			}

			if found {
				timeseries = append(timeseries, prompb.TimeSeries{
					Labels: labels,
					Samples: []prompb.Sample{
						{Value: value, Timestamp: timestamp},
					},
				})
			}
		}
	}

	return timeseries, nil
}

func pushToMimir(client *http.Client, url string, timeseries []prompb.TimeSeries) error {
	// Create WriteRequest
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Marshal to protobuf
	data, err := proto.Marshal(writeRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	// Compress with snappy
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
