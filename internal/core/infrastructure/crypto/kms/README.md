# KMS - 密钥管理服务实现

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-23
- **所有者**：密码学基础设施组
- **适用范围**：WES 项目密钥管理服务实现

---

## 🎯 实现定位

**路径**：`internal/core/infrastructure/crypto/kms/`

**目的**：提供密钥管理服务的基础实现，支持PIN密码管理和KMS集成。

**核心原则**：
- ✅ 实现密码学接口（`pkg/interfaces/infrastructure/crypto`）
- ✅ 通过接口抽象隔离外部SDK依赖
- ✅ 支持依赖注入提供具体实现
- ✅ 最小化依赖：不依赖AWS SDK、Vault SDK等商业SDK

**解决什么问题**：
- 提供PIN密码管理的基础实现（环境变量提供者）
- 提供KMS客户端适配器（供外部实现使用）
- 支持多种KMS提供商（通过接口抽象）

**不解决什么问题**（边界）：
- ❌ 不实现具体的AWS KMS、Vault SDK集成（由外部项目实现）
- ❌ 不包含业务逻辑（由 tx 模块处理）
- ❌ 不管理持久化存储（由 storage 模块处理）

---

## 🏗️ 架构设计

### 接口层次

```
pkg/interfaces/infrastructure/crypto/
  ├── PINPasswordProvider      # PIN密码提供者接口
  ├── KMSProvider              # KMS提供者接口
  └── KMSClient                # KMS客户端接口（供外部实现）

internal/core/infrastructure/crypto/kms/
  ├── EnvPINPasswordProvider   # 环境变量提供者（真实实现）
  ├── KMSClientAdapter         # KMS客户端适配器（真实实现）
  └── KMSPINPasswordProvider   # KMS PIN密码提供者（真实实现）
```

### 依赖关系

```
tx/ports/signer/hsm/
  ↓ (使用)
pkg/interfaces/infrastructure/crypto.PINPasswordProvider
  ↓ (实现)
internal/core/infrastructure/crypto/kms.EnvPINPasswordProvider
```

---

## 📋 实现内容

### 1. EnvPINPasswordProvider（环境变量提供者）

**文件**：`env_provider.go`

**功能**：从环境变量 `HSM_PIN_PASSWORD` 读取PIN解密密码。

**使用方式**：
```go
import "github.com/weisyn/v1/internal/core/infrastructure/crypto/kms"

provider := kms.NewEnvPINPasswordProvider(logger)
password, err := provider.GetPINPassword(ctx, "")
```

**配置**：
```bash
export HSM_PIN_PASSWORD="your-pin-password"
```

### 2. KMSClientAdapter（KMS客户端适配器）

**文件**：`env_provider.go`

**功能**：将外部提供的 `KMSClient` 适配为 `KMSProvider`。

**使用方式**：
```go
// 外部项目实现KMSClient接口
type AWSKMSClient struct { ... }
func (c *AWSKMSClient) Decrypt(ctx, keyID, ciphertext) ([]byte, error) { ... }

// 创建适配器
kmsClient := &AWSKMSClient{...}
kmsProvider := kms.NewKMSProviderFromClient(kmsClient, logger)
```

### 3. KMSPINPasswordProvider（KMS PIN密码提供者）

**文件**：`env_provider.go`

**功能**：从KMS解密加密的PIN密码。

**使用方式**：
```go
kmsProvider := kms.NewKMSProviderFromClient(awsClient, logger)
pinProvider := kms.NewPINPasswordProviderFromKMSProvider(
    kmsProvider,
    "arn:aws:kms:...",
    "AQICAHh...",
    logger,
)
password, err := pinProvider.GetPINPassword(ctx, "")
```

---

## 🔧 外部KMS集成指南

### 步骤1：实现KMSClient接口

```go
package yourproject

import (
    "context"
    "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

type AWSKMSClient struct {
    // AWS KMS客户端实现
}

func (c *AWSKMSClient) Decrypt(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error) {
    // 调用AWS KMS Decrypt API
    // ...
}

func (c *AWSKMSClient) GetSecret(ctx context.Context, keyID string) ([]byte, error) {
    // 调用AWS KMS GetSecret API
    // ...
}

func (c *AWSKMSClient) Encrypt(ctx context.Context, keyID string, plaintext []byte) ([]byte, error) {
    // 调用AWS KMS Encrypt API
    // ...
}
```

### 步骤2：创建KMSProvider

```go
import "github.com/weisyn/v1/internal/core/infrastructure/crypto/kms"

awsClient := &AWSKMSClient{...}
kmsProvider := kms.NewKMSProviderFromClient(awsClient, logger)
```

### 步骤3：创建PINPasswordProvider

```go
pinProvider := kms.NewPINPasswordProviderFromKMSProvider(
    kmsProvider,
    "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
    os.Getenv("HSM_ENCRYPTED_PIN_PASSWORD"),
    logger,
)
```

### 步骤4：在HSM签名器中使用

```go
config := &hsm.Config{
    // ... 其他配置 ...
    PINPasswordProvider: pinProvider,
}

signer, err := hsm.NewHSMSigner(config, ...)
```

---

## 📝 注意事项

1. **接口定义**：所有接口定义在 `pkg/interfaces/infrastructure/crypto/kms.go`
2. **实现位置**：所有实现都在 `internal/core/infrastructure/crypto/kms/`
3. **依赖注入**：通过 `crypto.PINPasswordProvider` 接口进行依赖注入
4. **外部实现**：外部项目需要实现 `crypto.KMSClient` 接口

---

## 🔗 相关文档

- [KMS架构分析](../../../../tx/ports/signer/hsm/KMS_ARCHITECTURE_ANALYSIS.md)
- [KMS集成指南](../../../../tx/ports/signer/hsm/KMS_INTEGRATION_GUIDE.md)
- [实现指南](../../../../tx/ports/signer/IMPLEMENTATION_GUIDE.md)

