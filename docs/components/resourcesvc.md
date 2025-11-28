# ResourceViewService 组件能力视图

---

## 🎯 组件定位

ResourceViewService 组件是 WES 系统的资源视图服务核心，提供统一的资源视图查询服务，整合 EUTXO 和 URES 两个视角。

**在三层模型中的位置**：基础设施层（Infrastructure Layer）

> **战略背景**：ResourceViewService 组件依赖 ResourceUTXOQuery（EUTXO 视角）和 ResourceQuery（URES 视角），提供统一的 ResourceView，简化资源查询的复杂度。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 统一资源视图

**能力描述**：
- 整合 EUTXO 和 URES 两个视角的资源信息
- 提供统一的 ResourceView 接口
- 支持资源列表、单个资源、资源历史的查询
- 简化资源查询的复杂度

**使用约束**：
- 查询是只读操作，不修改状态
- 查询结果反映当前状态
- 支持分页和过滤

**典型使用场景**：
- 资源查询：查询资源列表和详细信息
- 资源历史：查询资源的变更历史

### 2. 多视角资源整合

**能力描述**：
- **EUTXO视角**：资源作为UTXO的状态信息
- **URES视角**：资源作为内容寻址存储的文件信息
- **统一视图**：整合两个视角，提供完整的资源信息

**使用约束**：
- 依赖 EUTXO 和 URES 的查询能力
- 视图整合是只读操作

---

## 🔧 接口能力

### ResourceViewService（资源视图服务）

**能力**：
- `ListResources(ctx, filter, page)` - 列出资源列表
- `GetResource(ctx, contentHash)` - 获取单个资源
- `GetResourceHistory(ctx, contentHash, page)` - 获取资源历史

**约束**：
- 查询是只读操作
- 查询结果反映当前状态
- 支持分页和过滤

---

## ⚙️ 配置说明

### 资源视图服务配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_list_results` | int | 100 | 最大列表结果数 |
| `enable_history_cache` | bool | true | 启用历史缓存 |
| `history_cache_ttl` | duration | 5m | 历史缓存TTL |

---

## 📋 使用约束

### 资源视图约束

1. **查询约束**：
   - 查询是只读操作，不修改状态
   - 查询结果反映当前状态
   - 支持分页和过滤

2. **依赖约束**：
   - 依赖 EUTXO 和 URES 的查询能力
   - 视图整合需要两个视角的数据

---

## 🎯 典型使用场景

### 场景 1：列出资源列表

```go
resourceViewService := resourcesvc.NewResourceViewService()
filter := &resourcesvc.ResourceViewFilter{
    Type: "contract",
}
page := &resourcesvc.PageRequest{
    Page: 1,
    PageSize: 20,
}
resources, pageResp, err := resourceViewService.ListResources(ctx, filter, page)
if err != nil {
    return err
}
// 获取资源列表
```

### 场景 2：获取单个资源

```go
resourceViewService := resourcesvc.NewResourceViewService()
resource, err := resourceViewService.GetResource(ctx, contentHash)
if err != nil {
    return err
}
// 获取资源详细信息
```

### 场景 3：获取资源历史

```go
resourceViewService := resourcesvc.NewResourceViewService()
history, err := resourceViewService.GetResourceHistory(ctx, contentHash, page)
if err != nil {
    return err
}
// 获取资源变更历史
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [EUTXO 能力视图](./eutxo.md) - 了解账本能力
- [URES 能力视图](./ures.md) - 了解资源管理能力


