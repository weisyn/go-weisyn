# 第一笔交易

---

## 概述

本指南介绍如何在 WES 上发起你的第一笔交易，包括创建账户、查询余额和发送转账。

---

## 前置条件

- 已启动 WES 节点（参考 [本地快速开始](./quickstart-local.md) 或 [Docker 快速开始](./quickstart-docker.md)）
- 节点处于开发模式或已开启挖矿

---

## 创建账户

### 使用 CLI 创建

```bash
# 创建新账户
wes-node account create

# 输出示例：
# Address: wes1abc123...
# Private Key: (已保存到 keystore)
# 
# ⚠️ 请妥善保管私钥！
```

### 导入已有私钥

```bash
# 导入私钥
wes-node account import --private-key <your_private_key>
```

### 查看账户列表

```bash
# 列出所有账户
wes-node account list

# 输出示例：
# 0: wes1abc123... (default)
# 1: wes1def456...
```

---

## 获取测试代币

### 开发模式下

在开发模式下，第一个账户会自动获得测试代币。

```bash
# 查看余额
wes-node account balance --address wes1abc123...

# 输出示例：
# Balance: 1000000 WES
```

### 使用水龙头

如果连接到测试网络，可以使用水龙头获取测试代币：

```bash
curl -X POST https://faucet.testnet.weisyn.io/api/claim \
  -H "Content-Type: application/json" \
  -d '{"address": "wes1abc123..."}'
```

---

## 发送交易

### 使用 CLI 发送

```bash
# 发送转账交易
wes-node tx send \
  --from wes1abc123... \
  --to wes1def456... \
  --amount 100

# 输出示例：
# Transaction submitted
# TxHash: 0x789...
# Status: Pending
```

### 使用 API 发送

```bash
curl -X POST http://localhost:8545/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "from": "wes1abc123...",
    "to": "wes1def456...",
    "amount": "100"
  }'

# 响应示例：
# {
#   "txHash": "0x789...",
#   "status": "pending"
# }
```

---

## 查询交易

### 查询交易状态

```bash
# 使用 CLI
wes-node tx get --hash 0x789...

# 使用 API
curl http://localhost:8545/api/v1/tx/0x789...
```

### 交易状态说明

| 状态 | 说明 |
|------|------|
| `pending` | 已提交，等待确认 |
| `confirmed` | 已被包含在区块中 |
| `finalized` | 已达到最终确认 |
| `failed` | 交易失败 |

---

## 查询余额变化

### 发送方余额

```bash
wes-node account balance --address wes1abc123...

# 输出示例：
# Balance: 999900 WES (减少 100 + 手续费)
```

### 接收方余额

```bash
wes-node account balance --address wes1def456...

# 输出示例：
# Balance: 100 WES
```

---

## 完整示例

### 步骤 1：启动节点

```bash
wes-node start --dev --mine --api
```

### 步骤 2：创建两个账户

```bash
# 创建发送方账户
wes-node account create
# Address: wes1sender...

# 创建接收方账户
wes-node account create
# Address: wes1receiver...
```

### 步骤 3：检查发送方余额

```bash
wes-node account balance --address wes1sender...
# Balance: 1000000 WES
```

### 步骤 4：发送交易

```bash
wes-node tx send \
  --from wes1sender... \
  --to wes1receiver... \
  --amount 100
# TxHash: 0x789...
```

### 步骤 5：等待确认

```bash
# 等待几秒后查询
wes-node tx get --hash 0x789...
# Status: confirmed
```

### 步骤 6：验证余额

```bash
# 发送方
wes-node account balance --address wes1sender...
# Balance: 999900 WES

# 接收方
wes-node account balance --address wes1receiver...
# Balance: 100 WES
```

---

## 常见问题

### Q: 交易一直处于 pending 状态

A: 检查是否开启了挖矿：
```bash
wes-node start --dev --mine
```

### Q: 余额不足错误

A: 确保发送方有足够的余额（包括手续费）：
```bash
wes-node account balance --address <from_address>
```

### Q: 交易失败

A: 查看交易详情和错误信息：
```bash
wes-node tx get --hash <tx_hash> --verbose
```

---

## 下一步

- [核心概念](../concepts/) - 深入理解 WES 的技术架构
- [合约开发教程](../tutorials/contracts/) - 学习智能合约开发
- [API 参考](../reference/api/) - 完整的 API 文档

