## 🎯 client 子系统迁移规划（面向 client-sdk-go 的统一方案）

> 版本：v0.1（草案）  
> 状态：规划阶段，逐步实施中  
> 目标：**退役内部 `client/` 作为“官方 Go Client”，统一通过 `client-sdk-go` 与节点交互**

---

### 1. 现状回顾

- **内部 client（本目录）**
  - 位置：`weisyn.git/client/`
  - 角色：CLI 业务层（L2），为 `cmd/weisyn` 提供：
    - 简单转账 / 合约部署 / 基础查询
    - 本地钱包 / keystore 管理
    - JSON-RPC / REST 传输封装
  - 特点：
    - 直接依赖 `internal/*`、`pb/*` 等内部实现
    - 交易构建直接使用 protobuf 结构，紧耦合 TX 内核

- **外部 Go SDK**
  - 仓库：`client-sdk-go`（独立 repo，本地路径 `sdk/client-sdk-go.git`）
  - 角色：**SDK 层**，面向 DApp / 钱包 / 服务端应用：
    - 只通过 `internal/api` 暴露的 JSON-RPC/HTTP/gRPC 访问链
    - 内部自定义 Wallet/Client 抽象，不依赖 WES 内部类型

> 结果：目前存在“两套 Go Client”——一个内嵌在 WES（本目录），一个作为独立 SDK，这与架构图中的“SDK 层只有一个 Client SDK”不一致。

---

### 2. 目标状态

结合 `docs/system/architecture/1-STRUCTURE_VIEW.md` 中的 7 层结构：

- **WES 核心（internal/* + pb/*）**
  - 职责：定义 EUTXO / 交易 / 锁 / 证明 的权威语义
  - 只对外暴露：`internal/api` 提供的 JSON-RPC / REST / gRPC 协议
  - 不再提供“官方 Go Client”库

- **Client SDK（client-sdk-go）**
  - 职责：
    - 封装节点 API（JSON-RPC/REST/gRPC）
    - 管理私钥 / keystore
    - 提供高级业务服务：Token / Staking / Market / Governance / Resource
  - 作为 **唯一官方 Go SDK**，供：
    - CLI
    - 钱包
    - 区块浏览器
    - DApp 后端 服务使用

- **其他语言 SDK（JS / Python / Java 等）**
  - 以 Go SDK 作为“参考语义实现”
  - 同样只通过 `internal/api` 访问链，不直接依赖内核类型

---

### 3. 方案选择：采用方案 B（链侧暴露通用交易能力）

在交易构建与签名上，采用 **方案 B**：

- **链上（WES）负责：**
  - EUTXO / 锁模型（SingleKeyLock / HeightLock / ContractLock / DelegationLock 等）
  - 交易 DraftJSON → 交易对象 的构建逻辑
  - SignatureHash 计算、SingleKeyProof 结构、验证插件

- **SDK 负责：**
  - 私钥存储与解锁（keystore / 内存钱包）
  - 调用链上的通用交易辅助 API：
    - 构建草稿（现有 `wes_buildTransaction`）
    - 计算签名哈希
    - 传入签名 + 公钥，让链生成 `SingleKeyProof` 并挂载到交易输入
    - 最终调用 `wes_sendRawTransaction` 提交

#### 3.1 已有能力

- `wes_buildTransaction(draft)`：从 DraftJSON 生成内部交易，并返回 `unsignedTx`（当前已在 SDK 试用）。
- 内部已有：
  - `BuildTransactionFromDraft`
  - `ComputeSignatureHash`（通过 `txHashCli`）
  - `SingleKeyProof` 结构及验证插件
  - `SendRawTransaction` 校验签名与解锁证明

#### 3.2 计划新增 / 标准化的 JSON-RPC 能力（WES 侧）

> 以下为初步设计，后续将落地到 `internal/api/jsonrpc/methods/tx.go` 中：

- `wes_computeSignatureHashFromDraft`
  - **Params**（示意）：
    - `[{ "draft": {...}, "input_index": 0, "sighash_type": "SIGHASH_ALL" }]`
  - **Result**：
    - `{ "hash": "0x..." }`  // 待签名的消息

- `wes_finalizeTransactionFromDraft`
  - **Params**：
    - `[{ "draft": {...}, "input_index": 0, "pubkey": "0x...", "signature": "0x..." }]`
  - **Result**：
    - `{ "tx": "0x<protobuf-bytes>" }` 或 `{ "signedTx": "0x..." }`

> 安全约束：节点永远不会看到私钥，只接收 DraftJSON + 公钥 + 签名。

---

### 4. 迁移路线图（分阶段实施）

#### 阶段 1：文档与约束固化（当前阶段）

- [ ] 在本文件中记录迁移目标与边界（本文件 ✅）
- [ ] 在 `client/README.md` 增加简短的 **Deprecated** 提示，指向本迁移计划
- [ ] 在 `sdk/client-sdk-go.git` 内创建 `ARCHITECTURE_BOUNDARY.md`，声明：
  - 不依赖 `internal/*` / `pkg/interfaces/*` / protobuf
  - 所有高级交易能力通过 JSON-RPC 通用 API 实现

#### 阶段 2：在 WES 中暴露通用交易辅助 API

- [ ] 在 `internal/api/jsonrpc/README.md` 中补充上述新方法的设计说明
- [ ] 在 `internal/api/jsonrpc/methods/tx.go` 中实现：
  - `wes_computeSignatureHashFromDraft`
  - `wes_finalizeTransactionFromDraft`
- [ ] 为上述方法编写单元/集成测试，确保：
  - Draft → hash → 签名 → finalized tx → `wes_sendRawTransaction` 全链路可用

#### 阶段 3：在 client-sdk-go 中对接新 API（以 Token 为先）

- [ ] 在 `client-sdk-go` 的 `client` 层增加：
  - `ComputeSignatureHashFromDraft(ctx, draftJSON, inputIndex, sighashType)`
  - `FinalizeTransactionFromDraft(ctx, draftJSON, inputIndex, pubkey, signature)`
- [ ] 重构 `services/token`：
  - `Transfer` / `BatchTransfer` 不再直接处理 protobuf wire-format
  - 使用：
    1. 构建 DraftJSON
    2. 调 `wes_computeSignatureHashFromDraft`
    3. 用 Wallet 签名
    4. 调 `wes_finalizeTransactionFromDraft`
    5. 调 `wes_sendRawTransaction`
- [ ] 让 `TestTokenTransfer_Basic` / `TestTokenBatchTransfer_Basic` 在完全不接触内部类型的前提下通过。

#### 阶段 4：Staking / Market / Governance 等写类业务迁移

- [ ] 调整 Staking 相关 tx_builder，使 DraftJSON 符合链上 `DraftJSON` 规范
- [ ] 同样切换到“Draft + hash + 签名 + finalize”的模式
- [ ] 逐步打通：
  - `TestStaking_*`
  - Market / Governance / Resource 的交易型用例

#### 阶段 5：CLI 与内部 client 的逐步退役

- [ ] 在 `cmd/weisyn` 中引入 `client-sdk-go` 作为依赖
- [ ] 优先让 CLI 的读命令（查看区块/余额/交易）走 SDK 客户端
- [ ] 渐进迁移写命令（转账 / 部署合约），保留旧实现作为 fallback（或 behind feature flag）
- [ ] 当 CLI 全部通过 SDK 工作并稳定后：
  - 将 `client/core` / `client/pkg/transport` 标记为仅用于历史兼容
  - 后续版本中逐步裁剪不再使用的内部实现

---

### 5. 约束与注意事项

- **不在 SDK 中复制 EUTXO / 锁 / 证明语义**
  - 所有语义以链内实现为准，SDK 只通过 API 访问。
- **不在 WES 中保留“官方 Go Client”**
  - 对外统一推荐 `client-sdk-go` 作为唯一 Go SDK。
- **多语言 SDK 均参考 Go SDK 语义**
  - JS/Python/Java 等 SDK 只需绑定同一套 JSON-RPC API，不再重新实现交易内核。

---

> 本规划文件将随着后续实现不断更新，并在完成各阶段里程碑后补充“完成记录”和版本号。  
> 欢迎在实现过程中在此文件中追加具体 PR 链接与设计变更说明。


