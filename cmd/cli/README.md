# weisyn-cli - CLI 交互客户端

> **状态**: ✅ 已实现  
> **版本**: 1.0.0

## 📋 前置条件

在开始之前，请确保：

1. **已获取源代码**：克隆了 [GitHub 仓库](https://github.com/weisyn/go-weisyn)
2. **Go 环境**：Go 1.21 或更高版本（检查：`go version`）
3. **节点运行**：确保有一个 WES 节点正在运行（本地或远程）

> 💡 **如何启动节点？** 请见 `cmd/README.md` 的「3. 本地单机链快速上手」或「4. 加入公共测试网节点」。

---

## 🎯 功能概述

`weisyn-cli` 是一个独立的 CLI 客户端程序，用于：

- ✅ 连接本地或远程 WES 节点
- ✅ 管理本地钱包和账户
- ✅ 查询区块链状态、区块、交易
- ✅ 构建、签名、发送交易
- ✅ 部署和调用智能合约
- ✅ 订阅实时事件
- ✅ 挖矿控制

---

## 🚀 快速开始

### 方式一：使用 go run（推荐用于开发验证）

**适用场景**：开发、测试、快速验证代码修改。无需编译，直接运行源代码。

```bash
# 在项目根目录下执行
# 查看帮助
go run ./cmd/cli --help

# 查询链信息
go run ./cmd/cli chain info

# 列出账户
go run ./cmd/cli account list
```

### 方式二：先编译再运行（推荐用于生产环境）

**适用场景**：正式使用、需要重复运行。

#### 步骤 1：编译 CLI

```bash
# 在项目根目录下执行
# 方式 A：使用 Makefile（推荐）
make build-cli

# 方式 B：手动编译
mkdir -p bin
go build -o bin/weisyn-cli ./cmd/cli
```

#### 步骤 2：运行 CLI

```bash
# 查看帮助
./bin/weisyn-cli --help

# 查询链信息
./bin/weisyn-cli chain info

# 列出账户
./bin/weisyn-cli account list
```

### 命令名说明

- **二进制名**：`weisyn-cli`
- **命令名**：`wes`（在 `root.go` 中定义）

**使用方式**：

**使用 go run**：
```bash
go run ./cmd/cli account list
go run ./cmd/cli chain info
```

**使用编译后的二进制**：
```bash
# 方式1：直接使用二进制名
./bin/weisyn-cli account list

# 方式2：创建别名（推荐）
alias wes=./bin/weisyn-cli
wes account list

# 方式3：安装到 PATH
sudo cp bin/weisyn-cli /usr/local/bin/wes
wes account list
```

---

## 🔧 首次使用：创建 Profile

CLI 使用 Profile 来管理不同环境的配置。首次使用需要创建一个 Profile 连接到节点。

### 方式一：使用向导（推荐）

```bash
# 使用 go run
go run ./cmd/cli wizard

# 使用编译后的二进制
./bin/weisyn-cli wizard
# 或（如果创建了别名）
wes wizard
```

向导会引导你：
1. 输入节点的 JSON-RPC 地址（如 `http://localhost:28680`）
2. 输入链 ID（如 `wes-local-1`）
3. 自动创建并切换到默认 profile

### 方式二：手动创建 Profile（非交互式）

```bash
# 使用 go run
go run ./cmd/cli profile new dev-private-local \
  --jsonrpc http://localhost:28680 \
  --chain-id wes-local-1

# 使用编译后的二进制
./bin/weisyn-cli profile new dev-private-local \
  --jsonrpc http://localhost:28680 \
  --chain-id wes-local-1
```

### Profile 管理

```bash
# 列出所有 profiles
wes profile list

# 显示当前 profile 详情
wes profile show

# 切换 profile
wes profile switch test-public-demo
```

**配置目录**：
- **配置目录**: `~/.wes/` (默认)
- **Profile 文件**: `~/.wes/profiles/<name>.json`
- **Keystore**: `~/.wes/keystore/` (默认)

---

## 📚 命令列表

### 链查询 (chain)

- `chain info` - 查询链信息（链ID、高度、同步状态等）
- `chain syncing` - 查询同步状态

**示例**：
```bash
wes chain info
wes chain syncing
```

### 账户管理 (account)

- `account new` - 创建新账户
- `account list` - 列出所有账户
- `account show <address>` - 显示账户详情
- `account import <private-key>` - 导入私钥
- `account export <address>` - 导出私钥
- `account delete <address>` - 删除账户
- `account label <address> <label>` - 更新账户标签

**示例**：
```bash
wes account new --label "My Account"
wes account list
wes account show <address>
wes account balance <address>
```

### 交易操作 (tx)

- `tx build transfer` - 构建转账交易
- `tx sign` - 签名交易
- `tx send` - 发送交易
- `tx get <hash>` - 查询交易
- `tx receipt <hash>` - 查询交易回执

**示例**：
```bash
wes tx build transfer \
  --from <from-address> \
  --to <to-address> \
  --amount 1000

wes tx sign --tx-file tx.json --from <address>
wes tx send --tx-file signed-tx.json

wes tx get <hash>
wes tx receipt <hash>
```

### 合约操作 (contract)

- `contract deploy` - 部署合约
- `contract call` - 调用合约
- `contract query` - 查询合约状态

**示例**：
```bash
wes contract deploy \
  --bytecode <bytecode-file> \
  --from <address>

wes contract call \
  --contract <contract-address> \
  --method <method-name> \
  --args <args> \
  --from <address>
```

### 挖矿控制 (mining)

- `mining start` - 启动挖矿
- `mining stop` - 停止挖矿
- `mining status` - 查询挖矿状态

**示例**：
```bash
wes mining start
wes mining status
wes mining stop
```

> ⚠️ **注意**：只有连接到 `node_role=miner` 的节点时，挖矿命令才会生效。

### 节点管理 (node)

- `node info` - 查询节点基础信息（链ID、高度、同步状态）
- `node health` - 检查节点健康状态（连通性、同步、交易池）
- `node peers` - 查看网络同步相关信息（简化版 peers 视图）
- `node connect --peer-id <peerId> [--addr <multiaddr> ...] [--timeout <ms>]` - 主动尝试连接指定 P2P 节点（管理面）

**示例**：
```bash
wes node info
wes node health
wes node peers

# 主动连接指定 peer
wes node connect \
  --peer-id 12D3KooWQwA8KbfThGnuTXv67jMqPGwnd2bgASKrUaY9fV82iFTg \
  --addr /ip4/101.37.245.124/tcp/28703 \
  --timeout 10000
```

> 提示：`node connect` 命令依赖节点已开启 JSON-RPC，并实现 `wes_admin_connectPeer` 管理方法，适用于公有链/联盟链中已知节点的连通性诊断与拓扑增强。

### 其他命令

- `block get <height|hash>` - 查询区块
- `wizard` - 首次启动向导

**示例**：
```bash
wes block get 12345
wes block get 0x1234...
```

---

## 🎨 输出格式

CLI 支持多种输出格式：

- `json` - JSON 格式（默认）
- `pretty` - 格式化的 JSON
- `table` - 表格格式
- `text` - 纯文本格式

**示例**：
```bash
# 使用表格格式输出
wes account list --output table

# 使用纯文本格式
wes chain info --output text
```

---

## 📝 使用示例

### 完整流程：创建账户 → 转账 → 查询

```bash
# 1. 创建两个账户
wes account new --label "Alice"
wes account new --label "Bob"

# 2. 查看账户列表
wes account list

# 3. 构建转账交易（从 Alice 转 1000 给 Bob）
wes tx build transfer \
  --from <alice-address> \
  --to <bob-address> \
  --amount 1000

# 4. 签名交易
wes tx sign --tx-file tx.json --from <alice-address>

# 5. 发送交易
wes tx send --tx-file signed-tx.json

# 6. 查询交易回执
wes tx receipt <tx-hash>

# 7. 查询账户余额
wes account balance <alice-address>
wes account balance <bob-address>
```

---

## ❓ 常见问题

### Q: 使用 go run 还是编译后运行？

**A:** 
- **开发验证**：使用 `go run ./cmd/cli`，无需编译，修改代码后立即生效
- **生产环境**：先编译（`make build-cli`），然后运行 `./bin/weisyn-cli` 或创建别名 `wes`

### Q: 命令在哪里执行？

**A:** 在**终端/命令行**中执行。打开终端，进入项目根目录，然后执行命令。

### Q: 二进制名和命令名为什么不同？

**A:** 二进制名为 `weisyn-cli`，但内部命令名为 `wes`。建议创建别名：`alias wes=./bin/weisyn-cli`。

### Q: 如何连接到节点？

**A:** 首次使用需要运行 `wizard` 向导或手动创建 Profile，配置节点的 JSON-RPC 地址。

### Q: 如何切换不同的节点？

**A:** 使用 `profile switch` 命令：

```bash
wes profile list
wes profile switch <profile-name>
```

### Q: 如何查看命令帮助？

**A:** 使用 `--help` 参数：

```bash
wes --help
wes account --help
wes tx build transfer --help
```

---

## 🔗 相关文档

- **[cmd/README.md](../README.md)** - cmd/ 目录总览（任务导航、快速上手）
- **[node/README.md](../node/README.md)** - 节点启动说明
- **[client/README.md](../../client/README.md)** - CLI 支持库说明
- **[tools/README.md](../tools/README.md)** - 工具集说明
