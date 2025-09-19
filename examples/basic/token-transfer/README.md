# Token Transfer Application Example

## 📖 概述

这是一个完整的代币转账应用示例，展示如何基于WES区块链构建实用的代币转账应用。

## 🎯 学习目标

- 学习如何调用和使用已部署的代币合约
- 理解完整的代币转账应用流程
- 掌握客户端与合约的交互方式
- 了解实际应用场景的实现方法

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

### 1. 环境准备
```bash
# 进入示例目录
cd examples/basic/token-transfer

# 运行环境搭建
./scripts/setup.sh
```

### 2. 部署代币合约
```bash
# 部署测试代币合约
./scripts/deploy_token.sh
```

### 3. 运行转账演示
```bash
# 运行完整的转账演示
./scripts/run_demo.sh
```

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
