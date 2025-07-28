package prom

import (
	"testing"
	"time"
)

func TestFullIntegration(t *testing.T) {
	client := RequireTestClient(t)

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 直接使用客户端查询
		t.Log("=== Direct Client Usage ===")
		targets, err := client.GetActiveTargetsByPool()
		if err != nil {
			t.Fatalf("Direct client query failed: %v", err)
		}
		t.Logf("Direct query found %d target pools", len(targets))

		// 使用索引缓存
		t.Log("=== Indexed Cache Usage ===")
		indexedCache := NewIndexedTargetCache(client, 10*time.Second)
		defer indexedCache.Close()

		onlineTargets := indexedCache.GetTargetsByHealth("up")
		t.Logf("Indexed cache found %d online targets", len(onlineTargets))

		offlineTargets := indexedCache.GetTargetsByHealth("down")
		t.Logf("Indexed cache found %d offline targets", len(offlineTargets))

		// 使用普通缓存
		t.Log("=== Normal Cache Usage ===")
		normalCache := NewTargetCache(client, 30*time.Second)
		defer normalCache.Close()

		allTargets, err := normalCache.GetTargetsByType("all")
		if err != nil {
			t.Logf("Normal cache miss (expected): %v", err)
		} else {
			t.Logf("Normal cache found %d target pools", len(allTargets))
		}

		// 性能对比测试
		t.Log("=== Performance Comparison ===")

		// 直接查询性能测试（减少次数避免超时）
		start := time.Now()
		for i := 0; i < 10; i++ {
			_, _ = client.GetActiveTargetsByPool()
		}
		directTime := time.Since(start)
		t.Logf("10 direct queries took: %v", directTime)

		// 缓存查询性能测试
		start = time.Now()
		for i := 0; i < 100; i++ {
			_ = indexedCache.GetTargetsByHealth("up")
		}
		cacheTime := time.Since(start)
		t.Logf("100 cache queries took: %v", cacheTime)

		if cacheTime > 0 {
			t.Logf("Performance improvement: %.2fx", float64(directTime*10)/float64(cacheTime))
		}
	})
}
