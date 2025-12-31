# 基本功能测试模型

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中基本功能测试模型相关功能

---

## 📍 组件定位

**基本功能测试模型** - 用于验证 WES 平台的 ONNX 模型执行基本功能，包括模型加载、推理执行、输入输出处理等核心能力。

## 模型列表

### 1. sklearn_randomforest

**模型文件**: `sklearn_randomforest.onnx`

**生成脚本**: `generate_sklearn_network.py`

**描述**: 
- 使用 scikit-learn 训练的随机森林分类器
- 基于 Iris 数据集（鸢尾花分类）
- 测试 ONNX Runtime 对 Map 和 Sequence 数据类型的支持
- sklearn 模型大量使用这些复杂数据类型

**输入**:
- 名称: `X`
- 形状: `[batch, 4]` (4 个特征)
- 类型: `float32`

**输出**:
- `output_label`: 预测类别标签
- `output_probability`: 预测概率分布

**使用示例**:
```python
import onnxruntime as ort
import numpy as np

session = ort.InferenceSession("sklearn_randomforest.onnx")
inputs = np.array([[5.1, 3.5, 1.4, 0.2]], dtype=np.float32)
outputs = session.run(["output_label", "output_probability"], {"X": inputs})
```

**重新生成**:
```bash
cd sklearn_randomforest
python generate_sklearn_network.py
```

**依赖**:
- scikit-learn
- skl2onnx
- onnxruntime
- numpy

---

### 2. several_inputs_outputs

**模型文件**: `example_several_inputs_and_outputs.onnx`

**生成脚本**: `generate_several_inputs_and_outputs.py`

**描述**:
- 多输入多输出模型示例
- 测试 WES 平台处理多个输入和输出的能力
- 验证输入输出名称匹配和顺序处理

**输入**:
- 多个输入张量

**输出**:
- 多个输出张量

**使用场景**:
- 验证多输入输出处理逻辑
- 测试输入输出名称映射
- 验证张量顺序处理

**重新生成**:
```bash
cd several_inputs_outputs
python generate_several_inputs_and_outputs.py
```

**依赖**:
- torch
- onnx

---

### 3. multitype

**模型文件**: `example_multitype.onnx`

**生成脚本**: `generate_network_different_types.py`

**描述**:
- 多数据类型模型示例
- 测试不同数据类型的支持（float32, int64, uint8 等）
- 验证 WES 平台的数据类型转换能力

**数据类型**:
- 支持多种 ONNX 标准数据类型
- 测试类型转换和兼容性

**使用场景**:
- 验证数据类型支持
- 测试类型转换逻辑
- 验证混合类型处理

**重新生成**:
```bash
cd multitype
python generate_network_different_types.py
```

**依赖**:
- torch
- onnx

---

## 测试流程

### 1. 部署模型

```bash
# 部署随机森林模型
wes ai deploy models/examples/basic/sklearn_randomforest/sklearn_randomforest.onnx \
    --name "Random Forest Classifier" \
    --description "Iris classification model"
```

### 2. 调用模型

```bash
# 使用 JSON-RPC API
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_callAIModel",
    "params": {
      "private_key": "your_private_key",
      "model_hash": "0x...",
      "inputs": [{
        "name": "X",
        "data": [5.1, 3.5, 1.4, 0.2],
        "shape": [1, 4],
        "data_type": "float32"
      }]
    },
    "id": 1
  }'
```

## 模型来源

所有模型和生成脚本来自 [onnxruntime_go](https://github.com/yalue/onnxruntime_go) 项目的 `test_data` 目录。

**原始仓库**: https://github.com/yalue/onnxruntime_go

**许可证**: MIT License


