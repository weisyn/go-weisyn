# HSM KMS PIN 集成指南

---

## 📋 概述

本文档说明如何真实实现 KMS PIN 集成，支持从 AWS KMS、HashiCorp Vault、Azure Key Vault 等密钥管理服务获取 HSM PIN 解密密码。

**⚠️ 重要变更**：
- KMS接口和实现已迁移到 `internal/core/infrastructure/crypto/kms/`
- 请使用 `pkg/interfaces/infrastructure/crypto` 中的接口定义
- 请参考 `internal/core/infrastructure/crypto/kms/README.md` 获取最新使用方式

---

## 🎯 实现方式

### 方式1：实现 KMSClient 接口（推荐）

**步骤1**：实现 `crypto.KMSClient` 接口

```go
package yourproject

import (
    "context"
    "fmt"
    "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
    "github.com/aws/aws-sdk-go-v2/service/kms"
    "github.com/aws/aws-sdk-go-v2/config"
)

// AWSKMSClient AWS KMS客户端实现
type AWSKMSClient struct {
    kmsClient *kms.Client
}

func NewAWSKMSClient(ctx context.Context) (*AWSKMSClient, error) {
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, fmt.Errorf("加载AWS配置失败: %w", err)
    }
    
    return &AWSKMSClient{
        kmsClient: kms.NewFromConfig(cfg),
    }, nil
}

// Decrypt 实现 crypto.KMSClient 接口
func (c *AWSKMSClient) Decrypt(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error) {
    result, err := c.kmsClient.Decrypt(ctx, &kms.DecryptInput{
        CiphertextBlob: ciphertext,
        KeyId:          &keyID,
    })
    if err != nil {
        return nil, fmt.Errorf("AWS KMS解密失败: %w", err)
    }
    
    return result.Plaintext, nil
}

// GetSecret 实现 crypto.KMSClient 接口
func (c *AWSKMSClient) GetSecret(ctx context.Context, keyID string) ([]byte, error) {
    // AWS KMS不支持直接获取密钥，返回错误
    return nil, fmt.Errorf("AWS KMS不支持GetSecret操作")
}

// Encrypt 实现 crypto.KMSClient 接口
func (c *AWSKMSClient) Encrypt(ctx context.Context, keyID string, plaintext []byte) ([]byte, error) {
    result, err := c.kmsClient.Encrypt(ctx, &kms.EncryptInput{
        Plaintext: plaintext,
        KeyId:    &keyID,
    })
    if err != nil {
        return nil, fmt.Errorf("AWS KMS加密失败: %w", err)
    }
    
    return result.CiphertextBlob, nil
}
```

**步骤2**：创建KMSProvider和PINPasswordProvider

```go
import "github.com/weisyn/v1/internal/core/infrastructure/crypto/kms"

// 创建KMS客户端
awsClient, err := NewAWSKMSClient(ctx)
if err != nil {
    return nil, err
}

// 创建KMSProvider
kmsProvider := kms.NewKMSProviderFromClient(awsClient, logger)

// 创建PINPasswordProvider
pinProvider := kms.NewPINPasswordProviderFromKMSProvider(
    kmsProvider,
    "arn:aws:kms:us-east-1:123456789012:key/abc-def",
    os.Getenv("HSM_ENCRYPTED_PIN_PASSWORD"),
    logger,
)
```

**步骤3**：在HSM签名器中使用

```go
config := &hsm.Config{
    KeyLabel:         "my-signing-key",
    Algorithm:        transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
    LibraryPath:      "/usr/lib/softhsm/libsofthsm2.so",
    EncryptedPIN:     os.Getenv("HSM_ENCRYPTED_PIN"),
    PINPasswordProvider: pinProvider, // ✅ 注入PIN密码提供者
    SessionPoolSize:  10,
}

signer, err := hsm.NewHSMSigner(
    config,
    txHashClient,
    encryptionManager,
    hashManager,
    logger,
)
```

### 方式2：直接实现 PINPasswordProvider 接口

如果只需要PIN密码功能，可以直接实现 `crypto.PINPasswordProvider` 接口：

```go
package yourproject

import (
    "context"
    "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
    "github.com/hashicorp/vault/api"
)

// VaultPINPasswordProvider HashiCorp Vault PIN密码提供者
type VaultPINPasswordProvider struct {
    client     *api.Client
    secretPath string
}

func NewVaultPINPasswordProvider(vaultAddr, token, secretPath string) (crypto.PINPasswordProvider, error) {
    config := &api.Config{
        Address: vaultAddr,
    }
    
    client, err := api.NewClient(config)
    if err != nil {
        return nil, fmt.Errorf("创建Vault客户端失败: %w", err)
    }
    
    client.SetToken(token)
    
    return &VaultPINPasswordProvider{
        client:     client,
        secretPath: secretPath,
    }, nil
}

// GetPINPassword 实现 crypto.PINPasswordProvider 接口
func (p *VaultPINPasswordProvider) GetPINPassword(ctx context.Context, kmsKeyID string) (string, error) {
    secret, err := p.client.Logical().ReadWithContext(ctx, p.secretPath)
    if err != nil {
        return "", fmt.Errorf("读取Vault密钥失败: %w", err)
    }
    
    if secret == nil || secret.Data == nil {
        return "", fmt.Errorf("Vault密钥不存在: %s", p.secretPath)
    }
    
    // Vault KV v2格式
    data, ok := secret.Data["data"].(map[string]interface{})
    if ok {
        dataData, ok := data["data"].(map[string]interface{})
        if ok {
            password, ok := dataData["pin_password"].(string)
            if ok {
                return password, nil
            }
        }
    }
    
    // Vault KV v1格式
    password, ok := secret.Data["pin_password"].(string)
    if !ok {
        return "", fmt.Errorf("Vault密钥格式无效：缺少pin_password字段")
    }
    
    return password, nil
}
```

---

## 🔧 使用示例

### 在依赖注入中使用

```go
// module.go
import (
    "github.com/weisyn/v1/internal/core/infrastructure/crypto/kms"
    cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

fx.Provide(
    // 创建AWS KMS客户端（外部实现）
    func(ctx context.Context) (cryptointf.KMSClient, error) {
        return NewAWSKMSClient(ctx)
    },
    
    // 创建KMSProvider
    func(kmsClient cryptointf.KMSClient, logger log.Logger) cryptointf.KMSProvider {
        return kms.NewKMSProviderFromClient(kmsClient, logger)
    },
    
    // 创建PINPasswordProvider
    func(kmsProvider cryptointf.KMSProvider, logger log.Logger) cryptointf.PINPasswordProvider {
        return kms.NewPINPasswordProviderFromKMSProvider(
            kmsProvider,
            os.Getenv("HSM_KMS_KEY_ID"),
            os.Getenv("HSM_ENCRYPTED_PIN_PASSWORD"),
            logger,
        )
    },
    
    // 创建HSM签名器
    fx.Annotate(
        func(input ModuleInput, pinProvider cryptointf.PINPasswordProvider) (tx.Signer, error) {
            config := input.ConfigProvider.GetSigner().GetHSMSignerConfig()
            hsmConfig := &hsm.Config{
                KeyLabel:            config.KeyLabel,
                Algorithm:           config.Algorithm,
                LibraryPath:         config.LibraryPath,
                EncryptedPIN:        config.EncryptedPIN,
                PINPasswordProvider: pinProvider, // ✅ 注入PIN密码提供者
                SessionPoolSize:     config.SessionPoolSize,
            }
            return hsm.NewHSMSigner(
                hsmConfig,
                input.TransactionHashServiceClient,
                input.EncryptionManager,
                input.HashManager,
                input.Logger,
            )
        },
        fx.As(new(tx.Signer)),
    ),
)
```

---

## 📝 配置示例

### AWS KMS 配置

```yaml
signer:
  type: hsm
  hsm:
    key_label: "my-signing-key"
    library_path: "/usr/lib/softhsm/libsofthsm2.so"
    encrypted_pin: "AQICAHh..."  # Base64编码的加密PIN
    kms_key_id: "arn:aws:kms:us-east-1:123456789012:key/abc-def"
    session_pool_size: 10
```

**环境变量**：
```bash
export HSM_ENCRYPTED_PIN_PASSWORD="AQICAHh..."  # Base64编码的加密PIN
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
```

### HashiCorp Vault 配置

```yaml
signer:
  type: hsm
  hsm:
    key_label: "my-signing-key"
    library_path: "/usr/lib/softhsm/libsofthsm2.so"
    encrypted_pin: "AQICAHh..."
    vault_addr: "https://vault.example.com:8200"
    vault_secret_path: "secret/data/hsm/pin"
    session_pool_size: 10
```

**环境变量**：
```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_TOKEN="your-vault-token"
```

---

## ⚠️ 注意事项

1. **安全性**：PIN密码应加密存储，解密密码应从KMS获取
2. **错误处理**：KMS访问失败时应回退到环境变量（开发环境）
3. **性能**：考虑缓存PIN密码，避免频繁调用KMS API
4. **审计**：记录所有KMS访问日志，便于安全审计
5. **接口一致性**：确保实现的接口签名与 `pkg/interfaces/infrastructure/crypto` 中的定义一致

---

## 🔗 相关文档

- [KMS架构分析](./KMS_ARCHITECTURE_ANALYSIS.md)
- [KMS实现文档](../../../infrastructure/crypto/kms/README.md)
- [接口定义](../../../../pkg/interfaces/infrastructure/crypto/kms.go)
