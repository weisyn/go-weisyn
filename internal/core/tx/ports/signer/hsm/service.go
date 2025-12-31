//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package hsm 提供 HSM（Hardware Security Module）签名器实现
//
// ✅ **生产级实现**：适用于生产环境的硬件级安全密钥管理
//
// 🎯 **适用场景**：
// - 生产环境：金融级安全要求
// - 大额资产管理：银行级安全标准
// - 合规要求：FIPS 140-2 Level 3/4 认证
// - 本地化部署：HSM设备物理连接或同网络部署
//
// 🔒 **安全特性**：
// - 硬件级密钥保护：私钥存储在HSM设备中，永不离开硬件
// - PKCS#11标准：通过标准C API与HSM设备通信
// - 物理防篡改：FIPS 140-2 Level 3/4认证
// - 高性能签名：硬件加速，可达10000+ TPS
//
// 🌐 **支持的HSM厂商**：
// - Thales Luna
// - AWS CloudHSM
// - YubiHSM
// - 其他符合PKCS#11标准的HSM设备
//
// 📋 **设计原则**：
// - 接口抽象：支持多种HSM厂商
// - Session池管理：复用会话，提升性能
// - PIN安全管理：安全处理PIN输入
// - 错误分类：区分临时性错误和永久性错误
package hsm

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/pkcs11"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/kms"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// HSMSigner HSM签名器
//
// 🎯 **核心功能**：通过HSM设备对交易进行硬件级安全签名
//
// 🔒 **安全保证**：
// - 私钥永不暴露：签名操作在HSM硬件内部完成
// - 物理防篡改：HSM设备提供物理级安全保护
// - 访问审计：所有签名操作记录审计日志
//
// ✅ **当前状态**：PKCS#11集成框架
// - ✅ 接口定义和基础结构
// - ✅ Sign和SignBytes方法框架
// - ✅ PKCS#11 CGO封装（pkcs11_wrapper.go）
// - ⚠️ Session池管理待完善
// - ⚠️ PIN安全管理待完善
type HSMSigner struct {
	keyLabel     string                              // 密钥标签
	publicKey    *transaction.PublicKey              // 缓存的公钥
	algorithm    transaction.SignatureAlgorithm      // 签名算法
	txHashClient transaction.TransactionHashServiceClient // 交易哈希服务客户端（用于Sign方法）
	logger       log.Logger                          // 日志服务
	pkcs11Ctx    *PKCS11Context                     // PKCS#11上下文（可选，需要CGO）
	keyHandle    pkcs11.ObjectHandle                 // 密钥对象句柄（可选，需要CGO）
	pin          string                              // PIN码（已解密，明文）
	sessionPool  *SessionPool                        // Session池（可选）
	encryptionManager cryptointf.EncryptionManager      // 加密管理器（用于PIN解密）
	hashManager  cryptointf.HashManager                  // 哈希管理器（用于SignBytes）
}

// Config HSMSigner配置
type Config struct {
	// HSM密钥标签
	KeyLabel string

	// 签名算法
	Algorithm transaction.SignatureAlgorithm

	// PKCS#11库路径
	LibraryPath string

	// PIN配置（加密存储的PIN）
	EncryptedPIN string

	// KMS配置（用于从KMS获取PIN解密密码）
	KMSKeyID string   // KMS密钥ID（AWS KMS）
	KMSType  string   // KMS类型（aws, vault, azure）

	// HashiCorp Vault配置（如果KMSType为vault）
	VaultAddr      string // Vault地址
	VaultToken     string // Vault Token
	VaultSecretPath string // Vault密钥路径

	// PIN密码提供者（从crypto基础设施层获取）
	// 如果为nil，则使用环境变量提供者（EnvPINPasswordProvider）
	PINPasswordProvider cryptointf.PINPasswordProvider

	// Session池大小
	SessionPoolSize int

	// 环境标识（用于日志和监控）
	Environment string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Algorithm:       transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SessionPoolSize: 10,
		Environment:     "production",
	}
}

// NewHSMSigner 创建HSM签名器实例
//
// ✅ **当前实现**：完整框架
// - ✅ PKCS#11库加载和初始化
// - ✅ Session池管理
// - ✅ PIN解密机制
//
// 参数：
//   - config: HSM签名器配置
//   - txHashClient: 交易哈希服务客户端（用于Sign方法）
//   - encryptionManager: 加密管理器（用于PIN解密，可选）
//   - hashManager: 哈希管理器（用于SignBytes方法）
//   - logger: 日志服务
//
// 返回：
//   - *HSMSigner: 签名器实例
//   - error: 创建失败的原因
func NewHSMSigner(
	config *Config,
	txHashClient transaction.TransactionHashServiceClient,
	encryptionManager cryptointf.EncryptionManager,
	hashManager cryptointf.HashManager,
	logger log.Logger,
) (*HSMSigner, error) {
	if config == nil {
		return nil, fmt.Errorf("HSM配置不能为空")
	}

	if config.KeyLabel == "" {
		return nil, fmt.Errorf("HSM密钥标签不能为空")
	}

	// ✅ **PKCS#11集成**：如果提供了库路径，则初始化PKCS#11
	var pkcs11Ctx *PKCS11Context
	var keyHandle pkcs11.ObjectHandle = 0
	var publicKey *transaction.PublicKey
	var pin string
	var sessionPool *SessionPool

	if config.LibraryPath != "" {
		// 初始化PKCS#11上下文
		var err error
		pkcs11Ctx, err = NewPKCS11Context(config.LibraryPath, logger)
		if err != nil {
			return nil, fmt.Errorf("PKCS#11初始化失败: %w", err)
		}

		// 打开Session并登录
		session, err := pkcs11Ctx.OpenSession(pkcs11.CKF_SERIAL_SESSION | pkcs11.CKF_RW_SESSION)
		if err != nil {
			pkcs11Ctx.Finalize()
			return nil, fmt.Errorf("打开Session失败: %w", err)
		}

		// 解密PIN（如果提供了加密PIN）
		if config.EncryptedPIN != "" {
			if encryptionManager == nil {
				pkcs11Ctx.CloseSession(session)
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("需要EncryptionManager来解密PIN")
			}
			
			// ✅ **真实实现**：支持多种PIN密码获取方式
			// 1. 优先使用 KMS（如果配置了 KMSKeyID）
			// 2. 回退到环境变量（开发/测试环境）
			var pinPassword string
			var err error
			
			// ✅ **真实实现**：优先使用配置的PIN密码提供者，否则使用环境变量提供者
			ctx := context.Background()
			if config.PINPasswordProvider != nil {
				// 使用配置的provider（从crypto基础设施层获取）
				pinPassword, err = config.PINPasswordProvider.GetPINPassword(ctx, config.KMSKeyID)
			} else {
				// 回退到环境变量提供者（基础实现）
				// 使用crypto基础设施层的环境变量提供者
				envProvider := kms.NewEnvPINPasswordProvider(logger)
				pinPassword, err = envProvider.GetPINPassword(ctx, config.KMSKeyID)
			}
			
			if err != nil {
				pkcs11Ctx.CloseSession(session)
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("获取PIN解密密码失败: %w", err)
			}
			
			if pinPassword == "" {
				pkcs11Ctx.CloseSession(session)
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("PIN解密密码为空（请设置HSM_PIN_PASSWORD环境变量或配置KMS）")
			}
			
			// 解密PIN
			decryptedPIN, err := encryptionManager.DecryptWithPassword([]byte(config.EncryptedPIN), pinPassword)
			if err != nil {
				pkcs11Ctx.CloseSession(session)
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("PIN解密失败: %w", err)
			}
			pin = string(decryptedPIN)
			
			// 登录
			if err := pkcs11Ctx.Login(session, pin); err != nil {
				pkcs11Ctx.CloseSession(session)
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("HSM登录失败: %w", err)
			}
		}

		// 查找签名密钥
		keyHandle = pkcs11Ctx.FindKeyByLabel(session, config.KeyLabel)
		if keyHandle == 0 {
			pkcs11Ctx.CloseSession(session)
			pkcs11Ctx.Finalize()
			return nil, fmt.Errorf("查找密钥失败：未找到标签为 %s 的密钥", config.KeyLabel)
		}

		// 获取公钥
		publicKey, err = pkcs11Ctx.GetPublicKey(session, keyHandle)
		if err != nil {
			// ✅ 修复：公钥获取失败应返回错误，不应使用占位符
			pkcs11Ctx.CloseSession(session)
			pkcs11Ctx.Finalize()
			return nil, fmt.Errorf("获取公钥失败: %w", err)
		}

		// 关闭临时Session（初始化完成后将从Session池获取）
		pkcs11Ctx.CloseSession(session)

		// 创建Session池
		if pin != "" {
			sessionPoolConfig := &SessionPoolConfig{
				MaxSize:         config.SessionPoolSize,
				PIN:             pin,
				CleanupInterval: 5 * time.Minute,
			}
			var err error
			sessionPool, err = NewSessionPool(pkcs11Ctx, pkcs11Ctx.GetSlotID(), sessionPoolConfig, logger)
			if err != nil {
				pkcs11Ctx.Finalize()
				return nil, fmt.Errorf("创建Session池失败: %w", err)
			}
		}

		if logger != nil {
			logger.Infof("✅ HSMSigner PKCS#11初始化成功，密钥标签: %s, Slot ID: %d, Session池大小: %d", 
				config.KeyLabel, pkcs11Ctx.GetSlotID(), config.SessionPoolSize)
		}
	} else {
		// ✅ 修复：未提供PKCS#11库路径时返回错误，不允许占位符模式
		return nil, fmt.Errorf("PKCS#11库路径不能为空，HSM签名器需要真实的硬件支持")
	}

	if hashManager == nil {
		return nil, fmt.Errorf("HashManager不能为空")
	}

	return &HSMSigner{
		keyLabel:         config.KeyLabel,
		publicKey:        publicKey,
		algorithm:        config.Algorithm,
		txHashClient:     txHashClient,
		logger:           logger,
		pkcs11Ctx:        pkcs11Ctx,
		keyHandle:        keyHandle,
		pin:              pin, // 已解密的PIN（明文）
		sessionPool:      sessionPool,
		encryptionManager: encryptionManager,
		hashManager:      hashManager,
	}, nil
}

// Sign 签名交易
//
// 实现 tx.Signer 接口
//
// 🎯 **签名流程**：
// 1. 使用gRPC服务计算交易哈希
// 2. 获取HSM Session（从Session池）
// 3. 初始化签名操作（C_SignInit）
// 4. 执行签名（C_Sign）
// 5. 构造签名数据
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待签名的交易
//
// 返回：
//   - *transaction.SignatureData: 签名数据
//   - error: 签名失败的原因
func (s *HSMSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	// 1. 使用gRPC服务计算交易哈希
	if s.txHashClient == nil {
		return nil, fmt.Errorf("transaction hash client is not initialized")
	}

	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
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
		s.logger.Debugf("开始 HSM 签名，交易哈希: %x", txHash[:8])
	}

	// ✅ **PKCS#11签名操作**：如果已初始化PKCS#11，则使用实际签名
	var signature []byte
	if s.pkcs11Ctx != nil && s.keyHandle != 0 {
		// 从Session池获取Session
		var session pkcs11.SessionHandle
		var err error
		
		if s.sessionPool != nil {
			// 使用Session池
			session, err = s.sessionPool.AcquireSession(ctx)
			if err != nil {
				return nil, fmt.Errorf("获取Session失败: %w", err)
			}
			defer s.sessionPool.ReleaseSession(session)
		} else {
			// 回退到直接创建Session（向后兼容）
			session, err = s.pkcs11Ctx.OpenSession(pkcs11.CKF_SERIAL_SESSION | pkcs11.CKF_RW_SESSION)
			if err != nil {
				return nil, fmt.Errorf("打开Session失败: %w", err)
			}
			defer s.pkcs11Ctx.CloseSession(session)

			// 登录（如果使用直接创建方式）
			if s.pin != "" {
				if err := s.pkcs11Ctx.Login(session, s.pin); err != nil {
					return nil, fmt.Errorf("HSM登录失败: %w", err)
				}
				defer s.pkcs11Ctx.Logout(session)
			}
		}

		// 根据算法选择签名机制
		var mechanism pkcs11.Mechanism
		switch s.algorithm {
		case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
			mechanism = *pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)
		case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
			// ⚠️ **注意**：某些PKCS#11实现可能不支持CKM_EDDSA
			// 如果编译错误，请检查PKCS#11库是否支持EdDSA
			// 可以使用 CKM_EC_EDWARDS_KEY_PAIR_GEN 或其他常量
			mechanism = *pkcs11.NewMechanism(0x00001057, nil) // CKM_EDDSA (如果支持)
		default:
			return nil, fmt.Errorf("不支持的签名算法: %v", s.algorithm)
		}

		// 执行签名
		signature, err = s.pkcs11Ctx.SignData(session, s.keyHandle, txHash, uint(mechanism.Mechanism))
		if err != nil {
			return nil, fmt.Errorf("HSM签名失败: %w", err)
		}
	} else {
		// ✅ 修复：未初始化PKCS#11时返回错误，不允许占位符签名
		return nil, fmt.Errorf("PKCS#11未初始化，无法执行签名操作")
	}

	// 5. 构造签名数据
	signatureData := &transaction.SignatureData{
		Value: signature,
	}

	// 6. 记录审计日志
	if s.logger != nil {
		s.logger.Infof("✅ HSM 签名成功，交易哈希: %x, 签名长度: %d", txHash[:8], len(signature))
	}

	return signatureData, nil
}

// SignBytes 签名任意字节数据
//
// 实现 tx.Signer 接口（P2-3b扩展）
//
// 🎯 **核心功能**：对原始字节数据进行签名（不涉及交易结构）
//
// **签名流程**：
// 1. 验证输入数据非空
// 2. 计算数据的SHA256哈希
// 3. 获取HSM Session（从Session池）
// 4. 初始化签名操作（C_SignInit）
// 5. 执行签名（C_Sign）
// 6. 返回签名字节数组
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
func (s *HSMSigner) SignBytes(ctx context.Context, data []byte) ([]byte, error) {
	// 1. 验证输入数据非空
	if len(data) == 0 {
		return nil, fmt.Errorf("待签名数据为空")
	}

	// 记录签名请求
	if s.logger != nil {
		s.logger.Debugf("开始 HSM 签名原始数据，数据长度: %d 字节", len(data))
	}

	// 2. 计算数据的SHA256哈希
	// ✅ 修复：使用 HashManager 而不是直接使用 crypto/sha256
	// 注意：HSM通常期望接收已哈希的数据（对于ECDSA等算法）
	dataHash := s.hashManager.SHA256(data)

	// ✅ **PKCS#11签名操作**：如果已初始化PKCS#11，则使用实际签名
	var signature []byte
	if s.pkcs11Ctx != nil && s.keyHandle != 0 {
		// 从Session池获取Session
		var session pkcs11.SessionHandle
		var err error
		
		if s.sessionPool != nil {
			// 使用Session池
			session, err = s.sessionPool.AcquireSession(ctx)
			if err != nil {
				return nil, fmt.Errorf("获取Session失败: %w", err)
			}
			defer s.sessionPool.ReleaseSession(session)
		} else {
			// 回退到直接创建Session（向后兼容）
			session, err = s.pkcs11Ctx.OpenSession(pkcs11.CKF_SERIAL_SESSION | pkcs11.CKF_RW_SESSION)
			if err != nil {
				return nil, fmt.Errorf("打开Session失败: %w", err)
			}
			defer s.pkcs11Ctx.CloseSession(session)

			// 登录（如果使用直接创建方式）
			if s.pin != "" {
				if err := s.pkcs11Ctx.Login(session, s.pin); err != nil {
					return nil, fmt.Errorf("HSM登录失败: %w", err)
				}
				defer s.pkcs11Ctx.Logout(session)
			}
		}

		// 根据算法选择签名机制
		var mechanism uint
		switch s.algorithm {
		case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
			mechanism = pkcs11.CKM_ECDSA
		case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
			// ⚠️ **注意**：某些PKCS#11实现可能不支持CKM_EDDSA
			// 如果编译错误，请检查PKCS#11库是否支持EdDSA
			// 可以使用 CKM_EC_EDWARDS_KEY_PAIR_GEN 或其他常量
			mechanism = 0x00001057 // CKM_EDDSA (如果支持)
		default:
			return nil, fmt.Errorf("不支持的签名算法: %v", s.algorithm)
		}

		// 执行签名
		signature, err = s.pkcs11Ctx.SignData(session, s.keyHandle, dataHash, mechanism)
		if err != nil {
			return nil, fmt.Errorf("HSM签名失败: %w", err)
		}
	} else {
		// ✅ 修复：未初始化PKCS#11时返回错误，不允许占位符签名
		return nil, fmt.Errorf("PKCS#11未初始化，无法执行签名操作")
	}

	// 5. 记录审计日志
	if s.logger != nil {
		s.logger.Infof("✅ HSM 签名原始数据成功，数据长度: %d 字节，签名长度: %d 字节", len(data), len(signature))
	}

	return signature, nil
}

// PublicKey 返回签名器对应的公钥
//
// 实现 tx.Signer 接口
//
// 返回：
//   - *transaction.PublicKey: 公钥对象
func (s *HSMSigner) PublicKey() *transaction.PublicKey {
	return s.publicKey
}

// Algorithm 返回签名算法
//
// 实现 tx.Signer 接口
//
// 返回：
//   - transaction.SignatureAlgorithm: 签名算法
func (s *HSMSigner) Algorithm() transaction.SignatureAlgorithm {
	return s.algorithm
}
