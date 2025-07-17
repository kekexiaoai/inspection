package inspection

import (
	"bytes"
	"fmt"
	"text/template"
)

// varProcessor 定义变量处理阶段的接口
type varProcessor interface {
	// processVars 处理变量列表，返回上下文值和错误
	processVars(values map[string]string) error
}

// globalVarProcessor 处理全局变量
type globalVarProcessor struct {
	tpl   *Template
	input map[string]string
}

// indicatorVarProcessor 处理指标变量
type indicatorVarProcessor struct {
	ind   *Indicator
	input map[string]string
}

// processVars 实现全局变量的处理逻辑
func (p globalVarProcessor) processVars(values map[string]string) error {
	vars := p.tpl.Vars
	return processVarsInTwoPhases(vars, p.input, values, "global")
}

// processVars 实现指标变量的处理逻辑
func (p indicatorVarProcessor) processVars(values map[string]string) error {
	vars := p.ind.Vars
	return processVarsInTwoPhases(vars, p.input, values, "indicator")
}

// processVarsInTwoPhases 公共的两阶段变量处理逻辑
func processVarsInTwoPhases(vars []Variable, input map[string]string, values map[string]string, varType string) error {
	// 第一阶段：处理非模板值
	for _, v := range vars {
		raw := pickRaw(v, input)
		if raw == "" {
			if v.Required {
				return fmt.Errorf("missing required %s variable: %s", varType, v.Name)
			}
			continue
		}
		if !containsTpl(raw) {
			if err := validateVarType(v, raw); err != nil {
				return fmt.Errorf("%s variable %s invalid: %w", varType, v.Name, err)
			}
			values[v.Name] = raw
		}
	}

	// 第二阶段：处理模板值
	for _, v := range vars {
		if _, ok := values[v.Name]; ok {
			continue // 已在第一阶段处理
		}
		raw := pickRaw(v, input)
		if raw == "" {
			if v.Required {
				return fmt.Errorf("missing required %s variable: %s", varType, v.Name)
			}
			continue
		}
		rendered, err := renderStringTemplate(raw, values)
		if err != nil {
			return fmt.Errorf("render %s variable %s: %w", varType, v.Name, err)
		}
		if err := validateVarType(v, rendered); err != nil {
			return fmt.Errorf("%s variable %s invalid after render: %w", varType, v.Name, err)
		}
		values[v.Name] = rendered
	}

	return nil
}

// Helper to safely execute small templates for variable substitution
func renderStringTemplate(tmplStr string, values map[string]string) (string, error) {
	// 自定义函数映射
	funcMap := template.FuncMap{}
	tmpl, err := template.New("var").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return tmplStr, err
	}
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, values)
	return buf.String(), nil
}
