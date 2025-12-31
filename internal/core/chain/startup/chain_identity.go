// Package startup 提供链身份相关的启动时操作
package startup

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

const (
	// ChainIdentityMetadataKey 链身份元数据键
	// 格式：system:chain_identity:genesis_hash
	ChainIdentityMetadataKey = "system:chain_identity:genesis_hash"
)

// PersistGenesisHash 持久化创世哈希到 BadgerDB metadata
//
// 在创世区块创建成功后，将计算出的 genesis_hash 写入 metadata，
// 用于后续启动时比对，防止配置变更导致链身份不一致。
//
// 参数：
//   - ctx: 上下文
//   - store: BadgerStore（用于持久化）
//   - genesisConfig: 创世配置
//
// 返回：
//   - error: 持久化错误
func PersistGenesisHash(ctx context.Context, store storage.BadgerStore, genesisConfig *types.GenesisConfig) error {
	if store == nil {
		return fmt.Errorf("BadgerStore 不能为空")
	}
	if genesisConfig == nil {
		return fmt.Errorf("genesis config 不能为空")
	}

	// 计算 genesis hash
	genesisHash, err := node.CalculateGenesisHash(genesisConfig)
	if err != nil {
		return fmt.Errorf("计算 genesis hash 失败: %w", err)
	}

	// 写入 BadgerDB metadata
	key := []byte(ChainIdentityMetadataKey)
	value := []byte(genesisHash)
	if err := store.Set(ctx, key, value); err != nil {
		return fmt.Errorf("持久化 genesis hash 失败: %w", err)
	}

	return nil
}

// ValidatePersistedGenesisHash 验证持久化的创世哈希
//
// 在启动时读取历史记录的 genesis_hash，与当前配置计算出的 hash 比对。
// 如果不一致，说明配置被错误修改，直接 fail-fast。
//
// 参数：
//   - ctx: 上下文
//   - store: BadgerStore（用于读取）
//   - genesisConfig: 当前创世配置
//
// 返回：
//   - error: 验证失败的错误（不一致或读取错误）
func ValidatePersistedGenesisHash(ctx context.Context, store storage.BadgerStore, genesisConfig *types.GenesisConfig) error {
	if store == nil {
		return fmt.Errorf("BadgerStore 不能为空")
	}
	if genesisConfig == nil {
		return fmt.Errorf("genesis config 不能为空")
	}

	// 计算当前配置的 genesis hash
	calculatedHash, err := node.CalculateGenesisHash(genesisConfig)
	if err != nil {
		return fmt.Errorf("计算 genesis hash 失败: %w", err)
	}

	// 读取历史记录的 genesis hash
	key := []byte(ChainIdentityMetadataKey)
	storedHashBytes, err := store.Get(ctx, key)
	if err != nil {
		// 如果键不存在，说明是首次启动，允许继续
		return nil
	}

	if len(storedHashBytes) == 0 {
		// 空值，视为首次启动
		return nil
	}

	storedHash := string(storedHashBytes)
	if storedHash != calculatedHash {
		return fmt.Errorf("链身份不匹配: 历史记录的 genesis_hash=%s (前8位: %s), 当前配置计算的 genesis_hash=%s (前8位: %s). 这通常意味着配置被错误修改，可能导致链身份不一致", storedHash, storedHash[:min(8, len(storedHash))], calculatedHash, calculatedHash[:min(8, len(calculatedHash))])
	}

	return nil
}

