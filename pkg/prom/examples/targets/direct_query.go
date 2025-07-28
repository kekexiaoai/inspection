package targets

import (
	"fmt"
	"log"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func RunTargetsExamples(client *prom.Client) {
	// 直接查询示例
	fmt.Println("Direct Targets Query:")
	directTargetsQuery(client)

	// 缓存查询示例
	fmt.Println("\nCached Targets Query:")
	cachedTargetsQuery(client)

	// 索引缓存示例
	fmt.Println("\nIndexed Cache Query:")
	indexedCacheQuery(client)

	// 业务场景示例
	fmt.Println("\nBusiness Scenario:")
	businessScenario(client)
}

func directTargetsQuery(client *prom.Client) {
	// 获取所有 targets
	allTargets, err := client.GetActiveTargetsByPool()
	if err != nil {
		log.Printf("Failed to get targets: %v", err)
		return
	}
	fmt.Printf("Found %d target pools\n", len(allTargets))

	// 获取统计信息
	stats, err := client.GetTargetPoolStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		return
	}
	fmt.Printf("Found %d target pool stats\n", len(stats))
}
