# Core Concepts

This directory contains WES core concept documentation to help you deeply understand WES's technical architecture and design philosophy.

## 📚 Document List

### Overview

| Document | Description |
|----------|-------------|
| [What is WES](./what-is-wes.md) | WES positioning, value, and core innovations |
| [Architecture Overview](./architecture-overview.md) | System architecture overview |

### Four Core Innovations

| Document | Description | Corresponding Code Module |
|----------|-------------|--------------------------|
| [ISPC Intrinsic Self-Proving Computing](./ispc.md) | Execution as proof, Host ABI, ZK proof | `internal/core/ispc` |
| [EUTXO Extended Model](./eutxo.md) | Asset/Resource/State three-layer output | `internal/core/eutxo` |
| [URES Unified Resource Management](./ures.md) | Content addressing, executable resources | `internal/core/ures` |
| [PoW+XOR Distance Selection Consensus](./consensus-pow-xor.md) | Proof of Work + XOR distance selection | `internal/core/consensus` |

### Core Chain

| Document | Description | Corresponding Code Module |
|----------|-------------|--------------------------|
| [Transaction Model](./transaction.md) | Transaction construction, validation, lifecycle | `internal/core/tx` |
| [Block Model](./block.md) | Block construction, Merkle, difficulty | `internal/core/block` |
| [Chain Model](./chain.md) | Fork/Reorg/synchronization/height management | `internal/core/chain` |

### System Support

| Document | Description | Corresponding Code Module |
|----------|-------------|--------------------------|
| [Network and Topology](./network-and-topology.md) | P2P network, discovery, routing | `internal/core/network` |
| [Data Persistence](./data-persistence.md) | Storage, indexing, snapshots | `internal/core/persistence` |
| [Privacy and Proof](./privacy-and-proof.md) | ZK proof system | `internal/core/ispc/zkproof` |
| [Governance and Compliance](./governance-and-compliance.md) | Chain-level governance, compliance policies | `internal/core/compliance` |

### Reference

| Document | Description |
|----------|-------------|
| [Glossary](./glossary.md) | Core term definitions |

## 🎯 Recommended Reading Order

### Beginner Path

1. **[What is WES](./what-is-wes.md)** - Understand WES positioning and value
2. **[Architecture Overview](./architecture-overview.md)** - Understand overall system architecture
3. **[ISPC Intrinsic Self-Proving Computing](./ispc.md)** - Understand core computing paradigm

### Developer Path

1. **[Transaction Model](./transaction.md)** - Understand transaction structure
2. **[EUTXO Extended Model](./eutxo.md)** - Understand state model
3. **[URES Unified Resource Management](./ures.md)** - Understand resource management

### Architect Path

1. **[Architecture Overview](./architecture-overview.md)** - System architecture view
2. **Four Core Innovations** - Understand technical innovation points
3. **Core Chain** - Understand system operation

## 🔗 Related Resources

- **Internal Design Documents**: [`_dev/01-协议规范-specs/`](../../../_dev/01-协议规范-specs/) - Detailed protocol specification definitions
- **Internal Architecture Documents**: [`_dev/02-架构设计-architecture/`](../../../_dev/02-架构设计-architecture/) - Detailed architecture design descriptions

