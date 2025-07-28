package prom

import (
	"testing"
	"time"
)

func TestIndexedTargetCache(t *testing.T) {
	client := RequireTestClient(t)

	// 创建缓存
	cache := NewIndexedTargetCache(client, 5*time.Second)
	defer cache.Close()

	// 测试缓存创建
	if cache == nil {
		t.Fatal("Failed to create indexed target cache")
	}

	// 测试基本查询功能
	targets := cache.GetTargetsByHealth("up")
	t.Logf("Found %d online targets", len(targets))

	targets = cache.GetTargetsByHealth("down")
	t.Logf("Found %d offline targets", len(targets))

	// 测试按 job 查询
	targets = cache.GetTargetsByJob("prometheus")
	t.Logf("Found %d prometheus targets", len(targets))

	// 测试按 pool 查询
	targets = cache.GetTargetsByPool("prometheus")
	t.Logf("Found %d targets in prometheus pool", len(targets))

	// 测试组合查询
	targets = cache.GetTargetsByJobAndHealth("prometheus", "up")
	t.Logf("Found %d online prometheus targets", len(targets))

	// 测试获取所有 targets
	allTargets := cache.GetAllTargetsByPool()
	t.Logf("Found %d target pools in cache", len(allTargets))
}

func TestNormalTargetCache(t *testing.T) {
	client := RequireTestClient(t)

	// 创建普通缓存
	cache := NewTargetCache(client, 5*time.Second)
	defer cache.Close()

	if cache == nil {
		t.Fatal("Failed to create target cache")
	}

	// 测试获取不同类型的数据
	testCases := []string{"all", "online", "offline"}
	for _, targetType := range testCases {
		targets, err := cache.GetTargetsByType(targetType)
		if err != nil {
			t.Logf("Cache miss for %s targets (expected on first access): %v", targetType, err)
		} else {
			t.Logf("Found %d %s target pools", len(targets), targetType)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	client := RequireTestClient(t)

	cache := NewIndexedTargetCache(client, 10*time.Second)
	defer cache.Close()

	// 并发测试
	done := make(chan bool)
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			// 不同的查询类型
			switch goroutineID % 4 {
			case 0:
				targets := cache.GetTargetsByHealth("up")
				_ = len(targets)
			case 1:
				targets := cache.GetTargetsByJob("prometheus")
				_ = len(targets)
			case 2:
				targets := cache.GetTargetsByPool("prometheus")
				_ = len(targets)
			case 3:
				targets := cache.GetAllTargetsByPool()
				_ = len(targets)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestCacheRefresh(t *testing.T) {
	client := RequireTestClient(t)

	// 创建短 TTL 的缓存用于测试
	cache := NewIndexedTargetCache(client, 1*time.Second)
	defer cache.Close()

	// 强制刷新
	cache.mutex.Lock()
	err := cache.refreshCacheUnsafe()
	cache.mutex.Unlock()
	if err != nil {
		t.Fatalf("Cache refresh failed: %v", err)
	}

	// 检查缓存时间
	cache.mutex.RLock()
	cacheTime := cache.cacheTime
	cache.mutex.RUnlock()

	if cacheTime.IsZero() {
		t.Error("Cache time should be set after refresh")
	}

	t.Logf("Cache last updated: %v", cacheTime)
}

func TestCacheBackgroundRefresh(t *testing.T) {
	client := RequireTestClient(t)

	// 创建很短 TTL 的缓存来测试后台刷新
	cache := NewIndexedTargetCache(client, 100*time.Millisecond)
	defer cache.Close()

	// 等待后台刷新发生
	time.Sleep(200 * time.Millisecond)

	// 检查缓存是否已更新
	cache.mutex.RLock()
	cacheTime := cache.cacheTime
	cache.mutex.RUnlock()

	if cacheTime.IsZero() {
		t.Error("Cache should be refreshed by background goroutine")
	}

	t.Logf("Background refresh successful, cache updated at: %v", cacheTime)
}

func TestCacheClose(t *testing.T) {
	client := RequireTestClient(t)

	cache := NewIndexedTargetCache(client, 1*time.Second)

	// 关闭缓存
	cache.Close()

	// 再次关闭不应该 panic
	cache.Close()

	t.Log("Cache closed successfully")
}
