# ONNX 模型格式规范

## 版本信息
- **文档版本**: v1.0
- **创建日期**: 2024-11-12
- **最后更新**: 2024-11-12

## 模块定位

**ONNX 模型格式规范** - 定义 WES 平台支持的 ONNX 模型格式标准，包括模型结构、输入输出格式、数据类型、Opset 版本等要求。

## ONNX 标准

WES 平台基于 [ONNX (Open Neural Network Exchange)](https://onnx.ai/) 标准，这是一个开放的机器学习模型表示格式。

### 支持的 ONNX 版本

- **ONNX 规范版本**: 1.0+
- **Opset 版本**: 16, 17, 18（推荐使用最新版本）
- **运行时**: onnxruntime_go (基于 Microsoft ONNX Runtime)

## 模型文件格式

### 文件扩展名

- **标准扩展名**: `.onnx`
- **文件格式**: Protocol Buffers (protobuf) 二进制格式

### 文件结构

ONNX 模型文件包含以下主要部分：

1. **Model Metadata** - 模型元数据
   - 模型名称、版本、作者等
   - 模型描述和文档

2. **Graph** - 计算图
   - 节点（Nodes）- 操作符
   - 边（Edges）- 数据流
   - 输入输出定义

3. **Initializers** - 初始化器
   - 模型权重和偏置

## 输入输出规范

### 输入格式

**支持的输入类型**:
- `float32` - 32位浮点数（主要类型）
- `int64` - 64位整数（用于 NLP 模型的 token IDs）
- `uint8` - 8位无符号整数（用于图像数据）

**支持的输入形状**:
- 一维: `[batch]`
- 二维: `[batch, features]`
- 三维: `[batch, seq_len, features]`
- 四维: `[batch, channels, height, width]`（图像）
- 更高维度: 根据模型需求

**输入命名**:
- 输入必须有明确的名称
- 名称应与模型定义一致

### 输出格式

**支持的输出类型**:
- `float32` - 32位浮点数（主要类型）
- `int64` - 64位整数
- `uint8` - 8位无符号整数

**输出形状**:
- 根据模型任务而定
- 分类任务: `[batch, num_classes]`
- 回归任务: `[batch, output_dim]`
- 序列任务: `[batch, seq_len, features]`

## 数据类型映射

### WES TensorInput 到 ONNX 类型映射

| WES 字段 | ONNX 类型 | 说明 |
|---------|----------|------|
| `Data` (float64) | `float32` | 浮点数据 |
| `Int64Data` | `int64` | 整数数据（NLP） |
| `Uint8Data` | `uint8` | 无符号整数（图像） |

### 数据类型转换

WES 平台会自动处理数据类型转换：
- `float64` → `float32`（自动转换）
- `int64` → `int64`（直接使用）
- `uint8` → `uint8`（直接使用）

## Opset 版本

### 支持的 Opset 版本

- **Opset 16**: 基础支持
- **Opset 17**: 推荐版本
- **Opset 18**: 最新版本（推荐）

### Opset 选择建议

1. **新模型**: 使用 Opset 17 或 18
2. **兼容性**: 如果需要在多个运行时使用，选择 Opset 16
3. **新特性**: 需要新操作符时，使用 Opset 18

## 模型元数据

### 必需元数据

模型应包含以下元数据：

```protobuf
model {
  ir_version: 8
  opset_import {
    domain: ""
    version: 17
  }
  producer_name: "WES Model"
  producer_version: "1.0"
}
```

### 推荐元数据

- **模型名称**: 描述性名称
- **模型版本**: 版本号
- **作者信息**: 创建者
- **描述**: 模型用途和说明
- **输入输出说明**: 详细的输入输出描述

## 模型验证

### 验证工具

使用 ONNX Runtime 或 ONNX 工具验证模型：

```bash
# 使用 Python onnx 包验证
python -c "import onnx; onnx.checker.check_model('model.onnx')"

# 使用 onnxruntime_go 验证（在 WES 中）
# 模型加载时会自动验证
```

### 验证检查项

1. **格式检查**: 文件格式是否正确
2. **图结构**: 计算图是否有效
3. **类型检查**: 输入输出类型是否一致
4. **形状检查**: 张量形状是否兼容
5. **操作符支持**: 使用的操作符是否支持

## 模型优化建议

### 性能优化

1. **量化**: 使用 INT8 量化减小模型大小
2. **图优化**: 使用 ONNX Runtime 的图优化
3. **操作符融合**: 合并相关操作符

### 兼容性优化

1. **避免自定义操作符**: 使用标准 ONNX 操作符
2. **避免动态形状**: 尽量使用固定形状（如果可能）
3. **简化图结构**: 减少不必要的节点

## 常见问题

### Q: 模型文件太大怎么办？

A: 考虑以下方案：
- 使用模型量化（INT8）
- 使用模型剪枝
- 使用更小的模型架构

### Q: 模型加载失败？

A: 检查以下项：
- Opset 版本是否支持
- 使用的操作符是否支持
- 模型文件是否损坏
- 输入输出定义是否正确

### Q: 推理结果不正确？

A: 检查以下项：
- 输入数据预处理是否正确
- 输入形状是否匹配
- 数据类型是否正确
- 模型是否与预期一致

## 参考资源

- [ONNX 官方文档](https://onnx.ai/)
- [ONNX 规范](https://github.com/onnx/onnx/blob/main/docs/IR.md)
- [ONNX Runtime 文档](https://onnxruntime.ai/)
- [onnxruntime_go GitHub](https://github.com/yalue/onnxruntime_go)
- [ONNX Model Zoo](https://github.com/onnx/models)

---

**最后更新**: 2024-11-12

