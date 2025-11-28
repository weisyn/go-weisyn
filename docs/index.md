# WES 文档门户

欢迎来到 WES（Weisyn）文档中心！

**WES 定义区块链的可验证计算范式，开启 AI 时代的去中心化智能。**

---

## 🚀 快速开始

### 我是新人，应该从哪里开始？

**3 步快速上手：**

1. **读这个** → [`overview.md`](./overview.md) - 理解 WES 是什么（10 分钟）
   - 一句话定位：第三代区块链，定义可验证计算范式
   - 核心价值：AI Native 能力 + 企业应用支持

2. **读这个** → [`tutorials/quickstart/`](./tutorials/quickstart/) - 5 分钟跑起来
   - 本地单节点部署
   - Docker 快速启动
   - 云环境部署

3. **读这个** → [`reference/api/`](./reference/api/) - 开始集成开发
   - API 接口文档
   - 示例代码
   - 最佳实践

---

## 🎯 按任务查找

### 我想了解项目

**了解 WES 是什么**
- [`overview.md`](./overview.md) - 产品总览：定位、价值、特性
- [`product/positioning.md`](./product/positioning.md) - 市场定位与竞争对比
- [`product/value-proposition.md`](./product/value-proposition.md) - 核心价值与场景

**了解系统架构**
- [`architecture/overview.md`](./architecture/overview.md) - 总体架构鸟瞰
- [`architecture/views.md`](./architecture/views.md) - 多视角架构说明
- [`architecture/glossary.md`](./architecture/glossary.md) - 术语表

### 我想集成开发

**调用 API**
- [`reference/api/`](./reference/api/) - API 接口文档
- [`reference/schema/`](./reference/schema/) - 数据格式规范

**使用命令行**
- [`reference/cli/`](./reference/cli/) - CLI 命令参考

**配置系统**
- [`reference/config/`](./reference/config/) - 配置字段说明

### 我想学习开发

**快速开始**
- [`tutorials/quickstart/`](./tutorials/quickstart/) - 快速开始指南

**合约开发**
- [`tutorials/contracts/beginner.md`](./tutorials/contracts/beginner.md) - 新手入门
- [`tutorials/contracts/patterns.md`](./tutorials/contracts/patterns.md) - 推荐实践
- [`tutorials/contracts/troubleshooting.md`](./tutorials/contracts/troubleshooting.md) - 常见问题

**业务场景**
- [`tutorials/scenarios/ai-inference.md`](./tutorials/scenarios/ai-inference.md) - AI 推理场景
- [`tutorials/scenarios/enterprise-workflow.md`](./tutorials/scenarios/enterprise-workflow.md) - 企业工作流场景

### 我想部署运维

**部署指南**
- [`tutorials/deployment/`](./tutorials/deployment/) - 部署指南（生产/测试/常见拓扑）

**故障排查**
- [`troubleshooting/operations.md`](./troubleshooting/operations.md) - 节点/网络/存储问题
- [`troubleshooting/performance.md`](./troubleshooting/performance.md) - 性能问题
- [`troubleshooting/faq.md`](./troubleshooting/faq.md) - 通用常见问题

### 我想了解标准与承诺

**兼容性与支持**
- [`standards/compatibility.md`](./standards/compatibility.md) - 兼容性策略
- [`standards/support-policy.md`](./standards/support-policy.md) - 版本支持策略
- [`standards/releases.md`](./standards/releases.md) - 版本矩阵

**测试承诺**
- [`standards/testing-principles.md`](./standards/testing-principles.md) - 测试原则

---

## 👥 按角色查找

### 决策者 / 产品经理

**了解项目价值**
- [`overview.md`](./overview.md) - 产品总览
- [`product/positioning.md`](./product/positioning.md) - 市场定位
- [`product/value-proposition.md`](./product/value-proposition.md) - 核心价值
- [`product/faq.md`](./product/faq.md) - 常见问题

### 架构师

**了解系统架构**
- [`overview.md`](./overview.md) - 产品定位
- [`architecture/overview.md`](./architecture/overview.md) - 系统架构
- [`architecture/views.md`](./architecture/views.md) - 多视角说明
- [`components/`](./components/) - 组件能力视图

### 开发者

**快速上手**
- [`tutorials/quickstart/`](./tutorials/quickstart/) - 快速开始
- [`reference/api/`](./reference/api/) - API 文档
- [`reference/cli/`](./reference/cli/) - CLI 文档
- [`tutorials/contracts/`](./tutorials/contracts/) - 合约开发

**深入学习**
- [`components/`](./components/) - 组件能力与约束
- [`tutorials/scenarios/`](./tutorials/scenarios/) - 业务场景案例

### 运维

**部署与运维**
- [`tutorials/deployment/`](./tutorials/deployment/) - 部署指南
- [`reference/config/`](./reference/config/) - 配置说明
- [`troubleshooting/`](./troubleshooting/) - 故障排查

---

## 📚 文档地图

```
docs/
├── README.md              # GitHub 中的简要导航封面
├── index.md               # 文档门户（推荐入口）
├── overview.md            # 产品总览
│
├── product/               # 产品 & 定位层
│   ├── positioning.md     # 市场定位
│   ├── value-proposition.md  # 核心价值
│   └── faq.md            # 常见问题
│
├── architecture/          # 架构鸟瞰
│   ├── overview.md        # 总体架构
│   ├── views.md           # 多视角说明
│   └── glossary.md        # 术语表
│
├── components/            # 组件能力视图
│   ├── chain.md          # 链管理能力
│   ├── block.md          # 区块处理能力
│   ├── tx.md             # 交易能力
│   ├── eutxo.md          # 账本能力
│   ├── ures.md           # 资源服务能力
│   ├── ispc.md           # 可验证计算能力
│   ├── consensus.md      # 共识能力
│   ├── mempool.md        # 内存池能力
│   ├── compliance.md     # 合规能力
│   ├── network.md        # 网络能力
│   └── resourcesvc.md    # 资源视图服务能力
│
├── reference/             # API & CLI & 配置
│   ├── api/              # API 文档
│   ├── cli/              # CLI 文档
│   ├── config/           # 配置说明
│   └── schema/           # 数据格式
│
├── tutorials/            # 教程 & 场景实践
│   ├── quickstart/       # 快速开始
│   ├── deployment/       # 部署指南
│   ├── contracts/        # 合约开发
│   └── scenarios/        # 业务场景
│
├── standards/            # 公开标准 & 承诺
│   ├── compatibility.md  # 兼容性策略
│   ├── support-policy.md # 支持策略
│   ├── testing-principles.md  # 测试原则
│   └── releases.md       # 版本矩阵
│
└── troubleshooting/       # 故障排查
    ├── operations.md     # 运维问题
    ├── performance.md    # 性能问题
    └── faq.md            # 通用 FAQ
```

---

## ❓ 常见问题

### Q：文档是最新的吗？

A：文档会持续更新。建议查看文档头部的更新日期，或提交 Issue 询问。

### Q：我找不到我要的信息怎么办？

A：
1. 使用浏览器搜索功能（Ctrl+F / Cmd+F）
2. 查看 [`troubleshooting/faq.md`](./troubleshooting/faq.md)
3. 提交 Issue 告诉我们缺了什么

### Q：我想贡献代码怎么办？

A：欢迎提交 Pull Request！请查看项目根目录的贡献指南。

---

## 🔗 相关资源

- **源代码**：项目根目录
- **问题反馈**：GitHub Issues
- **社区讨论**：GitHub Discussions

---

**祝您学习愉快！** 🎉

如有任何建议，欢迎反馈。
