# WES系统常量归口管理

## 📋 **模块定位**

　　本模块是WES系统常量的统一归口管理中心，解决跨组件通信和协议复用问题。通过全局化的事件类型和网络协议定义，确保系统各组件间的通信标准化和一致性。

## 🎯 **核心问题解决**

### **事件系统的跨组件特性**
```text
❌ 问题：事件系统与网络系统的本质区别
┌─────────────┬───────────────┬─────────────────┐
│   特性      │   网络系统    │   事件系统      │
├─────────────┼───────────────┼─────────────────┤
│ 通信模式    │ 点对点       │ 跨组件全局      │
│ 订阅关系    │ 一对一       │ 一对多         │
│ 定义范围    │ 组件内部     │ 系统级统一      │
│ 复用需求    │ 较少         │ 频繁           │
└─────────────┴───────────────┴─────────────────┘

✅ 解决：分层次常量管理架构
```

### **现有架构问题**
1. **重复定义**：
   - `consensus`: `"consensus.system.reorg_handled"`  
   - `blockchain`: `"blockchain.chain.reorganized"`
   - **同一事件，不同名称！**

2. **跨组件订阅困难**：
   ```go
   // blockchain想订阅consensus事件，但无法访问定义
   subscriber.SubscribeConsensusEvents(handler) // ❌ 无法知道事件类型
   ```

3. **命名冲突风险**：各组件独立定义可能产生冲突

## 🏗️ **新架构设计**

### **分层次常量管理**

```text
📁 pkg/constants/
├── 📡 events/
│   ├── system_events.go           # 🌐 跨组件全局事件
│   └── README.md                  # 事件管理说明
├── 🔌 protocols/ 
│   ├── system_protocols.go        # 🌐 跨组件全局协议
│   └── README.md                  # 协议管理说明
└── 📖 README.md                   # 本文档

📁 internal/core/*/integration/
├── 📡 event/
│   └── events.go                  # ✅ 组件特定事件（业务专用）
└── 🔌 network/
    └── protocols.go               # ✅ 组件特定协议（业务专用）
```

### **事件分类策略**

**🌐 全局事件** (`pkg/constants/events/system_events.go`)：
```go
// 跨组件共享的标准事件
EventTypeChainReorganized     = "blockchain.chain.reorganized"     // blockchain发布，consensus订阅
EventTypeForkDetected         = "blockchain.fork.detected"         // blockchain发布，consensus订阅  
EventTypeNetworkQualityChanged = "network.quality.changed"         // network发布，consensus订阅
EventTypeSystemStopping       = "system.lifecycle.stopping"       // 系统发布，所有组件订阅
```

**🔧 组件特定事件** (保留在各组件内)：
```go  
// consensus组件内部业务事件
EventTypeMinerStateChanged           = "consensus.miner.state_changed"
EventTypeAggregatorDecisionMade      = "consensus.aggregator.decision_made"  
EventTypeAggregatorCollectionOpened  = "consensus.aggregator.collection_opened"
```

### **协议分类策略**

**🌐 全局协议** (`pkg/constants/protocols/system_protocols.go`)：
```go
// 跨组件复用的基础协议
ProtocolHeartbeat        = "/weisyn/node/heartbeat/v1.0.0"        // 所有组件都需要
ProtocolBlockSync        = "/weisyn/blockchain/block_sync/v1.0.0" // blockchain+consensus共用
ProtocolNodeInfo         = "/weisyn/node/info/v1.0.0"            // 节点信息交换
```

**🔧 组件特定协议** (保留在各组件内)：
```go
// consensus组件专用协议  
ProtocolBlockSubmission    = "/weisyn/consensus/block_submission/1.0.0"
ProtocolConsensusHeartbeat = "/weisyn/consensus/heartbeat/1.0.0"
```

## 💡 **使用方式**

### **跨组件事件通信**
```go
// blockchain组件发布链重组事件
import "github.com/weisyn/v1/pkg/constants/events"
import "github.com/weisyn/v1/pkg/types" 

// 发布标准全局事件
eventData := &types.ChainReorganizedEventData{
    OldHeight: 100,
    NewHeight: 105,
    // ...
}
eventBus.Publish(events.EventTypeChainReorganized, eventData)
```

```go
// consensus组件订阅链重组事件
import "github.com/weisyn/v1/pkg/constants/events"

// 订阅标准全局事件  
eventBus.Subscribe(events.EventTypeChainReorganized, func(eventData *types.ChainReorganizedEventData) error {
    // 处理链重组，调整聚合器状态
    return handleChainReorganization(eventData) 
})
```

### **跨组件协议复用**
```go
// 多个组件都可以使用心跳协议
import "github.com/weisyn/v1/pkg/constants/protocols"

// blockchain组件使用
network.RegisterStreamHandler(protocols.ProtocolHeartbeat, blockchainHeartbeatHandler)

// consensus组件也可以使用同一协议
network.RegisterStreamHandler(protocols.ProtocolHeartbeat, consensusHeartbeatHandler)
```

## 🔄 **迁移策略**

### **渐进式迁移**
1. **第一阶段**：创建全局常量定义，现有组件保持不变
2. **第二阶段**：逐步迁移跨组件使用的事件和协议
3. **第三阶段**：清理重复定义，统一命名规范

### **兼容性保证**  
```go
// 在组件特定文件中保持向后兼容
const (
    // 保留旧定义，标记为已废弃
    // Deprecated: 使用 events.EventTypeChainReorganized 替代
    EventTypeBlockchainReorganized = events.EventTypeChainReorganized
    
    // 新增标准引用
    EventTypeChainReorg = events.EventTypeChainReorganized
)
```

## 📈 **架构优势**

### **解决的核心问题**
1. ✅ **消除重复定义**：一个事件，一个定义
2. ✅ **简化跨组件通信**：标准化的事件类型访问
3. ✅ **避免命名冲突**：全局统一管理
4. ✅ **提供版本控制**：协议兼容性管理
5. ✅ **保持模块独立性**：只有跨组件需求才全局化

### **vs 其他方案比较**

| 方案 | 优点 | 缺点 | 适用性 |
|------|------|------|--------|
| 全部全局化 | 统一管理 | 打破模块边界，增加全局依赖 | ❌ 过度设计 |
| 全部组件化 | 模块独立 | 跨组件通信困难，重复定义 | ❌ 当前问题 |
| **分层次管理** | **精确解决跨组件问题，保持模块独立** | 需要判断哪些全局化 | ✅ **最优方案** |

## 🚀 **实施计划**

1. ✅ **已完成**：创建全局常量定义结构
2. 🔄 **进行中**：分析现有事件和协议，识别跨组件需求  
3. 📋 **待执行**：逐步迁移现有定义，确保兼容性
4. 📋 **待执行**：更新各组件使用全局常量
5. 📋 **待执行**：清理重复定义，完善文档

这个架构解决方案既解决了您提出的跨组件通信问题，又保持了系统的模块化设计原则。
