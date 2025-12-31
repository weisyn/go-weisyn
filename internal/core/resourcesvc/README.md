## 📦 resourcesvc 模块（internal/core/resourcesvc）

---

### 📍 模块定位

`internal/core/resourcesvc` 是 **资源视图服务（ResourceViewService）** 的实现模块，负责基于 EUTXO 与 URES 两个视角，组合出对外暴露的统一资源视图。

- **公共接口层**：`pkg/interfaces/resourcesvc`  
  - 定义 `Service` 接口与 `ResourceView` / `ResourceHistory` 等 DTO 类型。  
  - 被 API 层、SDK、集成测试等上层代码直接依赖。
- **实现层（本目录）**：  
  - `Service` 结构体实现 `resourcesvc.Service` 接口。  
  - 通过 Fx 模块 `resourcesvc.Module()` 装配依赖并导出为接口。

---

### 🏗️ 目录结构

```text
internal/core/resourcesvc/
├── service.go          # 包声明与模块级注释（实现模块）
├── service_impl.go     # Service 实现：组合 EUTXO / URES / Persistence 查询
├── types.go            # 类型别名，指向 pkg/interfaces/resourcesvc 中的 DTO
└── module.go           # Fx 模块：依赖注入与 Service 导出
```

---

### 🔌 依赖关系

`Service` 实现通过以下接口获取底层数据：

- `eutxo.ResourceUTXOQuery`（命名依赖 `resource_utxo_query`）  
  - 提供基于 UTXO 的资源实例视角。
- `persistence.ResourceQuery`（命名依赖 `resource_query`）  
  - 提供基于 URES 索引的资源元数据视角。
- `persistence.UTXOQuery` / `TxQuery` / `BlockQuery`  
  - 用于补全锁定条件、交易元数据、区块时间戳等信息。
- `storage.BadgerStore`  
  - 用于构建历史查询服务（`history.Service`），补全资源历史。
- `log.Logger`  
  - 记录查询与回退路径相关日志。

**Fx 装配（简化示意）：**

```go
// module.go
type ModuleInput struct {
    fx.In
    Logger            log.Logger
    ResourceUTXOQuery eutxo.ResourceUTXOQuery  `name:"resource_utxo_query"`
    ResourceQuery     persistence.ResourceQuery `name:"resource_query"`
    UTXOQuery         persistence.UTXOQuery    `name:"utxo_query"`
    TxQuery           persistence.TxQuery      `name:"tx_query"`
    BlockQuery        persistence.BlockQuery   `name:"block_query"`
    BadgerStore       storage.BadgerStore
}

type ModuleOutput struct {
    fx.Out
    ResourceViewService resourcesvciface.Service
}

func ProvideServices(input ModuleInput) (ModuleOutput, error) {
    svc, err := NewService(
        input.ResourceUTXOQuery,
        input.ResourceQuery,
        input.UTXOQuery,
        input.TxQuery,
        input.BlockQuery,
        input.BadgerStore,
        input.Logger,
    )
    if err != nil {
        return ModuleOutput{}, err
    }
    return ModuleOutput{ResourceViewService: svc}, nil
}
```

---

### 🔄 与公共接口的关系

- **公共接口**：`pkg/interfaces/resourcesvc.Service`
  - 上层只依赖该接口与 DTO。
- **实现类型**：`internal/core/resourcesvc.Service`
  - 实现公共接口，隐藏具体查询与回退逻辑。
- **类型别名**：`internal/core/resourcesvc/types.go`
  - 通过 `type ResourceView = resourcesvciface.ResourceView` 等别名，避免内部重复定义 DTO，同时保持现有实现文件的可读性。

---

### 📚 相关文档

- `pkg/interfaces/resourcesvc/service.go`：公共接口与 DTO 定义。  
- `docs/components/resourcesvc.md`：资源视图服务的能力视图与使用示例。  
- `_dev/01-协议规范-specs/09-协议版本与能力协商-meta/IDENTIFIER_IMPLEMENTATION_PLAN.md`：与资源标识协议（ResourceCodeId / ResourceInstanceId）相关的设计与演进计划。


