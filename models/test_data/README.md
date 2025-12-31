# 测试模型库（test_data）

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中测试模型库相关功能

---

## 📍 组件定位

本目录包含用于测试和验证ONNX引擎功能的测试模型。

## 📋 模型列表

### 当前状态

**待添加模型**：
- ⏳ `simple_linear.onnx` - 简单线性回归模型（`y = 2x + 1`）
- ⏳ `simple_classifier.onnx` - 简单二分类模型

## 🎯 模型要求

### 兼容性要求

**当前实现支持**（P0改进后）：
- ✅ 输入形状：`[batch, features]` 或 `[batch, channels, height, width]`
- ✅ 输入类型：float32
- ✅ 输出类型：float32

**待P1改进后支持**：
- ⚠️ 输入类型：int64（文本模型）

### 模型大小要求

- 测试模型应尽量小（< 100KB）
- 大型模型使用Git LFS管理
- 在模型README中标注模型大小

## 📝 模型文档格式

每个模型应包含：

```markdown
# 模型名称

## 基本信息
- **文件名**：model.onnx
- **大小**：XX KB
- **来源**：来源说明
- **许可证**：许可证信息

## 输入输出
- **输入形状**：[batch, features]
- **输入类型**：float32
- **输出形状**：[batch, 1]
- **输出类型**：float32

## 使用示例
```go
inputs := [][]float64{{1.0, 2.0, 3.0}}
outputs, err := engine.CallModel(ctx, modelHash, inputs)
```

## 预期结果
输入：[1.0, 2.0, 3.0]
输出：[3.0, 5.0, 7.0]（对于 y = 2x + 1）
```

## 🔍 模型获取

### 方案1：自己生成（推荐用于简单模型）

使用Python生成简单线性回归模型：

```python
import torch
import torch.onnx

class SimpleLinear(torch.nn.Module):
    def __init__(self):
        super().__init__()
        self.linear = torch.nn.Linear(1, 1)
        self.linear.weight.data.fill_(2.0)
        self.linear.bias.data.fill_(1.0)
    
    def forward(self, x):
        return self.linear(x)

model = SimpleLinear()
dummy_input = torch.tensor([[1.0], [2.0], [3.0]])
torch.onnx.export(model, dummy_input, "simple_linear.onnx")
```

### 方案2：从公开库获取

- ONNX Model Zoo：https://github.com/onnx/models
- Hugging Face：https://huggingface.co/models?library=onnx

**注意**：当前只选择简单的2D输入模型

## ✅ 验证清单

添加模型前请确认：

- [ ] 模型格式正确（`.onnx`文件）
- [ ] 模型大小合理（< 100KB）
- [ ] 输入输出格式明确
- [ ] 提供使用文档
- [ ] 记录模型来源和许可证
- [ ] 测试模型可以正常运行


