# ONNX 模型部署指南

## 版本信息
- **文档版本**: v1.0
- **创建日期**: 2024-11-12
- **最后更新**: 2024-11-12

## 模块定位

**ONNX 模型部署指南** - 为开发者提供在 WES 平台上部署 ONNX 模型的完整指南，包括部署流程、最佳实践、注意事项等。

## 部署方式

WES 平台支持两种方式部署 ONNX 模型：

1. **CLI 命令行** - 适合开发和测试
2. **JSON-RPC API** - 适合集成和自动化

## 使用 CLI 部署

### 基本部署命令

```bash
wes ai deploy <onnx-file> \
    --name "Model Name" \
    --description "Model description" \
    --private-key <your_private_key>
```

### 参数说明

- `onnx-file`: ONNX 模型文件路径
- `--name`: 模型名称（可选）
- `--description`: 模型描述（可选）
- `--private-key`: 部署者的私钥（必需）

### 部署示例

```bash
# 部署基本功能测试模型
wes ai deploy models/examples/basic/sklearn_randomforest.onnx \
    --name "Random Forest Classifier" \
    --description "Test model from onnxruntime_go"

# 部署边缘情况测试模型
wes ai deploy models/examples/edge_cases/example_big_fanout.onnx \
    --name "Big Fanout Test" \
    --description "Test model for large fanout networks"
```

## 使用 JSON-RPC API 部署

### 部署请求

```bash
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_deployAIModel",
    "params": {
      "private_key": "your_private_key",
      "onnx_content": "<base64_encoded_onnx_content>",
      "name": "Model Name",
      "description": "Model description"
    },
    "id": 1
  }'
```

### 请求参数

- `private_key`: 部署者的私钥（必需）
- `onnx_content`: Base64 编码的 ONNX 文件内容（必需）
- `name`: 模型名称（可选）
- `description`: 模型描述（可选）

### 响应格式

```json
{
  "jsonrpc": "2.0",
  "result": {
    "content_hash": "0x...",
    "tx_hash": "0x...",
    "success": true,
    "message": "Model deployed successfully"
  },
  "id": 1
}
```

### 响应字段

- `content_hash`: 模型内容哈希（模型 ID）
- `tx_hash`: 部署交易哈希
- `success`: 部署是否成功
- `message`: 状态消息

## 部署流程

### 1. 准备模型文件

确保模型文件：
- 格式正确（.onnx）
- 可以正常加载
- 符合 WES 平台要求

### 2. 验证模型

在部署前验证模型：

```bash
# 使用 Python 验证
python -c "import onnx; onnx.checker.check_model('model.onnx')"
```

### 3. 部署模型

使用 CLI 或 API 部署：

```bash
wes ai deploy model.onnx --private-key <key>
```

### 4. 获取模型 ID

部署成功后，保存 `content_hash`（模型 ID），用于后续调用。

### 5. 验证部署

调用模型验证部署是否成功：

```bash
wes ai call <content_hash> \
    --inputs '[[...]]' \
    --private-key <key>
```

## 模型存储

### CAS 存储

WES 平台使用内容寻址存储（CAS）存储模型：
- 模型通过内容哈希标识
- 相同内容的模型共享存储
- 自动去重和版本管理

### 模型元数据

部署时包含的元数据：
- 模型名称
- 模型描述
- 部署者地址
- 部署时间戳
- 交易哈希

## 最佳实践

### 1. 模型命名

使用描述性名称：
- ✅ `resnet50_imagenet_classifier`
- ✅ `bert_base_uncased_sentiment`
- ❌ `model1`
- ❌ `test`

### 2. 模型描述

提供详细的模型描述：
- 模型用途
- 输入输出格式
- 使用场景
- 性能指标（如适用）

### 3. 模型优化

部署前优化模型：
- 量化（如果适用）
- 图优化
- 移除不必要的节点

### 4. 版本管理

使用版本号管理模型：
- 在模型名称中包含版本
- 记录模型变更历史
- 保留旧版本用于回滚

## 注意事项

### 1. 模型大小限制

- 确保模型大小合理
- 大模型可能需要特殊处理
- 考虑使用模型压缩

### 2. 私钥安全

- ⚠️ **重要**: 不要泄露私钥
- 使用环境变量存储私钥
- 不要在代码中硬编码私钥

### 3. 网络连接

- 确保节点连接正常
- 检查网络延迟
- 处理网络错误

### 4. 交易确认

- 等待交易确认
- 检查交易状态
- 处理部署失败情况

## 部署后操作

### 1. 记录模型信息

保存以下信息：
- 模型 ID (content_hash)
- 交易哈希 (tx_hash)
- 部署时间
- 模型元数据

### 2. 测试模型

部署后立即测试：
- 使用简单输入测试
- 验证输出格式
- 检查推理结果

### 3. 文档化

记录模型使用信息：
- 输入输出格式
- 使用示例
- 注意事项

## 故障排查

### 问题 1: 部署失败

**可能原因**:
- 模型文件格式错误
- 模型文件过大
- 网络问题
- 私钥错误

**解决方法**:
- 检查模型文件
- 验证文件格式
- 检查网络连接
- 确认私钥正确

### 问题 2: 交易未确认

**可能原因**:
- 网络拥堵
- 交易费用不足
- 节点同步问题

**解决方法**:
- 等待确认
- 检查交易状态
- 重新提交交易

### 问题 3: 模型 ID 获取失败

**可能原因**:
- 部署未完成
- 响应解析错误
- API 调用失败

**解决方法**:
- 检查部署状态
- 查看响应内容
- 重新获取模型 ID

## 参考资源

- [模型格式规范](model_format.md)
- [测试指南](testing_guide.md)
- [WES API 文档](../../../docs/api/jsonrpc.md)
- [WES CLI 文档](../../../docs/cli/ai.md)

---

**最后更新**: 2024-11-12

