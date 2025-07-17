package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func main() {
	// 从命令行参数获取配置
	serverURL := flag.String("server", "http://10.120.1.6:9090", "Prometheus server URL")
	timeout := flag.Duration("timeout", 10*time.Second, "Query timeout")
	flag.Parse()

	// 创建 Prometheus 客户端
	client, err := prom.NewClient(*serverURL, prom.WithTimeout(*timeout))
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	now := time.Now()

	// 示例 1: 简单即时查询 - 获取所有目标的状态
	if err := executeInstantQuery(ctx, client, "up", now); err != nil {
		log.Printf("Instant query failed: %v", err)
	}

	// 示例 2: 带标签过滤的即时查询
	if err := executeInstantQuery(ctx, client, "up{job=\"prometheus\"}", now); err != nil {
		log.Printf("Filtered instant query failed: %v", err)
	}

	// 示例 3: 范围查询 - 获取时间序列数据
	rangeQuery := "rate(prometheus_tsdb_head_samples_appended_total[5m])"
	if err := executeRangeQuery(ctx, client, rangeQuery, now.Add(-1*time.Hour), now, time.Minute); err != nil {
		log.Printf("Range query failed: %v", err)
	}

	// 示例 4: 使用自定义处理器处理查询结果
	if err := executeCustomQuery(ctx, client, "up", now); err != nil {
		log.Printf("Custom query failed: %v", err)
	}
}

// executeInstantQuery 执行即时查询并使用默认处理器
func executeInstantQuery(ctx context.Context, client *prom.Client, query string, ts time.Time) error {
	fmt.Printf("\nExecuting instant query: %s\n", query)
	return prom.ExecuteQuery(client, query, ts, prom.DefaultVectorHandler)
}

// executeRangeQuery 执行范围查询并使用默认处理器
func executeRangeQuery(ctx context.Context, client *prom.Client, query string, start, end time.Time, step time.Duration) error {
	fmt.Printf("\nExecuting range query: %s\n", query)
	fmt.Printf("Time range: %s to %s, step: %s\n", start.Format(time.RFC3339), end.Format(time.RFC3339), step)

	return prom.ExecuteQueryRange(client, query, start, end, step, prom.DefaultMatrixHandler)
}

// executeCustomQuery 执行查询并使用自定义处理器
func executeCustomQuery(ctx context.Context, client *prom.Client, query string, ts time.Time) error {
	fmt.Printf("\nExecuting custom query: %s\n", query)

	customHandler := func(t any) error {
		switch v := t.(type) {
		case *model.Sample:
			labels := make(map[string]string)
			for k, val := range v.Metric {
				labels[string(k)] = string(val)
			}
			fmt.Printf("Custom Handler - Labels: %v, Value: %.2f\n", labels, float64(v.Value))
		case *model.SampleStream:
			fmt.Printf("Custom Handler - Stream with %d values\n", len(v.Values))
			// 仅显示前3个值以避免输出过多
			for i, val := range v.Values {
				if i >= 3 {
					break
				}
				fmt.Printf("  Time: %s, Value: %.2f\n", val.Timestamp.Time().Format(time.RFC3339), float64(val.Value))
			}
			if len(v.Values) > 3 {
				fmt.Printf("  ... and %d more values\n", len(v.Values)-3)
			}
		default:
			return fmt.Errorf("unexpected type %T in custom handler", t)
		}
		return nil
	}

	return prom.ExecuteQuery(client, query, ts, customHandler)
}
