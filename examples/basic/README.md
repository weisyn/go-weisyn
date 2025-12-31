# Examples/Basic - 基础应用示例

## 【模块定位】

    **应用层面的完整示例集合**
    展示如何使用WES区块链构建真实应用场景的端到端解决方案

## 【设计原则】

- **应用导向**: 以实际业务场景为驱动
- **完整流程**: 从客户端到合约的全链路演示
- **初学者友好**: 详细注释和循序渐进的学习曲线
- **即用即学**: 可直接运行的完整应用示例

## 【核心职责】

### 🎯 主要功能
- 提供完整的区块链应用开发示例
- 展示客户端与智能合约的交互模式
- 演示真实业务场景的技术实现
- 桥接hello-world与复杂应用之间的学习鸿沟

### 🚀 应用场景
- 代币经济应用（支付、转账、余额管理）
- 数据存储应用（文档管理、内容分发）
- 实用工具应用（身份验证、资产追踪）

## 🆚 与contracts/templates的差异化价值

| 维度 | examples/basic | contracts/templates |
|------|----------------|-------------------|
| **学习重点** | 如何使用合约构建应用 | 如何开发智能合约 |
| **技术层次** | 应用开发层面 | 合约开发层面 |
| **目标用户** | 应用开发者、产品经理 | 智能合约开发者 |
| **内容形式** | 完整应用示例 | 可复用代码模板 |
| **技能要求** | 客户端开发、API调用 | TinyGo、WASM开发 |
| **学习产出** | 能构建区块链应用 | 能开发智能合约 |

## ⚠️ 重要说明

本目录中的示例处于不同的完成状态：

- **hello-world**：✅ 已完成，可直接运行，展示合约部署与调用的完整流程
- **token-transfer**：🚧 架构演示，展示应用代码结构，需参考接口文档进行实际对接
- **data-storage**：🚧 开发中，暂未完成

### API 对接说明
示例使用的 API 接口已统一更新为实际实现：
- 合约部署：`POST /api/v1/contract/deploy`（参数：`deployer_private_key`、`contract_file_path`、`config`）
- 合约调用：`POST /api/v1/contract/call`（参数：`caller_private_key`、`contract_address`、`method_name`、`parameters`、`execution_fee_limit`）
- 详细文档：`internal/api/http/handlers/contract.go` 和 `pkg/interfaces/tx/`

## 📁 文件结构

```
basic/
├── README.md                    # 本文档
├── hello-world/                 # ✅ 最基础的入门示例（可运行）
│   ├── README.md               # 详细的入门指南
│   ├── BEGINNER_README.md      # 新手专用说明
│   ├── CONCEPTS.md             # 区块链基础概念
│   ├── src/
│   │   └── hello_world.go      # 智能合约源码（超详细注释）
│   ├── scripts/
│   │   ├── build.sh           # 标准构建脚本
│   │   ├── deploy.sh          # 部署脚本（已更新接口）
│   │   └── interact.sh        # 交互脚本（已更新接口）
│   └── build/                 # 构建输出目录
├── token-transfer/             # 🚧 代币转账应用示例（架构演示）
│   ├── README.md              # 应用详细说明
│   ├── src/
│   │   ├── transfer_client.go  # 转账客户端主程序
│   │   ├── wallet_manager.go   # 钱包管理模块
│   │   └── transaction_builder.go # 交易构建模块
│   ├── scripts/
│   │   ├── setup.sh           # 环境搭建
│   │   ├── deploy_token.sh    # 部署代币合约
│   │   ├── run_demo.sh        # 运行完整演示
│   │   └── check_balance.sh   # 余额查询工具
│   ├── docs/
│   │   ├── CONCEPTS.md        # 代币相关概念
│   │   └── TROUBLESHOOTING.md # 问题排查指南
│   └── config/                # 配置文件目录
└── data-storage/              # 数据存储应用示例
    ├── README.md              # 应用详细说明
    ├── src/
    │   ├── storage_client.go   # 存储客户端主程序
    │   ├── data_manager.go     # 数据管理模块
    │   ├── query_engine.go     # 查询引擎模块
    │   └── integrity_checker.go # 数据完整性检查
    ├── scripts/
    │   ├── setup.sh           # 环境搭建
    │   ├── deploy_storage.sh  # 部署存储合约
    │   ├── run_demo.sh        # 运行存储演示
    │   └── query_data.sh      # 数据查询脚本
    └── docs/
        ├── CONCEPTS.md        # 存储相关概念
        └── API_GUIDE.md       # API使用指南
```

## 🎯 学习路径设计

### 阶段1：入门体验 (hello-world)
- **目标**: 理解区块链应用的基本概念
- **内容**: 最简单的智能合约交互
- **技能**: 基础概念理解、环境搭建
- **时间**: 1-2小时

### 阶段2：实用应用 (token-transfer)
- **目标**: 掌握代币经济应用开发
- **内容**: 完整的转账应用系统
- **技能**: 钱包管理、交易构建、状态查询
- **时间**: 4-6小时

### 阶段3：数据应用 (data-storage)
- **目标**: 理解去中心化数据存储
- **内容**: 数据存储和查询系统
- **技能**: 数据加密、索引构建、完整性验证
- **时间**: 4-6小时

### 阶段4：进阶学习
- **方向A**: [contracts/templates/learning](../../contracts/templates/learning/) - 学习合约开发
- **方向B**: [examples/applications](../applications/) - 构建复杂应用

## 💡 使用建议

### 对于应用开发者
1. 从`hello-world`开始理解基础概念
2. 通过`token-transfer`学习经济模型实现
3. 通过`data-storage`掌握数据处理技术
4. 进入`examples/applications`实践复杂场景

### 对于产品经理
1. 重点理解各示例的业务场景和价值
2. 关注用户体验和交互流程设计
3. 了解区块链技术的能力边界
4. 思考产品功能与技术实现的映射

### 对于技术架构师
1. 分析客户端与合约的交互架构
2. 理解数据流和状态管理机制
3. 评估性能、安全性和可扩展性
4. 设计适合业务的技术方案

## 🔧 技术特色

### 代码质量
- **超详细注释**: 每行关键代码都有说明
- **生活化比喻**: 用通俗例子解释技术概念
- **错误处理**: 完整的异常处理和用户提示
- **最佳实践**: 遵循行业标准和安全规范

### 学习体验
- **循序渐进**: 从简单到复杂的合理安排
- **即学即用**: 每个示例都可以独立运行
- **问题预判**: 提前解决常见问题和困惑
- **跨平台**: 支持Windows、macOS、Linux

### 实用价值
- **真实场景**: 模拟实际业务需求
- **完整流程**: 覆盖开发、部署、运行全过程
- **可扩展性**: 代码结构支持功能扩展
- **生产就绪**: 代码质量达到生产标准

## 🚫 约束条件

- **❌ 禁止空目录**: 每个子目录必须包含完整的功能实现
- **❌ 禁止仅文档**: 必须有可运行的代码示例
- **❌ 禁止重复**: 避免与contracts/templates功能重叠
- **❌ 禁止复杂度**: 保持适合初学者的复杂度水平

## 🎉 开始学习

选择适合您当前水平的起点：

- **🔰 完全新手**: 从 [hello-world](./hello-world/) 开始
- **🚀 有基础**: 直接体验 [token-transfer](./token-transfer/)
- **💾 数据关注**: 探索 [data-storage](./data-storage/)

每个示例都提供了详细的README和使用指南，祝您学习愉快！✨