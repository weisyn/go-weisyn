# 设计文档说明

---

## 概述

本文档说明如何阅读和理解 WES 项目的内部设计文档（`_dev/` 目录）。

---

## 设计文档定位

### 两层文档体系

WES 项目采用两层文档体系：

| 层级 | 目录 | 受众 | 内容 |
|------|------|------|------|
| Layer 1 | `docs/` | 外部用户 | 用户友好的使用文档 |
| Layer 2 | `_dev/` | 核心开发者 | 内部设计和规范文档 |

### `_dev/` 是 Source of Truth

`_dev/` 目录是 WES 的技术真相来源（Source of Truth）：
- 所有协议定义都在这里
- 所有架构设计都在这里
- 所有概念都应能追溯到这里

---

## `_dev/` 目录结构

```
_dev/
├── 00-overview-总览/
├── 01-协议规范-specs/           # 协议定义、形式化规范
├── 02-架构设计-architecture/    # 系统架构、模块设计
├── 03-实现蓝图-implementation/  # 实现指南、代码映射
├── 04-工程标准-standards/       # 编码规范、接口标准
├── 05-开发流程-development/     # Git 流程、提交规范
├── 06-开发运维指南-guides/      # 操作手册、集成指南
├── 07-测试方案-testing/         # 测试策略、测试矩阵
├── 08-发布流程-publishing/      # 发布流程、版本管理
├── 09-工具与脚本-tools/         # 工具说明
├── 10-架构决策-decisions/       # ADR 记录
├── 11-历史与里程碑-history/     # 历史复盘
├── 12-文档模板-templates/       # 文档模板
├── 13-产品与市场-product-and-market/
└── 14-实施任务-implementation-tasks/
```

---

## 阅读路径

### 初学者

1. `_dev/00-overview-总览/` - 了解项目全貌
2. `_dev/02-架构设计-architecture/00-系统视图-system-views/` - 理解系统架构
3. `_dev/01-协议规范-specs/` - 深入协议细节

### 开发者

1. 先看协议规范（`01-协议规范-specs/`）了解"是什么"
2. 再看架构设计（`02-架构设计-architecture/`）了解"怎么设计"
3. 最后看实现蓝图（`03-实现蓝图-implementation/`）了解"怎么实现"

### 架构师

1. `02-架构设计-architecture/00-系统视图-system-views/` - 系统视图
2. `10-架构决策-decisions/` - ADR 记录
3. `01-协议规范-specs/` - 协议细节

---

## 关键目录说明

### 协议规范 (`01-协议规范-specs/`)

定义 WES 的技术标准和协议规范：

| 子目录 | 内容 |
|--------|------|
| 01-状态与资源模型协议 | EUTXO、URES 规范 |
| 02-交易协议 | 交易模型、验证规则 |
| 03-区块与链协议 | 区块、链管理规范 |
| 04-共识协议 | PoW+XOR 共识规范 |
| 05-网络协议 | P2P 网络规范 |
| 06-可执行资源执行协议 | ISPC、WASM、ONNX 规范 |
| 07-隐私与证明协议 | ZK 证明规范 |
| 08-治理与合规协议 | 治理、合规规范 |

### 架构设计 (`02-架构设计-architecture/`)

定义 WES 的系统架构：

| 子目录 | 内容 |
|--------|------|
| 00-系统视图 | 全局架构视图 |
| 01-分层与模块架构 | 分层设计 |
| 02-状态与资源架构 | EUTXO、URES 架构 |
| 03-交易架构 | 交易处理架构 |
| ... | ... |

### 实现蓝图 (`03-实现蓝图-implementation/`)

规范与代码的映射：

- 实现状态跟踪
- 代码目录映射
- 技术债务记录

### 架构决策 (`10-架构决策-decisions/`)

ADR（Architecture Decision Records）记录：

- 每个重大设计决策都有记录
- 包含背景、决策、后果

---

## 文档间的追溯关系

```
docs/zh/concepts/ispc.md
    ↓ 来源
_dev/01-协议规范-specs/06-可执行资源执行协议-executable-resource-execution/
    ↓ 架构
_dev/02-架构设计-architecture/06-执行与计算架构-execution-and-compute/
    ↓ 实现
_dev/03-实现蓝图-implementation/05-执行与计算实现-execution-and-compute/
    ↓ 代码
internal/core/ispc/
```

---

## 如何使用 `_dev/`

### 查找协议定义

```bash
# 查看 EUTXO 协议
cat _dev/01-协议规范-specs/01-状态与资源模型协议-state-and-resource/*.md
```

### 查找架构设计

```bash
# 查看系统架构
cat _dev/02-架构设计-architecture/00-系统视图-system-views/*.md
```

### 查找实现指南

```bash
# 查看交易实现
cat _dev/03-实现蓝图-implementation/02-交易实现-transaction/*.md
```

---

## 贡献设计文档

### 新增协议

1. 在 `01-协议规范-specs/` 创建规范文档
2. 在 `02-架构设计-architecture/` 创建架构文档
3. 在 `03-实现蓝图-implementation/` 创建实现指南

### 架构决策

1. 使用 ADR 模板
2. 放在 `10-架构决策-decisions/`
3. 编号递增

---

## 相关文档

- [开发环境搭建](./development-setup.md) - 环境配置
- [代码规范](./code-style.md) - 代码标准
- [文档规范](./docs-style.md) - 文档编写标准

