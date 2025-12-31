# Design Document Guide

---

## Overview

This document explains how to read and understand WES project's internal design documents (`_dev/` directory).

---

## Design Document Positioning

### Two-Layer Documentation System

WES project adopts a two-layer documentation system:

| Layer | Directory | Audience | Content |
|-------|-----------|----------|---------|
| Layer 1 | `docs/` | External users | User-friendly usage documentation |
| Layer 2 | `_dev/` | Core developers | Internal design and specification documents |

### `_dev/` is Source of Truth

The `_dev/` directory is WES's technical source of truth:
- All protocol definitions are here
- All architecture designs are here
- All concepts should be traceable here

---

## `_dev/` Directory Structure

```
_dev/
├── 00-overview-总览/
├── 01-协议规范-specs/           # Protocol definitions, formal specifications
├── 02-架构设计-architecture/    # System architecture, module design
├── 03-实现蓝图-implementation/  # Implementation guides, code mapping
├── 04-工程标准-standards/       # Coding standards, interface standards
├── 05-开发流程-development/     # Git workflow, commit standards
├── 06-开发运维指南-guides/      # Operation manuals, integration guides
├── 07-测试方案-testing/         # Testing strategies, test matrices
├── 08-发布流程-publishing/      # Release process, version management
├── 09-工具与脚本-tools/         # Tool descriptions
├── 10-架构决策-decisions/       # ADR records
├── 11-历史与里程碑-history/     # Historical reviews
├── 12-文档模板-templates/       # Document templates
├── 13-产品与市场-product-and-market/
└── 14-实施任务-implementation-tasks/
```

---

## Reading Paths

### Beginners

1. `_dev/00-overview-总览/` - Understand project overview
2. `_dev/02-架构设计-architecture/00-系统视图-system-views/` - Understand system architecture
3. `_dev/01-协议规范-specs/` - Deep dive into protocol details

### Developers

1. First read protocol specifications (`01-协议规范-specs/`) to understand "what"
2. Then read architecture design (`02-架构设计-architecture/`) to understand "how to design"
3. Finally read implementation blueprint (`03-实现蓝图-implementation/`) to understand "how to implement"

### Architects

1. `02-架构设计-architecture/00-系统视图-system-views/` - System views
2. `10-架构决策-decisions/` - ADR records
3. `01-协议规范-specs/` - Protocol details

---

## Key Directory Descriptions

### Protocol Specifications (`01-协议规范-specs/`)

Defines WES technical standards and protocol specifications:

| Subdirectory | Content |
|--------------|---------|
| 01-状态与资源模型协议 | EUTXO, URES specifications |
| 02-交易协议 | Transaction model, validation rules |
| 03-区块与链协议 | Block, chain management specifications |
| 04-共识协议 | PoW+XOR consensus specifications |
| 05-网络协议 | P2P network specifications |
| 06-可执行资源执行协议 | ISPC, WASM, ONNX specifications |
| 07-隐私与证明协议 | ZK proof specifications |
| 08-治理与合规协议 | Governance, compliance specifications |

### Architecture Design (`02-架构设计-architecture/`)

Defines WES system architecture:

| Subdirectory | Content |
|--------------|---------|
| 00-系统视图 | Global architecture views |
| 01-分层与模块架构 | Layered design |
| 02-状态与资源架构 | EUTXO, URES architecture |
| 03-交易架构 | Transaction processing architecture |
| ... | ... |

### Implementation Blueprint (`03-实现蓝图-implementation/`)

Mapping between specifications and code:

- Implementation status tracking
- Code directory mapping
- Technical debt records

### Architecture Decisions (`10-架构决策-decisions/`)

ADR (Architecture Decision Records):

- Each major design decision is recorded
- Includes background, decision, consequences

---

## Traceability Between Documents

```
docs/en/concepts/ispc.md
    ↓ source
_dev/01-协议规范-specs/06-可执行资源执行协议-executable-resource-execution/
    ↓ architecture
_dev/02-架构设计-architecture/06-执行与计算架构-execution-and-compute/
    ↓ implementation
_dev/03-实现蓝图-implementation/05-执行与计算实现-execution-and-compute/
    ↓ code
internal/core/ispc/
```

---

## How to Use `_dev/`

### Find Protocol Definitions

```bash
# View EUTXO protocol
cat _dev/01-协议规范-specs/01-状态与资源模型协议-state-and-resource/*.md
```

### Find Architecture Design

```bash
# View system architecture
cat _dev/02-架构设计-architecture/00-系统视图-system-views/*.md
```

### Find Implementation Guides

```bash
# View transaction implementation
cat _dev/03-实现蓝图-implementation/02-交易实现-transaction/*.md
```

---

## Contributing Design Documents

### Adding New Protocols

1. Create specification document in `01-协议规范-specs/`
2. Create architecture document in `02-架构设计-architecture/`
3. Create implementation guide in `03-实现蓝图-implementation/`

### Architecture Decisions

1. Use ADR template
2. Place in `10-架构决策-decisions/`
3. Increment numbering

---

## Related Documentation

- [Development Environment Setup](./development-setup.md) - Environment configuration
- [Code Standards](./code-style.md) - Code standards
- [Documentation Standards](./docs-style.md) - Documentation writing standards

