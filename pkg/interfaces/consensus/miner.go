// Package consensus 提供WES系统的矿工服务接口定义
//
// ⛏️ **矿工服务 (Miner Service)**
//
// 本文件定义了区块链矿工的公共接口，专注于：
// - 挖矿生命周期控制（启动/停止）
// - 挖矿状态查询和监控
// - 用户友好的挖矿管理
//
// 🎯 **设计原则**
// - **用户友好**：接受私钥参数，自动推导矿工地址
// - **接口一致**：与其他服务接口保持参数风格一致
// - **职责单一**：专注于挖矿控制，不涉及共识算法细节
// - **简单实用**：只暴露核心的挖矿管理功能
//
// 📊 **主要使用场景**
// - API层：用户挖矿控制接口
// - CLI工具：命令行挖矿管理
// - 应用集成：第三方应用集成挖矿功能
//
// 🔗 **与其他服务的关系**
// - 内部调用共识引擎进行实际挖矿
// - 从私钥自动推导矿工地址
// - 提供挖矿状态的实时查询
package consensus

import (
	"context"
)

// ==================== 矿工服务公共接口 ====================

// MinerService 矿工服务的公共接口
//
// 🎯 **设计原则：用户友好的挖矿控制**
// 矿工服务的核心职责：
// 1. 启动挖矿：用户提供私钥，系统自动推导地址并开始挖矿
// 2. 停止挖矿：优雅停止挖矿进程，释放计算资源
// 3. 状态查询：获取当前挖矿状态和矿工信息
//
// 🔑 **私钥参数设计**
// - 用户提供私钥，系统自动计算矿工地址
// - 保持与TransferAsset等接口的参数风格一致
// - 无状态设计：每次调用都携带身份凭证
//
// 📊 **接口特点**
// - 简单易用：只需3个核心方法
// - 参数统一：都使用私钥作为身份凭证
// - 状态透明：可实时查询挖矿状态
//
// 🎯 **这就是公共接口应该有的样子：简单、一致、用户友好**
type MinerService interface {
	// ==================== 挖矿控制 ====================

	// StartMining 启动挖矿
	//
	// 🎯 **功能说明**
	// 启动PoW挖矿进程，开始区块竞争和奖励获取。
	// 用户提供矿工地址，系统开始挖矿并将奖励发送到指定地址。
	//
	// 🔧 **参数说明**
	//   ctx: 上下文对象，用于控制启动超时和取消操作
	//   minerAddress: 矿工地址，用于接收区块奖励和交易费
	//
	// 📋 **地址格式规范**（遵循pb/blockchain/block/transaction/transaction.proto）
	//   - 地址长度：固定20字节（AddressHashLength = 20）
	//   - 地址格式：raw_hash字段，即RIPEMD160(SHA256(PublicKey))
	//   - 椭圆曲线：secp256k1（与Bitcoin兼容）
	//   - 版本字节：P2PKH=0x1C, P2SH=0x9C（WES专用版本）
	//   - 编码格式：内部存储为20字节raw_hash，显示时使用Base58Check编码
	//
	// 🔄 **行为说明**
	//   - 验证minerAddress格式和长度（必须为20字节raw_hash）
	//   - 启动PoW挖矿算法，开始计算工作量证明
	//   - 自动从交易池获取交易并构建区块候选
	//   - 挖矿成功后自动广播区块到网络
	//   - 奖励和交易费自动分配到指定的minerAddress
	//
	// ⚠️ **重要说明**
	//   - 挖矿是计算密集型操作，会占用大量CPU资源
	//   - 如果已在挖矿，会返回相应错误而不是覆盖
	//   - minerAddress必须是有效的20字节地址哈希
	//   - 启动是异步的，方法返回成功不代表立即开始挖矿
	//   - 地址格式必须符合internal/core/infrastructure/crypto/address/address.go规范
	//
	// 🚨 **错误情况**
	//   - ErrMiningAlreadyStarted: 挖矿已在运行
	//   - ErrInvalidMinerAddress: 矿工地址格式无效或长度错误
	//   - ErrUnsupportedAddressVersion: 不支持的地址版本
	//   - ErrSystemNotReady: 系统未就绪，无法启动挖矿
	//
	// 💡 **API使用场景**
	//   POST /api/mining/start
	//   Body: {"minerAddress": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"}  // Base58Check编码
	//   注意：API接收Base58Check格式，内部转换为20字节raw_hash
	//
	// 🔄 **返回值**
	//   - error: 错误信息，nil表示启动成功
	StartMining(ctx context.Context, minerAddress []byte) error

	// StartMiningOnce 启动单次挖矿（挖一个区块后自动停止）
	//
	// 🎯 **功能说明**
	// 启动PoW挖矿进程，但只挖掘一个区块后自动停止。
	// 适用于测试环境或需要手动控制挖矿节奏的场景。
	//
	// 🔧 **参数说明**
	//   ctx: 上下文对象，用于控制启动超时和取消操作
	//   minerAddress: 矿工地址，用于接收区块奖励和交易费
	//
	// 🔄 **行为说明**
	//   - 启动挖矿进程
	//   - 挖出一个区块后自动停止
	//   - 不触发下一轮挖矿
	//
	// ⚠️ **与 StartMining 的区别**
	//   - StartMining: 持续挖矿，需要手动调用 StopMining
	//   - StartMiningOnce: 单次挖矿，挖完一个区块自动停止
	//
	// 💡 **API使用场景**
	//   POST /api/mining/once
	//
	// 🔄 **返回值**
	//   - error: 错误信息，nil表示启动成功
	StartMiningOnce(ctx context.Context, minerAddress []byte) error

	// StopMining 停止挖矿
	//
	// 🎯 **功能说明**
	// 停止当前正在进行的PoW挖矿进程，释放计算资源。
	// 这是用户主动停止挖矿的标准接口。
	//
	// 🔧 **参数说明**
	//   ctx: 上下文对象，用于控制停止超时
	//
	// 🔄 **行为说明**
	//   - 优雅停止PoW挖矿算法
	//   - 完成当前正在处理的工作后退出
	//   - 释放挖矿占用的系统资源
	//   - 保存挖矿统计信息（内部处理）
	//
	// ⚠️ **重要说明**
	//   - 停止是异步的，方法返回不代表立即停止
	//   - 如果未在挖矿，会返回相应错误
	//   - 不会影响已挖出但未广播的区块
	//   - 系统会等待当前工作完成后再停止
	//
	// 🚨 **错误情况**
	//   - ErrMiningNotStarted: 挖矿未在运行
	//   - ErrStopTimeout: 停止操作超时
	//   - ErrInternalError: 内部系统错误
	//
	// 💡 **API使用场景**
	//   POST /api/mining/stop
	//
	// 🔄 **返回值**
	//   - error: 错误信息，nil表示停止成功
	StopMining(ctx context.Context) error

	// GetMiningStatus 获取挖矿状态
	//
	// 🎯 **功能说明**
	// 查询当前PoW挖矿的实时状态和矿工信息。
	// 用于用户了解挖矿状态和系统监控。
	//
	// 🔧 **参数说明**
	//   ctx: 上下文对象，用于控制查询超时
	//
	// 🔄 **返回值**
	//   isRunning: true表示正在挖矿，false表示未在挖矿
	//   minerAddress: 当前挖矿使用的地址（20字节raw_hash格式）
	//   err: 错误信息，nil表示查询成功
	//
	// 📋 **返回地址格式**（遵循WES地址规范）
	//   - 长度：固定20字节
	//   - 格式：raw_hash，即RIPEMD160(SHA256(PublicKey))
	//   - 显示：需要通过AddressService转换为Base58Check编码格式
	//   - 空值：isRunning=false时，minerAddress为空字节数组
	//
	// ⚠️ **重要说明**
	//   - 返回的是实时运行状态，不是配置状态
	//   - isRunning=false时，minerAddress为空（长度为0）
	//   - 这是高频调用的方法，实现需要优化性能
	//   - 用于状态监控和用户界面展示
	//   - 返回的minerAddress来自启动时设置的地址
	//
	// 🚨 **错误情况**
	//   - ErrInternalError: 内部系统错误
	//
	// 💡 **API使用场景**
	//   GET /api/mining/status
	//   Response: {"isRunning": true, "minerAddress": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"}
	//   注意：API返回Base58Check格式，内部需要从20字节raw_hash转换
	//
	// 💡 **使用示例**
	//   isRunning, addrBytes, err := minerService.GetMiningStatus(ctx)
	//   if err != nil {
	//     log.Printf("查询挖矿状态失败: %v", err)
	//     return
	//   }
	//
	//   if isRunning {
	//     // addrBytes是20字节的raw_hash，需要转换为可读格式
	//     log.Printf("⛏️ 正在挖矿，矿工地址哈希: %x", addrBytes)
	//   } else {
	//     log.Printf("⏸️ 挖矿已停止")
	//   }
	GetMiningStatus(ctx context.Context) (isRunning bool, minerAddress []byte, err error)
}

// ❌ **删除说明：解决职责边界混乱问题**
//
// 🚨 **删除内容包括**：
// - ValidateBlock() - ❌ 职责重复！应该由blockchain.BlockService负责
// - ProduceBlock() - ❌ 职责混乱！区块创建应该由blockchain负责
// - ValidateBlockHeader() - 应该是内部实现细节
// - VerifyProofOfWork() - 应该是内部实现细节
// - MiningService/ValidationService - 不必要的接口拆分
// - 25+个事件结构体 - 过度的事件设计
// - DistributionManager等分发接口 - 应该在network模块
// - 大量的Metrics和Progress结构 - 过度监控
//
// 🎯 **职责边界修正**：
// ✅ 从834行缩减到70行 (减少92%+)
// ✅ 从25+个方法缩减到2个方法
// ✅ 解决了与blockchain.BlockService的职责重复
// ✅ 删除了手动区块生产的不必要功能
// ✅ 专注于PoW挖矿这一核心职责
// ✅ 清晰的模块边界：consensus=挖矿，blockchain=区块处理
//
// 💡 **正确的架构分工**：
// ```
// consensus.ConsensusService:
//   ├── SetMiningEnabled() - 控制PoW挖矿
//   └── GetMiningEnabled() - 查询挖矿状态
//
// blockchain.BlockService:
//   ├── ValidateBlock() - 验证区块
//   ├── CreateBlockTemplate() - 创建区块模板
//   ├── ProcessBlock() - 处理区块
//   └── ApplyBlock() - 应用区块
// ```
//
// 🔧 **使用示例**：
//
// ```go
// // 用户控制挖矿（使用地址）
// // 方式1：直接使用20字节地址哈希
// minerAddrHash := []byte{0x1c, 0x23, 0x45, ...} // 20字节raw_hash
// err := minerService.StartMining(ctx, minerAddrHash)
// if err != nil {
//     log.Printf("启动挖矿失败: %v", err)
// }
//
// // 方式2：从Base58Check地址转换（推荐）
// base58Addr := "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"
// addrBytes, err := addressService.AddressToBytes(base58Addr)
// if err != nil {
//     log.Printf("地址转换失败: %v", err)
// }
// err = minerService.StartMining(ctx, addrBytes)
//
// // 查询挖矿状态
// isRunning, minerAddr, err := minerService.GetMiningStatus(ctx)
// if isRunning {
//     log.Printf("正在挖矿，矿工地址哈希: %x", minerAddr)
//
//     // 转换为可读地址格式
//     readableAddr, _ := addressService.BytesToAddress(minerAddr)
//     log.Printf("矿工地址: %s", readableAddr)
// }
//
// // 停止挖矿
// err = minerService.StopMining(ctx)
// if err != nil {
//     log.Printf("停止挖矿失败: %v", err)
// }
// ```
