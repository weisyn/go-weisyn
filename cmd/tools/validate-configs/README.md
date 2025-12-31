# ✅ 配置验证工具 (Validate Configs)

> **工具功能**: 验证链配置文件是否符合规范，防止配置/文档漂移

## 📋 快速开始

```bash
# 验证单个配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/test-public-demo.json

# 验证所有配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/*.json
```

## 功能说明

`validate-configs` 工具用于验证链配置文件是否符合 WES 配置规范，确保配置的正确性和一致性。

### 主要特性

1. **全面验证**: 验证链级身份、创世配置、链模式一致性、节点角色策略矩阵等
2. **Fail-fast**: 发现错误立即返回，不继续验证其他文件
3. **清晰输出**: 提供详细的错误信息，包含字段路径和错误原因
4. **CI/CD 友好**: 返回非零退出码，便于集成到 CI/CD 流程

### 验证项

工具会验证以下配置项：

1. **链级身份验证**：
   - `chain_id` 必须在对应范围内（公有链：1-9999，联盟链：20000-29999，私有链：10000-19999）
   - `network_namespace` 不能为空
   - `chain_mode` 必须与命令行参数匹配

2. **创世配置验证**：
   - `genesis.timestamp` 必须 > 0
   - `genesis.accounts` 至少包含一个账户
   - 每个账户必须包含 `address` 和 `initial_balance`
   - **链身份哈希验证**：如果配置了 `expected_genesis_hash`，必须与计算出的 `genesis_hash` 完全匹配

3. **链模式一致性验证**：
   - `network.chain_mode` 必须与 `security.permission_model` 一致
   - `security.access_control.mode` 必须符合链模式约束
   - `node.host.gater.mode` 必须符合链模式约束
   - `mining.enable_aggregator` 必须符合链模式约束

4. **节点角色策略矩阵验证**：
   - `node_role`、`environment`、`sync.startup_mode` 组合必须合法
   - 如果策略要求 `RequireTrustedCheckpoint=true`，必须配置完整的 `trusted_checkpoint`

## 使用方法

### 基本用法

```bash
# 使用 go run（推荐用于开发验证）
go run ./cmd/tools/validate-configs/main.go <config-file>...

# 先编译再运行（推荐用于生产环境）
go build -o bin/wes-validate-configs ./cmd/tools/validate-configs
./bin/wes-validate-configs <config-file>...
```

### 参数说明

| 参数 | 说明 | 必需 |
|------|------|------|
| `<config-file>...` | 一个或多个链配置文件路径（JSON 格式） | ✅ |

### 使用示例

```bash
# 验证单个配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/test-public-demo.json

# 验证所有配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/*.json

# 验证多个指定文件
go run ./cmd/tools/validate-configs/main.go \
  configs/chains/test-public-demo.json \
  configs/chains/test-consortium-demo.json
```

### 输出示例

**成功输出**：
```
✅ configs/chains/test-public-demo.json: 验证通过
✅ configs/chains/test-consortium-demo.json: 验证通过
```

**失败输出**：
```
❌ configs/chains/test-public-demo.json: 配置验证失败: 配置验证失败，发现以下问题：
  1. [genesis.expected_genesis_hash] 创世哈希不匹配: 配置值=abcd1234..., 计算值=11513698... (前8位: 11513698)
  2. [node_role_policy] 节点角色策略验证失败: 禁止的节点角色/环境/启动模式组合: role=miner env=prod startup_mode=from_genesis (禁止在生产环境从创世区块启动矿工)
```

## 退出码

| 退出码 | 说明 |
|--------|------|
| 0 | 所有配置文件验证通过 |
| 1 | 至少一个配置文件验证失败 |

## CI/CD 集成

工具设计用于集成到 CI/CD 流程中，在构建检查之后、单元测试之前运行：

```bash
#!/bin/bash
# CI 配置验证检查

echo "Validating configs..."
go run ./cmd/tools/validate-configs/main.go configs/chains/*.json || exit 1

echo "Config validation passed!"
```

**GitHub Actions 示例**：

```yaml
- name: Validate configs
  run: |
    go run ./cmd/tools/validate-configs/main.go configs/chains/*.json
```

**GitLab CI 示例**：

```yaml
validate-configs:
  stage: build
  script:
    - go run ./cmd/tools/validate-configs/main.go configs/chains/*.json
```

## 验证规则详解

### 1. 链身份哈希验证

如果配置文件中存在 `genesis.expected_genesis_hash` 字段：

1. 从 `genesis` 配置计算 `genesis_hash`（基于 `network_id`、`chain_id`、`timestamp`、`accounts` 的规范化序列化）
2. 规范化字符串（全部小写、去掉 `0x` 前缀）
3. 与 `expected_genesis_hash` 严格比较
4. 不一致则验证失败

**注意**：
- test/prod 环境建议必须配置 `expected_genesis_hash`
- dev 环境可省略（宽松策略）

### 2. 节点角色策略矩阵验证

验证 `node_role`、`environment`、`sync.startup_mode` 的组合是否合法：

- 查询策略矩阵（`internal/config/policy/node_role_policy.go`）
- 如果组合不被允许（`Allow=false`），验证失败
- 如果策略要求 `RequireTrustedCheckpoint=true`：
  - 必须配置 `sync.require_trusted_checkpoint=true`
  - 必须配置完整的 `sync.trusted_checkpoint.{height, block_hash}`

**策略矩阵说明**：参见 `11-CHAIN_IDENTITY_AND_NODE_ROLE_POLICY.md`

## 错误处理

工具会在以下情况返回错误：

- 配置文件不存在或无法读取
- 配置文件格式错误（JSON 解析失败）
- 配置验证失败（见"验证项"章节）

**错误格式**：
```
❌ <config-file>: 配置验证失败: 配置验证失败，发现以下问题：
  1. [<field>] <error-message>
  2. [<field>] <error-message>
  ...
```

## 相关文档

- **[链身份与节点角色策略](../../../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/11-CHAIN_IDENTITY_AND_NODE_ROLE_POLICY.md)** - 链身份约束与节点角色策略设计文档
- **[链配置规范](../../../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/09-CHAIN_CONFIG_SPEC_V1.md)** - 链配置规范文档
- **[CI Pipeline 与检查](../../../../_dev/05-开发流程-development/04-测试与CI流程-testing-and-ci-workflow/02-CI_PIPELINE_AND_CHECKS.md)** - CI 检查项说明
- **[configs/chains/README.md](../../../../configs/chains/README.md)** - 配置文件说明

## 实现细节

**实现位置**：
- `cmd/tools/validate-configs/main.go` - 工具入口
- `internal/config/validator.go::ValidateMandatoryConfig` - 验证逻辑

**依赖**：
- `github.com/weisyn/v1/pkg/types` - 配置类型定义
- `github.com/weisyn/v1/internal/config` - 配置验证函数

## 最佳实践

1. **提交前验证**：在提交代码前运行工具验证配置文件
2. **CI 集成**：将工具集成到 CI 流程中，自动验证所有配置文件
3. **定期检查**：定期运行工具检查配置文件是否符合最新规范
4. **文档同步**：配置规范变更时，同步更新工具验证逻辑

