### pkg 目录设计总览

　　本目录用于对外暴露“公共可复用”的类型与接口，服务于全局依赖注入与跨模块协作。为确保高内聚、低耦合与稳定依赖方向，`pkg` 内采用“类型（纯领域数据）”与“接口（行为契约）”分层：

- **types**: 纯领域数据结构与值对象（无副作用、无外部依赖）。
- **interfaces**: 跨模块行为契约，仅定义接口（不包含实现）。
- **adapters**: 显式转换层（types ↔ pb/** ↔ 统一 JSON）；当前网络 Envelope 已接入 `timeconv`，其余适配器根据实际对外需求再接线。

　　依赖方向保持单向：`interfaces` 可以引用 `types` 作为参数与返回值；`types` 禁止依赖 `interfaces`。

---

### 设计目标

- **稳定性**: 通过类型与接口的分层，稳定公共表面，降低实现演进对调用方的影响。
- **解耦性**: 领域数据与行为契约解耦，具体实现下沉到 `internal/**`，由 Fx 进行装配。
- **一致性**: 统一参数/返回值的领域建模，统一接口的风格与命名；哈希/默克尔等能力统一从抽象服务（如 txHashService）获取，而非在各处散落本地实现。

---

### 目录与职责

#### pkg/types（纯领域数据）

- **允许**
  - 领域实体、值对象、只读状态快照、轻量构造与校验（不依赖 IO/网络/存储）。
  - 与领域对象紧耦合的辅助类型（如枚举、原语别名、轻量工具函数）。
- **禁止**
  - 任何具体实现或外部系统交互（存储/网络/加密设备等）。
  - 直接使用具体哈希/默克尔算法实现（统一通过 `txHashService` 等抽象服务在实现层调用）。
  - 反向依赖 `pkg/interfaces`。

#### pkg/interfaces（行为契约/接口）

- **允许**
  - 以 `context.Context` 为首参的服务接口；参数/返回值引用 `pkg/types`。
  - 跨模块能力抽象，如 `BlockRepository`、`TxPool`、`HashService`、`MerkleService`、`StorageProvider`、`NodeHost` 等。
  - 配置抽象与提供者接口见 `interfaces/config/*`（配置内容存放于 `configs/**`，经 Provider 注入）。
- **禁止**
  - 任何实现代码、构造细节或与具体技术栈强绑定的符号（例如直接暴露某三方库类型）。
  - 生命周期方法：`Start/Stop/Run/Status` 等不应出现在公共接口中。
  - 指标/度量相关接口（保持公共接口纯业务能力）。

---

### 依赖与边界

- **单向依赖**: `interfaces → types`，严禁 `types → interfaces`。
- **实现归属**: 所有接口实现位于 `internal/**`，通过 Fx 注入到应用层；Fx 模块输出应为接口类型而非具体结构体。
- **哈希/默克尔**: 统一经抽象服务（如 `HashService`/`MerkleService`，内部对接统一的 txHashService），避免在接口或类型层直接散落本地算法实现。
- **节点网络/Network 边界**: 接口只暴露领域无关的能力，不泄漏具体传输/发现策略或第三方库细节。

---

### 命名与风格

- **接口命名**: 使用能力导向名词（如 `SomethingService`、`SomethingRepository`、`SomethingProvider`）。
- **类型命名**: 使用领域名词（如 `Block`、`Transaction`、`UTXO`、`AccountState`、`Resource`）。
- **方法约定**: 入参首个为 `context.Context`；参数与返回值优先使用领域类型（来自 `pkg/types`）。

---

### 常见反模式（需避免）

- 在 `pkg/interfaces` 放置具体实现、构造函数、具体三方库对象或 `New*` 工厂。
- 在 `pkg/types` 中引入 `net/*`、`database/*`、`crypto/*` 等实现依赖，或任何与 IO/网络/存储相关逻辑。
- 在接口中出现 `Start/Stop/Run/Status` 等生命周期方法，或任何指标/度量接口。
- 直接在类型/接口层面调用本地 `sha256/keccak/blake2` 等，而非通过统一的哈希服务抽象。

---

### 快速自检（只读审计）

```bash
# 1) 禁止 types 依赖 interfaces
rg 'pkg/interfaces' pkg/types -n || true

# 2) interfaces 中不应出现构造/实现线索
rg 'New\w+\(' pkg/interfaces -n -g '!**/*_test.go' || true

# 3) 禁止生命周期方法出现在接口
rg 'Start\(|Stop\(|Run\(|Status\(' pkg/interfaces -n || true

# 4) 禁止在公共层散落本地哈希算法调用（实现应统一走抽象服务）
rg 'crypto/(sha1|sha256|sha512)|blake2|keccak' pkg -n -g '!**/internal/**' -g '!**/*_test.go' || true
```

---

### 关联阅读

- 接口标准与示例：`pkg/interfaces/blockchain/INTERFACE_STANDARDS.md`
- 配置提供与注入：`pkg/interfaces/config/*`（实际配置存放于 `configs/**`）
- 架构与模块装配：参考项目架构文档（保持与本 README 描述一致）

---

### 验收标准（面向代码评审）

- `pkg/types` 未导入 `pkg/interfaces`；工程可构建且无环依赖。
- `pkg/interfaces` 中不含实现/构造细节、无生命周期/度量接口、无第三方具体类型泄漏。
- 哈希/默克尔等能力均通过抽象服务访问，未出现零散的本地算法调用。
- 相关文档与代码描述一致，能为调用方提供稳定与清晰的公共表面。


