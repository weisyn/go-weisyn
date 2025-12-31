# 术语表

---

本文档定义了 WES 系统中使用的核心术语。

---

## A

### Aggregator（XOR 距离选择节点）
在 PoW+XOR 共识中，负责收集候选区块并执行 XOR 距离选择的节点角色。基于 K-bucket 距离动态决定是否作为聚合节点，对候选区块执行 XOR 距离计算，选择距离最近的区块。

### AssetOutput（资产输出）
EUTXO 三层输出之一，代表可转移的价值，如代币、NFT、SFT。

---

## B

### Block（区块）
交易的容器，包含区块头和区块体，通过哈希链接形成区块链。

### Block Header（区块头）
区块的元信息，包含版本、前一区块哈希、Merkle 根、状态根、时间戳、难度、Nonce 等。

---

## C

### Chain（链）
区块的有序集合，形成不可篡改的历史记录。

### Consensus（共识）
多个节点就区块链状态达成一致的机制。WES 使用 PoW+XOR 混合共识。

### CU（Compute Units，计算单位）
WES 中用于计量算力的单位，合约和 AI 模型使用统一的 CU 计量。

---

## E

### EUTXO（Extended UTXO，扩展 UTXO）
WES 的状态模型，在传统 UTXO 基础上扩展了三层输出架构和引用不消费模式。

---

## G

### Genesis Block（创世区块）
区块链的第一个区块，没有前一区块。

### Gossip（八卦协议）
P2P 网络中的消息传播协议，通过节点间互相传播消息实现全网广播。

---

## H

### HostABI（宿主 ABI）
ISPC 执行环境提供给合约的接口，包括 UTXO 操作、资源操作、外部调用等。

---

## I

### ISPC（Intrinsic Self-Proving Computing，本征自证计算）
WES 的核心计算范式，执行计算的同时自动生成可验证的零知识证明。

---

## K

### K-bucket（K 桶）
Kademlia DHT 中的路由表结构，按 XOR 距离组织节点。

### Kademlia
基于 XOR 距离的分布式哈希表协议，用于 P2P 网络中的节点发现和路由。

---

## M

### Mempool（内存池/交易池）
存储待确认交易的内存区域。

### Merkle Root（Merkle 根）
Merkle 树的根哈希，用于高效验证数据完整性。

### Miner（矿工）
执行 PoW 计算，产生候选区块的节点角色。

---

## N

### Node（节点）
参与 WES 网络的计算机，可以是全节点、矿工节点或轻节点。

### Nonce（随机数）
PoW 计算中不断调整的值，用于找到满足难度要求的区块哈希。

---

## O

### ONNX（Open Neural Network Exchange）
开放神经网络交换格式，WES 支持执行 ONNX 格式的 AI 模型。

### OutPoint
UTXO 的唯一标识，由交易哈希和输出索引组成。

---

## P

### P2P（Peer-to-Peer，点对点）
去中心化的网络架构，节点直接相互连接，无需中心服务器。

### PoW（Proof of Work，工作量证明）
通过计算难题证明工作量的共识机制。

### Proof（证明）
在 ISPC 中，指零知识证明，用于验证计算的正确性。

---

## R

### Reference Input（引用型输入）
EUTXO 中的输入类型，引用但不消费 UTXO，用于读取共享数据。

### Reorg（重组）
当发现更优的分叉链时，回滚旧链并切换到新链的过程。

### ResourceOutput（资源输出）
EUTXO 三层输出之一，代表可引用的资源，如合约、模型、文件。

---

## S

### StateOutput（状态输出）
EUTXO 三层输出之一，代表执行结果或状态快照。

### StateRoot（状态根）
当前状态的 Merkle 根，用于验证状态完整性。

---

## T

### Tip（链尖）
当前主链的最新区块。

### Transaction（交易）
状态变更的基本单元，包含输入、输出和执行信息。

### TxID（交易标识）
交易的唯一哈希标识。

---

## U

### URES（统一资源管理）
WES 的资源管理系统，基于内容寻址统一管理各类资源。

### UTXO（Unspent Transaction Output，未消费交易输出）
尚未被消费的交易输出，代表可用的状态单元。

### 状态单元（Cell）
协议层的抽象概念，表示"状态单元"。在 EUTXO 模型中，包括：
- **AssetCell**：资产层状态单元，承载价值单元
- **ResourceCell**：资源层状态单元，承载或引用资源对象
- **StateRecordCell**：状态记录层状态单元，记录执行结果、审计信息与证据

**Cell vs Output**：
- `Cell` 是协议层的抽象概念，强调状态单元的生命周期（创建、消费、引用）
- `Output` 是交易层的具体实现，强调交易的输出结构
- 两者在语义上对应：`AssetCell` 对应 `AssetOutput`，`ResourceCell` 对应 `ResourceOutput`，`StateRecordCell` 对应 `StateOutput`

### 权利主体（RightBearer）
在协议视角下，对某些资源或状态拥有特定权利的抽象"控制权载体"。通过可验证的链上条件（地址、脚本、签名条件）体现，不等同于现实世界的个人/组织。

**权利类型**：
- **所有权（Ownership）**：对资源对象具有最终处置权
- **使用权（Usage）**：在给定条件下使用资源的权利
- **管理权（Management）**：调整使用条件、分配使用权等的权利

---

## W

### WASM（WebAssembly）
一种可移植的二进制指令格式，WES 支持执行 WASM 格式的智能合约。

### WES（Weisyn/微迅链）
第三代区块链，定义可验证计算范式，支持 AI 和企业应用在链上运行。

---

## X

### XOR Distance（XOR 距离）
两个标识符的异或值，用于 Kademlia 路由和 WES 共识中的区块选择。

---

## Z

### ZK Proof（Zero-Knowledge Proof，零知识证明）
允许证明者向验证者证明某个陈述为真，而不泄露任何额外信息的密码学技术。

