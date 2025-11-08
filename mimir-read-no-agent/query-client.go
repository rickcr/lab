package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const mimirQueryURL = "http://localhost:9009/prometheus/api/v1"

type QueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type RangeQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func main() {
	client := &http.Client{Timeout: 10 * time.Second}

	fmt.Println("=== Mimir Query Demo ===\n")

	// List all available metrics
	fmt.Println("1. Listing all available metrics...")
	metrics, err := listMetrics(client)
	if err != nil {
		log.Fatalf("Error listing metrics: %v", err)
	}
	fmt.Printf("Found %d metrics:\n", len(metrics))
	for i, metric := range metrics {
		if i < 10 { // Show first 10
			fmt.Printf("  - %s\n", metric)
		}
	}
	if len(metrics) > 10 {
		fmt.Printf("  ... and %d more\n", len(metrics)-10)
	}
	fmt.Println()

	// Query current value of a specific metric
	fmt.Println("2. Querying current value of 'go_goroutines'...")
	value, err := queryInstant(client, "go_goroutines")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Current value: %v\n", value)
	}
	fmt.Println()

	// Query custom metric
	fmt.Println("3. Querying 'last_metric_update_timestamp_seconds'...")
	value, err = queryInstant(client, "last_metric_update_timestamp_seconds")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Timestamp: %v\n", value)
		if ts, ok := value.(float64); ok {
			fmt.Printf("Human readable: %s\n", time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05"))
		}
	}
	fmt.Println()

	// Query HTTP requests total
	fmt.Println("4. Querying 'promhttp_metric_handler_requests_total'...")
	resp, err := queryInstantFull(client, "promhttp_metric_handler_requests_total")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Found %d series:\n", len(resp.Data.Result))
		for _, result := range resp.Data.Result {
			fmt.Printf("  Labels: %v, Value: %v\n", result.Metric, result.Value[1])
		}
	}
	fmt.Println()

	// Query range (last 5 minutes)
	fmt.Println("5. Querying 'go_goroutines' over last 5 minutes...")
	rangeData, err := queryRange(client, "go_goroutines", 5*time.Minute, 15*time.Second)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		if len(rangeData.Data.Result) > 0 {
			values := rangeData.Data.Result[0].Values
			fmt.Printf("Got %d data points:\n", len(values))
			// Show first 5 and last 5
			showCount := 5
			if len(values) < showCount {
				showCount = len(values)
			}
			for i := 0; i < showCount; i++ {
				timestamp := values[i][0].(float64)
				value := values[i][1].(string)
				fmt.Printf("  %s: %s\n", time.Unix(int64(timestamp), 0).Format("15:04:05"), value)
			}
			if len(values) > showCount*2 {
				fmt.Printf("  ... %d more points ...\n", len(values)-showCount*2)
				for i := len(values) - showCount; i < len(values); i++ {
					timestamp := values[i][0].(float64)
					value := values[i][1].(string)
					fmt.Printf("  %s: %s\n", time.Unix(int64(timestamp), 0).Format("15:04:05"), value)
				}
			}
		}
	}
	fmt.Println()

	// Query with rate calculation
	fmt.Println("6. Querying rate of requests (requests/sec over 1m)...")
	value, err = queryInstant(client, "rate(promhttp_metric_handler_requests_total[1m])")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Request rate: %v requests/second\n", value)
	}
	fmt.Println()

	fmt.Println("=== Query Demo Complete ===")
	fmt.Println("\nTry these queries in Grafana (http://localhost:3000):")
	fmt.Println("  - go_goroutines")
	fmt.Println("  - go_memstats_alloc_bytes")
	fmt.Println("  - rate(promhttp_metric_handler_requests_total[1m])")
	fmt.Println("  - last_metric_update_timestamp_seconds")
}

func listMetrics(client *http.Client) ([]string, error) {
	url := fmt.Sprintf("%s/label/__name__/values", mimirQueryURL)
	
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("query failed with status: %s", result.Status)
	}

	return result.Data, nil
}

func queryInstant(client *http.Client, query string) (interface{}, error) {
	resp, err := queryInstantFull(client, query)
	if err != nil {
		return nil, err
	}

	if len(resp.Data.Result) == 0 {
		return nil, fmt.Errorf("no data returned")
	}

	return resp.Data.Result[0].Value[1], nil
}

func queryInstantFull(client *http.Client, query string) (*QueryResponse, error) {
	params := url.Values{}
	params.Add("query", query)
	
	queryURL := fmt.Sprintf("%s/query?%s", mimirQueryURL, params.Encode())
	
	resp, err := client.Get(queryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("query failed with status: %s", result.Status)
	}

	return &result, nil
}

func queryRange(client *http.Client, query string, duration time.Duration, step time.Duration) (*RangeQueryResponse, error) {
	now := time.Now()
	start := now.Add(-duration)

	params := url.Values{}
	params.Add("query", query)
	params.Add("start", fmt.Sprintf("%d", start.Unix()))
	params.Add("end", fmt.Sprintf("%d", now.Unix()))
	params.Add("step", fmt.Sprintf("%ds", int(step.Seconds())))

	queryURL := fmt.Sprintf("%s/query_range?%s", mimirQueryURL, params.Encode())

	resp, err := client.Get(queryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RangeQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("query failed with status: %s", result.Status)
	}

	return &result, nil
}
