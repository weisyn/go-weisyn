// Package crypto 提供WES系统的POW（工作量证明）接口定义
//
// ⚡ **POW计算服务 (Proof of Work Service)**
//
// 本文件定义了WES区块链系统的POW计算接口，专注于：
// - 挖矿计算：对区块头进行POW计算，生成满足难度的nonce
// - 验证计算：验证区块头的POW是否满足难度要求
// - 算法封装：封装具体的POW算法实现细节
// - 性能优化：提供高效的挖矿和验证计算
//
// 🎯 **核心功能**
// - POWEngine：POW引擎接口，提供挖矿和验证服务
// - 算法抽象：统一的POW计算接口，支持不同算法实现
// - 难度管理：自动处理难度计算和验证逻辑
// - 上下文支持：支持取消和超时控制
//
// 🏧 **设计原则**
// - 接口简洁：仅提供挖矿和验证两个核心方法
// - 职责单一：专注POW计算，不涉及业务逻辑
// - 性能优先：高效的计算实现和内存管理
// - 易用性：统一的接口设计和错误处理
//
// 🔗 **组件关系**
// - POWEngine：被区块挖矿、区块验证等模块使用
// - 与HashManager：配合进行哈希计算
// - 与共识模块：提供POW计算支持
package crypto

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// POWEngine 定义POW（工作量证明）计算接口
//
// 提供WES区块链系统的POW计算服务：
// - 挖矿计算：对区块头进行POW计算，生成满足难度要求的nonce
// - 验证计算：验证区块头的POW是否满足难度要求
//
// 🎯 **接口设计原则**：
// - 简洁明了：仅提供挖矿和验证两个核心方法
// - 输入统一：都以BlockHeader为核心参数
// - 职责单一：专注POW计算，不涉及区块业务逻辑
// - 上下文控制：支持取消和超时机制
type POWEngine interface {
	// MineBlockHeader 对区块头进行POW挖矿计算
	//
	// 🎯 **挖矿计算**：
	// 对传入的区块头进行POW计算，通过不断尝试不同的nonce值，
	// 直到找到满足难度要求的哈希值，返回包含正确nonce的新区块头。
	//
	// 📋 **参数说明**：
	//   - ctx: 上下文控制，支持取消和超时
	//   - header: 输入的区块头（需要包含difficulty字段）
	//
	// 🔄 **返回值**：
	//   - *core.BlockHeader: 包含正确nonce的新区块头
	//   - error: 挖矿失败时的错误（如上下文取消、难度无效等）
	//
	// 💡 **实现要求**：
	// - 输入header不能为nil，且必须包含有效的difficulty
	// - 返回的header应包含满足难度要求的nonce和timestamp
	// - 支持上下文取消，避免无限计算
	// - 线程安全，支持并发调用
	MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error)

	// VerifyBlockHeader 验证区块头的POW是否有效
	//
	// 🎯 **验证计算**：
	// 验证传入的区块头是否满足POW要求，检查其nonce值产生的哈希
	// 是否满足指定的难度目标。
	//
	// 📋 **参数说明**：
	//   - header: 需要验证的区块头（必须包含nonce和difficulty）
	//
	// 🔄 **返回值**：
	//   - bool: true表示POW验证通过，false表示验证失败
	//   - error: 验证过程中的错误（如参数无效、计算错误等）
	//
	// 💡 **实现要求**：
	// - 输入header不能为nil，且必须包含有效的nonce和difficulty
	// - 快速验证，性能优化，适合频繁调用
	// - 验证算法必须与挖矿算法完全一致
	// - 线程安全，支持并发验证
	VerifyBlockHeader(header *core.BlockHeader) (bool, error)
}
