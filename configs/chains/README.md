# WES 链配置规范（v1）

> **⚠️ 重要说明**  
> **本目录存放的是"官方内嵌 Network Profile"的源配置，用于构建可以直接启动的官方公有链 / 联盟链 / 私有链网络。运行时配置来自内嵌数据，而不是直接读取本目录。**  
> - 官方发布的二进制**不会在运行时从磁盘读取**本目录下的任何文件，而是通过 `go:embed` 将这些 Profile 内嵌到二进制中  
> - 公链配置通过 `go:embed` 内嵌到二进制中，运行时只使用内存中的内嵌配置（一个默认 public 网络 profile），不限制用户另外创建自定义公有链实例  
> - 如需自建链（包括自定义公有链 / 联盟链 / 私有链），推荐使用 BaaS 或 `weisyn-node chain init` 等工具基于模板生成配置文件，而不是直接修改本目录下的官方 Profile 源文件

---

## 📋 目录

**上半部分：配置选型与使用指南（面向运维/开发）**
- [1. 我该选哪份配置？](#1-我该选哪份配置)
- [2. 官方链配置使用指南](#2-官方链配置使用指南)
- [3. 自建链配置：至少必须改哪些字段？](#3-自建链配置至少必须改哪些字段)
- [4. 节点角色与同步策略推荐](#4-节点角色与同步策略推荐)

**下半部分：配置字段规范与验证规则（参考手册）**
- [5. 配置体系概述](#5-配置体系概述)
- [6. 链配置规范（v1）](#6-链配置规范v1)
- [7. 配置验证规则](#7-配置验证规则)
- [8. 常见问题](#8-常见问题)

---

# 上半部分：配置选型与使用指南

## 1. 我该选哪份配置？

| 我的场景                                     | 推荐使用的配置文件                    | 启动命令                                    |
|--------------------------------------------|--------------------------------------|--------------------------------------------|
| 本地开发：想在本机起一条链、能打块、能发交易 | `dev-private-local.json`             | `weisyn-node --chain private --config ./configs/chains/dev-private-local.json` |
| 想连接 WES 公共测试网，看真实区块 / 调 RPC  | `test-public-demo.json`（内嵌，无需指定） | `weisyn-node --chain public`               |
| 本地开发：想测试公链模式（单节点）          | `dev-public-local.json`              | `weisyn-node --chain public --config ./configs/chains/dev-public-local.json` |
| 测试环境：想搭建联盟链网络                  | `test-consortium-demo.json`          | `weisyn-node --chain consortium --config ./configs/chains/test-consortium-demo.json` |
| 生产环境：需要自建链                        | 使用 `chain init` 生成模板，或通过 BaaS 创建 | 见下方「3. 自建链配置」 |

> 💡 **如果你只是想快速体验，优先选 `dev-private-local.json`**。详细步骤请见 `cmd/README.md` 的「3. 本地单机链快速上手」。

---

## 2. 官方链配置使用指南

### 2.1 dev-private-local.json（本地开发私有链）

**适合干嘛？**
- 本地开发 / 体验用户
- 单节点模式，不连接外部网络
- 快速测试链的基本功能（出块、转账、查询）

**直接使用**：
```bash
./bin/weisyn-node --chain private --config ./configs/chains/dev-private-local.json
```

**如果复制一份来自建链，至少必须改这些字段**：
- `network.chain_id`：改为 10000-19999 范围内的其他值（如 10002）
- `network.network_id`：改为新的唯一标识符（如 `WES_private_MY_DEV_2025`）
- `network.network_namespace`：改为新的命名空间（如 `private-my-dev`）
- `genesis.accounts`：改为你的初始账户和余额

**推荐的 node_role + sync 组合**：
- `node_role`：不填（dev 环境默认推导为 miner，预设模板）
- `sync.startup_mode`：`from_genesis`（本地开发从创世块启动）

### 2.2 test-public-demo.json（公共测试网，内嵌配置）

**适合干嘛？**
- 想体验真实网络（多节点共识、真实出块节奏）
- DApp / 后端开发者，需要连一个长期在线的测试网络

**直接使用**：
```bash
# 使用内嵌配置，无需指定 --config
./bin/weisyn-node --chain public
```

**配置特点**：
- `chain_id = 12001`（测试网段）
- `network_namespace = "public-testnet-demo"`
- `mining.enable_aggregator = true`（多节点共识）
- `node.enable_dht = true`（需要 DHT 发现其他节点）

**推荐的 node_role + sync 组合**：
- `node_role`：不填或 `full`（预设模板：完整同步 + 不参与共识）
- `sync.startup_mode`：`from_network`（从网络同步已有区块）

### 2.3 dev-public-local.json（本地开发公链）

**适合干嘛？**
- 本地开发，想测试公链模式（但单节点运行）
- 不连接外部网络，完全本地化

**直接使用**：
```bash
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json
```

**配置特点**：
- `chain_id = 11001`（开发段）
- `mining.enable_aggregator = false`（单节点模式）
- `node.enable_dht = false`（单节点不需要 DHT）

### 2.4 test-consortium-demo.json（测试环境联盟链）

**适合干嘛？**
- 测试环境，想搭建联盟链网络
- 多机构共同背书场景

**直接使用**：
```bash
./bin/weisyn-node --chain consortium --config ./configs/chains/test-consortium-demo.json
```

**如果复制一份来自建链，至少必须改这些字段**：
- `network.chain_id`：改为 20000-29999 范围内的其他值
- `network.network_id`：改为新的唯一标识符
- `network.network_namespace`：改为新的命名空间
- `genesis.accounts`：改为各机构的初始账户
- `node.host.gater.allow_cidrs`：配置联盟成员 IP 段
- `node.bootstrap_peers`：配置引导节点地址

**推荐的 node_role + sync 组合**：
- `node_role`：`miner` 或 `validator`（预设模板：共识节点，具备挖矿/投票资格）
- `sync.startup_mode`：`from_network`（从网络同步）
- `sync.require_trusted_checkpoint`：`true`（建议配置受信任检查点）
  
  **注意**：`miner`/`validator` 表示"具备挖矿/投票资格"，实际是否在挖矿由运行时 API（`StartMining/StopMining`）控制

---

## 3. 自建链配置：至少必须改哪些字段？

### 3.1 生成配置文件模板

**联盟链/私链**：
```bash
./bin/weisyn-node chain init --mode consortium --out ./my-consortium.json
./bin/weisyn-node chain init --mode private --out ./my-private.json
```

**自建公链**：
- 通过 BaaS Web 控制台创建，Node CLI 不支持 `chain init --mode public`

### 3.2 必须修改的字段（所有链模式通用）

生成模板后，**至少必须修改以下字段**：

1. **链身份字段**（必须唯一）：
   - `network.chain_id`：链ID
     - 公有链：1-9999（官方主网固定为 1）
     - 联盟链：20000-29999
     - 私有链：10000-19999
   - `network.network_id`：网络标识符（字符串，格式 `WES_<type>_<name>_<year>`）
   - `network.network_namespace`：网络命名空间（字符串，必须唯一）

2. **创世配置**：
   - `genesis.timestamp`：创世时间戳（Unix 时间戳，秒）
   - `genesis.accounts`：至少一个创世账户，每个账户必须包含：
     - `address`：账户地址
     - `initial_balance`：初始余额（字符串，BaseUnit单位，1 WES = 10^8 BaseUnit）

3. **运行环境**：
   - `environment`：`dev` | `test` | `prod`

### 3.3 链模式特定字段

**联盟链额外建议**：
- `node.bootstrap_peers`：至少一个引导节点地址
- `node.host.gater.allow_cidrs`：联盟成员 IP 段
- `security.certificate_management.*`：联盟 CA / 节点证书（由 BaaS 或运维系统注入）

**私链额外建议**：
- `security.psk.file`：PSK 文件路径（由工具生成，不建议手工填明文密钥）
- `node.listen_addresses`：仅绑定 `127.0.0.1` 或内网 IP

---

## 4. 节点角色与同步策略推荐

> **⚠️ 重要说明**  
> `node_role` 是 **v1 的预设模板**，内部会被映射为多个独立维度（共识维度、同步/存储维度）。  
> 未来版本将逐步引入显式的 `sync.profile` / `consensus.role` 字段，`node_role` 将标记为 deprecated。  
> **详细设计缺陷分析和 v2 改进方向，请参见**：`_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/12-NODE_ROLE_DESIGN_DEFECTS_AND_V2_PROPOSAL.md`

### 4.1 节点角色预设模板对照表

**节点能力的三维度模型**（概念层面）：

1. **共识维度**：是否参与共识、参与方式（出块/投票）
2. **同步/存储维度**：数据同步深度和存储策略（全节点/轻节点）
3. **网络/部署维度**：网络拓扑和准入策略（由 ChainMode + security + gater 等决定）

**v1 预设模板**（`node_role`）：

`node_role` 是上述维度的**便捷组合模板**，内部映射关系如下：

| 预设模板（node_role） | 内部映射 | 能力说明 | 典型场景 |
|---------------------|---------|---------|---------|
| `miner` | `sync.profile=full`<br>`consensus.role=proposer+voter` | 完整同步 + 可出块 + 参与共识投票 | 矿工节点、激励节点 |
| `validator` | `sync.profile=full`<br>`consensus.role=voter` | 完整同步 + 仅参与共识投票（不出块） | 多机构共同背书 |
| `full` | `sync.profile=full`<br>`consensus.role=none` | 完整同步区块和状态、提供 RPC<br>不参与挖矿/共识 | DApp 后端、数据服务 |
| `light` | `sync.profile=light`<br>`consensus.role=none` | 同步区块头、SPV 验证<br>不保存完整状态、不参与挖矿/共识 | 钱包、客户端、边缘设备 |

**说明**：

- **挖矿是运行时能力**：`miner`/`validator` 角色表示“具备挖矿/投票资格”，但实际是否在挖矿由运行时 API（`StartMining/StopMining`）控制
- **角色是预设模板**：这些角色名是 v1 的便捷方式，未来会逐步迁移到显式的多维度配置
- **详细策略矩阵**：不同角色在不同环境下的行为约束，请见下方「6. 链配置规范（v1）」章节

### 4.2 推荐的 node_role + sync 组合

**预设模板使用指南**：

| 环境      | node_role（预设模板） | sync.startup_mode | require_trusted_checkpoint | 说明 |
|----------|---------------------|-------------------|---------------------------|------|
| dev      | 不填（默认 miner） | `from_genesis` | `false` | 本地开发从创世块启动 |
| test/prod | `miner` | `from_network` | `true` | 共识节点必须配置检查点<br>**注意**：是否实际在挖矿由运行时 API 控制 |
| test/prod | `validator` | `from_network` | `true` | 共识节点必须配置检查点<br>**注意**：只参与投票，不出块 |
| test/prod | `full` | `from_network` | `false` | 全节点可选检查点<br>**注意**：不参与挖矿/共识，仅同步和提供 RPC |
| test/prod | `light` | `from_network` | `false` | 轻节点可选检查点<br>**注意**：不保存完整状态，不参与挖矿/共识 |

**配置示例（miner 节点）**：
```jsonc
"node_role": "miner",  // 预设模板：内部映射为 sync.profile=full, consensus.role=proposer+voter
"sync": {
  "startup_mode": "from_network",
  "require_trusted_checkpoint": true,
  "trusted_checkpoint": {
    "height": 123456,
    "block_hash": "0x...."
  }
}
```

**运行时控制挖矿**：
```bash
# 启动节点后，通过 JSON-RPC 控制是否实际在挖矿
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"wes_startMining","params":[],"id":1}'

# 停止挖矿
curl -X POST http://localhost:28680/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"wes_stopMining","params":[],"id":1}'
```

> 💡 **详细的策略矩阵与约束，请见下方「6. 链配置规范（v1）」章节。**
> 💡 **了解当前设计的局限性和未来改进方向，请参见**：`_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/12-NODE_ROLE_DESIGN_DEFECTS_AND_V2_PROPOSAL.md`

---

# 下半部分：配置字段规范与验证规则

## 5. 配置体系概述

WES 采用**三层配置抽象**：

1. **链模式（ChainMode）** - 产品视角：`public`（公链）、`consortium`（联盟链）、`private`（私链）
2. **运行环境（Environment）** - 运维视角：`dev`（开发）、`test`（测试）、`prod`（生产）
3. **节点级覆盖（Node Local Overrides）** - 本地节点差异：端口、数据目录等

### 链级配置 vs 节点级配置

**链级配置**（chain-level）：
- 只能来自内嵌配置（公链）或用户 JSON（联/私链）
- 不允许来自命令行 / 环境变量
- 对公链来说，用户没有"链级配置入口"

**节点级配置**（node-level）：
- 可以来自命令行（flag）、环境变量、本地 node 配置
- 对所有链模式都生效
- 不会改变链 ID / genesis / 共识，只改变"这个节点怎么暴露自己"
- **支持的节点级覆盖参数**：
  - `--http-port <port>`：覆盖 `api.http_port`（HTTP API 端口）
  - `--grpc-port <port>`：覆盖 `api.grpc_port`（gRPC API 端口）
  - `--diagnostics-port <port>`：覆盖 `node.host.diagnostics_port`（诊断/pprof 端口）
  - `--node-role <role>`：覆盖 `node_role`（预设模板：miner/validator/full/light）
  - `--data-dir <path>`：覆盖 `storage.data_root`（数据目录）

---

## 6. 链配置规范（v1）

### 6.1 核心原则

1. **共识算法唯一性**：WES 仅支持 **PoW+XOR** 共识算法，不支持其他共识类型（PoS、PoA、PBFT 等）
2. **链级配置不可变**：链级参数（`chain_id`、`network_id`、`network_namespace`、`genesis.*`、`chain_mode`）在链启动后不可修改
3. **配置分层明确**：链级配置与节点级配置严格分离，节点级配置不能覆盖链级配置
4. **Fail-fast 验证**：配置加载时立即验证所有约束，发现错误立即失败，不向后兼容

### 6.2 链模式（ChainMode）定义

WES 通过 **5 个维度**定义三种链模式：

#### 6.2.1 维度一：链身份与创世（Chain Identity & Genesis）

**定义**：一条链的全局唯一标识与初始状态

**关键字段**：
- `network.chain_id`：链ID，整数类型
  - 公有链：1-9999（官方主网固定为 1）
  - 联盟链：20000-29999
  - 私有链：10000-19999
- `network.network_id`：网络标识符，字符串，格式 `WES_<type>_<name>_<year>`
- `network.network_namespace`：网络命名空间，字符串，用于 P2P 网络隔离
- `network.chain_mode`：链模式，枚举值 `public` / `consortium` / `private`
- `genesis.timestamp`：创世时间戳，Unix 时间戳（秒），所有节点必须相同
- `genesis.accounts`：创世账户列表，至少包含一个账户

**约束**：
- `chain_id` 必须在对应范围内且全局唯一
- `network_namespace` 必须唯一，不同链实例不能相同
- `genesis.timestamp` 必须 > 0
- `genesis.accounts` 至少包含一个账户，每个账户必须包含 `address` 和 `initial_balance`

#### 6.2.2 维度二：成员准入模型（Member Admission Model）

**定义**：谁有资格运行节点、谁算"成员"

**三种模式**：

| ChainMode | 准入模型 | 实现方式 |
|-----------|---------|---------|
| `public` | **完全开放准入** | 任意节点都可以运行全节点/挖矿节点，不需要事前列入"成员列表" |
| `consortium` | **证书许可制** | 每个机构有自己的组织证书（Org CA），每个节点有节点证书，由联盟 CA 颁发或被 CA 链信任 |
| `private` | **小范围/单机构** | 每条私链生成一个 PSK（Pre-Shared Key），只有持有这个 PSK 的节点能加入对应的 P2P 网络 |

**配置字段**：
- `security.access_control.mode`：接入控制模式
  - `public`：`"open"` - 开放接入，只做黑名单/行为过滤
  - `consortium`：`"allowlist"` - 证书许可 + IP 白名单
  - `private`：`"psk"` - PSK + 内网限制
- `node.host.gater.mode`：P2P 连接门控模式
  - `public`：`"open"` - 仅通过 `deny_*` 列表拒绝恶意 IP
  - `consortium`：`"allowlist"` - 只接受白名单 IP 段/证书主体的连接
  - `private`：`"allowlist"` - 默认仅允许本机（127.0.0.1）或内网地址

#### 6.2.3 维度三：安全/加密模型（Security/Encryption Model）

**定义**：P2P 连接的安全策略与加密方式

**三种模式**：

| ChainMode | 安全模型 | 实现方式 |
|-----------|---------|---------|
| `public` | **开放协议 + 防滥用** | P2P 连接：基础加密用 libp2p 自带的 TLS/Noise，主要是保护链路，不是做白名单 |
| `consortium` | **mTLS + allowlist** | `certificate_management` 明确：联盟根 CA / 中间 CA；证书签发、吊销、轮换策略 |
| `private` | **PSK + 内网** | 启用 libp2p Private Network：所有连接都必须带 PSK；默认只监听 `127.0.0.1` 或内网地址 |

**配置字段**：
- `security.access_control.mode`：见维度二
- `security.certificate_management`：证书管理配置（仅联盟链）
  - `ca_bundle_path`：CA 证书包文件路径
- `security.psk`：PSK 配置（仅私有链）
  - `file`：PSK 文件路径

#### 6.2.4 维度四：经济模型（Economic Model）

**定义**：是否有代币、是否有挖矿/验证激励、手续费机制

**三种模式**：

| ChainMode | 经济模型 | 说明 |
|-----------|---------|------|
| `public` | **有代币 / 有挖矿奖励 / 有手续费** | 创世分配 + 后续出块奖励；适配公开市场场景 |
| `consortium` | **有代币/资产，偏记账/权限控制** | 创世可以为每个机构分配份额；也可以发行稳定币/积分等；**没有强制"必须有矿工奖励"** |
| `private` | **内部记账单位** | 默认建议：模板允许写 `genesis.accounts` 和 `mining`；但在文档和 BaaS 向导里明确：这是"**内部记账单位**"，不会对外产生任何经济属性 |

**配置字段**：
- `mining.target_block_time`：目标出块时间，时间字符串（如 `"600s"`）
  - `public`：通常 5-15 分钟（如 `"600s"`）
  - `consortium`：通常 5-30 秒（如 `"15s"`）
  - `private`：通常 3-10 秒（如 `"5s"`）
- `mining.enable_aggregator`：是否启用聚合器
  - `public`：**必须为 `true`**（生产环境必须使用分布式聚合器）
  - `consortium`：**必须为 `true`**（多机构共识）
  - `private`：**可以为 `false`**（单节点模式）
- `mining.max_mining_threads`：最大挖矿线程数
  - `public`：默认 16
  - `consortium`：默认 4
  - `private`：默认 2

#### 6.2.5 维度五：网络隔离（Network Isolation）

**定义**：libp2p 层如何确保不同链间不串线

**实现方式**：
- `network_namespace`：逻辑隔离，不同链实例使用不同的 namespace
- 安全模型叠加：`open` / `mTLS` / `PSK` 提供物理隔离
- 独立的 bootstrap 列表：每个链实例有自己的引导节点列表

**配置字段**：
- `network.network_namespace`：网络命名空间，字符串，必须唯一
- `node.bootstrap_peers`：引导节点列表，字符串数组（libp2p multiaddr 格式）
- `node.enable_dht`：是否启用 DHT
  - `public`：`true`（公网发现）
  - `consortium`：`true`（联盟内发现）
  - `private`：`false`（单节点模式）
- `node.expected_min_peers`：期望的最小 DHT peers 数量（可选，高级配置）
  - 用于 DHT 发现状态机从 Bootstrap 阶段切换到 Steady 阶段的阈值；
  - 公有链/联盟链典型值：`3`；**单节点/孤立网络**可设置为 `0`。
- `node.single_node_mode`：单节点/孤立网络模式开关（可选，高级配置）
  - `true`：明确告知运行时"这是单节点/孤立网络"，内部会自动关闭 DHT rendezvous 循环，避免无意义的 DHT 空跑；
  - `false` 或未设置：按正常 DHT 发现流程运行。

### 6.3 链模式对照表

| 维度 | Public Chain（公有链） | Consortium Chain（联盟链） | Private Chain（私有链） |
|------|----------------------|---------------------------|----------------------|
| **链身份与创世** | 可以有**多条公链实例**；每条链有独立的 `chain_id/network_id/namespace`；创世聚焦**代币和权限的初始分配** | 每条联盟链有独立 `chain_id/network_id/namespace`；`chain_mode="consortium"`；创世记录**各机构的初始账户** | 一条完整链；`chain_id/network_id/namespace/chain_mode="private"`；创世可发"内部代币"，但**默认视为内部计量单位** |
| **成员准入模型** | **完全开放准入**：任意人都可以运行全节点/挖矿节点 | **证书许可制**：每个机构有自己的组织证书（Org CA）；每个节点有节点证书 | **小范围/单机构**：每条私链生成一个 PSK，只有持有这个 PSK 的节点能加入 |
| **安全/加密模型** | P2P 连接：基础加密用 libp2p 自带的 **TLS/Noise**；`access_control.mode = open` | `access_control.mode = allowlist`；`certificate_management` 明确：联盟根 CA / 中间 CA | `access_control.mode = psk`；默认只监听 `127.0.0.1` 或内网地址；启用 **libp2p Private Network** |
| **经济模型** | 默认 **有代币 / 有挖矿奖励 / 有手续费**；共识为 **PoW+XOR** | 联盟链一般仍然"有代币/资产"，但更偏**记账/权限控制**；共识为 **PoW+XOR** | 默认建议：这是"**内部记账单位**"，不会对外产生任何经济属性；共识为 **PoW+XOR** |
| **网络隔离** | 通过 `network_namespace` 明确区分；公有链之间不会因为"都开放"就互相看见 | `network_namespace` 区分不同联盟；再叠加证书许可：不会出现"A 联盟的节点去连 B 联盟"的情况 | 三层隔离叠加：`network_namespace` + PSK + 内网 IP 限制 |

### 6.4 配置字段约束

#### 6.4.1 必需字段

所有链模式都必须包含以下字段：

- `environment`：运行环境，枚举值 `dev` / `test` / `prod`
- `network.chain_id`：链ID，整数类型
- `network.network_id`：网络标识符，字符串
- `network.network_name`：网络名称，字符串
- `network.network_namespace`：网络命名空间，字符串
- `network.chain_mode`：链模式，枚举值 `public` / `consortium` / `private`
- `genesis.timestamp`：创世时间戳，整数类型（Unix 时间戳，秒）
- `genesis.accounts`：创世账户列表，对象数组，至少包含一个账户
- `api.http_enabled`：是否启用 HTTP API，布尔值
- `api.http_port`：HTTP 端口，整数类型（1024-65535），可通过 `--http-port` 覆盖
- `api.grpc_enabled`：是否启用 gRPC API，布尔值
- `api.grpc_port`：gRPC 端口，整数类型（1024-65535），可通过 `--grpc-port` 覆盖
- `mining.target_block_time`：目标出块时间，时间字符串
- `mining.enable_aggregator`：是否启用聚合器，布尔值
- `mining.max_mining_threads`：最大挖矿线程数，整数类型
- `node.listen_addresses`：监听地址列表，字符串数组（libp2p multiaddr 格式）
- `node.host.identity.key_file`：P2P 身份密钥文件路径，字符串
- `node.host.gater.mode`：接入控制模式，枚举值 `open` / `allowlist` / `denylist`
- `security.access_control.mode`：接入控制模式，枚举值 `open` / `allowlist` / `psk`
- `security.permission_model`：权限模型，枚举值 `public` / `consortium` / `private`

#### 6.4.2 链模式特定约束

**公有链（`chain_mode="public"`）**：
- `mining.enable_aggregator` **必须为 `true`**（生产环境必须使用分布式聚合器）
- `security.access_control.mode` **必须为 `"open"`**
- `node.host.gater.mode` **必须为 `"open"`**
- `security.permission_model` **必须为 `"public"`**

**联盟链（`chain_mode="consortium"`）**：
- `mining.enable_aggregator` **必须为 `true`**（多机构共识）
- `security.access_control.mode` **必须为 `"allowlist"`**
- `node.host.gater.mode` **必须为 `"allowlist"`**
- `security.permission_model` **必须为 `"consortium"`**
- `security.certificate_management` **建议配置**（由 BaaS/运维下发）

**私有链（`chain_mode="private"`）**：
- `mining.enable_aggregator` **可以为 `false`**（单节点模式）
- `security.access_control.mode` **必须为 `"psk"`**
- `security.permission_model` **必须为 `"private"`**
- `security.psk.file` **建议配置**（由工具或运维生成）

#### 6.4.3 字段值范围约束

- `network.chain_id`：
  - 公有链：1-9999（生产主网固定为 1，但开源仓库不包含生产主网配置）
    - 开发环境公链：11000-11999（如 dev-public-local = 11001）
    - 测试环境公链：12000-12999（如 test-public-demo = 12001）
  - 联盟链：20000-29999
  - 私有链：10000-19999
- `api.http_port` / `api.grpc_port`：1024-65535
- `mining.max_mining_threads`：>= 1
- `genesis.timestamp`：> 0（Unix 时间戳，秒）
- `genesis.accounts[].initial_balance`：字符串，BaseUnit单位（1 WES = 10^8 BaseUnit）
- `genesis.expected_genesis_hash`：字符串，64字符十六进制（可选，test/prod 环境建议必须配置）

#### 6.4.4 链身份与节点角色字段

- `environment`：枚举值 `dev` / `test` / `prod`，影响节点角色策略矩阵和启动行为
- `node_role`：枚举值 `miner` / `validator` / `full` / `light`（可选，test/prod 环境建议显式配置）
  - **注意**：`node_role` 是 v1 的预设模板，内部映射为多个独立维度（共识维度、同步/存储维度）
  - 未来版本将逐步引入显式的 `sync.profile` / `consensus.role` 字段，`node_role` 将标记为 deprecated
- `sync.startup_mode`：枚举值 `from_genesis` / `from_network` / `snapshot`
- `sync.require_trusted_checkpoint`：布尔值，是否强制要求配置受信任检查点
- `sync.trusted_checkpoint`：对象，包含 `height`（uint64）和 `block_hash`（string）

**详细说明**：
- 节点角色策略矩阵：参见 [`_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/11-CHAIN_IDENTITY_AND_NODE_ROLE_POLICY.md`](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/11-CHAIN_IDENTITY_AND_NODE_ROLE_POLICY.md)
- 设计缺陷分析与 v2 改进方向：参见 [`_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/12-NODE_ROLE_DESIGN_DEFECTS_AND_V2_PROPOSAL.md`](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/12-NODE_ROLE_DESIGN_DEFECTS_AND_V2_PROPOSAL.md)

#### 6.4.5 诊断与 pprof（可选）

为方便在开发 / 测试 / 运维环境中排查性能与内存问题，WES 内置了一个**诊断 HTTP 服务**，提供标准的 Go `pprof` 端点。

- `node.host.diagnostics_enabled`：布尔值  
  - `true`：在 `diagnostics_port` 上启动内部诊断 HTTP 服务（默认只监听本机）；  
  - `false`：不启动诊断服务（默认值，推荐在生产环境保持关闭）。
- `node.host.diagnostics_port`：整数类型，诊断 HTTP 端口（默认 `28686`，定义见 `internal/config/node/defaults.go`）。

**典型用法（本地 / 阿里云单节点调试）：**

**方式 A：通过配置文件设置**

在链配置的 `node.host` 下增加：

```json
"node": {
  "host": {
    "diagnostics_enabled": true,
    "diagnostics_port": 28686
  },
  ...
}
```

**方式 B：通过命令行参数覆盖（推荐用于端口冲突场景）**

如果配置文件中的默认端口被占用，可以直接通过命令行覆盖：

```bash
# 覆盖诊断端口（无需修改配置文件）
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json --diagnostics-port 28706

# 同时覆盖多个端口（适配本机环境）
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json \
  --http-port 28700 \
  --grpc-port 28702 \
  --diagnostics-port 28706
```

启动节点后，可以在节点所在机器上访问：

- 浏览器：`http://127.0.0.1:28686/debug/pprof/`（或使用覆盖后的端口）  
- 命令行：

```bash
# 直接连接在线 pprof（使用默认端口或覆盖后的端口）
go tool pprof http://127.0.0.1:28686/debug/pprof/heap
go tool pprof http://127.0.0.1:28686/debug/pprof/goroutine

# 或先下载，再离线分析
curl -s http://127.0.0.1:28686/debug/pprof/heap > heap.out
go tool pprof heap.out
```

> **安全建议**：诊断端口仅用于内网 / 运维场景，生产环境请：
> - 保持 `diagnostics_enabled=false`，或  
> - 通过防火墙 / 安全组将 `diagnostics_port` 限制在内网 / 管理网段内访问。

> 💡 **详细步骤**：参见 `cmd/README.md` 的"7. 去哪里看日志和诊断"章节和 `_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/03-NODE_DIAGNOSTICS_PRACTICAL_GUIDE.md`。

---

## 7. 配置验证规则

### 7.1 启动时验证

节点启动时必须验证以下约束：

1. **链级身份验证**：
   - `chain_id` 必须在对应范围内
   - `network_namespace` 不能为空
   - `chain_mode` 必须与命令行参数匹配

2. **创世配置验证**：
   - `genesis.timestamp` 必须 > 0
   - `genesis.accounts` 至少包含一个账户
   - 每个账户必须包含 `address` 和 `initial_balance`
   - **链身份哈希验证**（如果配置了 `expected_genesis_hash`）：
     - 从 `genesis` 配置确定性计算 `genesis_hash`（基于 `network_id`、`chain_id`、`timestamp`、`accounts` 的规范化序列化）
     - 如果 `genesis.expected_genesis_hash` 非空，必须与计算出的 `genesis_hash` 完全匹配，否则启动失败
     - test/prod 环境建议必须配置 `expected_genesis_hash`，dev 环境可省略
   - **持久化链身份验证**：
     - 如果本地 BadgerDB 中已存在持久化的 `genesis_hash`，启动时会与当前配置计算的 `genesis_hash` 比对
     - 不一致时启动失败，防止配置被错误修改导致链身份不一致

3. **节点角色策略矩阵验证**：
   - 从 `appConfig` 获取 `node_role`、`environment`、`sync.startup_mode`
   - 查询策略矩阵（`internal/config/policy/node_role_policy.go`）：
     - 如果组合不被允许（`Allow=false`），启动失败
     - 如果策略要求 `RequireTrustedCheckpoint=true`，必须配置 `sync.require_trusted_checkpoint=true` 且 `sync.trusted_checkpoint.{height, block_hash}` 完整

4. **链模式一致性验证**：
   - `network.chain_mode` 必须与 `security.permission_model` 一致
   - `security.access_control.mode` 必须符合链模式约束
   - `node.host.gater.mode` 必须符合链模式约束
   - `mining.enable_aggregator` 必须符合链模式约束

5. **网络配置验证**：
   - `node.listen_addresses` 不能为空
   - `node.bootstrap_peers`（如配置）必须为有效的 libp2p multiaddr 格式

### 7.2 Fail-fast 原则

- 配置加载时立即验证所有约束
- 发现错误立即失败，不向后兼容
- 不提供"兼容模式"或"降级策略"

---

## 8. 常见问题

### Q: 公链模式能否修改 chain_id？

**A:** 不能。公链的链级参数由内嵌配置锁定，用户不能修改。如果想使用不同的 chain_id，应该使用联盟链或私链模式。

### Q: 联盟链/私链能否不提供配置文件？

**A:** 不能。联盟链和私链模式**必须**通过 `--config` 指定配置文件，否则启动失败。

### Q: 运行环境（env）会影响链级配置吗？

**A:** 不会。`environment` 字段只影响运行时行为（日志级别、监听地址、安全策略等），不会改变链级配置（chain_id、genesis 等）。`environment` 必须从配置文件读取，不能通过命令行参数修改。

### Q: 如何生成配置文件模板？

**A:** 
- **联盟链/私有链**：使用 `weisyn-node chain init --mode <consortium|private> --out <path>` 命令
- **公有链**：通过 BaaS Web 控制台创建，Node CLI 不支持 `chain init --mode public`

### Q: 自建公有链与官方测试网有什么区别？

**A:** 
- **官方测试网**：chain_id=12001，通过 `--chain public`（无 --config）启动，使用内嵌配置（test-public-demo）
- **自建公有链**：chain_id 由用户自定义（建议 1000-9999），通过 BaaS 创建，然后使用 `--chain public --config <baas-generated-config.json>` 启动
- 两者都是 `chain_mode="public"`，共识都是 PoW+XOR，但是完全独立的链实例，通过不同的 `network_namespace` 隔离
- ⚠️ **注意**：开源仓库不再内嵌生产主网配置。如需连接生产主网，请通过 BaaS 或运维工具获取生产配置。

### Q: 配置文件中的 chain_mode 必须与命令行参数匹配吗？

**A:** 是的。配置文件中的 `network.chain_mode` 必须与 `--chain` 参数完全匹配，否则启动失败。

### Q: 是否支持其他共识算法（PoS、PoA、PBFT 等）？

**A:** 不支持。WES 仅支持 **PoW+XOR** 共识算法，不支持其他共识类型。所有链模式（public/consortium/private）都使用 PoW+XOR 共识。

### Q: 配置文件中的端口被占用了怎么办？

**A:** 使用节点级端口覆盖参数，无需修改配置文件：

```bash
# 覆盖 HTTP 端口
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json --http-port 28700

# 覆盖 gRPC 端口
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json --grpc-port 28702

# 覆盖诊断端口
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json --diagnostics-port 28706

# 同时覆盖多个端口
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json \
  --http-port 28700 \
  --grpc-port 28702 \
  --diagnostics-port 28706
```

**说明**：
- 所有端口参数都是**节点级覆盖**，只影响当前设备，不会改变链级配置（chain_id、genesis 等）。
- JSON 配置文件中的端口值作为**默认值**，命令行参数优先级更高。
- 适用于：端口冲突、多节点部署在同一台机器、不同环境使用不同端口等场景。

### Q: 节点级配置会改变链级配置吗？

**A:** 不会。`--http-port`、`--grpc-port`、`--diagnostics-port`、`--node-role`、`--data-dir` 等节点级参数只影响本地节点，不会改变链 ID、genesis、network_namespace 等链级配置。

---

## 📖 相关文档

- **[cmd/README.md](../../cmd/README.md)** - cmd/ 目录总览（任务导航、快速上手）
- **[配置架构设计](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment)** - 配置架构设计文档
- **[ChainMode 设计文档](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/04-CHAINMODE_DESIGN.md)** - 链模式设计文档
- **[JSON 配置注释规范](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/08-JSON_CONFIG_COMMENT_STANDARD.md)** - JSON 配置注释规范
- **[官方 Profile 命名规范](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/07-OFFICIAL_PROFILE_NAMING.md)** - 官方 Profile 命名规范
- **[网络协议设计](../../_dev/01-协议规范-specs/05-网络协议-network)** - 网络协议设计
- **[BaaS 链配置模板文档](../../baas/weisyn-baas.git/_dev/04-架构设计/05-链配置模板与安全模型.md)** - BaaS 链配置模板文档
