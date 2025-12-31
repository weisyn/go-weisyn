package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/types"
)

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证失败 [%s]: %s", e.Field, e.Message)
}

// ValidateMandatoryConfig 验证必填配置项
//
// 🎯 **配置验证职责**：在启动时验证必填配置项，确保系统正常运行
//
// 📋 **必填配置项**：
// - chain_id: 链ID（必需，用于网络隔离）
// - network_name: 网络名称（必需，用于网络标识）
// - genesis.timestamp: 创世时间戳（必需，用于创世区块）
// - genesis.accounts: 创世账户（至少一个，必需）
//
// 参数：
//   - appConfig: 应用配置
//   - unifiedGenesis: 统一创世配置（可选，如果提供则使用）
//
// 返回：
//   - error: 验证失败的错误列表
func ValidateMandatoryConfig(appConfig *types.AppConfig, unifiedGenesis *types.GenesisConfig) error {
	var errors []error

	// 1. 验证网络配置（chain_id, network_name）
	if appConfig != nil && appConfig.Network != nil {
		if appConfig.Network.ChainID == nil || *appConfig.Network.ChainID == 0 {
			errors = append(errors, &ValidationError{
				Field:   "network.chain_id",
				Message: "链ID不能为空或0，必须配置有效的链ID",
			})
		}

		if appConfig.Network.NetworkName == nil || *appConfig.Network.NetworkName == "" {
			errors = append(errors, &ValidationError{
				Field:   "network.network_name",
				Message: "网络名称不能为空，必须配置有效的网络名称",
			})
		}
	} else {
		errors = append(errors, &ValidationError{
			Field:   "network",
			Message: "网络配置不能为空，必须配置chain_id和network_name",
		})
	}

	// 1.5 验证共识关键参数（必须显式来自链配置，禁止悄悄使用默认值）
	//
	// 目标：
	// - 避免出现“配置写了但解析/映射链路失效，系统静默回退默认值”导致共识策略整体跑偏；
	// - 对共识关键参数采取 fail-fast：缺失或非法直接启动失败。
	if appConfig == nil || appConfig.Mining == nil || appConfig.Mining.TargetBlockTime == nil || strings.TrimSpace(*appConfig.Mining.TargetBlockTime) == "" {
		errors = append(errors, &ValidationError{
			Field:   "mining.target_block_time",
			Message: "目标出块时间不能为空，必须在链配置中显式配置 mining.target_block_time（例如 \"30s\"）",
		})
	} else {
		durStr := strings.TrimSpace(*appConfig.Mining.TargetBlockTime)
		d, err := time.ParseDuration(durStr)
		if err != nil || d <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "mining.target_block_time",
				Message: fmt.Sprintf("目标出块时间格式无效: %q（期望类似 \"30s\"），err=%v", durStr, err),
			})
		}
	}

	// 1.6 v2：挖矿稳定性门闸配置验证（fail-fast）
	if appConfig != nil && appConfig.Mining != nil {
		if appConfig.Mining.MinNetworkQuorumTotal != nil && *appConfig.Mining.MinNetworkQuorumTotal < 1 {
			errors = append(errors, &ValidationError{
				Field:   "mining.min_network_quorum_total",
				Message: "min_network_quorum_total 必须 >= 1（至少包含本机）",
			})
		}

		if appConfig.Mining.NetworkDiscoveryTimeoutSeconds != nil && *appConfig.Mining.NetworkDiscoveryTimeoutSeconds <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "mining.network_discovery_timeout_seconds",
				Message: "network_discovery_timeout_seconds 必须 > 0",
			})
		}
		if appConfig.Mining.QuorumRecoveryTimeoutSeconds != nil && *appConfig.Mining.QuorumRecoveryTimeoutSeconds <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "mining.quorum_recovery_timeout_seconds",
				Message: "quorum_recovery_timeout_seconds 必须 > 0",
			})
		}
		if appConfig.Mining.MaxHeightSkew != nil && *appConfig.Mining.MaxHeightSkew == 0 {
			errors = append(errors, &ValidationError{
				Field:   "mining.max_height_skew",
				Message: "max_height_skew 必须 > 0",
			})
		}
		if appConfig.Mining.MaxTipStalenessSeconds != nil && *appConfig.Mining.MaxTipStalenessSeconds == 0 {
			errors = append(errors, &ValidationError{
				Field:   "mining.max_tip_staleness_seconds",
				Message: "max_tip_staleness_seconds 必须 > 0",
			})
		}

		// allow_single_node_mining 严格限制：仅 dev 且显式 startup_mode=from_genesis
		if appConfig.Mining.AllowSingleNodeMining != nil && *appConfig.Mining.AllowSingleNodeMining {
			env := ""
			if appConfig.Environment != nil {
				env = strings.ToLower(strings.TrimSpace(*appConfig.Environment))
			}
			startupMode := ""
			if appConfig.Sync != nil && appConfig.Sync.StartupMode != nil {
				startupMode = strings.ToLower(strings.TrimSpace(*appConfig.Sync.StartupMode))
			}

			if env != "dev" {
				errors = append(errors, &ValidationError{
					Field:   "mining.allow_single_node_mining",
					Message: "allow_single_node_mining=true 仅允许在 environment=dev 下启用",
				})
			}
			if startupMode != "from_genesis" {
				errors = append(errors, &ValidationError{
					Field:   "sync.startup_mode",
					Message: "allow_single_node_mining=true 时必须显式配置 sync.startup_mode=from_genesis",
				})
			}
		}
	}

	// 2. 验证创世配置
	// 优先使用统一创世配置，否则使用appConfig中的创世配置
	var genesisConfig *types.GenesisConfig
	if unifiedGenesis != nil {
		genesisConfig = unifiedGenesis
	} else if appConfig != nil && appConfig.Genesis != nil {
		// 转换appConfig.Genesis为统一格式
		if len(appConfig.Genesis.Accounts) > 0 {
			genesisConfig = &types.GenesisConfig{
				GenesisAccounts: make([]types.GenesisAccount, 0, len(appConfig.Genesis.Accounts)),
			}
			for _, acc := range appConfig.Genesis.Accounts {
				genesisConfig.GenesisAccounts = append(genesisConfig.GenesisAccounts, types.GenesisAccount{
					Address:        acc.Address,
					PrivateKey:     acc.PrivateKey,
					InitialBalance: acc.InitialBalance, // 🔧 修复：使用InitialBalance字符串字段
				})
			}
			// 从appConfig.Genesis获取时间戳（Timestamp是int64类型，不是指针）
			if appConfig.Genesis.Timestamp != 0 {
				genesisConfig.Timestamp = appConfig.Genesis.Timestamp
			}
		}
	}

	if genesisConfig == nil {
		errors = append(errors, &ValidationError{
			Field:   "genesis",
			Message: "创世配置不能为空，必须配置创世账户和时间戳",
		})
	} else {
		// 验证创世时间戳
		if genesisConfig.Timestamp == 0 {
			errors = append(errors, &ValidationError{
				Field:   "genesis.timestamp",
				Message: "创世时间戳不能为0，必须配置有效的Unix时间戳",
			})
		}

		// 验证创世账户（至少一个）
		if len(genesisConfig.GenesisAccounts) == 0 {
			errors = append(errors, &ValidationError{
				Field:   "genesis.accounts",
				Message: "创世账户不能为空，必须配置至少一个创世账户",
			})
		} else {
			// ✅ 安全硬闸：禁止在链配置中携带私钥（除非 dev + 显式允许）
			env := ""
			if appConfig != nil && appConfig.Environment != nil {
				env = strings.ToLower(strings.TrimSpace(*appConfig.Environment))
			}
			allowInsecurePK := strings.ToLower(strings.TrimSpace(os.Getenv("WES_ALLOW_INSECURE_GENESIS_PRIVATE_KEYS")))
			allowInsecure := allowInsecurePK == "1" || allowInsecurePK == "true" || allowInsecurePK == "yes"

			// 验证每个账户的必要字段
			for i, acc := range genesisConfig.GenesisAccounts {
				// 统一要求：创世账户必须显式给出 address（创世交易构建依赖 address）
				if strings.TrimSpace(acc.Address) == "" {
					errors = append(errors, &ValidationError{
						Field:   fmt.Sprintf("genesis.accounts[%d]", i),
						Message: "账户必须配置 address（创世交易构建依赖 address）",
					})
				}

				// 禁止把私钥塞进链配置（test/prod 一律禁止；dev 需显式开关允许）
				if strings.TrimSpace(acc.PrivateKey) != "" {
					if env != "dev" || !allowInsecure {
						msg := "检测到 genesis.accounts[%d].private_key。出于安全考虑，链配置禁止包含私钥；请删除该字段，仅保留 address/public_key。"
						if env == "dev" && !allowInsecure {
							msg += "（如确需本地临时调试，可在 environment=dev 下设置环境变量 WES_ALLOW_INSECURE_GENESIS_PRIVATE_KEYS=true 以绕过，但强烈不建议提交/分发）"
						}
						errors = append(errors, &ValidationError{
							Field:   fmt.Sprintf("genesis.accounts[%d].private_key", i),
							Message: fmt.Sprintf(msg, i),
					})
					}
				}
			}
		}
	}

	// 3. 节点角色策略矩阵验证已移除
	// 节点能力现在由状态机模型控制（sync.mode, is_fully_synced, mining.enabled）
	// 不再依赖 node_role 配置字段

	// 4. 验证创世哈希（强制校验 expected_genesis_hash）
	if genesisConfig != nil && appConfig != nil {
		// 计算本地 genesis hash
		calculatedHash, err := node.CalculateGenesisHash(genesisConfig)
		if err != nil {
			errors = append(errors, &ValidationError{
				Field:   "genesis.hash_calculation",
				Message: fmt.Sprintf("计算创世哈希失败: %v", err),
			})
		} else {
			// 如果配置了 expected_genesis_hash，必须严格匹配
			if appConfig.Genesis != nil && appConfig.Genesis.ExpectedGenesisHash != nil {
				expectedHash := strings.ToLower(strings.TrimSpace(*appConfig.Genesis.ExpectedGenesisHash))
				// 移除 0x 前缀（如果有）
				expectedHash = strings.TrimPrefix(expectedHash, "0x")
				calculatedHashLower := strings.ToLower(calculatedHash)

				if expectedHash != calculatedHashLower {
					errors = append(errors, &ValidationError{
						Field:   "genesis.expected_genesis_hash",
						Message: fmt.Sprintf("创世哈希不匹配: 配置值=%s, 计算值=%s (前8位: %s)", expectedHash, calculatedHashLower, calculatedHashLower[:min(8, len(calculatedHashLower))]),
					})
				}
			}
		}
	}

	// 如果有错误，返回组合错误
	if len(errors) > 0 {
		return &ValidationErrors{Errors: errors}
	}

	return nil
}

// ValidationErrors 多个验证错误
type ValidationErrors struct {
	Errors []error
}

func (e *ValidationErrors) Error() string {
	msg := "配置验证失败，发现以下问题：\n"
	for i, err := range e.Errors {
		msg += fmt.Sprintf("  %d. %s\n", i+1, err.Error())
	}
	return msg
}

// min 返回两个整数中的较小值（辅助函数）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
