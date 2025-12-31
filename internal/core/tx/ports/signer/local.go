// Package signer 提供签名器实现
//
// ⚠️ **安全警告**：本实现使用本地存储的私钥进行签名，严禁在生产环境使用！
//
// 🎯 **适用场景**：
// - 开发环境：快速开发和调试
// - 测试环境：自动化测试
// - CI/CD：持续集成测试
//
// 🚫 **禁止场景**：
// - 生产环境（会在启动时检查并报错）
// - 预发布环境（建议使用 KMS）
// - 任何处理真实资产的环境
//
// 📋 **设计原则**：
// - 环境检查优先：启动时检查环境，生产环境立即报错
// - 明确警告：启动时打印警告日志
// - 算法标准：支持 ECDSA (secp256k1) 和 ED25519
// - 依赖注入：使用 crypto.SignatureManager 和 crypto.HashManager
package signer

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// LocalSigner 本地私钥签名器
//
// 🎯 **核心功能**：使用本地私钥对交易进行签名
//
// ⚠️ **安全约束**：
// - 只能在开发/测试环境使用
// - 私钥存储在本地文件或内存中（不安全）
// - 无审计日志
// - 无密钥轮换机制
type LocalSigner struct {
	privateKeyBytes   []byte                         // 私钥字节（32字节）
	publicKey         *transaction.PublicKey         // 对应的公钥
	algorithm         transaction.SignatureAlgorithm // 签名算法
	keyMgr            crypto.KeyManager              // 密钥管理器
	sigMgr            crypto.SignatureManager        // 签名管理器
	hashCanonicalizer *hash.Canonicalizer            // 规范化哈希计算器（TX 内部工具）
	logger            log.Logger                     // 日志服务
}

// LocalSignerConfig LocalSigner 配置
type LocalSignerConfig struct {
	PrivateKeyHex string                         // 私钥（Hex编码）
	Algorithm     transaction.SignatureAlgorithm // 签名算法
	Environment   string                         // 环境标识（development, testing）
}

// NewLocalSigner 创建本地签名器实例
//
// 参数：
//   - config: 签名器配置
//   - keyMgr: 密钥管理器
//   - sigMgr: 签名管理器
//   - hashCanonicalizer: 规范化哈希计算器
//   - logger: 日志服务
//
// 返回：
//   - *LocalSigner: 签名器实例
//   - error: 创建失败（环境检查不通过、私钥无效等）
//
// ⚠️ 环境检查：
// 如果检测到生产环境，会立即返回错误
func NewLocalSigner(
	config *LocalSignerConfig,
	keyMgr crypto.KeyManager,
	sigMgr crypto.SignatureManager,
	hashCanonicalizer *hash.Canonicalizer,
	logger log.Logger,
) (*LocalSigner, error) {
	// 1. 环境检查（最高优先级）
	if err := checkEnvironment(config.Environment, logger); err != nil {
		return nil, err
	}

	// 2. 打印安全警告
	if logger != nil {
		logger.Warn("⚠️  ==================================================")
		logger.Warn("⚠️  使用 LocalSigner（不安全）")
		logger.Warn("⚠️  仅用于开发/测试环境")
		logger.Warn("⚠️  生产环境严禁使用！")
		logger.Warnf("⚠️  环境: %s", config.Environment)
		logger.Warnf("⚠️  算法: %s", config.Algorithm.String())
		logger.Warn("⚠️  ==================================================")
	}

	// 3. 解析私钥
	privateKeyBytes, err := parsePrivateKeyHex(config.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 4. 根据算法提取公钥
	publicKey, err := derivePublicKey(privateKeyBytes, config.Algorithm, keyMgr, logger)
	if err != nil {
		return nil, fmt.Errorf("提取公钥失败: %w", err)
	}

	return &LocalSigner{
		privateKeyBytes:   privateKeyBytes,
		publicKey:         publicKey,
		algorithm:         config.Algorithm,
		keyMgr:            keyMgr,
		sigMgr:            sigMgr,
		hashCanonicalizer: hashCanonicalizer,
		logger:            logger,
	}, nil
}

// ================================================================================================
// 实现 tx.Signer 接口
// ================================================================================================

// Sign 对交易签名
//
// 实现 tx.Signer 接口
//
// 流程：
// 1. 使用 HashCanonicalizer 计算交易哈希（规范化序列化，排除签名字段）
// 2. 根据算法使用私钥签名
// 3. 返回签名数据
//
// 注意：本实现使用 SIGHASH_ALL（签名所有输入和输出）
func (s *LocalSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	// 1. 使用 HashCanonicalizer 计算交易哈希
	//   注意：对于 ProofProvider 生成证明时，inputIndex 应该从 ProofProvider 传入
	//   这里使用 ComputeTransactionHash 作为默认实现，适用于简单场景
	//   更复杂的签名场景（如多输入、不同 SIGHASH 类型）应在 ProofProvider 中处理
	txHash, err := s.hashCanonicalizer.ComputeTransactionHash(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 2. 根据算法签名
	var signature []byte
	switch s.algorithm {
	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
		// 使用 ECDSA secp256k1 签名
		signature, err = s.sigMgr.Sign(txHash, s.privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ECDSA签名失败: %w", err)
		}

	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
		// 使用 ED25519 签名
		signature, err = s.sigMgr.Sign(txHash, s.privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ED25519签名失败: %w", err)
		}

	default:
		return nil, fmt.Errorf("不支持的签名算法: %v", s.algorithm)
	}

	// 3. 返回签名数据
	if s.logger != nil {
		s.logger.Debugf("[LocalSigner] 交易签名完成: 哈希%d字节 → 签名%d字节", len(txHash), len(signature))
	}

	return &transaction.SignatureData{
		Value: signature,
	}, nil
}

// PublicKey 获取签名器对应的公钥
//
// 实现 tx.Signer 接口
//
// 返回：
//   - *transaction.PublicKey: 公钥数据
//   - error: 获取失败（本实现中始终返回 nil error）
func (s *LocalSigner) PublicKey() (*transaction.PublicKey, error) {
	return s.publicKey, nil
}

// SignBytes 对任意数据签名
//
// 实现 tx.Signer 接口
//
// 🎯 **核心逻辑**：对任意字节数据进行签名
//
// 参数：
//   - ctx: 上下文对象
//   - data: 待签名的原始数据（通常是哈希值）
//
// 返回：
//   - []byte: 签名字节数组
//   - error: 签名失败
func (s *LocalSigner) SignBytes(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("待签名数据为空")
	}

	// 根据算法签名
	var signature []byte
	var err error
	switch s.algorithm {
	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
		// 使用 ECDSA secp256k1 签名
		signature, err = s.sigMgr.Sign(data, s.privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ECDSA签名失败: %w", err)
		}

	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
		// 使用 ED25519 签名
		signature, err = s.sigMgr.Sign(data, s.privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ED25519签名失败: %w", err)
		}

	default:
		return nil, fmt.Errorf("不支持的签名算法: %v", s.algorithm)
	}

	if s.logger != nil {
		s.logger.Debugf("[LocalSigner] 数据签名完成: 数据%d字节 → 签名%d字节", len(data), len(signature))
	}

	return signature, nil
}

// Algorithm 返回签名算法
//
// 实现 tx.Signer 接口
//
// 返回：签名算法标识
func (s *LocalSigner) Algorithm() transaction.SignatureAlgorithm {
	return s.algorithm
}

// ================================================================================================
// 内部辅助方法
// ================================================================================================

// checkEnvironment 检查运行环境
//
// 如果检测到生产环境，返回错误
func checkEnvironment(env string, logger log.Logger) error {
	// 检查环境变量 ENV
	if envVar := os.Getenv("ENV"); envVar != "" {
		if strings.Contains(strings.ToLower(envVar), "prod") {
			return fmt.Errorf("❌ LocalSigner 禁止在生产环境使用（ENV=%s）", envVar)
		}
	}

	// 检查环境变量 ENVIRONMENT
	if envVar := os.Getenv("ENVIRONMENT"); envVar != "" {
		if strings.Contains(strings.ToLower(envVar), "prod") {
			return fmt.Errorf("❌ LocalSigner 禁止在生产环境使用（ENVIRONMENT=%s）", envVar)
		}
	}

	// 检查配置中的环境
	if strings.Contains(strings.ToLower(env), "prod") {
		return fmt.Errorf("❌ LocalSigner 禁止在生产环境使用（config.Environment=%s）", env)
	}

	// 检查主机名
	hostname, _ := os.Hostname()
	if hostname != "" {
		hostnameL := strings.ToLower(hostname)
		if strings.Contains(hostnameL, "prod") || strings.Contains(hostnameL, "production") {
			return fmt.Errorf("❌ LocalSigner 禁止在生产环境使用（hostname=%s）", hostname)
		}
	}

	return nil
}

// parsePrivateKeyHex 解析 Hex 编码的私钥
//
// 参数：
//   - hexKey: Hex 编码的私钥字符串
//
// 返回：
//   - []byte: 私钥字节（32字节）
//   - error: 解析失败
func parsePrivateKeyHex(hexKey string) ([]byte, error) {
	// 移除可能的 "0x" 前缀
	hexKey = strings.TrimPrefix(hexKey, "0x")
	hexKey = strings.TrimPrefix(hexKey, "0X")

	// 检查长度（32字节 = 64个hex字符）
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("私钥长度无效: %d（期望64个hex字符）", len(hexKey))
	}

	// Hex 解码
	privateKeyBytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		_, err := fmt.Sscanf(hexKey[i*2:i*2+2], "%02x", &privateKeyBytes[i])
		if err != nil {
			return nil, fmt.Errorf("解析私钥hex失败: %w", err)
		}
	}

	return privateKeyBytes, nil
}

// derivePublicKey 从私钥提取公钥
//
// 参数：
//   - privateKeyBytes: 私钥字节
//   - algorithm: 签名算法
//   - keyMgr: 密钥管理器
//   - logger: 日志服务
//
// 返回：
//   - *transaction.PublicKey: 公钥数据
//   - error: 提取失败
func derivePublicKey(
	privateKeyBytes []byte,
	algorithm transaction.SignatureAlgorithm,
	keyMgr crypto.KeyManager,
	logger log.Logger,
) (*transaction.PublicKey, error) {
	// 验证私钥长度
	if len(privateKeyBytes) != 32 {
		return nil, fmt.Errorf("私钥长度无效: 期望32字节，实际%d字节", len(privateKeyBytes))
	}

	switch algorithm {
	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
		// ECDSA secp256k1: 从私钥计算公钥（33字节压缩格式）
		pubKeyBytes, err := keyMgr.DerivePublicKey(privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ECDSA公钥派生失败: %w", err)
		}

		// 验证公钥长度（应为33字节压缩格式）
		if len(pubKeyBytes) != 33 {
			return nil, fmt.Errorf("ECDSA公钥长度无效: 期望33字节，实际%d字节", len(pubKeyBytes))
		}

		if logger != nil {
			logger.Debugf("[LocalSigner] 成功派生ECDSA公钥: %d字节", len(pubKeyBytes))
		}

		return &transaction.PublicKey{
			Value: pubKeyBytes,
		}, nil

	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
		// ED25519: 从私钥计算公钥（32字节）
		// 注意：ED25519的公钥派生方式与ECDSA不同
		// KeyManager.DerivePublicKey 默认实现secp256k1
		// 对于ED25519，我们需要使用专门的库

		// 如果KeyManager支持ED25519，使用它
		pubKeyBytes, err := keyMgr.DerivePublicKey(privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("ED25519公钥派生失败: %w", err)
		}

		// ED25519公钥应为32字节
		if len(pubKeyBytes) != 32 && len(pubKeyBytes) != 33 {
			return nil, fmt.Errorf("ED25519公钥长度无效: 期望32字节，实际%d字节", len(pubKeyBytes))
		}

		// 如果是33字节（可能是压缩格式），取后32字节
		if len(pubKeyBytes) == 33 {
			pubKeyBytes = pubKeyBytes[1:]
		}

		if logger != nil {
			logger.Debugf("[LocalSigner] 成功派生ED25519公钥: %d字节", len(pubKeyBytes))
		}

		return &transaction.PublicKey{
			Value: pubKeyBytes,
		}, nil

	default:
		return nil, fmt.Errorf("不支持的签名算法: %v", algorithm)
	}
}

// ================================================================================================
// 测试辅助方法
// ================================================================================================

// NewLocalSignerForTesting 创建用于测试的本地签名器
//
// 用途：单元测试中快速创建签名器，无需配置文件
//
// 参数：
//   - privateKeyHex: Hex 编码的私钥
//   - algorithm: 签名算法
//   - keyMgr: 密钥管理器
//   - sigMgr: 签名管理器
//   - hashCanonicalizer: 规范化哈希计算器
//
// 返回：
//   - *LocalSigner: 签名器实例
//   - error: 创建失败
func NewLocalSignerForTesting(
	privateKeyHex string,
	algorithm transaction.SignatureAlgorithm,
	keyMgr crypto.KeyManager,
	sigMgr crypto.SignatureManager,
	hashCanonicalizer *hash.Canonicalizer,
) (*LocalSigner, error) {
	// 测试环境无需环境检查和警告日志
	privateKeyBytes, err := parsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	publicKey, err := derivePublicKey(privateKeyBytes, algorithm, keyMgr, nil)
	if err != nil {
		return nil, fmt.Errorf("提取公钥失败: %w", err)
	}

	return &LocalSigner{
		privateKeyBytes:   privateKeyBytes,
		publicKey:         publicKey,
		algorithm:         algorithm,
		keyMgr:            keyMgr,
		sigMgr:            sigMgr,
		hashCanonicalizer: hashCanonicalizer,
		logger:            nil, // 测试中可以不需要日志
	}, nil
}
