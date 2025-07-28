package timeout

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func RunTimeoutExamples(client *prom.Client) {
	// 示例1：使用默认超时的简单查询
	fmt.Println("=== Default Timeout Query ===")
	result, warnings, err := client.Query("up", time.Now())
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	fmt.Printf("Query result: %v, warnings: %v\n", result, warnings)

	// 示例2：使用自定义超时的简单查询（链式调用）
	fmt.Println("\n=== Custom Timeout Query (5s) ===")
	result, warnings, err = client.WithTimeout(5*time.Second).Query("up", time.Now())
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	fmt.Printf("Query result with 5s timeout: %v, warnings: %v\n", result, warnings)

	// 示例3：使用自定义超时的范围查询
	fmt.Println("\n=== Custom Timeout Range Query (30s) ===")
	rangeClient := client.WithTimeout(30 * time.Second)
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

	// 示例4：上下文超时示例
	fmt.Println("\n=== Context Timeout Example ===")
	executeContextTimeoutExample(client)
}

func executeContextTimeoutExample(client *prom.Client) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用带上下文的客户端
	contextClient := client.WithContext(ctx)

	fmt.Println("Executing query with 2-second context timeout...")
	result, warnings, err := contextClient.Query("up", time.Now())
	if err != nil {
		fmt.Printf("Query failed (possibly timeout): %v\n", err)
	} else {
		fmt.Printf("Query succeeded: %v\n", result)
		if len(warnings) > 0 {
			fmt.Printf("Warnings: %v\n", warnings)
		}
	}
}
