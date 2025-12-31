# multitype - 多数据类型模型

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 multitype 模型相关功能

---

## 📍 组件定位

多数据类型模型示例，用于测试 WES 平台对不同数据类型的支持。该模型接受两种不同类型的输入（uint8 和 float64），并产生两种不同类型的输出（int16 和 int64），测试类型转换和兼容性。

## 文件说明

- **example_multitype.onnx**: ONNX 格式的模型文件
- **generate_network_different_types.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释模型设计、类型转换、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习 ONNX 模型生成和 WES 平台数据类型支持

## 模型规格

### 输入

1. **InputA**
   - 形状: `[1, 1, 1]`
   - 类型: `uint8`
   - 描述: 8 位无符号整数张量（0-255）

2. **InputB**
   - 形状: `[1, 2, 2]`
   - 类型: `float64`
   - 描述: 64 位双精度浮点张量

### 输出

1. **OutputA**
   - 形状: `[1, 2, 2]`
   - 类型: `int16`
   - 描述: 16 位有符号整数张量
   - 计算: `(InputB * InputA[0][0][0]) - 512`

2. **OutputB**
   - 形状: `[1, 1, 1]`
   - 类型: `int64`
   - 描述: 64 位有符号整数张量
   - 计算: `InputA * 1234`

## 使用方法

### 重新生成模型

```bash
cd multitype
python generate_network_different_types.py
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
session = ort.InferenceSession("example_multitype.onnx")

# 准备输入数据
input_a = np.random.randint(0, 256, (1, 1, 1), dtype=np.uint8)
input_b = np.random.rand(1, 2, 2).astype(np.float64)

# 运行推理
outputs = session.run(
    ["OutputA", "OutputB"],
    {"InputA": input_a, "InputB": input_b}
)

print(f"OutputA (int16): {outputs[0]}")
print(f"OutputB (int64): {outputs[1]}")
```

### WES 部署

```bash
wes ai deploy example_multitype.onnx \
    --name "Multi Type Model" \
    --description "Test model for different data types"
```

## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：建议使用当前主干分支对应的最新构建（例如通过 `make build-test` 生成的 `weisyn-testing`）
- **运行环境**：`env = testing`，单节点模式（`configs/testing/config.json` 中 `mining.enable_aggregator = false`）
- **关键依赖**：
  - `onnxruntime_go`：与项目 `go.mod` 中版本一致
  - Go / Python 版本与项目开发环境一致

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称      | 形状       | 数据类型   | 字段         | 示例值                         |
|-----------|------------|------------|--------------|--------------------------------|
| `InputA`  | `[1,1,1]`  | `uint8`    | `uint8_data` | `[128]`                        |
| `InputB`  | `[1,2,2]`  | `float64`  | `data`       | `[1.0, 2.0, 3.0, 4.0]`         |

对应的 JSON 输入片段（与 `testcases/default.json` 和 `onnx_models_test.sh` 推荐用法一致）：

```json
[
  {
    "name": "InputA",
    "uint8_data": [128],
    "shape": [1, 1, 1],
    "data_type": "uint8"
  },
  {
    "name": "InputB",
    "data": [1.0, 2.0, 3.0, 4.0],
    "shape": [1, 2, 2],
    "data_type": "float64"
  }
]
```

#### 期望输出

- 输出张量数量：2
- 输出 0（`OutputA`）：
  - 形状：`[1, 2, 2]`
  - 类型：`int16`
  - 典型值（前缀示例）：`[-384, -256, -128, 0]`
- 输出 1（`OutputB`）：
  - 形状：`[1, 1, 1]`
  - 类型：`int64`
  - 典型值：`[157952]`

> 注意：示例值与 onnxruntime_go 仓库中的测试用例保持一致，用于校验类型转换和运算逻辑。

### 3. 典型复现步骤

#### 脚本路径（推荐）

```bash
# 1. 构建测试二进制
make build-test

# 2. 从项目根目录运行单模型测试
bash scripts/testing/models/onnx_models_test.sh example_multitype
```

脚本会：

1. 通过 `test_init.sh` 初始化测试环境
2. 启动单节点 `weisyn-testing`
3. 部署 `example_multitype.onnx`
4. 等待交易确认与资源索引写入
5. 调用模型并检查输出结构与数据类型（后续脚本可根据 `testcases/default.json` 做更精确的数值断言）

#### JSON-RPC / CLI 路径（链路级验证）

1. 部署模型同上 `wes ai deploy`。
2. 记下 `content_hash` 与 `tx_hash`，通过：
   - `wes_getResourceByContentHash` 验证资源信息；
   - `wes_getTransactionReceipt` 确认部署交易上链。
3. 手工构造调用请求（见下节 JSON-RPC 示例）并核对输出形状与数据类型。

### 4. 已知限制与回归要求

- **类别**：`Basic`（基础功能模型，用于验证多数据类型支持）
- **已知限制**：
  - 输出 0 使用 `int16` 类型，需依赖当前 ONNX 引擎和 onnxruntime 的类型支持状况；如遇到库限制，可在测试脚本中降级为“Edge-OK”处理，但本 README 按理想支持情形描述。
- **回归要求**：
  - 修改 ONNX 引擎对多数据类型（尤其是 `uint8` / `int16` / `float64`）支持逻辑时，必须重跑本用例；
  - 升级 `onnxruntime_go` 或底层 ONNX Runtime 时，也应重跑本用例，验证类型转换行为是否保持一致。

### WES 平台调用示例

**JSON-RPC 调用**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_callAIModel",
  "params": [{
    "private_key": "0x<your_private_key>",
    "model_hash": "<model_content_hash>",
    "inputs": [
      {
        "name": "InputA",
        "uint8_data": [128],
        "shape": [1, 1, 1],
        "data_type": "uint8"
      },
      {
        "name": "InputB",
        "data": [1.0, 2.0, 3.0, 4.0],
        "shape": [1, 2, 2],
        "data_type": "float64"
      }
    ]
  }],
  "id": 1
}
```

**关键注意事项**:
- ✅ **InputA**: 必须使用 `uint8_data` 字段（不是 `data` 字段）
- ✅ **InputB**: 使用 `data` 字段，指定 `data_type: "float64"`
- ✅ **数据类型**: WES 平台支持 `uint8`、`int64`、`float32`、`float64` 等类型
- ✅ **float64 支持**: WES 平台完全支持 float64，无需类型转换

**⚠️ 已知限制**:
- **int16 输出类型**: 模型输出 `OutputA` 是 `int16` 类型，但 WES 平台（基于 onnxruntime_go）可能不完全支持 `int16`。如果调用失败，这是预期的，因为库限制。
- **int32 类型**: WES 平台不支持 `int32` 类型，只能通过 `int64` 传递。

## 测试场景

- ✅ uint8 数据类型支持
- ✅ float64 数据类型支持（完全支持，无需转换）
- ✅ int16 数据类型支持（通过 int64 传递）
- ✅ int64 数据类型支持
- ✅ 类型转换处理
- ✅ 混合类型输入输出

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


