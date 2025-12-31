package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// adapter.go 工具函数测试
// ============================================================================
//
// 🎯 **测试目的**：发现 findSubstring 和 contains 函数的缺陷和BUG
//
// ============================================================================

// TestFindSubstring 测试 findSubstring 函数的所有边界情况
func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{
			name:     "空子串应该返回0",
			s:        "hello",
			substr:   "",
			expected: 0,
		},
		{
			name:     "子串长度大于主串应该返回-1",
			s:        "hello",
			substr:   "world",
			expected: -1,
		},
		{
			name:     "找到子串在开头",
			s:        "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "找到子串在中间",
			s:        "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "找到子串在结尾",
			s:        "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "多个匹配应该返回第一个",
			s:        "hello hello",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "部分匹配不应该返回",
			s:        "hello",
			substr:   "helloworld",
			expected: -1,
		},
		{
			name:     "单字符匹配",
			s:        "hello",
			substr:   "e",
			expected: 1,
		},
		{
			name:     "单字符不匹配",
			s:        "hello",
			substr:   "x",
			expected: -1,
		},
		{
			name:     "空主串",
			s:        "",
			substr:   "hello",
			expected: -1,
		},
		{
			name:     "空主串和空子串",
			s:        "",
			substr:   "",
			expected: 0,
		},
		{
			name:     "相同字符串",
			s:        "hello",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "子串在中间但部分匹配",
			s:        "hello world",
			substr:   "worl",
			expected: 6,
		},
		{
			name:     "子串不匹配",
			s:        "hello world",
			substr:   "xyz",
			expected: -1,
		},
		{
			name:     "Unicode字符",
			s:        "你好世界",
			substr:   "世界",
			expected: 6, // 每个中文字符3字节
		},
		{
			name:     "Unicode字符不匹配",
			s:        "你好世界",
			substr:   "xyz",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result, "findSubstring(%q, %q) = %d, expected %d", tt.s, tt.substr, result, tt.expected)
		})
	}
}

// TestContains 测试 contains 函数的所有边界情况
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "包含子串",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "不包含子串",
			s:        "hello world",
			substr:   "xyz",
			expected: false,
		},
		{
			name:     "空子串应该返回true",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "子串长度大于主串应该返回false",
			s:        "hello",
			substr:   "world",
			expected: false,
		},
		{
			name:     "相同字符串应该返回true",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "空主串和空子串应该返回true",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "空主串和非空子串应该返回false",
			s:        "",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "子串在开头",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "子串在结尾",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "子串在中间",
			s:        "hello world",
			substr:   "lo wo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result, "contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
		})
	}
}

