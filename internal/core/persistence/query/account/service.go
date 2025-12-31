// Package account 实现账户查询服务（聚合视图）
package account

import (
	"context"
	"fmt"
	"math/big"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// Service 账户查询服务
type Service struct {
	storage   storage.BadgerStore
	utxoQuery interfaces.InternalUTXOQuery
	logger    log.Logger
}

// NewService 创建账户查询服务
func NewService(storage storage.BadgerStore, utxoQuery interfaces.InternalUTXOQuery, logger log.Logger) (interfaces.InternalAccountQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if utxoQuery == nil {
		return nil, fmt.Errorf("utxoQuery 不能为空")
	}

	s := &Service{
		storage:   storage,
		utxoQuery: utxoQuery,
		logger:    logger,
	}

	if logger != nil {
		logger.Info("✅ AccountQuery 服务已创建")
	}

	return s, nil
}

// GetAccountBalance 获取账户余额（聚合视图）
func (s *Service) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	// 1. 获取地址拥有的所有资产UTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	utxos, err := s.utxoQuery.GetUTXOsByAddress(ctx, address, &assetCategory, true)
	if err != nil {
		return nil, fmt.Errorf("获取地址UTXO失败: %w", err)
	}

	// 2. 聚合余额
	totalBalance := big.NewInt(0)
	availableBalance := big.NewInt(0)
	lockedBalance := big.NewInt(0)

	for _, utxoObj := range utxos {
		// 检查是否是目标代币
		if !matchesTokenID(utxoObj, tokenID) {
			continue
		}

		// 提取金额
		amount := extractAmount(utxoObj)
		if amount == nil {
			continue
		}

		// 累加到总余额
		totalBalance.Add(totalBalance, amount)

		// 根据状态分类
		if utxoObj.Status == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
			availableBalance.Add(availableBalance, amount)
		} else {
			lockedBalance.Add(lockedBalance, amount)
		}
	}

	// 3. 构造余额信息
	// 将big.Int转换为uint64
	total := uint64(0)
	available := uint64(0)
	locked := uint64(0)
	if totalBalance.IsUint64() {
		total = totalBalance.Uint64()
	}
	if availableBalance.IsUint64() {
		available = availableBalance.Uint64()
	}
	if lockedBalance.IsUint64() {
		locked = lockedBalance.Uint64()
	}

	balanceInfo := &types.BalanceInfo{
		Address:   &transaction.Address{RawHash: address},
		TokenID:   tokenID,
		Total:     total,
		Available: available,
		Locked:    locked,
		Pending:   0,
	}

	if s.logger != nil {
		s.logger.Debugf("账户余额: address=%x, total=%s, available=%s, locked=%s",
			address, totalBalance.String(), availableBalance.String(), lockedBalance.String())
	}

	return balanceInfo, nil
}

// matchesTokenID 检查UTXO是否匹配指定的代币ID
func matchesTokenID(utxo *utxo.UTXO, tokenID []byte) bool {
	// 获取缓存的输出
	cachedOutput := utxo.GetCachedOutput()
	if cachedOutput == nil {
		return false
	}

	// 获取资产输出
	assetOutput := cachedOutput.GetAsset()
	if assetOutput == nil {
		return false
	}

	// 检查代币类型
	if tokenID == nil || len(tokenID) == 0 {
		// 查询原生代币
		return assetOutput.GetNativeCoin() != nil
	}

	// 查询合约代币
	contractToken := assetOutput.GetContractToken()
	if contractToken == nil {
		return false
	}

	// 比较合约地址
	return string(contractToken.ContractAddress) == string(tokenID)
}

// extractAmount 从UTXO中提取金额
func extractAmount(utxo *utxo.UTXO) *big.Int {
	// 获取缓存的输出
	cachedOutput := utxo.GetCachedOutput()
	if cachedOutput == nil {
		return nil
	}

	// 获取资产输出
	assetOutput := cachedOutput.GetAsset()
	if assetOutput == nil {
		return nil
	}

	// 提取金额
	var amountStr string
	if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
		amountStr = nativeCoin.Amount
	} else if contractToken := assetOutput.GetContractToken(); contractToken != nil {
		amountStr = contractToken.Amount
	}

	if amountStr == "" {
		return big.NewInt(0)
	}

	// 解析金额
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return big.NewInt(0)
	}

	return amount
}

// 编译时检查接口实现
var _ interfaces.InternalAccountQuery = (*Service)(nil)

