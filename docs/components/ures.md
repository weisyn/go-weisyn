# URES 组件能力视图

---

## 🎯 组件定位

URES（Universal Resource State）组件是 WES 系统的资源管理核心，负责内容寻址存储，管理 WASM/ONNX/文件等资源。

**在三层模型中的位置**：账本层（Ledger Layer）

> **战略背景**：URES 组件位于核心业务层垂直依赖链的第③层，依赖 ISPC（②），被 EUTXO（④）依赖。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 内容寻址存储

**能力描述**：
- 基于 SHA-256 哈希的内容寻址
- 相同内容产生相同哈希，自动去重
- 环境无关性：相同资源在不同节点产生相同哈希

**使用约束**：
- 资源内容通过哈希寻址
- 资源内容不可修改（修改后哈希改变）
- 资源大小有限制（参见配置说明）

**典型使用场景**：
- 合约部署：存储 WASM 合约文件
- 模型部署：存储 ONNX 模型文件
- 文件存储：存储任意文件资源

---

### 2. 统一资源管理

**能力描述**：
- 统一管理静态资源和可执行资源
- 静态资源：文件/数据/图片等
- 可执行资源：WASM 合约/ONNX 模型等

**使用约束**：
- 资源类型必须明确
- 资源内容必须可验证
- 资源元信息必须完整

**资源类型**：
- **静态资源**：文件、数据、图片等不可执行内容
- **可执行资源**：WASM 合约、ONNX 模型等可执行内容

---

### 3. 资源查询

**能力描述**：
- 支持按内容哈希查询资源
- 支持查询资源元信息
- 支持检查资源文件存在性

**使用约束**：
- 查询是只读操作
- 查询结果反映当前状态
- 支持本地文件检查

**查询能力**：
- **按哈希查询**：查询资源元信息和文件
- **元信息查询**：查询资源类型、大小、时间等
- **文件存在性检查**：检查本地文件是否存在

---

### 4. 资源关联

**能力描述**：
- 关联资源与交易
- 关联资源与区块
- 支持资源部署和确认

**使用约束**：
- 资源必须先存储
- 关联信息不可修改
- 区块确认后更新关联信息

**关联流程**：
- **资源部署**：存储资源，关联交易（blockHash 为 nil）
- **区块确认**：更新关联信息（传入实际的 blockHash）

---

## 🔧 接口能力

### ResourceWriter（资源写入器）

**能力**：
- `StoreResourceFile()` - 存储资源文件
- `LinkResourceToTransaction()` - 关联资源与交易

**约束**：
- 存储操作是原子操作
- 存储失败会回滚
- 去重检查自动执行

### ResourceQuery（资源查询器）

**能力**：
- `GetResourceFromBlockchain()` - 从区块链查询资源
- `GetResourceFile()` - 获取资源文件
- `CheckResourceFileExists()` - 检查资源文件存在性

**约束**：
- 查询是只读操作
- 查询结果反映当前状态
- 文件检查是动态的

---

## ⚙️ 配置说明

### 资源管理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_resource_size` | int | 100MB | 最大资源大小 |
| `enable_deduplication` | bool | true | 启用去重 |
| `storage_path` | string | "./storage" | 存储路径 |

---

## 📋 使用约束

### 资源存储约束

1. **大小约束**：
   - 资源大小不能超过 `max_resource_size`
   - 大资源需要分块存储

2. **格式约束**：
   - 资源内容必须可验证
   - 资源哈希必须正确

3. **去重约束**：
   - 相同内容自动去重
   - 已存在的资源直接返回哈希

### 资源查询约束

1. **存在性约束**：
   - 资源必须存在
   - 资源元信息必须完整

2. **文件约束**：
   - 文件可能不在本地
   - 需要从其他节点同步

### 资源关联约束

1. **关联时机**：
   - 资源部署时立即关联
   - 区块确认后更新关联

2. **关联信息**：
   - 关联信息不可修改
   - 关联信息必须完整

---

## 🎯 典型使用场景

### 场景 1：合约部署

```go
// 存储 WASM 合约文件
writer := ures.NewResourceWriter()
hash, err := writer.StoreResourceFile("contract.wasm")
if err != nil {
    return err
}
// 使用 hash 创建 ResourceOutput
```

### 场景 2：模型部署

```go
// 存储 ONNX 模型文件
writer := ures.NewResourceWriter()
hash, err := writer.StoreResourceFile("model.onnx")
if err != nil {
    return err
}
// 使用 hash 创建 ResourceOutput
```

### 场景 3：资源查询

```go
// 查询资源信息
query := ures.NewResourceQuery()
resource, err := query.GetResourceFromBlockchain(contentHash)
if err != nil {
    return err
}
// 使用 resource 信息
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [EUTXO 能力视图](./eutxo.md) - 了解账本能力
- [ISPC 能力视图](./ispc.md) - 了解可验证计算能力

