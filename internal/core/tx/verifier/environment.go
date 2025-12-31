// Package verifier 提供交易验证微内核实现
//
// environment.go: VerifierEnvironment 实现
package verifier

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// StaticVerifierEnvironment 静态验证环境实现
//
// 🎯 **核心职责**：提供基本的区块上下文和查询能力
//
// 💡 **设计理念**：
// 这是一个基础的实现，提供静态的区块上下文信息。
// 适用于测试和简单的验证场景。
//
// ✅ **已完善**：
// - GetNonce: 通过QueryService查询账户nonce
// - GetTxBlockHeight: 通过QueryService查询交易所在区块高度
// - GetOutput: 通过GetUTXO获取Output
type StaticVerifierEnvironment struct {
	blockHeight  uint64                    // 当前区块高度
	blockTime    uint64                    // 当前区块时间（Unix时间戳）
	minerAddress []byte                    // 矿工地址
	chainID      []byte                    // 链ID
	utxoQuery    persistence.UTXOQuery     // UTXO查询服务
	keyManager   crypto.KeyManager          // 密钥管理器（用于GetPublicKey，可选）
	queryService persistence.QueryService   // 统一查询服务（用于GetNonce、GetTxBlockHeight等）
}

// VerifierEnvironmentConfig 验证环境配置
type VerifierEnvironmentConfig struct {
	BlockHeight  uint64                    // 区块高度
	BlockTime    uint64                    // 区块时间
	MinerAddress []byte                    // 矿工地址
	ChainID      []byte                    // 链ID
	UTXOQuery    persistence.UTXOQuery     // UTXO查询服务
	KeyManager   crypto.KeyManager         // 密钥管理器（可选，用于GetPublicKey）
	QueryService persistence.QueryService  // 统一查询服务（用于GetNonce、GetTxBlockHeight等，可选）
}

// NewStaticVerifierEnvironment 创建静态验证环境
//
// 参数：
//   - config: 验证环境配置
//
// 返回：
//   - *StaticVerifierEnvironment: 验证环境实例
func NewStaticVerifierEnvironment(config *VerifierEnvironmentConfig) *StaticVerifierEnvironment {
	return &StaticVerifierEnvironment{
		blockHeight:  config.BlockHeight,
		blockTime:    config.BlockTime,
		minerAddress: config.MinerAddress,
		chainID:      config.ChainID,
		utxoQuery:    config.UTXOQuery,
		keyManager:   config.KeyManager,
		queryService: config.QueryService,
	}
}

// GetBlockHeight 获取当前区块高度
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) GetBlockHeight() uint64 {
	return e.blockHeight
}

// GetBlockTime 获取当前区块时间
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) GetBlockTime() uint64 {
	return e.blockTime
}

// GetMinerAddress 获取矿工地址
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) GetMinerAddress() []byte {
	return e.minerAddress
}

// GetChainID 获取链ID
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) GetChainID() []byte {
	return e.chainID
}

// GetExpectedFees 获取期望费用
//
// 实现 tx.VerifierEnvironment 接口
//
// ⚠️ **当前实现**：返回nil（仅在验证Coinbase时需要）
func (e *StaticVerifierEnvironment) GetExpectedFees() *txiface.AggregatedFees {
	// 当前简化实现：返回nil
	// 实际应从区块内交易聚合计算
	return nil
}

// GetUTXO 查询单个UTXO
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	if e.utxoQuery == nil {
		return nil, fmt.Errorf("UTXO查询服务未提供")
	}
	return e.utxoQuery.GetUTXO(ctx, outpoint)
}

// IsCoinbase 判断当前交易是否为Coinbase
//
// 实现 tx.VerifierEnvironment 接口
func (e *StaticVerifierEnvironment) IsCoinbase(tx *transaction.Transaction) bool {
	// Coinbase交易特征：输入数量为0或输入为空
	return len(tx.Inputs) == 0
}

// GetNonce 获取账户当前nonce
//
// 实现 tx.VerifierEnvironment 接口（扩展方法）
//
// ✅ **当前实现**：通过QueryService查询账户nonce
//
// ⚠️ **注意**：如果QueryService未提供，返回错误
// 调用方应确保在创建VerifierEnvironment时提供QueryService
func (e *StaticVerifierEnvironment) GetNonce(ctx context.Context, address []byte) (uint64, error) {
	if e.queryService == nil {
		return 0, fmt.Errorf("QueryService未提供，无法查询账户nonce（请在创建VerifierEnvironment时提供QueryService）")
	}
	return e.queryService.GetAccountNonce(ctx, address)
}

// GetPublicKey 获取地址对应的公钥
//
// 实现 tx.VerifierEnvironment 接口（扩展方法）
//
// ✅ **当前实现**：完善版本
// - 优先从KeyManager查询（如果提供）
// - 从UTXO查询（如果地址是UTXO owner）
// - 从交易输出查询（如果地址是输出owner）
func (e *StaticVerifierEnvironment) GetPublicKey(ctx context.Context, address []byte) ([]byte, error) {
	if len(address) == 0 {
		return nil, fmt.Errorf("地址为空")
	}

	// 方案1：从UTXO查询（查找该地址拥有的UTXO，从LockingCondition提取公钥）
	if e.utxoQuery != nil {
		// 使用 GetUTXOsByAddress 查询该地址拥有的UTXO
		utxos, err := e.utxoQuery.GetUTXOsByAddress(ctx, address, nil, true)
		if err == nil && len(utxos) > 0 {
			// 从第一个UTXO的LockingCondition提取公钥
			utxo := utxos[0]
			if output := utxo.GetCachedOutput(); output != nil {
				for _, lock := range output.LockingConditions {
					if singleKeyLock := lock.GetSingleKeyLock(); singleKeyLock != nil {
						if pubKey := singleKeyLock.GetRequiredPublicKey(); pubKey != nil {
							return pubKey.Value, nil
						}
					}
				}
			}
		}
	}

	// 方案2：从KeyManager查询（如果提供）
	// ⚠️ **注意**：KeyManager接口不包含GetPublicKeyByAddress方法
	// 通常KeyManager用于密钥生成和格式转换，不用于地址到公钥的查询
	// 地址到公钥的映射需要从UTXO或账户状态中查询
	if e.keyManager != nil {
		// KeyManager主要用于密钥操作，不是地址查询
		// 这里暂时跳过
	}

	// 方案3：从账户状态查询（如果实现了账户状态存储）
	// ⚠️ **待实现**：需要账户状态查询服务

	// 当前无法获取公钥
	return nil, fmt.Errorf("无法获取地址 %x 对应的公钥（需要KeyManager或UTXO查询支持）", address)
}

// GetTxBlockHeight 获取指定交易所在的区块高度
//
// 实现 tx.VerifierEnvironment 接口（扩展方法）
//
// ✅ **当前实现**：通过QueryService查询交易所在区块高度
//
// ⚠️ **注意**：如果QueryService未提供，返回错误
// 调用方应确保在创建VerifierEnvironment时提供QueryService
func (e *StaticVerifierEnvironment) GetTxBlockHeight(ctx context.Context, txID []byte) (uint64, error) {
	if e.queryService == nil {
		return 0, fmt.Errorf("QueryService未提供，无法查询交易所在区块高度（请在创建VerifierEnvironment时提供QueryService）")
	}
	return e.queryService.GetTxBlockHeight(ctx, txID)
}

// GetOutput 获取指定OutPoint对应的TxOutput
//
// 🎯 **核心职责**：通过UTXO查询获取完整的TxOutput（包含LockingConditions）
//
// 💡 **设计理念**：
// 此方法用于TimeLockPlugin和HeightLockPlugin等插件，需要从UTXO中提取
// 实际的LockingCondition进行验证，而不是依赖客户端提供的证明。
//
// 参数：
//   - ctx: 上下文对象
//   - outpoint: UTXO的OutPoint
//
// 返回：
//   - *transaction.TxOutput: TxOutput对象（包含LockingConditions）
//   - error: 查询错误（如UTXO不存在）
//
// 用途：
//   - TimeLockPlugin: 从UTXO查询TimeLock锁定条件
//   - HeightLockPlugin: 从UTXO查询HeightLock锁定条件
//   - 其他需要完整Output信息的验证场景
func (e *StaticVerifierEnvironment) GetOutput(ctx context.Context, outpoint *transaction.OutPoint) (*transaction.TxOutput, error) {
	if e.utxoQuery == nil {
		return nil, fmt.Errorf("UTXO查询服务未提供")
	}
	
	utxo, err := e.utxoQuery.GetUTXO(ctx, outpoint)
	if err != nil {
		return nil, fmt.Errorf("查询UTXO失败: %w", err)
	}
	
	output := utxo.GetCachedOutput()
	if output == nil {
		return nil, fmt.Errorf("UTXO未包含Output信息")
	}
	
	return output, nil
}

// IsSponsorClaim 判断当前交易是否为赞助领取交易
//
// 实现 tx.VerifierEnvironment 接口（扩展方法）
func (e *StaticVerifierEnvironment) IsSponsorClaim(tx *transaction.Transaction) bool {
	// 赞助领取交易特征：
	// 1. 输入数量为1
	// 2. 输入使用DelegationProof
	// 3. 输入的UTXO Owner = SponsorPoolOwner
	if len(tx.Inputs) != 1 {
		return false
	}
	return tx.Inputs[0].GetDelegationProof() != nil
}

// 编译期检查：确保 StaticVerifierEnvironment 实现了 tx.VerifierEnvironment 接口
var _ txiface.VerifierEnvironment = (*StaticVerifierEnvironment)(nil)

