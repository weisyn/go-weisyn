// Package compliance 提供WES系统的合规服务接口定义
//
// 🛡️ **合规服务接口 (Compliance Service Interfaces)**
//
// 本包定义了WES系统合规功能的公共接口，包括：
// - 合规策略判定接口
// - 身份凭证验证接口
// - 地理位置查询接口
// - 合规决策结果定义
//
// 🎯 **设计原则**
// - 接口导向：所有合规功能通过接口提供，便于测试和替换实现
// - 上下文支持：所有操作支持context.Context，便于超时和取消控制
// - 多信源融合：支持身份凭证、GeoIP、P2P等多种信息源的决策融合
// - 缓存友好：支持决策结果缓存，减少重复计算开销
package compliance

import (
	"context"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Policy 合规策略接口
//
// 🎯 **合规决策引擎 (Compliance Decision Engine)**
//
// 合规策略的核心接口，负责根据配置的合规规则对交易进行判定。
// 支持多种信息源的融合决策，包括身份凭证、地理位置等。
//
// 使用场景：
// - 内存池交易准入检查
// - 共识层交易选择过滤
// - 网关层请求拦截
// - API层操作权限验证
type Policy interface {
	// CheckTransaction 检查交易的合规性
	//
	// 对单笔交易进行完整的合规性检查，包括地理限制、操作限制等。
	//
	// 参数：
	// - ctx: 上下文，支持超时和取消
	// - tx: 待检查的交易
	// - source: 交易来源信息（IP地址、节点ID等）
	//
	// 返回：
	// - Decision: 合规决策结果
	// - error: 检查过程中的错误
	CheckTransaction(ctx context.Context, tx *transaction.Transaction, source *TransactionSource) (*Decision, error)

	// CheckOperation 检查特定操作的合规性
	//
	// 对特定操作类型进行合规性检查，支持更细粒度的控制。
	//
	// 参数：
	// - ctx: 上下文，支持超时和取消
	// - operation: 操作类型（如"transfer"、"contract.payments.send"）
	// - address: 发起操作的地址
	// - source: 操作来源信息
	//
	// 返回：
	// - Decision: 合规决策结果
	// - error: 检查过程中的错误
	CheckOperation(ctx context.Context, operation string, address string, source *TransactionSource) (*Decision, error)
}

// IdentityRegistry 身份凭证登记接口
//
// 🎫 **身份凭证验证服务 (Identity Verification Service)**
//
// 负责管理和验证用户的身份凭证，支持地址到属地的映射验证。
// 可与外部身份验证服务集成，提供可信的身份属地证明。
//
// 使用场景：
// - 地址属地验证
// - 身份凭证缓存管理
// - 外部身份服务集成
type IdentityRegistry interface {
	// VerifyAddressIdentity 验证地址的身份凭证
	//
	// 验证指定地址的身份信息，优先使用缓存，必要时查询外部服务。
	//
	// 参数：
	// - ctx: 上下文，支持超时和取消
	// - address: 区块链地址
	//
	// 返回：
	// - *AddressIdentity: 验证后的身份信息，nil表示无有效凭证
	// - error: 验证过程中的错误
	VerifyAddressIdentity(ctx context.Context, address string) (*AddressIdentity, error)

	// CacheIdentity 缓存已验证的身份信息
	//
	// 将已验证的身份信息存入本地缓存，提高后续查询性能。
	//
	// 参数：
	// - address: 区块链地址
	// - identity: 身份信息
	// - ttl: 缓存有效期
	CacheIdentity(address string, identity *AddressIdentity, ttl time.Duration)

	// ClearCache 清除身份缓存
	//
	// 清除指定地址或所有地址的身份缓存。
	//
	// 参数：
	// - address: 要清除的地址，空字符串表示清除所有缓存
	ClearCache(address string)
}

// GeoIPService 地理位置查询接口
//
// 🌍 **地理位置查询服务 (Geographic Location Service)**
//
// 负责根据IP地址查询地理位置信息，支持缓存和数据库更新。
// 可与第三方GeoIP数据库集成，提供准确的地理位置信息。
//
// 使用场景：
// - IP地址到国家的映射
// - 网关层地理位置检查
// - P2P节点地理特征分析
type GeoIPService interface {
	// GetCountryByIP 根据IP地址获取国家代码
	//
	// 查询指定IP地址对应的国家代码，支持IPv4和IPv6。
	//
	// 参数：
	// - ctx: 上下文，支持超时和取消
	// - ipAddress: IP地址字符串
	//
	// 返回：
	// - string: ISO-3166-1 alpha-2国家代码，空字符串表示未知
	// - error: 查询过程中的错误
	GetCountryByIP(ctx context.Context, ipAddress string) (string, error)

	// UpdateDatabase 更新GeoIP数据库
	//
	// 从指定源更新本地GeoIP数据库，确保数据的时效性。
	//
	// 参数：
	// - ctx: 上下文，支持超时和取消
	//
	// 返回：
	// - error: 更新过程中的错误
	UpdateDatabase(ctx context.Context) error
}

// ============================================================================
//                              数据结构定义
// ============================================================================

// Decision 合规决策结果
//
// 🎯 **合规决策结果 (Compliance Decision Result)**
//
// 包含合规检查的完整决策信息，包括决策结果、原因和信息源。
// 提供足够的信息用于审计和问题诊断。
type Decision struct {
	// Allowed 是否允许执行
	Allowed bool `json:"allowed"`

	// Reason 决策原因代码
	// 允许时为空，拒绝时包含具体原因
	Reason string `json:"reason,omitempty"`

	// ReasonDetail 详细原因描述
	ReasonDetail string `json:"reason_detail,omitempty"`

	// Country 判定的国家代码
	// 来源于身份凭证、GeoIP查询或P2P特征
	Country string `json:"country,omitempty"`

	// Source 决策信息源
	// 标识决策依据的主要信息来源
	Source DecisionSource `json:"source"`

	// Timestamp 决策时间戳
	Timestamp time.Time `json:"timestamp"`
}

// DecisionSource 决策信息源枚举
//
// 📍 **决策信息源 (Decision Source)**
//
// 标识合规决策所依据的主要信息来源，便于追踪和审计。
type DecisionSource string

const (
	// DecisionSourceIdentity 基于身份凭证的决策
	DecisionSourceIdentity DecisionSource = "identity_credential"

	// DecisionSourceGeoIP 基于GeoIP查询的决策
	DecisionSourceGeoIP DecisionSource = "geoip_lookup"

	// DecisionSourceP2P 基于P2P连接特征的决策
	DecisionSourceP2P DecisionSource = "p2p_geographic"

	// DecisionSourceConfig 基于配置规则的决策
	DecisionSourceConfig DecisionSource = "config_rule"

	// DecisionSourceUnknown 未知信息源
	DecisionSourceUnknown DecisionSource = "unknown"
)

// TransactionSource 交易来源信息
//
// 📍 **交易来源信息 (Transaction Source Information)**
//
// 包含交易提交时的来源信息，用于合规判定。
// 信息来源可能包括HTTP请求、P2P连接、gRPC调用等。
type TransactionSource struct {
	// IPAddress 来源IP地址
	IPAddress string `json:"ip_address,omitempty"`

	// NodeID 来源节点ID
	NodeID string `json:"node_id,omitempty"`

	// UserAgent HTTP请求的用户代理字符串
	UserAgent string `json:"user_agent,omitempty"`

	// Protocol 提交协议（http、grpc、p2p等）
	Protocol string `json:"protocol,omitempty"`

	// Timestamp 接收时间戳
	Timestamp time.Time `json:"timestamp"`

	// GeoLocation 已知的地理位置信息（可选）
	GeoLocation *GeoLocation `json:"geo_location,omitempty"`
}

// AddressIdentity 地址身份信息
//
// 🎫 **地址身份信息 (Address Identity Information)**
//
// 包含经过验证的地址身份凭证信息。
// 身份信息通过外部身份验证服务提供，经过数字签名验证。
type AddressIdentity struct {
	// Address 区块链地址
	Address string `json:"address"`

	// Country 注册国家代码（ISO-3166-1 alpha-2）
	Country string `json:"country"`

	// VerifiedAt 验证时间
	VerifiedAt time.Time `json:"verified_at"`

	// ExpiresAt 凭证过期时间
	ExpiresAt time.Time `json:"expires_at"`

	// CredentialHash 凭证哈希值
	// 用于验证凭证完整性，不包含敏感信息
	CredentialHash string `json:"credential_hash"`

	// IssuerID 凭证颁发者标识
	IssuerID string `json:"issuer_id,omitempty"`
}

// GeoLocation 地理位置信息
//
// 🌍 **地理位置信息 (Geographic Location Information)**
//
// 包含IP地址对应的地理位置详细信息。
// 支持多级地理精度，从国家到城市级别。
type GeoLocation struct {
	// Country 国家代码（ISO-3166-1 alpha-2）
	Country string `json:"country"`

	// CountryName 国家名称
	CountryName string `json:"country_name,omitempty"`

	// Region 地区/州/省代码
	Region string `json:"region,omitempty"`

	// City 城市名称
	City string `json:"city,omitempty"`

	// Accuracy GeoIP查询的准确度级别
	// city: 城市级准确度
	// region: 地区级准确度
	// country: 国家级准确度
	Accuracy string `json:"accuracy,omitempty"`
}

// ============================================================================
//                              常量定义
// ============================================================================

// ComplianceReason 合规拒绝原因常量
//
// 📋 **合规拒绝原因 (Compliance Rejection Reasons)**
//
// 标准化的合规拒绝原因代码，便于统一处理和分析。
const (
	// ReasonCountryBanned 国家被禁用
	ReasonCountryBanned = "country_banned"

	// ReasonOperationBanned 操作类型被禁用
	ReasonOperationBanned = "operation_banned"

	// ReasonIdentityInvalid 身份凭证无效
	ReasonIdentityInvalid = "identity_invalid"

	// ReasonIdentityExpired 身份凭证过期
	ReasonIdentityExpired = "identity_expired"

	// ReasonUnknownCountry 未知国家且配置拒绝
	ReasonUnknownCountry = "unknown_country_rejected"

	// ReasonInternalError 内部处理错误
	ReasonInternalError = "internal_error"
)

// OperationType 操作类型常量
//
// 📋 **操作类型 (Operation Types)**
//
// 标准化的操作类型定义，用于精确的合规控制。
const (
	// OperationTransfer 普通转账操作
	OperationTransfer = "transfer"

	// OperationContractCall 合约调用（通用）
	OperationContractCall = "contract.*"

	// OperationContractDeploy 合约部署
	OperationContractDeploy = "contract.deploy"

	// OperationContractPayments 支付相关合约方法
	OperationContractPayments = "contract.payments.*"

	// OperationContractGovernance 治理相关合约方法
	OperationContractGovernance = "contract.governance.*"

	// OperationContractStaking 质押相关合约方法
	OperationContractStaking = "contract.staking.*"
)
