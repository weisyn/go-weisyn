# 签名器真实实现指南

## 📋 概述

本文档说明如何真实实现以下三个问题：

1. **构造函数签名变更** - 更新依赖注入配置
2. **Session 池改进** - 实现真实的 Session 有效性检查
3. **KMS PIN 集成** - 实现真实的 KMS 集成

## ✅ 问题1：构造函数签名变更

### 已完成的修复

- ✅ `ModuleInput` 已包含 `HashManager`（第121行）
- ✅ `ModuleInput` 已添加 `EncryptionManager`（可选，HSM需要）
- ✅ `NewHSMSigner` 和 `NewKMSSigner` 已更新签名，需要 `hashManager` 参数

### 如何更新依赖注入配置

#### 步骤1：检查 ModuleInput

`ModuleInput` 结构体已包含所需依赖：

```go
type ModuleInput struct {
    // ...
    HashManager       crypto.HashManager       `optional:"false"`
    EncryptionManager crypto.EncryptionManager `optional:"true"` // ✅ 已添加
    // ...
}
```

#### 步骤2：更新 LocalSigner（当前默认）

当前 `module.go` 中的 LocalSigner 创建代码已正确，无需修改（LocalSigner 不需要 HashManager，但为未来扩展预留）。

#### 步骤3：添加 KMSSigner 支持（可选）

如果需要使用 KMSSigner，在 `module.go` 中添加：

```go
// 提供 KMS 客户端（需要实现 signer.KMSClient 接口）
fx.Provide(
    func(ctx context.Context) (signer.KMSClient, error) {
        // 实现 AWS KMS、GCP KMS 或 Azure Key Vault 客户端
        return NewAWSKMSClient(ctx), nil
    },
),

// 提供 KMSSigner（替换 LocalSigner）
fx.Annotate(
    func(input ModuleInput, kmsClient signer.KMSClient) (tx.Signer, error) {
        signerConfig := input.ConfigProvider.GetSigner()
        kmsConfig := signerConfig.GetKMSSignerConfig()
        
        config := &signer.KMSSignerConfig{
            KeyID:       kmsConfig.KeyID,
            Algorithm:   kmsConfig.Algorithm,
            RetryCount:  kmsConfig.RetryCount,
            RetryDelay:  time.Duration(kmsConfig.RetryDelayMs) * time.Millisecond,
            SignTimeout: time.Duration(kmsConfig.SignTimeoutMs) * time.Millisecond,
            Environment: kmsConfig.Environment,
        }
        
        // ✅ 使用 input.HashManager
        return signer.NewKMSSigner(
            config,
            kmsClient,
            input.TransactionHashServiceClient,
            input.HashManager, // ✅ 注入 HashManager
            input.Logger,
        )
    },
    fx.As(new(tx.Signer)),
),
```

#### 步骤4：添加 HSMSigner 支持（可选）

如果需要使用 HSMSigner，在 `module.go` 中添加：

```go
// 提供 PIN 密码提供者（可选，如果使用 KMS PIN）
fx.Provide(
    func(ctx context.Context) (hsm.PINPasswordProvider, error) {
        // 实现 KMS PIN 密码提供者（见问题3）
        return hsm.NewAWSKMSPINPasswordProvider(ctx)
    },
),

// 提供 HSMSigner（替换 LocalSigner）
fx.Annotate(
    func(input ModuleInput, pinProvider hsm.PINPasswordProvider) (tx.Signer, error) {
        signerConfig := input.ConfigProvider.GetSigner()
        hsmConfig := signerConfig.GetHSMSignerConfig()
        
        config := &hsm.Config{
            KeyLabel:          hsmConfig.KeyLabel,
            Algorithm:         hsmConfig.Algorithm,
            LibraryPath:       hsmConfig.LibraryPath,
            EncryptedPIN:      hsmConfig.EncryptedPIN,
            KMSKeyID:          hsmConfig.KMSKeyID,
            PINPasswordProvider: pinProvider, // ✅ 注入 PIN 密码提供者
            SessionPoolSize:   hsmConfig.SessionPoolSize,
            Environment:       hsmConfig.Environment,
        }
        
        hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
        
        // ✅ 使用 input.HashManager 和 input.EncryptionManager
        return hsm.NewHSMSigner(
            config,
            input.TransactionHashServiceClient,
            input.EncryptionManager, // ✅ 注入 EncryptionManager
            input.HashManager,        // ✅ 注入 HashManager
            input.Logger,
        )
    },
    fx.As(new(tx.Signer)),
),
```

## ✅ 问题2：Session 池改进

### 已完成的修复

- ✅ **条件变量等待**：已实现 `sync.Cond` 等待机制（`session_pool.go:144-169`）
- ✅ **Session 有效性检查**：已实现真实的 PKCS#11 API 调用（`session_pool.go:258-287`）
- ✅ **GetSessionInfo 方法**：已在 `pkcs11_wrapper.go` 中实现（第261-281行）

### 实现细节

#### 1. 条件变量等待

```go
// session_pool.go:144-169
// ✅ 真实实现：使用条件变量等待可用Session
for {
    // 检查是否有可用Session
    for _, session := range p.sessions {
        if !p.inUse[session] && p.isSessionValid(session) {
            p.inUse[session] = true
            return session, nil
        }
    }

    // 检查超时
    select {
    case <-ctx.Done():
        return 0, fmt.Errorf("获取Session超时: %w", ctx.Err())
    default:
    }

    // 等待Session释放（使用条件变量）
    p.cond.Wait()
}
```

#### 2. Session 有效性检查

```go
// session_pool.go:258-287
// ✅ 真实实现：调用 PKCS#11 API 检查Session状态
func (p *SessionPool) isSessionValid(session pkcs11.SessionHandle) bool {
    info, err := p.ctx.GetSessionInfo(session)
    if err != nil {
        // Session 无效
        return false
    }

    // 检查 Session 状态（State != 0 表示有效）
    if info.State == 0 {
        return false
    }

    return true
}
```

#### 3. GetSessionInfo 方法

```go
// pkcs11_wrapper.go:261-281
// ✅ 真实实现：调用 PKCS#11 C_GetSessionInfo API
func (c *PKCS11Context) GetSessionInfo(session pkcs11.SessionHandle) (pkcs11.SessionInfo, error) {
    info, err := c.ctx.GetSessionInfo(session)
    if err != nil {
        return pkcs11.SessionInfo{}, fmt.Errorf("GetSessionInfo失败: %w", err)
    }
    return info, nil
}
```

### 使用说明

Session 池现在支持：
- ✅ 并发安全的 Session 获取和释放
- ✅ 条件变量等待，避免忙等待
- ✅ Context 超时控制
- ✅ 真实的 Session 状态检查

## ✅ 问题3：KMS PIN 集成

### 已完成的修复

- ✅ **PINPasswordProvider 接口**：已定义接口和示例实现（`pin.go:28-122`）
- ✅ **EnvPINPasswordProvider**：环境变量提供者（已实现）
- ✅ **GetPINPasswordFromKMS**：支持通过 provider 获取密码（已实现）
- ✅ **Config 扩展**：添加 `KMSKeyID` 和 `PINPasswordProvider` 字段

### 实现方式

#### 方式1：使用环境变量（当前默认）

```go
// 无需额外配置，自动使用环境变量 HSM_PIN_PASSWORD
config := &hsm.Config{
    KeyLabel:     "my-key",
    LibraryPath:  "/usr/lib/softhsm/libsofthsm2.so",
    EncryptedPIN: "AQICAHh...", // 加密的PIN
    // PINPasswordProvider 为 nil，自动使用环境变量
}
```

#### 方式2：实现 PINPasswordProvider 接口

##### AWS KMS 实现示例

```go
package hsm

import (
    "context"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/service/kms"
    "github.com/aws/aws-sdk-go-v2/config"
)

// AWSKMSPINPasswordProvider AWS KMS PIN密码提供者
type AWSKMSPINPasswordProvider struct {
    kmsClient *kms.Client
    secretKeyID string // KMS密钥ID（用于解密加密的PIN）
}

// NewAWSKMSPINPasswordProvider 创建AWS KMS PIN密码提供者
func NewAWSKMSPINPasswordProvider(ctx context.Context, secretKeyID string) (*AWSKMSPINPasswordProvider, error) {
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, fmt.Errorf("加载AWS配置失败: %w", err)
    }
    
    return &AWSKMSPINPasswordProvider{
        kmsClient:   kms.NewFromConfig(cfg),
        secretKeyID: secretKeyID,
    }, nil
}

// GetPINPassword 从AWS KMS获取PIN解密密码
func (p *AWSKMSPINPasswordProvider) GetPINPassword(kmsKeyID string) (string, error) {
    // 从配置或环境变量获取加密的PIN密码
    encryptedPIN := os.Getenv("HSM_ENCRYPTED_PIN_PASSWORD")
    if encryptedPIN == "" {
        return "", fmt.Errorf("环境变量HSM_ENCRYPTED_PIN_PASSWORD未设置")
    }
    
    encryptedPINBytes := []byte(encryptedPIN) // Base64解码
    
    // 调用AWS KMS Decrypt API
    result, err := p.kmsClient.Decrypt(ctx, &kms.DecryptInput{
        CiphertextBlob: encryptedPINBytes,
        KeyId:          &p.secretKeyID,
    })
    if err != nil {
        return "", fmt.Errorf("AWS KMS解密失败: %w", err)
    }
    
    return string(result.Plaintext), nil
}
```

##### HashiCorp Vault 实现示例

```go
package hsm

import (
    "context"
    "fmt"
    
    "github.com/hashicorp/vault/api"
)

// VaultPINPasswordProvider HashiCorp Vault PIN密码提供者
type VaultPINPasswordProvider struct {
    client     *api.Client
    secretPath string
}

// NewVaultPINPasswordProvider 创建Vault PIN密码提供者
func NewVaultPINPasswordProvider(vaultAddr, token, secretPath string) (*VaultPINPasswordProvider, error) {
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

// GetPINPassword 从Vault获取PIN解密密码
func (p *VaultPINPasswordProvider) GetPINPassword(kmsKeyID string) (string, error) {
    secret, err := p.client.Logical().Read(p.secretPath)
    if err != nil {
        return "", fmt.Errorf("读取Vault密钥失败: %w", err)
    }
    
    if secret == nil || secret.Data == nil {
        return "", fmt.Errorf("Vault密钥不存在: %s", p.secretPath)
    }
    
    // 从 Vault 的 data 字段获取密码
    data, ok := secret.Data["data"].(map[string]interface{})
    if !ok {
        return "", fmt.Errorf("Vault密钥格式无效")
    }
    
    password, ok := data["pin_password"].(string)
    if !ok {
        return "", fmt.Errorf("Vault密钥缺少pin_password字段")
    }
    
    return password, nil
}
```

### 使用示例

#### 在依赖注入中使用

```go
// module.go
fx.Provide(
    // 1. 创建 PIN 密码提供者
    func(ctx context.Context) (hsm.PINPasswordProvider, error) {
        // 方式1：AWS KMS
        return hsm.NewAWSKMSPINPasswordProvider(ctx, "arn:aws:kms:...")
        
        // 方式2：HashiCorp Vault
        // return hsm.NewVaultPINPasswordProvider(
        //     "https://vault.example.com:8200",
        //     os.Getenv("VAULT_TOKEN"),
        //     "secret/data/hsm/pin",
        // )
    },
    
    // 2. 创建 HSMSigner
    fx.Annotate(
        func(input ModuleInput, pinProvider hsm.PINPasswordProvider) (tx.Signer, error) {
            config := input.ConfigProvider.GetSigner().GetHSMSignerConfig()
            hsmConfig := &hsm.Config{
                KeyLabel:          config.KeyLabel,
                Algorithm:         config.Algorithm,
                LibraryPath:       config.LibraryPath,
                EncryptedPIN:      config.EncryptedPIN,
                KMSKeyID:          config.KMSKeyID,
                PINPasswordProvider: pinProvider, // ✅ 注入 provider
                SessionPoolSize:   config.SessionPoolSize,
                Environment:       config.Environment,
            }
            
            hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
            
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
    environment: "production"
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
    environment: "production"
```

## ✅ 总结

### 已完成的实现

1. **构造函数签名变更** ✅
   - `ModuleInput` 已包含 `HashManager` 和 `EncryptionManager`
   - `NewHSMSigner` 和 `NewKMSSigner` 已更新签名
   - 提供了依赖注入配置示例

2. **Session 池改进** ✅
   - 实现了条件变量等待机制
   - 实现了真实的 Session 有效性检查（调用 PKCS#11 API）
   - 添加了 `GetSessionInfo` 方法

3. **KMS PIN 集成** ✅
   - 定义了 `PINPasswordProvider` 接口
   - 实现了 `EnvPINPasswordProvider`（环境变量）
   - 提供了 AWS KMS 和 Vault 实现示例
   - 更新了 `Config` 结构体支持 KMS 配置

### 下一步操作

1. **实现 KMS 客户端**：
   - 根据实际使用的 KMS 服务（AWS/GCP/Azure/Vault）实现 `PINPasswordProvider`
   - 参考 `KMS_INTEGRATION_GUIDE.md` 中的示例代码

2. **更新配置系统**：
   - 在 `internal/config/tx/signer/config.go` 中添加 KMS 相关配置字段
   - 更新配置解析逻辑

3. **测试**：
   - 测试依赖注入配置
   - 测试 Session 池的条件变量等待
   - 测试 KMS PIN 集成

## 📚 相关文档

- `DEPENDENCY_INJECTION_GUIDE.md` - 依赖注入配置详细指南
- `KMS_INTEGRATION_GUIDE.md` - KMS PIN 集成详细指南
- `README.md` - HSM 签名器使用文档

