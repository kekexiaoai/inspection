package prom

import (
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestGetActiveTargetsByPool(t *testing.T) {
	client := RequireTestClient(t)

	targets, err := client.GetActiveTargetsByPool()
	if err != nil {
		t.Fatalf("GetActiveTargetsByPool failed: %v", err)
	}

	// 验证返回结果结构
	for _, pool := range targets {
		if pool.ScrapePool == "" {
			t.Error("ScrapePool should not be empty")
		}
		if pool.Targets == nil {
			t.Error("Targets should not be nil")
		}
	}

	t.Logf("Found %d target pools", len(targets))
}

func TestGetOnlineTargetsByPool(t *testing.T) {
	client := RequireTestClient(t)

	onlineTargets, err := client.GetOnlineTargetsByPool()
	if err != nil {
		t.Fatalf("GetOnlineTargetsByPool failed: %v", err)
	}

	// 验证所有返回的 targets 都是在线状态
	for _, pool := range onlineTargets {
		for _, target := range pool.Targets {
			if target.Health != TargetHealthGood {
				t.Errorf("Expected health 'up', got '%s'", target.Health)
			}
		}
	}

	t.Logf("Found %d online target pools", len(onlineTargets))
}

func TestGetOfflineTargetsByPool(t *testing.T) {
	client := RequireTestClient(t)

	offlineTargets, err := client.GetOfflineTargetsByPool()
	if err != nil {
		t.Fatalf("GetOfflineTargetsByPool failed: %v", err)
	}

	// 验证所有返回的 targets 都是离线状态（如果有的话）
	for _, pool := range offlineTargets {
		for _, target := range pool.Targets {
			if target.Health != TargetHealthBad && target.Health != TargetHealthUnknown {
				t.Errorf("Expected health 'down' or 'unknown', got '%s'", target.Health)
			}
		}
	}

	t.Logf("Found %d offline target pools", len(offlineTargets))
}

func TestGetTargetPoolStats(t *testing.T) {
	client := RequireTestClient(t)

	stats, err := client.GetTargetPoolStats()
	if err != nil {
		t.Fatalf("GetTargetPoolStats failed: %v", err)
	}

	// 验证统计信息的完整性
	totalCount := 0
	for _, stat := range stats {
		if stat.ScrapePool == "" {
			t.Error("ScrapePool should not be empty in stats")
		}
		totalCount += stat.TotalCount
		totalCountCheck := stat.OnlineCount + stat.OfflineCount + stat.UnknownCount
		if stat.TotalCount != totalCountCheck {
			t.Errorf("Total count mismatch for pool %s: got %d, expected %d",
				stat.ScrapePool, stat.TotalCount, totalCountCheck)
		}
	}

	t.Logf("Total targets across all pools: %d", totalCount)
	t.Logf("Found %d target pools in stats", len(stats))
}

func TestFilterFunctionality(t *testing.T) {
	client := RequireTestClient(t)

	// 测试自定义过滤器 - 获取所有 targets
	filteredTargets, err := client.GetActiveTargetsByPoolWithFilter(func(target v1.ActiveTarget) bool {
		return true
	})
	if err != nil {
		t.Fatalf("Custom filter failed: %v", err)
	}

	// 应该返回所有 targets
	allTargets, err := client.GetActiveTargetsByPool()
	if err != nil {
		t.Fatalf("Get all targets failed: %v", err)
	}

	// 比较结果数量
	totalFiltered := 0
	for _, pool := range filteredTargets {
		totalFiltered += len(pool.Targets)
	}

	totalAll := 0
	for _, pool := range allTargets {
		totalAll += len(pool.Targets)
	}

	if totalFiltered != totalAll {
		t.Errorf("Filtered results don't match all targets. Got %d, expected %d",
			totalFiltered, totalAll)
	}
}

func TestPrintFunctions(t *testing.T) {
	client := RequireTestClient(t)

	targets, err := client.GetActiveTargetsByPool()
	if err != nil {
		t.Fatalf("Failed to get targets for printing: %v", err)
	}

	// 测试打印功能不会 panic
	PrintTargetsByPool(targets)

	stats, err := client.GetTargetPoolStats()
	if err != nil {
		t.Fatalf("Failed to get stats for printing: %v", err)
	}

	PrintTargetPoolStats(stats)
}

func TestGetTargetHealthSummary(t *testing.T) {
	client := RequireTestClient(t)

	summary, err := client.GetTargetHealthSummary()
	if err != nil {
		t.Fatalf("GetTargetHealthSummary failed: %v", err)
	}

	total := 0
	for health, count := range summary {
		t.Logf("Health %s: %d targets", health, count)
		total += count
	}

	t.Logf("Total targets: %d", total)
}
