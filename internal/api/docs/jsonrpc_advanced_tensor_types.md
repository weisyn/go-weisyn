# WES JSON-RPC 高级张量类型协议规范（草案）

---

## 📌 版本信息

- **版本**：1.0
- **状态**：draft
- **最后更新**：2025-11-15
- **最后审核**：2025-11-15
- **所有者**：接口规范组
- **适用范围**：WES JSON-RPC 接口中与高级张量类型（float16 / bfloat16 / 量化张量等）相关的请求与响应设计

---

## 🎯 目标

- **统一**：为 JSON-RPC 接口中的张量输出提供唯一、规范的表达方式，适配 float16 / bfloat16 / int8 量化 / bool / int64 等常见 dtype。
- **简单**：对只关心数值的调用方，仍然提供方便消费的数值数组视图（`values`）。
- **可复现**：通过 `raw_data` 提供 base64 编码的原始字节，以支持链下/链上重放验证（在可行的类型范围内）。
- **可演进**：保持字段级别向前兼容，允许未来在不破坏调用方的前提下扩展更多 dtype / 量化方案。

---

## 📍 范围与非目标

### ✅ 本规范包含

- JSON-RPC 响应中 **扩展张量结果区** 的结构定义（`tensor_outputs`）。
- 请求中 **协议版本与能力协商** 字段的约定。
- 对 **高级 dtype 与量化张量** 的表达方式（dtype、shape、raw bytes、量化参数）。
- 对客户端 SDK 在消费这些字段时的抽象建议。

### ❌ 本规范不包含

- 模型权重（weights）的导出与动态加载协议。
- 中间激活（intermediate activations）的全量 dump 协议。
- 面向特定业务（例如文本分类、检索、语音识别）的高层语义业务 API 设计。

---

## 🧭 设计原则

### 原则 1：单一真相（Single Source of Truth）

- 推理相关 JSON-RPC 方法的张量输出 **统一使用 `tensor_outputs` 字段** 表达。
- 不再在协议层维护旧的 `outputs`（`[][]float64`）字段作为并行视图。
- 对数值视图的需求，通过 `tensor_outputs[*].values` 统一满足。

### 原则 2：语义透明

- 所有精度变化（例如 float16 → float32）必须在字段和文档中显式说明。
- 禁止“隐形转换”：不得将 float16 当作 float32 暴露却不标注真实 dtype。

### 原则 3：表达优先，易用次之

- 第一优先级是 **信息可被完整表达**（无损表达 dtype、shape、layout、raw bytes、量化元数据）。
- 易用性通过 SDK 与工具封装来解决，而不是牺牲表达能力。

### 原则 4：与现有 JSON-RPC 规范分工清晰

- 本规范只定义 **张量相关字段与结构**，不改变区块/交易等 JSON-RPC 方法的语义。
- 与 [WES JSON-RPC API 规范](./jsonrpc_spec.md) 保持互补关系：  
  该文档定义“有哪些方法”，本规范定义“这些方法在涉及张量输出时如何表达结果”。

---

## 📦 统一响应字段：`tensor_outputs`

### 结构概览

在涉及 **推理结果 / 张量输出** 的 JSON-RPC 方法中，`result` 统一包含字段 `tensor_outputs`，用于表达所有张量输出。

```json
{
  "jsonrpc": "2.0",
  "result": {
    "tensor_outputs": [
      {
        "name": "logits",
        "dtype": "float64",
        "shape": [1, 1000],
        "layout": "NCHW",
        "encoding": "base64",
        "raw_data": "....",
        "values": [0.1, 0.2, 0.3],
        "quantization": null
      }
    ]
  },
  "id": 1
}
```

### 字段定义

- **`name`**：  
  - 类型：`string`  
  - 说明：张量名称，用于在客户端侧进行引用和调试，例如 `"logits"`、`"embedding"`。

- **`dtype`**：  
  - 类型：`string`  
  - 说明：张量元素的数据类型，必须是本规范定义的枚举值之一（见下文）。

- **`shape`**：  
  - 类型：`number[]`  
  - 说明：张量的形状，按约定维度顺序给出，例如 `[batch, channels, height, width]`。

- **`layout`**（可选）：  
  - 类型：`string`  
  - 示例：`"NCHW"`、`"NHWC"`  
  - 说明：多维张量的维度语义布局，便于跨框架解释。

- **`encoding`**：  
  - 类型：`string`  
  - 当前允许值：`"base64"`  
  - 说明：`raw_data` 使用的编码方式，未来如扩展其他编码方式必须在此明确声明。

- **`raw_data`**：  
  - 类型：`string`  
  - 说明：张量内容的原始字节序列，使用 `encoding` 指定的方式编码（默认 base64）。

- **`values`**：  
  - 类型：`number[]`  
  - 说明：按 `shape` 展平顺序排列的数值数组，便于调用方直接消费与可视化。  
  - 要求：在服务端可行的情况下，**必须提供**；当出于体积或安全考虑不返回具体数值时，可设为 `[]` 并在文档/日志中注明。

- **`quantization`**（可选）：  
  - 类型：`null | object`  
  - 说明：量化张量的元信息，不是量化张量时为 `null`。示例：

    ```json
    "quantization": {
      "scheme": "per_tensor_affine",
      "scale": 0.02,
      "zero_point": 128,
      "axis": null
    }
    ```

---

## 🔤 `dtype` 枚举与语义

### 支持的 `dtype` 列表

本规范建议与 ONNX / NumPy 常用 dtype 对齐，初始支持如下枚举值：

- **浮点类型**：
  - `float16`
  - `bfloat16`
  - `float32`
  - `float64`
- **整数类型**：
  - `int8`
  - `uint8`
  - `int16`
  - `int32`
  - `int64`
- **逻辑类型**：
  - `bool`

如需扩展其他 dtype（例如自定义定点格式），必须：

- 在本规范的后续版本中增加相应枚举值；
- 或使用约定的扩展机制（例如 `"custom:xint4"`），并在文档中说明序列化与对齐方式。

> **当前实现说明（2025-11-15）**  
> - ONNX 引擎层已经统一返回富张量结构，`DType` 由 ONNX 元数据映射，`Shape` 优先使用模型的输出维度（若包含动态轴则回退为 `[len(values)]`）；  
> - 对于 `DType="float32"` 的输出，`RawData` 使用 **小端 float32** 编码；对于其他已支持的数值类型，当前阶段 `RawData` 仍为 `values` 的 **小端 float64** 编码；  
> - 后续版本会在不破坏 `tensor_outputs` 字段结构的前提下，逐步为更多 dtype（如 `float16`/`bfloat16`/`int64` 等）提供与 `DType` 完全对齐的原始字节表示。

### 引擎内部映射要求

- 所有底层运行时 / ONNX TensorProto 的 dtype 映射关系必须在实现文档中明确列出。
- 禁止出现“内部为 float16，但对外标注为 float32”这类不一致情况。

---

## 🔁 与现有 `output` 字段的关系

### 兼容策略

- 对于现有 JSON-RPC 响应中的 `output` 字段：
  - 表示的是 **“用户友好视图”**，优先面向上层业务与可视化使用；
  - 可以对底层张量进行 **类型转换或后处理**（例如 float16 → float32、量化反量化）。
- 对于新增的 `tensor_outputs` 字段：
  - 表示的是 **“无损视图”**，必须忠实反映底层引擎输出的 dtype 和 raw bytes；
  - 不允许在这里改变 dtype 或进行任何隐式数值变换。

### 典型场景约定

- 底层为 `float16` / `bfloat16`：
  - `output` 中返回转换为 `float32` 的数据，便于常规 chart/图表展示和计算；
  - `tensor_outputs` 中保留原始 `float16` / `bfloat16` bytes 与真实 dtype。

- 底层为 **量化张量（如 `int8`）**：
  - `output` 中可返回已经 **反量化到 float32** 的数据；
  - `tensor_outputs` 中保留原始整数 bytes，并在 `quantization` 中附加量化参数。

---

## 🧩 协议版本与能力协商

### 请求侧：声明期望的协议版本与能力

建议在 JSON-RPC 请求的 `params` 中扩展以下字段：

```json
{
  "jsonrpc": "2.0",
  "method": "wes_infer",
  "params": {
    "api_version": "v2",
    "capabilities": {
      "accept_tensor_outputs": true,
      "preferred_dtypes": ["float16", "bfloat16"],
      "max_raw_size_bytes": 1048576
    },
    "inputs": {
      "...": "模型输入部分"
    }
  },
  "id": 1
}
```

- **`api_version`**：  
  - 客户端期望使用的 API 版本（例如：`"v1"`、`"v2"`）。
  - 服务端可以根据版本决定是否启用 `tensor_outputs`、是否保持旧行为等。

- **`capabilities`**：  
  - `accept_tensor_outputs`（bool）：客户端是否能消费 `tensor_outputs`。  
  - `preferred_dtypes`（string[]）：客户端偏好的结果 dtype，可用于未来的精度协商。  
  - `max_raw_size_bytes`（number）：客户端可接受的 `tensor_outputs` 单次最大字节数，用于防止响应过大。

### 响应侧：回显实际采用的版本与特性

服务端在响应中回显实际采用的版本与启用的特性，便于客户端做精细化处理：

```json
{
  "jsonrpc": "2.0",
  "result": {
    "api_version": "v2",
    "features": {
      "tensor_outputs": true,
      "dtype_preserved": ["float16", "bfloat16"],
      "quantization_info": true
    },
    "output": { "...": "用户友好视图" },
    "tensor_outputs": [ "...": "无损视图" ]
  },
  "id": 1
}
```

说明：

- `api_version`：服务端实际选择的版本（可能不同于请求中的期望值）。  
- `features.tensor_outputs`：是否返回了 `tensor_outputs`。  
- `features.dtype_preserved`：哪些 dtype 在本次响应中严格保真。  
- `features.quantization_info`：量化信息是否完整返回。

---

## 🧰 SDK 与工具链建议

> 本章节为 **强建议的设计模式**，不强制规定具体实现细节。

### 客户端 SDK 抽象（以 Python 为例）

SDK 内部负责处理 base64 解码、dtype/shape 解释、量化反量化等细节，对调用方暴露统一抽象：

```python
result = client.infer(...)

# 用户友好视图：适合可视化/快速使用
logits_np = result.outputs["logits"].to_numpy(dtype="float32")

# 无损视图：保留底层 dtype + raw bytes
raw_tensor = result.outputs["logits"].raw()
# raw_tensor 可能包含：
# - data: bytes
# - dtype: str
# - shape: List[int]
# - quantization: Optional[Dict]
```

SDK 应至少满足：

- 统一处理 `tensor_outputs` 中的 base64 → bytes 解码；
- 根据 `dtype` / `shape` 组装为 NumPy / Torch 张量；
- 对量化张量提供：
  - 直接返回原始量化视图；
  - 或提供便捷的反量化 API。

### 工具链支持

- CLI 工具：支持从 JSON-RPC 响应导出张量为 `.npy` / `.npz` / `.pt` 等格式。
- 调试/可视化工具：支持展示 dtype、shape、部分数值预览，便于联调和问题定位。

---

## 🔗 与链上验证 / Phase A 的关系

- 若链上仅存储 **输入/输出/模型的哈希或摘要**，本规范主要影响链下 JSON-RPC 与 SDK，不改变链上结构。  
- 若未来需要支持 **链上重放验证**：
  - 可使用 `tensor_outputs.raw_data` + 模型哈希 + 输入哈希，实现 bit-level 输出复现；
  - 本规范保证输出侧表达能力足以支撑这一场景。
- 已存在的 `example_float16` 等用例：
  - 可作为第一批启用 `tensor_outputs` 的试点模型；
  - 同时保留现有 float32 友好视图，确保既有测试与上层业务不受影响。

---

## 🗺️ 迭代计划（建议）

### Phase 1：协议草案与 PoC

- 目标：
  - 确认 `tensor_outputs` 字段结构与 `dtype` 枚举列表；
  - 选取 1–2 个模型（例如 `example_float16` + 一个 int8 量化模型）完成端到端 PoC。
- 产出：
  - 文档：本规范初版；
  - 代码：服务端对 `tensor_outputs` 的最小实现；
  - 测试：最小回归集（结构字段检查 + 张量复原对比）。

### Phase 2：协议版本化与能力协商

- 目标：
  - 引入 `api_version` / `capabilities` 字段并在服务端实现实际分支行为；
  - 明确 v1/v2 的行为差异、兼容策略和默认行为。
- 产出：
  - 文档：版本说明与升级指南；
  - 代码：基于能力协商决定是否返回 `tensor_outputs`、是否对大张量进行截断等；
  - 测试：覆盖 v1-only / v2-with-features / 能力不匹配等组合。

### Phase 3：SDK 与工具链完善

- 目标：
  - 至少一个主力语言（推荐 Python）的 SDK 完整适配本规范；
  - 回归测试矩阵中纳入 dtype / 量化场景的全链路测试。
- 产出：
  - SDK：张量抽象类型、无损视图/友好视图 API；
  - 工具：CLI 导出、可视化支持；
  - 文档：SDK 使用示例与常见问题。

### Phase 4：评估与后续演进

- 目标：
  - 评估新协议在实际业务中的使用情况；
  - 决定是否：
    - 将部分旧字段标记为 deprecated；
    - 或在无损张量协议之上推出更语义化的“视图 API”。
- 产出：
  - 评估报告；
  - 后续提案（例如“语义视图 API 规范”）。

---

## 📚 相关文档

- [WES JSON-RPC API 规范](./jsonrpc_spec.md)
- [WES 项目规范体系总览](../../system/standards/README.md)
- [文档规范](../../system/standards/principles/documentation.md)

---

## 🔄 规范演进

| 版本 | 日期       | 变更内容 |
|------|------------|----------|
| 1.0  | 2025-11-15 | 初始版本，定义 `tensor_outputs` 结构、dtype 枚举、能力协商建议与迭代计划 |


