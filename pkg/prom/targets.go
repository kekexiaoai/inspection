package prom

import (
	"fmt"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// ActiveTargetByPool 表示按 scrapePool 分类的活跃目标
type ActiveTargetByPool struct {
	ScrapePool string            `json:"scrapePool"`
	Targets    []v1.ActiveTarget `json:"targets"`
}

// TargetHealthStatus 定义目标健康状态的常量
const (
	TargetHealthGood    = "up"      // 健康状态
	TargetHealthBad     = "down"    // 不健康状态
	TargetHealthUnknown = "unknown" // 未知状态
)

// GetActiveTargetsByPool 获取所有活跃目标并按 scrapePool 分类
// GetActiveTargetsByPool 获取所有活跃目标并按 scrapePool 分类
func (c *Client) GetActiveTargetsByPool() ([]ActiveTargetByPool, error) {
	targetsResult, err := c.Targets()
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	// 按 scrapePool 分类
	poolMap := make(map[string][]v1.ActiveTarget)
	for _, target := range targetsResult.Active {
		poolMap[target.ScrapePool] = append(poolMap[target.ScrapePool], target)
	}

	// 转换为切片格式
	var result []ActiveTargetByPool
	for pool, targets := range poolMap {
		result = append(result, ActiveTargetByPool{
			ScrapePool: pool,
			Targets:    targets,
		})
	}

	return result, nil
}

// GetActiveTargetsByPoolWithFilter 获取活跃目标并按 scrapePool 分类，支持过滤器
func (c *Client) GetActiveTargetsByPoolWithFilter(filterFunc func(target v1.ActiveTarget) bool) ([]ActiveTargetByPool, error) {
	targetsResult, err := c.Targets()
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	// 按 scrapePool 分类并过滤
	poolMap := make(map[string][]v1.ActiveTarget)
	for _, target := range targetsResult.Active {
		// 如果提供了过滤器，只添加符合条件的目标
		if filterFunc == nil || filterFunc(target) {
			poolMap[target.ScrapePool] = append(poolMap[target.ScrapePool], target)
		}
	}

	// 转换为切片格式
	var result []ActiveTargetByPool
	for pool, targets := range poolMap {
		if len(targets) > 0 { // 只返回有目标的池
			result = append(result, ActiveTargetByPool{
				ScrapePool: pool,
				Targets:    targets,
			})
		}
	}

	return result, nil
}

// GetOnlineTargetsByPool 获取在线状态的活跃目标并按 scrapePool 分类
func (c *Client) GetOnlineTargetsByPool() ([]ActiveTargetByPool, error) {
	return c.GetActiveTargetsByPoolWithFilter(func(target v1.ActiveTarget) bool {
		return target.Health == TargetHealthGood
	})
}

// GetOfflineTargetsByPool 获取离线状态的活跃目标并按 scrapePool 分类
func (c *Client) GetOfflineTargetsByPool() ([]ActiveTargetByPool, error) {
	return c.GetActiveTargetsByPoolWithFilter(func(target v1.ActiveTarget) bool {
		return target.Health == TargetHealthBad
	})
}

// TargetPoolStats 表示每个 scrapePool 的统计信息
type TargetPoolStats struct {
	ScrapePool   string `json:"scrapePool"`
	TotalCount   int    `json:"totalCount"`
	OnlineCount  int    `json:"onlineCount"`
	OfflineCount int    `json:"offlineCount"`
	UnknownCount int    `json:"unknownCount"`
}

// GetTargetPoolStats 获取每个 scrapePool 的统计信息
func (c *Client) GetTargetPoolStats() ([]TargetPoolStats, error) {
	targetsResult, err := c.Targets()
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	statsMap := make(map[string]*TargetPoolStats)

	// 统计每个池的目标状态
	for _, target := range targetsResult.Active {
		stats, exists := statsMap[target.ScrapePool]
		if !exists {
			stats = &TargetPoolStats{
				ScrapePool: target.ScrapePool,
			}
			statsMap[target.ScrapePool] = stats
		}

		stats.TotalCount++
		switch target.Health {
		case TargetHealthGood:
			stats.OnlineCount++
		case TargetHealthBad:
			stats.OfflineCount++
		default:
			stats.UnknownCount++
		}
	}

	// 转换为切片
	var result []TargetPoolStats
	for _, stats := range statsMap {
		result = append(result, *stats)
	}

	return result, nil
}

func (c *Client) GetTargetHealthSummary() (map[string]int, error) {
	allTargets, err := c.GetActiveTargetsByPool()
	if err != nil {
		return nil, err
	}

	summary := make(map[string]int)
	for _, pool := range allTargets {
		for _, target := range pool.Targets {
			summary[string(target.Health)]++
		}
	}

	return summary, nil
}

// PrintTargetsByPool 打印按 scrapePool 分类的目标信息
func PrintTargetsByPool(pools []ActiveTargetByPool) {
	fmt.Printf("Targets grouped by scrapePool:\n")
	fmt.Printf("================================\n")

	for _, pool := range pools {
		fmt.Printf("\nScrape Pool: %s (Total: %d)\n", pool.ScrapePool, len(pool.Targets))
		fmt.Printf("--------------------------------\n")

		for i, target := range pool.Targets {
			fmt.Printf("  %d. URL: %s\n", i+1, target.ScrapeURL)
			fmt.Printf("     Health: %s\n", target.Health)
			if target.LastError != "" {
				fmt.Printf("     Last Error: %s\n", target.LastError)
			}
			fmt.Printf("     Labels: %v\n", target.Labels)
		}
	}
}

// PrintTargetPoolStats 打印目标池统计信息
func PrintTargetPoolStats(stats []TargetPoolStats) {
	fmt.Printf("Target Pool Statistics:\n")
	fmt.Printf("======================\n")

	for _, stat := range stats {
		fmt.Printf("\nPool: %s\n", stat.ScrapePool)
		fmt.Printf("  Total:   %d\n", stat.TotalCount)
		fmt.Printf("  Online:  %d\n", stat.OnlineCount)
		fmt.Printf("  Offline: %d\n", stat.OfflineCount)
		fmt.Printf("  Unknown: %d\n", stat.UnknownCount)
	}
}
