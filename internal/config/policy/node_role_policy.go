// Package policy 提供节点角色策略矩阵
package policy

import (
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// NodePolicy 节点策略（启动安全策略）
//
// ✅ Phase 5.2：已简化为仅包含启动安全策略
// 运行时能力控制（挖矿、共识投票）已迁移到状态机模型（NodeRuntimeState）
// 详见：_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/13-NODE_CONSENSUS_AND_SYNC_PROFILE_DESIGN.md
type NodePolicy struct {
	Allow                    bool   // 是否允许此组合启动
	RequireTrustedCheckpoint bool   // 是否要求配置受信任检查点
	Notes                    string // 策略说明
}

// PolicyKey 策略键（用于查找策略）
type PolicyKey struct {
	Role        types.NodeRole
	Env         types.Environment
	StartupMode types.StartupMode
}

// nodePolicies 节点策略矩阵
//
// 定义所有合法的 role x environment x startup_mode 组合及其策略
var nodePolicies = map[PolicyKey]NodePolicy{
	// ========== 生产环境 (prod) ==========
	// 生产矿工：必须从网络同步，且必须有受信任检查点
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: true,
		Notes:                    "生产矿工必须有可信检查点，禁止从创世跑完整历史",
	},
	// 生产矿工：禁止从创世启动
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    false,
		RequireTrustedCheckpoint: false,
		Notes:                    "禁止在生产环境从创世跑完整历史",
	},
	// 生产验证节点：必须从网络同步，且必须有受信任检查点
	{
		Role:        types.NodeRoleValidator,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: true,
		Notes:                    "生产验证节点必须有可信检查点",
	},
	// 生产验证节点：禁止从创世启动
	{
		Role:        types.NodeRoleValidator,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    false,
		RequireTrustedCheckpoint: false,
		Notes:                    "禁止在生产环境从创世跑完整历史",
	},
	// 生产全节点：允许从网络同步，不要求检查点（但建议配置）
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "生产全节点允许从网络同步，建议配置检查点",
	},
	// 生产全节点：禁止从创世启动
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    false,
		RequireTrustedCheckpoint: false,
		Notes:                    "禁止在生产环境从创世跑完整历史",
	},
	// 生产轻节点：允许从网络同步，不需要完整同步
	{
		Role:        types.NodeRoleLight,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "生产轻节点只需要头部和部分状态",
	},
	// 生产轻节点：允许从快照启动
	{
		Role:        types.NodeRoleLight,
		Env:         types.EnvProd,
		StartupMode: types.StartupModeSnapshot,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "生产轻节点可以从快照启动",
	},

	// ========== 测试环境 (test) ==========
	// 测试矿工：允许从网络同步，建议配置检查点
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvTest,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "测试矿工允许从网络同步，建议配置检查点",
	},
	// 测试矿工：允许从创世启动（测试环境放宽）
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvTest,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "测试环境允许从创世启动",
	},
	// 测试全节点：允许从网络同步
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvTest,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "测试全节点允许从网络同步",
	},
	// 测试全节点：允许从创世启动
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvTest,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "测试环境允许从创世启动",
	},

	// ========== 开发环境 (dev) ==========
	// 开发矿工：允许所有启动模式（开发环境最宽松）
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvDev,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "开发环境允许从创世启动",
	},
	{
		Role:        types.NodeRoleMiner,
		Env:         types.EnvDev,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "开发环境允许从网络同步",
	},
	// 开发全节点：允许所有启动模式
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvDev,
		StartupMode: types.StartupModeFromGenesis,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "开发环境允许从创世启动",
	},
	{
		Role:        types.NodeRoleFull,
		Env:         types.EnvDev,
		StartupMode: types.StartupModeFromNetwork,
	}: {
		Allow:                    true,
		RequireTrustedCheckpoint: false,
		Notes:                    "开发环境允许从网络同步",
	},
}

// LookupNodePolicy 查找节点策略
//
// 根据角色、环境、启动模式查找对应的策略
//
// 参数：
//   - role: 节点角色
//   - env: 运行环境
//   - mode: 启动模式
//
// 返回：
//   - NodePolicy: 找到的策略
//   - bool: 是否找到（false 表示未定义的组合）
func LookupNodePolicy(role types.NodeRole, env types.Environment, mode types.StartupMode) (NodePolicy, bool) {
	key := PolicyKey{
		Role:        role,
		Env:         env,
		StartupMode: mode,
	}
	policy, ok := nodePolicies[key]
	return policy, ok
}

// ValidateNodePolicy 验证节点策略配置
//
// 检查给定的角色/环境/启动模式组合是否合法，以及是否满足策略要求
// （如 require_trusted_checkpoint）
//
// 参数：
//   - role: 节点角色
//   - env: 运行环境
//   - mode: 启动模式
//   - requireTrustedCheckpoint: 配置中是否要求受信任检查点
//   - hasTrustedCheckpoint: 是否已配置受信任检查点
//
// 返回：
//   - error: 验证失败的错误
func ValidateNodePolicy(
	role types.NodeRole,
	env types.Environment,
	mode types.StartupMode,
	requireTrustedCheckpoint bool,
	hasTrustedCheckpoint bool,
) error {
	policy, ok := LookupNodePolicy(role, env, mode)
	if !ok {
		return fmt.Errorf("不支持的组合: role=%s env=%s startup_mode=%s", role, env, mode)
	}

	if !policy.Allow {
		return fmt.Errorf("禁止的组合: role=%s env=%s startup_mode=%s (%s)", role, env, mode, policy.Notes)
	}

	if policy.RequireTrustedCheckpoint {
		if !requireTrustedCheckpoint {
			return fmt.Errorf("此组合要求配置 require_trusted_checkpoint=true: role=%s env=%s startup_mode=%s", role, env, mode)
		}
		if !hasTrustedCheckpoint {
			return fmt.Errorf("此组合要求配置完整的 trusted_checkpoint (height 和 block_hash): role=%s env=%s startup_mode=%s", role, env, mode)
		}
	}

	return nil
}
