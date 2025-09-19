// Package blockchain 提供WES系统的区块管理接口定义
//
// ⛓️ **区块管理服务 (Block Management Service)**
//
// 本文件定义了区块管理的核心接口，专注于：
// - 矿工挖矿：区块模板创建和最终区块提交
// - 同步验证：接收区块的验证和处理
// - 区块查询：基础的区块数据查询功能
//
// 🎯 **核心业务场景**
// - 矿工挖矿：创建区块模板、提交挖出的区块
// - 节点同步：验证和处理从网络接收的区块
// - 区块查询：获取区块信息用于展示和分析
// - 分叉处理：支持链重组和分叉解决
//
// 🏗️ **设计原则**
// - **业务导向**：面向真实的矿工和同步需求
// - **简化接口**：只提供核心必要的区块操作
// - **职责聚焦**：专注区块管理，不涉及链状态查询
// - **性能优化**：优化矿工和同步的关键路径
//
// 详细使用说明请参考：pkg/interfaces/blockchain/README.md
package blockchain

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// BlockService 区块管理服务接口
//
// 🎯 **专注矿工挖矿和节点同步核心业务逻辑**
//
// 核心职责：
// - 支持矿工的完整挖矿流程
// - 支持节点的区块验证和处理
// - 通过依赖注入调用 RepositoryManager 进行数据操作
// - 处理链重组和分叉情况
//
// 设计理念：
// - 业务导向：专注区块管理的核心业务逻辑，不处理数据层操作
// - 架构分层：严格遵循分层架构，通过 Repository 层进行数据访问
// - 职责单一：只处理挖矿、验证、处理等业务流程
// - 边界清晰：数据查询由 RepositoryManager 负责，业务逻辑由 BlockService 负责
type BlockService interface {
	// ==================== 矿工挖矿支持 ====================

	// CreateMiningCandidate 创建挖矿候选区块并返回区块哈希
	//
	// 🎯 **创建候选区块并采用哈希+缓存架构**
	//
	// 直接从交易池获取最优交易，构建候选区块供矿工挖矿。
	// 候选区块保存在内存缓存中，返回区块哈希作为标识符。
	// 矿工通过哈希从缓存获取区块进行POW计算。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//
	// 返回：
	//   []byte: 32字节候选区块哈希（基于区块头，不含POW字段）
	//   error: 创建错误，nil表示成功
	//
	// 内部流程：
	//   1. 通过 ConsensusService.GetCurrentMinerAddress() 获取矿工地址
	//   2. 通过 TxPool.GetTransactionsForMining() 获取优质交易
	//   3. 计算 Coinbase 交易（挖矿奖励 + 手续费）
	//   4. 构建候选区块结构（POW字段为空）
	//   5. 计算区块哈希并保存到缓存，设置TTL
	//
	// 🏗️ **架构一致性**：
	// 与 TransactionService 保持一致的哈希+缓存架构：
	//   - 返回轻量级哈希标识符，减少网络传输
	//   - 复杂对象存储在内存缓存中
	//   - 支持后续修改（POW计算类似签名过程）
	//
	// 使用场景：
	//   • ConsensusEngine 获取候选区块哈希
	//   • 矿工通过哈希获取区块进行 POW 计算
	//   • 共识引擎的区块模板管理
	//
	// 示例：
	//   blockHash, err := blockService.CreateMiningCandidate(ctx)
	//   if err != nil {
	//     return fmt.Errorf("创建候选区块失败: %v", err)
	//   }
	//   // 从缓存获取完整区块进行挖矿
	//   candidateBlock, err := blockCache.GetBlock(blockHash)
	CreateMiningCandidate(ctx context.Context) ([]byte, error)

	// ==================== 同步验证支持 ====================

	// ValidateBlock 验证区块
	//
	// 🎯 **验证从网络接收的区块**
	//
	// 对从其他节点接收的区块进行完整验证，
	// 确保符合共识规则和协议要求。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//   block: 待验证的区块
	//
	// 返回：
	//   bool: 验证结果，true表示有效
	//   error: 验证错误，nil表示验证完成
	//
	// 使用场景：
	//   • 节点验证从网络接收的区块
	//   • 同步过程中的区块有效性检查
	//   • 分叉处理中的区块验证
	//
	// 示例：
	//   // 从P2P网络接收区块
	//   receivedBlock := p2p.ReceiveBlock()
	//
	//   // 验证区块有效性
	//   valid, err := blockService.ValidateBlock(ctx, receivedBlock)
	//   if err != nil {
	//     return fmt.Errorf("验证区块失败: %v", err)
	//   }
	//
	//   if valid {
	//     // 处理有效区块
	//     err = blockService.ProcessBlock(ctx, receivedBlock)
	//   } else {
	//     // 拒绝无效区块
	//     fmt.Printf("❌ 拒绝无效区块: %x\n", receivedBlock.Header.Hash)
	//   }
	ValidateBlock(ctx context.Context, block *core.Block) (bool, error)

	// ProcessBlock 处理区块
	//
	// 🎯 **处理验证通过的区块**
	//
	// 执行区块中的交易，更新区块链状态，
	// 将区块添加到区块链中。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//   block: 已验证的区块
	//
	// 返回：
	//   error: 处理错误，nil表示成功
	//
	// 使用场景：
	//   • 处理验证通过的新区块
	//   • 同步过程中应用历史区块
	//   • 分叉解决后重新应用区块
	//
	// 示例：
	//   if valid, _ := blockService.ValidateBlock(ctx, newBlock); valid {
	//     err := blockService.ProcessBlock(ctx, newBlock)
	//     if err != nil {
	//       return fmt.Errorf("处理区块失败: %v", err)
	//     }
	//     fmt.Printf("✅ 区块 %d 处理完成\n", newBlock.Header.Height)
	//   }
	ProcessBlock(ctx context.Context, block *core.Block) error
}

// ============================================================================
//                              设计说明
// ============================================================================

// 🎯 **BlockService设计理念**
//
// **专注区块管理核心业务逻辑，遵循架构分层原则**：
//
// 1. **矿工挖矿支持**：
//    ```go
//    blockHash, err := blockService.CreateMiningCandidate(ctx)  // 创建候选区块，返回哈希
//    block, err := blockCache.GetBlock(blockHash)               // 从缓存获取完整区块
//    ```
//    - 采用与 TransactionService 一致的哈希+缓存架构
//    - 返回轻量级哈希，减少网络传输开销
//    - 支持矿工修改区块（类似交易签名流程）
//
// 2. **同步验证支持**：
//    ```go
//    valid, err := blockService.ValidateBlock(ctx, receivedBlock)  // 验证区块
//    err := blockService.ProcessBlock(ctx, validBlock)             // 处理区块
//    ```
//    - 支持节点间的区块同步
//    - 完整的区块验证和处理流程
//    - 处理分叉和链重组场景
//
// **这就是区块管理的核心业务逻辑！**
//
// ✅ **正确的架构分层**：
// ```
// 矿工/同步组件
//    ↓ (挖矿和同步需求)
// pkg/interfaces/blockchain/block (业务接口) ← 当前文件
//    ↓ (业务实现层，调用数据层)
// internal/core/blockchain/services/block
//    ↓ (数据访问层)
// pkg/interfaces/repository/repository (数据接口)
//    ↓ (数据实现层)
// internal/core/repositories
// ```
//
// 🎯 **与其他接口的清晰边界**：
// - **BlockService**：区块管理业务逻辑（挖矿、验证、处理）
// - **RepositoryManager**：数据存储和查询（GetBlock、GetBlockByHeight、GetBlockRange）
// - **ChainService**：链状态查询（面向状态监控）
// - **TransactionService**：交易处理和查询（面向交易流程）
// - **AccountService**：用户账户和资产查询（面向用户资产）
// - **ResourceService**：资源管理和调用（面向资源生命周期）
//
// 🏗️ **架构优势**：
// - **职责单一**：BlockService 专注业务逻辑，RepositoryManager 专注数据操作
// - **依赖清晰**：BlockService 通过依赖注入调用 RepositoryManager
// - **架构一致**：与 TransactionService 统一采用哈希+缓存模式
// - **可测试性**：业务逻辑与数据操作分离，便于单元测试
// - **可维护性**：清晰的分层边界，降低组件间耦合
//
// 🔐 **哈希计算原则**：
// - **CreateMiningCandidate**：返回候选区块哈希（基于区块头，不含POW字段）
// - **矿工修改**：类似交易签名，矿工在缓存中修改POW字段
// - **最终提交**：挖矿成功后提交完整区块（含POW字段）
// - **一致性**：与 TransactionService 的"构建→修改→提交"流程保持一致
//
// 💡 **实际业务场景覆盖**：
// - 矿工挖矿：候选区块创建的完整业务流程
// - 节点同步：区块验证和处理的完整业务流程
// - 数据查询：通过 RepositoryManager 进行所有区块数据查询
// - 分叉处理：支持链重组和分叉解决的业务逻辑
