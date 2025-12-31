# 模型文档库

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-12
- **最后审核**：2025-11-12
- **所有者**：AI模型管理组
- **适用范围**：WES 项目中模型文档库相关功能

---

## 📍 组件定位

**模型文档库** - 为 WES 平台的 ONNX 模型使用提供完整的文档支持，包括格式规范、测试指南、部署指南等。

## 文档列表

### 1. [模型格式规范](model_format.md)

**内容**:
- ONNX 标准说明
- 支持的输入输出格式
- 数据类型映射
- Opset 版本要求
- 模型验证方法

**适用对象**: 模型开发者、平台使用者

### 2. [部署指南](deployment_guide.md)

**内容**:
- 部署方式（CLI/API）
- 部署流程
- 最佳实践
- 注意事项
- 故障排查

**适用对象**: 模型开发者、运维人员

---

## 📝 测试文档说明

**重要提示**：测试相关的文档已重组，请参考以下位置：

- ✅ **测试脚本和指南**：`scripts/testing/models/README.md` - 完整的 ONNX 模型测试指南
- ✅ **测试脚本总入口**：`scripts/testing/README.md` - 测试脚本目录总览
- ⚠️ **历史测试文档**：`docs/analysis/testing/ARCHIVED_*.md` - 已归档的历史测试指南（仅供参考）

**文档重组说明**：
- 测试指南已整合到 `scripts/testing/models/README.md`，与测试脚本在同一目录
- 历史测试文档已归档到 `docs/analysis/testing/` 目录
- `models/docs/` 目录现在只保留技术规范和使用指南

## 快速开始

### 新用户

1. 阅读 [模型格式规范](model_format.md) 了解模型要求
2. 查看 [测试指南](../../scripts/testing/models/README.md) 学习如何测试模型
3. 参考 [部署指南](deployment_guide.md) 部署模型

### 模型开发者

1. 确保模型符合 [模型格式规范](model_format.md)
2. 使用 [测试指南](../../scripts/testing/models/README.md) 验证模型
3. 按照 [部署指南](deployment_guide.md) 部署模型

## 相关资源

- [模型示例库](../examples/README.md)
- [测试模型](../examples/test/README.md)
- [WES ONNX 引擎文档](../../../docs/system/core/onnx_engine.md)
- [WES API 文档](../../../docs/api/jsonrpc.md)


