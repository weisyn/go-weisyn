# 签名器依赖注入配置指南

## 📋 概述

本文档说明如何更新依赖注入配置，以支持 `NewHSMSigner` 和 `NewKMSSigner` 的新签名（需要 `hashManager` 参数）。

## ✅ 已完成的修复

### 1. ModuleInput 已包含 HashManager

`ModuleInput` 结构体已经包含 `HashManager` 字段（第121行），无需修改：

```go
type ModuleInput struct {
    fx.In
    
    // ...
    HashManager crypto.HashManager `optional:"false"`
    // ...
}
```

### 2. LocalSigner 已更新

`LocalSigner` 的创建已更新，使用 `input.HashManager`（虽然当前 LocalSigner 不需要 HashManager，但为未来扩展预留）。

## 🔧 如何添加 KMS/HSM 签名器支持

### 方式1：替换 LocalSigner（推荐）

在 `internal/core/tx/module.go` 中，注释掉 LocalSigner 的提供，添加 KMSSigner 或 HSMSigner：

```go
// 注释掉 LocalSigner
// fx.Annotate(
//     func(input ModuleInput) (tx.Signer, error) {
//         // ... LocalSigner 创建代码
//     },
//     fx.As(new(tx.Signer)),
// ),

// 添加 KMSSigner
fx.Annotate(
    func(input ModuleInput, kmsClient signer.KMSClient) (tx.Signer, error) {
        signerConfig := input.ConfigProvider.GetSigner()
        kmsConfig := signerConfig.GetKMSSignerConfig()
        
        config := &signer.KMSSignerConfig{
            KeyID:         kmsConfig.KeyID,
            Algorithm:     kmsConfig.Algorithm,
            RetryCount:    kmsConfig.RetryCount,
            RetryDelay:    time.Duration(kmsConfig.RetryDelayMs) * time.Millisecond,
            SignTimeout:   time.Duration(kmsConfig.SignTimeoutMs) * time.Millisecond,
            Environment:   kmsConfig.Environment,
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

// 或添加 HSMSigner
fx.Annotate(
    func(input ModuleInput) (tx.Signer, error) {
        signerConfig := input.ConfigProvider.GetSigner()
        hsmConfig := signerConfig.GetHSMSignerConfig()
        
        config := &hsm.Config{
            KeyLabel:          hsmConfig.KeyLabel,
            Algorithm:         hsmConfig.Algorithm,
            LibraryPath:       hsmConfig.LibraryPath,
            EncryptedPIN:      hsmConfig.EncryptedPIN,
            KMSKeyID:          hsmConfig.KMSKeyID,
            PINPasswordProvider: nil, // 或注入 PINPasswordProvider
            SessionPoolSize:   hsmConfig.SessionPoolSize,
            Environment:       hsmConfig.Environment,
        }
        
        hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
        
        // ✅ 使用 input.HashManager 和 input.EncryptionManager
        return hsm.NewHSMSigner(
            config,
            input.TransactionHashServiceClient,
            input.EncryptionManager, // ✅ 注入 EncryptionManager
            input.HashManager,       // ✅ 注入 HashManager
            input.Logger,
        )
    },
    fx.As(new(tx.Signer)),
),
```

### 方式2：提供 KMSClient（KMSSigner 需要）

如果使用 KMSSigner，需要提供 KMSClient 实现：

```go
// 提供 AWS KMS 客户端
fx.Provide(
    func(ctx context.Context) (signer.KMSClient, error) {
        // 实现 AWS KMS 客户端
        return NewAWSKMSClient(ctx)
    },
),
```

### 方式3：提供 PINPasswordProvider（HSMSigner 可选）

如果使用 HSMSigner 并需要从 KMS 获取 PIN，需要提供 PINPasswordProvider：

```go
// 提供 PIN 密码提供者
fx.Provide(
    func(ctx context.Context) (hsm.PINPasswordProvider, error) {
        // 实现 KMS PIN 密码提供者
        return hsm.NewAWSKMSPINPasswordProvider(ctx)
    },
),
```

## 📝 完整示例

### 示例1：使用 KMSSigner（AWS KMS）

```go
// module.go
fx.Provide(
    // 1. 提供 AWS KMS 客户端
    func(ctx context.Context) (signer.KMSClient, error) {
        cfg, err := config.LoadDefaultConfig(ctx)
        if err != nil {
            return nil, err
        }
        return NewAWSKMSClient(cfg), nil
    },
    
    // 2. 提供 KMSSigner
    fx.Annotate(
        func(input ModuleInput, kmsClient signer.KMSClient) (tx.Signer, error) {
            signerConfig := input.ConfigProvider.GetSigner()
            kmsConfig := signerConfig.GetKMSSignerConfig()
            
            config := &signer.KMSSignerConfig{
                KeyID:       kmsConfig.KeyID,
                Algorithm:   kmsConfig.Algorithm,
                RetryCount:  3,
                RetryDelay:  100 * time.Millisecond,
                SignTimeout: 5 * time.Second,
                Environment: kmsConfig.Environment,
            }
            
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
)
```

### 示例2：使用 HSMSigner（带 KMS PIN）

```go
// module.go
fx.Provide(
    // 1. 提供 PIN 密码提供者
    func(ctx context.Context) (hsm.PINPasswordProvider, error) {
        return hsm.NewAWSKMSPINPasswordProvider(ctx)
    },
    
    // 2. 提供 HSMSigner
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
            
            return hsm.NewHSMSigner(
                config,
                input.TransactionHashServiceClient,
                input.EncryptionManager, // ✅ 注入 EncryptionManager
                input.HashManager,       // ✅ 注入 HashManager
                input.Logger,
            )
        },
        fx.As(new(tx.Signer)),
    ),
)
```

## ⚠️ 注意事项

1. **ModuleInput 已包含所需依赖**：
   - ✅ `HashManager` - 已包含（第121行）
   - ✅ `EncryptionManager` - 需要检查是否已包含

2. **向后兼容**：
   - LocalSigner 不需要 HashManager，但为未来扩展预留
   - 如果未提供 HashManager，会在运行时返回错误

3. **配置系统**：
   - 确保配置系统支持 KMS/HSM 签名器配置
   - 参考 `internal/config/tx/signer/config.go`

## 🔍 检查清单

- [ ] `ModuleInput` 包含 `HashManager` ✅
- [ ] `ModuleInput` 包含 `EncryptionManager`（HSM需要）
- [ ] 更新 `module.go` 中的签名器提供者
- [ ] 实现 `KMSClient`（KMSSigner需要）
- [ ] 实现 `PINPasswordProvider`（HSM KMS PIN需要）
- [ ] 更新配置系统支持新参数
- [ ] 测试依赖注入配置

