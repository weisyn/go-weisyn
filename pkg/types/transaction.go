// Package types 提供区块链交易相关的业务数据结构
//
// 🎯 **设计理念 - 简洁实用原则**
//
// 本文件遵循"简洁实用"的设计原则，只保留真正有价值的业务数据结构：
// - ✅ **有明确业务价值**：每个类型都解决具体业务问题
// - ✅ **可实现性强**：所有字段都有明确的数据来源
// - ✅ **用户友好**：提供直观易懂的业务抽象
// - ✅ **避免过度设计**：拒绝无用的评分、建议、统计等伪功能
//
// 🏗️ **架构分层清晰**
//
// - **pb层**：标准化的protobuf交易结构（核心协议定义）
// - **types层**：业务友好的扩展类型（本文件，补充pb层）
// - **interface层**：面向用户的TransactionService接口
//
// 📋 **核心类型价值**
//
// - **TransactionStatusEnum**：简洁的交易状态枚举
// - **TransferParams**：基础转账参数封装
// - **MultiSigSession**：多签会话状态管理
// - **各种Options**：高级功能的业务友好封装
//
// ⚠️ **设计反思与避坑指南**
//
// 本文件体现了"从过度设计到简洁实用"的重构思路，旨在为后续开发提供参考：
//
// 🚫 **已摒弃的错误设计模式**：
// - **虚假评分系统**：ValidationScore、Complexity等评分字段，实际无评价标准
// - **空想建议功能**：Suggestions、优化建议等字段，缺乏算法和数据支撑
// - **无用统计信息**：ValidationTime、NetworkCongestion等，用户不关心也无价值
// - **技术细节泄露**：过度暴露内部实现，如ValidationItems、详细错误分类
//
// ✅ **正确的设计理念**：
// - **用户价值导向**：每个字段都要解决真实的业务问题
// - **可实现性原则**：必须有明确可靠的数据来源
// - **简洁性原则**：避免为了"看起来完整"而添加无用字段
// - **边界清晰**：区分用户关心的业务信息和系统内部实现细节
//
// 💡 **三问题判断法**：
// 设计每个字段都问三个问题：
// 1. 用户真的需要这个信息吗？（业务价值）
// 2. 系统有可靠的数据来源吗？（可实现性）
// 3. 这个功能能够稳定实现吗？（维护性）
//
// 只有三个答案都是"是"，才应该保留该字段。
// 宁可功能简单可靠，也不要复杂而虚假。
package types

import (
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ================================================================================================
// 🎯 第一部分：基础交易参数类型
// ================================================================================================

// TransferParams 转账参数
//
// 🎯 **简洁实用的转账参数封装**
//
// 将复杂的EUTXO转账操作抽象为用户友好的参数结构。
// 隐藏区块链技术细节，提供直观的转账接口。
//
// 💡 **核心价值**:
// - ✅ **用户友好**：地址、金额、备注，符合直觉
// - ✅ **精度安全**：使用字符串避免浮点数精度问题
// - ✅ **灵活支付**：支持原生币和任意合约代币
// - ✅ **可追溯性**：支持转账备注便于记账和审计
//
// 📝 **典型应用场景**:
// - 个人转账：朋友间转账、商家付款
// - 企业财务：工资发放、供应商付款、股东分红
// - DeFi操作：流动性提供、代币交换、收益提取
//
// ⚠️ **设计原则**：
// 每个字段都有明确的业务含义，不包含技术实现细节。
// 用户只需要关心"给谁转多少钱"，系统自动处理底层逻辑。
type TransferParams struct {
	ToAddress string `json:"to_address"` // 接收方地址（十六进制字符串）
	Amount    string `json:"amount"`     // 转账金额（字符串，支持小数）
	TokenID   string `json:"token_id"`   // 代币标识（""表示原生代币，其他为合约地址）
	Memo      string `json:"memo"`       // 转账备注（可选）
}

// ================================================================================================
// 🎯 第二部分：交易状态和查询结果
// ================================================================================================

// TransactionStatus 交易状态信息
//
// 🎯 **用于交易状态查询的标准响应结构**
//
// 提供交易在区块链中的完整状态信息，包括确认状态、
// 执行结果、执行费用消耗等详细信息。
//
// 📝 **状态流转**:
// pending → confirmed/failed
//
// 💡 **字段说明**:
// - Hash: 交易哈希标识符
// - Status: 交易当前状态
// - BlockHeight: 交易所在区块高度（仅confirmed状态）
// - Confirmations: 确认区块数
// - ExecutionFeeUsed: 执行费用消耗（仅可执行交易）
// - ExecutionResult: 执行结果（仅合约/AI推理）
// TransactionStatusEnum 交易状态枚举
//
// 🎯 **简洁实用的交易状态定义**
//
// 采用最简化的状态枚举，只包含用户真正关心的三种状态。
// 避免过度复杂的状态分类，确保状态语义清晰直观。
//
// 💡 **设计价值**：
// - ✅ **用户导向**：只提供用户需要的状态信息
// - ✅ **语义清晰**：每个状态都有明确的业务含义
// - ✅ **易于理解**：不需要区块链技术背景也能看懂
// - ✅ **稳定可靠**：状态转换逻辑简单，不易出错
//
// 📝 **状态说明**：
// - **pending**：交易已提交，等待矿工打包确认
// - **confirmed**：交易已成功确认，写入区块链
// - **failed**：交易执行失败，不会产生任何状态变更
type TransactionStatusEnum string

const (
	TxStatus_Pending   TransactionStatusEnum = "pending"   // 等待确认（在内存池中）
	TxStatus_Confirmed TransactionStatusEnum = "confirmed" // 已确认（已入块）
	TxStatus_Failed    TransactionStatusEnum = "failed"    // 执行失败
)

// TransactionReceipt 交易执行回执
//
// 🎯 **已确认交易的完整执行信息**
//
// 包含交易执行后的完整状态变更信息，用于审计和分析。
//
// 📝 **使用场景**:
// - 合约调用结果查询
// - AI推理结果获取
// - 状态变更审计
// - 事件日志查询
type TransactionReceipt struct {
	Hash             []byte                `json:"hash"`              // 交易哈希
	Status           TransactionStatusEnum `json:"status"`            // 最终状态
	BlockHeight      uint64                `json:"block_height"`      // 区块高度
	BlockHash        []byte                `json:"block_hash"`        // 区块哈希
	TransactionIndex uint32                `json:"transaction_index"` // 区块内交易索引

	// 执行信息
	ExecutionFeeUsed  uint64 `json:"execution_fee_used"`  // 实际执行费用消耗
	ExecutionFeeLimit uint64 `json:"execution_fee_limit"` // 执行费用限制
	ExecutionFeePrice uint64 `json:"execution_fee_price"` // 执行费用价格

	// 结果数据
	ExecutionResult map[string]interface{} `json:"execution_result,omitempty"` // 执行结果
	Events          []Event                `json:"events,omitempty"`           // 触发的事件
	StateChanges    []StateChange          `json:"state_changes,omitempty"`    // 状态变更

	// 时间信息
	ExecutionTime time.Duration `json:"execution_time"` // 执行耗时
	ConfirmedAt   time.Time     `json:"confirmed_at"`   // 确认时间
}

// Event 交易触发的事件
//
// 🎯 **记录交易执行过程中触发的事件**
type Event struct {
	Address string                 `json:"address"` // 事件来源地址（合约地址）
	Topics  [][]byte               `json:"topics"`  // 事件主题（索引参数）
	Data    []byte                 `json:"data"`    // 事件数据（非索引参数）
	Decoded map[string]interface{} `json:"decoded"` // 解码后的事件数据
}

// StateChange 状态变更记录
//
// 🎯 **记录交易导致的状态变化**
type StateChange struct {
	Address   string `json:"address"`   // 状态变更的地址
	Key       []byte `json:"key"`       // 状态键
	OldValue  []byte `json:"old_value"` // 变更前的值
	NewValue  []byte `json:"new_value"` // 变更后的值
	Operation string `json:"operation"` // 操作类型（create/update/delete）
}

// ================================================================================================
// 🎯 第三部分：多签会话管理
// ================================================================================================

// MultiSigSession 多重签名会话
//
// 🎯 **企业级协作交易的核心枢纽**
//
// 将复杂的多签流程简化为直观的会话状态管理，让企业用户
// 轻松跟踪"谁签了、还差几个、什么时候到期"等核心信息。
//
// 💡 **核心价值**：
// - ✅ **协作简化**：复杂多签流程一目了然
// - ✅ **异步友好**：支持跨时空的签名收集
// - ✅ **状态透明**：清晰的进度跟踪和监控
// - ✅ **安全防护**：自动过期机制防止遗留风险
//
// 📝 **典型企业场景**：
// - **财务审批**：大额转账需要CFO+CEO联合签名
// - **投资决策**：重大投资需要董事会多人批准
// - **供应商付款**：采购付款需要多部门联合审批
// - **薪资发放**：批量工资发放需要HR+财务双签名
//
// ⚠️ **简化设计原则**：
// 只保留用户真正关心的信息，去除技术实现细节。
// 让企业管理者专注于业务审批，而非区块链技术。
type MultiSigSession struct {
	SessionID          string `json:"session_id"`          // 会话唯一标识符
	RequiredSignatures uint32 `json:"required_signatures"` // 需要的签名数量（M）
	CurrentSignatures  uint32 `json:"current_signatures"`  // 当前已收集的签名数量
	Status             string `json:"status"`              // 会话状态（"active", "completed", "expired"）

	// 基本时间信息
	ExpiryTime time.Time `json:"expiry_time"` // 过期时间

	// 完成时的结果
	FinalTransactionHash []byte `json:"final_tx_hash,omitempty"` // 最终交易哈希（完成时）
}

// MultiSigSignature 多签签名条目
//
// 🎯 **多签会话中的单个签名记录**
//
// 记录单个参与者的签名信息，包括签名数据、身份验证、时间戳等。
//
// 💡 **安全特性**:
// - 完整的身份验证信息
// - 签名时间戳防重放
// - 支持多种签名算法
// - 可选的签名者角色
type MultiSigSignature struct {
	SignerAddress      string                         `json:"signer_address"`      // 签名者地址
	PublicKey          []byte                         `json:"public_key"`          // 签名者公钥
	Signature          []byte                         `json:"signature"`           // 签名数据
	SignatureAlgorithm transaction.SignatureAlgorithm `json:"signature_algorithm"` // 签名算法
	SignedAt           time.Time                      `json:"signed_at"`           // 签名时间
	SignerRole         string                         `json:"signer_role"`         // 签名者角色（可选）
}

// ================================================================================================
// 🎯 第三部分：批量操作和高级功能
// ================================================================================================

// BatchTransferResult 批量转账结果
//
// 🎯 **批量转账操作的执行结果**
//
// 记录批量转账中每笔交易的执行情况，便于用户跟踪和处理失败项。
type BatchTransferResult struct {
	TotalCount   int `json:"total_count"`   // 总转账数量
	SuccessCount int `json:"success_count"` // 成功数量
	FailureCount int `json:"failure_count"` // 失败数量

	// 详细结果
	Results []SingleTransferResult `json:"results"` // 各笔转账结果

	// 汇总信息
	TotalAmount string `json:"total_amount"` // 转账总金额
	TotalFee    uint64 `json:"total_fee"`    // 总手续费

	// 时间信息
	ProcessingTime time.Duration `json:"processing_time"` // 处理耗时
	SubmittedAt    time.Time     `json:"submitted_at"`    // 提交时间
}

// SingleTransferResult 单笔转账结果
//
// 🎯 **批量转账中单笔交易的结果**
type SingleTransferResult struct {
	Index    int    `json:"index"`     // 在批量中的索引
	TxHash   []byte `json:"tx_hash"`   // 交易哈希（成功时）
	Status   string `json:"status"`    // 状态（success/failed）
	ErrorMsg string `json:"error_msg"` // 错误信息（失败时）

	// 转账信息
	ToAddress string `json:"to_address"` // 接收方地址
	Amount    string `json:"amount"`     // 转账金额
	Fee       uint64 `json:"fee"`        // 手续费
}

// ================================================================================================
// 🎯 第七部分：高级锁定控制参数（业务友好抽象）
// ================================================================================================

// TransferOptions 转账高级选项
//
// 🎯 **业务友好的高级转账控制**
//
// 将底层的7种锁定机制抽象为用户容易理解的业务概念。
// 用户只需要设置业务策略，系统自动映射到对应的锁定机制。
//
// 📋 **支持的业务场景**：
// - 个人转账：默认SingleKeyLock
// - 企业多签：自动创建MultiKeyLock
// - 付费使用：自动创建ContractLock
// - 临时授权：自动创建DelegationLock
// - 定时发布：自动创建TimeLock
// - 分阶段释放：自动创建HeightLock
// - 银行级安全：自动创建ThresholdLock
//
// 💡 **设计理念**：
// - 业务概念优先，技术细节隐藏
// - 渐进式复杂度，简单场景保持简单
// - 参数化扩展，不破坏现有接口
type TransferOptions struct {
	// 访问控制策略（映射到不同的锁定机制）
	AccessPolicy *AccessControlPolicy `json:"access_policy,omitempty"`

	// 时间控制策略
	TimingControl *TimingControlPolicy `json:"timing_control,omitempty"`

	// 授权模式
	AuthMode *AuthorizationMode `json:"auth_mode,omitempty"`

	// 企业级选项
	Enterprise *EnterpriseOptions `json:"enterprise,omitempty"`

	// 费用控制
	FeeControl *FeeControlOptions `json:"fee_control,omitempty"`

	// 合规和审计
	Compliance *ComplianceOptions `json:"compliance,omitempty"`
}

// ResourceDeployOptions 资源部署高级选项
//
// 🎯 **企业级资源部署控制**
//
// 专门用于资源（合约、AI模型、文件）部署的高级控制选项。
// 支持复杂的访问控制、商业化模式、企业级治理。
//
// 💡 **核心价值**：
// - 支持付费使用的商业模式
// - 支持企业内部权限管理
// - 支持临时权限租借
// - 支持多层级审批流程
type ResourceDeployOptions struct {
	// 访问控制策略
	AccessPolicy *AccessControlPolicy `json:"access_policy,omitempty"`

	// 商业模式配置
	BusinessModel *BusinessModelOptions `json:"business_model,omitempty"`

	// 权限管理
	PermissionModel *PermissionModelOptions `json:"permission_model,omitempty"`

	// 生命周期控制
	LifecycleControl *LifecycleControlOptions `json:"lifecycle_control,omitempty"`

	// 企业级功能
	Enterprise *EnterpriseResourceOptions `json:"enterprise,omitempty"`
}

// ================================================================================================
// 🎯 第八部分：查询和分析参数
// ================================================================================================

// TransactionQuery 交易查询参数
//
// 🎯 **灵活的交易查询条件**
//
// 支持多维度的交易查询，包括时间范围、地址过滤、
// 交易类型、状态等条件的组合查询。
type TransactionQuery struct {
	// 基础过滤条件
	Address     string `json:"address,omitempty"`      // 相关地址（发送方或接收方）
	FromAddress string `json:"from_address,omitempty"` // 发送方地址
	ToAddress   string `json:"to_address,omitempty"`   // 接收方地址

	// 时间范围
	StartTime time.Time `json:"start_time,omitempty"` // 开始时间
	EndTime   time.Time `json:"end_time,omitempty"`   // 结束时间

	// 区块范围
	StartHeight uint64 `json:"start_height,omitempty"` // 开始区块高度
	EndHeight   uint64 `json:"end_height,omitempty"`   // 结束区块高度

	// 状态和类型
	Status  TransactionStatusEnum `json:"status,omitempty"`   // 交易状态
	TxTypes []string              `json:"tx_types,omitempty"` // 交易类型列表

	// 分页参数
	Limit  int `json:"limit"`  // 返回数量限制
	Offset int `json:"offset"` // 偏移量

	// 排序参数
	OrderBy  string `json:"order_by"`  // 排序字段
	OrderDir string `json:"order_dir"` // 排序方向（asc/desc）
}

// ================================================================================================
// 🎯 第四部分：访问控制策略定义
// ================================================================================================

// AccessControlPolicy 访问控制策略
//
// 🎯 **统一的访问控制抽象**
//
// 将7种锁定机制抽象为5种用户理解的访问控制策略：
// - personal: 个人私有（SingleKeyLock）
// - shared: 多人共享（MultiKeyLock）
// - commercial: 商业付费（ContractLock）
// - enterprise: 企业治理（ThresholdLock + DelegationLock）
// - public: 公开访问（无锁定，任何人可访问）
type AccessControlPolicy struct {
	PolicyType string `json:"policy_type"` // "personal", "shared", "commercial", "enterprise", "public"

	// 个人访问配置（映射到SingleKeyLock）
	Personal *PersonalAccessConfig `json:"personal,omitempty"`

	// 共享访问配置（映射到MultiKeyLock）
	SharedAccess *SharedAccessConfig `json:"shared_access,omitempty"`

	// 商业化配置（映射到ContractLock）
	Commercial *CommercialAccessConfig `json:"commercial,omitempty"`

	// 企业级配置（映射到ThresholdLock）
	Enterprise *EnterpriseAccessConfig `json:"enterprise,omitempty"`

	// 公开访问配置（无需特殊锁，任何人可访问）
	Public *PublicAccessConfig `json:"public,omitempty"`
}

// PersonalAccessConfig 个人访问配置
//
// 🎯 **映射到SingleKeyLock的个人私有访问**
type PersonalAccessConfig struct {
	OwnerOnly    bool   `json:"owner_only"`            // 仅所有者可访问
	Description  string `json:"description,omitempty"` // 访问控制描述
	Transferable bool   `json:"transferable"`          // 是否可转移所有权
}

// SharedAccessConfig 共享访问配置
//
// 🎯 **映射到MultiKeyLock的多人共享访问**
//
// 📝 **典型应用场景**：
// - 团队协作资源（AI模型、合约、文档）
// - 企业部门内共享资源
// - 多用户联合拥有的资产
type SharedAccessConfig struct {
	AuthorizedUsers []string `json:"authorized_users"` // 授权用户地址列表
	RequiredSigners uint32   `json:"required_signers"` // 需要的签名数量（1=任一用户，N=需要N个签名）
	Description     string   `json:"description"`      // 共享策略描述
	AllowAddUsers   bool     `json:"allow_add_users"`  // 是否允许动态添加用户
	MaxUsers        uint32   `json:"max_users"`        // 最大用户数量限制
}

// CommercialAccessConfig 商业访问配置
//
// 🎯 **映射到ContractLock的付费使用模式**
//
// 📝 **商业模式支持**：
// - 按次付费：每次访问需要支付费用
// - 订阅制：按时间周期付费
// - 配额制：购买使用配额
// - 分层定价：不同用户等级不同价格
type CommercialAccessConfig struct {
	PriceModel      string `json:"price_model"`      // "per_use", "subscription", "quota", "tiered"
	PricePerUse     string `json:"price_per_use"`    // 按次付费价格
	SubscriptionFee string `json:"subscription_fee"` // 订阅费用（月费/年费）
	PaymentToken    string `json:"payment_token"`    // 支付代币类型（""=原生币）
	AccessContract  string `json:"access_contract"`  // 访问控制合约地址

	// 配额控制
	QuotaLimit     uint64 `json:"quota_limit"`      // 配额限制（每用户）
	QuotaPeriod    string `json:"quota_period"`     // 配额周期（"daily", "monthly"）
	FreeTrialQuota uint64 `json:"free_trial_quota"` // 免费试用配额

	// 分层定价
	TierPricing    []TierPricing `json:"tier_pricing,omitempty"` // 分层价格
	DiscountPolicy string        `json:"discount_policy"`        // 折扣策略
}

// TierPricing 分层定价
//
// 🎯 **支持不同用户等级的差异化定价**
type TierPricing struct {
	TierName     string `json:"tier_name"`      // 等级名称（"basic", "premium", "enterprise"）
	MinUsage     uint64 `json:"min_usage"`      // 最小使用量
	PricePerUnit string `json:"price_per_unit"` // 单价
	Description  string `json:"description"`    // 等级描述
}

// EnterpriseAccessConfig 企业访问配置
//
// 🎯 **映射到ThresholdLock的企业级治理**
//
// 📝 **企业级特性**：
// - 门限签名：需要多个高级管理人员联合签名
// - 审批流程：多级审批工作流
// - 合规检查：自动执行合规规则
// - 风险控制：风险评估和限额控制
type EnterpriseAccessConfig struct {
	SecurityLevel   string   `json:"security_level"`    // "standard", "high", "critical"
	RequiredSigners uint32   `json:"required_signers"`  // 需要的签名数量
	AuthorizedRoles []string `json:"authorized_roles"`  // 授权角色列表（"CEO", "CFO", "CTO"）
	ApprovalFlow    []string `json:"approval_flow"`     // 审批流程节点
	ComplianceRules []string `json:"compliance_rules"`  // 合规规则列表
	RiskAssessment  bool     `json:"risk_assessment"`   // 是否启用风险评估
	AuditTrailLevel string   `json:"audit_trail_level"` // 审计跟踪级别
}

// PublicAccessConfig 公开访问配置
//
// 🎯 **完全公开的资源访问**
//
// 📝 **典型应用场景**：
// - 开源软件发布、技术文档分享
// - 公益项目资料、教育资源分享
// - 营销材料、品牌宣传内容
// - 公共数据集、研究成果发布
type PublicAccessConfig struct {
	Description  string `json:"description,omitempty"` // 公开访问描述
	IndexPublic  bool   `json:"index_public"`          // 是否允许搜索引擎索引
	DownloadFree bool   `json:"download_free"`         // 是否允许免费下载
	Attribution  string `json:"attribution,omitempty"` // 署名要求
}

// ================================================================================================
// 🎯 第十部分：时间控制和授权模式定义
// ================================================================================================

// TimingControlPolicy 时间控制策略
//
// 🎯 **时间相关的锁定控制**
//
// 支持多种时间控制模式：
// - 延迟发布：在指定时间后才能访问（映射到TimeLock）
// - 锁定期：锁定一段时间后才能操作（映射到TimeLock）
// - 分阶段释放：按区块高度分批释放（映射到HeightLock）
// - 定时任务：定时执行特定操作
type TimingControlPolicy struct {
	ControlType string `json:"control_type"` // "delay", "lock_period", "staged", "scheduled"

	// 延迟发布（映射到TimeLock）
	DelayedRelease *DelayedReleaseConfig `json:"delayed_release,omitempty"`

	// 锁定期（映射到TimeLock）
	LockPeriod *LockPeriodConfig `json:"lock_period,omitempty"`

	// 分阶段释放（映射到HeightLock）
	StagedRelease *StagedReleaseConfig `json:"staged_release,omitempty"`

	// 定时任务
	ScheduledTask *ScheduledTaskConfig `json:"scheduled_task,omitempty"`
}

// DelayedReleaseConfig 延迟发布配置
//
// 🎯 **映射到TimeLock的延迟发布**
//
// 📝 **典型应用**：
// - 定时发布的公告
// - 延迟生效的政策变更
// - 定时解锁的奖励
type DelayedReleaseConfig struct {
	ReleaseTime time.Time `json:"release_time"` // 发布时间
	TimeSource  string    `json:"time_source"`  // "block_timestamp", "oracle", "consensus_time"
	Description string    `json:"description"`  // 发布说明
	AllowEarly  bool      `json:"allow_early"`  // 是否允许提前发布（需要额外权限）
}

// LockPeriodConfig 锁定期配置
//
// 🎯 **映射到TimeLock/HeightLock的锁定期**
type LockPeriodConfig struct {
	LockDuration time.Duration `json:"lock_duration"`           // 锁定时长
	LockType     string        `json:"lock_type"`               // "time_based", "height_based"
	UnlockHeight uint64        `json:"unlock_height,omitempty"` // 解锁区块高度（height_based时）
	Description  string        `json:"description"`             // 锁定说明
	BasePolicy   string        `json:"base_policy"`             // 基础策略（锁定期满后的访问控制）
}

// StagedReleaseConfig 分阶段释放配置
//
// 🎯 **映射到HeightLock的分阶段释放**
//
// 📝 **典型应用**：
// - 员工股权激励的分期释放
// - 项目资金的阶段性拨付
// - 奖励的分批发放
type StagedReleaseConfig struct {
	Stages      []ReleaseStage `json:"stages"`       // 释放阶段列表
	Description string         `json:"description"`  // 分阶段释放说明
	AutoExecute bool           `json:"auto_execute"` // 是否自动执行释放
}

// ReleaseStage 释放阶段
type ReleaseStage struct {
	ReleaseHeight uint64 `json:"release_height"` // 释放区块高度
	ReleaseRatio  string `json:"release_ratio"`  // 释放比例（"0.25" = 25%）
	Description   string `json:"description"`    // 阶段描述
	Condition     string `json:"condition"`      // 释放条件（可选）
}

// ScheduledTaskConfig 定时任务配置
type ScheduledTaskConfig struct {
	TaskType      string    `json:"task_type"`      // 任务类型
	ExecuteTime   time.Time `json:"execute_time"`   // 执行时间
	RecurringType string    `json:"recurring_type"` // 重复类型（"once", "daily", "weekly"）
	MaxExecutions uint32    `json:"max_executions"` // 最大执行次数
}

// AuthorizationMode 授权模式
//
// 🎯 **统一的授权机制抽象**
//
// 将复杂的授权机制抽象为用户容易理解的模式：
// - single: 单人授权（SingleKeyLock）
// - multi: 多重签名（MultiKeyLock）
// - threshold: 门限签名（ThresholdLock）
// - delegation: 委托授权（DelegationLock）
type AuthorizationMode struct {
	ModeType string `json:"mode_type"` // "single", "multi", "threshold", "delegation"

	// 多重签名配置
	MultiSigConfig *MultiSigConfig `json:"multi_sig,omitempty"`

	// 门限签名配置
	ThresholdConfig *ThresholdConfig `json:"threshold,omitempty"`

	// 委托授权配置
	DelegationConfig *DelegationConfig `json:"delegation,omitempty"`
}

// MultiSigConfig 多重签名配置
//
// 🎯 **映射到MultiKeyLock的多重签名**
type MultiSigConfig struct {
	RequiredSignatures  uint32        `json:"required_signatures"`   // 需要的签名数量（M）
	AuthorizedSigners   []string      `json:"authorized_signers"`    // 授权签名者地址列表（N个）
	Description         string        `json:"description"`           // 多签策略描述
	AllowPartialSigning bool          `json:"allow_partial_signing"` // 是否允许部分签名
	SigningTimeout      time.Duration `json:"signing_timeout"`       // 签名超时时间
}

// ThresholdConfig 门限签名配置
//
// 🎯 **映射到ThresholdLock的门限签名**
//
// 📝 **银行级安全应用**：
// - 央行数字货币发行
// - 大额资产管理
// - 关键系统权限控制
type ThresholdConfig struct {
	Threshold       uint32   `json:"threshold"`        // 门限值（需要的最少份额数）
	TotalParties    uint32   `json:"total_parties"`    // 总参与方数量
	PartyRoles      []string `json:"party_roles"`      // 参与方角色列表
	SecurityLevel   uint32   `json:"security_level"`   // 安全级别（位数）
	SignatureScheme string   `json:"signature_scheme"` // 签名方案（"BLS_THRESHOLD"等）
	CeremonyID      string   `json:"ceremony_id"`      // 可信设置仪式ID
}

// DelegationConfig 委托授权配置
//
// 🎯 **映射到DelegationLock的委托授权**
//
// 📝 **典型应用场景**：
// - 临时项目协作权限
// - 代理交易授权
// - 权限租借服务
// - 员工权限管理
type DelegationConfig struct {
	AllowedDelegates []string      `json:"allowed_delegates"` // 允许的被委托者地址列表
	Operations       []string      `json:"operations"`        // 授权操作类型（"reference", "execute", "transfer"）
	ExpiryDuration   time.Duration `json:"expiry_duration"`   // 委托过期时间
	MaxValuePerOp    string        `json:"max_value_per_op"`  // 单次操作最大价值限制
	RenewalAllowed   bool          `json:"renewal_allowed"`   // 是否允许续期
	DelegationPolicy string        `json:"delegation_policy"` // 委托策略描述
}

// ================================================================================================
// 🎯 第十一部分：企业级功能和业务配置
// ================================================================================================

// BusinessModelOptions 商业模式选项
//
// 🎯 **资源商业化配置**
//
// 支持多种商业化模式，让资源提供者能够通过区块链实现商业价值。
type BusinessModelOptions struct {
	RevenueSharing   string     `json:"revenue_sharing"`   // 收入分成比例（"0.7" = 70%给资源方）
	PlatformFee      string     `json:"platform_fee"`      // 平台手续费比例
	QualityAssurance bool       `json:"quality_assurance"` // 是否启用质量保证机制
	SLA              *SLAConfig `json:"sla,omitempty"`     // 服务质量协议
}

// SLAConfig 服务质量协议
//
// 🎯 **定义服务质量承诺和补偿机制**
type SLAConfig struct {
	ResponseTime       time.Duration `json:"response_time"`       // 响应时间承诺
	Availability       float64       `json:"availability"`        // 可用性承诺（99.9%）
	ErrorRate          float64       `json:"error_rate"`          // 错误率上限
	CompensationPolicy string        `json:"compensation_policy"` // 补偿政策
}

// PermissionModelOptions 权限模型选项
//
// 🎯 **灵活的权限控制模式**
type PermissionModelOptions struct {
	PermissionType   string   `json:"permission_type"`   // "rbac", "abac", "custom"
	DefaultPerm      string   `json:"default_perm"`      // 默认权限
	InheritanceRules []string `json:"inheritance_rules"` // 权限继承规则
	AuditEnabled     bool     `json:"audit_enabled"`     // 是否启用审计
}

// LifecycleControlOptions 生命周期控制选项
//
// 🎯 **资源生命周期管理**
type LifecycleControlOptions struct {
	AutoExpiry      bool          `json:"auto_expiry"`      // 是否自动过期
	ExpiryDuration  time.Duration `json:"expiry_duration"`  // 过期时间
	RenewalPolicy   string        `json:"renewal_policy"`   // 续期政策
	DeprecationPlan string        `json:"deprecation_plan"` // 废弃计划
}

// EnterpriseOptions 企业级选项
//
// 🎯 **企业级功能配置**
//
// 为企业用户提供完整的治理、合规、审计功能。
type EnterpriseOptions struct {
	ComplianceCheck  bool   `json:"compliance_check"`  // 合规检查
	AuditTrail       string `json:"audit_trail"`       // 审计跟踪信息
	ApprovalWorkflow string `json:"approval_workflow"` // 审批工作流
	RiskAssessment   bool   `json:"risk_assessment"`   // 风险评估
	EmployeeVesting  bool   `json:"employee_vesting"`  // 员工股权激励
	VestingPolicy    string `json:"vesting_policy"`    // 股权激励政策
	ComplianceLevel  string `json:"compliance_level"`  // 合规等级
	RegulatoryZone   string `json:"regulatory_zone"`   // 监管区域
	DataResidency    string `json:"data_residency"`    // 数据驻留要求
}

// EnterpriseResourceOptions 企业级资源选项
//
// 🎯 **企业级资源管理功能**
type EnterpriseResourceOptions struct {
	SecurityClassification string   `json:"security_classification"` // 安全分级
	AccessLogging          bool     `json:"access_logging"`          // 访问日志
	DataEncryption         bool     `json:"data_encryption"`         // 数据加密
	BackupPolicy           string   `json:"backup_policy"`           // 备份策略
	DisasterRecovery       bool     `json:"disaster_recovery"`       // 灾难恢复
	ComplianceTags         []string `json:"compliance_tags"`         // 合规标签
}

// FeeControlOptions 费用控制选项
//
// 🎯 **智能费用优化**
type FeeControlOptions struct {
	MaxFee                   string       `json:"max_fee"`                    // 最大费用限制
	FeeStrategy              string       `json:"fee_strategy"`               // 费用策略（"minimize", "balance", "priority"）
	ExecutionFeeOptimization bool         `json:"execution_fee_optimization"` // 执行费用优化
	FeeScheduling            *FeeSchedule `json:"fee_scheduling,omitempty"`   // 费用调度
}

// FeeSchedule 费用调度
//
// 🎯 **智能费用调度策略**
type FeeSchedule struct {
	ScheduleType  string        `json:"schedule_type"`  // "immediate", "delayed", "optimal"
	DelayTime     time.Duration `json:"delay_time"`     // 延迟时间
	OptimalWindow time.Duration `json:"optimal_window"` // 最优时间窗口
}

// ComplianceOptions 合规选项
//
// 🎯 **全面的合规管理**
type ComplianceOptions struct {
	KYCRequired       bool     `json:"kyc_required"`       // 是否需要KYC
	AMLCheck          bool     `json:"aml_check"`          // 反洗钱检查
	TaxReporting      bool     `json:"tax_reporting"`      // 税务报告
	JurisdictionRules []string `json:"jurisdiction_rules"` // 司法管辖规则
	PrivacyLevel      string   `json:"privacy_level"`      // 隐私保护级别
}
