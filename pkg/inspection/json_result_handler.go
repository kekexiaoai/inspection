package inspection

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/common/model"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

// JSONResultHandler 封装累积结果的状态
type JSONResultHandler struct {
	indicator *Indicator
	result    *IndicatorResult
	// 用于临时存储所有样本（因为处理器会被多次调用，每次处理一个样本）
	samples []*model.Sample
}

// NewJSONResultHandler 创建一个用于转换 Prometheus 查询结果为 JSON 格式的处理器
// 返回值：
//   - *JSONResultHandler：结构体指针，用于在所有数据处理完成后调用 Finalize() 生成最终结果
//   - prom.ResultHandler：处理器函数，用于传递给 prom 包处理查询结果
func NewJSONResultHandler(indicator *Indicator) (*JSONResultHandler, prom.ResultHandler) {
	// 初始化处理器结构体，存储指标元信息和临时数据
	handler := &JSONResultHandler{
		indicator: indicator, // 保存指标元信息（如名称、阈值、显示配置等）
		result: &IndicatorResult{ // 初始化最终要返回的 JSON 结构
			Indicator:   indicator.Name,
			Type:        indicator.Type,
			Unit:        indicator.Display.Unit,
			DisplayType: indicator.Display.Type,
			Summary:     Summary{}, // 用于统计总数量、各状态数量
			Page: PageInfo{
				Size:  indicator.Display.PageSize, // 分页大小（从指标配置中获取）
				Index: 1,                          // 默认第一页
			},
			Values: []ValueItem{}, // 存储具体数据项

			Fields: indicator.Display.Fields, // 显示字段配置
		},
		samples: []*model.Sample{}, // 临时存储所有 *model.Sample 类型的样本（即时查询结果）
	}
	handler.result.StatusMapping = make(map[string]string)

	// 定义实际传给 prom 包的处理器函数（闭包，共享 handler 内部状态）
	resultHandler := func(data any) error {
		switch v := data.(type) {
		case *model.Sample:
			// 处理即时查询（Vector）的单个样本：
			// 1. *model.Sample 代表"某个时间点的单个指标值"（如"node-1 的当前 GPU 使用率"）
			// 2. 每个样本独立对应一个实例/标签组合，但最终需要汇总所有样本才能生成完整结果
			// 3. 必须暂存所有样本，等待全部处理后再统一计算统计信息（如 total/ok/warning 数量）
			handler.samples = append(handler.samples, v)
			return nil

		case *model.SampleStream:
			// 处理范围查询（Matrix）的单个时间序列流：
			// 1. *model.SampleStream 代表"一个时间序列的连续数据点"（如"node-1 过去1小时的 GPU 使用率变化"）
			// 2. 每个 Stream 对应一个独立的时间序列，可单独处理（无需依赖其他 Stream）
			// 3. 可直接从 Stream 中提取所需信息（如最新值、趋势等），无需暂存
			return handler.handleSampleStream(v)

		default:
			return fmt.Errorf("unsupported data type: %T (expected *model.Sample or *model.SampleStream)", data)
		}
	}

	// 同时返回结构体指针（用于后续生成最终结果）和处理器函数（用于 prom 包回调）
	return handler, resultHandler
}

// Finalize 处理完所有样本后，调用此方法生成最终 JSON（需在查询结束后手动调用）
func (h *JSONResultHandler) Finalize() (*IndicatorResult, error) {
	// 处理所有累积的 *model.Sample 样本
	h.processSamples()

	// 处理缺失值
	h.handleMissingValues()

	// 排序并提取高亮项
	h.sortAndExtractHighlights()

	// 应用分页
	// h.applyPagination()

	// // 转换为 JSON
	// indent, err := json.MarshalIndent(h.result, "", "  ")
	// fmt.Println(string(indent))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to marshal indicator result: %v", err)
	// }

	for _, threshold := range h.indicator.Thresholds {
		h.result.StatusMapping[threshold.Level] = threshold.Description
	}

	return h.result, nil
}

// 处理 *model.Sample 样本集合（复用 addValueItem 统一逻辑）
func (h *JSONResultHandler) processSamples() {
	for _, sample := range h.samples {
		// 1. 提取目标名称（优先取 "instance"，其次取 "node"）
		target := ""
		if instance, ok := sample.Metric["instance"]; ok {
			target = string(instance)
		} else if node, ok := sample.Metric["node"]; ok {
			target = string(node)
		} else {
			// 无明确标签时，用所有标签拼接作为名称
			target = sample.Metric.String()
		}

		// 2. 提取数值并计算状态
		value := float64(sample.Value)
		status := h.determineStatus(value)

		// 3. 统一添加数据项并更新统计（复用 addValueItem 方法）
		h.addValueItem(target, &value, false, status)
	}
}

// handleSampleStream 处理 *model.SampleStream 类型的时间序列流
// 注：此方法为示例实现，需根据实际业务需求调整
func (h *JSONResultHandler) handleSampleStream(stream *model.SampleStream) error {
	// 1. 提取时间序列的标签信息
	target := ""
	if instance, ok := stream.Metric["instance"]; ok {
		target = string(instance)
	} else {
		target = stream.Metric.String()
	}

	// 2. 从时间序列中提取关键值
	if len(stream.Values) == 0 {
		// 无数据时标记为缺失
		h.addValueItem(target, nil, true)
		return nil
	}
	latestValue := stream.Values[len(stream.Values)-1]
	currentValue := float64(latestValue.Value)

	// 3. 计算状态并添加到结果集
	status := h.determineStatus(currentValue)
	h.addValueItem(target, &currentValue, false, status)

	return nil
}

// addValueItem 统一添加数据项并更新统计信息
func (h *JSONResultHandler) addValueItem(target string, value *float64, missing bool, status ...string) {
	// 构建 ValueItem
	item := ValueItem{
		Target:  target,
		Value:   value,
		Missing: missing,
	}

	// 非缺失值时设置状态
	if !missing && len(status) > 0 {
		item.Status = status[0]
	}

	// 添加到结果集
	h.result.Values = append(h.result.Values, item)

	// 更新统计信息
	h.updateSummary(item)
}

// updateSummary 根据 ValueItem 更新统计信息
func (h *JSONResultHandler) updateSummary(item ValueItem) {
	h.result.Summary.Total++

	if item.Missing {
		h.result.Summary.Missing++
		return
	}

	// 根据 inspection 包定义的常量更新计数
	switch item.Status {
	case ThresholdLevelCritical:
		h.result.Summary.Critical++
	case ThresholdLevelWarning:
		h.result.Summary.Warning++
	case ThresholdLevelInfo:
		h.result.Summary.Info++
	case ThresholdLevelOk:
		h.result.Summary.Ok++
	default:
		h.result.Summary.Ok++ // 未知状态默认计入 ok
	}
}

// determineStatus 根据指标的阈值配置（Thresholds 数组）判断状态
// 直接复用 Indicator 结构体的 DetermineStatus 方法，避免重复逻辑
func (h *JSONResultHandler) determineStatus(value float64) string {
	// 调用 indicator 自身的 DetermineStatus 方法（包内已实现多阈值判断）
	return h.indicator.DetermineStatus(value)
}

func (h *JSONResultHandler) handleMissingValues() {
	// 处理缺失值逻辑（根据业务需求补充）
}

func (h *JSONResultHandler) applyPagination() {
	pageSize := h.result.Page.Size
	if pageSize <= 0 {
		pageSize = 10
		h.result.Page.Size = pageSize
	}

	total := len(h.result.Values)
	h.result.Page.HasMore = total > pageSize*h.result.Page.Index

	// 截取当前页数据
	start := (h.result.Page.Index - 1) * pageSize
	end := start + pageSize
	if start >= total {
		h.result.Values = []ValueItem{}
		return
	}
	if end > total {
		end = total
	}
	h.result.Values = h.result.Values[start:end]
}

// 提取高亮项，支持多种条件和限制
func (h *JSONResultHandler) sortAndExtractHighlights() {
	config := h.indicator.Display.Highlight
	h.result.Highlight.Enabled = config.Enabled // 严格同步 enabled 状态

	if !config.Enabled {
		h.result.Highlight.Values = []ValueItem{}
		return
	}

	// 1. 按逻辑过滤符合条件的项（支持 and/or）
	logic := config.Logic
	if logic == "" {
		logic = LogicOr // 兜底默认 or（与 ParseTemplateBytes 保持一致）
	}
	filtered := h.filterByConditions(config.Conditions, logic)

	// 2. 解析 limit 并排序
	limit := h.parseHighlightLimit(config.Limit)
	if limit > 0 && len(filtered) > limit {
		// 根据 limit 类型排序（top 降序，bottom 升序）
		if strings.HasPrefix(config.Limit, LimitTop) {
			sort.Slice(filtered, func(i, j int) bool {
				return *filtered[i].Value > *filtered[j].Value
			})
		} else if strings.HasPrefix(config.Limit, LimitBottom) {
			sort.Slice(filtered, func(i, j int) bool {
				return *filtered[i].Value < *filtered[j].Value
			})
		}
		filtered = filtered[:limit] // 截取前 N 项
	}

	// 3. 赋值最终高亮结果
	h.result.Highlight.Values = filtered
}

// 按条件和逻辑关系过滤项（核心优化）
func (h *JSONResultHandler) filterByConditions(conditions []Condition, logic string) []ValueItem {
	var result []ValueItem
	for _, item := range h.result.Values {
		if item.Missing || item.Value == nil {
			continue // 跳过缺失值或无值项
		}

		// 根据 logic 判断满足任一条件（or）还是所有条件（and）
		matches := false
		if logic == LogicAnd {
			matches = h.matchesAllConditions(item, conditions)
		} else { // 默认 LogicOr
			matches = h.matchesAnyCondition(item, conditions)
		}

		if matches {
			result = append(result, item)
		}
	}
	return result
}

// 满足所有条件（and 逻辑）
func (h *JSONResultHandler) matchesAllConditions(item ValueItem, conditions []Condition) bool {
	for _, cond := range conditions {
		if !h.matchesSingleCondition(item, cond) {
			return false
		}
	}
	return true
}

// 满足任一条件（or 逻辑）
func (h *JSONResultHandler) matchesAnyCondition(item ValueItem, conditions []Condition) bool {
	for _, cond := range conditions {
		if h.matchesSingleCondition(item, cond) {
			return true
		}
	}
	return false
}

// 满足单个条件（复用已有的 meetsCondition 函数）
func (h *JSONResultHandler) matchesSingleCondition(item ValueItem, cond Condition) bool {
	// 检查状态级别条件（如 level: critical）
	if cond.Level != "" && item.Status != cond.Level {
		return false
	}
	// 检查数值阈值条件（如 value: 90, operator: gt）
	if cond.Operator != "" {
		if !meetsCondition(*item.Value, cond.Operator, *cond.Value) {
			return false
		}
	}
	return true
}

// 解析高亮 limit 配置
func (h *JSONResultHandler) parseHighlightLimit(limitStr string) int {
	if limitStr == "" || limitStr == LimitAll {
		return -1 // 不限制数量
	}

	// 解析 top_N（如 top_5）
	if strings.HasPrefix(limitStr, LimitTop) {
		n, err := strconv.Atoi(limitStr[4:])
		if err == nil && n > 0 {
			return n
		}
	}

	// 解析 bottom_N（如 bottom_3）
	if strings.HasPrefix(limitStr, LimitBottom) {
		n, err := strconv.Atoi(limitStr[7:])
		if err == nil && n > 0 {
			return n
		}
	}

	// 格式错误时返回 -1（不限制，避免错误截取）
	return -1
}
