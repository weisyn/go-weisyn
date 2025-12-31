# 权限管理 JSON-RPC 方法说明

## 概述

权限管理功能通过通用的交易构建和提交 RPC 方法实现，遵循 EUTXO 模型。所有权限变更操作都通过标准的交易流程完成。

## 使用的 RPC 方法

### 1. 查询资源 UTXO

**方法**: `wes_getUTXO`

**用途**: 获取资源的当前 UTXO，包括锁定条件信息

**参数**:
```json
{
  "txId": "0x...",
  "outputIndex": 0
}
```

**返回**: UTXO 信息，包含 `output.locking_conditions`

### 2. 构建权限变更交易

**方法**: `wes_buildTransaction`

**用途**: 根据交易草稿（draft）构建未签名交易

**权限操作 Draft 格式**:

#### 所有权转移
```json
{
  "sign_mode": "defer_sign",
  "inputs": [
    {
      "tx_hash": "0x...",
      "output_index": 0,
      "is_reference_only": false
    }
  ],
  "outputs": [
    {
      "owner": "0x...",
      "output_type": "resource",
      "resource_output": { ... },
      "locking_conditions": [
        {
          "single_key_lock": {
            "required_address_hash": "0x...",
            "required_algorithm": "ECDSA_SECP256K1",
            "sighash_type": "SIGHASH_ALL"
          }
        }
      ]
    }
  ],
  "metadata": {
    "operation": "transfer_ownership",
    "memo": "转移给新所有者"
  }
}
```

#### 协作者管理
```json
{
  "metadata": {
    "operation": "update_collaborators",
    "required_signatures": 2,
    "collaborators_count": 3
  },
  "outputs": [
    {
      "locking_conditions": [
        {
          "multi_key_lock": {
            "required_signatures": 2,
            "authorized_keys": [
              { "value": "0x...", "algorithm": "ECDSA_SECP256K1" },
              { "value": "0x...", "algorithm": "ECDSA_SECP256K1" },
              { "value": "0x...", "algorithm": "ECDSA_SECP256K1" }
            ],
            "required_algorithm": "ECDSA_SECP256K1",
            "require_ordered_signatures": false,
            "sighash_type": "SIGHASH_ALL"
          }
        }
      ]
    }
  ]
}
```

#### 委托授权
```json
{
  "metadata": {
    "operation": "grant_delegation",
    "delegate_address": "0x...",
    "authorized_operations": "reference,execute,query",
    "expiry_blocks": 14400
  },
  "outputs": [
    {
      "locking_conditions": [
        { ...原有锁定条件... },
        {
          "delegation_lock": {
            "original_owner": "0x...",
            "allowed_delegates": ["0x..."],
            "authorized_operations": ["reference", "execute", "query"],
            "expiry_duration_blocks": 14400,
            "max_value_per_operation": "1000"
          }
        }
      ]
    }
  ]
}
```

#### 时间/高度锁
```json
{
  "metadata": {
    "operation": "set_time_lock",
    "unlock_timestamp": 1735689600
  },
  "outputs": [
    {
      "locking_conditions": [
        {
          "time_lock": {
            "unlock_timestamp": 1735689600,
            "base_lock": { ...原有锁定条件... },
            "time_source": "TIME_SOURCE_BLOCK_TIMESTAMP"
          }
        }
      ]
    }
  ]
}
```

### 3. 计算签名哈希

**方法**: `wes_computeSignatureHashFromDraft`

**用途**: 计算交易的签名哈希，用于钱包签名

**参数**:
```json
{
  "draft": { ...交易草稿... },
  "input_index": 0,
  "sighash_type": "SIGHASH_ALL"
}
```

**返回**:
```json
{
  "hash": "0x...",
  "unsigned_tx": "0x..."
}
```

### 4. 完成交易

**方法**: `wes_finalizeTransactionFromDraft`

**用途**: 将签名添加到交易中，完成交易构建

**参数**:
```json
{
  "draft": { ...交易草稿... },
  "unsigned_tx": "0x...",
  "input_index": 0,
  "sighash_type": "SIGHASH_ALL",
  "pubkey": "0x...",
  "signature": "0x..."
}
```

**返回**: 已签名交易的 hex 编码

### 5. 提交交易

**方法**: `wes_sendRawTransaction`

**用途**: 提交已签名的权限变更交易

**参数**: `["0x已签名交易hex"]`

**返回**:
```json
{
  "txHash": "0x...",
  "accepted": true
}
```

## 权限操作流程

### 完整流程示例（所有权转移）

```javascript
// 1. 查询当前资源 UTXO
const utxo = await client.call('wes_getUTXO', [{ txId, outputIndex }]);

// 2. 构建交易草稿
const draft = {
  sign_mode: 'defer_sign',
  inputs: [{ tx_hash: txId, output_index: outputIndex, is_reference_only: false }],
  outputs: [{ ...新锁定条件... }],
  metadata: { operation: 'transfer_ownership' }
};

// 3. 构建未签名交易
const buildResult = await client.call('wes_buildTransaction', [draft]);

// 4. 计算签名哈希
const hashResult = await client.call('wes_computeSignatureHashFromDraft', [{
  draft,
  input_index: 0,
  sighash_type: 'SIGHASH_ALL'
}]);

// 5. 钱包签名
const signature = wallet.signHash(hashResult.hash);

// 6. 完成交易
const signedTx = await client.call('wes_finalizeTransactionFromDraft', [{
  draft,
  unsigned_tx: hashResult.unsigned_tx,
  input_index: 0,
  sighash_type: 'SIGHASH_ALL',
  pubkey: '0x' + bytesToHex(wallet.publicKey),
  signature: '0x' + bytesToHex(signature)
}]);

// 7. 提交交易
const result = await client.call('wes_sendRawTransaction', [signedTx]);
```

## 权限变更识别

权限历史查询通过以下方式识别权限相关交易：

1. **metadata.operation**: 交易草稿中的 `metadata.operation` 字段标识操作类型
2. **方法名**: 交易的方法名可能包含权限相关关键词
3. **锁定条件变化**: 比较交易前后的锁定条件差异

## 错误处理

权限操作可能遇到的错误：

- `INVALID_PARAMS`: 参数格式错误（如无效的资源 ID、地址格式）
- `TX_FEE_TOO_LOW`: 交易费用过低
- `TX_CONFLICTS`: UTXO 冲突（资源已被消费）
- `INVALID_SIGNATURE`: 签名无效（权限不足）
- `RPC_NOT_IMPLEMENTED`: RPC 方法未实现（节点版本过旧）

## 相关文档

- [资源元数据标准化规范](./jsonrpc_resource_metadata.md)
- [权限管理实施报告](../../workbench/contract-workbench.git/_dev/PERMISSION_MANAGEMENT_IMPLEMENTATION.md)
- [EUTXO 锁定条件设计](../../workbench/contract-workbench.git/_dev/EXECUTABLE_RESOURCE_LOCKING_DESIGN.md)

