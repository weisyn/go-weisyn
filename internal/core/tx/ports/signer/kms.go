// Package signer 提供签名器实现
//
// ✅ **生产级实现**：适用于生产环境的安全密钥管理
//
// 🎯 **适用场景**：
// - 生产环境：企业级安全要求
// - 预发布环境：接近生产的测试
// - 审计要求：需要完整审计日志
// - 合规要求：密钥管理合规性
//
// 🔒 **安全特性**：
// - 私钥永不离开 KMS：签名操作在 KMS 内部完成
// - 访问控制：基于 IAM/RBAC 的细粒度权限
// - 审计日志：所有签名操作记录到审计系统
// - 密钥轮换：支持自动密钥轮换
// - 密钥备份：KMS 提供商负责密钥备份和恢复
//
// 🌐 **支持的 KMS 提供商**：
// - AWS KMS（Amazon Web Services）
// - GCP KMS（Google Cloud Platform）
// - Azure Key Vault（Microsoft Azure）
// - HashiCorp Vault
// - 自定义 KMS（通过 KMSClient 接口）
//
// 📋 **设计原则**：
// - 接口抽象：KMSClient 接口支持多种 KMS 提供商
// - 重试机制：自动重试临时性失败
// - 超时控制：避免长时间阻塞
// - 错误分类：区分临时性错误和永久性错误
package signer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// KMSClient KMS 客户端接口（用于依赖注入和测试）
//
// 🎯 **设计理念**：
// 定义最小化的 KMS 操作接口，支持多种 KMS 提供商实现。
// 生产环境可以使用 AWS SDK / GCP SDK / Azure SDK，测试环境可以使用 mock。
type KMSClient interface {
	// Sign 使用 KMS 密钥对数据进行签名
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: 密钥标识符（KMS 特定格式）
	//   - data: 待签名的数据（已哈希）
	//   - algorithm: 签名算法
	//
	// 返回：
	//   - []byte: 签名字节
	//   - error: 签名失败的原因
	Sign(ctx context.Context, keyID string, data []byte, algorithm transaction.SignatureAlgorithm) ([]byte, error)

	// GetPublicKey 获取密钥对应的公钥
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: 密钥标识符
	//
	// 返回：
	//   - *transaction.PublicKey: 公钥对象
	//   - error: 获取失败的原因
	GetPublicKey(ctx context.Context, keyID string) (*transaction.PublicKey, error)

	// VerifyKeyAccess 验证是否有权访问指定密钥
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: 密钥标识符
	//
	// 返回：
	//   - error: 访问验证失败的原因
	VerifyKeyAccess(ctx context.Context, keyID string) error

	// ListKeys 列出所有可访问的密钥
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - []string: 密钥 ID 列表
	//   - error: 列出失败的原因
	ListKeys(ctx context.Context) ([]string, error)
}

// KMSSigner KMS 签名器
//
// 🎯 **核心功能**：通过 KMS 对交易进行安全签名
//
// 🔒 **安全保证**：
// - 私钥永不暴露：签名操作在 KMS 内部完成
// - 访问审计：所有签名操作记录审计日志
// - 密钥隔离：不同环境使用不同密钥
// - 错误恢复：自动重试机制
type KMSSigner struct {
	client         KMSClient                           // KMS 客户端
	keyID          string                              // 密钥 ID
	publicKey      *transaction.PublicKey              // 缓存的公钥
	algorithm      transaction.SignatureAlgorithm      // 签名算法
	txHashClient   transaction.TransactionHashServiceClient // 交易哈希服务客户端
	hashManager    crypto.HashManager                  // 哈希管理器（用于SignBytes）
	logger         log.Logger                          // 日志服务
	retryCount     int                                 // 重试次数
	retryDelay     time.Duration                       // 重试延迟
	signTimeout    time.Duration                       // 签名超时
}

// KMSSignerConfig KMSSigner 配置
type KMSSignerConfig struct {
	// KMS 密钥标识符
	// 格式示例：
	//   - AWS KMS: "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"
	//   - GCP KMS: "projects/my-project/locations/global/keyRings/my-keyring/cryptoKeys/my-key"
	//   - Azure: "https://my-vault.vault.azure.net/keys/my-key/version"
	//   - Vault: "transit/keys/my-key"
	KeyID string

	// 签名算法
	Algorithm transaction.SignatureAlgorithm

	// 重试配置
	RetryCount int           // 重试次数（默认 3）
	RetryDelay time.Duration // 重试延迟（默认 100ms）

	// 超时配置
	SignTimeout time.Duration // 签名超时（默认 5s）

	// 环境标识（用于日志和监控）
	Environment string
}

// DefaultKMSSignerConfig 返回默认配置
func DefaultKMSSignerConfig() *KMSSignerConfig {
	return &KMSSignerConfig{
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		RetryCount:  3,
		RetryDelay:  100 * time.Millisecond,
		SignTimeout: 5 * time.Second,
		Environment: "production",
	}
}

// NewKMSSigner 创建 KMS 签名器实例
//
// 参数：
//   - config: 签名器配置
//   - client: KMS 客户端（需实现 KMSClient 接口）
//   - txHashClient: 交易哈希服务客户端（用于计算交易哈希）
//   - hashManager: 哈希管理器（用于SignBytes方法）
//   - logger: 日志服务
//
// 返回：
//   - *KMSSigner: 签名器实例
//   - error: 创建失败（密钥无效、无访问权限等）
func NewKMSSigner(
	config *KMSSignerConfig,
	client KMSClient,
	txHashClient transaction.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	logger log.Logger,
) (*KMSSigner, error) {
	if config == nil {
		config = DefaultKMSSignerConfig()
	}

	if client == nil {
		return nil, fmt.Errorf("KMS client cannot be nil")
	}

	if txHashClient == nil {
		return nil, fmt.Errorf("transaction hash client cannot be nil")
	}

	if hashManager == nil {
		return nil, fmt.Errorf("hash manager cannot be nil")
	}

	if config.KeyID == "" {
		return nil, fmt.Errorf("key ID cannot be empty")
	}

	// 验证密钥访问权限
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.VerifyKeyAccess(ctx, config.KeyID); err != nil {
		return nil, fmt.Errorf("failed to verify key access: %w", err)
	}

	// 获取公钥
	publicKey, err := client.GetPublicKey(ctx, config.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	signer := &KMSSigner{
		client:       client,
		keyID:        config.KeyID,
		publicKey:    publicKey,
		algorithm:    config.Algorithm,
		txHashClient: txHashClient,
		hashManager:  hashManager,
		logger:       logger,
		retryCount:   config.RetryCount,
		retryDelay:   config.RetryDelay,
		signTimeout:  config.SignTimeout,
	}

	// 打印初始化日志
	if logger != nil {
		logger.Info("✅ KMSSigner 初始化成功")
		logger.Infof("   密钥 ID: %s", maskKeyID(config.KeyID))
		logger.Infof("   算法: %s", config.Algorithm.String())
		logger.Infof("   环境: %s", config.Environment)
		logger.Infof("   重试次数: %d", config.RetryCount)
		logger.Infof("   签名超时: %s", config.SignTimeout)
	}

	return signer, nil
}

// Sign 对交易进行签名
//
// 实现 tx.Signer 接口
//
// 🎯 **签名流程**：
// 1. 序列化交易为待签名数据
// 2. 计算交易哈希
// 3. 调用 KMS 进行签名（带重试机制）
// 4. 记录审计日志
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待签名的交易
//
// 返回：
//   - *transaction.SignatureData: 签名数据
//   - error: 签名失败的原因
func (s *KMSSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	// 1. 使用 gRPC 服务计算交易哈希
	if s.txHashClient == nil {
		return nil, fmt.Errorf("transaction hash client is not initialized")
	}

	req := &transaction.ComputeHashRequest{
		Transaction:     tx,
		IncludeDebugInfo: false,
	}
	resp, err := s.txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to compute transaction hash: %w", err)
	}
	if !resp.IsValid {
		return nil, fmt.Errorf("transaction structure is invalid")
	}
	txHash := resp.Hash

	// 记录签名请求
	if s.logger != nil {
		s.logger.Debugf("开始 KMS 签名，交易哈希: %x", txHash[:8])
	}

	// 3. 创建签名上下文（带超时）
	signCtx, cancel := context.WithTimeout(ctx, s.signTimeout)
	defer cancel()

	// 4. 调用 KMS 签名（带重试）
	var signature []byte
	var lastErr error

	for attempt := 0; attempt <= s.retryCount; attempt++ {
		if attempt > 0 {
			// 重试延迟
			if s.logger != nil {
				s.logger.Warnf("KMS 签名重试 %d/%d", attempt, s.retryCount)
			}
			time.Sleep(s.retryDelay)
		}

		signature, lastErr = s.client.Sign(signCtx, s.keyID, txHash, s.algorithm)
		if lastErr == nil {
			break // 签名成功
		}

		// 判断是否为临时性错误，决定是否重试
		if !isRetryableError(lastErr) {
			break // 永久性错误，不重试
		}
	}

	if lastErr != nil {
		if s.logger != nil {
			s.logger.Errorf("KMS 签名失败: %v", lastErr)
		}
		return nil, fmt.Errorf("KMS sign failed after %d retries: %w", s.retryCount, lastErr)
	}

	// 5. 构造签名数据
	signatureData := &transaction.SignatureData{
		Value: signature,
	}

	// 6. 记录审计日志
	if s.logger != nil {
		s.logger.Infof("✅ KMS 签名成功，交易哈希: %x, 签名长度: %d", txHash[:8], len(signature))
	}

	return signatureData, nil
}

// PublicKey 返回签名器对应的公钥
//
// 实现 tx.Signer 接口
//
// 返回：
//   - *transaction.PublicKey: 公钥对象
func (s *KMSSigner) PublicKey() *transaction.PublicKey {
	return s.publicKey
}

// Algorithm 返回签名算法
//
// 实现 tx.Signer 接口
//
// 返回：
//   - transaction.SignatureAlgorithm: 签名算法
func (s *KMSSigner) Algorithm() transaction.SignatureAlgorithm {
	return s.algorithm
}

// SignBytes 签名任意字节数据
//
// 实现 tx.Signer 接口（P2-3b扩展）
//
// 🎯 **核心功能**：对原始字节数据进行签名（不涉及交易结构）
//
// **签名流程**：
// 1. 验证输入数据非空
// 2. 计算数据的SHA256哈希（KMS期望接收已哈希的数据）
// 3. 调用KMS客户端签名哈希数据
// 4. 返回签名字节数组
//
// **与Sign方法的区别**：
// - Sign方法：签名完整的Transaction对象（通过gRPC服务计算交易哈希）
// - SignBytes方法：签名任意原始字节数据（直接哈希后签名）
//
// 参数：
//   - ctx: 上下文对象
//   - data: 待签名的原始字节数据
//
// 返回：
//   - []byte: 签名字节数组
//   - error: 签名失败的原因
func (s *KMSSigner) SignBytes(ctx context.Context, data []byte) ([]byte, error) {
	// 1. 验证输入数据非空
	if len(data) == 0 {
		return nil, fmt.Errorf("待签名数据为空")
	}

	// 记录签名请求
	if s.logger != nil {
		s.logger.Debugf("开始 KMS 签名原始数据，数据长度: %d 字节", len(data))
	}

	// 2. 计算数据的SHA256哈希
	// ✅ 修复：使用 HashManager 而不是直接使用 crypto/sha256
	// 注意：KMS.Sign方法期望接收已哈希的数据（根据KMSClient接口注释）
	dataHash := s.hashManager.SHA256(data)

	// 3. 创建签名上下文（带超时）
	signCtx, cancel := context.WithTimeout(ctx, s.signTimeout)
	defer cancel()

	// 4. 调用 KMS 签名（带重试）
	var signature []byte
	var lastErr error

	for attempt := 0; attempt <= s.retryCount; attempt++ {
		if attempt > 0 {
			// 重试延迟
			if s.logger != nil {
				s.logger.Warnf("KMS 签名原始数据重试 %d/%d", attempt, s.retryCount)
			}
			time.Sleep(s.retryDelay)
		}

		signature, lastErr = s.client.Sign(signCtx, s.keyID, dataHash, s.algorithm)
		if lastErr == nil {
			break // 签名成功
		}

		// 判断是否为临时性错误，决定是否重试
		if !isRetryableError(lastErr) {
			break // 永久性错误，不重试
		}
	}

	if lastErr != nil {
		if s.logger != nil {
			s.logger.Errorf("KMS 签名原始数据失败: %v", lastErr)
		}
		return nil, fmt.Errorf("KMS sign bytes failed after %d retries: %w", s.retryCount, lastErr)
	}

	// 5. 记录审计日志
	if s.logger != nil {
		s.logger.Infof("✅ KMS 签名原始数据成功，数据长度: %d 字节，签名长度: %d 字节", len(data), len(signature))
	}

	return signature, nil
}

// RefreshPublicKey 刷新公钥缓存
//
// 扩展方法（非 tx.Signer 接口定义）
//
// 🎯 **使用场景**：
// - 密钥轮换后更新公钥
// - 定期刷新公钥缓存
//
// 参数：
//   - ctx: 上下文对象
//
// 返回：
//   - error: 刷新失败的原因
func (s *KMSSigner) RefreshPublicKey(ctx context.Context) error {
	publicKey, err := s.client.GetPublicKey(ctx, s.keyID)
	if err != nil {
		return fmt.Errorf("failed to refresh public key: %w", err)
	}

	s.publicKey = publicKey

	if s.logger != nil {
		s.logger.Info("✅ 公钥缓存已刷新")
	}

	return nil
}

// VerifyAccess 验证当前是否有权访问 KMS 密钥
//
// 扩展方法（非 tx.Signer 接口定义）
//
// 🎯 **使用场景**：
// - 健康检查
// - 启动时验证
// - 定期权限检查
//
// 参数：
//   - ctx: 上下文对象
//
// 返回：
//   - error: 访问验证失败的原因
func (s *KMSSigner) VerifyAccess(ctx context.Context) error {
	return s.client.VerifyKeyAccess(ctx, s.keyID)
}

// maskKeyID 掩码密钥 ID（用于日志输出，避免敏感信息泄露）
//
// 示例：
//   - 输入：arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
//   - 输出：arn:aws:kms:us-east-1:123456789012:key/1234****-****-****-****-********9012
func maskKeyID(keyID string) string {
	if len(keyID) < 20 {
		// 太短，只显示前4后4
		if len(keyID) <= 8 {
			return "****"
		}
		return keyID[:4] + "****" + keyID[len(keyID)-4:]
	}

	// 显示前20后12，中间掩码
	return keyID[:20] + "****" + keyID[len(keyID)-12:]
}

// isRetryableError 判断错误是否可重试
//
// 🎯 **重试策略**：
// - 网络错误：可重试
// - 超时错误：可重试
// - 限流错误：可重试
// - 权限错误：不可重试
// - 密钥不存在：不可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 可重试的错误模式
	retryablePatterns := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"temporary failure",
		"throttling",
		"rate limit",
		"service unavailable",
		"internal server error",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	// 不可重试的错误模式
	nonRetryablePatterns := []string{
		"not found",
		"invalid key",
		"access denied",
		"permission denied",
		"unauthorized",
		"forbidden",
		"invalid signature",
	}

	for _, pattern := range nonRetryablePatterns {
		if contains(errStr, pattern) {
			return false
		}
	}

	// 默认不重试（保守策略）
	return false
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && containsIgnoreCase(s, substr))
}

// containsIgnoreCase 不区分大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower 转小写（使用标准库，支持 Unicode）
// ✅ 修复：使用标准库 strings.ToLower 替代简化实现
func toLower(s string) string {
	return strings.ToLower(s)
}

// serializeTransaction 序列化交易为字节数组（用于计算哈希）
//
// 🎯 **规范化序列化**：
// 使用 protobuf Marshal 进行规范化序列化，确保签名的一致性。
//
// ⚠️ **签名注意事项**：
// 签名时不应包含 signatures 字段本身，否则会产生循环依赖。
// 这里序列化完整交易，但在实际签名验证时需要清除 signatures 字段。
//
// 参数：
//   - tx: 待序列化的交易
//
// 返回：
//   - []byte: 序列化的字节数组
//   - error: 序列化失败的原因
func serializeTransaction(tx *transaction.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction cannot be nil")
	}

	// 使用 protobuf Marshal 进行规范化序列化
	// proto.Marshal 会按照 protobuf 的规范进行序列化，确保一致性
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	return txBytes, nil
}
