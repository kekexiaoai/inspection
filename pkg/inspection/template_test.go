package inspection

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kekexiaoai/inspection/pkg/prom"
)

var globalClient *prom.Client
var indexedPromTargetCache *prom.IndexedTargetCache
var dcID = "01fd9896-3e25-4d68-b7ce-c20ab7ee13ca"

func TestMain(m *testing.M) {
	var err error
	// globalClient, err = prom.NewClient("http://10.120.1.6:9090", prom.WithTimeout(10*time.Second))
	globalClient, err = prom.NewClient("http://10.111.201.1:9090", prom.WithTimeout(10*time.Second))
	if err != nil {
		fmt.Printf("Error creating global client: %v\n", err)
		os.Exit(1)
	}
	defer globalClient.Close()

	indexedPromTargetCache = prom.NewIndexedTargetCache(globalClient, 60*time.Second)
	defer indexedPromTargetCache.Close()

	// 过滤标签，组装 map
	// targetMap = make(map[string]v1.ActiveTarget)

	// targets := indexedPromTargetCache.GetTargetsByPool("node_exporter")
	// fmt.Printf("targets: %v", targets)

	// for _, target := range targets {
	// 	if target.Labels["instance"] == "" {
	// 		continue
	// 	}
	// 	instance := string(target.Labels["instance"])
	// 	// 过滤掉 data_center_id标签不符合给定值
	// 	if string(target.Labels["data_center_id"]) != dcID {
	// 		continue
	// 	}
	// 	_target := strings.Split(instance, ":")[0]
	// 	targetMap[_target] = target
	// }

	code := m.Run()
	os.Exit(code)
}

// 测试专用的 Vector 结果处理器（替代原 ExecuteQuery 的输出逻辑）
func testVectorHandler(t *testing.T, emptyMsg ...string) prom.ResultHandler {
	return func(data any) error {
		sample, ok := data.(*model.Sample)
		if !ok {
			return fmt.Errorf("expected *model.Sample, got %T", data)
		}

		// 提取标签并打印
		labels := make(map[string]string)
		for k, v := range sample.Metric {
			labels[string(k)] = string(v)
		}
		t.Logf("Labels: %v", labels)

		// 打印值
		value := float64(sample.Value)
		t.Logf("Value: %.2f", value)
		return nil
	}
}

// 测试专用的 Matrix 结果处理器（替代原 ExecuteQueryRange 的输出逻辑）
func testMatrixHandler(t *testing.T) prom.ResultHandler {
	return func(data any) error {
		stream, ok := data.(*model.SampleStream)
		if !ok {
			return fmt.Errorf("expected *model.SampleStream, got %T", data)
		}

		// 提取标签并打印
		labels := make(map[string]string)
		for k, v := range stream.Metric {
			labels[string(k)] = string(v)
		}
		t.Logf("Labels: %v", labels)

		// 打印时间序列数据
		for _, val := range stream.Values {
			t.Logf("Time: %v, Value: %.2f", val.Timestamp.Time(), float64(val.Value))
		}
		return nil
	}
}

// 优化后的 ExecuteQuery：复用 prom 包的 ExecuteQuery 并注入测试处理器
func ExecuteQuery(t *testing.T, query string, ts time.Time, emptyMsg ...string) {
	// 定义空结果回调（使用自定义提示）
	var onEmpty func(string)
	if len(emptyMsg) > 0 {
		onEmpty = func(q string) {
			t.Logf(emptyMsg[0], q)
		}
	}

	handler := func(data any) error {
		return testVectorHandler(t)(data)
	}

	// 调用 prom 包的 ExecuteQuery，传入自定义空结果回调
	err := prom.ExecuteQuery(globalClient, query, ts, handler, onEmpty)
	if err != nil {
		t.Fatalf("Query execution error: %v", err)
	}
}

// 优化后的 ExecuteQueryRange：复用 prom 包的 ExecuteQueryRange 并注入测试处理器
func ExecuteQueryRange(t *testing.T, query string, rangeStart, rangeEnd time.Time, step time.Duration) {
	// 调用 prom 包的 ExecuteQueryRange，使用测试专用的 Matrix 处理器
	err := prom.ExecuteQueryRange(
		globalClient,
		query,
		rangeStart,
		rangeEnd,
		step,
		testMatrixHandler(t),
	)
	if err != nil {
		t.Fatalf("QueryRange execution error: %v", err)
	}
}

// 以下测试函数保持不变（复用优化后的 ExecuteQuery/ExecuteQueryRange）
func TestRenderGPUUsage(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[0]

	// 渲染查询语句
	queryVars := map[string]string{
		"ClusterRegex": `10\\.120\\.[0-9]+\\.[0-9]+`,
	}
	query, err := tpl.RenderQueryWithVars(ind, queryVars)
	if err != nil {
		t.Fatal(err)
	}

	// 输出渲染的查询语句
	fmt.Println("Rendered Query:\n", query)

	now := time.Now()
	ExecuteQuery(t, query, now)
}

// 以下测试函数保持不变（复用优化后的 ExecuteQuery/ExecuteQueryRange）
func TestRenderGPUUsage_for_EmptyStr_does_not_exists(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[0]

	// 渲染查询语句
	queryVars := map[string]string{
		"ClusterRegex": `100\\.120\\.[0-9]+\\.[0-9]+`,
	}
	query, err := tpl.RenderQueryWithVars(ind, queryVars)
	if err != nil {
		t.Fatal(err)
	}

	// 输出渲染的查询语句
	fmt.Println("Rendered Query:\n", query)

	now := time.Now()
	ExecuteQuery(t, query, now)
}

// 以下测试函数保持不变（复用优化后的 ExecuteQuery/ExecuteQueryRange）
func TestRenderGPUUsage_for_EmptyStr_exists(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[0]

	// 渲染查询语句
	queryVars := map[string]string{
		"ClusterRegex": `100\\.120\\.[0-9]+\\.[0-9]+`,
	}
	query, err := tpl.RenderQueryWithVars(ind, queryVars)
	if err != nil {
		t.Fatal(err)
	}

	// 输出渲染的查询语句
	fmt.Println("Rendered Query:\n", query)

	now := time.Now()
	ExecuteQuery(t, query, now, "[test empty string]: %s")
}

func TestRenderGPUUsage_with_TimeRange(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[0]

	query, err := tpl.RenderQueryWithVars(ind, map[string]string{
		"ClusterRegex": `10\\.120\\.[0-9]+\\.[0-9]+`,
		"TimeRange":    "1d",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 输出渲染的查询语句
	fmt.Println("Rendered Query:\n", query)

	now := time.Now()
	ExecuteQuery(t, query, now)
}

func TestAnotherGPUQuery(t *testing.T) {
	query := `avg by (Hostname) (rate(DCGM_FI_DEV_GPU_UTIL{Hostname=~"worker-[0-9]+"}[5m]))`

	now := time.Now()
	ExecuteQuery(t, query, now)
}

func TestGPUUsageRange(t *testing.T) {
	// 示例：使用范围查询
	query := `rate(DCGM_FI_DEV_GPU_UTIL{Hostname=~"worker-[0-9]+"}[5m])`
	now := time.Now()
	rangeStart := now.Add(-1 * time.Hour) // 过去 1 小时
	rangeEnd := now
	step := 5 * time.Minute // 每 5 分钟一个数据点

	// 输出渲染的查询语句
	fmt.Println("Rendered Range Query:\n", query)

	// 使用范围查询函数
	ExecuteQueryRange(t, query, rangeStart, rangeEnd, step)
}

func TestRenderGPUUsageWithJSON_result(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[0]

	query, err := tpl.RenderQueryWithVars(ind, map[string]string{
		"ClusterRegex": `10\\.120\\.[0-9]+\\.[0-9]+`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建处理器：同时获取结构体指针和处理器函数
	jsonHandler, resultHandler := NewJSONResultHandler(ind, indexedPromTargetCache)

	// 执行查询：传递 resultHandler 给 prom 包
	now := time.Now()
	if err := prom.ExecuteQuery(globalClient, query, now, resultHandler); err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// 生成最终 JSON：通过 jsonHandler 结构体指针调用 Finalize()
	result, err := jsonHandler.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// 输出结果
	fmt.Println("Final JSON:\n", result)
}

func TestRenderGPUUsageWithJSON_result_2(t *testing.T) {
	tpl, err := ParseTemplateFile("template/template-indicator-gpu-prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ind := tpl.Indicators[1]

	query, err := tpl.RenderQueryWithVars(ind, map[string]string{
		// "ClusterRegex": `10\\.120\\.[0-9]+\\.[0-9]+`,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Rendered Query:\n", query)

	// 创建处理器：同时获取结构体指针和处理器函数
	jsonHandler, resultHandler := NewJSONResultHandler(ind, indexedPromTargetCache)

	// 执行查询：传递 resultHandler 给 prom 包
	now := time.Now()
	if err := prom.ExecuteQuery(globalClient, query, now, resultHandler); err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// 生成最终 JSON：通过 jsonHandler 结构体指针调用 Finalize()
	result, err := jsonHandler.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// 输出结果
	fmt.Println("Final JSON:\n", result)
}

func TestRenderTemplate(t *testing.T) {
	tpl, err := ParseTemplateFile("template/gpu/gpu-node.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tpl.DataCenter.ID = dcID
	now := time.Now()
	result := &Report{}
	result.Template.Name = tpl.Name
	result.Template.DisplayName = tpl.DisplayName
	result.Template.ExecutedAt = now
	result.Template.ExecutedBy = "admin"
	result.Sections = tpl.ReportLayout.Sections

	for _, ind := range tpl.Indicators {
		if !*ind.Enabled {
			continue
		}
		query, err := tpl.RenderQueryWithVars(ind, map[string]string{
			// "ClusterRegex": `10\\.120\\.[0-9]+\\.[0-9]+`,
		})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("Rendered Query:\n", query)

		// 创建处理器：同时获取结构体指针和处理器函数
		jsonHandler, resultHandler := NewJSONResultHandler(ind, indexedPromTargetCache)
		if err := prom.ExecuteQuery(globalClient, query, now, resultHandler); err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		indResult, err := jsonHandler.Finalize()
		if err != nil {
			t.Fatalf("Finalize failed: %v", err)
		}
		result.Results = append(result.Results, indResult)

		//
		so := &SummaryOverview{
			Indicator: ind.Name,
			Unit:      indResult.Unit,
			Total:     indResult.Summary.Total,
			Ok:        indResult.Summary.Ok,
			Info:      indResult.Summary.Info,
			Warning:   indResult.Summary.Warning,
			Critical:  indResult.Summary.Critical,
			Missing:   indResult.Summary.Missing,
		}
		result.SummaryOverviews = append(result.SummaryOverviews, so)

		// marshal, err := json.MarshalIndent(indResult, "", "  ")

		// fmt.Printf("Final JSON: %#v\n%s\n%v\n", indResult, marshal, err)
	}

	marshal, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("result: \n", string(marshal))
}
