// Package incentive 提供矿工侧激励收集功能
//
// 本包实现矿工在创建候选区块时，收集激励交易（Coinbase + 赞助）的逻辑。
package incentive

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	configiface "github.com/weisyn/v1/pkg/interfaces/config"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// Collector 矿工激励收集器
//
// 🎯 **矿工侧激励收集**
//
// 职责:
//   - 调用 IncentiveTxBuilder 构建激励交易
//   - 返回 [Coinbase, ClaimTxs...] 供区块组装
//
// 设计说明:
//   - minerAddr: 运行时通过SetMinerAddress设置（挖矿启动时）
//   - chainID: 构造时从配置自动获取，无需传递参数
type Collector struct {
	incentiveBuilder txiface.IncentiveTxBuilder
	minerAddr        []byte               // 矿工地址（通过SetMinerAddress运行时设置）
	chainID          []byte               // 链ID（构造时从配置获取，8字节）
	config           configiface.Provider // 配置提供者（用于日志等，chainID已提取）
	mu               sync.RWMutex         // 保护minerAddr的并发访问
}

// NewCollector 创建激励收集器
//
// 参数:
//
//	incentiveBuilder: 激励交易构建器
//	config: 配置提供者（用于获取chainID）
//
// 设计说明:
//   - minerAddr 不在构造时设置，必须在挖矿启动时通过 SetMinerAddress 提供
//   - chainID 从配置中自动获取
//   - 这是正确的设计：业务参数（minerAddr）不应在系统启动时注入
func NewCollector(
	incentiveBuilder txiface.IncentiveTxBuilder,
	config configiface.Provider,
) (*Collector, error) {
	if incentiveBuilder == nil {
		return nil, fmt.Errorf("incentiveBuilder不能为nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config不能为nil（用于获取chainID）")
	}

	// 从配置获取chainID
	blockchainCfg := config.GetBlockchain()
	if blockchainCfg == nil || blockchainCfg.ChainID == 0 {
		return nil, fmt.Errorf("链ID未配置: 配置中未找到有效的blockchain.chain_id")
	}

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, blockchainCfg.ChainID)

	return &Collector{
		incentiveBuilder: incentiveBuilder,
		minerAddr:        nil, // 运行时通过 SetMinerAddress 设置
		chainID:          chainIDBytes,
		config:           config,
	}, nil
}

// CollectIncentiveTxs 收集激励交易
//
// 在 BlockManager.CreateMiningCandidate() 中调用。
//
// 参数:
//
//	ctx: 上下文
//	candidateTxs: 候选交易列表
//	blockHeight: 当前区块高度
//
// 返回:
//
//	[]*Transaction: [Coinbase, ClaimTx1, ClaimTx2, ...]
//	error: 收集错误
//
// P1-3健壮性保证:
//   - 自动获取minerAddr和chainID（多级回退）
//   - 验证地址有效性
func (c *Collector) CollectIncentiveTxs(
	ctx context.Context,
	candidateTxs []*transaction_pb.Transaction,
	blockHeight uint64,
) ([]*transaction_pb.Transaction, error) {
	// P1-3: 健壮获取minerAddr和chainID
	minerAddr, err := c.getMinerAddress()
	if err != nil {
		return nil, fmt.Errorf("获取矿工地址失败: %w", err)
	}

	chainID, err := c.getChainID()
	if err != nil {
		return nil, fmt.Errorf("获取链ID失败: %w", err)
	}

	return c.incentiveBuilder.BuildIncentiveTransactions(
		ctx,
		candidateTxs,
		minerAddr,
		chainID,
		blockHeight,
	)
}

// SetMinerAddress 运行时设置矿工地址
//
// 🎯 **运行时矿工地址设置**
//
// 用于在启动挖矿时设置矿工地址，支持动态矿工切换。
// 这个方法应该在挖矿启动时由 MinerController 调用。
//
// 参数:
//
//	minerAddr: 矿工地址（20字节）
//
// 返回:
//
//	error: 设置失败（地址长度错误等）
func (c *Collector) SetMinerAddress(minerAddr []byte) error {
	if len(minerAddr) != 20 {
		return fmt.Errorf("矿工地址长度错误: 期望20字节，实际%d字节", len(minerAddr))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 创建副本以避免外部修改
	c.minerAddr = make([]byte, 20)
	copy(c.minerAddr, minerAddr)

	return nil
}

// getMinerAddress 获取矿工地址
//
// 返回:
//
//	[]byte: 矿工地址（20字节）
//	error: 获取失败
//
// 设计说明:
//   - 矿工地址是业务参数，必须在挖矿启动时通过 SetMinerAddress 设置
//   - 如果未设置，说明业务流程错误（StartMining 未正确调用）
func (c *Collector) getMinerAddress() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 验证地址
	if len(c.minerAddr) == 20 {
		// 返回副本，避免外部修改
		addr := make([]byte, 20)
		copy(addr, c.minerAddr)
		return addr, nil
	}

	// 矿工地址未设置
	if len(c.minerAddr) == 0 {
		return nil, fmt.Errorf("矿工地址未设置: 必须在挖矿启动时通过 SetMinerAddress 提供")
	}

	// 矿工地址长度错误（不应发生，SetMinerAddress 已验证）
	return nil, fmt.Errorf("矿工地址长度错误: 期望20字节，实际%d字节（代码bug）", len(c.minerAddr))
}

// getChainID 获取链ID
//
// 返回:
//
//	[]byte: 链ID（8字节，big-endian编码的uint64）
//	error: 获取失败（不应发生，因为构造时已验证）
//
// 设计说明:
//   - chainID在构造时已从配置获取并验证，此方法直接返回
//   - 如果返回错误，说明构造时未正确初始化（代码bug）
func (c *Collector) getChainID() ([]byte, error) {
	if len(c.chainID) != 8 {
		return nil, fmt.Errorf("链ID未初始化: 期望8字节，实际%d字节", len(c.chainID))
	}
	// 返回副本，避免外部修改
	chainID := make([]byte, 8)
	copy(chainID, c.chainID)
	return chainID, nil
}
