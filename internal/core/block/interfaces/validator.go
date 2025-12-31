// Package interfaces 定义 Block 模块的内部接口
package interfaces

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/block"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// InternalBlockValidator 内部区块验证接口
//
// 🎯 **设计理念**：
// - 继承公共接口，确保外部可见性
// - 添加内部验证方法，支持模块化验证
// - 提供指标接口，支持监控和调试
//
// 📞 **使用者**：
// - BlockProcessor：处理前验证区块
// - Sync 模块：同步过程中验证区块
// - 网络层：接收区块时验证
type InternalBlockValidator interface {
	block.BlockValidator // 嵌入公共接口

	// ==================== 内部管理方法 ====================

	// GetValidatorMetrics 获取验证服务指标
	//
	// 用途：
	// - 监控系统：收集验证性能指标
	// - 调试工具：分析验证行为
	// - 告警系统：检测异常情况
	//
	// 返回：
	//   - *ValidatorMetrics: 验证服务指标
	//   - error: 获取错误
	GetValidatorMetrics(ctx context.Context) (*ValidatorMetrics, error)

	// ValidateStructure 验证区块结构（内部方法）
	//
	// 用途：
	// - 模块化验证：分步验证，便于调试
	// - 快速失败：结构错误时快速返回
	//
	// 参数：
	//   - ctx: 上下文
	//   - block: 待验证区块
	//
	// 返回：
	//   - error: 验证错误，nil表示通过
	ValidateStructure(ctx context.Context, block *core.Block) error

	// ValidateConsensus 验证共识规则（内部方法）
	//
	// 用途：
	// - 共识验证：检查POW、难度等
	// - 独立测试：可单独测试共识验证
	//
	// 参数：
	//   - ctx: 上下文
	//   - block: 待验证区块
	//
	// 返回：
	//   - error: 验证错误，nil表示通过
	ValidateConsensus(ctx context.Context, block *core.Block) error
}

// ValidatorMetrics 验证服务指标
//
// 📊 **指标说明**：
// - 统计指标：记录验证活动统计
// - 失败分类：记录不同类型的验证失败
// - 时间指标：记录验证性能
// - 状态指标：记录服务健康状态
type ValidatorMetrics struct {
	// ==================== 统计指标 ====================

	// BlocksValidated 已验证区块数
	BlocksValidated uint64 `json:"blocks_validated"`

	// ValidationsPassed 验证通过次数
	ValidationsPassed uint64 `json:"validations_passed"`

	// ValidationsFailed 验证失败次数
	ValidationsFailed uint64 `json:"validations_failed"`

	// ==================== 失败分类 ====================

	// StructureErrors 结构错误次数
	StructureErrors uint64 `json:"structure_errors"`

	// ConsensusErrors 共识错误次数
	ConsensusErrors uint64 `json:"consensus_errors"`

	// TransactionErrors 交易错误次数
	TransactionErrors uint64 `json:"transaction_errors"`

	// ChainErrors 链连接性错误次数（P3-8）
	ChainErrors uint64 `json:"chain_errors"`

	// ==================== 时间指标 ====================

	// LastValidateTime 最后验证时间（Unix时间戳）
	LastValidateTime int64 `json:"last_validate_time"`

	// AvgValidateTime 平均验证耗时（秒）
	AvgValidateTime float64 `json:"avg_validate_time"`

	// MaxValidateTime 最大验证耗时（秒）
	MaxValidateTime float64 `json:"max_validate_time"`

	// ==================== 状态指标 ====================

	// IsHealthy 健康状态
	IsHealthy bool `json:"is_healthy"`

	// ErrorMessage 错误信息（如果有）
	ErrorMessage string `json:"error_message,omitempty"`
}

