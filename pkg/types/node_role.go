// Package types 提供节点角色相关的类型定义
package types

// NodeRole 节点角色
//
// 用于区分节点在网络中的职责：
// - miner:     出块节点，通常需要 from_genesis 或受信任快照 + 完整同步
// - validator: 共识验证节点，参与投票/验证但不直接挖矿
// - full:      普通全节点，仅同步与转发，不参与出块/投票
// - light:     轻节点，仅维护头部与部分状态
type NodeRole string

const (
	NodeRoleMiner     NodeRole = "miner"
	NodeRoleValidator NodeRole = "validator"
	NodeRoleFull      NodeRole = "full"
	NodeRoleLight     NodeRole = "light"
)

// StartupMode 启动同步模式
//
// 用于控制节点启动时的同步策略：
// - from_genesis: 节点可以从本地创世高度开始（典型 dev/单节点挖矿场景）
// - from_network: 节点应从网络获取已有区块高度再参与出块/业务（典型 test/prod follower）
// - snapshot:     节点从快照导入后再追同步（预留，当前实现视为 from_network 的变体）
type StartupMode string

const (
	StartupModeFromGenesis StartupMode = "from_genesis"
	StartupModeFromNetwork StartupMode = "from_network"
	StartupModeSnapshot    StartupMode = "snapshot"
)

// Environment 运行环境
//
// 描述部署的生命周期阶段，影响日志级别、指标上报、默认端口等运维属性：
// - dev:  开发环境，允许更宽松的约束
// - test: 测试环境，接近生产但允许部分降级
// - prod: 生产环境，最严格的约束和验证
type Environment string

const (
	EnvDev  Environment = "dev"
	EnvTest Environment = "test"
	EnvProd Environment = "prod"
)

