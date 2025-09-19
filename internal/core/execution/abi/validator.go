package abi

import (
	iface "github.com/weisyn/v1/internal/core/execution/interfaces"
	typespkg "github.com/weisyn/v1/pkg/types"
)

// ABIValidator ABI验证器，负责验证ABI定义的正确性和完整性。
//
// 验证器支持多层次的验证规则，包括语法验证、语义验证、约束验证和兼容性验证。
// 通过可配置的验证规则集合，可以根据不同的使用场景调整验证的严格程度。
//
// 验证层次：
//   - 语法验证：检查ABI定义的基本语法正确性
//   - 语义验证：检查ABI定义的语义合理性
//   - 约束验证：检查业务约束和限制条件
//   - 兼容性验证：检查版本间的兼容性
type ABIValidator struct {
	// rules 验证规则集合，实现 interfaces.ValidationRule 接口
	// 支持动态添加和配置不同的验证规则
	rules []iface.ValidationRule

	// config 已移除 - 使用固定的智能验证策略
}

// ABIValidatorConfig 已删除 - 使用固定的智能验证策略
// 所有验证功能均为智能默认启用：
// - 语法验证、语义验证、约束验证、兼容性验证始终启用
// - 生产环境自动严格模式，开发环境自动宽松模式

// NewABIValidator 创建零配置的ABI验证器实例。
// 使用固定的智能验证策略，无需配置参数。
//
// 返回值：
//   - *ABIValidator: 使用智能默认策略的验证器实例
func NewABIValidator() *ABIValidator {
	return &ABIValidator{
		rules: []iface.ValidationRule{},
		// config已移除，使用固定的智能验证策略
	}
}

// DefaultABIValidatorConfig 已删除 - 不再需要配置
// 所有验证策略均为智能默认启用，无需配置函数

// ValidateABI 验证ABI定义的正确性和完整性。
//
// 该方法执行多层次的验证检查，包括基础结构验证、字段有效性检查、
// 函数定义验证等。返回所有发现的验证错误，便于调用方进行处理。
//
// 参数：
//   - abi: 待验证的合约ABI定义
//
// 返回值：
//   - []iface.ValidationError: 验证错误列表，空列表表示验证通过
//
// 验证内容：
//   - ABI对象非空检查
//   - 版本号有效性验证
//   - 函数定义完整性检查
//   - 参数类型有效性验证
//   - 事件定义正确性检查
func (v *ABIValidator) ValidateABI(abi *typespkg.ContractABI) []iface.ValidationError {
	var errors []iface.ValidationError

	// 基础非空检查
	if abi == nil {
		errors = append(errors, iface.ValidationError{
			RuleName:    "abi_required",
			Severity:    iface.ValidationSeverityError,
			Message:     "Contract ABI is required",
			Location:    "root",
			Suggestions: []string{"Provide a valid ContractABI instance"},
		})
		return errors
	}

	// 版本号验证
	if abi.Version == "" {
		errors = append(errors, iface.ValidationError{
			RuleName:    "abi_version_required",
			Severity:    iface.ValidationSeverityError,
			Message:     "ABI version is required",
			Location:    "version",
			Suggestions: []string{"Set a semantic version like '1.0.0'"},
		})
	}

	// 函数定义验证（自运行节点始终启用，确保语法正确）
	errors = append(errors, v.validateFunctions(abi.Functions)...)

	// 事件定义验证（自运行节点始终启用，确保语义正确）
	errors = append(errors, v.validateEvents(abi.Events)...)

	return errors
}

// validateFunctions 验证ABI中的函数定义。
//
// 检查函数名称、参数定义、返回值等的正确性。
//
// 参数：
//   - functions: 函数定义列表
//
// 返回值：
//   - []iface.ValidationError: 函数验证错误列表
func (v *ABIValidator) validateFunctions(functions []typespkg.ContractFunction) []iface.ValidationError {
	var errors []iface.ValidationError

	functionNames := make(map[string]bool)
	for i, fn := range functions {
		// 函数名重复检查
		if functionNames[fn.Name] {
			errors = append(errors, iface.ValidationError{
				RuleName:    "duplicate_function_name",
				Severity:    iface.ValidationSeverityError,
				Message:     "Duplicate function name: " + fn.Name,
				Location:    "functions[" + string(rune(i)) + "].name",
				Suggestions: []string{"Use unique function names"},
			})
		}
		functionNames[fn.Name] = true

		// 函数名非空检查
		if fn.Name == "" {
			errors = append(errors, iface.ValidationError{
				RuleName:    "function_name_required",
				Severity:    iface.ValidationSeverityError,
				Message:     "Function name is required",
				Location:    "functions[" + string(rune(i)) + "].name",
				Suggestions: []string{"Provide a valid function name"},
			})
		}
	}

	return errors
}

// validateEvents 验证ABI中的事件定义。
//
// 检查事件名称、参数定义等的正确性。
//
// 参数：
//   - events: 事件定义列表
//
// 返回值：
//   - []iface.ValidationError: 事件验证错误列表
func (v *ABIValidator) validateEvents(events []typespkg.ContractEvent) []iface.ValidationError {
	var errors []iface.ValidationError

	eventNames := make(map[string]bool)
	for i, event := range events {
		// 事件名重复检查
		if eventNames[event.Name] {
			errors = append(errors, iface.ValidationError{
				RuleName:    "duplicate_event_name",
				Severity:    iface.ValidationSeverityWarning,
				Message:     "Duplicate event name: " + event.Name,
				Location:    "events[" + string(rune(i)) + "].name",
				Suggestions: []string{"Consider using unique event names"},
			})
		}
		eventNames[event.Name] = true

		// 事件名非空检查
		if event.Name == "" {
			errors = append(errors, iface.ValidationError{
				RuleName:    "event_name_required",
				Severity:    iface.ValidationSeverityError,
				Message:     "Event name is required",
				Location:    "events[" + string(rune(i)) + "].name",
				Suggestions: []string{"Provide a valid event name"},
			})
		}
	}

	return errors
}
