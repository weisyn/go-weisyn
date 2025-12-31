# 门限签名验证实现

## 📋 概述

本目录提供了完整的门限签名验证实现，支持 BLS 和 FROST 两种主流的门限签名方案。

## ✅ 已实现功能

### 1. BLS 门限签名验证 (`bls.go`)

**实现状态**：✅ **完整实现**

- ✅ 使用 `gnark-crypto` 的 BLS12-381 API
- ✅ 哈希到曲线映射（`HashToG2`）
- ✅ 配对验证（`PairingCheck`）
- ✅ 支持压缩和未压缩格式
- ✅ 完整的签名份额验证

**核心依赖**：
- `github.com/consensys/gnark-crypto/ecc/bls12-381`

**实现细节**：
- G1 公钥：48 字节（压缩）或 96 字节（未压缩）
- G2 签名：96 字节（压缩）或 192 字节（未压缩）
- 使用 `HashToG2` 进行哈希到曲线映射
- 使用 `PairingCheck` 进行配对验证：`e(pubKey, hashPoint) * e(-g1Gen, sig) == 1`

### 2. FROST Schnorr 门限签名验证 (`frost.go`)

**实现状态**：✅ **完整实现**

- ✅ 支持 Ed25519 曲线
- ✅ 支持 secp256k1 曲线
- ✅ 完整的组合签名验证
- ✅ 签名份额验证（简化版）

**核心依赖**：
- `crypto/ed25519`（标准库）
- `github.com/weisyn/v1/internal/core/infrastructure/crypto/frost` (封装dcrd依赖)

**实现细节**：
- Ed25519：64 字节签名（R: 32字节 + s: 32字节）
- secp256k1：65 字节签名（R: 33字节压缩 + s: 32字节）
- 使用标准 Schnorr 签名验证：`s*G == R + c*P`

**注意**：
- FROST 签名份额验证使用简化实现（需要聚合 R 的完整实现可在后续完善）

### 3. 默认验证器 (`verifier.go`)

**实现状态**：✅ **完整实现**

- ✅ 路由到具体的验证器实现（BLS 或 FROST）
- ✅ 完整的参数验证
- ✅ 统一的错误处理

## 🧪 测试

运行测试：
```bash
go test -v ./internal/core/infrastructure/crypto/threshold/...
```

## 📚 使用示例

### BLS 门限签名验证

```go
verifier := threshold.NewBLSThresholdVerifier()
valid, err := verifier.VerifyThresholdSignature(
    dataHash,
    combinedSignature,
    shares,
    groupPublicKey,
    threshold,
    totalParties,
    "BLS_THRESHOLD",
)
```

### FROST Schnorr 门限签名验证

```go
verifier := threshold.NewFROSTThresholdVerifier()
valid, err := verifier.VerifyThresholdSignature(
    dataHash,
    combinedSignature,
    shares,
    groupPublicKey,
    threshold,
    totalParties,
    "FROST_SCHNORR",
)
```

## 🔧 依赖管理

已安装的第三方依赖：
- `github.com/consensys/gnark-crypto`（BLS12-381）
- `github.com/miekg/pkcs11`（HSM PKCS#11）
- `github.com/coinbase/kryptology`（可选，用于高级 FROST 功能）

## ⚠️ 注意事项

1. **BLS 签名**：
   - 确保使用正确的 DST（Domain Separation Tag）
   - 支持压缩和未压缩格式，但建议使用压缩格式以节省空间

2. **FROST 签名**：
   - Ed25519 和 secp256k1 的签名格式不同
   - 签名份额验证需要聚合所有参与方的 nonce commitment（当前为简化实现）

3. **性能考虑**：
   - BLS 配对验证计算量较大，建议使用缓存或批处理
   - FROST 验证相对轻量，适合高频场景

## 📖 参考标准

- BLS 签名：RFC 9380 (BLS Signatures)
- FROST 签名：RFC 9483 (FROST: Flexible Round-Optimized Schnorr Threshold Signatures)
- BLS12-381 曲线：IETF draft-irtf-cfrg-bls-signature

