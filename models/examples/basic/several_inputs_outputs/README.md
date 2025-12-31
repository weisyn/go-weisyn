# several_inputs_outputs - 多输入多输出模型

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 several_inputs_outputs 模型相关功能

---

## 📍 组件定位

多输入多输出模型示例，用于测试 WES 平台处理多个输入和输出的能力。该模型包含 3 个输入和 2 个输出，具有不同的数据类型和维度。

## 文件说明

- **example_several_inputs_and_outputs.onnx**: ONNX 格式的模型文件
- **generate_several_inputs_and_outputs.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释模型设计、多输入输出处理、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习 ONNX 模型生成和 WES 平台多输入输出支持

## 模型规格

### 输入

1. **input 1**
   - 形状: `[2, 5, 2, 5]`
   - 类型: `int32`
   - 描述: 4 维整数张量

2. **input 2**
   - 形状: `[2, 3, 20]`
   - 类型: `float32`
   - 描述: 3 维浮点张量

3. **input 3**
   - 形状: `[9]`
   - 类型: `bfloat16`
   - 描述: 1 维 bfloat16 张量

### 输出

1. **output 1**
   - 形状: `[10, 10]`
   - 类型: `int64`
   - 描述: 2 维整数张量（由 input 1 重塑）

2. **output 2**
   - 形状: `[1, 2, 3, 4, 5]`
   - 类型: `double`
   - 描述: 5 维双精度浮点张量（由 input 2 重塑）

## 使用方法

### 重新生成模型

```bash
cd several_inputs_outputs
python generate_several_inputs_and_outputs.py
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
session = ort.InferenceSession("example_several_inputs_and_outputs.onnx")

# 准备输入数据
input1 = np.zeros((2, 5, 2, 5), dtype=np.int32)
input2 = np.zeros((2, 3, 20), dtype=np.float32)
input3 = np.zeros((9), dtype=np.float16)  # bfloat16 在 numpy 中用 float16 近似

# 运行推理
outputs = session.run(
    ["output 1", "output 2"],
    {"input 1": input1, "input 2": input2, "input 3": input3}
)

print(f"Output 1 shape: {outputs[0].shape}")
print(f"Output 2 shape: {outputs[1].shape}")
```

### WES 部署

```bash
wes ai deploy example_several_inputs_and_outputs.onnx \
    --name "Multi Input Output Model" \
    --description "Test model for multiple inputs and outputs"
```
## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：推荐使用当前主干分支对应的 `weisyn-testing` 构建（`make build-test`）
- **运行环境**：`env = testing`，单节点模式（`configs/testing/config.json` 中 `mining.enable_aggregator = false`）
- **关键依赖**：与项目 `go.mod` 中的 `onnxruntime_go` 版本保持一致

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称       | 形状          | 数据类型   | 字段        | 示例值说明             |
|------------|---------------|------------|-------------|------------------------|
| `input 1`  | `[2,5,2,5]`   | `int32`    | `int32_data` | 100 个 0              |
| `input 2`  | `[2,3,20]`    | `float32`  | `data`      | 120 个 0.0            |
| `input 3`  | `[9]`         | `bfloat16` | `data`      | 9 个 0.0（float32 近似） |

与脚本 `get_test_inputs()` 和 `testcases/default.json` 保持一致的 JSON 片段：

```json
[
  {
    "name": "input 1",
    "int32_data": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    "shape": [2, 5, 2, 5],
    "data_type": "int32"
  },
  {
    "name": "input 2",
    "data": [0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0],
    "shape": [2, 3, 20],
    "data_type": "float32"
  },
  {
    "name": "input 3",
    "data": [0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0],
    "shape": [9],
    "data_type": "bfloat16"
  }
]
```

#### 期望输出

- 输出 0（`output 1`）：
  - 形状：`[10, 10]`
  - 类型：`int64`
- 输出 1（`output 2`）：
  - 形状：`[1, 2, 3, 4, 5]`
  - 类型：`float64`

测试脚本当前主要检查形状和数据类型，数值用于日志观察。

### 3. 典型复现步骤

#### 脚本路径（推荐）

```bash
make build-test
bash scripts/testing/models/onnx_models_test.sh example_several_inputs_and_outputs
```

脚本会自动完成部署、触发单节点挖矿、等待交易确认与资源索引写入，并使用上面的输入进行调用。

#### JSON-RPC / CLI 路径（链路级验证）

1. 使用 `wes ai deploy` 部署模型（见上文 “WES 部署” 示例）。  
2. 使用 `wes_getResourceByContentHash` / `wes_getTransactionReceipt` 验证：
   - 模型资源已写入链上；
   - 部署交易已被打包到区块。
3. 构造与上面 JSON 片段等价的 `wes_callAIModel` 请求，确认调用成功且输出结构符合预期。

### 4. 已知限制 & 回归要求

- **类别**：`Basic`（基础功能，多输入多输出 + 多类型）
- **已知限制**：
  - `input 3` 使用 bfloat16，在 WES 内部通过 float32 → bfloat16 编码实现，存在精度近似。
- **回归要求**：
  - 修改 ONNX 引擎对 `int32_data` / `bfloat16` 预处理逻辑，或升级 `onnxruntime_go` / ONNX Runtime 时，应重跑本用例；
  - 若测试脚本改为从 `testcases/default.json` 读取用例，也需保证 README 中的描述与 JSON 保持一致。

## 测试场景

- ✅ 多输入处理
- ✅ 多输出处理
- ✅ 输入输出名称映射
- ✅ 不同数据类型支持（int32, float32, bfloat16→float32, int64, double）
- ✅ 不同维度处理
- ⚠️ **bfloat16 限制**: 通过 float32 近似实现，存在精度差异

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


