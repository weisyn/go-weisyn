// Package selector 提供 UTXO 选择策略实现
//
// 🎯 **设计定位**：TX 内部实现，不暴露公共接口
//
// 📋 **核心职责**：
// - 基于地址和金额需求，选择合适的 UTXO 集合
// - 实现贪心算法：按金额从小到大排序，优先选择接近目标金额的 UTXO
// - 支持多资产选择：原生币和合约代币（FungibleToken）
// - 自动计算找零：Σ(选中的 UTXO) - Σ(目标金额) = 找零
//
// ⚠️ **核心约束**：
// - 只选择 Available 状态的 UTXO
// - 只处理 AssetOutput 类型的 UTXO
// - 不暴露为公共接口（TX 内部实现细节）
//
// 🔄 **使用流程**：
// 1. SelectUTXOs(...) → 返回选中的 UTXO 列表
// 2. CalculateChange(...) → 计算每个资产的找零金额
package selector

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// AssetRequest 表示单个资产的请求
type AssetRequest struct {
	// TokenID 资产标识符
	// - 原生币：使用空字符串 "" 或 "native"
	// - 合约代币：使用 "contract_address:class_id" 格式
	TokenID string

	// Amount 需要的金额（字符串格式，支持大数）
	Amount string

	// ContractAddress 合约地址（可选，仅合约代币需要）
	ContractAddress []byte

	// ClassID 代币类别 ID（可选，仅合约代币需要）
	ClassID []byte
}

// SelectionResult 选择结果
type SelectionResult struct {
	// SelectedUTXOs 选中的 UTXO 列表
	SelectedUTXOs []*utxopb.UTXO

	// ChangeAmounts 找零金额（按 TokenID 分组）
	ChangeAmounts map[string]string

	// TotalSelected 选中的总金额（按 TokenID 分组）
	TotalSelected map[string]string
}

// Service UTXO 选择器服务
type Service struct {
	utxoMgr persistence.UTXOQuery
	logger  log.Logger
}

// NewService 创建 UTXO 选择器服务
func NewService(
	utxoMgr persistence.UTXOQuery,
	logger log.Logger,
) *Service {
	return &Service{
		utxoMgr: utxoMgr,
		logger:  logger,
	}
}

// SelectUTXOs 选择满足多资产需求的 UTXO 集合
//
// 参数：
//   - ctx: 上下文
//   - ownerAddress: UTXO 所有者地址
//   - requests: 多资产请求列表
//
// 返回：
//   - *SelectionResult: 选择结果（包含 UTXO 列表和找零金额）
//   - error: 余额不足或查询失败的错误
func (s *Service) SelectUTXOs(
	ctx context.Context,
	ownerAddress []byte,
	requests []*AssetRequest,
) (*SelectionResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("请求列表不能为空")
	}

	// 1. 查询所有可用的 Asset UTXO
	assetCategory := utxopb.UTXOCategory_UTXO_CATEGORY_ASSET
	availableUTXOs, err := s.utxoMgr.GetUTXOsByAddress(ctx, ownerAddress, &assetCategory, true)
	if err != nil {
		return nil, fmt.Errorf("查询 UTXO 失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("[UTXOSelector] 查询到 %d 个可用 UTXO", len(availableUTXOs))
	}

	// 2. 按资产分组
	utxosByAsset := s.groupUTXOsByAsset(availableUTXOs)

	// 3. 对每个资产请求，执行贪心选择
	selectedUTXOs := make([]*utxopb.UTXO, 0)
	totalSelected := make(map[string]string)
	requiredAmounts := make(map[string]*big.Int)

	for _, req := range requests {
		// 解析目标金额
		targetAmount, ok := new(big.Int).SetString(req.Amount, 10)
		if !ok {
			return nil, fmt.Errorf("无效的金额格式: %s", req.Amount)
		}

		if targetAmount.Sign() <= 0 {
			return nil, fmt.Errorf("无效的金额: %s", req.Amount)
		}

		requiredAmounts[req.TokenID] = targetAmount

		// 获取此资产的 UTXO 列表
		utxos, ok := utxosByAsset[req.TokenID]
		if !ok || len(utxos) == 0 {
			return nil, fmt.Errorf("资产 %s 余额不足（没有可用 UTXO）", req.TokenID)
		}

		// 贪心算法选择 UTXO
		selected, selectedTotal, err := s.greedySelect(utxos, targetAmount)
		if err != nil {
			return nil, fmt.Errorf("资产 %s 选择失败: %w", req.TokenID, err)
		}

		selectedUTXOs = append(selectedUTXOs, selected...)
		totalSelected[req.TokenID] = selectedTotal.String()

		if s.logger != nil {
			s.logger.Debugf("[UTXOSelector] 资产 %s: 需要 %s, 选中 %s, 选中 %d 个 UTXO",
				req.TokenID, targetAmount.String(), selectedTotal.String(), len(selected))
		}
	}

	// 4. 计算找零
	changeAmounts := make(map[string]string)
	for tokenID, selected := range totalSelected {
		selectedBig := new(big.Int)
		selectedBig.SetString(selected, 10)

		required := requiredAmounts[tokenID]
		change := new(big.Int).Sub(selectedBig, required)

		if change.Sign() > 0 {
			changeAmounts[tokenID] = change.String()
		}
	}

	return &SelectionResult{
		SelectedUTXOs: selectedUTXOs,
		ChangeAmounts: changeAmounts,
		TotalSelected: totalSelected,
	}, nil
}

// groupUTXOsByAsset 按资产 ID 分组 UTXO
func (s *Service) groupUTXOsByAsset(utxos []*utxopb.UTXO) map[string][]*utxopb.UTXO {
	grouped := make(map[string][]*utxopb.UTXO)

	for _, u := range utxos {
		// 提取 AssetOutput
		txOutput := u.GetCachedOutput()
		if txOutput == nil {
			continue
		}

		assetOutput := txOutput.GetAsset()
		if assetOutput == nil {
			continue
		}

		// 提取 TokenID 和金额
		tokenID, _, err := s.extractAssetInfo(assetOutput)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("[UTXOSelector] 跳过无效 UTXO: %v", err)
			}
			continue
		}

		grouped[tokenID] = append(grouped[tokenID], u)
	}

	return grouped
}

// extractAssetInfo 从 AssetOutput 中提取资产信息
func (s *Service) extractAssetInfo(assetOutput *transaction.AssetOutput) (string, *big.Int, error) {
	switch asset := assetOutput.AssetContent.(type) {
	case *transaction.AssetOutput_NativeCoin:
		// 原生币
		amount, ok := new(big.Int).SetString(asset.NativeCoin.Amount, 10)
		if !ok {
			return "", nil, fmt.Errorf("原生币金额格式无效: %s", asset.NativeCoin.Amount)
		}
		return "native", amount, nil

	case *transaction.AssetOutput_ContractToken:
		// 合约代币（仅处理 FungibleToken）
		token := asset.ContractToken
		if token.GetFungibleClassId() == nil {
			return "", nil, fmt.Errorf("不支持的代币类型（仅支持 FungibleToken）")
		}

		// TokenID 格式：contract_address:class_id
		tokenID := fmt.Sprintf("%x:%x", token.ContractAddress, token.GetFungibleClassId())
		amount, ok := new(big.Int).SetString(token.Amount, 10)
		if !ok {
			return "", nil, fmt.Errorf("合约代币金额格式无效: %s", token.Amount)
		}
		return tokenID, amount, nil

	default:
		return "", nil, fmt.Errorf("未知的资产类型")
	}
}

// greedySelect 贪心算法选择 UTXO
//
// 策略：
// 1. 按金额从小到大排序
// 2. 优先选择接近目标金额的 UTXO
// 3. 如果没有单个 UTXO 满足，则累加多个 UTXO
func (s *Service) greedySelect(utxos []*utxopb.UTXO, targetAmount *big.Int) ([]*utxopb.UTXO, *big.Int, error) {
	// 1. 提取金额并排序
	type utxoWithAmount struct {
		utxo   *utxopb.UTXO
		amount *big.Int
	}

	utxoList := make([]utxoWithAmount, 0, len(utxos))
	for _, u := range utxos {
		txOutput := u.GetCachedOutput()
		if txOutput == nil {
			continue
		}

		assetOutput := txOutput.GetAsset()
		if assetOutput == nil {
			continue
		}

		_, amount, err := s.extractAssetInfo(assetOutput)
		if err != nil {
			continue
		}

		utxoList = append(utxoList, utxoWithAmount{
			utxo:   u,
			amount: amount,
		})
	}

	// 按金额从小到大排序（确定性排序：金额 → txid → index）
	sort.Slice(utxoList, func(i, j int) bool {
		// Level 1: 按金额升序
		amountCmp := utxoList[i].amount.Cmp(utxoList[j].amount)
		if amountCmp != 0 {
			return amountCmp < 0
		}

		// Level 2: 若金额相同，按 txid 字节序升序
		txidCmp := bytes.Compare(utxoList[i].utxo.Outpoint.TxId, utxoList[j].utxo.Outpoint.TxId)
		if txidCmp != 0 {
			return txidCmp < 0
		}

		// Level 3: 若 txid 也相同（理论不可能），按 index 升序
		return utxoList[i].utxo.Outpoint.OutputIndex < utxoList[j].utxo.Outpoint.OutputIndex
	})

	// 2. 贪心选择策略：优先选择最接近目标金额的单个 UTXO
	for _, item := range utxoList {
		if item.amount.Cmp(targetAmount) >= 0 {
			// 找到一个 UTXO 就足够了
			return []*utxopb.UTXO{item.utxo}, item.amount, nil
		}
	}

	// 3. 如果没有单个 UTXO 满足，则累加多个 UTXO
	selected := make([]*utxopb.UTXO, 0)
	selectedTotal := new(big.Int)

	for _, item := range utxoList {
		selected = append(selected, item.utxo)
		selectedTotal.Add(selectedTotal, item.amount)

		if selectedTotal.Cmp(targetAmount) >= 0 {
			// 已经满足目标金额
			return selected, selectedTotal, nil
		}
	}

	// 4. 所有 UTXO 加起来也不够
	return nil, nil, fmt.Errorf("余额不足：需要 %s，可用 %s", targetAmount.String(), selectedTotal.String())
}
