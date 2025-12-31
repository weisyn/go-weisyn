# cmd/ - WES 可执行程序使用指南

> **定位**：`cmd/` 目录的**导航首页**，面向日常开发者，提供快速上手和任务导航。

---

## 📋 目录结构

`cmd/` 目录包含 WES 项目的所有可执行程序：

| 可执行程序 | 路径       | 适合谁用           | 主要作用                                   | 详细文档 |
|-----------|------------|--------------------|--------------------------------------------|----------|
| weisyn-node | `cmd/node` | 运维 / 节点管理员   | 启动区块链节点（公链 / 联盟链 / 私链）      | [node/README.md](./node/README.md) |
| weisyn-cli  | `cmd/cli`  | 开发者 / 矿工       | 管理账户、发送交易、控制挖矿、查询链状态    | [cli/README.md](./cli/README.md) |
| weisyn      | `cmd/weisyn` | 本地体验用户     | 一键启动本地私链 + 交互控制台               | [weisyn/README.md](./weisyn/README.md) |
| 工具集      | `cmd/tools` | 运维 / 开发       | 清理数据、生成密钥、编码参数、验证配置等    | [tools/README.md](./tools/README.md) |

---

## 🎯 快速任务导航

**我现在要做 X，应该看哪个文档 + 用什么命令？**

| 我的任务                                     | 看哪个文档 | 命令示例 |
|--------------------------------------------|-----------|---------|
| **本地起一条 dev 私链做开发**                | [node/README.md](./node/README.md) → 快速上手 | `weisyn-node --chain private --config ./configs/chains/dev-private-local.json` |
| **本地起 dev 公链测试**                      | [node/README.md](./node/README.md) → 快速上手 | `weisyn-node --chain public --config ./configs/chains/dev-public-local.json` |
| **连接公共测试网**                           | [node/README.md](./node/README.md) → 公共测试网 | `weisyn-node --chain public` |
| **用 CLI 访问节点 / 调用合约 / 做诊断**      | [cli/README.md](./cli/README.md) | `weisyn-cli chain info` |
| **一键启动本地私链 + 交互控制台**            | [weisyn/README.md](./weisyn/README.md) | `weisyn` |
| **查看节点所有启动参数**                     | [node/README.md](./node/README.md) → 节点级参数总表 | - |
| **生产环境打包部署**                         | [node/README.md](./node/README.md) → 生产打包与部署 | - |
| **了解链配置规范**                           | [configs/chains/README.md](../configs/chains/README.md) | - |

---

## ⚡ 开发/测试高频命令速查

### 本地开发（dev-private-local / dev-public-local）

```bash
# 1. 编译节点（一次即可）
make build-node

# 2. 启动本地私链（单节点，自动挖矿）
./bin/weisyn-node --chain private --config ./configs/chains/dev-private-local.json

# 或启动本地公链
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json

# 3. 生成诊断报告（需要先启动节点）
bash ./scripts/diagnose_node.sh http://localhost:28680 http://127.0.0.1:28686
open ./data/dev/dev-private-local/diagnostics/report.html  # macOS
```

### 连接公共测试网（test-public-demo）

```bash
# 启动公共测试网节点（使用内嵌配置，无需 --config）
./bin/weisyn-node --chain public

# 注意：
# - `weisyn-node` 只负责启动“节点进程”（P2P/共识/API），不会启动“可视化/交互式 CLI 界面”
# - 交互式向导与命令行管理请使用 `./bin/weisyn-cli`（例如：`./bin/weisyn-cli wizard`）

# 端口被占用时覆盖端口
./bin/weisyn-node --chain public --http-port 28700

# 端口被占用时覆盖端口
./bin/weisyn-node --chain public --http-port 28700 --grpc-port 28702 --diagnostics-port 28706
```

### CLI 常用命令

```bash
# 创建别名
alias wes=./bin/weisyn-cli

# 连接节点（首次使用）
wes wizard  # 交互式配置，输入 http://localhost:28680

# 常用查询
wes chain info
wes account list
wes account balance <address>

# 挖矿控制（如果连接的是 miner 节点）
wes mining start
wes mining status
```

> 💡 **完整命令列表**：见 [cli/README.md](./cli/README.md)

---

## 🔗 链模式快速对照

| 启动命令                                      | 链模式        | 配置来源                                      | 典型用途                 |
|---------------------------------------------|-------------|---------------------------------------------|--------------------------|
| `weisyn-node --chain public`                | 公链（测试网） | **内嵌** `configs/chains/test-public-demo.json` | 连接公共测试网（可联网） |
| `weisyn-node --chain public --config ./configs/chains/dev-public-local.json` | 公链（开发）   | `dev-public-local.json`                     | 本地单机挖矿、公链开发   |
| `weisyn-node --chain private --config ./configs/chains/dev-private-local.json` | 私链（开发）    | `dev-private-local.json`                    | 本地/内网私链开发        |

> **详细说明**：见 [node/README.md](./node/README.md) → "启动模式 & 链模式说明"

---

## 📖 详细文档索引

### 节点启动相关

- **[node/README.md](./node/README.md)** - `weisyn-node` 权威手册
  - 所有启动模式（public/consortium/private）
  - 所有命令行参数（`--http-port`、`--grpc-port`、`--diagnostics-port`、`--data-dir` 等）
  - 环境与角色推荐（dev/test/prod）
  - **生产打包与部署**（构建、systemd、Docker、K8s）

### CLI 工具相关

- **[cli/README.md](./cli/README.md)** - CLI 客户端完整文档
  - 所有子命令列表（query、tx、keys、diagnostics 等）
  - 连接节点配置
  - 高级用法

- **[weisyn/README.md](./weisyn/README.md)** - 可视化启动器文档
  - 一键启动本地私链
  - 交互式控制台功能

### 配置相关

- **[configs/chains/README.md](../configs/chains/README.md)** - 链配置规范
  - 配置选型指南
  - 字段规范与约束
  - 节点角色与同步策略

### 诊断与运维

- **[_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/03-NODE_DIAGNOSTICS_PRACTICAL_GUIDE.md](../_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/03-NODE_DIAGNOSTICS_PRACTICAL_GUIDE.md)** - 节点诊断实战指南
  - L1-L4 诊断模型
  - pprof 使用方法
  - 标准排查流程

---

## ❓ 常见问题

### Q: 命令在哪里执行？

**A:** 在**终端/命令行**中执行。打开终端（Terminal、PowerShell、CMD 等），进入项目目录，然后执行命令。

### Q: 需要先编译吗？

**A:** 有两种方式：

1. **使用 `go run`（推荐用于开发验证）**：
   ```bash
   go run ./cmd/node --chain public
   ```
   - 无需编译，直接运行
   - 适合快速测试和开发验证

2. **先编译再运行（推荐用于生产环境）**：
   ```bash
   make build-node
   ./bin/weisyn-node --chain public
   ```
   - 适合正式使用和生产部署

### Q: 为什么终端会疯狂刷新日志？如何让日志只写入文件？

**A:** 使用环境变量关闭控制台输出：

```bash
export WES_CLI_MODE=true
./bin/weisyn-node --chain public
```

设置后，所有日志只写入文件，不再在终端刷屏。日志文件位置：`{data_dir}/{env}/{instance}/logs/node-system.log`

> **详细说明**：见 [node/README.md](./node/README.md) → "与日志/诊断相关的参数"

### Q: 配置文件中的端口被占用了怎么办？

**A:** 使用节点级端口覆盖参数，无需修改配置文件：

```bash
./bin/weisyn-node --chain public --http-port 28700 --grpc-port 28702 --diagnostics-port 28706
```

> **详细说明**：见 [node/README.md](./node/README.md) → "节点级参数总表"

### Q: 节点级配置会改变链级配置吗？

**A:** 不会。`--http-port`、`--grpc-port`、`--diagnostics-port`、`--data-dir` 等节点级参数只影响本地节点，不会改变链 ID、genesis、network_namespace 等链级配置。

> **详细说明**：见 [configs/chains/README.md](../configs/chains/README.md) → "链级配置 vs 节点级配置"

### Q: 如何查看节点所有启动参数？

**A:** 见 [node/README.md](./node/README.md) → "节点级参数总表" 章节，包含所有命令行参数的完整说明。

### Q: 生产环境如何打包部署？

**A:** 见 [node/README.md](./node/README.md) → "生产打包与部署" 章节，包含构建、systemd、Docker、K8s 等部署方式。

### Q: 单节点矿工场景下，为什么一直显示"系统正在同步中，无法开始挖矿"？

**A:** 这是单节点矿工/首块出块场景的特殊情况。系统已自动识别并处理：当检测到 `Bootstrapping + localHeight=0 + networkHeight=0` 时，会视为"首个矿工节点"，允许直接开始挖矿。

> **详细说明**：见 [node/README.md](./node/README.md) → "常见问题" → "单节点矿工场景"

---

## 🎓 学习路径建议

**如果你是新手**：

1. **第一步**：本地起一条 dev 私链（见上方"开发/测试高频命令速查"）
2. **第二步**：用 CLI 连接节点，发一笔交易（见 [cli/README.md](./cli/README.md)）
3. **第三步**：了解链配置（见 [configs/chains/README.md](../configs/chains/README.md)）
4. **第四步**：深入学习节点参数（见 [node/README.md](./node/README.md)）

**如果你是运维/DevOps**：

1. **直接看**：[node/README.md](./node/README.md) → "生产打包与部署"
2. **参考**：[configs/chains/README.md](../configs/chains/README.md) → "节点角色与同步策略推荐"
