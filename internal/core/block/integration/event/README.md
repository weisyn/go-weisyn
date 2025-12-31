# Block 模块事件集成

---

## 📍 **模块定位**

Block 模块的事件集成层，负责：
- ✅ 发布区块处理完成事件（`EventTypeBlockProcessed`）
- ✅ 发布分叉检测事件（`EventTypeForkDetected`，如需要）

**解决什么问题**：
- 事件发布：通知其他模块区块处理状态
- 解耦模块：通过事件实现模块间松耦合通信

---

## 📤 **出站事件（Block 模块发布）**

### 1. EventTypeBlockProcessed - 区块处理完成事件

**触发时机**：`BlockProcessor.ProcessBlock()` 成功完成后

**事件数据**：
```go
type BlockProcessedEventData struct {
    Block *core.Block  // 已处理的区块
}
```

**订阅者**：
- Chain 模块：自动更新链尖状态
- 其他模块：根据需要订阅

**发布位置**：`processor/service.go` 中的 `publishBlockProcessedEvent()`

---

## 📥 **入站事件（Block 模块订阅）**

Block 模块目前**不订阅**任何事件，仅发布事件。

如果未来需要订阅事件，可以在此目录添加 `subscribe_handlers.go`。

---

## 🔗 **事件常量**

事件类型定义在 `pkg/constants/events/`:
- `EventTypeBlockProcessed = "block.processed"`
- `EventTypeForkDetected = "fork.detected"`

---

## 📚 **参考文档**

- [Chain 模块事件集成](../../../chain/integration/event/README.md) - 订阅示例
- [事件总线接口](../../../../../pkg/interfaces/infrastructure/event/README.md)

---

**状态**：✅ 已完成（仅发布事件）

**维护者**：WES Block 开发组

