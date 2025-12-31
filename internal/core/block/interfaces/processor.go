// Package interfaces 定义 Block 模块的内部接口
package interfaces

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/block"
)

// InternalBlockProcessor 内部区块处理接口
//
// 🎯 **设计理念**：
// - 继承公共接口，确保外部可见性
// - 添加状态管理，支持处理流程控制
// - 提供指标接口，支持监控和调试
//
// 📞 **使用者**：
// - Sync 模块：处理同步的区块
// - Consensus 模块：处理挖矿成功的区块
// - 内部管理工具：监控处理性能
type InternalBlockProcessor interface {
	block.BlockProcessor // 嵌入公共接口

	// ==================== 内部管理方法 ====================

	// GetProcessorMetrics 获取处理服务指标
	//
	// 用途：
	// - 监控系统：收集处理性能指标
	// - 调试工具：分析处理行为
	// - 告警系统：检测异常情况
	//
	// 返回：
	//   - *ProcessorMetrics: 处理服务指标
	//   - error: 获取错误
	GetProcessorMetrics(ctx context.Context) (*ProcessorMetrics, error)

	// SetValidator 设置验证器（延迟依赖注入）
	//
	// 用途：
	// - 避免循环依赖：Processor 需要 Validator，但不在构造时注入
	// - fx 生命周期：在模块启动后注入
	//
	// 参数：
	//   - validator: 验证器实例
	SetValidator(validator InternalBlockValidator)
}

// ProcessorMetrics 处理服务指标
//
// 📊 **指标说明**：
// - 统计指标：记录处理活动统计
// - 时间指标：记录处理性能
// - 数据指标：记录处理数据
// - 状态指标：记录服务健康状态
type ProcessorMetrics struct {
	// ==================== 统计指标 ====================

	// BlocksProcessed 已处理区块数
	BlocksProcessed uint64 `json:"blocks_processed"`

	// TransactionsExecuted 已执行交易数
	TransactionsExecuted uint64 `json:"transactions_executed"`

	// SuccessCount 成功次数
	SuccessCount uint64 `json:"success_count"`

	// FailureCount 失败次数
	FailureCount uint64 `json:"failure_count"`

	// ==================== 时间指标 ====================

	// LastProcessTime 最后处理时间（Unix时间戳）
	LastProcessTime int64 `json:"last_process_time"`

	// AvgProcessTime 平均处理耗时（秒）
	AvgProcessTime float64 `json:"avg_process_time"`

	// MaxProcessTime 最大处理耗时（秒）
	MaxProcessTime float64 `json:"max_process_time"`

	// ==================== 数据指标 ====================

	// LastBlockHeight 最后处理区块高度
	LastBlockHeight uint64 `json:"last_block_height"`

	// LastBlockHash 最后处理区块哈希
	LastBlockHash []byte `json:"last_block_hash,omitempty"`

	// ==================== 状态指标 ====================

	// IsProcessing 是否正在处理
	IsProcessing bool `json:"is_processing"`

	// IsHealthy 健康状态
	IsHealthy bool `json:"is_healthy"`

	// ErrorMessage 错误信息（如果有）
	ErrorMessage string `json:"error_message,omitempty"`
}

