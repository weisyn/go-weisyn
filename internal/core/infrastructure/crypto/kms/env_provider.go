// Package kms 提供 KMS（密钥管理服务）实现
//
// 🎯 **核心职责**：实现密钥管理服务接口，提供PIN密码管理能力
//
// 💡 **设计理念**：
// - 接口抽象：通过接口隔离外部SDK依赖
// - 依赖注入：外部项目通过依赖注入提供具体实现
// - 最小化依赖：项目本身不依赖AWS SDK、Vault SDK等商业SDK
//
// 📋 **实现内容**：
// - EnvPINPasswordProvider：环境变量PIN密码提供者（真实实现）
// - KMSClientAdapter：KMS客户端适配器（供外部实现使用）
package kms

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// EnvPINPasswordProvider 环境变量PIN密码提供者
//
// ✅ **真实实现**：从环境变量 HSM_PIN_PASSWORD 读取PIN解密密码
//
// 🎯 **适用场景**：
// - 开发环境：快速配置
// - 测试环境：CI/CD自动化测试
// - 简单部署：单机部署场景
//
// 📋 **使用方式**：
// 1. 设置环境变量：export HSM_PIN_PASSWORD="your-pin-password"
// 2. 创建提供者：provider := kms.NewEnvPINPasswordProvider(logger)
// 3. 获取密码：password, err := provider.GetPINPassword(ctx, "")
type EnvPINPasswordProvider struct {
	logger log.Logger
}

// NewEnvPINPasswordProvider 创建环境变量PIN密码提供者
//
// 参数：
//   - logger: 日志服务（可选）
//
// 返回：
//   - crypto.PINPasswordProvider: PIN密码提供者实例
func NewEnvPINPasswordProvider(logger log.Logger) crypto.PINPasswordProvider {
	return &EnvPINPasswordProvider{
		logger: logger,
	}
}

// GetPINPassword 从环境变量获取PIN解密密码
//
// ✅ **真实实现**：从环境变量 HSM_PIN_PASSWORD 读取
//
// 参数：
//   - ctx: 上下文对象（用于取消和超时控制）
//   - kmsKeyID: KMS密钥ID（环境变量提供者不使用此参数，忽略）
//
// 返回：
//   - string: PIN解密密码（明文）
//   - error: 获取失败的原因
func (p *EnvPINPasswordProvider) GetPINPassword(ctx context.Context, kmsKeyID string) (string, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	password := os.Getenv("HSM_PIN_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("环境变量HSM_PIN_PASSWORD未设置")
	}

	if p.logger != nil {
		p.logger.Debugf("成功从环境变量获取PIN解密密码")
	}

	return password, nil
}

// KMSClientAdapter KMS客户端适配器
//
// ✅ **真实实现**：将外部提供的KMSClient适配为KMSProvider
//
// 🎯 **设计理念**：
// - 外部项目可以实现KMSClient接口（集成AWS SDK、Vault SDK等）
// - 通过此适配器将KMSClient转换为KMSProvider
// - 实现依赖注入和解耦
//
// 📋 **使用方式**：
// 1. 外部项目实现 crypto.KMSClient 接口
// 2. 使用 NewKMSProviderFromClient 创建适配器
// 3. 通过依赖注入提供KMSProvider
type KMSClientAdapter struct {
	client crypto.KMSClient
	logger log.Logger
}

// NewKMSProviderFromClient 从KMSClient创建KMSProvider
//
// ✅ **真实实现**：适配外部提供的KMSClient实现
//
// 参数：
//   - client: KMS客户端（外部实现）
//   - logger: 日志服务（可选）
//
// 返回：
//   - crypto.KMSProvider: KMS提供者实例
//
// 💡 **使用示例**：
//   // 外部项目实现KMSClient
//   type AWSKMSClient struct { ... }
//   func (c *AWSKMSClient) Decrypt(ctx, keyID, ciphertext) ([]byte, error) { ... }
//
//   // 创建适配器
//   kmsClient := &AWSKMSClient{...}
//   kmsProvider := kms.NewKMSProviderFromClient(kmsClient, logger)
func NewKMSProviderFromClient(client crypto.KMSClient, logger log.Logger) crypto.KMSProvider {
	return &KMSClientAdapter{
		client: client,
		logger: logger,
	}
}

// DecryptSecret 解密KMS中的加密密钥
//
// 实现 crypto.KMSProvider 接口
func (a *KMSClientAdapter) DecryptSecret(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error) {
	if a.client == nil {
		return nil, fmt.Errorf("KMS客户端未初始化")
	}

	return a.client.Decrypt(ctx, keyID, ciphertext)
}

// GetSecret 从KMS获取密钥（明文）
//
// 实现 crypto.KMSProvider 接口
func (a *KMSClientAdapter) GetSecret(ctx context.Context, keyID string) ([]byte, error) {
	if a.client == nil {
		return nil, fmt.Errorf("KMS客户端未初始化")
	}

	return a.client.GetSecret(ctx, keyID)
}

// EncryptSecret 加密密钥到KMS
//
// 实现 crypto.KMSProvider 接口
func (a *KMSClientAdapter) EncryptSecret(ctx context.Context, keyID string, plaintext []byte) ([]byte, error) {
	if a.client == nil {
		return nil, fmt.Errorf("KMS客户端未初始化")
	}

	return a.client.Encrypt(ctx, keyID, plaintext)
}

// NewPINPasswordProviderFromKMSProvider 从KMSProvider创建PINPasswordProvider
//
// ✅ **真实实现**：将KMSProvider适配为PINPasswordProvider
//
// 🎯 **使用场景**：
// - 当有KMSProvider实现时，可以用于获取PIN密码
// - 支持从KMS解密加密的PIN密码
//
// 参数：
//   - provider: KMS提供者
//   - encryptedPINKeyID: 加密PIN密码的KMS密钥ID
//   - encryptedPINBase64: 加密的PIN密码（Base64编码）
//   - logger: 日志服务（可选）
//
// 返回：
//   - crypto.PINPasswordProvider: PIN密码提供者实例
//
// 💡 **使用示例**：
//   kmsProvider := kms.NewKMSProviderFromClient(awsClient, logger)
//   pinProvider := kms.NewPINPasswordProviderFromKMSProvider(
//       kmsProvider,
//       "arn:aws:kms:...",
//       "AQICAHh...",
//       logger,
//   )
type KMSPINPasswordProvider struct {
	provider          crypto.KMSProvider
	encryptedPINKeyID string
	encryptedPINBase64 string
	logger            log.Logger
}

// NewPINPasswordProviderFromKMSProvider 从KMSProvider创建PINPasswordProvider
func NewPINPasswordProviderFromKMSProvider(
	provider crypto.KMSProvider,
	encryptedPINKeyID string,
	encryptedPINBase64 string,
	logger log.Logger,
) crypto.PINPasswordProvider {
	return &KMSPINPasswordProvider{
		provider:          provider,
		encryptedPINKeyID: encryptedPINKeyID,
		encryptedPINBase64: encryptedPINBase64,
		logger:            logger,
	}
}

// GetPINPassword 从KMS获取PIN解密密码
//
// ✅ **真实实现**：使用KMSProvider解密加密的PIN密码
//
// 参数：
//   - ctx: 上下文对象
//   - kmsKeyID: KMS密钥ID（可选，如果为空则使用encryptedPINKeyID）
//
// 返回：
//   - string: PIN解密密码（明文）
//   - error: 获取失败的原因
func (p *KMSPINPasswordProvider) GetPINPassword(ctx context.Context, kmsKeyID string) (string, error) {
	if p.provider == nil {
		return "", fmt.Errorf("KMS提供者未初始化")
	}

	// 使用提供的kmsKeyID或默认的encryptedPINKeyID
	keyID := kmsKeyID
	if keyID == "" {
		keyID = p.encryptedPINKeyID
	}

	if keyID == "" {
		return "", fmt.Errorf("KMS密钥ID不能为空")
	}

	// 获取加密的PIN密码
	encryptedPINBase64 := p.encryptedPINBase64
	if encryptedPINBase64 == "" {
		encryptedPINBase64 = os.Getenv("HSM_ENCRYPTED_PIN_PASSWORD")
	}

	if encryptedPINBase64 == "" {
		return "", fmt.Errorf("加密的PIN密码未设置（请设置HSM_ENCRYPTED_PIN_PASSWORD环境变量或配置encryptedPINBase64）")
	}

	// Base64解码
	encryptedPIN, err := base64.StdEncoding.DecodeString(encryptedPINBase64)
	if err != nil {
		return "", fmt.Errorf("Base64解码失败: %w", err)
	}

	// 调用KMS解密
	plaintext, err := p.provider.DecryptSecret(ctx, keyID, encryptedPIN)
	if err != nil {
		return "", fmt.Errorf("KMS解密失败: %w", err)
	}

	if p.logger != nil {
		p.logger.Debugf("成功从KMS获取PIN解密密码")
	}

	return string(plaintext), nil
}
