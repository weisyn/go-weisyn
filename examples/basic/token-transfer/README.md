# Token Transfer Application Example

## 📖 概述

这是一个**演示性质**的代币转账应用示例，展示代币转账应用的架构设计、交易构建流程和客户端代码结构。

⚠️ **当前状态**：本示例为演示代码，展示应用架构与逻辑流程，**暂未对接真实的 WES API 或 pkg/interfaces**。

## 🎯 学习目标

- 理解代币转账应用的整体架构设计
- 学习交易构建器（TransactionBuilder）的设计模式
- 掌握钱包管理（WalletManager）的基本概念
- 了解客户端与合约交互的流程框架

## 🔄 实际对接说明

如需对接真实 WES 区块链，请参考以下接口：
- **合约调用**：`pkg/interfaces/tx/ContractService.CallContract()`
- **交易管理**：`pkg/interfaces/tx/TransactionManager`
- **资产服务**：`pkg/interfaces/tx/AssetService`
- **HTTP API**：`POST /api/v1/contract/call`、`POST /api/v1/transactions/*`

详细接口文档请查看 `pkg/interfaces/README.md` 和 `internal/api/http/handlers/`。

## 💡 与contracts/templates的区别

| 方面 | contracts/templates/learning | examples/basic/token-transfer |
|------|------------------------------|-------------------------------|
| **学习重点** | 如何开发代币合约 | 如何使用代币合约构建应用 |
| **内容层次** | 合约开发层面 | 应用开发层面 |
| **目标用户** | 智能合约开发者 | 应用开发者 |
| **技能要求** | TinyGo、WASM开发 | 客户端开发、API调用 |

## 📁 文件结构

```
token-transfer/
├── README.md                    # 本文档
├── src/
│   ├── transfer_client.go      # 转账客户端主程序
│   ├── wallet_manager.go       # 钱包管理模块
│   └── transaction_builder.go  # 交易构建模块
├── scripts/
│   ├── setup.sh               # 环境搭建脚本
│   ├── deploy_token.sh        # 部署代币合约
│   ├── run_demo.sh            # 运行转账演示
│   └── check_balance.sh       # 查询余额脚本
└── docs/
    ├── CONCEPTS.md            # 概念说明
    └── TROUBLESHOOTING.md     # 问题排查
```

## 🚀 快速开始

### 演示模式（当前可用）
```bash
# 进入示例目录
cd examples/basic/token-transfer

# 运行本地演示（无需区块链节点）
go run src/*.go
# 或查看代码了解应用架构
```

### 实际对接模式（需要开发）
要对接真实 WES 区块链，需要：
1. 引入 `pkg/interfaces` 中的服务接口
2. 替换 `simulate*` 方法为真实 API 调用
3. 参考 `examples/basic/hello-world` 的 HTTP API 调用方式
4. 或直接使用 `pkg/interfaces/tx` 中的 Go 接口

## 🎮 实际应用场景

这个示例模拟了以下真实场景：

1. **用户注册**：创建新的钱包地址
2. **初始分发**：给用户分发初始代币
3. **转账操作**：用户之间进行代币转账
4. **余额查询**：实时查询账户余额
5. **交易历史**：查看转账历史记录

## 📚 学习路径

```
hello-world          ←  最基础入门
    ↓
token-transfer       ←  当前位置（应用层面）
    ↓
contracts/templates  ←  深入合约开发
    ↓
examples/applications ←  复杂应用实践
```

## 🔗 相关资源

- [简单代币合约模板](../../../contracts/templates/learning/simple-token/)
- [代币开发指南](../../../contracts/BEGINNER_GUIDE.md)
- [完整应用示例](../../applications/)
