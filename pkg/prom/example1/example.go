package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

// 全局持久化客户端
var promClient *prom.Client

func init() {
	var err error
	promClient, err = prom.NewClient("http://prometheus-server:9090")
	if err != nil {
		log.Fatalf("Failed to create Prometheus client: %v", err)
	}
}

func main() {
	defer promClient.Close() // 程序退出时释放资源

	// 示例1：使用默认超时的简单查询
	result, warnings, err := promClient.Query("up", time.Now())
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	fmt.Printf("Query result: %v, warnings: %v\n", result, warnings)

	// 示例2：使用自定义超时的简单查询（链式调用）
	result, warnings, err = promClient.WithTimeout(5*time.Second).Query("up", time.Now())
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	fmt.Printf("Query result with 5s timeout: %v, warnings: %v\n", result, warnings)

	// 示例3：使用自定义超时的范围查询
	rangeClient := promClient.WithTimeout(30 * time.Second)
	rangeResult, warnings, err := rangeClient.QueryRange(
		"sum(rate(http_requests_total[5m]))",
		prom.NewRange(
			time.Now().Add(-1*time.Hour),
			time.Now(),
			time.Minute,
		),
	)
	if err != nil {
		log.Printf("Range query failed: %v", err)
		return
	}
	fmt.Printf("Range query result: %v, warnings: %v\n", rangeResult, warnings)
}
