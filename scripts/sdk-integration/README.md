# SDK 集成测试环境配置

**版本**: 1.0  


---

## 📋 概述

本目录包含 SDK 集成测试专用的 WES 节点环境配置和启动脚本，为 Go/JS Client SDK 的集成测试提供统一的测试环境。

---

## 🎯 设计目标

### 统一测试环境

- **单节点 Devnet Profile**：`profiles/sdk-integration`
- **固定端口配置**：避免端口冲突
- **预置账户和合约**：快速启动测试
- **一键启动/停止**：简化开发者流程

### 环境隔离

- SDK 仓库不自己起节点，而是依赖 `weisyn.git` 提供的"SDK 集成测试专用环境"
- 测试环境与开发环境隔离，避免相互影响

---

## 🏗️ 目录结构

```
scripts/
  sdk-integration/
    README.md              # 本文档
    start.sh               # 启动 devnet + 预置数据
    stop.sh                # 停止 devnet
    config/                 # 配置文件
      profile.json         # SDK 集成测试 Profile
      accounts.json        # 预置账户配置
      contracts.json       # 预置合约配置
    fixtures/              # 预置数据
      genesis.json         # 创世块配置
      contracts/           # 预置合约
```

---

## ⚙️ 环境配置

### 固定端口

- **HTTP JSON-RPC**: `http://127.0.0.1:28680`
- **WebSocket**: `ws://127.0.0.1:28681`
- **gRPC**（如启用）: `127.0.0.1:28682`

### 预置账户

| 账户 | 地址 | 私钥环境变量 | 用途 |
|------|------|------------|------|
| Miner | `0x...` | `WES_TEST_PRIVKEY_MINER` | 出块账户，初始大余额 |
| User A | `0x...` | `WES_TEST_PRIVKEY_USER_A` | 普通用户 A，有初始 WES |
| User B | `0x...` | `WES_TEST_PRIVKEY_USER_B` | 普通用户 B |

> **注意**：实际私钥通过环境变量注入，不提交到仓库。

### 预置合约

- **标准 Token 合约**：已部署，地址记录在配置中
- **Staking 合约**：已部署（如需要）
- **Market 合约**：已部署（如需要）
- **Governance 合约**：已部署（如需要）

---

## 🚀 使用方式

### 启动环境

```bash
cd /Users/qinglong/go/src/chaincodes/WES/weisyn.git
./scripts/sdk-integration/start.sh
```

**启动脚本功能**：
1. 检查依赖（Go 版本、依赖包等）
2. 编译 WES 节点（如需要）
3. 启动单节点 devnet（使用 `profiles/sdk-integration`）
4. 预置账户和合约
5. 导出环境变量

**导出的环境变量**：
```bash
export WES_ENDPOINT_HTTP=http://127.0.0.1:28680
export WES_ENDPOINT_WS=ws://127.0.0.1:28681
export WES_TEST_PRIVKEY_MINER=0x...
export WES_TEST_PRIVKEY_USER_A=0x...
export WES_TEST_PRIVKEY_USER_B=0x...
```

### 停止环境

```bash
./scripts/sdk-integration/stop.sh
```

**停止脚本功能**：
1. 停止 WES 节点进程
2. 清理临时文件（可选）
3. 清理环境变量（可选）

### 验证环境

```bash
# 检查节点健康状态
curl http://127.0.0.1:28680/health

# 检查 JSON-RPC 是否可用
curl -X POST http://127.0.0.1:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"wes_blockNumber","params":[],"id":1}'
```

---

## 📝 配置文件说明

### profile.json

SDK 集成测试 Profile 配置：

```json
{
  "name": "sdk-integration",
  "description": "SDK 集成测试专用环境",
  "network": {
    "chainId": 1337,
    "networkId": 1337
  },
  "rpc": {
    "http": {
      "enabled": true,
      "host": "127.0.0.1",
      "port": 28680
    },
    "websocket": {
      "enabled": true,
      "host": "127.0.0.1",
      "port": 28681
    }
  },
  "mining": {
    "enabled": true,
    "miner": "WES_TEST_MINER"
  }
}
```

### accounts.json

预置账户配置：

```json
{
  "miner": {
    "address": "0x...",
    "balance": "1000000000000000000000",
    "description": "出块账户"
  },
  "userA": {
    "address": "0x...",
    "balance": "1000000000000000000",
    "description": "普通用户 A"
  },
  "userB": {
    "address": "0x...",
    "balance": "0",
    "description": "普通用户 B"
  }
}
```

### contracts.json

预置合约配置：

```json
{
  "token": {
    "name": "StandardToken",
    "address": "0x...",
    "bytecode": "0x...",
    "description": "标准 Token 合约"
  },
  "staking": {
    "name": "StakingContract",
    "address": "0x...",
    "bytecode": "0x...",
    "description": "Staking 合约"
  }
}
```

---

## 🔧 开发指南

### 添加新的预置合约

1. 在 `config/contracts.json` 中添加合约配置
2. 在 `fixtures/contracts/` 中添加合约字节码
3. 在 `start.sh` 中添加部署逻辑

### 修改端口配置

1. 修改 `config/profile.json` 中的端口配置
2. 更新本文档中的端口说明
3. 更新 SDK 测试中的默认端点配置

---

## 🐛 故障排查

### 问题 1：端口被占用

**错误信息**：
```
bind: address already in use
```

**解决方案**：
1. 检查端口是否被占用：`lsof -i :28680`
2. 停止占用端口的进程
3. 或修改 `config/profile.json` 中的端口配置

### 问题 2：节点启动失败

**错误信息**：
```
failed to start node
```

**解决方案**：
1. 检查日志文件：`data/logs/weisyn.log`
2. 检查配置文件格式是否正确
3. 检查数据目录权限

### 问题 3：预置合约部署失败

**错误信息**：
```
failed to deploy contract
```

**解决方案**：
1. 检查账户余额是否足够
2. 检查合约字节码是否正确
3. 检查网络连接是否正常

---

## 🔗 相关文档

- [Go SDK 集成测试设计](../../../sdk/client-sdk-go.git/test/integration/DESIGN.md)
- [JS SDK 集成测试设计](../../../sdk/client-sdk-js.git/tests/integration/DESIGN.md)
- [WES 节点配置文档](../../docs/reference/config.md)

---

  
**维护者**: WES Team

