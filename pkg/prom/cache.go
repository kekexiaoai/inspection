package prom

import (
	"fmt"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// ActiveTargetsByPool 使用 map 结构存储按 scrapePool 分组的 targets
type ActiveTargetsByPool map[string][]v1.ActiveTarget

// TargetIndex 多维度索引
type TargetIndex struct {
	ByJob         map[string][]v1.ActiveTarget
	ByInstance    map[string][]v1.ActiveTarget
	ByHealth      map[string][]v1.ActiveTarget
	ByPool        map[string][]v1.ActiveTarget
	ByCustomLabel map[string]map[string][]v1.ActiveTarget // 支持任意标签索引
}

// IndexedTargetCache 带索引的目标缓存
type IndexedTargetCache struct {
	client     *Client
	allTargets ActiveTargetsByPool
	index      TargetIndex
	cacheTime  time.Time
	mutex      sync.RWMutex
	ttl        time.Duration
	stopChan   chan struct{}
}

// NewIndexedTargetCache 创建带索引的目标缓存
func NewIndexedTargetCache(client *Client, ttl time.Duration) *IndexedTargetCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	tc := &IndexedTargetCache{
		client:     client,
		allTargets: make(ActiveTargetsByPool),
		index: TargetIndex{
			ByJob:         make(map[string][]v1.ActiveTarget),
			ByInstance:    make(map[string][]v1.ActiveTarget),
			ByHealth:      make(map[string][]v1.ActiveTarget),
			ByPool:        make(map[string][]v1.ActiveTarget),
			ByCustomLabel: make(map[string]map[string][]v1.ActiveTarget),
		},
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}
	// 立即刷新缓存，确保缓存有初始数据
	tc.mutex.Lock()
	_ = tc.refreshCacheUnsafe()
	tc.mutex.Unlock()

	go tc.backgroundRefresh()
	return tc
}

// refreshCacheUnsafe 刷新缓存并重建索引
func (tc *IndexedTargetCache) refreshCacheUnsafe() error {
	// 通过 Client 获取原始数据
	allTargets, err := tc.client.GetActiveTargetsByPool()
	if err != nil {
		return fmt.Errorf("failed to get targets: %w", err)
	}

	// 转换为内部存储格式
	newTargets := make(ActiveTargetsByPool)
	for _, pool := range allTargets {
		newTargets[pool.ScrapePool] = pool.Targets
	}

	// 更新缓存
	tc.allTargets = newTargets

	// 重建索引
	tc.buildIndex()

	tc.cacheTime = time.Now()
	return nil
}

// buildIndex 构建多维度索引
func (tc *IndexedTargetCache) buildIndex() {
	// 清空现有索引
	tc.index.ByJob = make(map[string][]v1.ActiveTarget)
	tc.index.ByInstance = make(map[string][]v1.ActiveTarget)
	tc.index.ByHealth = make(map[string][]v1.ActiveTarget)
	tc.index.ByPool = make(map[string][]v1.ActiveTarget)
	tc.index.ByCustomLabel = make(map[string]map[string][]v1.ActiveTarget)

	// 重建索引
	for _, targets := range tc.allTargets {
		for _, target := range targets {
			// 按 job 索引
			if job, exists := target.Labels["job"]; exists {
				_job := string(job)
				tc.index.ByJob[_job] = append(tc.index.ByJob[_job], target)
			}

			// 按 instance 索引
			if instance, exists := target.Labels["instance"]; exists {
				_instance := string(instance)
				tc.index.ByInstance[_instance] = append(tc.index.ByInstance[_instance], target)
			}

			// 按健康状态索引
			tc.index.ByHealth[string(target.Health)] = append(tc.index.ByHealth[string(target.Health)], target)

			// 按 scrapePool 索引
			tc.index.ByPool[target.ScrapePool] = append(tc.index.ByPool[target.ScrapePool], target)

			// 为所有自定义标签建立索引
			for labelName, labelValue := range target.Labels {
				_label := string(labelName)
				_value := string(labelValue)
				if _, exists := tc.index.ByCustomLabel[_label]; !exists {
					tc.index.ByCustomLabel[_label] = make(map[string][]v1.ActiveTarget)
				}
				tc.index.ByCustomLabel[_label][_value] = append(
					tc.index.ByCustomLabel[_label][_value], target)
			}
		}
	}
}

func (tc *IndexedTargetCache) GetTargetsByJob(jobName string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.getCopy(tc.index.ByJob[jobName])
}

func (tc *IndexedTargetCache) GetTargetsByInstance(instanceName string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.getCopy(tc.index.ByInstance[instanceName])
}

func (tc *IndexedTargetCache) GetTargetsByHealth(health string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.getCopy(tc.index.ByHealth[health])
}

func (tc *IndexedTargetCache) GetTargetsByPool(poolName string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.getCopy(tc.index.ByPool[poolName])
}

func (tc *IndexedTargetCache) GetTargetsByLabel(labelName, labelValue string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	if labelMap, exists := tc.index.ByCustomLabel[labelName]; exists {
		return tc.getCopy(labelMap[labelValue])
	}
	return []v1.ActiveTarget{}
}

// getCopy 返回切片的副本，避免并发问题
func (tc *IndexedTargetCache) getCopy(targets []v1.ActiveTarget) []v1.ActiveTarget {
	if len(targets) == 0 {
		return []v1.ActiveTarget{}
	}
	// 创建副本
	result := make([]v1.ActiveTarget, len(targets))
	copy(result, targets)
	return result
}

// GetTargetsByJobAndHealth 组合查询方法
func (tc *IndexedTargetCache) GetTargetsByJobAndHealth(jobName, health string) []v1.ActiveTarget {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	var result []v1.ActiveTarget
	if jobTargets, exists := tc.index.ByJob[jobName]; exists {
		for _, target := range jobTargets {
			if string(target.Health) == health {
				result = append(result, target)
			}
		}
	}
	return result
}

// GetAllTargetsByPool 获取所有按 pool 分组的 targets
func (tc *IndexedTargetCache) GetAllTargetsByPool() []ActiveTargetByPool {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	var result []ActiveTargetByPool
	for poolName, targets := range tc.allTargets {
		// 创建 targets 副本
		targetsCopy := make([]v1.ActiveTarget, len(targets))
		copy(targetsCopy, targets)

		result = append(result, ActiveTargetByPool{
			ScrapePool: poolName,
			Targets:    targetsCopy,
		})
	}
	return result
}

// 后台刷新
func (tc *IndexedTargetCache) backgroundRefresh() {
	ticker := time.NewTicker(tc.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.mutex.Lock()
			err := tc.refreshCacheUnsafe()
			tc.mutex.Unlock()
			if err != nil {
				fmt.Printf("Background cache refresh failed: %v\n", err)
			}
		case <-tc.stopChan:
			return
		}
	}
}

// Close 关闭缓存，停止后台刷新
func (tc *IndexedTargetCache) Close() {
	select {
	case <-tc.stopChan:
		// 已经关闭
	default:
		close(tc.stopChan)
	}
}

// TargetCache 普通缓存（不带索引）
type TargetCache struct {
	client    *Client
	cache     map[string][]ActiveTargetByPool
	cacheTime time.Time
	mutex     sync.RWMutex
	ttl       time.Duration
	stopChan  chan struct{}
}

// NewTargetCache 创建新的目标缓存实例
func NewTargetCache(client *Client, ttl time.Duration) *TargetCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	tc := &TargetCache{
		client:   client,
		cache:    make(map[string][]ActiveTargetByPool),
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	// 立即刷新缓存，确保缓存有初始数据
	tc.mutex.Lock()
	_ = tc.refreshCacheUnsafe()
	tc.mutex.Unlock()

	go tc.backgroundRefresh()
	return tc
}

// refreshCacheUnsafe 刷新缓存
func (tc *TargetCache) refreshCacheUnsafe() error {
	// 只调用一次接口获取所有活跃目标
	allTargets, err := tc.client.GetActiveTargetsByPool()
	if err != nil {
		return fmt.Errorf("failed to refresh targets cache: %w", err)
	}

	// 在内存中过滤在线和离线目标，而不是再次调用接口
	var onlineTargets []ActiveTargetByPool
	var offlineTargets []ActiveTargetByPool

	for _, pool := range allTargets {
		var onlinePoolTargets []v1.ActiveTarget
		var offlinePoolTargets []v1.ActiveTarget

		for _, target := range pool.Targets {
			if target.Health == TargetHealthGood {
				onlinePoolTargets = append(onlinePoolTargets, target)
			} else if target.Health == TargetHealthBad {
				offlinePoolTargets = append(offlinePoolTargets, target)
			}
		}

		// 只有当池中有目标时才添加
		if len(onlinePoolTargets) > 0 {
			onlineTargets = append(onlineTargets, ActiveTargetByPool{
				ScrapePool: pool.ScrapePool,
				Targets:    onlinePoolTargets,
			})
		}

		if len(offlinePoolTargets) > 0 {
			offlineTargets = append(offlineTargets, ActiveTargetByPool{
				ScrapePool: pool.ScrapePool,
				Targets:    offlinePoolTargets,
			})
		}
	}

	// 更新缓存
	tc.cache["all"] = allTargets
	tc.cache["online"] = onlineTargets
	tc.cache["offline"] = offlineTargets
	tc.cacheTime = time.Now()

	return nil
}

// 后台刷新
func (tc *TargetCache) backgroundRefresh() {
	ticker := time.NewTicker(tc.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.mutex.Lock()
			err := tc.refreshCacheUnsafe()
			tc.mutex.Unlock()
			if err != nil {
				fmt.Printf("Background cache refresh failed: %v\n", err)
			}
		case <-tc.stopChan:
			return
		}
	}
}

// GetTargetsByType 获取指定类型的 targets
func (tc *TargetCache) GetTargetsByType(targetType string) ([]ActiveTargetByPool, error) {
	tc.mutex.RLock()
	cached, exists := tc.cache[targetType]
	tc.mutex.RUnlock()

	if !exists {
		tc.mutex.Lock()
		err := tc.refreshCacheUnsafe()
		if err != nil {
			tc.mutex.Unlock()
			return nil, err
		}
		cached = tc.cache[targetType]
		tc.mutex.Unlock()
	}

	return cached, nil
}

// Close 关闭缓存，停止后台刷新
func (tc *TargetCache) Close() {
	select {
	case <-tc.stopChan:
		// 已经关闭
	default:
		close(tc.stopChan)
	}
}
