# odd_name - 特殊字符文件名模型

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 odd_name 模型相关功能

---

## 📍 组件定位

特殊字符文件名模型，用于测试文件名编码处理。该模型文件名包含 Unicode 特殊字符（ż, 大, 김），主要用于验证 WES 平台对 Unicode 文件名的支持和文件系统兼容性。

## 文件说明

- **example ż 大 김.onnx**: ONNX 格式的模型文件（包含 Unicode 字符）
- **generate_odd_name_onnx.py**: 用于生成模型的 Python 脚本
  - ⭐ **详细注释**：脚本包含详细的中文注释，解释 Unicode 文件名、文件系统兼容性、ONNX 导出等关键概念
  - 📚 **学习价值**：适合学习 Unicode 文件名处理和 WES 平台文件系统兼容性

## 模型规格

### 输入
- **名称**: `in`
- **形状**: `[1, 2]`
- **类型**: `int32`
- **描述**: 1x2 整数张量

### 输出
- **名称**: `out`
- **形状**: `[1]`
- **类型**: `int32`
- **描述**: 输入两个元素的和

### 计算过程
- 对输入张量按行求和
- 返回标量结果

### 文件名特点
- 包含 Unicode 字符：
  - `ż` (波兰语字符)
  - `大` (中文字符)
  - `김` (韩文字符)
- 测试文件系统编码兼容性

## 使用方法

### 重新生成模型

```bash
cd odd_name
python generate_odd_name_onnx.py
```

**注意**: 生成的文件名包含特殊字符，某些文件系统或工具可能无法正确处理。

### 依赖要求

```bash
pip install torch onnx
```

### Python 测试示例

```python
import onnxruntime as ort
import numpy as np

# 加载模型（注意文件名编码）
model_path = "example ż 大 김.onnx"
session = ort.InferenceSession(model_path)

# 准备输入数据
inputs = np.ones((1, 2), dtype=np.int32)

# 运行推理
outputs = session.run(["out"], {"in": inputs})

print(f"Output: {outputs[0]}")  # 应该是 [2]
```

### WES 部署

```bash
# 注意：某些系统可能需要特殊处理文件名
wes ai deploy "example ż 大 김.onnx" \
    --name "Odd Name Model" \
    --description "Test model with Unicode filename"
```
## 🧪 测试规范（WES）

### 1. 参考环境

- **WES 版本**：`weisyn-testing`（`make build-test`）
- **运行环境**：`env = testing`，单节点模式

### 2. 基准测试用例（Canonical Test Case）

#### 输入定义

| 名称 | 形状    | 数据类型  | 字段        | 示例值 |
|------|---------|-----------|-------------|--------|
| `in` | `[1, 2]` | `int32`   | `int32_data` | `[1,2]` |

对应 JSON 片段：

```json
[
  {
    "name": "in",
    "int32_data": [1, 2],
    "shape": [1, 2],
    "data_type": "int32"
  }
]
```

#### 期望输出

- 输出 0（`out`）：
  - 形状：`[1]`
  - 类型：`int32`
  - 示例值：`[3]`

### 3. 典型复现步骤

```bash
make build-test
bash scripts/testing/models/onnx_models_test.sh example_odd_name
```

### 4. 已知限制 & 回归要求

- **类别**：`Basic`（文件名 & int32 输入）
- 回归时需重点验证：
  - 不同操作系统下对 Unicode 文件名的处理是否影响部署；
  - `int32_data` 输入路径在引擎调整后仍能正确工作。

## 测试场景

- ✅ Unicode 文件名支持
- ✅ 文件系统编码兼容性
- ✅ 文件路径处理
- ✅ 特殊字符处理

## 注意事项

- 某些文件系统（如 FAT32）可能不支持某些 Unicode 字符
- 在不同操作系统上文件名显示可能不同
- 建议在部署前测试文件名兼容性

## 模型来源

**原始仓库**: [onnxruntime_go](https://github.com/yalue/onnxruntime_go)  
**许可证**: MIT License


