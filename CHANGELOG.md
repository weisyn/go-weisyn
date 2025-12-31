# 变更日志

**版本**：1.0  
**状态**：stable  
**最后更新**：2025-01-XX  
**所有者**：WES 核心团队

---

本文档记录 WES 区块链核心节点的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [0.1.0] - 2025-12-31

### 新增

#### 挖矿稳定性门闸（V2）
- ✅ **网络法定人数检查**：挖矿前置条件升级为"网络法定人数+高度一致性确认"
  - 新增 `min_network_quorum_total` 配置项（最小网络法定人数，含本机）
  - 新增 `allow_single_node_mining` 配置项（仅 dev 环境允许单节点挖矿）
  - 新增 `network_discovery_timeout_seconds` 配置项（网络发现超时）
  - 新增 `quorum_recovery_timeout_seconds` 配置项（法定人数恢复超时）
  - 新增 `max_initial_height_skew` 配置项（初始高度偏差阈值）
  - 新增 `max_runtime_height_skew` 配置项（运行时高度偏差阈值）
  - 新增 `max_tip_staleness_seconds` 配置项（链尖时效性阈值）
  - 新增 `enable_tip_freshness_check` 配置项（是否启用链尖新鲜度检查）
  - 新增 `enable_network_alignment_check` 配置项（是否启用网络对齐检查，默认 true，允许关闭以在生产环境逐步启用）
- ✅ **网络法定人数状态机**：7 个状态（NotStarted, Discovering, QuorumPending, QuorumReached, HeightAligned, HeightConflict, Isolated）
- ✅ **高度一致性判定**：基于中位数 peer 高度 + 偏差阈值
- ✅ **链尖前置条件检查**：链尖可读性 + 新鲜度（V2 调整：`tip_stale` 不直接阻止挖矿，除非同时存在 `HeightConflict`）
- ✅ **JSON-RPC API**：`wes_getMiningQuorumStatus` 返回完整门闸状态、指标、链尖信息
- ✅ **HTTP Debug 端点**：`GET /api/v1/debug/mining/quorum` 返回人类可读的挖矿门闸状态
- ✅ **Prometheus 指标**：5 个指标（`weisyn_mining_quorum_state`, `weisyn_mining_quorum_peers`, `weisyn_mining_quorum_height_skew`, `weisyn_mining_quorum_check_total`, `weisyn_mining_quorum_tip_age_seconds`）

### 改进

- ✅ **挖矿前置条件检查**：从"仅检查同步模式"升级为"网络法定人数+高度一致性确认"
- ✅ **孤岛挖矿处理**：默认禁止孤岛挖矿，仅 dev 环境允许通过配置显式启用
- ✅ **链尖新鲜度语义**：`tip_stale` 不再直接阻止挖矿，避免全网自锁
- ✅ **API 使用规范**：`wes_setMiningEnabled(enabled=true)` 和 HTTP REST API 的 `enabled=true` 请求将被拒绝，强制使用 `wes_startMining`（包含 V2 挖矿门闸检查）
- ✅ **高度交换并发优化**：门闸检查中的高度交换从串行改为并发（worker pool + semaphore），降低 90% 延迟（30 peers 场景：150s → 15s）
- ✅ **挖矿启动语义强化**：`wes_startMining` 从"后台等待模式"改为"硬门槛模式"，确保 API 响应与实际挖矿状态一致
- ✅ **配置简化**：`min_network_quorum_total` 统一从配置文件读取（默认 2），不再依赖环境判断（test/prod），用户应根据实际需求在配置文件中显式设置

### 修复

- ✅ 修复配置默认值位置（从 `config.go` 迁移到 `defaults.go`，符合架构规范）
- ✅ 修复配置默认值逻辑（从环境依赖改为统一默认值 2，由用户在配置文件中显式控制）
- ✅ 修复配置验证逻辑（`min_network_quorum_total` 从硬编码 `>= 2` 改为 `>= 1`，完全由配置文件控制，允许单节点测试）
- ✅ 修复 `wes_startMining` API 语义偏差（P0）：移除后台挂起等待逻辑，门槛未通过时直接返回错误
- ✅ 修复 JSON-RPC 字段命名（P1）：`ready_for_network_handshake` 重命名为 `tip_healthy_for_handshake`，明确包含新鲜度检查的语义
- ✅ 修复高度交换性能瓶颈（P1）：实现 worker pool (10 并发度) + semaphore 限流，避免连接风暴

### ⚠️ 破坏性变更

#### API 行为变更

1. **`wes_startMining` 行为变更**（P0 修复）
   - **旧行为**：门槛未通过时返回成功，后台挂起等待
   - **新行为**：门槛未通过时直接返回错误
   - **影响**：调用方需要处理门槛检查失败的错误，不能假设总是成功启动
   - **迁移**：检查错误信息中的 `suggested_action` 字段，根据建议进行故障排查
   - **示例错误**：`"挖矿门槛未通过: 网络法定人数不足（当前=1 需要=2）（建议操作: 等待更多节点加入网络）"`

2. **JSON-RPC 字段重命名**（P1 修复）
   - **旧字段**：`wes_getMiningQuorumStatus` 响应中的 `chain_tip.ready_for_network_handshake`
   - **新字段**：`chain_tip.tip_healthy_for_handshake`
   - **影响**：客户端解析 JSON-RPC 响应时需要更新字段名
   - **迁移**：将客户端代码中的字段引用从 `ready_for_network_handshake` 改为 `tip_healthy_for_handshake`

#### 配置兼容性

- ✅ 所有现有配置项保持不变，无破坏性变更
- ✅ 新增的配置项均有合理的默认值

### 文档

- ✅ 更新配置模板（`configs/templates/private-chain.json`、`configs/templates/consortium-chain.json`）添加 V2 挖矿门闸配置项
- ✅ 更新运维手册（`_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/04-NODE_CAPABILITY_OPERATIONS_GUIDE.md`）添加 V2 挖矿门闸故障排查指南
- ✅ 更新架构文档（8 个文档）反映 V2 挖矿稳定性门闸设计
- ✅ 添加迁移指南：`wes_startMining` 硬门槛语义变更 + JSON-RPC 字段重命名

### 计划中

- 文档结构标准化
- 多项目文档对齐

---

## [1.0.0] - 2025-01-XX

### 新增

#### 核心功能
- ✅ **ISPC 可验证计算**
  - WASM 智能合约执行
  - ONNX AI 模型推理
  - 单次执行+多点验证范式
  - ZK 证明生成和验证

- ✅ **EUTXO 扩展模型**
  - 三层输出架构（Asset/Resource/State）
  - 引用不消费模式
  - 原生多资产支持

- ✅ **URES 统一资源管理**
  - 内容寻址存储
  - 静态资源和可执行资源统一管理
  - 资源完整性验证

- ✅ **PoW+XOR 聚合共识**
  - 工作量证明 + XOR 距离选择
  - 微秒级确认
  - 完全去中心化

#### 接口和工具
- ✅ HTTP/RPC API
- ✅ gRPC API
- ✅ WebSocket 实时推送
- ✅ CLI 命令行工具
- ✅ SDK 支持（Go/JS）

### 改进

- 文档结构标准化
- 性能优化
- 安全性增强

### 修复

- 各种 bug 修复

---

**注意**: 详细变更历史请参考 [_dev/history/](./_dev/history/) 目录

