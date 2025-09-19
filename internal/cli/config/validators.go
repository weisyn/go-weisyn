// Package config 提供CLI的配置管理功能 - 验证器
package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// DefaultValidator 默认配置验证器
type DefaultValidator struct {
	constraints map[string]map[string]interface{}
}

// NewDefaultValidator 创建默认验证器
func NewDefaultValidator() ConfigValidator {
	v := &DefaultValidator{
		constraints: make(map[string]map[string]interface{}),
	}

	v.initializeConstraints()
	return v
}

// Validate 验证配置值
func (v *DefaultValidator) Validate(key string, value interface{}) error {
	constraints, exists := v.constraints[key]
	if !exists {
		return nil // 没有约束条件，直接通过
	}

	return v.validateWithConstraints(key, value, constraints)
}

// GetConstraints 获取约束条件
func (v *DefaultValidator) GetConstraints(key string) map[string]interface{} {
	if constraints, exists := v.constraints[key]; exists {
		return constraints
	}
	return nil
}

// validateWithConstraints 根据约束条件验证
func (v *DefaultValidator) validateWithConstraints(key string, value interface{}, constraints map[string]interface{}) error {
	// 类型验证
	if expectedType, exists := constraints["type"]; exists {
		if !v.validateType(value, expectedType.(string)) {
			return fmt.Errorf("配置 %s 类型错误: 期望 %s", key, expectedType)
		}
	}

	// 必填验证
	if required, exists := constraints["required"]; exists && required.(bool) {
		if v.isEmpty(value) {
			return fmt.Errorf("配置 %s 不能为空", key)
		}
	}

	// 范围验证
	if min, exists := constraints["min"]; exists {
		if !v.validateMin(value, min) {
			return fmt.Errorf("配置 %s 值过小: 最小值 %v", key, min)
		}
	}

	if max, exists := constraints["max"]; exists {
		if !v.validateMax(value, max) {
			return fmt.Errorf("配置 %s 值过大: 最大值 %v", key, max)
		}
	}

	// 枚举验证
	if enum, exists := constraints["enum"]; exists {
		if !v.validateEnum(value, enum.([]interface{})) {
			return fmt.Errorf("配置 %s 值不在允许范围内: %v", key, enum)
		}
	}

	// 正则表达式验证
	if pattern, exists := constraints["pattern"]; exists {
		if !v.validatePattern(value, pattern.(string)) {
			return fmt.Errorf("配置 %s 格式不正确: 应匹配 %s", key, pattern)
		}
	}

	// 长度验证
	if minLength, exists := constraints["min_length"]; exists {
		if !v.validateMinLength(value, minLength.(int)) {
			return fmt.Errorf("配置 %s 长度过短: 最小长度 %d", key, minLength)
		}
	}

	if maxLength, exists := constraints["max_length"]; exists {
		if !v.validateMaxLength(value, maxLength.(int)) {
			return fmt.Errorf("配置 %s 长度过长: 最大长度 %d", key, maxLength)
		}
	}

	return nil
}

// validateType 验证类型
func (v *DefaultValidator) validateType(value interface{}, expectedType string) bool {
	actualType := reflect.TypeOf(value).String()

	// 类型映射
	typeMapping := map[string][]string{
		"string": {"string"},
		"int":    {"int", "int64", "int32", "int16", "int8"},
		"float":  {"float64", "float32"},
		"bool":   {"bool"},
		"slice":  {"[]string", "[]interface {}", "[]int", "[]float64"},
		"map":    {"map[string]interface {}", "map[string]string"},
	}

	if allowedTypes, exists := typeMapping[expectedType]; exists {
		for _, allowedType := range allowedTypes {
			if actualType == allowedType {
				return true
			}
		}
		return false
	}

	return actualType == expectedType
}

// isEmpty 检查值是否为空
func (v *DefaultValidator) isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// validateMin 验证最小值
func (v *DefaultValidator) validateMin(value, min interface{}) bool {
	return v.compareNumbers(value, min, ">=")
}

// validateMax 验证最大值
func (v *DefaultValidator) validateMax(value, max interface{}) bool {
	return v.compareNumbers(value, max, "<=")
}

// compareNumbers 比较数字
func (v *DefaultValidator) compareNumbers(value1, value2 interface{}, operator string) bool {
	f1, ok1 := v.toFloat64(value1)
	f2, ok2 := v.toFloat64(value2)

	if !ok1 || !ok2 {
		return true // 无法比较，默认通过
	}

	switch operator {
	case ">=":
		return f1 >= f2
	case "<=":
		return f1 <= f2
	case ">":
		return f1 > f2
	case "<":
		return f1 < f2
	case "==":
		return f1 == f2
	default:
		return true
	}
}

// toFloat64 转换为float64
func (v *DefaultValidator) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// validateEnum 验证枚举值
func (v *DefaultValidator) validateEnum(value interface{}, enum []interface{}) bool {
	for _, item := range enum {
		if reflect.DeepEqual(value, item) {
			return true
		}
	}
	return false
}

// validatePattern 验证正则表达式
func (v *DefaultValidator) validatePattern(value interface{}, pattern string) bool {
	str, ok := value.(string)
	if !ok {
		return false
	}

	// 简化的模式匹配实现
	switch pattern {
	case "email":
		return strings.Contains(str, "@") && strings.Contains(str, ".")
	case "url":
		return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
	case "ipv4":
		parts := strings.Split(str, ".")
		if len(parts) != 4 {
			return false
		}
		for _, part := range parts {
			if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
				return false
			}
		}
		return true
	default:
		return true // 不支持的模式，默认通过
	}
}

// validateMinLength 验证最小长度
func (v *DefaultValidator) validateMinLength(value interface{}, minLength int) bool {
	length := v.getLength(value)
	return length >= minLength
}

// validateMaxLength 验证最大长度
func (v *DefaultValidator) validateMaxLength(value interface{}, maxLength int) bool {
	length := v.getLength(value)
	return length <= maxLength
}

// getLength 获取长度
func (v *DefaultValidator) getLength(value interface{}) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case []interface{}:
		return len(v)
	case []string:
		return len(v)
	case map[string]interface{}:
		return len(v)
	default:
		return 0
	}
}

// initializeConstraints 初始化约束条件
func (v *DefaultValidator) initializeConstraints() {
	// UI相关约束
	v.constraints["ui.theme"] = map[string]interface{}{
		"type":     "string",
		"required": true,
		"enum":     []interface{}{"default", "dark", "light", "blue", "green"},
	}

	v.constraints["ui.language"] = map[string]interface{}{
		"type":     "string",
		"required": true,
		"enum":     []interface{}{"zh-CN", "en-US", "ja-JP"},
	}

	v.constraints["ui.page_size"] = map[string]interface{}{
		"type": "int",
		"min":  float64(5),
		"max":  float64(100),
	}

	// 网络相关约束
	v.constraints["network.api_url"] = map[string]interface{}{
		"type":    "string",
		"pattern": "url",
	}

	v.constraints["network.timeout"] = map[string]interface{}{
		"type": "int",
		"min":  float64(1),
		"max":  float64(300),
	}

	v.constraints["network.max_retries"] = map[string]interface{}{
		"type": "int",
		"min":  float64(0),
		"max":  float64(10),
	}

	// 安全相关约束
	v.constraints["security.session_timeout"] = map[string]interface{}{
		"type": "int",
		"min":  float64(60),
		"max":  float64(7200),
	}

	// 系统相关约束
	v.constraints["system.log_level"] = map[string]interface{}{
		"type": "string",
		"enum": []interface{}{"debug", "info", "warn", "error"},
	}

	v.constraints["system.cleanup_interval"] = map[string]interface{}{
		"type": "int",
		"min":  float64(60),
		"max":  float64(86400),
	}

	// 钱包相关约束
	v.constraints["wallet.auto_lock_timeout"] = map[string]interface{}{
		"type": "int",
		"min":  float64(60),
		"max":  float64(3600),
	}
}
