# ONNX 模型完整测试指南

## 📋 概述

本文档提供在 WES 平台上测试 ONNX 模型的完整指南，包括环境准备、节点启动、共识配置、模型部署和调用等所有步骤。**遵循此文档可以避免在其他电脑上反复调试如何启动节点/共识/部署/调用等问题**。

---

## 📝 临时文档管理

**重要提示**：测试过程中产生的临时文档（分析、总结、修复跟踪等）应统一管理：

- ✅ **存放位置**：`docs/analysis/testing/`
- ✅ **命名规范**：`{类型}_{主题}_{日期}.md`（如：`ANALYSIS_ONNX_ERROR_20251113.md`）
- ✅ **生命周期**：根据文档类型设置保留期限（30-90天），定期清理过期文档
- ❌ **禁止行为**：不要在 `models/docs/` 或 `scripts/testing/` 中创建临时文档

**详细规范**：请参考 [`scripts/testing/README.md`](../../scripts/testing/README.md#临时文档管理规范) 和 [`docs/analysis/testing/README.md`](../../docs/analysis/testing/README.md)

---

## 🎯 快速开始（5分钟）

### 前置要求

1. **已构建项目**
   ```bash
   make build-dev
   # 或
   make build-test
   ```

2. **测试配置文件存在**
   - `configs/testing/config.json` ✅ 已包含单节点共识配置

3. **依赖工具**
   - `curl` - API调用
   - `base64` - 模型编码
   - `jq` (可选) - JSON解析增强

### 一键测试

```bash
# 进入项目根目录
cd /Users/qinglong/go/src/chaincodes/WES/weisyn.git

# 运行测试脚本（自动处理节点启动、共识配置、部署、调用）
bash scripts/testing/models/onnx_models_test.sh
```

---

## 📚 详细说明

### 1. 单节点共识模式配置

#### 1.1 为什么需要单节点共识？

在测试环境中，**必须使用单节点共识模式**，原因如下：

- ✅ **避免网络等待**：多节点共识需要等待其他节点响应，测试时会超时
- ✅ **快速出块**：单节点模式下区块立即本地确认，无需等待网络共识
- ✅ **简化测试**：无需配置多个节点，适合开发和测试环境

#### 1.2 配置位置

配置文件：`configs/testing/config.json`

```json
{
  "mining": {
    "_comment_consensus_mode": "⚠️ 单节点开发模式已启用 - 仅用于开发/测试，禁止用于生产",
    "_warning": "enable_aggregator=false 表示单节点模式：区块立即本地确认，无分布式共识保障",
    "_production_requirement": "生产环境必须设置 enable_aggregator=true",
    "target_block_time": "15s",
    "enable_aggregator": false,  // ⚠️ 关键配置：false = 单节点模式
    "max_mining_threads": 8
  }
}
```

#### 1.3 配置说明

| 配置项 | 值 | 说明 |
|--------|-----|------|
| `enable_aggregator` | `false` | 单节点模式：区块立即本地确认 |
| `target_block_time` | `"15s"` | 目标区块生成时间（单节点模式下实际可能更快） |
| `max_mining_threads` | `8` | 最大挖矿线程数 |

#### 1.4 验证配置

```bash
# 检查配置是否正确
grep -A 3 "enable_aggregator" configs/testing/config.json

# 应该看到：
# "enable_aggregator": false,
```

---

### 2. 节点启动

#### 2.1 自动启动（推荐）

测试脚本会自动检测节点状态，如果未运行则自动启动：

```bash
bash scripts/testing/models/onnx_models_test.sh
```

脚本会自动：
1. ✅ 检查节点是否运行（检查端口 28680）
2. ✅ 如果未运行，自动启动测试节点
3. ✅ 等待节点就绪（最多 60 秒）
4. ✅ 验证节点健康状态

#### 2.2 手动启动

如果需要手动启动节点：

```bash
# 方式1：使用测试二进制文件
./bin/testing --api-only

# 方式2：使用开发二进制文件
./bin/development --config configs/testing/config.json --api-only

# 方式3：使用 go run（开发环境）
go run ./cmd/weisyn --api-only --env testing
```

#### 2.3 验证节点运行

```bash
# 检查节点健康状态
curl http://localhost:28680/api/v1/health/live

# 检查 JSON-RPC 端点
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"wes_blockNumber","params":[],"id":1}'
```

---

### 3. 模型部署

#### 3.1 使用测试脚本（推荐）

测试脚本会自动处理模型部署：

```bash
# 测试所有模型
bash scripts/testing/models/onnx_models_test.sh

# 测试单个模型
bash scripts/testing/models/onnx_models_test.sh sklearn_randomforest
```

#### 3.2 手动部署（JSON-RPC API）

```bash
# 1. 读取模型文件并 Base64 编码
MODEL_BASE64=$(base64 -i models/examples/basic/sklearn_randomforest/sklearn_randomforest.onnx)

# 2. 调用部署 API
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"wes_deployAIModel\",
    \"params\": {
      \"private_key\": \"0xae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323\",
      \"onnx_content\": \"${MODEL_BASE64}\",
      \"name\": \"Random Forest Test\",
      \"description\": \"Test model for validation\"
    },
    \"id\": 1
  }" | jq .

# 3. 获取模型哈希（从响应中提取）
MODEL_HASH=$(curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"wes_deployAIModel\",
    \"params\": {
      \"private_key\": \"0xae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323\",
      \"onnx_content\": \"${MODEL_BASE64}\",
      \"name\": \"Random Forest Test\",
      \"description\": \"Test model\"
    },
    \"id\": 1
  }" | jq -r '.result.content_hash')
```

#### 3.3 等待交易确认

在单节点模式下，交易会很快确认（通常 < 5 秒）：

```bash
# 获取交易哈希（从部署响应中提取）
TX_HASH="<transaction_hash>"

# 等待交易确认
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"wes_getTransactionReceipt\",
    \"params\": [\"${TX_HASH}\"],
    \"id\": 1
  }" | jq .
```

---

### 4. 模型调用

#### 4.1 使用测试脚本（推荐）

测试脚本会自动处理模型调用：

```bash
bash scripts/testing/models/onnx_models_test.sh sklearn_randomforest
```

#### 4.2 手动调用（JSON-RPC API）

```bash
# 调用模型
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"wes_callAIModel\",
    \"params\": {
      \"private_key\": \"0xae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323\",
      \"model_hash\": \"${MODEL_HASH}\",
      \"inputs\": [{
        \"name\": \"X\",
        \"data\": [5.1, 3.5, 1.4, 0.2],
        \"shape\": [1, 4],
        \"data_type\": \"float32\"
      }]
    },
    \"id\": 1
  }" | jq .
```

#### 4.3 输入格式说明

根据 [onnxruntime_go](https://github.com/yalue/onnxruntime_go) 的标准，输入格式如下：

**基本格式**：
```json
{
  "name": "input_name",
  "data": [1.0, 2.0, 3.0],
  "shape": [1, 3],
  "data_type": "float32"
}
```

**支持的数据类型**：
- `float32` - 使用 `data` 字段
- `float64` - 使用 `data` 字段
- `int32` - 使用 `int32_data` 字段
- `int64` - 使用 `int64_data` 字段
- `uint8` - 使用 `uint8_data` 字段

**示例：int32 输入**：
```json
{
  "name": "input",
  "int32_data": [1, 2, 3],
  "shape": [1, 3],
  "data_type": "int32"
}
```

---

### 5. 测试流程详解

#### 5.1 完整测试流程

测试脚本执行以下步骤：

1. **环境检查**
   - ✅ 检查依赖工具（curl, jq, base64）
   - ✅ 检查节点状态
   - ✅ 如果节点未运行，自动启动

2. **查找模型**
   - ✅ 扫描 `models/examples/` 目录
   - ✅ 查找所有 `.onnx` 文件

3. **对每个模型执行**：
   - **步骤 1/3: 部署模型**
     - 读取 ONNX 文件
     - Base64 编码
     - 调用 `wes_deployAIModel` API
     - 获取模型哈希和交易哈希
   
   - **步骤 2/3: 等待确认**
     - 在单节点模式下，主动触发区块生成
     - 等待交易确认（最多 120 秒）
     - 等待模型资源可用（最多 60 秒）
   
   - **步骤 3/3: 调用模型**
     - 根据模型类型准备测试输入
     - 调用 `wes_callAIModel` API
     - 验证输出结果

4. **生成测试报告**
   - ✅ 统计总模型数
   - ✅ 统计通过/失败数量
   - ✅ 显示最终结果

#### 5.2 单节点模式特殊处理

测试脚本在单节点模式下会：

1. **主动触发区块生成**：
   ```bash
   # 启动挖矿
   wes_startMining <miner_address>
   
   # 等待区块生成
   # 检查区块高度变化
   
   # 停止挖矿
   wes_stopMining
   ```

2. **快速确认**：
   - 单节点模式下，区块立即本地确认
   - 无需等待网络共识
   - 交易确认时间 < 5 秒

---

### 6. 常见问题排查

#### 6.1 节点启动失败

**症状**：
```
❌ 节点启动超时
```

**解决方法**：
1. 检查端口是否被占用：
   ```bash
   lsof -i :28680
   ```

2. 检查配置文件是否正确：
   ```bash
   cat configs/testing/config.json | jq .mining.enable_aggregator
   # 应该输出: false
   ```

3. 检查日志：
   ```bash
   tail -50 data/testing/onnx_test_logs/node.log
   ```

#### 6.2 交易确认超时

**症状**：
```
⚠️ 交易确认超时（等待了 120 秒）
```

**解决方法**：
1. 确认单节点模式已启用：
   ```bash
   grep "enable_aggregator" configs/testing/config.json
   ```

2. 手动触发区块生成：
   ```bash
   curl -X POST http://localhost:28680/jsonrpc \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "method": "wes_startMining",
       "params": ["CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR"],
       "id": 1
     }'
   ```

#### 6.3 模型调用失败

**症状**：
```
❌ 模型调用失败: Internal error
```

**解决方法**：
1. 检查模型是否已部署：
   ```bash
   curl -X POST http://localhost:28680/jsonrpc \
     -H "Content-Type: application/json" \
     -d "{
       \"jsonrpc\": \"2.0\",
       \"method\": \"wes_getTransactionReceipt\",
       \"params\": [\"${TX_HASH}\"],
       \"id\": 1
     }" | jq .
   ```

2. 检查输入格式是否正确：
   - 确认输入名称匹配模型定义
   - 确认输入形状正确
   - 确认数据类型正确

3. 查看详细错误信息：
   ```bash
   tail -100 data/testing/onnx_test_logs/node.log | grep -i error
   ```

---

### 7. 测试账户

测试脚本使用以下测试账户：

```json
{
  "private_key": "ae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323",
  "address": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
  "initial_balance": "100000000000000000"
}
```

这些账户在 `configs/testing/config.json` 的 `genesis.accounts` 中预配置。

---

### 8. 测试报告

测试报告保存在：`data/testing/onnx_test_logs/`

报告格式：
```
test_report_YYYYMMDD_HHMMSS.txt
```

报告内容：
- ✅ 环境检查结果
- ✅ 每个模型的测试过程
- ✅ 最终统计（通过/失败/跳过）

查看最新报告：
```bash
ls -t data/testing/onnx_test_logs/test_report_*.txt | head -1 | xargs cat
```

---

### 9. 参考资源

- **测试脚本**: `scripts/testing/models/onnx_models_test.sh`
- **测试配置**: `configs/testing/config.json`
- **模型目录**: `models/examples/`
- **onnxruntime_go 文档**: https://github.com/yalue/onnxruntime_go
- **ONNX 模型测试指南**: `models/docs/testing_guide.md`

---

## ✅ 检查清单

在开始测试前，确认以下项：

- [ ] 项目已构建：`make build-dev` 或 `make build-test`
- [ ] 配置文件存在：`configs/testing/config.json`
- [ ] 单节点模式已启用：`enable_aggregator: false`
- [ ] 依赖工具已安装：`curl`, `base64`, `jq`
- [ ] 端口 28680 未被占用
- [ ] 测试账户已配置（在 genesis 中）

---

**最后更新**: 2025-11-14  
**文档版本**: v2.0  
**维护者**: WES 开发团队

