# 内部接口层 (internal/core/mempool/interfaces)

【模块定位】
　　本目录定义 mempool 组件的内部接口，作为公共接口和具体实现之间的桥梁，遵循代码组织规范的三层架构。

【设计原则】
- **强制继承**：所有内部接口必须嵌入对应的公共接口
- **节制扩展**：只在确实需要时才添加内部专用方法
- **实现约束**：所有实现必须实现内部接口，不直接实现公共接口

【核心职责】
1. **接口桥接**：作为公共接口到实现的桥梁
2. **内部扩展**：支持组件内部模块间协作（如需要）
3. **架构约束**：强制实现层通过内部接口实现，不直接实现公共接口

## 目录结构

```
interfaces/
├── txpool.go          # InternalTxPool 接口定义
├── candidatepool.go    # InternalCandidatePool 接口定义
└── README.md          # 本文档
```

## 接口定义

### InternalTxPool

交易池内部接口，继承 `mempoolIfaces.TxPool`。

**当前状态**：
- ✅ 嵌入公共接口 `mempoolIfaces.TxPool`
- ✅ 无额外内部方法（纯继承）

**未来扩展**：
- 如需要内部协作方法，可在此添加（例如：`SetEventSink`）

### InternalCandidatePool

候选区块池内部接口，继承 `mempoolIfaces.CandidatePool`。

**当前状态**：
- ✅ 嵌入公共接口 `mempoolIfaces.CandidatePool`
- ✅ 无额外内部方法（纯继承）

**未来扩展**：
- 如需要内部协作方法，可在此添加（例如：`SetEventSink`）

## 架构关系

```
pkg/interfaces/mempool (公共接口)
    ↓ 继承
internal/core/mempool/interfaces (内部接口) ← 本目录
    ↓ 实现
internal/core/mempool/txpool (具体实现)
internal/core/mempool/candidatepool (具体实现)
```

## 符合代码组织规范

- ✅ 内部接口嵌入公共接口（强制继承）
- ✅ 实现层实现内部接口而非公共接口
- ✅ 通过 module.go 绑定到公共接口
- ✅ 编译期检查确保实现关系正确

