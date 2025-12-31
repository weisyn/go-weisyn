# dynamic_axes - 动态轴模型

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 dynamic_axes 模型相关功能

---

## 📍 组件定位

动态轴模型，用于测试动态形状处理。该模型接受可变批次大小的输入（动态批次维度），主要用于验证 WES 平台对动态输入大小的支持和运行时形状推断能力。

## 文件说明

- **example_dynamic_axes.onnx**: ONNX 格式的模型文件
- **generate_dynamic_axes_network.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释动态轴、动态批次大小、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习动态输入大小处理和 WES 平台运行时形状推断

## 模型规格

### 输入
- **名称**: `input_vectors`
- **形状**: `[-1, 10]` (动态批次大小)
- **类型**: `float32`
- **描述**: 批次大小可变，每行 10 个特征

### 输出
- **名称**: `output_scalars`
- **形状**: `[-1]` (动态批次大小)
- **类型**: `float32`
- **描述**: 每个输入向量的和（标量）

### 计算过程
- 对每个输入向量按行求和
- 输出每个向量的总和

### 动态轴
- 批次维度（第 0 维）是动态的
- 运行时根据实际输入大小确定

## 使用方法

### 重新生成模型

```bash
cd dynamic_axes
python generate_dynamic_axes_network.py
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
session = ort.InferenceSession("example_dynamic_axes.onnx")

# 准备不同批次大小的输入
inputs_batch1 = np.random.rand(1, 10).astype(np.float32)
inputs_batch5 = np.random.rand(5, 10).astype(np.float32)
inputs_batch10 = np.random.rand(10, 10).astype(np.float32)

# 运行推理（不同批次大小）
output1 = session.run(["output_scalars"], {"input_vectors": inputs_batch1})
output5 = session.run(["output_scalars"], {"input_vectors": inputs_batch5})
output10 = session.run(["output_scalars"], {"input_vectors": inputs_batch10})

print(f"Batch 1 output shape: {output1[0].shape}")   # (1,)
print(f"Batch 5 output shape: {output5[0].shape}")   # (5,)
print(f"Batch 10 output shape: {output10[0].shape}") # (10,)
```

### WES 部署

```bash
wes ai deploy example_dynamic_axes.onnx \
    --name "Dynamic Axes Model" \
    --description "Test model for dynamic batch size"
```
## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：`weisyn-testing`（`make build-test`）
- **运行环境**：`env = testing`，单节点模式

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称            | 形状     | 数据类型  | 字段  | 示例值                         |
|-----------------|----------|-----------|-------|--------------------------------|
| `input_vectors` | `[1,10]` | `float32` | `data` | `[1.0, 2.0, ..., 10.0]` |

对应 JSON 片段：

```json
[
  {
    "name": "input_vectors",
    "data": [1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0],
    "shape": [1, 10],
    "data_type": "float32"
  }
]
```

#### 期望输出

- 输出 0（`output_scalars`）：
  - 形状：`[1]`
  - 类型：`float32`
  - 典型值：`[55.0]`（1..10 求和）

### 3. 典型复现步骤

```bash
make build-test
bash scripts/testing/models/onnx_models_test.sh example_dynamic_axes
```

### 4. 已知限制 & 回归要求

- **类别**：`Basic`（动态批次维度功能验证）
- 回归时重点验证：
  - 批次维度为 1 时形状与数值是否正确；
  - 后续可扩展用例覆盖批次为 5、10 的情况。

## 测试场景

- ✅ 动态形状处理
- ✅ 运行时形状推断
- ✅ 可变批次大小
- ✅ 动态轴支持

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


