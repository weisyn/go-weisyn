// Package log 提供WES系统的日志级别接口定义
//
// 📃 **日志级别管理 (Logging Level Management)**
//
// 本文件定义了WES系统的日志级别和相关接口，专注于：
// - 统一的日志级别定义
// - 日志级别的管理和控制
// - 日志输出的格式化和过滤
// - 运行时日志级别的动态调整
//
// 🎯 **设计原则**
// - 标准化：遵循常见的日志级别标准
// - 灵活控制：支持细粒度的日志级别控制
// - 性能优先：优化日志处理性能，避免影响主业务
// - 可观测性：为系统监控和调试提供充分信息
// Package log 提供WES系统的日志级别接口定义
//
// 📊 **日志级别管理 (Log Level Management)**
//
// 本文件定义了WES区块链系统的日志级别管理接口，专注于：
// - 日志级别定义：提供标准的日志级别常量和枚举
// - 级别判断：支持日志级别的比较和选择
// - 字符串转换：提供日志级别和字符串的相互转换
// - 默认配置：提供合理的默认日志级别设置
//
// 🎯 **核心功能**
// - Level：日志级别枚举类型，定义所有可用的日志级别
// - 级别常量：提供Debug、Info、Warn、Error等标准级别
// - 转换方法：支持级别名称和枚举值的相互转换
// - 比较方法：支持日志级别的大小比较和筛选
//
// 🏧 **设计原则**
// - 标准兼容：遵循通用的日志级别标准和命名规范
// - 性能高效：使用枚举类型提高比较和判断效率
// - 易用性：提供简单直观的级别操作接口
// - 灵活性：支持多种级别表示和转换方式
//
// 🔗 **组件关系**
// - Level：被日志记录器、配置系统等模块使用
// - 与Logger：为日志记录器提供级别管理能力
// - 与Config：为配置系统提供日志级别选项
package log

import "github.com/weisyn/v1/pkg/types"

// 兼容别名（迁至 pkg/types）
type LogLevel = types.LogLevel

// 常量别名（向后兼容）
const (
	DebugLevel = types.DebugLevel
	InfoLevel  = types.InfoLevel
	WarnLevel  = types.WarnLevel
	ErrorLevel = types.ErrorLevel
	FatalLevel = types.FatalLevel
)
