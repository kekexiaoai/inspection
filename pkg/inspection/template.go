// Package inspection parses YAML-based巡检模板 and renders queries with
// fully-typed variable substitution, including two-phase resolution so that
// variables may reference other variables without relying on YAML order.
package inspection

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// 包级常量：定义阈值级别及其优先级（数值越小，优先级越高）
const (
	ThresholdLevelCritical = "critical"
	ThresholdLevelWarning  = "warning"
	ThresholdLevelInfo     = "info"
	ThresholdLevelOk       = "ok" // default
)

// 包级变量：阈值级别优先级映射（外部可访问）
var ThresholdLevelPriorities = map[string]int{
	ThresholdLevelCritical: 1,
	ThresholdLevelWarning:  2,
	ThresholdLevelInfo:     3,
	ThresholdLevelOk:       4,
}

// -----------------------------------------------------------------------------
// Data Model (matches YAML spec)
// -----------------------------------------------------------------------------

type Template struct {
	TemplateName   string         `yaml:"template_name" validate:"required"`
	DisplayName    string         `yaml:"display_name" validate:"required"`
	Description    string         `yaml:"description"`
	Version        string         `yaml:"version"`
	CreatedBy      string         `yaml:"created_by"`
	Schedule       Schedule       `yaml:"schedule" validate:"required,dive"`
	TimeRange      string         `yaml:"time_range" validate:"required"`
	Tags           []string       `yaml:"tags"`
	TargetRegistry TargetRegistry `yaml:"target_registry" validate:"required,dive"`
	Indicators     []Indicator    `yaml:"indicators" validate:"required,min=1,dive"`
	ReportLayout   ReportLayout   `yaml:"report_layout" validate:"required,dive"`
}

// SortIndicatorThresholds 解析模板后调用，对阈值按优先级排序
func (tpl *Template) SortIndicatorThresholds() {
	for i := range tpl.Indicators {
		ind := &tpl.Indicators[i]
		sort.Slice(ind.Thresholds, func(i, j int) bool {
			return ThresholdLevelPriorities[ind.Thresholds[i].Level] < ThresholdLevelPriorities[ind.Thresholds[j].Level]
		})
	}
}

type Schedule struct {
	Cron        string `yaml:"cron" validate:"required,cronexpr"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

type TargetRegistry struct {
	Source string                 `yaml:"source" validate:"oneof=metadata"`
	Query  map[string]interface{} `yaml:"query" validate:"required"`
}

type Indicator struct {
	Name        string      `yaml:"name" validate:"required"`
	Description string      `yaml:"description"`
	Source      string      `yaml:"source" validate:"required,oneof=prometheus elasticsearch metadata"`
	Type        string      `yaml:"type"   validate:"required,oneof=point trend alert_list"`
	Query       interface{} `yaml:"query" validate:"required"`
	TimeRange   string      `yaml:"time_range"`
	Resolution  string      `yaml:"resolution"`
	Thresholds  []Threshold `yaml:"thresholds"`
	Required    bool        `yaml:"required"`
	Display     Display     `yaml:"display" validate:"required,dive"`
	Vars        []Variable  `yaml:"vars" validate:"dive"`
}

// -----------------------------------------------------------------------------
// 阈值处理相关方法（集成到 Indicator 中）
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// 示例：外部调用方式
// -----------------------------------------------------------------------------

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
// DetermineStatus 根据指标的阈值配置判断数值对应的状态
func (ind *Indicator) DetermineStatus(value float64) string {
	// 按配置顺序遍历阈值（假设用户已按优先级排序）
	for _, th := range ind.Thresholds {
		if meetsCondition(value, th.Operator, th.Value) {
			return th.Level
		}
	}
	return ThresholdLevelOk // 默认状态
}

// meetsCondition 判断数值是否满足阈值条件
func meetsCondition(value float64, op string, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

type Threshold struct {
	Level    string  `yaml:"level" validate:"required,oneof=critical warning info"` // 状态级别
	Value    float64 `yaml:"value" validate:"required"`                             // 阈值数值
	Operator string  `yaml:"operator" validate:"required,oneof=gt gte lt lte eq"`   // 运算符
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
	Type             string              `yaml:"type" validate:"required,oneof=table line_chart status_light bar_chart heatmap"`
	Unit             string              `yaml:"unit"`
	GroupBy          string              `yaml:"group_by"`
	MissingIndicator bool                `yaml:"missing_indicator"`
	SummaryMode      string              `yaml:"summary_mode" validate:"omitempty,oneof=count_by_status total_count"`
	PageSize         int                 `yaml:"page_size" validate:"omitempty,min=1"`
	Fields           []map[string]string `yaml:"fields"`
}

type ReportLayout struct {
	Sections []Section `yaml:"sections" validate:"required,min=1,dive"`
}

type Section struct {
	Title   string   `yaml:"title" validate:"required"`
	Include []string `yaml:"include" validate:"required,min=1,dive,required"`
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
	if err := validate.Struct(tpl); err != nil {
		return nil, fmt.Errorf("template validation: %w", err)
	}

	// 暂时强制 yaml 填写的时候，thresholds 按优先级排序
	// 后续可以考虑在解析后自动排序
	// 自动排序阈值
	// tpl.SortIndicatorThresholds()
	// 验证每个指标的阈值顺序
	for _, ind := range tpl.Indicators {
		if err := validateThresholdOrder(ind); err != nil {
			return nil, fmt.Errorf("indicator %s: %w", ind.Name, err)
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
			return fmt.Errorf("variable %s must be one of %v", v.Name, v.EnumValues)
		}
	}
	return nil
}

// validateThresholdOrder 验证阈值顺序：禁止相同级别，且必须按优先级排列
func validateThresholdOrder(ind Indicator) error {
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
// Query Rendering (two-phase variable resolution)
// -----------------------------------------------------------------------------

func (tpl *Template) RenderQueryWithVars(ind Indicator, input map[string]string) (string, error) {
	qTemplate, ok := ind.Query.(string)
	if !ok {
		return "", fmt.Errorf("indicator query must be string template")
	}

	// -------- base context with reserved keys --------
	ctxValues := map[string]string{
		"IndicatorTimeRange": ind.TimeRange,
		"GlobalTimeRange":    tpl.TimeRange,
		"IndicatorName":      ind.Name,
	}

	// -------- Pass-1: put non-template raw values into ctxValues --------
	for _, v := range ind.Vars {
		raw := pickRaw(v, input)
		if raw == "" {
			continue
		}
		if !containsTpl(raw) { // constant value, safe to insert now
			if err := validateVarType(v, raw); err != nil {
				return "", err
			}
			ctxValues[v.Name] = raw
		}
	}

	// -------- Pass-2: render template-containing values --------
	for _, v := range ind.Vars {
		if _, ok := ctxValues[v.Name]; ok {
			continue // already resolved in pass-1
		}
		raw := pickRaw(v, input)
		if raw == "" {
			if v.Required {
				return "", fmt.Errorf("missing required variable: %s", v.Name)
			}
			continue
		}
		rendered, err := renderStringTemplate(raw, ctxValues)
		if err != nil {
			return "", err
		}
		if err := validateVarType(v, rendered); err != nil {
			return "", err
		}
		ctxValues[v.Name] = rendered
	}

	// -------- Ensure TimeRange present --------
	if _, ok := ctxValues["TimeRange"]; !ok {
		if ctxValues["IndicatorTimeRange"] != "" {
			ctxValues["TimeRange"] = ctxValues["IndicatorTimeRange"]
		} else {
			ctxValues["TimeRange"] = ctxValues["GlobalTimeRange"]
		}
	}

	// -------- Render final PromQL/DSL --------
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

// Helper to safely execute small templates for variable substitution
func renderStringTemplate(tmplStr string, ctxValues map[string]string) (string, error) {
	// 自定义函数映射
	funcMap := template.FuncMap{}
	tmpl, err := template.New("var").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return tmplStr, err
	}
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, ctxValues)
	return buf.String(), nil
}
