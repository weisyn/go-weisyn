// Package crypto 提供WES系统的密钥管理服务接口定义
//
// kms.go: KMS（密钥管理服务）接口定义
//
// 🎯 **核心职责**：定义密钥管理服务的抽象接口，支持多种KMS提供商
//
// 💡 **设计理念**：
// - 接口抽象：通过接口隔离外部SDK依赖
// - 依赖注入：外部项目通过依赖注入提供具体实现
// - 最小化依赖：项目本身不依赖AWS SDK、Vault SDK等商业SDK
//
// 📋 **支持的KMS提供商**：
// - 环境变量：基础实现，用于开发/测试环境
// - AWS KMS：通过外部实现KMSClient接口提供
// - HashiCorp Vault：通过外部实现KMSClient接口提供
// - Azure Key Vault：通过外部实现KMSClient接口提供
// - 自定义KMS：通过实现KMSClient接口提供
package crypto

import (
	"context"
)

// KMSProvider 密钥管理服务提供者接口
//
// 🎯 **设计理念**：
// - 定义最小化的KMS操作接口
// - 支持多种KMS提供商（AWS、Vault、Azure等）
// - 通过依赖注入提供具体实现
//
// 💡 **使用场景**：
// - HSM PIN密码解密：从KMS获取PIN解密密码
// - 密钥加密存储：将敏感密钥加密存储到KMS
// - 密钥轮换：支持密钥自动轮换
//
// 📋 **实现要求**：
// - 所有实现必须线程安全
// - 所有实现必须支持上下文取消和超时
// - 所有实现必须提供详细的错误信息
type KMSProvider interface {
	// DecryptSecret 解密KMS中的加密密钥
	//
	// 参数：
	//   - ctx: 上下文对象（用于取消和超时控制）
	//   - keyID: KMS密钥ID（用于解密的密钥标识符）
	//   - ciphertext: 加密的数据（Base64编码或原始字节）
	//
	// 返回：
	//   - []byte: 解密后的明文数据
	//   - error: 解密失败的原因
	//
	// 💡 **使用示例**：
	//   plaintext, err := provider.DecryptSecret(ctx, "arn:aws:kms:...", encryptedData)
	DecryptSecret(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error)

	// GetSecret 从KMS获取密钥（明文）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: 密钥标识符（KMS特定格式，如Vault路径、AWS密钥ARN等）
	//
	// 返回：
	//   - []byte: 密钥明文数据
	//   - error: 获取失败的原因
	//
	// 💡 **使用示例**：
	//   secret, err := provider.GetSecret(ctx, "secret/data/hsm/pin")
	GetSecret(ctx context.Context, keyID string) ([]byte, error)

	// EncryptSecret 加密密钥到KMS
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: KMS密钥ID（用于加密的密钥标识符）
	//   - plaintext: 待加密的明文数据
	//
	// 返回：
	//   - []byte: 加密后的数据（Base64编码或原始字节）
	//   - error: 加密失败的原因
	//
	// 💡 **使用示例**：
	//   ciphertext, err := provider.EncryptSecret(ctx, "arn:aws:kms:...", plaintext)
	EncryptSecret(ctx context.Context, keyID string, plaintext []byte) ([]byte, error)
}

// PINPasswordProvider PIN密码提供者接口
//
// 🎯 **设计理念**：
// - 专门用于HSM PIN密码获取
// - 支持多种来源（环境变量、KMS、Vault等）
// - 简化HSM签名器的PIN密码管理
//
// 💡 **使用场景**：
// - HSM PIN密码解密：从KMS获取PIN解密密码
// - 环境变量回退：开发/测试环境使用环境变量
// - 密钥轮换：支持PIN密码自动轮换
//
// 📋 **实现要求**：
// - 所有实现必须线程安全
// - 所有实现必须支持上下文取消和超时
// - 所有实现必须提供详细的错误信息
type PINPasswordProvider interface {
	// GetPINPassword 获取PIN解密密码
	//
	// 参数：
	//   - ctx: 上下文对象（用于取消和超时控制）
	//   - kmsKeyID: KMS密钥ID（可选，某些实现可能不需要）
	//
	// 返回：
	//   - string: PIN解密密码（明文）
	//   - error: 获取失败的原因
	//
	// 💡 **使用示例**：
	//   pinPassword, err := provider.GetPINPassword(ctx, "arn:aws:kms:...")
	GetPINPassword(ctx context.Context, kmsKeyID string) (string, error)
}

// KMSClient KMS客户端接口（供外部实现）
//
// 🎯 **设计理念**：
// - 定义最小化的KMS操作接口
// - 外部项目可以实现此接口，集成AWS SDK、Vault SDK等
// - 通过依赖注入提供实现
//
// 💡 **实现方式**：
// - AWS KMS：实现Decrypt方法，调用AWS KMS Decrypt API
// - HashiCorp Vault：实现GetSecret方法，调用Vault API
// - Azure Key Vault：实现Decrypt方法，调用Azure Key Vault API
//
// 📋 **实现要求**：
// - 所有实现必须线程安全
// - 所有实现必须支持上下文取消和超时
// - 所有实现必须提供详细的错误信息
//
// ⚠️ **注意**：
// - 此接口由外部项目实现，本项目不提供具体实现
// - 实现者需要自行处理SDK依赖和认证
type KMSClient interface {
	// Decrypt 解密数据
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: KMS密钥ID（用于解密的密钥标识符）
	//   - ciphertext: 加密的数据（Base64编码或原始字节）
	//
	// 返回：
	//   - []byte: 解密后的明文数据
	//   - error: 解密失败的原因
	Decrypt(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error)

	// GetSecret 获取密钥（明文）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: 密钥标识符（KMS特定格式）
	//
	// 返回：
	//   - []byte: 密钥明文数据
	//   - error: 获取失败的原因
	GetSecret(ctx context.Context, keyID string) ([]byte, error)

	// Encrypt 加密数据
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - keyID: KMS密钥ID（用于加密的密钥标识符）
	//   - plaintext: 待加密的明文数据
	//
	// 返回：
	//   - []byte: 加密后的数据
	//   - error: 加密失败的原因
	Encrypt(ctx context.Context, keyID string, plaintext []byte) ([]byte, error)
}

