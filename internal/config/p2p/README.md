## P2P 配置模块（`internal/config/p2p`）

**P2P 配置模块**负责从链级配置（`config.Provider`）中生成统一的 `p2p.Options`，是所有 P2P 行为和默认值的**唯一来源**。  
`internal/core/p2p/*` 只能消费这里生成的配置，不得自行定义/修改用户级默认值。

---

### 1. 配置生成链路

```text
config.Provider
  ├─ GetChainMode()          # 链治理模式：public | consortium | private
  ├─ GetNetworkNamespace()   # 网络命名空间：mainnet | testnet | dev ...
  └─ GetNode()               # 节点级 P2P 配置（NodeOptions）
        ↓
internal/config/p2p.NewFromChainConfig(provider)
        ↓
p2p.Options                  # 统一的 P2P 配置对象
        ↓
internal/core/p2p/*          # Runtime / Swarm / Routing / Discovery / Connectivity
```

- **禁止**：在 `internal/core/p2p/*` 中再次从 `NodeOptions` 读取配置或硬编码默认值  
- **允许**：在 `internal/core/p2p/*` 中只读 `*p2pcfg.Options` 并据此行为

---

### 2. 链模式 → Profile / DHT / 私网 的规则

`chainMode` 由 `provider.GetChainMode()` 提供：

- **public（公有链）**
  - `Profile`：若未显式配置，强制设为 `server`
  - `PrivateNetwork`：`false`
  - `DHT`：
    - 若 `EnableDHT == true` 且 `DHTMode` 为空或 `"auto"` → 强制改为 `"server"`

- **consortium（联盟链）**
  - `Profile`：若未显式配置，强制设为 `server`
  - `PrivateNetwork`：`true`（需要 PSK，由上层提供）
  - `DHT`：
    - 若 `EnableDHT == true` 且 `DHTMode` 为空 → 默认 `"client"`（可由运维按需调整为 `"server"` / `"auto"`）

- **private（私有链）**
  - `Profile`：若未显式配置，强制设为 `lan`
  - `PrivateNetwork`：`true`
  - `DHT`：
    - 若 `EnableDHT == true` 且 `DHTMode` 为空或 `"auto"` → 强制改为 `"lan"`

> 说明：未知链模式会回退到 `Profile=server`，其余行为依赖显式配置和默认值。

---

### 3. DiscoveryNamespace（Rendezvous）命名规则

`DiscoveryNamespace` 由两部分共同决定：

- 用户显式配置：`nodeCfg.Discovery.RendezvousNamespace`
- 系统网络命名空间：`provider.GetNetworkNamespace()`（如 `mainnet` / `testnet` / `dev`）

**规则：**

- 如果 `RendezvousNamespace` 在 `NodeOptions` 中为**非空且不等于 `"weisyn"`**：
  - 视为用户显式配置，**直接复用**，`p2p.Options.DiscoveryNamespace = nodeCfg.Discovery.RendezvousNamespace`

- 否则：
  - 统一采用：`p2p.Options.DiscoveryNamespace = "weisyn-" + networkNamespace`
  - 例如：`weisyn-mainnet`、`weisyn-testnet`、`weisyn-dev`

**目的：**

- 保证**同一网络命名空间内的节点在同一 Rendezvous 空间下发现彼此**；
- 不同网络环境（主网/测试网/开发网）天然隔离，避免误连。

> 注意：`applyDefaults()` 不再为 `DiscoveryNamespace` 提供兜底默认值，完全由上述规则决定。

---

### 4. 连接水位与互联网场景默认值

`applyDefaults()` 中对连接水位的默认值采用“互联网生产环境友好”的固定推荐值：

- **MinPeers**（期望的最小连接数）
  - 默认：`8`
  - 含义：在发现循环中，低于该值时将持续主动拨号，防止网络分割

- **MaxPeers**（允许的最大连接数）
  - 默认：`50`
  - 含义：平衡拓扑冗余与资源消耗，适合中小规模链的默认运行

- **LowWater / HighWater**（连接管理水位）
  - `LowWater` 默认：`10`
    - 低于此值时，连接管理器会主动寻求新连接
  - `HighWater` 默认：`25`
    - 高于此值时，开始淘汰低质量连接，避免资源浪费

- **GracePeriod**
  - 默认：`20s`
  - 含义：连接关闭前的优雅期，给正在进行的传输留出时间

这些默认值与旧 `internal/config/node/defaults.go` 的设计意图保持一致，但**唯一生效来源**现在是 `internal/config/p2p.applyDefaults()`。

---

### 5. 资源管理与带宽配置默认值

`p2p.Options` 中的资源相关字段：

- **内存/FD 限制**
  - `MemoryLimitMB`：
    - 默认：`512`
    - 含义：为 P2P 模块预留的内存上限（MB）
  - `MaxFileDescriptors`：
    - 默认：`4096`
    - 含义：进程可用的最大 FD 数量，用于承载大量并发连接

- **Relay Service 资源**
  - `RelayMaxReservations`：
    - 默认：`128`
    - 含义：Relay 服务允许的最大预约数
  - `RelayMaxCircuits`：
    - 默认：`16`
    - 含义：每个 peer 允许的最大中继电路数
  - `RelayBufferSize`：
    - 默认：`2048`
    - 含义：Relay 连接的缓冲区大小

所有这些默认值都在 `applyDefaults()` 中统一设置，`internal/core/p2p/*` **不得**再硬编码。

---

### 6. NAT / AutoNAT / AutoRelay 的默认行为（摘要）

**NAT / Reachability / AutoNAT**

- `EnableNATPortMap`：
  - 默认：`true`（通过 NodeOptions 映射为兜底）
  - 策略：连接优先，自动尝试 UPnP/NAT-PMP 端口映射
- `ForceReachability`：
  - 默认：`""`（不强制）
  - 策略：优先由 AutoNAT 决定真实 Reachability
- `EnableAutoNATClient / EnableAutoNATService`：
  - 默认：`false`（需要显式开启）

**AutoRelay**

- `EnableAutoRelay`：默认 `false`（按需开启）
- `StaticRelayPeers`：默认从 `NodeOptions.Discovery.StaticRelayPeers` 映射
- `AutoRelayDynamicCandidates`：默认 `16`

> 具体逻辑由 `internal/core/p2p/connectivity` 模块实现，这里只负责配置与默认值的定义。

---

### 7. 与 `internal/core/p2p/README.md` 的关系

- 本文档聚焦于**配置与默认值的规范**，面向配置与基础设施开发人员；
- `internal/core/p2p/README.md` 聚焦于**模块架构和运行时职责**，面向整体架构与调用链设计；
- 二者共同约束：
  - P2P 默认行为、链模式策略、Rendezvous 命名、连接水位和资源限额，都必须以本模块中的实现为唯一来源。


