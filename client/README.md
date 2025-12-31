# client/ - CLI 客户端支持库

---

## 📌 版本信息

- **版本**：2.0
- **状态**：stable
- **最后更新**：2025-11-01
- **适用范围**：CLI 客户端支持库

---

## 🎯 目录定位

**路径**：`client/`

**职责**：为 `cmd/weisyn` 提供功能支持，不包含入口点

**架构定位**：CLI业务层 (L2) - 提供基础业务语义的封装，介于应用层和TX核心层之间

根据 [CLI架构规范](../_docs/architecture/CLI_ARCHITECTURE_SPECIFICATION.md)，`client/` 目录承担 **CLI业务层** 的角色：

```
L4: 应用层 (cmd/weisyn)          ← CLI 入口
    ↓
L3: SDK层 (Client SDK - 独立仓库)  ← 完整业务语义（用于DApp/钱包）
    
L2: CLI业务层 (client/core) ⭐     ← 本目录核心
    ↓                              • 基础业务封装
                                   • 简化实现
                                   • 90%场景
L1: TX核心层 (internal/core/tx)   ← 纯技术实现
    ↓
L0: 节点核心                       ← 区块链运行时
```

**核心原则**：
- ✅ **包含基础业务逻辑**: 简单转账、合约部署等基础操作
- ✅ **允许与SDK重复**: 功能可重复，但实现更简单
- ❌ **不追求完整语义**: 不实现批量操作、高级策略、复杂编排
- 🎯 **目标**: 满足90%的CLI使用场景，代码简单直接

> ⚠️ **迁移提示（重要）**
>
> - 自 2025-11 起，WES 官方 Go SDK 统一为独立仓库 `client-sdk-go`（本地路径：`sdk/client-sdk-go.git`），  
>   本目录 `client/` 仅用于 CLI 兼容，后续会逐步退役。
> - 所有新业务能力（Token / Staking / Market / Governance / Resource）的 SDK 封装，  
>   应优先在 `client-sdk-go` 中实现，并通过 `internal/api` 提供的 JSON-RPC/REST/gRPC 与节点交互。
> - 详细迁移规划参见：`client/CLIENT_MIGRATION_PLAN.md`。

---

## 📁 目录结构

```
client/
├── core/ ⭐                # CLI 核心业务层（L2）
│   ├── builder/            # 交易构建器（基础业务封装）
│   ├── transport/          # 传输层（API客户端封装）
│   ├── wallet/             # 钱包操作（本地签名）
│   ├── config/             # 配置管理
│   └── output/             # 格式化输出
│
├── launcher/               # 子进程启动器（启动本地节点）
│
└── pkg/                    # CLI 公共库（UI/工具）
    ├── config/             # CLI配置管理
    ├── transport/          # API客户端（JSON-RPC/REST）
    ├── ux/                 # 用户界面组件（pterm）
    └── wallet/             # 本地钱包管理

> 📝 **迁移说明**
>
> - `client/cli/` 目录已迁移到 `cmd/cli/`（2025-01-XX）
> - CLI 入口点现在位于 `cmd/cli/main.go`
> - `client/core/` 和 `client/pkg/` 继续作为 CLI 的支持库
```

---

## 🎯 核心模块说明

### 0. core/ - CLI 核心业务层 ⭐

**定位**: CLI 业务逻辑层（L2），提供基础业务语义封装

**包含的业务逻辑**：
- ✅ **简单转账** - 1对1转账，基础UTXO选择，简单找零逻辑
- ✅ **合约部署** - 上传WASM文件，构建部署交易
- ✅ **简单合约调用** - 基础参数编码，单个合约调用
- ✅ **基础查询** - 封装API调用，格式化输出

**不包含的业务逻辑**：
- ❌ 批量转账、复杂UTXO策略优化
- ❌ 质押/解质押、奖励领取、治理投票
- ❌ 复杂合约编排、事件监听、状态订阅
- ❌ DeFi操作、NFT市场等高级业务

**实现策略**：
- **简单直接**: 使用最简单的算法，代码易读易维护
- **满足90%场景**: 覆盖常用操作，不追求极致优化
- **快速实现**: 代码量控制在合理范围（~2000行业务逻辑）

**目录结构**：
```
core/
├── builder/        # 交易构建器
│   ├── builder.go       # Draft交易构建
│   ├── types.go         # 业务类型定义
│   └── README.md
├── transport/      # 传输层（API客户端封装）
├── wallet/         # 本地签名
├── config/         # 配置管理
└── output/         # 格式化输出
```

详见：[client/core/README.md](./core/README.md)

---

### 1. launcher/ - 子进程启动器

**用途**：CLI 可选地启动本地节点作为子进程

**功能**：
- 查找节点二进制（`bin/weisyn-node`）
- 生成临时配置文件
- 启动子进程并监控
- 健康检查（等待节点就绪）
- 优雅停机和资源清理

**使用示例**：
```go
import "github.com/weisyn/v1/client/launcher"

// 启动本地测试节点
opts := launcher.LaunchOptions{
    Env:      "testing",
    KeepData: false,
    Endpoint: "http://localhost:28680",
}

nodeProcess, err := launcher.LaunchNode(ctx, opts)
if err != nil {
    log.Fatal(err)
}

// 等待节点就绪
if err := nodeProcess.Wait(30 * time.Second); err != nil {
    log.Fatal(err)
}

// 停止节点
defer nodeProcess.Stop()
```

---

### 2. pkg/config - CLI 配置

**用途**：管理 CLI 自身的配置

**配置文件**：`~/.wes-cli/config.json`

**配置项**：
```json
{
  "node_endpoint": "http://localhost:28680/jsonrpc",
  "node_rest_url": "http://localhost:28680",
  "wallet_data_dir": "~/.wes-cli/wallets",
  "first_time_setup": true,
  "language": "zh-CN"
}
```

---

### 3. pkg/transport - API 客户端

**用途**：通过 API 与节点通信

**支持协议**：
- JSON-RPC 2.0（主要）
- REST API
- WebSocket（计划中）

**健康检查**：
```go
import "github.com/weisyn/v1/client/pkg/transport"

// 检查节点健康
err := transport.CheckNodeHealth("http://localhost:28680")

// 等待节点就绪（带重试）
err := transport.WaitForNodeReady(ctx, "http://localhost:28680", 30*time.Second)
```

---

### 4. pkg/ux - 用户界面

**用途**：基于 `pterm` 的终端 UI 组件

**功能**：
- ASCII Logo 显示
- 交互式菜单（pterm.DefaultInteractiveSelect）
- 进度条和 Spinner
- 表格显示
- 彩色输出

**业务流程**（`pkg/ux/flows/`）：
- 账户管理流程
- 转账操作流程
- 钱包管理流程
- 首次引导流程

---

### 5. pkg/wallet - 本地钱包

**用途**：本地 Keystore 管理

**功能**：
- 创建钱包
- 导入私钥
- 导出私钥
- 签名交易
- 密码管理

**存储位置**：`~/.wes-cli/wallets/`

---

## 🔌 与节点的交互

```
┌─────────┐
│ cmd/weisyn │  CLI 入口
└────┬────┘
     │
     ├──► client/launcher/      启动本地节点（可选）
     │
     ├──► client/pkg/transport/ 通过 API 与节点通信
     │         │
     │         └──► http://localhost:28680/jsonrpc
     │                     │
     │                     ▼
     │              ┌──────────────┐
     │              │ WES 节点     │
     │              │ (cmd/weisyn-node)
     │              └──────────────┘
     │
     └──► client/pkg/ux/          显示交互界面
```

---

## 🚫 非入口点目录

**重要**：`client/` 不包含 `main.go` 入口

- ✅ 所有入口在 `cmd/` 目录
- ✅ `client/` 仅提供支持库
- ✅ 通过 import 使用：`github.com/weisyn/v1/client/...`

---

## 📚 相关文档

- [CLI 入口](../cmd/README.md)
- [统一启动器](../cmd/weisyn/README.md)
- [节点 API](../_docs/architecture/API_GATEWAY_ARCHITECTURE.md)
- [配置说明](../configs/README.md)

---

## 📝 变更历史

| 版本 | 日期 | 变更内容 |
|-----|------|---------|
| 2.0 | 2025-11-01 | 清理冗余文档，规范化 README |
| 1.1 | 2025-10-27 | 添加整体定位说明，明确CLI业务层(L2)的架构定位 |
| 1.0 | 2025-11-26 | CLI从`internal/cli`迁移到`cmd/wes/`，创建`client/`支持库 |
