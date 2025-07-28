package prom

import (
	"testing"
)

var (
	// 全局测试客户端
	GlobalTestClient *Client
)

// TestMain 在所有测试开始前初始化全局客户端
func TestMain(m *testing.M) {
	var err error
	GlobalTestClient, err = NewClient("http://10.111.201.1:9090")
	if err != nil {
		// 如果无法连接到 Prometheus，记录但不退出
		GlobalTestClient = nil
	}

	// 运行所有测试
	m.Run()

	// 清理资源
	if GlobalTestClient != nil {
		GlobalTestClient.Close()
	}
}

// RequireTestClient 检查测试客户端是否可用
func RequireTestClient(t *testing.T) *Client {
	if GlobalTestClient == nil {
		t.Skip("Skipping test - cannot connect to Prometheus at http://10.111.201.1:9090")
	}
	return GlobalTestClient
}
