# WES ONNX 模型示例库

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中 ONNX 模型示例库相关功能

---

## 📍 组件定位

**ONNX 模型资源级示例库** - 为 WES 平台提供标准化的 ONNX 模型可执行资源，用于功能验证和开发测试。本目录包含来自 [onnxruntime_go](https://github.com/yalue/onnxruntime_go) 的测试模型。

**与其他目录的关系**：
- `models/examples/`：**模型资源级示例**，单个 ONNX 模型作为可执行资源
- `contracts/examples/`：**合约资源级示例**，单个 WASM 合约作为可执行资源（结构与本目录对齐）
- `examples/`（仓库根）：**场景级示例**，组合使用模型、合约等多种资源

## 模型来源

### onnxruntime_go 测试模型

**来源**: [yalue/onnxruntime_go](https://github.com/yalue/onnxruntime_go)

**特点**:
- 用于测试和验证 ONNX Runtime 功能
- 包含基本功能和边缘情况测试
- 模型小巧，适合快速测试
- 真实模型文件，可直接使用

## 目录结构

```
models/examples/
├── basic/                            # 基本功能测试模型
│   ├── sklearn_randomforest/        # 随机森林分类器
│   │   ├── sklearn_randomforest.onnx
│   │   ├── generate_sklearn_network.py
│   │   └── README.md
│   ├── several_inputs_outputs/       # 多输入输出示例
│   │   ├── example_several_inputs_and_outputs.onnx
│   │   ├── generate_several_inputs_and_outputs.py
│   │   └── README.md
│   ├── multitype/                    # 多数据类型示例
│   │   ├── example_multitype.onnx
│   │   ├── generate_network_different_types.py
│   │   └── README.md
│   └── README.md
│
└── edge_cases/                      # 边缘情况测试模型
    ├── big_fanout/                  # 大扇出网络
    │   ├── example_big_fanout.onnx
    │   ├── generate_big_fanout.py
    │   └── README.md
    ├── big_compute/                  # 大计算量网络
    │   ├── example_big_compute.onnx
    │   ├── generate_network_big_compute.py
    │   ├── modify_metadata.py
    │   └── README.md
    ├── zero_dim_output/              # 零维输出
    │   ├── example_0_dim_output.onnx
    │   ├── generate_0_dimension_output.py
    │   └── README.md
    ├── dynamic_axes/                 # 动态轴
    │   ├── example_dynamic_axes.onnx
    │   ├── generate_dynamic_axes_network.py
    │   └── README.md
    ├── float16/                      # Float16 精度
    │   ├── example_float16.onnx
    │   ├── generate_float16_network.py
    │   └── README.md
    ├── odd_name/                     # 特殊字符文件名
    │   ├── example ż 大 김.onnx
    │   ├── generate_odd_name_onnx.py
    │   └── README.md
    └── README.md
```

## 模型列表

### 基本功能测试 (`basic/`)

每个模型都有独立的子目录，包含模型文件、生成脚本和 README 文档：

| 模型目录 | 模型文件 | 描述 | 输入/输出 |
|---------|---------|------|----------|
| `sklearn_randomforest/` | `sklearn_randomforest.onnx` | scikit-learn 随机森林分类器 | 输入: `[batch, 4]` float32<br>输出: 标签 + 概率 |
| `several_inputs_outputs/` | `example_several_inputs_and_outputs.onnx` | 多输入多输出示例 | 3个输入，2个输出 |
| `multitype/` | `example_multitype.onnx` | 多数据类型示例 | uint8 + float64 → int16 + int64 |

### 边缘情况测试 (`edge_cases/`)

每个模型都有独立的子目录，包含模型文件、生成脚本和 README 文档：

| 模型目录 | 模型文件 | 描述 | 测试场景 |
|---------|---------|------|----------|
| `big_fanout/` | `example_big_fanout.onnx` | 大扇出网络 | 100个并行矩阵乘法 |
| `big_compute/` | `example_big_compute.onnx` | 大计算量网络 | 52M元素，40次运算 |
| `zero_dim_output/` | `example_0_dim_output.onnx` | 零维输出 | 标量输出处理 |
| `dynamic_axes/` | `example_dynamic_axes.onnx` | 动态轴 | 可变批次大小 |
| `float16/` | `example_float16.onnx` | Float16 精度 | 半精度浮点 |
| `odd_name/` | `example ż 大 김.onnx` | 特殊字符文件名 | Unicode 文件名 |

## 模型统计

| 分类 | 模型数量 | 来源 | 说明 |
|------|---------|------|------|
| **基本功能测试** | 3 | onnxruntime_go | ✅ 已包含真实模型文件 |
| **边缘情况测试** | 6 | onnxruntime_go | ✅ 已包含真实模型文件 |
| **总计** | **9** | - | - |

## 使用指南

### 1. 部署模型

使用 WES CLI 部署模型到区块链：

```bash
# 部署基本功能测试模型
wes ai deploy models/examples/basic/sklearn_randomforest/sklearn_randomforest.onnx \
    --name "Random Forest Classifier" \
    --description "Test model from onnxruntime_go"

# 部署边缘情况测试模型
wes ai deploy models/examples/edge_cases/big_fanout/example_big_fanout.onnx \
    --name "Big Fanout Test" \
    --description "Test model for large fanout networks"
```

### 重新生成模型

每个模型目录都包含生成脚本，可以重新生成模型：

```bash
# 重新生成随机森林模型
cd models/examples/basic/sklearn_randomforest
python generate_sklearn_network.py

# 重新生成大扇出模型
cd models/examples/edge_cases/big_fanout
python generate_big_fanout.py
```

#### 📝 关于脚本注释的说明

**重要提示**：本目录中的所有 Python 生成脚本都经过了注释增强，与原始版本可能有所不同：

1. **格式规范**：
   - ✅ 所有脚本已按照 **PEP 257** 规范修正了 docstring 格式
   - ✅ 类和方法的第一行 docstring 以句号结尾，符合 Python 标准规范
   - ✅ 这是为了符合 Python 语言规范，而非原始代码的错误

2. **注释增强**：
   - ✅ **保留原有注释**：所有原始英文注释都完整保留
   - ✅ **添加中文注释**：增加了详细的中文 docstring 和行内注释
   - ✅ **双语支持**：提供英文（原始）+ 中文（补充）的双语注释

3. **注释目的**：
   - 📚 **学习价值**：帮助开发者理解 ONNX 模型生成的流程和概念
   - 🔗 **WES 关联**：说明模型设计与 WES 平台测试场景的关联
   - 💡 **技术细节**：解释类型转换、ONNX 导出参数等关键技术点

4. **功能一致性**：
   - ✅ **功能完全一致**：脚本的功能逻辑与原始版本完全相同
   - ✅ **输出一致**：生成的 ONNX 模型文件与原始版本完全一致
   - ✅ **仅注释增强**：只增加了注释，没有修改任何功能代码

**如果您需要查看原始版本的脚本**，请访问：
- [onnxruntime_go 原始仓库](https://github.com/yalue/onnxruntime_go)

### 2. 调用模型

使用 JSON-RPC API 调用模型：

```bash
# 调用模型进行推理
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_callAIModel",
    "params": {
      "private_key": "your_private_key",
      "model_hash": "0x...",
      "inputs": [
        {
          "name": "input",
          "data": [1.0, 2.0, 3.0, 4.0],
          "shape": [1, 4],
          "data_type": "float32"
        }
      ]
    },
    "id": 1
  }'
```

### 3. 使用 CLI

```bash
# 调用模型
wes ai call <model-hash> \
    --inputs '[[1.0, 2.0, 3.0, 4.0]]' \
    --private-key <your_private_key>
```

## ONNX Model Zoo 模型

虽然本目录不包含 ONNX Model Zoo 的模型文件（因为需要使用 Git LFS），但我们提供以下分类的文档说明，帮助您了解可用的模型类型：

### 计算机视觉模型

**来源**: [ONNX Model Zoo - Computer Vision](https://github.com/onnx/models/tree/main/validated/vision)

**应用场景**:
- 图像分类（ResNet、MobileNet、EfficientNet、Vision Transformer 等）
- 目标检测（YOLO、Faster R-CNN 等）
- 语义分割
- 人脸识别
- 人体姿态估计

**获取方式**:
```bash
# 从 GitHub 克隆并拉取 LFS 文件
git clone https://github.com/onnx/models.git
cd models
git lfs pull
```

### 自然语言处理模型

**来源**: [ONNX Model Zoo - NLP](https://github.com/onnx/models/tree/main/validated/text)

**应用场景**:
- 文本分类（BERT、RoBERTa、DistilBERT 等）
- 情感分析
- 机器翻译（T5、mT5 等）
- 问答系统
- 命名实体识别

**典型模型**: BERT、RoBERTa、T5、GPT 系列

### 生成式 AI 模型

**来源**: [ONNX Model Zoo - Generative AI](https://github.com/onnx/models/tree/main/Generative_AI)

**应用场景**:
- 文本生成（GPT-NeoX、GPT-2 等）
- 对话生成
- 代码生成
- 创意写作

### 图机器学习模型

**来源**: [ONNX Model Zoo - Graph ML](https://github.com/onnx/models/tree/main/Graph_Machine_Learning)

**应用场景**:
- 节点分类（GraphSAGE、TAGConv 等）
- 图分类
- 链接预测
- 推荐系统

**典型模型**: GraphSAGE、TAGConv、FEASTConv、GCN

## 测试场景

### 基本功能验证

使用 `basic/` 目录中的模型验证：
- ✅ 模型加载
- ✅ 输入输出处理
- ✅ 推理执行
- ✅ 结果返回

### 边缘情况测试

使用 `edge_cases/` 目录中的模型测试：
- ✅ 特殊网络结构
- ✅ 不同数据类型
- ✅ 动态形状处理
- ✅ 文件名编码

## 兼容性说明

### 当前 WES 平台支持

**✅ 已支持**:
- Float32 数据类型
- 多维张量输入（P0 改进后）
- Int64 数据类型（P2 改进后）
- Uint8 数据类型（P2 改进后）

**⚠️ 限制**:
- 部分复杂模型可能需要特定的输入预处理
- 某些模型需要特定的 ONNX Runtime 版本

### 推荐使用的模型

**初学者**:
- `basic/sklearn_randomforest.onnx` - 简单易用
- `basic/example_several_inputs_and_outputs.onnx` - 多输入输出示例

**进阶用户**:
- `edge_cases/example_big_fanout.onnx` - 测试复杂网络结构
- `edge_cases/example_dynamic_axes.onnx` - 测试动态形状

## 参考资源

### 原始仓库

1. **onnxruntime_go**
   - GitHub: https://github.com/yalue/onnxruntime_go
   - 文档: https://pkg.go.dev/github.com/yalue/onnxruntime_go
   - 示例: https://github.com/yalue/onnxruntime_go_examples

2. **ONNX Model Zoo**
   - GitHub: https://github.com/onnx/models
   - 文档: https://github.com/onnx/models#readme
   - 模型列表: https://github.com/onnx/models/tree/main/validated

### WES 平台文档

- [ONNX 引擎文档](../../../docs/system/core/onnx_engine.md)
- [API 文档](../../../docs/api/jsonrpc.md)
- [CLI 文档](../../../docs/cli/ai.md)
- [模型格式规范](../docs/model_format.md)
- [测试指南](../docs/testing_guide.md)
- [部署指南](../docs/deployment_guide.md)

## 贡献指南

### 添加新模型

1. 确保模型来自官方来源（onnxruntime_go）
2. 按照功能分类放入 `basic/` 或 `edge_cases/` 目录
3. 更新本 README 的模型统计
4. 添加模型使用说明（如适用）

### 报告问题

如果发现模型问题：
1. 检查原始仓库的 issue
2. 在 WES 项目仓库创建 issue
3. 提供模型路径和错误信息

## 许可证

本目录中的模型遵循其原始来源的许可证：

- **onnxruntime_go 模型**: MIT License

使用前请查看原始仓库了解具体许可证信息。

