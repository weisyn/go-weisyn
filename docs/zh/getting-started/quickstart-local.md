# 本地快速开始

---

## 概述

本指南介绍如何在本地快速启动一个 WES 单节点环境，5 分钟内完成体验。

---

## 前置条件

- 已完成 [安装指南](./installation.md) 中的安装步骤
- 本地有足够的磁盘空间（至少 10 GB）

---

## 快速启动

### 步骤 1：初始化节点

```bash
# 创建数据目录
mkdir -p ~/.wes/data

# 初始化配置
wes-node init --datadir ~/.wes/data
```

### 步骤 2：启动节点

```bash
# 启动单节点（开发模式）
wes-node start --dev

# 或启动并开启挖矿
wes-node start --dev --mine
```

### 步骤 3：验证运行

```bash
# 检查节点状态
wes-node status

# 预期输出：
# Node Status: Running
# Height: 0
# Peers: 0
# Mining: true/false
```

---

## 开发模式说明

`--dev` 标志会启动一个预配置的开发环境：

- **单节点模式**：不需要连接其他节点
- **快速出块**：出块时间设置为 1 秒
- **预设账户**：自动创建测试账户
- **自动挖矿**：可选开启挖矿

---

## 使用 CLI 交互

### 查看区块

```bash
# 查看最新区块
wes-node block latest

# 查看指定高度的区块
wes-node block get --height 1
```

### 查看账户

```bash
# 列出账户
wes-node account list

# 查看账户余额
wes-node account balance --address <address>
```

### 发送交易

```bash
# 发送转账交易
wes-node tx send \
  --from <from_address> \
  --to <to_address> \
  --amount 100
```

---

## 使用 API 交互

### 启动 API 服务

API 服务默认在 `http://localhost:8545` 启动。

### 查询节点信息

```bash
curl http://localhost:8545/api/v1/node/info
```

### 查询最新区块

```bash
curl http://localhost:8545/api/v1/block/latest
```

### 提交交易

```bash
curl -X POST http://localhost:8545/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "from": "<from_address>",
    "to": "<to_address>",
    "amount": "100"
  }'
```

---

## 停止节点

```bash
# 优雅停止
wes-node stop

# 或直接 Ctrl+C
```

---

## 数据目录结构

```
~/.wes/
├── config.yaml     # 配置文件
└── data/
    ├── blocks/     # 区块数据
    ├── state/      # 状态数据
    ├── resources/  # 资源数据
    └── logs/       # 日志文件
```

---

## 常见问题

### Q: 节点启动后看不到新区块

A: 确保开启了挖矿：
```bash
wes-node start --dev --mine
```

### Q: API 无法访问

A: 检查 API 是否启用：
```bash
wes-node start --dev --api
```

### Q: 如何重置数据

A: 删除数据目录并重新初始化：
```bash
rm -rf ~/.wes/data
wes-node init --datadir ~/.wes/data
```

---

## 下一步

- [Docker 快速开始](./quickstart-docker.md) - 使用 Docker 启动
- [第一笔交易](./first-transaction.md) - 发起第一笔交易
- [核心概念](../concepts/) - 深入理解 WES

