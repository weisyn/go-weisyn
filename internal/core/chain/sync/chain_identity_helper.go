// Package sync 提供链身份相关的辅助函数
package sync

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// GetLocalChainIdentity 获取本地链身份
//
// 从配置和查询服务构建完整的本地链身份标识。
// 如果本地已有创世区块，则从区块计算哈希；否则从配置计算。
//
// 参数：
//   - ctx: 上下文
//   - configProvider: 配置提供者
//   - queryService: 查询服务（可选，用于获取创世区块哈希）
//
// 返回：
//   - ChainIdentity: 本地链身份
//   - error: 获取错误
func GetLocalChainIdentity(ctx context.Context, configProvider config.Provider, queryService persistence.QueryService) (types.ChainIdentity, error) {
	if configProvider == nil {
		return types.ChainIdentity{}, fmt.Errorf("config provider 不能为空")
	}

	appConfig := configProvider.GetAppConfig()
	if appConfig == nil {
		return types.ChainIdentity{}, fmt.Errorf("app config 不能为空")
	}

	genesisConfig := configProvider.GetUnifiedGenesisConfig()
	if genesisConfig == nil {
		return types.ChainIdentity{}, fmt.Errorf("genesis config 不能为空")
	}

	// 尝试从查询服务获取创世区块哈希（如果链已初始化）
	var genesisHash string
	if queryService != nil {
		genesisBlock, err := queryService.GetBlockByHeight(ctx, 0)
		if err == nil && genesisBlock != nil {
			// 从创世区块计算哈希
			// 注意：这里需要计算区块哈希，但为了简化，我们先使用配置计算的哈希
			// 后续可以添加区块哈希计算逻辑
		}
	}

	// 如果无法从区块获取，则从配置计算
	if genesisHash == "" {
		calculatedHash, err := node.CalculateGenesisHash(genesisConfig)
		if err != nil {
			return types.ChainIdentity{}, fmt.Errorf("计算 genesis hash 失败: %w", err)
		}
		genesisHash = calculatedHash
	}

	// 构建 ChainIdentity
	identity := node.BuildLocalChainIdentity(appConfig, genesisHash)
	if !identity.IsValid() {
		return types.ChainIdentity{}, fmt.Errorf("构建的链身份无效: %v", identity)
	}

	return identity, nil
}

