# WES 日志架构和使用指南

## 📋 概述

WES 采用**多文件日志架构**，将日志按职责拆分为多个文件，提高可读性和可维护性。

### 🎯 核心特性

1. **统一目录策略**：每个环境只有一个日志根目录 `{data_path}/logs/`
2. **多文件分离**：系统日志和业务日志分离，避免噪声干扰
3. **结构化日志**：支持基于 `module` 字段的日志路由和过滤
4. **环境隔离**：开发、测试、生产环境完全隔离

## 📁 日志文件结构

### 多文件模式（默认启用）

```
{data_path}/logs/
├── node-system.log      # 系统日志（P2P、共识、存储等基础设施）
└── node-business.log    # 业务日志（API、合约执行、Workbench交互等）
```

### 单文件模式（向后兼容）

```
{data_path}/logs/
└── weisyn.log           # 所有日志（兼容旧配置）
```

## 🔧 配置说明

### 日志路径自动推导

日志路径基于 `storage.data_path` 自动推导，**不再需要显式配置 `log.file_path`**：

```json
{
  "storage": {
    "data_path": "./data/testing"
  },
  "log": {
    "level": "info"
  }
}
```

**结果**：日志文件自动创建在 `./data/testing/logs/` 目录下

### 多文件日志配置

多文件日志默认启用，可以通过配置禁用（回退到单文件模式）：

```json
{
  "log": {
    "level": "info",
    "enable_multi_file": true,      // 默认 true
    "system_log_file": "node-system.log",    // 默认值
    "business_log_file": "node-business.log" // 默认值
  }
}
```

## 📊 模块分类

### 系统模块（写入 `node-system.log`）

- `p2p` - P2P 网络连接和发现
- `consensus` - 共识算法和区块生成
- `storage` - 存储子系统（BadgerDB、文件存储等）
- `persistence` - 持久化查询服务
- `network` - 网络层（GossipSub、消息路由）
- `chain` - 链状态管理和同步
- `block` - 区块构建、验证和处理
- `event` - 事件总线
- `kademlia` - Kademlia 路由表
- `compliance` - 合规服务
- `crypto` - 加密模块
- `sync` - 区块同步（兼容旧代码）
- `infra` / `system` - 基础设施模块（通用）

### 业务模块（写入 `node-business.log`）

- `api` - HTTP/JSON-RPC/gRPC API
- `executor` - 合约执行器（ISPC）
- `tx` - 交易处理
- `mempool` - 内存池（交易池和候选区块池）
- `ures` - URES 资源存储
- `eutxo` - EUTXO 模型
- `contract` - 智能合约相关（兼容旧代码）
- `workbench` - Workbench 交互（兼容旧代码）
- `business` / `app` - 业务逻辑模块（通用）

## 🛠️ 日志查看工具

### 1. 查看业务日志

```bash
# 查看测试环境的业务日志
./scripts/logs/tail_business.sh test

# 查看开发环境的业务日志
./scripts/logs/tail_business.sh dev

# 使用环境变量指定日志目录
WES_LOG_DIR=/path/to/logs ./scripts/logs/tail_business.sh
```

### 2. 查看系统日志

```bash
# 查看测试环境的系统日志
./scripts/logs/tail_system.sh test

# 查看开发环境的系统日志
./scripts/logs/tail_system.sh dev
```

### 3. 按模块过滤日志

```bash
# 实时查看 API 模块日志
./scripts/logs/grep_module.sh -m api -F

# 查看 P2P 模块日志（开发环境）
./scripts/logs/grep_module.sh -m p2p -e dev

# 查看合约模块日志
./scripts/logs/grep_module.sh -m contract -F

# 从指定文件查看
./scripts/logs/grep_module.sh -m api -f /path/to/log
```

## 💻 代码中使用

### 为模块添加 module 字段

在模块的 `Module()` 函数中，使用 `With("module", "<module_name>")` 创建带模块标识的 logger：

```go
// 示例：API 模块
func Module() fx.Option {
    return fx.Module("api",
        fx.Provide(func(logger logInterface.Logger) *zap.Logger {
            // 为 API 模块添加 module 字段
            apiLogger := logger.With("module", "api")
            return apiLogger.GetZapLogger()
        }),
        // ... 其他提供者
    )
}
```

### 在代码中记录日志

```go
// 使用带 module 字段的 logger
logger.Info("处理请求", "request_id", reqID, "method", method)

// 日志会自动路由到对应的文件：
// - module=api → node-business.log
// - module=p2p → node-system.log
```

## 🔍 日志格式

日志采用 JSON 格式，便于解析和过滤：

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "info",
  "module": "api",
  "message": "处理请求",
  "request_id": "abc123",
  "method": "POST",
  "path": "/api/v1/tx",
  "caller": "api/handler.go:42"
}
```

## 📝 最佳实践

### 1. 开发/测试时

- 使用 `tail_business.sh` 查看业务日志，专注于 API 和合约执行
- 需要排查网络问题时，使用 `tail_system.sh` 查看系统日志
- 使用 `grep_module.sh` 快速定位特定模块的问题

### 2. 生产环境

- 定期归档和清理日志文件
- 使用日志聚合工具（如 ELK、Loki）集中管理
- 监控日志文件大小，确保轮转正常工作

### 3. 调试时

- 临时提高日志级别：`"log": { "level": "debug" }`
- 使用 `grep_module.sh` 过滤特定模块的日志
- 结合 `jq` 工具格式化 JSON 日志

## ⚠️ 注意事项

1. **单一日志目录**：每个环境只有一个日志根目录，确保日志集中管理
2. **模块字段**：建议所有模块都添加 `module` 字段，便于日志路由和过滤
3. **向后兼容**：如果禁用多文件日志，会回退到单文件模式（`weisyn.log`）
4. **路径推导**：日志路径基于 `storage.data_path` 自动推导，无需手动配置

## 🔄 迁移指南

### 从旧配置迁移

**旧配置**（已废弃）：
```json
{
  "log": {
    "level": "info",
    "file_path": "./data/logs/development.log"
  }
}
```

**新配置**（推荐）：
```json
{
  "storage": {
    "data_path": "./data/development/single"
  },
  "log": {
    "level": "info"
  }
}
```

日志会自动创建在 `./data/development/single/logs/` 目录下。

## 📚 相关文档

- [数据架构文档](../../docs/system/designs/storage/data-architecture.md)
- [日志配置文档](../../internal/config/log/README.md)

