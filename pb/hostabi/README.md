# Host ABI / SDK 层 Protobuf 定义

> **协议类型**: 非共识层协议  
> **用途**: Host ABI / SDK 跨边界数据传输对象（DTO）  
> **创建时间**: 2025-10-17

---

## 📋 目录说明

本目录包含 **Host ABI / SDK 层** 的 Protobuf 定义，这些数据结构**不属于共识层协议**，仅用于：

1. **跨边界传输**：Host ABI ↔ SDK、SDK ↔ 钱包、CLI ↔ 服务端
2. **交易编排**：解锁计划、签名任务、批量签名等
3. **业务抽象**：高层业务对象，不上链

---

## 🔄 与共识层的关系

### 清晰分离

| **协议层**          | **定义位置**                                      | **用途**                           | **是否上链** |
|---------------------|---------------------------------------------------|------------------------------------|------------|
| **共识层**          | `pb/blockchain/block/transaction/transaction.proto` | 交易、输入/输出、锁定条件、证明等     | ✅ 上链     |
| **Host ABI / SDK 层** | `pb/hostabi/` (本目录)                            | 解锁计划、签名任务、批量签名等       | ❌ 不上链   |

### 为什么需要分离？

1. **共识纯净性**：共识层协议应该只包含验证所需的最小信息，避免污染
2. **演进独立性**：Host ABI 层可以快速迭代，不影响共识协议
3. **跨边界灵活性**：不同的客户端可以使用不同的 Host ABI 实现
4. **架构清晰性**：分层明确，职责单一

---

## 📁 文件列表

### txplanning.proto - 交易规划协议

**职责**：定义交易构建过程中的编排数据结构

**主要消息类型**：
- `UnlockPlan` - 解锁计划：分析未签名交易，生成签名任务清单
- `InputUnlockSpec` - 输入解锁规范：描述单个输入的解锁要求
- `SignTask` - 签名任务：描述单个签名任务的详细参数
- `*Spec` - 锁定规范：7种锁定条件的详细参数
  - `SingleKeySpec` - 单密钥锁定
  - `MultiKeySpec` - 多密钥锁定
  - `ContractSpec` - 合约锁定
  - `DelegationSpec` - 委托锁定
  - `ThresholdSpec` - 门限签名锁定
  - `TimeSpec` - 时间锁定
  - `HeightSpec` - 高度锁定
- `BatchSignRequest/Response` - 批量签名请求/响应
- `Signature` - 签名结果

**使用场景**：
- Off-chain 交易构建：钱包/CLI 构建交易时，生成 UnlockPlan 供用户签名
- Host 模式协调：合约执行完成后，协调器生成 UnlockPlan 并调用签名器
- 多签收集：多签收集器根据 UnlockPlan 识别所需签名人并收集签名
- KMS/HSM 签名：签名服务根据 SignTask 参数调用外部签名提供者

---

## 🚀 使用指南

### 生成 Go 代码

```bash
# 安装 protoc 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# 生成 Go 代码
protoc --go_out=. --go_opt=paths=source_relative \
    pb/hostabi/txplanning.proto
```

### 使用示例

#### 1. 生成解锁计划

```go
// UnlockingOrchestrator 生成解锁计划
unlockPlan, err := unlockingOrchestrator.GenerateUnlockPlan(ctx, unsignedTx, buildPlan)
if err != nil {
    return nil, err
}

// UnlockPlan 示例：
// {
//     draft_id: "draft_123",
//     unsigned_tx_hash: [0xabc...],
//     inputs: [
//         {
//             input_index: 0,
//             single: {
//                 required_address_hash: [0x123...],
//                 sighash_type: SIGHASH_ALL,
//                 algorithm: ECDSA_SECP256K1,
//                 signer_hint: "local"
//             }
//         }
//     ],
//     sign_tasks: [
//         {
//             input_index: 0,
//             hash_to_sign: [0xdef...],
//             sighash_type: SIGHASH_ALL,
//             algorithm: ECDSA_SECP256K1,
//             public_key_hint: [0x123...],
//             signer_hint: "local"
//         }
//     ]
// }
```

#### 2. 执行签名

```go
// SigningOrchestrator 根据 UnlockPlan 执行签名
signedTx, witnessTxID, err := signingOrchestrator.Sign(ctx, unsignedTx, unlockPlan, signers)
if err != nil {
    return nil, err
}

// 批量签名示例：
batchSignReq := &txplanning.BatchSignRequest{
    Tasks: unlockPlan.SignTasks,
}

batchSignResp, err := kmsSigner.SignBatch(ctx, batchSignReq)
if err != nil {
    return nil, err
}
```

#### 3. 多签收集

```go
// MultiSigCollector 根据 MultiKeySpec 收集签名
multiSpec := unlockPlan.Inputs[0].GetMulti()
collector := NewMultiSigCollector(multiSpec.RequiredSignatures, signers)

signatures, err := collector.Collect(ctx, unlockPlan.SignTasks)
if err != nil {
    return nil, err
}

// 组装 MultiKeyProof
multiKeyProof := &pb.MultiKeyProof{
    Signatures: signatures.Signatures,
    PublicKeys: signatures.PublicKeys,
    Algorithm:  pb.CryptoAlgorithm_ECDSA_SECP256K1,
    SighashType: pb.SighashType_SIGHASH_ALL,
}
```

---

## 🔧 扩展指南

### 新增锁定类型

如果需要支持新的锁定类型（如 `RingSignatureLock`），按以下步骤操作：

1. **在 `txplanning.proto` 中添加新的 *Spec**：

```protobuf
// RingSignatureSpec - 环签名锁定规范
message RingSignatureSpec {
  // 环成员公钥列表
  repeated bytes ring_members = 1;
  
  // 环签名参数
  bytes ring_parameters = 2;
}
```

2. **在 `InputUnlockSpec.requirement` 中添加新选项**：

```protobuf
message InputUnlockSpec {
  uint32 input_index = 1;
  
  oneof requirement {
    SingleKeySpec single = 10;
    MultiKeySpec multi = 11;
    // ... 其他锁定类型
    RingSignatureSpec ring_signature = 17;  // 新增
  }
}
```

3. **更新 `UnlockingOrchestrator` 分析逻辑**：

```go
case *pb.LockingCondition_RingSignatureLock:
    inputSpec = &InputUnlockSpec{
        InputIndex: uint32(inputIndex),
        Requirement: &InputUnlockSpec_RingSignature{
            RingSignature: &RingSignatureSpec{
                RingMembers:    cond.RingSignatureLock.RingMembers,
                RingParameters: cond.RingSignatureLock.RingParameters,
            },
        },
    }
```

4. **实现新的 Signer**：

```go
type RingSignatureSigner struct {
    // 环签名器实现
}

func (s *RingSignatureSigner) Sign(ctx context.Context, hashToSign []byte, ...) ([]byte, []byte, error) {
    // 生成环签名
}
```

### 新增签名提供者

如果需要支持新的签名提供者（如 Google Cloud KMS），按以下步骤操作：

1. **更新 `SignTask.signer_hint` 文档**：

```protobuf
// signer_hint: "local" | "kms:aws" | "kms:azure" | "kms:aliyun" | "kms:gcp" | "hsm:xxx"
string signer_hint = 6;
```

2. **实现新的 Signer**：

```go
type GCPKMSSigner struct {
    kmsClient *cloudkms.KeyManagementClient
}

func (s *GCPKMSSigner) Sign(ctx context.Context, hashToSign []byte, ...) ([]byte, []byte, error) {
    // 调用 GCP KMS API
}

func (s *GCPKMSSigner) Type() SignerType {
    return "kms:gcp"
}
```

3. **注册到 SigningOrchestrator**：

```go
orchestrator.RegisterSigner("kms:gcp", gcpKMSSigner)
```

---

## 📚 参考文档

- [TX 架构设计完整版](../../_docs/architecture/TX_UNIFIED_ARCHITECTURE.md)
- [TX 公共接口层](../../pkg/interfaces/tx/TX_ARCHITECTURE_V2_README.md)
- [TX 实现层](../../internal/core/tx/TX_IMPLEMENTATION_V2_README.md)
- [共识层交易协议](../blockchain/block/transaction/transaction.proto)
- [EUTXO 权利具体化理论](../../_docs/architecture/EUTXO_RIGHT_MATERIALIZATION_THEORY.md)

---

## 🔄 维护指南

### 版本兼容性

1. **向后兼容**：使用 Protobuf 的向后兼容特性
   - 保留字段编号
   - 新增字段使用 `optional`
   - 废弃字段使用 `reserved`

2. **版本标识**：在注释中标注版本
   ```protobuf
   // @since v2.0
   message NewSpec {
       // ...
   }
   ```

3. **迁移指南**：在 README 中记录重大变更

### 文档更新

每次修改协议后，必须更新：
1. 本 README 文件
2. 使用示例
3. 相关架构文档

### 代码生成

修改 proto 文件后，运行以下命令重新生成代码：

```bash
# 重新生成所有 Host ABI proto
make proto-hostabi

# 或者手动生成
protoc --go_out=. --go_opt=paths=source_relative \
    pb/hostabi/*.proto
```

---

> 📝 **文档维护**: 本目录随 TX 架构演进持续更新。  
> 🔄 **最后更新**: 2025-10-17  
> 📧 **维护者**: WES 核心团队

