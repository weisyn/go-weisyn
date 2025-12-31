# 🔐 计算创世哈希工具 (Calculate Genesis Hash)

> **工具功能**: 从链配置文件计算确定性的创世区块哈希（genesis_hash）

## 📋 快速开始

```bash
# 计算单个配置文件的创世哈希
go run ./cmd/tools/calculate-genesis-hash/main.go configs/chains/test-public-demo.json

# 计算多个配置文件的创世哈希
go run ./cmd/tools/calculate-genesis-hash/main.go configs/chains/*.json
```

## 功能说明

`calculate-genesis-hash` 工具用于从链配置文件中计算确定性的创世区块哈希，这是**链身份（ChainIdentity）**的核心组成部分。

### 主要特性

1. **确定性计算**: 基于 `network_id`、`chain_id`、`timestamp`、`genesis_accounts` 的规范化序列化计算 SHA256 哈希
2. **配置验证**: 自动解析和验证配置文件格式
3. **输出友好**: 提供清晰的输出格式，包含配置摘要和计算得到的哈希值

### 计算策略

创世哈希的计算基于以下字段的规范化序列化：

- `network_id`（字符串）
- `chain_id`（uint64）
- `timestamp`（int64，Unix 时间戳）
- `genesis_accounts`（数组，按 `public_key` 排序）

**计算步骤**：

1. 构建规范化结构（只包含影响创世状态的关键字段）
2. 对账户列表按 `public_key` 排序（确保确定性）
3. JSON 序列化（使用 sorted keys）
4. SHA256 哈希
5. 返回十六进制字符串（64字符）

## 使用方法

### 基本用法

```bash
# 使用 go run（推荐用于开发验证）
go run ./cmd/tools/calculate-genesis-hash/main.go <config-file>

# 先编译再运行（推荐用于生产环境）
go build -o bin/wes-calculate-genesis-hash ./cmd/tools/calculate-genesis-hash
./bin/wes-calculate-genesis-hash <config-file>
```

### 参数说明

| 参数 | 说明 | 必需 |
|------|------|------|
| `<config-file>` | 链配置文件路径（JSON 格式） | ✅ |

### 使用示例

```bash
# 计算测试网公链的创世哈希
go run ./cmd/tools/calculate-genesis-hash/main.go configs/chains/test-public-demo.json

# 输出示例：
# 配置文件: configs/chains/test-public-demo.json
# 链ID: 12001
# 网络ID: WES_public_testnet_demo_2025
# 创世时间戳: 1704067200
# 创世账户数: 2
#
# 计算得到的 genesis_hash: 1151369864ab748d449b3b51d2791e026aa44486d09de9a79d01bb875463ac95
#
# 请在配置文件的 genesis 段添加:
#   "expected_genesis_hash": "1151369864ab748d449b3b51d2791e026aa44486d09de9a79d01bb875463ac95"
```

## 输出说明

工具输出包含以下信息：

1. **配置文件路径**: 正在处理的配置文件
2. **链ID**: 配置中的 `network.chain_id`
3. **网络ID**: 配置中的 `network.network_id`
4. **创世时间戳**: 配置中的 `genesis.timestamp`
5. **创世账户数**: `genesis.accounts` 数组长度
6. **计算得到的 genesis_hash**: 64字符十六进制字符串
7. **配置建议**: 提示在配置文件中添加 `expected_genesis_hash` 字段

## 配置更新

计算得到 `genesis_hash` 后，需要在配置文件的 `genesis` 段添加 `expected_genesis_hash` 字段：

```json
{
  "genesis": {
    "timestamp": 1704067200,
    "expected_genesis_hash": "1151369864ab748d449b3b51d2791e026aa44486d09de9a79d01bb875463ac95",
    "accounts": [...]
  }
}
```

**注意**：
- test/prod 环境建议必须配置 `expected_genesis_hash`
- dev 环境可省略（宽松策略）
- 启动时会校验：如果配置了 `expected_genesis_hash`，必须与计算出的 `genesis_hash` 完全匹配，否则启动失败

## 错误处理

工具会在以下情况返回错误：

- 配置文件不存在或无法读取
- 配置文件格式错误（JSON 解析失败）
- 缺少必需字段（`network.chain_id`、`network.network_id`、`genesis.timestamp`、`genesis.accounts`）
- 计算哈希时发生错误

## CI/CD 集成

工具可以集成到 CI/CD 流程中，用于验证配置文件：

```bash
# 在 CI 中验证配置文件
for config in configs/chains/*.json; do
  go run ./cmd/tools/calculate-genesis-hash/main.go "$config" || exit 1
done
```

## 相关文档

- **[链身份与节点角色策略](../../../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/11-CHAIN_IDENTITY_AND_NODE_ROLE_POLICY.md)** - 链身份约束设计文档
- **[链配置规范](../../../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/09-CHAIN_CONFIG_SPEC_V1.md)** - 链配置规范文档
- **[configs/chains/README.md](../../../../configs/chains/README.md)** - 配置文件说明

## 实现细节

**实现位置**：
- `cmd/tools/calculate-genesis-hash/main.go` - 工具入口
- `internal/config/node/chain_identity.go::CalculateGenesisHash` - 哈希计算逻辑

**依赖**：
- `github.com/weisyn/v1/pkg/types` - 配置类型定义
- `github.com/weisyn/v1/internal/config/node` - 链身份计算函数

