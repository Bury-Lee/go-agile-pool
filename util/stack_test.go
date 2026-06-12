package utils

import (
	"strings"
	"testing"
)

func TestStack(t *testing.T) {
	// 测试获取堆栈信息
	stack := Stack(1)
	stackStr := string(stack)

	// 检查是否包含必要信息
	if len(stackStr) == 0 {
		t.Error("Stack返回空内容")
	}

	// 检查是否包含当前测试函数名
	if !strings.Contains(stackStr, "TestStack") {
		t.Errorf("堆栈信息中应包含TestStack函数，实际内容:\n%s", stackStr)
	}

	// 检查格式是否包含文件路径和行号
	if !strings.Contains(stackStr, ".go:") {
		t.Errorf("堆栈信息格式错误，应包含文件路径和行号:\n%s", stackStr)
	}

	t.Logf("堆栈信息:\n%s", stackStr)
}

func TestNestedCall(t *testing.T) {
	// 测试嵌套调用
	result := level1(t)
	resultStr := string(result)

	if len(resultStr) == 0 {
		t.Error("嵌套调用返回空内容")
	}

	// 应该包含多层调用信息
	if !strings.Contains(resultStr, "level1") {
		t.Error("应包含level1函数")
	}
	if !strings.Contains(resultStr, "level2") {
		t.Error("应包含level2函数")
	}
	if !strings.Contains(resultStr, "TestNestedCall") {
		t.Error("应包含TestNestedCall函数")
	}

	t.Logf("嵌套调用堆栈:\n%s", resultStr)
}

func level1(t *testing.T) []byte {
	return level2(t)
}

func level2(t *testing.T) []byte {
	return Stack(0)
}
