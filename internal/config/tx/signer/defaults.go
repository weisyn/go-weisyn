package signer

import (
	"os"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// getDefaultLocalSignerConfig 获取默认本地签名器配置
//
// 🎯 **默认值策略**：
// - 私钥：优先从环境变量读取，否则使用测试私钥（仅测试环境）
// - 算法：默认使用 ECDSA secp256k1
// - 环境：从环境变量或默认值推断
func getDefaultLocalSignerConfig() LocalSignerConfig {
	// 1. 尝试从环境变量读取私钥
	privateKeyHex := os.Getenv("WES_SIGNER_PRIVATE_KEY")
	
	// 2. 如果环境变量未设置，尝试从配置文件路径读取
	if privateKeyHex == "" {
		keyPath := os.Getenv("WES_SIGNER_PRIVATE_KEY_PATH")
		if keyPath != "" {
			// 这里可以添加从文件读取的逻辑
			// 为了简化，暂时保持为空，由用户配置提供
		}
	}

	// 3. 如果仍未设置，且是测试环境，使用测试私钥
	// ⚠️ 注意：生产环境不应有默认私钥
	environment := getEnvironment()
	if privateKeyHex == "" && (environment == "testing" || environment == "development") {
		// 测试环境默认私钥（全1，仅用于测试）
		privateKeyHex = "1111111111111111111111111111111111111111111111111111111111111111"
	}

	return LocalSignerConfig{
		PrivateKeyHex: privateKeyHex,
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   environment,
	}
}

// getDefaultKMSSignerConfig 获取默认KMS签名器配置
func getDefaultKMSSignerConfig() KMSSignerConfig {
	return KMSSignerConfig{
		KeyID:          "", // 必须由用户配置提供
		Algorithm:      transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		RetryCount:     3,
		RetryDelayMs:   100,
		SignTimeoutMs:  5000,
		Environment:    getEnvironment(),
	}
}

// getDefaultHSMSignerConfig 获取默认HSM签名器配置
func getDefaultHSMSignerConfig() HSMSignerConfig {
	return HSMSignerConfig{
		KeyID:          "", // 必须由用户配置提供
		KeyLabel:       "", // 必须由用户配置提供
		Algorithm:      transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		LibraryPath:    "", // 必须由用户配置提供
		EncryptedPIN:   "", // 可选，如果使用KMS则不需要
		KMSKeyID:       "", // 可选，用于从KMS获取PIN解密密码
		KMSType:        "", // 可选，aws/vault/azure
		VaultAddr:      "", // 可选，Vault地址
		VaultToken:     "", // 可选，Vault Token
		VaultSecretPath: "", // 可选，Vault密钥路径
		SessionPoolSize: 10, // 默认Session池大小
		Endpoint:       "", // 兼容旧配置
		Username:       "",
		Password:       "",
		Environment:    getEnvironment(),
	}
}

// getEnvironment 获取环境标识
//
// 优先级：
// 1. 环境变量 ENV
// 2. 环境变量 ENVIRONMENT
// 3. 默认值 "development"
func getEnvironment() string {
	if envVar := os.Getenv("ENV"); envVar != "" {
		return envVar
	}
	if envVar := os.Getenv("ENVIRONMENT"); envVar != "" {
		return envVar
	}
	return "development"
}

// DefaultRetryDelay 获取默认重试延迟
func DefaultRetryDelay() time.Duration {
	return 100 * time.Millisecond
}

// DefaultSignTimeout 获取默认签名超时
func DefaultSignTimeout() time.Duration {
	return 5 * time.Second
}

