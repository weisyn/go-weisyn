# sklearn_randomforest - 随机森林分类器

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 sklearn_randomforest 模型相关功能

---

## 📍 组件定位

基于 scikit-learn 训练的随机森林分类器，用于 Iris（鸢尾花）数据集分类。该模型主要用于测试 ONNX Runtime 对 Map 和 Sequence 数据类型的支持，因为 sklearn 模型大量使用这些复杂数据类型。

## 文件说明

- **sklearn_randomforest.onnx**: ONNX 格式的模型文件
- **generate_sklearn_network.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释 sklearn 模型转换、Map/Sequence 数据类型、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习 sklearn 模型转换和 WES 平台对复杂数据类型的支持

## 模型规格

### 输入
- **名称**: `X`
- **形状**: `[batch, 4]`
- **类型**: `float32`
- **描述**: 4 个特征值（花萼长度、花萼宽度、花瓣长度、花瓣宽度）

### 输出
- **output_label**: 预测的类别标签（int64）
- **output_probability**: 预测的概率分布（Map 类型）

## 使用方法

### 重新生成模型

```bash
cd sklearn_randomforest
python generate_sklearn_network.py
```

### 依赖要求

```bash
pip install scikit-learn skl2onnx onnxruntime numpy
```

### Python 测试示例

```python
import onnxruntime as ort
import numpy as np

# 加载模型
session = ort.InferenceSession("sklearn_randomforest.onnx")

# 准备输入数据（示例：Iris setosa）
inputs = np.array([[5.1, 3.5, 1.4, 0.2]], dtype=np.float32)

# 运行推理
outputs = session.run(["output_label", "output_probability"], {"X": inputs})

print(f"预测标签: {outputs[0]}")
print(f"预测概率: {outputs[1]}")
```

### WES 部署

```bash
wes ai deploy sklearn_randomforest.onnx \
    --name "Random Forest Classifier" \
    --description "Iris classification model from sklearn"
```

## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：建议使用当前主干分支对应的最新构建（例如通过 `make build-test` 生成的 `weisyn-testing`）
- **运行环境**：`env = testing`，单节点模式（`configs/testing/config.json` 中 `mining.enable_aggregator = false`）
- **关键依赖**：
  - `onnxruntime_go`：与项目 `go.mod` 中版本一致
  - Go 版本、Python 版本与项目开发环境一致

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称 | 形状   | 数据类型  | 字段  | 示例值                       |
|------|--------|-----------|-------|------------------------------|
| `X`  | `[1,4]` | `float32` | `data` | `[5.1, 3.5, 1.4, 0.2]` |

对应的 JSON 输入片段（与 `testcases/default.json` 及 `onnx_models_test.sh` 保持一致）：

```json
[
  {
    "name": "X",
    "data": [5.1, 3.5, 1.4, 0.2],
    "shape": [1, 4],
    "data_type": "float32"
  }
]
```

#### 期望输出

- 输出张量数量：2
- 输出 0（`output_label`）：
  - 形状：`[1]`
  - 类型：`int64`
  - 示例值：`[0]`（对应 Iris setosa）
- 输出 1（`output_probability`）：
  - 类型：Map（类别 → 概率）
  - 当前测试脚本只检查该输出存在且类型正确，不对具体概率分布数值做严格断言

### 3. 典型复现步骤

#### 脚本路径（推荐）

```bash
# 1. 构建测试二进制
make build-test

# 2. 从项目根目录运行单模型测试
bash scripts/testing/models/onnx_models_test.sh sklearn_randomforest
```

脚本将自动完成：

1. 使用 `scripts/testing/common/test_init.sh` 初始化测试环境（停止旧节点、根据 `configs/testing/config.json` 清理测试数据、准备日志目录等）
2. 启动 `weisyn-testing` 节点（单节点共识）
3. 部署 `sklearn_randomforest.onnx` 至链上（调用 `wes_deployAIModel`）
4. 等待部署交易确认并写入资源索引
5. 调用模型（`wes_callAIModel`），并验证输出结构

#### JSON-RPC / CLI 路径（链路级验证）

1. 部署模型（CLI 示例）：

```bash
wes ai deploy sklearn_randomforest.onnx \
  --name "Random Forest Classifier" \
  --description "Iris classification model from sklearn"
```

2. 记下返回的 `content_hash` 与 `tx_hash`，通过：
   - `wes_getResourceByContentHash` 验证链上 `Resource` 字段（`category=EXECUTABLE`、`executable_type=AIMODEL`、`content_hash` 与文件哈希一致等）
   - `wes_getTransactionReceipt` / `wes_getTransaction` 验证部署交易已被写入区块

3. 调用模型（JSON-RPC 示例）：

```json
{
  "jsonrpc": "2.0",
  "method": "wes_callAIModel",
  "params": [{
    "private_key": "0x<your_private_key>",
    "model_hash": "<model_content_hash>",
    "inputs": [
      {
        "name": "X",
        "data": [5.1, 3.5, 1.4, 0.2],
        "shape": [1, 4],
        "data_type": "float32"
      }
    ]
  }],
  "id": 1
}
```

### 4. 已知限制 & 回归要求

- **类别**：`Basic`（基础功能模型，要求部署 + 调用 + 链上校验完整通过）
- **已知限制**：
  - `output_probability` 为 Map 类型，在当前测试脚本中仅做存在性 / 类型检查，不对具体概率值做强约束。
- **回归要求**：
  - 每次修改 ONNX 引擎实现、Resource 索引 / TxPool / 共识链路，或升级 `onnxruntime_go` / ONNX Runtime 动态库时，必须至少重跑本用例，并检查：
    - 模型部署是否成功，`Resource` 字段与 `resource.proto` 约定一致；
    - 模型调用是否成功，`output_label` 是否在合理类别范围内；
    - Map 输出是否仍然存在且类型正确。

## 测试场景

- ✅ Map 数据类型支持
- ✅ Sequence 数据类型支持
- ✅ 多输出处理
- ✅ 复杂数据结构处理

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


