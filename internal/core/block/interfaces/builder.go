// Package interfaces 定义 Block 模块的内部接口
//
// 🎯 **内部接口层**
//
// 本包定义 Block 模块的内部接口，这些接口：
// - 继承公共接口（pkg/interfaces/block）
// - 扩展内部管理方法
// - 提供指标和监控接口
//
// 🏗️ **设计原则**：
// - 接口继承：通过嵌入继承公共接口
// - 职责分离：每个接口专注一个核心能力
// - 内部扩展：只添加内部需要的方法
package interfaces

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/block"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// InternalBlockBuilder 内部区块构建接口
//
// 🎯 **设计理念**：
// - 继承公共接口，确保外部可见性
// - 添加缓存管理，支持候选区块复用
// - 提供指标接口，支持监控和调试
//
// 📞 **使用者**：
// - Consensus 模块：创建挖矿候选区块
// - 内部管理工具：监控构建性能
// - 测试框架：验证候选区块
type InternalBlockBuilder interface {
	block.BlockBuilder // 嵌入公共接口

	// ==================== 内部管理方法 ====================

	// GetBuilderMetrics 获取构建服务指标
	//
	// 用途：
	// - 监控系统：收集构建性能指标
	// - 调试工具：分析构建行为
	// - 告警系统：检测异常情况
	//
	// 返回：
	//   - *BuilderMetrics: 构建服务指标
	//   - error: 获取错误
	GetBuilderMetrics(ctx context.Context) (*BuilderMetrics, error)

	// GetCachedCandidate 获取缓存的候选区块
	//
	// 用途：
	// - 共识引擎：获取待挖矿区块
	// - 测试工具：验证候选区块
	//
	// 参数：
	//   - ctx: 上下文
	//   - blockHash: 候选区块哈希
	//
	// 返回：
	//   - *core.Block: 候选区块
	//   - error: 获取错误（如缓存不存在）
	GetCachedCandidate(ctx context.Context, blockHash []byte) (*core.Block, error)

	// ClearCandidateCache 清理候选区块缓存
	//
	// 用途：
	// - 内存管理：定期清理过期候选区块
	// - 链切换：分叉后清理无效候选
	//
	// 参数：
	//   - ctx: 上下文
	//
	// 返回：
	//   - error: 清理错误
	ClearCandidateCache(ctx context.Context) error

	// RemoveCachedCandidate 从缓存中移除指定的候选区块
	//
	// 用途：
	// - 区块挖出后：移除已成功挖出的候选区块
	// - 过期清理：移除过期的候选区块
	// - 分叉处理：移除分叉链上的无效候选区块
	//
	// 参数：
	//   - ctx: 上下文
	//   - blockHash: 候选区块哈希
	//
	// 返回：
	//   - error: 移除错误（如缓存不存在）
	RemoveCachedCandidate(ctx context.Context, blockHash []byte) error

	// SetMinerAddress 设置矿工地址
	//
	// 🎯 **运行时矿工地址设置**
	//
	// 用途：
	// - 挖矿启动时设置矿工地址
	// - 用于构建包含区块奖励的 Coinbase 交易
	//
	// 参数：
	//   - minerAddr: 矿工地址（20字节）
	//
	// 说明：
	//   - 在挖矿启动时由 MinerController 调用
	//   - 支持运行时动态设置
	SetMinerAddress(minerAddr []byte)
}

// BuilderMetrics 构建服务指标
//
// 📊 **指标说明**：
// - 统计指标：记录构建活动统计
// - 时间指标：记录构建性能
// - 缓存指标：记录缓存使用情况
// - 状态指标：记录服务健康状态
type BuilderMetrics struct {
	// ==================== 统计指标 ====================

	// CandidatesCreated 已创建候选区块数
	CandidatesCreated uint64 `json:"candidates_created"`

	// CacheHits 缓存命中次数
	CacheHits uint64 `json:"cache_hits"`

	// CacheMisses 缓存未命中次数
	CacheMisses uint64 `json:"cache_misses"`

	// ==================== 时间指标 ====================

	// LastCandidateTime 最后创建时间（Unix时间戳）
	LastCandidateTime int64 `json:"last_candidate_time"`

	// AvgCreationTime 平均创建耗时（秒）
	AvgCreationTime float64 `json:"avg_creation_time"`

	// MaxCreationTime 最大创建耗时（秒）
	MaxCreationTime float64 `json:"max_creation_time"`

	// ==================== 缓存指标 ====================

	// CacheSize 当前缓存大小
	CacheSize int `json:"cache_size"`

	// MaxCacheSize 最大缓存大小
	MaxCacheSize int `json:"max_cache_size"`

	// ==================== 状态指标 ====================

	// IsHealthy 健康状态
	IsHealthy bool `json:"is_healthy"`

	// ErrorMessage 错误信息（如果有）
	ErrorMessage string `json:"error_message,omitempty"`
}

