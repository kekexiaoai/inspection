package inspection

// 阈值运算符常量
const (
	OpGt  = "gt"  // >
	OpGte = "gte" // >=
	OpLt  = "lt"  // <
	OpLte = "lte" // <=
	OpEq  = "eq"  // ==
)

// 高亮逻辑常量
const (
	LogicAnd = "and" // 满足所有条件
	LogicOr  = "or"  // 满足任一条件
)

// 高亮Limit类型常量
const (
	LimitAll    = "all"    // 不限制
	LimitTop    = "top"    // 前缀：如 top_5
	LimitBottom = "bottom" // 前缀：如 bottom_3
)

// 显示类型常量
const (
	DisplayTable       = "table"
	DisplayLineChart   = "line_chart"
	DisplayStatusLight = "status_light"
	DisplayBarChart    = "bar_chart"
	DisplayHeatmap     = "heatmap"
)

// 数据源类型常量
const (
	SourcePrometheus    = "prometheus"
	SourceElasticsearch = "elasticsearch"
	SourceMetadata      = "metadata"
)

// 指标类型常量
const (
	IndicatorTypePoint     = "point"
	IndicatorTypeTrend     = "trend"
	IndicatorTypeAlertList = "alert_list"
)
