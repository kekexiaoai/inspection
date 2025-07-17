package main

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

var (
	prometheusServer = flag.String("server", "http://10.120.1.6:9090", "Prometheus server URL")
	queryTimeout     = flag.Duration("timeout", 10*time.Second, "Query timeout")
)

func TestInstantQuery(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	now := time.Now()

	testCases := []struct {
		name  string
		query string
	}{
		{"BasicUpQuery", "up"},
		{"FilteredUpQuery", "up{job=\"prometheus\"}"},
		{"RateCalculation", "rate(prometheus_http_requests_total[5m])"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n=== Testing instant query: %s ===\n", tc.query)
			err := prom.ExecuteQuery(client, tc.query, now, prom.DefaultVectorHandler)
			if err != nil {
				t.Errorf("Query failed: %v", err)
			}
		})
	}
}

func TestRangeQuery(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	step := time.Minute

	testCases := []struct {
		name  string
		query string
	}{
		{"TimeSeriesRate", "rate(prometheus_tsdb_head_samples_appended_total[5m])"},
		{"CounterIncrease", "increase(prometheus_http_requests_total[1h])"},
		{"GaugeValue", "prometheus_tsdb_head_series"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n=== Testing range query: %s ===\n", tc.query)
			fmt.Printf("Time range: %s to %s, step: %s\n",
				start.Format(time.RFC3339), now.Format(time.RFC3339), step)

			err := prom.ExecuteQueryRange(client, tc.query, start, now, step, prom.DefaultMatrixHandler)
			if err != nil {
				t.Errorf("Range query failed: %v", err)
			}
		})
	}
}

func TestCustomHandler(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	now := time.Now()

	customHandler := func(t any) error {
		switch v := t.(type) {
		case *model.Sample:
			labels := make(map[string]string)
			for k, val := range v.Metric {
				labels[string(k)] = string(val)
			}
			fmt.Printf("Custom Handler - Labels: %v, Value: %.2f\n", labels, float64(v.Value))
		case *model.SampleStream:
			fmt.Printf("Custom Handler - Stream with %d values for metric %v\n",
				len(v.Values), v.Metric)
			// 仅显示前3个值以避免输出过多
			for i, val := range v.Values {
				if i >= 3 {
					break
				}
				fmt.Printf("  Time: %s, Value: %.2f\n",
					val.Timestamp.Time().Format(time.RFC3339), float64(val.Value))
			}
			if len(v.Values) > 3 {
				fmt.Printf("  ... and %d more values\n", len(v.Values)-3)
			}
		default:
			return fmt.Errorf("unexpected type %T in custom handler", t)
		}
		return nil
	}

	testCases := []struct {
		name  string
		query string
	}{
		{"CustomUpQuery", "up"},
		{"CustomRateQuery", "rate(prometheus_http_requests_total[5m])"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n=== Testing custom handler with query: %s ===\n", tc.query)
			err := prom.ExecuteQuery(client, tc.query, now, customHandler)
			if err != nil {
				t.Errorf("Query with custom handler failed: %v", err)
			}
		})
	}
}

func createClient() (*prom.Client, error) {
	// 初始化命令行参数（如果尚未初始化）
	if !flag.Parsed() {
		flag.Parse()
	}

	// 创建 Prometheus 客户端
	client, err := prom.NewClient(*prometheusServer, prom.WithTimeout(*queryTimeout))
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	return client, nil
}
