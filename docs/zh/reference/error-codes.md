# 错误码参考

---

## 概述

本文档定义了 WES 系统中使用的错误码及其含义。

---

## 错误码格式

错误码格式：`WES-XXXX`

- `WES`：错误域前缀
- `XXXX`：4 位数字错误码

---

## 通用错误 (0xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-0001 | InternalError | 内部错误 |
| WES-0002 | InvalidParameter | 参数无效 |
| WES-0003 | NotFound | 资源未找到 |
| WES-0004 | AlreadyExists | 资源已存在 |
| WES-0005 | PermissionDenied | 权限不足 |
| WES-0006 | Timeout | 操作超时 |
| WES-0007 | Unavailable | 服务不可用 |

---

## 交易错误 (1xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-1001 | InvalidTxFormat | 交易格式无效 |
| WES-1002 | InvalidSignature | 签名无效 |
| WES-1003 | InsufficientBalance | 余额不足 |
| WES-1004 | DuplicateTx | 重复交易 |
| WES-1005 | TxNotFound | 交易未找到 |
| WES-1006 | TxExpired | 交易已过期 |
| WES-1007 | TxRejected | 交易被拒绝 |
| WES-1008 | InvalidNonce | Nonce 无效 |

---

## UTXO 错误 (2xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-2001 | UTXONotFound | UTXO 未找到 |
| WES-2002 | UTXOAlreadySpent | UTXO 已被消费 |
| WES-2003 | InvalidOutPoint | OutPoint 无效 |
| WES-2004 | InvalidUnlockProof | 解锁证明无效 |
| WES-2005 | ValueMismatch | 价值不守恒 |

---

## 执行错误 (3xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-3001 | ExecutionFailed | 执行失败 |
| WES-3002 | OutOfGas | 计算资源耗尽 |
| WES-3003 | InvalidContract | 合约无效 |
| WES-3004 | MethodNotFound | 方法未找到 |
| WES-3005 | InvalidProof | 证明无效 |
| WES-3006 | ProofGenerationFailed | 证明生成失败 |

---

## 资源错误 (4xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-4001 | ResourceNotFound | 资源未找到 |
| WES-4002 | InvalidResourceFormat | 资源格式无效 |
| WES-4003 | ResourceTooLarge | 资源过大 |
| WES-4004 | ResourceHashMismatch | 资源哈希不匹配 |

---

## 网络错误 (5xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-5001 | ConnectionFailed | 连接失败 |
| WES-5002 | PeerNotFound | 节点未找到 |
| WES-5003 | MessageTooLarge | 消息过大 |
| WES-5004 | ProtocolError | 协议错误 |

---

## 共识错误 (6xxx)

| 错误码 | 名称 | 说明 |
|--------|------|------|
| WES-6001 | InvalidBlock | 区块无效 |
| WES-6002 | InvalidPoW | PoW 无效 |
| WES-6003 | ForkDetected | 检测到分叉 |
| WES-6004 | ReorgFailed | 重组失败 |

---

## 错误处理示例

### Go

```go
if err != nil {
    if wesErr, ok := err.(*wes.Error); ok {
        switch wesErr.Code {
        case "WES-1003":
            // 处理余额不足
        case "WES-3001":
            // 处理执行失败
        default:
            // 处理其他错误
        }
    }
}
```

### JavaScript

```javascript
try {
    await client.sendTransaction(tx);
} catch (err) {
    if (err.code === 'WES-1003') {
        console.log('余额不足');
    } else {
        console.log('错误:', err.message);
    }
}
```

---

## 相关文档

- [API 参考](./api/) - API 错误响应
- [故障排查](../how-to/troubleshoot/) - 问题诊断

