// Package repository 提供UTXO数据仓储服务接口定义
//
// 📊 **UTXO Repository设计定位**
//
// 🎯 **核心职责**：
// 作为EUTXO系统的数据访问层，为系统内其他组件提供高效的UTXO数据服务。
// 专注于数据存储、查询和状态管理，不处理业务逻辑。
//
// 🏗️ **架构边界**：
// ✅ 数据仓储：专注UTXO数据的高效存取和索引查询
// ✅ 状态管理：支持UTXO生命周期状态转换和约束检查
// ✅ 操作支持：为引用、消费等交互操作提供数据层支持
// ❌ 业务逻辑：不处理余额计算、权限验证等业务逻辑
// ❌ 用户接口：用户通过AccountService接口，不直接接触UTXO
// ❌ 直接写入：UTXO创建只能通过区块处理内部流程
//
// 🔗 **服务对象**：
// • AccountService：提供余额聚合计算的底层数据支撑
// • TransactionValidator：提供交易验证所需的UTXO状态数据
// • ContractEngine：提供合约执行的UTXO引用和状态管理
// • ResourceManager：提供资源UTXO的管理和查询服务
// • API Layer：提供查询服务的数据基础
//
// 💡 **设计理念**：
// 遵循"数据源头约束"原则：所有UTXO数据来源于TxOutput，通过区块处理统一写入。
// Repository只提供读取、查询、状态更新接口，确保数据一致性和架构清晰性。
//
// ⚠️ **极简设计原则**：
// 本接口遵循极简设计哲学，拒绝过度工程化：
// • 存储层只负责CRUD操作，不处理UTXO选择逻辑
// • 不提供健康度报告、优化建议等无实际使用场景的功能
// • UTXO选择逻辑应内嵌在Transaction Manager等使用方内部
// • 当考虑添加新方法时，问自己："有人会真正使用这个功能吗？"
// • 如果答案不够肯定，答案就是"不需要"
package repository

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// UTXOManager UTXO数据仓储管理器接口
//
// 🎯 **业务导向简化设计**：
// 基于实际业务需求精简设计，去除过度复杂的功能，专注核心场景：
// • 交易验证：根据OutPoint获取UTXO进行验证
// • 余额计算：根据地址获取UTXO进行聚合
// • 并发控制：ResourceUTXO的引用计数管理
//
// 📊 **核心业务场景**：
// 1. TransactionValidator 验证交易输入
// 2. AccountService 计算用户余额
// 3. ContractEngine 管理资源并发访问
type UTXOManager interface {

	// ==================== 🔍 核心查询接口 ====================

	// GetUTXO 根据OutPoint精确获取UTXO
	//
	// 🎯 **核心用途**：交易验证的基础操作
	// 📞 **主要调用者**：TransactionValidator, ContractEngine
	GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error)

	// GetUTXOsByAddress 获取地址拥有的UTXO列表
	//
	// 🎯 **核心用途**：AccountService余额计算的数据基础
	// 📞 **主要调用者**：AccountService
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 所有者地址
	//   category: UTXO类型过滤（nil表示所有类型）
	//   onlyAvailable: 是否只返回可用状态的UTXO
	//
	// 返回：
	//   []*utxo.UTXO: UTXO列表
	//   error: 查询错误
	GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error)

	// ==================== 🔄 核心状态操作 ====================

	// ReferenceUTXO 引用UTXO（增加引用计数）
	//
	// 🎯 **核心用途**：ResourceUTXO并发控制
	// 📞 **主要调用者**：ContractEngine, ResourceManager
	//
	// 注意：只对ResourceUTXO有效，其他类型UTXO忽略此操作
	ReferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error

	// UnreferenceUTXO 解除UTXO引用（减少引用计数）
	//
	// 🎯 **核心用途**：ResourceUTXO引用完成后的清理
	// 📞 **主要调用者**：ContractEngine, ResourceManager
	UnreferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error

	// GetCurrentStateRoot 获取当前UTXO状态根
	//
	// 🎯 **核心用途**：为区块构建提供状态根哈希
	// 📞 **主要调用者**：BlockManager 构建区块头时使用
	//
	// 说明：
	//   状态根是当前所有UTXO状态的Merkle树根哈希
	//   用于区块头中记录当前区块链状态的摘要
	//   支持轻客户端和状态验证
	//
	// 返回：
	//   []byte: 32字节的状态根哈希，如果没有UTXO则返回空字节数组
	//   error: 计算错误
	GetCurrentStateRoot(ctx context.Context) ([]byte, error)

	// ⚠️ **重要说明**：UTXO消费不对外暴露
	// UTXO的消费只能由区块处理内部流程触发，当区块确认后才真正消费UTXO
	// 外部组件不应该直接消费UTXO，这违背了区块链的共识机制
}

// ==================== 📝 架构设计说明 ====================
//
// 🎯 **UTXO存储策略**：
//
// 📦 **双重存储模式**：
// • 区块存储：完整保存区块（含交易、资源），作为权威数据源
// • UTXO存储：从交易中解析的UTXO单独存储，用于快速查询和验证
// • UTXO存储相当于区块数据的"索引层"，提供高效的UTXO访问
//
// ♻️ **UTXO生命周期管理**：
// • 创建：区块确认后，从TxOutput解析创建UTXO
// • 引用：ResourceUTXO可被多次引用（引用计数管理）
// • 消费：区块确认后，直接从UTXO存储中删除，不保留历史
// • ⚠️ 关键：只有区块确认后才真正更新UTXO状态
//
// 🔒 **权限边界**：
// • 公共接口：只提供查询和引用计数管理
// • UTXO消费：只能由区块处理内部流程触发
// • 时序保证：交易提交 ≠ UTXO立即消费，需等待区块确认
//
// 🚫 **删除的过度设计**：
// • ConsumeUTXO公共方法：违背区块链共识机制
// • 复杂过滤器和统计：不是Repository层的核心职责
// • 批量操作：实际业务场景中使用频率低
//
// ✅ **保留的核心功能**：
// • GetUTXO：交易验证的基础（根据OutPoint精确查询）
// • GetUTXOsByAddress：余额计算的基础（按地址聚合查询）
// • Reference/UnreferenceUTXO：ResourceUTXO并发控制
//
// 💡 **实际调用示例**：
//
// // 1. TransactionValidator - 验证交易输入的UTXO存在性
// func (tv *TransactionValidator) ValidateInput(ctx context.Context, input *TxInput) error {
//     utxo, err := tv.utxoManager.GetUTXO(ctx, input.PreviousOutput)
//     if err != nil || utxo == nil {
//         return fmt.Errorf("引用的UTXO不存在")
//     }
//     if utxo.Status != utxo.UTXO_LIFECYCLE_AVAILABLE {
//         return fmt.Errorf("UTXO不可用")
//     }
//     // ⚠️ 注意：这里只是验证，不消费UTXO
//     // UTXO消费由区块确认后的内部流程处理
//     return nil
// }
//
// // 2. AccountService - 计算地址余额
// func (as *AccountService) GetPlatformBalance(ctx context.Context, address []byte) (*BalanceInfo, error) {
//     // 获取用户的Asset类型UTXO
//     assetUTXOs, err := as.utxoManager.GetUTXOsByAddress(ctx, address, &utxo.UTXO_CATEGORY_ASSET, true)
//     if err != nil {
//         return nil, err
//     }
//
//     var totalBalance uint64
//     for _, utxo := range assetUTXOs {
//         // 聚合计算总余额
//         totalBalance += extractAmountFromUTXO(utxo)
//     }
//     return &BalanceInfo{Available: totalBalance, UTXOCount: len(assetUTXOs)}, nil
// }
//
// // 3. ContractEngine - 引用ResourceUTXO进行合约执行
// func (ce *ContractEngine) ExecuteContract(ctx context.Context, contractOutPoint *OutPoint) error {
//     // 引用合约UTXO（增加引用计数，防止并发时被消费）
//     if err := ce.utxoManager.ReferenceUTXO(ctx, contractOutPoint); err != nil {
//         return fmt.Errorf("无法引用合约UTXO: %v", err)
//     }
//
//     defer func() {
//         // 执行完成后解除引用（减少引用计数）
//         _ = ce.utxoManager.UnreferenceUTXO(ctx, contractOutPoint)
//     }()
//
//     // 获取合约UTXO详细信息
//     contractUTXO, err := ce.utxoManager.GetUTXO(ctx, contractOutPoint)
//     if err != nil {
//         return fmt.Errorf("获取合约UTXO失败: %v", err)
//     }
//
//     // 执行合约逻辑...
//     return ce.executeContractLogic(contractUTXO)
// }
//
// // 🚫 反例：错误的UTXO消费方式
// // func SomeService(utxoManager UTXOManager) {
// //     // ❌ 错误：外部组件不应该直接消费UTXO
// //     // utxoManager.ConsumeUTXO(ctx, outpoint) // 这个方法不存在，也不应该存在
// //
// //     // ✅ 正确：UTXO消费由区块确认后的内部流程自动处理
// //     // 1. 用户提交交易到交易池
// //     // 2. 矿工将交易打包到区块
// //     // 3. 区块确认后，系统自动消费相关UTXO
// //     // 4. Repository内部通过区块处理流程更新UTXO状态
// // }
