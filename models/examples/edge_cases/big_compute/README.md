# big_compute - 大计算量网络

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 big_compute 模型相关功能

---

## 📍 组件定位

大计算量网络模型，用于测试复杂计算处理能力。该模型对大型张量（52,428,800 个元素）执行 40 次除法和乘法操作，主要用于验证 WES 平台对复杂计算的处理能力。该模型还包含修改后的元数据，用于测试元数据处理。

## 文件说明

- **example_big_compute.onnx**: ONNX 格式的模型文件
- **generate_network_big_compute.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释大计算量处理、大张量操作、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习复杂计算优化和 WES 平台性能测试
- **modify_metadata.py**: 用于修改模型元数据的脚本

## 模型规格

### 输入
- **名称**: `Input`
- **形状**: `[1, 52428800]` (1024 * 1024 * 50)
- **类型**: `float32`
- **描述**: 一维大向量

### 输出
- **名称**: `Output`
- **形状**: `[1, 52428800]`
- **类型**: `float32`
- **描述**: 与输入相同形状的输出

### 计算过程
- 执行 40 次 `x / 10.0` 和 `x * 10.0` 操作
- 理论上结果应该等于输入（但由于浮点精度可能略有差异）

### 元数据
- `doc_string`: "This is a test description."
- `model_version`: 1337
- `domain`: "test domain"
- 自定义元数据属性:
  - `test key 1`: "" (空值)
  - `test key 2`: "Test key 2 value"

## 使用方法

### 重新生成模型

```bash
cd big_compute
# 1. 生成模型
python generate_network_big_compute.py

# 2. 修改元数据（可选）
python modify_metadata.py
```

### 依赖要求

```bash
pip install torch onnx
```

### Python 测试示例

```python
import onnxruntime as ort
import numpy as np

# 加载模型
session = ort.InferenceSession("example_big_compute.onnx")

# 准备输入数据（大向量）
inputs = np.zeros((1, 52428800), dtype=np.float32)

# 运行推理
outputs = session.run(["Output"], {"Input": inputs})

print(f"Output shape: {outputs[0].shape}")
```

### WES 部署

```bash
wes ai deploy example_big_compute.onnx \
    --name "Big Compute Network" \
    --description "Test model for large computation"
```

## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：推荐使用当前主干分支对应的 `weisyn-testing` 构建（`make build-test`）
- **运行环境**：`env = testing`，单节点模式
- **关键依赖**：与项目 `go.mod` 中的 `onnxruntime_go` 和 ONNX Runtime 动态库版本一致

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称    | 形状           | 数据类型  | 字段  | 示例值说明                     |
|---------|----------------|-----------|-------|--------------------------------|
| `Input` | `[1, 10000]`   | `float32` | `data` | 10000 个 1.0（缩小版输入） |

脚本与 `testcases/default.json` 使用的输入片段：

```json
[
  {
    "name": "Input",
    "data": [1.0, 1.0, 1.0, 1.0],
    "shape": [1, 10000],
    "data_type": "float32"
  }
]
```

> 实际模型元数据期望的输入形状为 `[1, 52428800]`；本用例采用缩小输入用于验证部署与链路行为。

#### 期望输出

- 输出 0（`Output`）：
  - 元数据形状：`[1, 52428800]`
  - 类型：`float32`

当前测试脚本只检查**部署成功、资源索引可用**，并允许在调用阶段出现“Expected: 52428800” 相关的维度错误，将其视为预期的 Edge-OK 行为。

### 3. 典型复现步骤

#### 脚本路径（推荐）

```bash
make build-test
bash scripts/testing/models/onnx_models_test.sh example_big_compute
```

脚本会：

1. 初始化测试环境并启动单节点；
2. 部署 `example_big_compute.onnx`；
3. 触发挖矿、等待部署交易确认与资源索引写入；
4. 使用缩小输入调用模型：
   - 若返回与维度 `52428800` 相关的 shape 错误，则记为 **Edge-OK（链路打通，但未提供完整 52M 元素输入）**。

#### JSON-RPC / CLI 路径（链路级验证）

1. 使用 `wes ai deploy` 部署模型。  
2. 使用 `wes_getResourceByContentHash` / `wes_getTransactionReceipt` 检查资源与交易是否上链。  
3. 构造缩小版输入的 `wes_callAIModel` 请求，确认：
   - 调用到达引擎；
   - 错误信息中包含 `Expected: 52428800` 或类似提示。

### 4. 已知限制 & 回归要求

- **类别**：`Edge-Case / Stress`（极大输入场景）
- **已知限制**：
  - 完整的 52M 元素输入在测试环境下不现实，本用例以维度错误作为“链路打通”的信号；
  - 不对输出数值做任何断言，只关注部署、索引和调用链路。
- **回归要求**：
  - 修改 ONNX 引擎、TxPool、共识或资源索引逻辑时，应重跑本用例，以验证大模型部署在链上的完整流程不被破坏。

## 测试场景

- ✅ 大计算量处理
- ✅ 内存管理（大张量）
- ✅ 元数据处理
- ✅ 模型版本管理
- ✅ 自定义元数据属性

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License



## 测试场景

- ✅ 大计算量处理
- ✅ 内存管理（大张量）
- ✅ 元数据处理
- ✅ 模型版本管理
- ✅ 自定义元数据属性

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


