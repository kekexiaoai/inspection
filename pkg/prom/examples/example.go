package main

import (
	"fmt"
	"log"

	"github.com/kekexiaoai/inspection/pkg/prom"
	"github.com/kekexiaoai/inspection/pkg/prom/examples/advanced"
	"github.com/kekexiaoai/inspection/pkg/prom/examples/basic"
	"github.com/kekexiaoai/inspection/pkg/prom/examples/targets"
	"github.com/kekexiaoai/inspection/pkg/prom/examples/timeout"
)

// 全局持久化客户端
var promClient *prom.Client

func init() {
	var err error
	promClient, err = prom.NewClient("http://10.120.1.6:9090")
	if err != nil {
		log.Fatalf("Failed to create Prometheus client: %v", err)
	}
}

func main() {
	defer promClient.Close()

	fmt.Println("=== Prometheus Client Examples ===")

	// 运行基本示例
	fmt.Println("\n1. Running Basic Examples...")
	basic.RunBasicExamples(promClient)

	// 运行超时示例
	fmt.Println("\n2. Running Timeout Examples...")
	timeout.RunTimeoutExamples(promClient)

	// 运行 targets 示例
	fmt.Println("\n3. Running Targets Examples...")
	targets.RunTargetsExamples(promClient)

	// 运行高级示例
	fmt.Println("\n4. Running Advanced Examples...")
	advanced.RunAdvancedExamples(promClient)

	fmt.Println("\n=== All Examples Completed ===")
}
