package signer

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// SignerOptions 签名器配置选项
//
// 🎯 **配置职责**：管理签名器相关的所有配置
//
// 📋 **配置分类**：
// - 用户配置：私钥路径（用户必须提供）
// - 内部配置：算法、环境标识等（有默认值）
type SignerOptions struct {
	// 本地签名器配置（LocalSigner）
	Local LocalSignerConfig `json:"local"`

	// KMS签名器配置（KMSSigner）
	KMS KMSSignerConfig `json:"kms"`

	// HSM签名器配置（HSMSigner）
	HSM HSMSignerConfig `json:"hsm"`
}

// LocalSignerConfig 本地签名器配置
//
// ⚠️ **安全警告**：仅用于开发/测试环境
type LocalSignerConfig struct {
	// 私钥路径（Hex编码字符串或文件路径）
	// 优先级：环境变量 > 配置文件 > 默认值（仅测试环境）
	PrivateKeyHex string `json:"private_key_hex"`

	// 签名算法
	Algorithm transaction.SignatureAlgorithm `json:"algorithm"`

	// 环境标识（development, testing, production）
	// 生产环境会自动拒绝使用LocalSigner
	Environment string `json:"environment"`
}

// KMSSignerConfig KMS签名器配置
type KMSSignerConfig struct {
	// KMS密钥标识符
	KeyID string `json:"key_id"`

	// 签名算法
	Algorithm transaction.SignatureAlgorithm `json:"algorithm"`

	// 重试配置
	RetryCount  int `json:"retry_count"`  // 重试次数
	RetryDelayMs int `json:"retry_delay_ms"` // 重试延迟（毫秒）

	// 超时配置
	SignTimeoutMs int `json:"sign_timeout_ms"` // 签名超时（毫秒）

	// 环境标识
	Environment string `json:"environment"`
}

// HSMSignerConfig HSM签名器配置
type HSMSignerConfig struct {
	// HSM密钥标识符（兼容旧配置）
	KeyID string `json:"key_id"`

	// HSM密钥标签（PKCS#11）
	KeyLabel string `json:"key_label"`

	// 签名算法
	Algorithm transaction.SignatureAlgorithm `json:"algorithm"`

	// PKCS#11库路径
	LibraryPath string `json:"library_path"`

	// PIN配置（加密存储的PIN）
	EncryptedPIN string `json:"encrypted_pin"`

	// KMS配置（用于从KMS获取PIN解密密码）
	KMSKeyID string `json:"kms_key_id"`   // KMS密钥ID（AWS KMS）
	KMSType  string `json:"kms_type"`     // KMS类型（aws, vault, azure）

	// HashiCorp Vault配置（如果KMSType为vault）
	VaultAddr      string `json:"vault_addr"`       // Vault地址
	VaultToken     string `json:"vault_token"`      // Vault Token
	VaultSecretPath string `json:"vault_secret_path"` // Vault密钥路径

	// Session池配置
	SessionPoolSize int `json:"session_pool_size"` // Session池大小

	// HSM连接配置（兼容旧配置，用于非PKCS#11的HSM）
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`

	// 环境标识
	Environment string `json:"environment"`
}

// UserSignerConfig 用户签名器配置（从configs/*/config.json加载）
//
// 📋 **配置来源**：用户配置文件
type UserSignerConfig struct {
	// 签名器类型（local, kms, hsm）
	Type string `json:"type"`

	// 本地签名器配置
	Local *LocalSignerConfig `json:"local,omitempty"`

	// KMS签名器配置
	KMS *KMSSignerConfig `json:"kms,omitempty"`

	// HSM签名器配置
	HSM *HSMSignerConfig `json:"hsm,omitempty"`
}

// New 创建签名器配置选项
//
// 参数：
//   - userConfig: 用户配置（从configs/*/config.json加载，可为nil）
//
// 返回：
//   - *SignerOptions: 签名器配置选项
func New(userConfig *UserSignerConfig) *SignerOptions {
	opts := &SignerOptions{
		Local: getDefaultLocalSignerConfig(),
		KMS:   getDefaultKMSSignerConfig(),
		HSM:   getDefaultHSMSignerConfig(),
	}

	// 应用用户配置
	if userConfig != nil {
		applyUserConfig(opts, userConfig)
	}

	return opts
}

// applyUserConfig 应用用户配置
func applyUserConfig(opts *SignerOptions, userConfig *UserSignerConfig) {
	// 应用本地签名器配置
	if userConfig.Local != nil {
		if userConfig.Local.PrivateKeyHex != "" {
			opts.Local.PrivateKeyHex = userConfig.Local.PrivateKeyHex
		}
		if userConfig.Local.Algorithm != transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN {
			opts.Local.Algorithm = userConfig.Local.Algorithm
		}
		if userConfig.Local.Environment != "" {
			opts.Local.Environment = userConfig.Local.Environment
		}
	}

	// 应用KMS签名器配置
	if userConfig.KMS != nil {
		if userConfig.KMS.KeyID != "" {
			opts.KMS.KeyID = userConfig.KMS.KeyID
		}
		if userConfig.KMS.Algorithm != transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN {
			opts.KMS.Algorithm = userConfig.KMS.Algorithm
		}
		if userConfig.KMS.RetryCount > 0 {
			opts.KMS.RetryCount = userConfig.KMS.RetryCount
		}
		if userConfig.KMS.RetryDelayMs > 0 {
			opts.KMS.RetryDelayMs = userConfig.KMS.RetryDelayMs
		}
		if userConfig.KMS.SignTimeoutMs > 0 {
			opts.KMS.SignTimeoutMs = userConfig.KMS.SignTimeoutMs
		}
		if userConfig.KMS.Environment != "" {
			opts.KMS.Environment = userConfig.KMS.Environment
		}
	}

	// 应用HSM签名器配置
	if userConfig.HSM != nil {
		if userConfig.HSM.KeyID != "" {
			opts.HSM.KeyID = userConfig.HSM.KeyID
		}
		if userConfig.HSM.KeyLabel != "" {
			opts.HSM.KeyLabel = userConfig.HSM.KeyLabel
		}
		if userConfig.HSM.Algorithm != transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN {
			opts.HSM.Algorithm = userConfig.HSM.Algorithm
		}
		if userConfig.HSM.LibraryPath != "" {
			opts.HSM.LibraryPath = userConfig.HSM.LibraryPath
		}
		if userConfig.HSM.EncryptedPIN != "" {
			opts.HSM.EncryptedPIN = userConfig.HSM.EncryptedPIN
		}
		if userConfig.HSM.KMSKeyID != "" {
			opts.HSM.KMSKeyID = userConfig.HSM.KMSKeyID
		}
		if userConfig.HSM.KMSType != "" {
			opts.HSM.KMSType = userConfig.HSM.KMSType
		}
		if userConfig.HSM.VaultAddr != "" {
			opts.HSM.VaultAddr = userConfig.HSM.VaultAddr
		}
		if userConfig.HSM.VaultToken != "" {
			opts.HSM.VaultToken = userConfig.HSM.VaultToken
		}
		if userConfig.HSM.VaultSecretPath != "" {
			opts.HSM.VaultSecretPath = userConfig.HSM.VaultSecretPath
		}
		if userConfig.HSM.SessionPoolSize > 0 {
			opts.HSM.SessionPoolSize = userConfig.HSM.SessionPoolSize
		}
		// 保留旧字段的兼容性
		if userConfig.HSM.Endpoint != "" {
			opts.HSM.Endpoint = userConfig.HSM.Endpoint
		}
		if userConfig.HSM.Username != "" {
			opts.HSM.Username = userConfig.HSM.Username
		}
		if userConfig.HSM.Password != "" {
			opts.HSM.Password = userConfig.HSM.Password
		}
		if userConfig.HSM.Environment != "" {
			opts.HSM.Environment = userConfig.HSM.Environment
		}
	}
}

// GetLocalSignerConfig 获取本地签名器配置
func (o *SignerOptions) GetLocalSignerConfig() *LocalSignerConfig {
	return &o.Local
}

// GetKMSSignerConfig 获取KMS签名器配置
func (o *SignerOptions) GetKMSSignerConfig() *KMSSignerConfig {
	return &o.KMS
}

// GetHSMSignerConfig 获取HSM签名器配置
func (o *SignerOptions) GetHSMSignerConfig() *HSMSignerConfig {
	return &o.HSM
}

