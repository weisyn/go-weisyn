// Package blockchain 提供WES系统的交易服务接口定义
//
// 🎯 **统一交易服务接口架构**
//
// 本文件定义了完整的区块链交易服务接口，基于以下架构设计：
// - 业务友好的参数接口，隐藏底层复杂性
// - 统一的交易生命周期管理
// - 分离的资源类型处理（合约、AI模型、静态资源）
// - 企业级多签会话支持
//
// 🏗️ **核心设计原则**
//
// - **业务导向**: 从用户操作出发，提供直观的业务接口
// - **统一流程**: 所有操作都遵循 构建→签名→提交→查询 的标准流程
// - **分层职责**: 接口层只定义服务契约，数据结构在pkg/types中定义
// - **pb集成**: 底层使用protobuf标准定义，上层提供业务抽象
//
// 🔗 **与其他服务的关系**
//
// - AccountService: 处理余额查询和账户管理
// - ChainService: 处理区块链状态查询
// - 本服务: 专注交易的创建、签名、提交和状态管理
//
// 详细使用说明请参考：pkg/interfaces/blockchain/README.md
package blockchain

import (
	"context"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/types"
)

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                           🏦  TRANSACTION SERVICE INTERFACE                                  █
// █                                                                                              █
// █   统一交易服务：处理所有类型的区块链交易操作（价值转移、资源部署、合约执行）                         █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████
//
// # TransactionService 统一交易服务接口
//
// 🎯 **核心职责**：
//   - 价值载体交易：资产转账、代币操作
//   - 能力载体交易：静态资源部署和管理
//   - 提供简单接口(90%用户) + 高级接口(10%企业用户)
//
// 🏗️ **设计原则**：
//   - 用户意图 → 业务操作 → 交易构建 → 签名授权 → 网络提交
//   - 业务友好的参数，隐藏底层protobuf复杂性
//   - 统一的错误处理和状态管理
type TransactionService interface {

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                          💰  价值载体交易 (AssetOutput)                                   ║
	// ║                                                                                          ║
	// ║  处理所有与资产转移相关的交易：原生币转账、代币转账、批量转账等                                 ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// TransferAsset 转账操作（支持基础和高级模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil）：个人日常转账，系统自动处理
	//   - 高级模式（options!=nil）：企业级转账，支持复杂业务场景
	//
	// 📝 **基础模式典型流程**：
	//   用户输入接收地址和金额 → 系统选择UTXO → 计算找零 → 生成交易
	//
	// 📝 **高级模式支持的业务场景**：
	//   - 个人私有：默认SingleKeyLock
	//   - 团队多签：自动MultiKeyLock（如：3-of-5签名）
	//   - 定时转账：自动TimeLock（如：年终奖延迟发放）
	//   - 分期释放：自动HeightLock（如：员工期权分4年释放）
	//   - 委托代理：自动DelegationLock（如：财务代理转账）
	//   - 付费转账：自动ContractLock（如：条件满足时自动转账）
	//   - 银行级：自动ThresholdLock（如：央行数字货币）
	//
	// 📊 **自动处理特性**：
	//   ✓ UTXO智能选择     ✓ 找零自动计算
	//   ✓ 手续费估算       ✓ 余额充足性验证
	//   ✓ 锁定机制映射     ✓ 业务策略转换
	//
	// 🔐 **锁定机制映射**：
	//   业务策略 → 系统自动选择对应的protobuf锁定机制
	//
	// 💡 **参数说明**：
	//   - senderPrivateKey: 发送方私钥（每次调用携带，实现无状态设计）
	//   - toAddress: 接收方地址（通过私钥内部计算得出发送方地址）
	//   - amount: 转账金额（字符串，支持小数，如"1.23456789"）
	//   - tokenID: 代币标识（""=原生代币，其他=合约地址）
	//   - memo: 转账备注（可选，显示在区块浏览器）
	//   - options: 高级控制选项（可选，省略=基础转账，传入=企业级高级功能）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 构建错误
	//
	// 💡 **调用示例**：
	//   - 基础转账：TransferAsset(ctx, privateKey, addr, "100.0", "", "转账备注")
	//   - 高级转账：TransferAsset(ctx, privateKey, addr, "100.0", "", "转账备注", &transferOptions)
	TransferAsset(ctx context.Context,
		senderPrivateKey []byte,
		toAddress string,
		amount string,
		tokenID string,
		memo string,
		options ...*types.TransferOptions,
	) ([]byte, error)

	// BatchTransfer 批量转账操作
	//
	// 🎯 **效率优化**：一次性处理多笔转账，降低手续费
	//
	// 📝 **适用场景**：
	//   - 工资发放、红包分发、空投发放
	//   - 批量退款、分润结算
	//
	// 📊 **优化特性**：
	//   ✓ UTXO批量选择优化  ✓ 手续费分摊计算
	//   ✓ 原子性保证        ✓ 失败全部回滚
	//
	// 💡 **参数说明**：
	//   - senderPrivateKey: 发送方私钥（每次调用携带，实现无状态设计）
	//   - transfers: 转账参数列表（最多1000笔）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 构建错误
	BatchTransfer(ctx context.Context,
		senderPrivateKey []byte,
		transfers []types.TransferParams,
	) ([]byte, error)

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                         📄  能力载体交易 (ResourceOutput)                                ║
	// ║                                                                                          ║
	// ║  处理静态资源的上链操作：文档、图片、视频、数据文件等                                         ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// DeployStaticResource 静态资源部署（支持基础和高级模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil或AccessPolicy=nil）：资源默认为部署者私有访问
	//   - 高级模式（提供AccessPolicy）：精确的权限控制和企业级资源管理
	//
	// 📝 **基础模式典型应用**：
	//   - 个人照片备份、重要文档存证（默认私有，仅部署者可访问）
	//   - 创作作品版权保护、学历证书存储（安全的个人资产管理）
	//
	// 📝 **高级模式支持的业务场景**：
	//   - 公开资源：设置为"public"策略，任何人可访问
	//   - 企业机密文档：多重签名访问控制，精确权限管理
	//   - 付费数字内容：按次付费下载，支持商业化运营
	//   - 团队协作文档：部门内共享访问，灵活的用户组管理
	//   - 定时发布内容：预设时间自动公开，生命周期控制
	//
	// 📊 **自动处理特性**：
	//   ✓ 文件哈希计算     ✓ 存储成本估算
	//   ✓ 重复检测         ✓ 格式验证
	//   ✓ 访问控制映射     ✓ 商业化配置
	//
	// 🔐 **统一权限控制模式**（通过AccessPolicy配置）：
	//   - 默认模式（无AccessPolicy）：仅部署者可访问，最安全
	//   - personal：个人私有访问，支持所有权转移
	//   - shared：团队共享访问，灵活用户组管理
	//   - commercial：商业化模式，按访问付费
	//   - enterprise：企业治理，多重签名和审批流程
	//   - public：完全公开访问，任何人可读取
	//
	// 💡 **参数说明**：
	//   - deployerPrivateKey: 部署者私钥（每次调用携带，实现无状态设计）
	//   - filePath: 本地文件路径（如："/path/to/document.pdf"）
	//   - name: 资源显示名称（如："我的毕业证书"）
	//   - description: 资源描述信息（如："清华大学计算机学士学位证书"）
	//   - tags: 资源分类标签（如：["证书", "教育", "个人"]）
	//   - options: 权限和部署选项（可选，省略=仅部署者私有，传入=精确权限控制）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 部署错误
	//
	// 💡 **调用示例**：
	//   - 私有部署：DeployStaticResource(ctx, privateKey, "/path/file.pdf", "证书", "学位证书", []string{"教育"})
	//   - 公开部署：DeployStaticResource(ctx, privateKey, "/path/file.pdf", "证书", "学位证书", []string{"教育"}, &ResourceDeployOptions{AccessPolicy: &AccessControlPolicy{PolicyType: "public"}})
	//   - 团队共享：DeployStaticResource(ctx, privateKey, "/path/file.pdf", "证书", "学位证书", []string{"教育"}, &deployOptions)
	DeployStaticResource(ctx context.Context,
		deployerPrivateKey []byte,
		filePath string,
		name string,
		description string,
		tags []string,
		options ...*types.ResourceDeployOptions,
	) ([]byte, error)

	// FetchStaticResourceFile 获取静态资源文件（支持权限控制和本地保存）
	//
	// 🎯 **功能说明**：
	//   - 根据内容哈希获取已部署的静态资源文件
	//   - 验证请求者权限（仅资源部署者可获取）
	//   - 支持自定义保存目录或使用默认目录
	//   - 自动处理文件名冲突（iOS风格递增）
	//
	// 📝 **权限控制**：
	//   - 仅资源的原始部署者（通过私钥验证）可以获取文件
	//   - 确保敏感资源不被未授权访问
	//
	// 📝 **文件保存策略**：
	//   - 如果指定目录，保存到指定路径
	//   - 如果目录为空，使用操作系统默认下载目录：
	//     * Windows: %USERPROFILE%\Downloads
	//     * macOS: ~/Downloads
	//     * Linux: ~/Downloads 或 ~/下载 (中文系统)
	//     * 其他: ./downloads (后备方案)
	//   - 文件名冲突时自动重命名：file.txt -> file(1).txt -> file(2).txt
	//
	// 💡 **参数说明**：
	//   - ctx: 上下文对象，用于超时控制和取消操作
	//   - contentHash: 资源内容的SHA-256哈希值（32字节）
	//   - requesterPrivateKey: 请求者私钥，用于权限验证
	//   - targetDir: 目标保存目录（可选，为空时使用默认目录）
	//
	// 💡 **返回值说明**：
	//   - string: 实际保存的文件路径
	//   - error: 操作错误（权限不足、资源不存在、磁盘空间不足等）
	//
	// 💡 **调用示例**：
	//   - 保存到系统默认下载目录：FetchStaticResourceFile(ctx, hash, privateKey, "")
	//   - 保存到指定目录：FetchStaticResourceFile(ctx, hash, privateKey, "/path/to/save")
	//   - 跨平台兼容：在 Windows 保存到 %USERPROFILE%\Downloads，在 macOS/Linux 保存到 ~/Downloads
	FetchStaticResourceFile(ctx context.Context,
		contentHash []byte,
		requesterPrivateKey []byte,
		targetDir string,
	) (string, error)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                         🔗  CONTRACT SERVICE INTERFACE                                       █
// █                                                                                              █
// █   智能合约服务：处理WASM合约的部署、调用和管理（分离独立服务）                                      █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████
//
// # ContractService 智能合约服务接口
//
// 🎯 **核心职责**：
//   - 智能合约部署：WASM字节码上链
//   - 合约方法调用：执行合约业务逻辑
//   - 合约状态管理：查询和监控
type ContractService interface {

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                          📄  基础合约操作                                                 ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// DeployContract 智能合约部署（支持基础和高级模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil或AccessPolicy=nil）：合约默认为公开可调用，任何人可访问
	//   - 高级模式（提供AccessPolicy）：精确的访问控制，支持私有合约和商业化运营
	//
	// 📝 **基础模式典型应用**：
	//   - DeFi协议部署、游戏逻辑合约
	//   - 投票治理、资产管理合约
	//
	// 📝 **高级模式支持的业务场景**：
	//   - 私有合约：企业内部业务逻辑（仅授权人员可调用）
	//   - 付费服务：按调用次数收费的合约服务
	//   - 多签治理：需要多方签名才能升级的关键合约
	//   - 定时上线：预设时间自动激活的合约功能
	//
	// 📊 **自动处理特性**：
	//   ✓ WASM格式验证    ✓ 执行费用消耗预估
	//   ✓ 安全性检查      ✓ 依赖关系分析
	//   ✓ 访问控制映射    ✓ 商业化配置
	//
	// 🔐 **统一权限控制模式**（通过AccessPolicy配置）：
	//   - 默认模式（无AccessPolicy）：公开合约，任何人可调用
	//   - personal：个人私有合约，仅部署者可调用
	//   - shared：团队共享合约，指定用户组可调用
	//   - commercial：商业化合约，按调用次数付费
	//   - enterprise：企业治理合约，多重签名和审批流程
	//
	// 💡 **参数说明**：
	//   - deployerPrivateKey: 部署者私钥（每次调用携带，实现无状态设计）
	//   - contractFilePath: 合约WASM文件路径（如："/path/to/contract.wasm"）
	//   - config: 执行配置（执行费用限制、权限等）
	//   - name: 合约显示名称（如："去中心化投票系统"）
	//   - description: 合约功能描述
	//   - options: 高级部署选项（可选，省略=基础部署，传入=企业级高级功能）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 部署错误
	//
	// 💡 **调用示例**：
	//   - 基础部署：DeployContract(ctx, privateKey, "/path/contract.wasm", config, "投票合约", "去中心化投票系统")
	//   - 高级部署：DeployContract(ctx, privateKey, "/path/contract.wasm", config, "投票合约", "去中心化投票系统", &deployOptions)
	DeployContract(ctx context.Context,
		deployerPrivateKey []byte,
		contractFilePath string,
		config *resource.ContractExecutionConfig,
		name string,
		description string,
		options ...*types.ResourceDeployOptions,
	) ([]byte, error)

	// CallContract 智能合约调用（支持基础和高级模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil）：用户直接调用合约方法执行业务逻辑
	//   - 高级模式（options!=nil）：企业级合约调用，支持委托、多签等控制
	//
	// 📝 **基础模式典型应用**：
	//   - 代币转账、NFT交易、投票参与
	//   - 查询余额、获取状态信息
	//
	// 📝 **高级模式支持的调用场景**：
	//   - 委托调用：代理其他用户执行合约方法
	//   - 多签调用：需要多方授权的重要操作
	//   - 定时调用：延迟执行的合约调用
	//   - 批量调用：优化执行费用费用的批量操作
	//
	// 📊 **自动处理特性**：
	//   ✓ 参数类型转换    ✓ 执行费用费用计算
	//   ✓ 状态一致性      ✓ 异常处理
	//   ✓ 授权检查        ✓ 执行优化
	//
	// 💡 **参数说明**：
	//   - callerPrivateKey: 调用者私钥（每次调用携带，实现无状态设计）
	//   - contractAddress: 合约地址（部署后返回的地址）
	//   - methodName: 方法名（如："transfer", "vote", "query"）
	//   - parameters: 方法参数（JSON格式，如：{"to": "0x123", "amount": "100"}）
	//   - 执行费用Limit: 执行费用限制（防止无限循环）
	//   - value: 发送的代币数量（可选，如："1.5"）
	//   - options: 高级调用选项（可选，省略=基础调用，传入=企业级高级功能）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 调用错误
	//
	// 💡 **调用示例**：
	//   - 基础调用：CallContract(ctx, privateKey, contractAddr, "transfer", params, 100000, "0")
	//   - 高级调用：CallContract(ctx, privateKey, contractAddr, "transfer", params, 100000, "0", &callOptions)
	CallContract(ctx context.Context,
		callerPrivateKey []byte,
		contractAddress string,
		methodName string,
		parameters map[string]interface{},
		执行费用Limit uint64,
		value string,
		options ...*types.TransferOptions,
	) ([]byte, error)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                           🤖  AI MODEL SERVICE INTERFACE                                     █
// █                                                                                              █
// █   AI模型服务：处理AI模型的部署、推理和商业化管理（分离独立服务）                                    █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████
//
// # AIModelService AI模型服务接口
//
// 🎯 **核心职责**：
//   - AI模型部署：机器学习模型上链
//   - 推理服务：执行AI推理计算
//   - 商业化管理：按次付费、订阅模式
type AIModelService interface {

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                          🧠  基础AI模型操作                                               ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// DeployAIModel AI模型部署（支持基础和商业化模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil）：AI开发者上传模型到区块链，公开可用
	//   - 商业化模式（options!=nil）：企业级AI模型部署和商业化，支持复杂商业模式
	//
	// 📝 **基础模式典型应用**：
	//   - 图像识别、文本分析、语音识别模型
	//   - 预测模型、推荐算法、决策树模型
	//
	// 📝 **商业化模式支持的场景**：
	//   - 按次付费：每次推理收费（如：图片识别0.01原生币/次）
	//   - 订阅模式：月费制无限使用（如：文本分析99原生币/月）
	//   - 分层定价：不同用户等级不同价格
	//   - 企业授权：内部团队共享使用高价值模型
	//
	// 📊 **自动处理特性**：
	//   ✓ 模型格式验证    ✓ 推理性能评估
	//   ✓ 存储优化        ✓ 版本管理
	//   ✓ 商业化配置      ✓ 收入分成设置
	//
	// 💰 **收入分成模式**：
	//   - 开发者获得80%收入，平台获得20%手续费
	//   - 支持质量保证和SLA服务承诺
	//
	// 💡 **参数说明**：
	//   - deployerPrivateKey: 部署者私钥（每次调用携带，实现无状态设计）
	//   - modelFilePath: AI模型文件路径（如："/path/to/resnet50.onnx"）
	//   - config: AI推理配置（GPU需求、内存限制等）
	//   - name: 模型显示名称（如："ResNet50图像分类器"）
	//   - description: 模型功能描述
	//   - options: 高级部署选项（可选，省略=基础部署，传入=商业化模式）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 部署错误
	//
	// 💡 **调用示例**：
	//   - 基础部署：DeployAIModel(ctx, privateKey, "/path/model.onnx", config, "图像识别", "ResNet50模型")
	//   - 商业化部署：DeployAIModel(ctx, privateKey, "/path/model.onnx", config, "图像识别", "ResNet50模型", &deployOptions)
	DeployAIModel(ctx context.Context,
		deployerPrivateKey []byte,
		modelFilePath string,
		config *resource.AIModelExecutionConfig,
		name string,
		description string,
		options ...*types.ResourceDeployOptions,
	) ([]byte, error)

	// InferAIModel AI推理执行（支持基础和高级模式）
	//
	// 🎯 **功能说明**：
	//   - 基础模式（options=nil）：用户使用AI模型进行推理计算
	//   - 高级模式（options!=nil）：企业级推理管理，支持委托、批量、付费等
	//
	// 📝 **基础模式典型应用**：
	//   - 上传图片进行识别、输入文本进行分析
	//   - 实时预测、数据处理
	//
	// 📝 **高级模式支持的推理场景**：
	//   - 批量推理：一次处理多个输入，优化费用
	//   - 委托推理：代理其他用户执行推理
	//   - 定时推理：延迟执行的推理任务
	//   - 付费推理：自动处理费用支付和结算
	//
	// 📊 **自动处理特性**：
	//   ✓ 输入数据预处理  ✓ 推理结果后处理
	//   ✓ 性能监控        ✓ 错误恢复
	//   ✓ 批量优化        ✓ 费用结算
	//
	// 💡 **参数说明**：
	//   - callerPrivateKey: 调用者私钥（每次调用携带，实现无状态设计）
	//   - modelAddress: 模型地址（部署后返回的地址）
	//   - inputData: 输入数据（基础模式：map[string]interface{}；高级模式：支持批量interface{}）
	//   - parameters: 推理参数（如：{"temperature": 0.7, "max_tokens": 100}）
	//   - options: 高级推理选项（可选，省略=基础推理，传入=企业级高级功能）
	//
	// 💡 **返回值说明**：
	//   - []byte: 未签名交易哈希
	//   - error: 推理错误
	//
	// 💡 **调用示例**：
	//   - 基础推理：InferAIModel(ctx, privateKey, modelAddr, inputData, params)
	//   - 批量推理：InferAIModel(ctx, privateKey, modelAddr, batchInputData, params, &inferOptions)
	InferAIModel(ctx context.Context,
		callerPrivateKey []byte,
		modelAddress string,
		inputData interface{},
		parameters map[string]interface{},
		options ...*types.TransferOptions,
	) ([]byte, error)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                       📋  TRANSACTION MANAGER INTERFACE                                      █
// █                                                                                              █
// █   交易管理器：处理交易生命周期管理（签名、提交、状态查询、多签协作）                                 █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████
//
// # TransactionManager 交易生命周期管理接口
//
// 🎯 **核心职责**：
//   - 交易签名：数字签名和多重签名
//   - 网络提交：将交易广播到区块链网络
//   - 状态跟踪：监控交易确认状态
//   - 多签协作：企业级多重签名工作流
type TransactionManager interface {

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                         ✍️  交易签名和提交                                                 ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// SignTransaction 签名交易
	//
	// 🎯 **最关键操作**：用户对交易进行数字签名授权
	//
	// 📝 **业务流程**：
	//   用户确认交易详情 → 私钥签名 → 生成可提交交易
	//
	// 🔐 **安全特性**：
	//   ✓ 私钥本地处理    ✓ 签名算法验证
	//   ✓ 交易完整性检查  ✓ 防重放攻击
	//
	// 💡 **参数说明**：
	//   - txHash: 未签名交易哈希（由各Service接口生成）
	//   - privateKey: 用户私钥（ECDSA secp256k1格式）
	//
	// 💡 **返回值说明**：
	//   - []byte: 已签名交易哈希
	//   - error: 签名错误
	SignTransaction(ctx context.Context,
		txHash []byte,
		privateKey []byte,
	) ([]byte, error)

	// SubmitTransaction 提交交易到网络
	//
	// 🎯 **网络广播**：将已签名交易提交到区块链网络
	//
	// 📝 **网络流程**：
	//   交易验证 → P2P网络广播 → 内存池排队 → 等待打包
	//
	// 📊 **自动处理**：
	//   ✓ 网络连接重试    ✓ 交易格式验证
	//   ✓ 手续费检查      ✓ 重复提交防护
	//
	// 💡 **参数说明**：
	//   - signedTxHash: 已签名交易哈希
	//
	// 💡 **返回值说明**：
	//   - error: 提交错误，nil表示成功
	SubmitTransaction(ctx context.Context,
		signedTxHash []byte,
	) error

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                         📊  交易状态查询                                                   ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// GetTransactionStatus 查询交易状态
	//
	// 🎯 **状态跟踪**：查询交易在区块链中的确认状态
	//
	// 📝 **状态类型**：
	//   - pending：在内存池中等待确认
	//   - confirmed：已被打包到区块
	//   - failed：执行失败（执行费用不足等）
	//
	// 💡 **参数说明**：
	//   - txHash: 交易哈希（签名前后均可）
	//
	// 💡 **返回值说明**：
	//   - types.TransactionStatusEnum: 交易状态
	//   - error: 查询错误
	GetTransactionStatus(ctx context.Context,
		txHash []byte,
	) (types.TransactionStatusEnum, error)

	// GetTransaction 查询完整交易信息
	//
	// 🎯 **详细查询**：获取交易的完整原始数据
	//
	// 📝 **返回信息**：
	//   - 交易输入输出详情、锁定条件和解锁证明
	//   - 执行结果和执行费用消耗
	//
	// 💡 **参数说明**：
	//   - txHash: 交易哈希
	//
	// 💡 **返回值说明**：
	//   - *transaction.Transaction: 完整的protobuf交易结构
	//   - error: 查询错误
	GetTransaction(ctx context.Context,
		txHash []byte,
	) (*transaction.Transaction, error)

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                         💰  费用估算和验证                                                 ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// EstimateTransactionFee 费用估算
	//
	// 🎯 **简单实用**：估算交易所需的基本费用
	//
	// 💡 **参数说明**：
	//   - txHash: 未签名交易哈希（用于大小计算）
	//
	// 💡 **返回值说明**：
	//   - uint64: 预估费用（以最小单位计算）
	//   - error: 估算错误
	EstimateTransactionFee(ctx context.Context,
		txHash []byte,
	) (uint64, error)

	// ValidateTransaction 交易验证
	//
	// 🎯 **简单验证**：验证交易是否有效
	//
	// 📝 **验证内容**：
	//   - 交易格式正确性 - 签名有效性 - 余额充足性 - 基本规则检查
	//
	// 💡 **参数说明**：
	//   - txHash: 交易哈希（签名前后均可）
	//
	// 💡 **返回值说明**：
	//   - bool: 验证结果（true=通过，false=不通过）
	//   - error: 验证过程中的错误
	ValidateTransaction(ctx context.Context,
		txHash []byte,
	) (bool, error)

	// ╔══════════════════════════════════════════════════════════════════════════════════════════╗
	// ║                         🤝  企业级多签协作                                                 ║
	// ╚══════════════════════════════════════════════════════════════════════════════════════════╝

	// StartMultiSigSession 创建多签会话
	//
	// 🎯 **企业协作**：启动企业级多重签名工作流
	//
	// 📝 **典型场景**：
	//   - 大额资金转移需要3-of-5高管签名
	//   - 重要合约部署需要技术+法务+财务签名
	//
	// 💡 **参数说明**：
	//   - requiredSignatures: 需要的签名数量（M，如：3）
	//   - authorizedSigners: 授权签名者地址列表（N个，如：5个地址）
	//   - expiryDuration: 会话过期时间（如：7天）
	//   - description: 会话描述（如："Q4季度资金划拨"）
	//
	// 💡 **返回值说明**：
	//   - string: 多签会话ID
	//   - error: 创建错误
	StartMultiSigSession(ctx context.Context,
		requiredSignatures uint32,
		authorizedSigners []string,
		expiryDuration time.Duration,
		description string,
	) (string, error)

	// AddSignatureToMultiSigSession 添加签名到多签会话
	//
	// 🎯 **异步签名**：参与者异步贡献签名
	//
	// 📝 **工作流程**：
	//   签名者收到通知 → 审查交易详情 → 提供数字签名 → 系统记录状态
	//
	// 💡 **参数说明**：
	//   - sessionID: 多签会话ID
	//   - signature: 签名数据（包含签名者身份）
	//
	// 💡 **返回值说明**：
	//   - error: 添加签名错误，nil表示成功
	AddSignatureToMultiSigSession(ctx context.Context,
		sessionID string,
		signature *types.MultiSigSignature,
	) error

	// GetMultiSigSessionStatus 查询多签会话状态
	//
	// 🎯 **进度跟踪**：查询多签会话的进展状态
	//
	// 📝 **状态信息**：
	//   - 已收集/需要签名数 - 会话状态 - 剩余有效时间
	//
	// 💡 **参数说明**：
	//   - sessionID: 多签会话ID
	//
	// 💡 **返回值说明**：
	//   - *types.MultiSigSession: 简化的会话状态信息
	//   - error: 查询错误
	GetMultiSigSessionStatus(ctx context.Context,
		sessionID string,
	) (*types.MultiSigSession, error)

	// FinalizeMultiSigSession 完成多签会话
	//
	// 🎯 **会话完成**：达到签名门限后，生成最终交易
	//
	// 📝 **完成条件**：
	//   - 收集到足够数量的有效签名 - 所有签名验证通过 - 会话在有效期内
	//
	// 💡 **参数说明**：
	//   - sessionID: 多签会话ID
	//
	// 💡 **返回值说明**：
	//   - []byte: 最终交易哈希
	//   - error: 完成错误
	FinalizeMultiSigSession(ctx context.Context,
		sessionID string,
	) ([]byte, error)
}

// ============================================================================
//                              详细使用示例和架构图解
// ============================================================================

// 🎯 **TransactionService设计理念与完整示例**
//
// **统一交易流程 + 业务友好接口 + 企业级功能**

// ==================== 示例1：基础转账流程 ====================
//
// 📊 **转账流程图**：
//
//     用户Alice                  WES系统                    网络节点
//        │                        │                           │
//        │ 1. 发起转账请求          │                           │
//        ├──TransferAsset()────────▶│                           │
//        │                        │ 2. 构建交易（选择UTXO）     │
//        │                        │ 3. 计算找零和费用          │
//        │ 4. 返回交易哈希         │                           │
//        ◄────txHash──────────────┤                           │
//        │                        │                           │
//        │ 5. 签名授权              │                           │
//        ├──SignTransaction()──────▶│                           │
//        │                        │ 6. 验证签名和交易          │
//        │ 7. 返回签名交易哈希      │                           │
//        ◄────signedHash──────────┤                           │
//        │                        │                           │
//        │ 8. 提交到网络           │                           │
//        ├──SubmitTransaction()────▶│ 9. 广播到网络            │
//        │                        ├─────────────────────────▶│
//        │                        │                          │ 10. 验证和打包
//        │ 11. 查询确认状态        │                          │
//        ├──GetTransactionStatus()─▶│ 12. 查询区块链状态       │
//        │                        ├─────────────────────────▶│
//        │ 13. 返回确认状态        │ 14. 返回区块信息          │
//        ◄────status──────────────◄─────────────────────────┤
//
// 🔧 **详细代码示例**：
//
//    ```go
//    // ===================== 步骤1：构建转账交易 =====================
//    //
//    // 🎯 用户意图：Alice要给Bob转账100个原生币，备注"午餐费"
//    // 💡 系统自动：选择合适的UTXO、计算找零、估算手续费
//    txHash, err := transactionService.TransferAsset(ctx,
//        "0x1234567890abcdef...bob",  // Bob的钱包地址
//        "100.0",                    // 转账金额：100个原生币
//        "",                         // 空字符串表示原生代币
//        "午餐费",                    // 转账备注
//    )
//    if err != nil {
//        log.Printf("构建交易失败: %v", err)
//        return
//    }
//    log.Printf("交易构建成功，交易哈希: %x", txHash)
//
//    // ===================== 步骤2：用户签名授权 =====================
//    //
//    // 🎯 用户操作：使用私钥对交易进行数字签名
//    // 💡 安全保证：私钥永不离开用户设备，确保资产安全
//    signedHash, err := transactionManager.SignTransaction(ctx,
//        txHash,        // 待签名的交易哈希
//        privateKey,    // Alice的私钥（32字节）
//    )
//    if err != nil {
//        log.Printf("交易签名失败: %v", err)
//        return
//    }
//    log.Printf("交易签名成功，签名交易哈希: %x", signedHash)
//
//    // ===================== 步骤3：提交到区块链网络 =====================
//    //
//    // 🎯 网络操作：将签名交易广播给网络中的所有节点
//    // 💡 网络处理：节点验证交易→加入内存池→等待矿工打包
//    err = transactionManager.SubmitTransaction(ctx, signedHash)
//    if err != nil {
//        log.Printf("交易提交失败: %v", err)
//        return
//    }
//    log.Printf("交易已成功提交到网络")
//
//    // ===================== 步骤4：查询交易确认状态 =====================
//    //
//    // 🎯 状态跟踪：实时监控交易是否被确认
//    // 💡 确认机制：pending→confirmed→finalized
//    status, err := transactionManager.GetTransactionStatus(ctx, signedHash)
//    if err != nil {
//        log.Printf("查询状态失败: %v", err)
//        return
//    }
//
//    switch status.Status {
//    case types.TxStatus_Pending:
//        log.Printf("交易待确认，在内存池中等待打包")
//    case types.TxStatus_Confirmed:
//        log.Printf("交易已确认，区块高度: %d, 确认数: %d",
//            status.BlockHeight, status.Confirmations)
//    case types.TxStatus_Failed:
//        log.Printf("交易执行失败: %s", status.ErrorMessage)
//    }
//    ```
//
//    **核心优势**：
//    - ✅ **用户友好**：只需提供接收地址和金额，系统处理所有技术细节
//    - ✅ **安全可靠**：私钥本地签名，交易哈希验证，多重安全保障
//    - ✅ **状态透明**：实时查询确认进度，完整的交易生命周期跟踪

// ==================== 示例2：智能合约完整工作流 ====================
//
// 📊 **合约部署和调用流程图**：
//
//     开发者                    WES系统                    合约引擎                    用户Alice
//        │                        │                           │                           │
//        │ 1. 部署合约请求          │                           │                           │
//        ├──DeployContract()───────▶│                           │                           │
//        │                        │ 2. 验证WASM字节码          │                           │
//        │                        │ 3. 生成合约地址            │                           │
//        │ 4. 返回部署交易哈希      │                           │                           │
//        ◄────deployHash──────────┤                           │                           │
//        │                        │                           │                           │
//        │ 5. 签名部署交易          │                           │                           │
//        ├──SignTransaction()──────▶│                           │                           │
//        │ 6. 提交部署交易          │                           │                           │
//        ├──SubmitTransaction()────▶│ 7. 部署到执行引擎         │                           │
//        │                        ├───────────────────────────▶│                           │
//        │                        │                          │ 8. 加载合约到沙箱          │
//        │                        │                          │                           │
//        │                        │                          │                           │
//        │                        │                          │ 9. 用户调用合约方法        │
//        │                        │                          │ ◄─────────────────────────┤
//        │                        │                          │                          │CallContract()
//        │                        │ 10. 执行合约逻辑          │                          │
//        │                        ◄──────────────────────────┤                          │
//        │                        │ 11. 返回执行结果          │                          │
//        │                        ├───────────────────────────────────────────────────▶│
//
// 🔧 **详细代码示例**：
//
//    ```go
//    // ===================== 阶段1：开发者部署智能合约 =====================
//    //
//    // 🎯 开发者场景：部署一个ERC-20代币合约"MyToken"
//    // 💡 技术要求：WASM字节码、执行配置、初始化参数
//
//    // 读取编译好的WASM合约文件
//    wasmBytes, err := os.ReadFile("./contracts/mytoken.wasm")
//    if err != nil {
//        log.Fatalf("无法读取合约文件: %v", err)
//    }
//
//    // 配置合约执行环境
//    contractConfig := &resource.ContractExecutionConfig{
//        Max执行费用Limit:    1000000,           // 最大执行费用限制
//        MaxMemoryPages: 256,               // 最大内存页数（256页 = 16MB）
//        MaxStackHeight: 1024,              // 最大调用栈深度
//        Timeout:        30,                // 执行超时时间（秒）
//        AllowedImports: []string{          // 允许的系统调用
//            "env.storage_read",
//            "env.storage_write",
//            "env.event_emit",
//        },
//    }
//
//    // 构建部署交易
//    deployHash, err := contractService.DeployContract(ctx,
//        wasmBytes,                         // 合约WASM字节码
//        contractConfig,                    // 执行配置
//        "MyToken",                         // 合约名称
//        "一个标准的ERC-20代币合约",          // 合约描述
//    )
//    if err != nil {
//        log.Printf("合约部署构建失败: %v", err)
//        return
//    }
//    log.Printf("部署交易构建成功: %x", deployHash)
//
//    // 开发者签名和提交
//    signedDeployHash, err := transactionManager.SignTransaction(ctx, deployHash, devPrivateKey)
//    err = transactionManager.SubmitTransaction(ctx, signedDeployHash)
//
//    // 等待部署确认
//    for {
//        status, _ := transactionManager.GetTransactionStatus(ctx, signedDeployHash)
//        if status.Status == types.TxStatus_Confirmed {
//            log.Printf("合约部署成功！合约地址: %s", status.ContractAddress)
//            break
//        }
//        time.Sleep(2 * time.Second) // 每2秒检查一次
//    }
//
//    // ===================== 阶段2：用户调用合约方法 =====================
//    //
//    // 🎯 用户场景：Alice调用MyToken合约的transfer方法，转100个代币给Bob
//    // 💡 智能合约：自动验证余额、执行转账、更新状态、发出事件
//
//    contractAddress := "0xabcdef...mytoken" // 从部署确认中获取
//
//    // 准备调用参数（JSON格式）
//    callParams := map[string]interface{}{
//        "to":     "0x1234567890abcdef...bob",  // 接收方地址
//        "amount": "100000000000000000000",     // 100个代币（18位小数）
//    }
//
//    // 构建合约调用交易
//    callHash, err := contractService.CallContract(ctx,
//        contractAddress,                   // 合约地址
//        "transfer",                        // 调用的方法名
//        callParams,                        // 方法参数
//        500000,                           // 执行费用限制
//        "0",                              // 不发送原生币
//    )
//    if err != nil {
//        log.Printf("合约调用构建失败: %v", err)
//        return
//    }
//
//    // 用户签名和提交
//    signedCallHash, err := transactionManager.SignTransaction(ctx, callHash, alicePrivateKey)
//    err = transactionManager.SubmitTransaction(ctx, signedCallHash)
//
//    // 查询执行结果
//    status, err := transactionManager.GetTransactionStatus(ctx, signedCallHash)
//    if status.Status == types.TxStatus_Confirmed {
//        log.Printf("代币转账成功！执行费用消耗: %d", status.ExecutionFeeUsed)
//        log.Printf("交易事件: %v", status.ExecutionResult["events"])
//    }
//    ```
//
//    **企业级特性**：
//    - ✅ **沙箱执行**：合约在隔离环境中运行，保证系统安全
//    - ✅ **执行费用计量**：精确的资源消耗计算，防止无限循环攻击
//    - ✅ **事件系统**：合约执行结果通过事件机制通知外部系统
//    - ✅ **状态证明**：所有状态变更都有密码学证明，确保可验证性

// ==================== 示例3：企业级多签工作流 ====================
//
// 📊 **多签协作流程图**：
//
//     财务经理                  CFO                    CEO                    WES系统
//        │                       │                       │                       │
//        │ 1. 创建多签会话        │                       │                       │
//        ├──StartMultiSigSession()─────────────────────────────────────────────▶│
//        │                       │                       │                       │ 2. 创建会话记录
//        │ 3. 返回会话ID          │                       │                       │ 3. 设置过期时间
//        ◄─────sessionID──────────────────────────────────────────────────────────┤
//        │                       │                       │                       │
//        │ 4. 财务经理签名        │                       │                       │
//        ├──AddSignature()────────────────────────────────────────────────────▶│
//        │                       │                       │                       │ 5. 验证签名1/3
//        │                       │ 6. CFO收到通知并签名    │                       │
//        │                       ├──AddSignature()────────────────────────────▶│
//        │                       │                       │                       │ 7. 验证签名2/3
//        │                       │                       │ 8. CEO收到通知并签名   │
//        │                       │                       ├──AddSignature()────▶│
//        │                       │                       │                       │ 9. 验证签名3/3
//        │                       │                       │                       │10. 会话完整，可执行
//        │ 11. 查询会话状态       │                       │                       │
//        ├──GetMultiSigStatus()──────────────────────────────────────────────▶│
//        │ 12. 返回"已满足条件"   │                       │                       │
//        ◄─────status══════════════════════════════════════════════════════════┤
//        │                       │                       │                       │
//        │ 13. 完成多签，生成交易  │                       │                       │
//        ├──FinalizeMultiSig()────────────────────────────────────────────────▶│
//        │                       │                       │                       │14. 合并签名生成交易
//        │ 15. 返回最终交易哈希   │                       │                       │15. 提交到区块链
//        ◄─────finalTxHash════════════════════════════════════════════════════┤
//
// 🔧 **详细代码示例**：
//
//    ```go
//    // ===================== 场景：企业转账500万原生币需要3人签名 =====================
//    //
//    // 🎯 企业场景：大额转账需要财务经理、CFO、CEO三人共同签名确认
//    // 💡 安全机制：任何单人都无法独自完成转账，需要协作授权
//
//    // ===================== 步骤1：财务经理发起多签会话 =====================
//    signerAddresses := []string{
//        "0xabc...finance_manager", // 财务经理地址
//        "0xdef...cfo",            // CFO地址
//        "0x123...ceo",            // CEO地址
//    }
//
//    sessionID, err := transactionManager.StartMultiSigSession(ctx,
//        3,                         // 需要3个签名
//        signerAddresses,           // 授权签名者列表
//        24 * time.Hour,           // 24小时过期
//        "向供应商支付货款500万原生币",   // 会话描述
//    )
//    if err != nil {
//        log.Printf("创建多签会话失败: %v", err)
//        return
//    }
//    log.Printf("多签会话创建成功，会话ID: %s", sessionID)
//
//    // 构建待签名的转账交易
//    transferHash, err := transactionService.TransferAsset(ctx,
//        "0x999...supplier",        // 供应商地址
//        "5000000.0",              // 500万原生币
//        "",                       // 原生代币
//        "供应商货款支付",           // 备注
//    )
//
//    // ===================== 步骤2：财务经理提交第一个签名 =====================
//    financeSignature := &types.MultiSigSignature{
//        SignerAddress: "0xabc...finance_manager",
//        Signature:     financeManagerSign(transferHash), // 使用财务经理私钥签名
//        Timestamp:     time.Now(),
//        Role:          "财务经理",
//    }
//
//    err = transactionManager.AddSignatureToMultiSigSession(ctx, sessionID, financeSignature)
//    if err != nil {
//        log.Printf("财务经理签名失败: %v", err)
//        return
//    }
//    log.Printf("财务经理签名成功 (1/3)")
//
//    // ===================== 步骤3：CFO收到通知后提交第二个签名 =====================
//    // 💡 实际应用中，CFO会收到邮件/短信通知，登录系统查看待签名交易
//
//    // CFO查看会话详情
//    sessionStatus, err := transactionManager.GetMultiSigSessionStatus(ctx, sessionID)
//    log.Printf("当前签名进度: %d/%d", sessionStatus.CurrentSignatures, sessionStatus.RequiredSignatures)
//    log.Printf("会话描述: %s", sessionStatus.Description)
//    log.Printf("剩余时间: %v", sessionStatus.ExpiresAt.Sub(time.Now()))
//
//    // CFO确认无误后签名
//    cfoSignature := &types.MultiSigSignature{
//        SignerAddress: "0xdef...cfo",
//        Signature:     cfoSign(transferHash), // 使用CFO私钥签名
//        Timestamp:     time.Now(),
//        Role:          "首席财务官",
//    }
//
//    err = transactionManager.AddSignatureToMultiSigSession(ctx, sessionID, cfoSignature)
//    log.Printf("CFO签名成功 (2/3)")
//
//    // ===================== 步骤4：CEO提交最终签名 =====================
//    ceoSignature := &types.MultiSigSignature{
//        SignerAddress: "0x123...ceo",
//        Signature:     ceoSign(transferHash), // 使用CEO私钥签名
//        Timestamp:     time.Now(),
//        Role:          "首席执行官",
//    }
//
//    err = transactionManager.AddSignatureToMultiSigSession(ctx, sessionID, ceoSignature)
//    log.Printf("CEO签名成功 (3/3)")
//
//    // ===================== 步骤5：系统自动完成多签交易 =====================
//    // 💡 当签名数量满足要求时，系统可以自动完成交易，也可以手动触发
//
//    finalTxHash, err := transactionManager.FinalizeMultiSigSession(ctx, sessionID)
//    if err != nil {
//        log.Printf("多签交易完成失败: %v", err)
//        return
//    }
//
//    log.Printf("多签交易成功完成！")
//    log.Printf("最终交易哈希: %x", finalTxHash)
//
//    // 查询最终确认状态
//    finalStatus, _ := transactionManager.GetTransactionStatus(ctx, finalTxHash)
//    log.Printf("交易状态: %s, 区块高度: %d", finalStatus.Status, finalStatus.BlockHeight)
//    ```
//
//    **企业级优势**：
//    - ✅ **权限分离**：不同角色的人员具有不同的签名权限
//    - ✅ **审计跟踪**：完整记录每个签名者的操作时间和身份
//    - ✅ **灵活配置**：支持m-of-n多签，适应不同企业治理结构
//    - ✅ **时间控制**：自动过期机制，避免长期悬挂的签名会话

// ==================== 示例4：AI模型推理完整流程 ====================
//
// 📊 **AI模型部署和推理流程图**：
//
//     AI开发者                  WES系统                    AI执行引擎                 用户Alice
//        │                        │                           │                           │
//        │ 1. 部署AI模型           │                           │                           │
//        ├──DeployAIModel()───────▶│                           │                           │
//        │                        │ 2. 验证模型格式            │                           │
//        │                        │ 3. 上传到存储层            │                           │
//        │ 4. 返回部署交易哈希      │                           │                           │
//        ◄────deployHash──────────┤                           │                           │
//        │                        │                           │                           │
//        │ 5. 签名并提交           │                           │                           │
//        ├──SignAndSubmit()────────▶│ 6. 加载到AI引擎           │                           │
//        │                        ├───────────────────────────▶│                           │
//        │                        │                          │ 7. 模型预热和优化          │
//        │                        │                          │                           │
//        │                        │                          │                           │
//        │                        │                          │ 8. 用户发起推理请求        │
//        │                        │                          │ ◄─────────────────────────┤
//        │                        │                          │                          │InferAIModel()
//        │                        │ 9. 执行推理计算           │                          │
//        │                        ◄──────────────────────────┤                          │
//        │                        │ 10. 返回推理结果          │                          │
//        │                        ├───────────────────────────────────────────────────▶│
//
// 🔧 **详细代码示例**：
//
//    ```go
//    // ===================== 阶段1：AI开发者部署图像分类模型 =====================
//    //
//    // 🎯 AI场景：部署一个ONNX格式的图像分类模型"ImageNet ResNet-50"
//    // 💡 技术要求：ONNX模型文件、输入输出规范、推理配置
//
//    // 读取训练好的ONNX模型文件
//    modelBytes, err := os.ReadFile("./models/resnet50.onnx")
//    if err != nil {
//        log.Fatalf("无法读取模型文件: %v", err)
//    }
//
//    // 配置AI模型执行环境
//    modelConfig := &resource.AIModelExecutionConfig{
//        Format:         "onnx",                    // 模型格式
//        Framework:      "onnxruntime",             // 推理框架
//        InputShape:     []int64{1, 3, 224, 224},  // 输入张量形状 [批次, 通道, 高, 宽]
//        OutputShape:    []int64{1, 1000},         // 输出张量形状 [批次, 类别数]
//        InputNames:     []string{"input"},         // 输入张量名称
//        OutputNames:    []string{"output"},        // 输出张量名称
//        MaxBatchSize:   32,                       // 最大批处理大小
//        MaxMemoryMB:    2048,                     // 最大内存使用 (2GB)
//        TimeoutSec:     30,                       // 推理超时时间
//        OptimizationLevel: 2,                     // 优化级别 (0-3)
//    }
//
//    // 构建AI模型部署交易
//    deployHash, err := aiModelService.DeployAIModel(ctx,
//        modelBytes,                              // ONNX模型字节
//        modelConfig,                            // 执行配置
//        "ResNet50图像分类器",                     // 模型名称
//        "基于ImageNet训练的ResNet-50图像分类模型", // 模型描述
//    )
//    if err != nil {
//        log.Printf("AI模型部署构建失败: %v", err)
//        return
//    }
//    log.Printf("AI模型部署交易构建成功: %x", deployHash)
//
//    // AI开发者签名和提交
//    signedDeployHash, err := transactionManager.SignTransaction(ctx, deployHash, aiDevPrivateKey)
//    err = transactionManager.SubmitTransaction(ctx, signedDeployHash)
//
//    // 等待模型部署和预热完成
//    for {
//        status, _ := transactionManager.GetTransactionStatus(ctx, signedDeployHash)
//        if status.Status == types.TxStatus_Confirmed {
//            log.Printf("AI模型部署成功！模型地址: %s", status.ContractAddress)
//            log.Printf("模型预热耗时: %v", status.ExecutionResult["warmup_time"])
//            break
//        }
//        time.Sleep(3 * time.Second) // AI模型加载较慢，每3秒检查一次
//    }
//
//    // ===================== 阶段2：用户使用AI模型进行图像分类 =====================
//    //
//    // 🎯 用户场景：Alice上传一张猫的照片，让AI模型识别是什么动物
//    // 💡 AI推理：图像预处理→神经网络推理→后处理→返回分类结果
//
//    modelAddress := "0x789...resnet50model" // 从部署确认中获取
//
//    // 准备推理输入数据（实际应用中需要图像预处理）
//    // 这里简化为已经预处理过的张量数据
//    inputData := map[string]interface{}{
//        "input": []float32{
//            // 224x224x3的图像像素数据（已标准化到[-1,1]区间）
//            // 实际应用中这里是图像预处理后的150528个浮点数
//            0.485, 0.456, 0.406, /* ... 更多像素数据 ... */
//        },
//    }
//
//    // 设置推理参数
//    inferParams := map[string]interface{}{
//        "top_k":      5,    // 返回前5个最可能的分类
//        "confidence": 0.1,  // 最低置信度阈值
//        "batch_size": 1,    // 单张图片推理
//    }
//
//    // 构建AI推理交易
//    inferHash, err := aiModelService.InferAIModel(ctx,
//        modelAddress,                          // AI模型地址
//        inputData,                            // 输入图像数据
//        inferParams,                          // 推理参数
//    )
//    if err != nil {
//        log.Printf("AI推理构建失败: %v", err)
//        return
//    }
//
//    // 用户签名和提交推理请求
//    signedInferHash, err := transactionManager.SignTransaction(ctx, inferHash, alicePrivateKey)
//    err = transactionManager.SubmitTransaction(ctx, signedInferHash)
//
//    // 等待AI推理完成
//    for {
//        status, _ := transactionManager.GetTransactionStatus(ctx, signedInferHash)
//        if status.Status == types.TxStatus_Confirmed {
//            // 解析推理结果
//            results := status.ExecutionResult["predictions"].([]map[string]interface{})
//
//            log.Printf("🎉 AI推理完成！识别结果:")
//            for i, result := range results {
//                className := result["class"].(string)
//                confidence := result["confidence"].(float64)
//                log.Printf("  %d. %s (置信度: %.2f%%)", i+1, className, confidence*100)
//            }
//
//            log.Printf("推理耗时: %v", status.ExecutionResult["inference_time"])
//            log.Printf("使用GPU: %v", status.ExecutionResult["gpu_used"])
//            break
//        }
//        time.Sleep(1 * time.Second) // AI推理通常很快，每秒检查一次
//    }
//    ```
//
//    **AI推理特色功能**：
//    - ✅ **多格式支持**：ONNX、TensorFlow、PyTorch等主流模型格式
//    - ✅ **GPU加速**：自动检测并使用可用的GPU资源进行推理加速
//    - ✅ **批处理优化**：支持批量推理，提高吞吐量和资源利用率
//    - ✅ **结果验证**：AI推理结果通过零知识证明验证，确保计算正确性

// 🎯 **接口职责边界**：
//
// - **TransactionService**: 基础交易操作（转账、静态资源）
// - **ContractService**: 智能合约专用（部署、调用）
// - **AIModelService**: AI模型专用（部署、推理）
// - **TransactionManager**: 交易生命周期管理（签名、提交、查询、多签）
//
// 💡 **架构优势**：
//
// - **业务友好**: 简化参数，隐藏技术复杂性
// - **类型安全**: 基于protobuf的强类型定义
// - **企业级**: 完整的多签会话和治理流程
// - **可扩展**: 清晰的职责分离，便于功能扩展
