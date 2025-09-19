# 高度门闸管理器（Height Gate Manager）

## 🎯 **模块定位（基于统一Aggregator架构）**

高度门闸是**专注挖矿算法**的矿工模块中的极简高度跟踪组件。在WES统一Aggregator架构下，矿工专注于"纯粹的挖矿算法"，高度门闸作为基础设施，提供**防重复挖矿的原子高度跟踪**能力。

## 📋 **设计约束（严格遵循）**

### **架构约束**
- **职责单一**: 仅提供高度读写，不涉及网络/缓存/事件等复杂功能
- **极简设计**: 遵循项目"避免过度设计"原则[[memory:8349168]]
- **内部交互**: 通过interfaces.HeightGateManager与其他子组件交互
- **配置化**: 分叉深度等参数从配置读取，禁止硬编码[[memory:8405374]]

### **核心职责**
1. **原子高度跟踪**: 记录矿工最后处理的区块高度
2. **重复挖矿防护**: 防止在同一高度重复挖矿计算  
3. **分叉友好回退**: 支持合理深度的高度回退（配置化分叉深度）
4. **纳秒级查询**: 高性能原子读取操作

## 📁 **文件组织架构**

```
height_gate/
├── README.md              # 设计文档：架构约束与职责边界
├── IMPLEMENTATION_PLAN.md # 实施计划：具体任务与验收标准
├── manager.go             # 薄实现：依赖注入+方法委托
├── get_height.go          # 纳秒级原子查询实现
└── update_height.go       # 分叉友好的原子更新实现
```

**极简设计原则**：移除了所有过度设计的组件
- ❌ 缓存管理：`atomic.Uint64`已足够高效
- ❌ 持久化存储：上层负责状态恢复
- ❌ 事件发布：违反集成归口约束[[memory:7733090]]  
- ❌ 统计监控：违反极简原则[[memory:8349168]]

## 🏗️ **极简门闸架构**

### **核心流程（基于权威接口）**

```
挖矿流程中的门闸作用：

1. ExecuteMiningRound 开始 → GetLastProcessedHeight() 检查
2. 挖矿完成/区块确认 → UpdateLastProcessedHeight(height) 更新  
3. 分叉场景 → UpdateLastProcessedHeight(forkPoint) 回退
```

**架构特点**：
- ✅ **原子操作**: `atomic.Uint64` 保证线程安全
- ✅ **零依赖**: 不依赖缓存/存储/事件总线等外部组件
- ✅ **纳秒响应**: 无锁读取，极致性能
- ✅ **分叉友好**: 支持配置化深度的高度回退

## 🔧 **权威接口定义**

基于 `internal/core/consensus/interfaces/miner.go` 的权威定义：

```go
type HeightGateManager interface {
    // 获取最后处理高度 - 纳秒级原子查询
    GetLastProcessedHeight() uint64
    
    // 更新最后处理高度 - 原子更新，支持分叉回退
    UpdateLastProcessedHeight(height uint64)
}
```

**设计说明**：
- ✅ 仅2个方法，极简设计
- ✅ 无error返回，原子操作保证可靠性
- ❌ 禁止扩展方法：IsHeightProcessed/GetHeightStatus/SyncWithBlockchain等

## 🔗 **集成模式**

### **事件驱动的高度更新**
高度门闸不主动发布事件，而是被动接受上层调用：

```
miner.Manager 事件回调模式：
├── handleBlockProcessed   → heightGate.UpdateLastProcessedHeight(height)
├── handleBlockFinalized   → heightGate.UpdateLastProcessedHeight(height)  
└── handleSyncCompleted    → heightGate.UpdateLastProcessedHeight(height)
```

**集成约束**：
- ✅ 门闸仅提供读写方法，不参与事件发布
- ✅ 上层Manager负责在适当时机调用门闸更新
- ❌ 门闸内禁止依赖EventBus或发布任何事件

## ⚡ **性能要求**

### **原子操作保证**
- **查询性能**: GetLastProcessedHeight() < 100ns（无锁原子读取）
- **更新性能**: UpdateLastProcessedHeight() < 1μs（包含分叉校验）
- **并发支持**: 支持10,000+并发读取，单一写入者
- **内存占用**: < 1KB（仅包含一个atomic.Uint64和配置字段）

### **分叉处理能力**
- **配置化深度**: MaxForkDepth通过配置注入，支持运行时调整
- **回退支持**: 允许高度回退，但限制在合理分叉深度内
- **边界校验**: 超出分叉深度的回退请求将被记录但不阻塞操作

极简、高效、可靠！
