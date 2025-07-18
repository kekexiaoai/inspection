package inspection

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

const (
	ThresholdLevelCritical = "critical"
	ThresholdLevelWarning  = "warning"
	ThresholdLevelInfo     = "info"
	ThresholdLevelOk       = "ok" // default
)

var ThresholdLevelPriorities = map[string]int{
	ThresholdLevelCritical: 1,
	ThresholdLevelWarning:  2,
	ThresholdLevelInfo:     3,
	ThresholdLevelOk:       4,
}

type Template struct {
	TemplateName   string         `yaml:"template_name" validate:"required"`
	DisplayName    string         `yaml:"display_name" validate:"required"`
	Description    string         `yaml:"description"`
	Version        string         `yaml:"version"`
	CreatedBy      string         `yaml:"created_by"`
	Schedule       Schedule       `yaml:"schedule" validate:"required"`
	TimeRange      string         `yaml:"time_range" validate:"required"`
	Tags           []string       `yaml:"tags"`
	TargetRegistry TargetRegistry `yaml:"target_registry" validate:"required"`
	Vars           []Variable     `yaml:"vars" validate:"dive"` // 全局变量
	Indicators     []*Indicator   `yaml:"indicators" validate:"required,min=1,dive"`
	ReportLayout   ReportLayout   `yaml:"report_layout" validate:"required"`
}

// SortIndicatorThresholds 解析模板后调用，对阈值按优先级排序
func (tpl *Template) SortIndicatorThresholds() {
	for i := range tpl.Indicators {
		ind := &tpl.Indicators[i]
		sort.Slice((*ind).Thresholds, func(i, j int) bool {
			return ThresholdLevelPriorities[(*ind).Thresholds[i].Level] < ThresholdLevelPriorities[(*ind).Thresholds[j].Level]
		})
	}
}

type Schedule struct {
	Cron        string `yaml:"cron" validate:"required,cronexpr"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

type TargetRegistry struct {
	Source string         `yaml:"source" validate:"oneof=metadata"`
	Query  map[string]any `yaml:"query" validate:"required"`
}

type Indicator struct {
	Name        string       `yaml:"name" validate:"required"`
	Description string       `yaml:"description"`
	Source      string       `yaml:"source" validate:"required,oneof=prometheus elasticsearch metadata"`
	Type        string       `yaml:"type"   validate:"required,oneof=point range trend alert_list"`
	Query       any          `yaml:"query" validate:"required"`
	TimeRange   string       `yaml:"time_range"`
	Resolution  string       `yaml:"resolution"`
	Thresholds  []*Threshold `yaml:"thresholds" validate:"dive"`
	Required    bool         `yaml:"required"`
	Display     Display      `yaml:"display" validate:"required"`
	Vars        []Variable   `yaml:"vars" validate:"dive"`
}

/*
// 外部代码可以这样使用：
func ExampleIndicator_DetermineStatus() {
	// 解析模板
	tpl, _ := ParseTemplateFile("template.yaml")
	// 获取某个指标
	indicator := tpl.Indicators[0]

	// 判断数值对应的状态
	value := 95.0
	status := indicator.DetermineStatus(value)
	fmt.Printf("数值 %.2f 的状态: %s\n", value, status) // 输出：数值 95.00 的状态: warning（假设阈值为 gt:90, level:warning）
}
*/

// DetermineStatus 根据指标的阈值配置判断数值对应的状态
// 外部可直接调用：indicator.DetermineStatus(value)
func (ind *Indicator) DetermineStatus(value float64) string {
	// 按配置顺序遍历阈值（假设用户已按优先级排序）
	for _, th := range ind.Thresholds {
		if meetsCondition(value, th.Operator, *th.Value) {
			return th.Level
		}
	}
	return ThresholdLevelOk // 默认状态
}

// meetsCondition 判断数值是否满足阈值条件
func meetsCondition(value float64, op string, threshold float64) bool {
	switch op {
	case OpGt:
		return value > threshold
	case OpGte:
		return value >= threshold
	case OpLt:
		return value < threshold
	case OpLte:
		return value <= threshold
	case OpEq:
		return value == threshold
	default:
		return false
	}
}

type Threshold struct {
	Level       string   `yaml:"level" validate:"required,oneof=critical warning info"` // 状态级别
	Value       *float64 `yaml:"value" validate:"required"`                             // 阈值数值
	Operator    string   `yaml:"operator" validate:"required,oneof=gt gte lt lte eq"`   // 运算符
	Description string   `yaml:"description" validate:"required"`                       // 状态描述
}

type Variable struct {
	Name         string   `yaml:"name" validate:"required"`
	Type         string   `yaml:"type" validate:"required,oneof=string number boolean enum"`
	Required     bool     `yaml:"required"`
	Value        string   `yaml:"value"`
	DefaultValue string   `yaml:"default_value"`
	Description  string   `yaml:"description"`
	EnumValues   []string `yaml:"enum_values"`
}

type Display struct {
	Type             string           `yaml:"type" validate:"required,oneof=table line_chart status_light bar_chart heatmap"`
	Unit             string           `yaml:"unit"`
	GroupBy          string           `yaml:"group_by"`
	MissingIndicator bool             `yaml:"missing_indicator"`
	SummaryMode      string           `yaml:"summary_mode" validate:"omitempty,oneof=count_by_status total_count"`
	PageSize         int              `yaml:"page_size" validate:"omitempty,min=1"`
	Fields           []map[string]any `yaml:"fields"`
	Highlight        HighlightConfig  `yaml:"highlight"`
}

type HighlightConfig struct {
	Enabled    bool        `yaml:"enabled"`
	Limit      string      `yaml:"limit"` // 取值："all", "top_n", "bottom_n"
	Logic      string      `yaml:"logic" validate:"omitempty,oneof=and or" default:"or"`
	Conditions []Condition `yaml:"conditions"` // 支持多个条件
}

// 预编译 Limit 格式验证正则表达式
var validLimitPattern = regexp.MustCompile(`^(all|top_\d+|bottom_\d+)$`)

// Validate 验证 HighlightConfig 结构
func (h *HighlightConfig) Validate() error {
	// 若未启用，无需验证其他字段
	if !h.Enabled {
		return nil
	}

	// 验证 Limit 格式
	if h.Limit != "" && !validLimitPattern.MatchString(h.Limit) {
		return fmt.Errorf("invalid highlight limit format: %s", h.Limit)
	}

	// 验证 Conditions
	if len(h.Conditions) == 0 {
		return fmt.Errorf("highlight is enabled but no conditions specified")
	}

	for i, cond := range h.Conditions {
		// 验证 Level
		if cond.Level != "" {
			if _, ok := ThresholdLevelPriorities[cond.Level]; !ok {
				return fmt.Errorf("invalid level in condition %d: %s", i, cond.Level)
			}
		}

		// 验证 Operator
		if cond.Operator != "" {
			valid := false
			for _, op := range []string{"gt", "gte", "lt", "lte", "eq"} {
				if cond.Operator == op {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid operator in condition %d: %s", i, cond.Operator)
			}
		}

		// 验证 value 和 Operator 必须同时存在或同时不存在
		if (cond.Value != nil) != (cond.Operator != "") {
			return fmt.Errorf("value and operator must be specified together in condition %d", i)
		}
	}

	return nil
}

type Condition struct {
	Level    string   `yaml:"level"`    // 支持 critical/warning/info/ok
	Value    *float64 `yaml:"value"`    // 可选：数值阈值
	Operator string   `yaml:"operator"` // 可选：gt/gte/lt/lte/eq
}
type ReportLayout struct {
	Sections []*Section `yaml:"sections" json:"sections" validate:"required,min=1,dive"`
}

type Section struct {
	Title      string   `yaml:"title" json:"title" validate:"required"`
	Indicators []string `yaml:"Indicators" json:"indicators" validate:"required,min=1,dive,required"`
}

// -----------------------------------------------------------------------------
// Initialisation & Validation helpers
// -----------------------------------------------------------------------------

var validate *validator.Validate

func init() {
	validate = validator.New()
	_ = validate.RegisterValidation("cronexpr", func(fl validator.FieldLevel) bool {
		_, err := cron.ParseStandard(fl.Field().String())
		return err == nil
	})
}

// -----------------------------------------------------------------------------
// Parsing helpers
// -----------------------------------------------------------------------------

func ParseTemplateFile(path string) (*Template, error) {
	byts, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}
	return ParseTemplateBytes(byts)
}

func ParseTemplateBytes(data []byte) (*Template, error) {
	var tpl Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	// 补全 HighlightConfig.Logic 的默认值
	for _, ind := range tpl.Indicators {
		if ind.Display.Highlight.Logic == "" {
			ind.Display.Highlight.Logic = "or" // 默认 or 逻辑
		}
	}

	if err := validate.Struct(tpl); err != nil {
		return nil, fmt.Errorf("template validation: %w", err)
	}

	// 暂时强制 yaml 填写的时候，thresholds 按优先级排序
	// 后续可以考虑在解析后自动排序
	// 自动排序阈值
	// tpl.SortIndicatorThresholds()
	// 验证每个指标的阈值顺序和高亮配置
	for _, ind := range tpl.Indicators {
		if err := validateThresholdOrder(ind); err != nil {
			return nil, fmt.Errorf("indicator %s: %w", ind.Name, err)
		}
		if err := ind.Display.Highlight.Validate(); err != nil {
			return nil, fmt.Errorf("indicator %s highlight invalid: %w", ind.Name, err)
		}
	}

	return &tpl, nil
}

// -----------------------------------------------------------------------------
// Variable helpers
// -----------------------------------------------------------------------------

func containsTpl(s string) bool { return strings.Contains(s, "{{") && strings.Contains(s, "}}") }

// value picking with priority: input -> Variable.Value -> Variable.DefaultValue
func pickRaw(v Variable, input map[string]string) string {
	if val, ok := input[v.Name]; ok {
		return val
	}
	if v.Value != "" {
		return v.Value
	}
	return v.DefaultValue
}

func validateVarType(v Variable, val string) error {
	switch v.Type {
	case "number":
		if _, err := strconv.ParseFloat(val, 64); err != nil {
			return fmt.Errorf("variable %s must be number", v.Name)
		}
	case "boolean":
		if val != "true" && val != "false" {
			return fmt.Errorf("variable %s must be true/false", v.Name)
		}
	case "enum":
		allowed := false
		for _, e := range v.EnumValues {
			if val == e {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("variable %s must be one of %v, got %s", v.Name, v.EnumValues, val)
		}
	}
	return nil
}

// validateThresholdOrder 验证阈值顺序：禁止相同级别，且必须按优先级排列
func validateThresholdOrder(ind *Indicator) error {
	// 记录已出现的级别，确保唯一
	seenLevels := make(map[string]bool)
	lastPriority := 0 // 初始化为最低优先级

	for _, th := range ind.Thresholds {
		// 1. 检查级别是否合法
		priority, ok := ThresholdLevelPriorities[th.Level]
		if !ok {
			return fmt.Errorf("无效的阈值级别: %s（允许的值：%v）",
				th.Level, getValidLevels())
		}

		// 2. 检查级别是否重复
		if seenLevels[th.Level] {
			return fmt.Errorf("阈值级别重复: %s 不能出现多次", th.Level)
		}
		seenLevels[th.Level] = true

		// 3. 检查顺序是否正确（低级不能在高级之前）
		if priority < lastPriority {
			return fmt.Errorf("阈值顺序错误: %s（优先级 %d）不应在 %s（优先级 %d）之后",
				th.Level, priority, getLevelByPriority(lastPriority), lastPriority)
		}

		lastPriority = priority
	}
	return nil
}

// 辅助函数：获取允许的级别列表（用于错误提示）
func getValidLevels() []string {
	levels := make([]string, 0, len(ThresholdLevelPriorities))
	for level := range ThresholdLevelPriorities {
		levels = append(levels, level)
	}
	return levels
}

// 辅助函数：通过优先级获取级别名称（用于错误提示）
func getLevelByPriority(priority int) string {
	for level, p := range ThresholdLevelPriorities {
		if p == priority {
			return level
		}
	}
	return "unknown"
}

// -----------------------------------------------------------------------------
// Query Rendering
// -----------------------------------------------------------------------------

func (tpl *Template) RenderQueryWithVars(ind *Indicator, input map[string]string) (string, error) {
	qTemplate, ok := ind.Query.(string)
	if !ok {
		return "", fmt.Errorf("indicator query must be string template")
	}

	// 初始化基础上下文
	values := tpl.initBaseContext(ind)

	// 创建处理器链
	processors := []varProcessor{
		globalVarProcessor{tpl: tpl, input: input},
		indicatorVarProcessor{ind: ind, input: input},
	}

	// 依次执行处理器
	for _, p := range processors {
		if err := p.processVars(values); err != nil {
			return "", err
		}
	}

	// 确保 TimeRange 存在
	if _, ok := values["TimeRange"]; !ok {
		if values["IndicatorTimeRange"] != "" {
			values["TimeRange"] = values["IndicatorTimeRange"]
		} else {
			values["TimeRange"] = values["GlobalTimeRange"]
		}
	}

	// 渲染最终查询
	return tpl.renderQuery(qTemplate, values)
}

// initBaseContext 初始化基础上下文
func (tpl *Template) initBaseContext(ind *Indicator) map[string]string {
	return map[string]string{
		"IndicatorTimeRange": ind.TimeRange,
		"GlobalTimeRange":    tpl.TimeRange,
		"IndicatorName":      ind.Name,
	}
}

// renderQuery 渲染最终查询
func (tpl *Template) renderQuery(qTemplate string, ctxValues map[string]string) (string, error) {
	t, err := template.New("q").Parse(qTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctxValues); err != nil {
		return "", err
	}
	return buf.String(), nil
}
