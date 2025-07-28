package targets

import (
	"fmt"
	"time"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

func businessScenario(client *prom.Client) {
	// 创建缓存用于业务场景
	cache := prom.NewIndexedTargetCache(client, 30*time.Second)
	defer cache.Close()

	fmt.Println("Monitoring Scenario:")

	// 检查离线服务
	offlineTargets := cache.GetTargetsByHealth("down")
	if len(offlineTargets) > 0 {
		fmt.Printf("⚠️  Warning: %d targets offline\n", len(offlineTargets))
	} else {
		fmt.Println("✅ All targets are online")
	}

	// 健康状态统计
	online := cache.GetTargetsByHealth("up")
	offline := cache.GetTargetsByHealth("down")
	unknown := cache.GetTargetsByHealth("unknown")

	total := len(online) + len(offline) + len(unknown)
	fmt.Printf("Health Summary - Total: %d, Online: %d, Offline: %d, Unknown: %d\n",
		total, len(online), len(offline), len(unknown))
}
