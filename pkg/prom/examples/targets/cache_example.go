package targets

import (
	"fmt"
	"log"
	"time"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func cachedTargetsQuery(client *prom.Client) {
	// 普通缓存
	cache := prom.NewTargetCache(client, 30*time.Second)
	defer cache.Close()

	fmt.Println("Normal Cache Example:")

	// 第一次查询
	fmt.Println("First query (API call):")
	start := time.Now()
	targets, err := cache.GetTargetsByType("all")
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	firstTime := time.Since(start)
	fmt.Printf("Found %d pools, time: %v\n", len(targets), firstTime)

	// 第二次查询（使用缓存）
	fmt.Println("Second query (cached):")
	start = time.Now()
	targets, err = cache.GetTargetsByType("all")
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}
	secondTime := time.Since(start)
	fmt.Printf("Found %d pools, time: %v\n", len(targets), secondTime)

	if secondTime > 0 {
		fmt.Printf("Performance improvement: %.2fx\n", float64(firstTime)/float64(secondTime))
	}
}

func indexedCacheQuery(client *prom.Client) {
	// 索引缓存
	indexedCache := prom.NewIndexedTargetCache(client, 10*time.Second)
	defer indexedCache.Close()

	fmt.Println("Indexed Cache Example:")

	// O(1) 查询
	onlineTargets := indexedCache.GetTargetsByHealth("up")
	fmt.Printf("Online targets: %d\n", len(onlineTargets))

	prometheusTargets := indexedCache.GetTargetsByJob("prometheus")
	fmt.Printf("Prometheus targets: %d\n", len(prometheusTargets))

	allTargets := indexedCache.GetAllTargetsByPool()
	fmt.Printf("All target pools: %d\n", len(allTargets))
}
