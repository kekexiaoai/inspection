package prom

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/common/model"
)

// ResultHandler 定义处理结果的函数类型
type ResultHandler func(t any) error

// ExecuteQuery 执行 Prometheus 即时查询并解析结果
func ExecuteQuery(client *Client, query string, ts time.Time, handler ResultHandler, onEmpty ...func(string)) error {
	// 使用客户端执行查询
	result, warnings, err := client.Query(query, ts)
	if err != nil {
		log.Printf("Query execution error: %v\n", err)
		return fmt.Errorf("query execution failed: %w", err)
	}

	for _, warning := range warnings {
		log.Printf("Query warning: %v\n", warning)
	}

	// 使用 handler 处理结果
	return processResult(result, query, handler, onEmpty...)
}

// ExecuteQueryRange 执行 Prometheus 范围查询并解析结果
func ExecuteQueryRange(client *Client, query string, rangeStart, rangeEnd time.Time, step time.Duration, handler ResultHandler) error {
	// 创建范围对象
	rangeObj := NewRange(rangeStart, rangeEnd, step)

	// 使用客户端执行范围查询
	result, warnings, err := client.QueryRange(query, rangeObj)
	if err != nil {
		log.Printf("QueryRange execution error: %v\n", err)
		return fmt.Errorf("query range execution failed: %w", err)
	}

	for _, warning := range warnings {
		log.Printf("QueryRange warning: %v\n", warning)
	}

	// 使用 handler 处理结果
	return processResult(result, query, handler)
}

// processResult 处理查询结果的公共逻辑
func processResult(result model.Value, query string, handler ResultHandler, onEmpty ...func(string)) error {
	switch v := result.(type) {
	case model.Vector:
		if len(v) == 0 {
			// 优先使用自定义回调，否则用默认提示
			if len(onEmpty) > 0 && onEmpty[0] != nil {
				onEmpty[0](query)
			} else {
				fmt.Printf("No data found for query: %s\n", query)
			}
			return nil
		}
		for _, sample := range v {
			if err := handler(sample); err != nil {
				return err
			}
		}
	case model.Matrix:
		if len(v) == 0 {
			log.Printf("No data found for query: %s\n", query)
			return nil
		}
		for _, stream := range v {
			if err := handler(stream); err != nil {
				return err
			}
		}
	default:
		log.Printf("Unexpected result type: %T\n", result)
		return fmt.Errorf("unexpected result type: %T", result)
	}
	return nil
}

// DefaultVectorHandler 提供默认的 Vector 结果处理
func DefaultVectorHandler(t any) error {
	sample, ok := t.(*model.Sample)
	if !ok {
		return fmt.Errorf("expected model.Sample, got %T", t)
	}

	labels := make(map[string]string)
	for k, v := range sample.Metric {
		labels[string(k)] = string(v)
	}
	fmt.Printf("\n\nLabels: %v\n", labels)
	value := float64(sample.Value)
	fmt.Printf("Value: %.2f\n", value)

	return nil
}

// DefaultMatrixHandler 提供默认的 Matrix 结果处理
func DefaultMatrixHandler(t any) error {
	stream, ok := t.(*model.SampleStream)
	if !ok {
		return fmt.Errorf("expected model.SampleStream, got %T", t)
	}

	labels := make(map[string]string)
	for k, v := range stream.Metric {
		labels[string(k)] = string(v)
	}
	fmt.Printf("\n\nLabels: %v\n", labels)
	for _, value := range stream.Values {
		fmt.Printf("Time: %v, Value: %.2f\n", value.Timestamp.Time(), float64(value.Value))
	}

	return nil
}
