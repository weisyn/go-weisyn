# WES 文档中心

欢迎来到 WES（微迅链）文档中心！

**WES 定义区块链的可验证计算范式，开启 AI 时代的去中心化智能。**

---

## 🎯 关于 WES

WES 是**第三代区块链**，通过 **ISPC（Intrinsic Self-Proving Computing，本征自证计算）** 可验证计算范式，突破了传统区块链的确定性共识限制。

### 核心创新

| 创新特性 | 定位 | 核心价值 |
|---------|------|---------|
| **ISPC 本征自证计算** | 计算执行层创新 | 单次执行 + 多点验证，支持 AI 等复杂计算链上运行 |
| **EUTXO 扩展模型** | 状态层创新 | 三层输出架构（Asset/Resource/State）+ 引用不消费模式 |
| **URES 统一资源管理** | 资源管理层创新 | 内容寻址存储，统一管理合约/AI模型/文件 |
| **PoW+XOR 距离选择共识** | 共识层创新 | 工作量证明 + XOR 距离选择，高性能共识 |

### 核心价值

- ✅ **AI Native**：行业唯一支持链上 AI 模型推理的区块链
- ✅ **企业应用支持**：支持长事务、外部系统集成，真正承载企业级业务
- ✅ **用户免 Gas 体验**：使用 CU（Compute Units，计算单位）作为内部算力计量，用户无需理解

---

## 🚀 快速开始

### 我是新人，应该从哪里开始？

**3 步快速上手：**

1. **了解 WES** → [WES 是什么](./concepts/what-is-wes.md) - 理解 WES 的定位和价值（10 分钟）
2. **快速体验** → [本地快速开始](./getting-started/quickstart-local.md) - 5 分钟跑起来
3. **开始开发** → [API 参考](./reference/api/) - 开始集成开发

---

## 🧭 入口关系（与仓库 README 的分工）

- **仓库根 [`README.md`](../../../README.md)**：产品/愿景入口（为什么做、能解决什么问题、快速体验），适合第一次了解 WES 的读者。
- **本文档中心 `docs/zh/`**：系统化学习与使用入口（入门 → 概念 → 教程 → 操作指南 → 参考），面向开发/架构/运维/贡献者。
- **内部研发知识库 `_dev/`**：协议规范与设计文档（Source of Truth），面向实现者；公开文档只摘要关键契约与边界，不复制全部规范文本。

---

## 👥 按角色导航

### 👨‍💻 开发者

**快速上手**
- [安装指南](./getting-started/installation.md) → [本地快速开始](./getting-started/quickstart-local.md) → [第一笔交易](./getting-started/first-transaction.md)

**深入学习**
- [核心概念](./concepts/) → [合约开发教程](./tutorials/contracts/) → [API 参考](./reference/api/)

**学习路径**：理解 WES → 部署节点 → 编写合约 → 集成应用

---

### 🏗️ 架构师

**了解系统架构**
- [架构总览](./concepts/architecture-overview.md) → [核心概念](./concepts/) → [ISPC 技术详解](./concepts/ispc.md)

**深入学习**
- [EUTXO 模型](./concepts/eutxo.md) → [URES 资源管理](./concepts/ures.md) → [PoW+XOR 共识](./concepts/consensus-pow-xor.md)

**学习路径**：系统架构 → 核心创新 → 技术实现

---

### 💼 决策者 / 产品经理

**了解项目价值**
- [WES 是什么](./concepts/what-is-wes.md) → [常见问题](./getting-started/faq.md)

**学习路径**：战略定位 → 竞争分析 → 应用场景

---

### 🔧 运维人员

**部署与运维**
- [安装指南](./getting-started/installation.md) → [部署指南](./how-to/deploy/) → [故障排查](./how-to/troubleshoot/)

**学习路径**：环境部署 → 故障排查 → 性能调优

---

## 📚 文档地图

```
docs/zh/
├── getting-started/           # 🚀 入门指南
│   ├── installation.md        # 安装指南
│   ├── quickstart-local.md    # 本地快速开始
│   ├── quickstart-docker.md   # Docker 快速开始
│   ├── first-transaction.md   # 第一笔交易
│   └── faq.md                 # 常见问题
│
├── concepts/                  # 💡 核心概念
│   ├── what-is-wes.md         # WES 是什么
│   ├── architecture-overview.md # 架构总览
│   ├── ispc.md                # ISPC 本征自证计算
│   ├── eutxo.md               # EUTXO 扩展模型
│   ├── ures.md                # URES 统一资源管理
│   ├── consensus-pow-xor.md   # PoW+XOR 共识
│   ├── transaction.md         # 交易模型
│   ├── block.md               # 区块模型
│   ├── chain.md               # 链模型
│   ├── network-and-topology.md # 网络与拓扑
│   ├── data-persistence.md    # 数据持久化
│   ├── privacy-and-proof.md   # 隐私与证明
│   ├── governance-and-compliance.md # 治理与合规
│   └── glossary.md            # 术语表
│
├── tutorials/                 # 📖 教程
│   ├── contracts/             # 合约开发教程
│   ├── ispc/                  # ISPC 教程
│   ├── deployment/            # 部署教程
│   └── scenarios/             # 场景实践
│
├── how-to/                    # 🔧 操作指南
│   ├── operate/               # 运维操作
│   ├── deploy/                # 部署操作
│   ├── configure/             # 配置指南
│   ├── integrate/             # 集成指南
│   ├── secure/                # 安全操作
│   └── troubleshoot/          # 故障排查
│
├── reference/                 # 📋 参考文档
│   ├── api/                   # API 参考
│   ├── cli/                   # CLI 参考
│   ├── config/                # 配置参考
│   ├── schema/                # 数据格式
│   ├── error-codes.md         # 错误码参考
│   └── ports.md               # 端口规范
│
├── contributing/              # 🤝 贡献指南
│   ├── development-setup.md   # 开发环境搭建
│   ├── code-style.md          # 代码规范
│   ├── docs-style.md          # 文档规范
│   └── design-docs.md         # 设计文档说明
│
└── support/                   # 📞 支持
    ├── compatibility.md       # 兼容性策略
    ├── support-policy.md      # 支持策略
    └── releases.md            # 版本发布
```

---

## 🎯 按任务查找

### 我想了解项目

- [WES 是什么](./concepts/what-is-wes.md) - 产品总览：定位、价值、特性
- [架构总览](./concepts/architecture-overview.md) - 系统架构鸟瞰
- [术语表](./concepts/glossary.md) - 术语定义

### 我想开始开发

- [安装指南](./getting-started/installation.md) - 环境准备
- [快速开始](./getting-started/quickstart-local.md) - 5 分钟上手
- [API 参考](./reference/api/) - 接口文档

### 我想学习合约开发

- [合约入门](./tutorials/contracts/) - 合约开发教程
- [ISPC 教程](./tutorials/ispc/) - ISPC 端到端教程

### 我想部署运维

- [部署指南](./how-to/deploy/) - 部署操作指南
- [配置指南](./how-to/configure/) - 配置说明
- [故障排查](./how-to/troubleshoot/) - 问题排查

### 我想贡献代码

- [开发环境搭建](./contributing/development-setup.md) - 环境准备
- [代码规范](./contributing/code-style.md) - 编码标准
- [设计文档说明](./contributing/design-docs.md) - 如何阅读 `_dev/` 中的设计文档

---

## ❓ 常见问题

### Q：文档是最新的吗？

A：文档会持续更新。建议查看文档头部的更新日期，或提交 Issue 询问。

### Q：我找不到我要的信息怎么办？

A：
1. 使用浏览器搜索功能（Ctrl+F / Cmd+F）
2. 查看 [常见问题](./getting-started/faq.md)
3. 提交 Issue 告诉我们缺了什么

### Q：我想贡献代码怎么办？

A：欢迎提交 Pull Request！请查看 [贡献指南](./contributing/development-setup.md)。

---

## 🔗 相关资源

- **内部设计文档**：[`_dev/`](../../_dev/) - 核心研发、架构师的内部知识库
- **问题反馈**：GitHub Issues
- **社区讨论**：GitHub Discussions

---

**WES：让生产关系真正承载生产力。** 🚀

