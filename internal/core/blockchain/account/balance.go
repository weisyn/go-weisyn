// Package account 余额查询实现
//
// 💰 **余额查询核心实现 (Balance Query Implementation)**
//
// 本文件实现账户余额查询的核心逻辑，包括：
// - 平台主币余额计算和聚合
// - 自定义代币余额查询和统计
// - 全量代币资产视图构建
//
// 🎯 **核心功能**
// - UTXO聚合：将分散的UTXO聚合为统一的余额视图
// - 多币种支持：同时支持平台主币和各种自定义代币
// - 状态计算：准确区分可用、锁定、待确认等不同状态的余额
package account

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
//                              平台主币余额查询
// ============================================================================

// getPlatformBalance 获取平台主币余额
//
// 🎯 **平台主币余额查询核心实现**
//
// 实现流程：
// 1. 查询地址相关的所有平台主币UTXO
// 2. 按状态分类统计（可用/锁定/待确认）
// 3. 构建完整的余额信息对象
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//
// 返回：
//
//	*types.BalanceInfo: 平台主币余额信息
//	error: 查询错误
func (m *Manager) getPlatformBalance(ctx context.Context, address []byte) (*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询平台主币余额 - address: %x", address)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 构建地址对象
	addressObj := &transaction.Address{RawHash: address}

	// 🔥 实现核心逻辑：查询Asset类型的UTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	assetUTXOs, err := m.utxoManager.GetUTXOsByAddress(ctx, address, &assetCategory, true)
	if err != nil {
		m.logger.Errorf("查询Asset UTXO失败: %v", err)
		return nil, fmt.Errorf("查询Asset UTXO失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("找到 %d 个Asset UTXO", len(assetUTXOs))
	}

	// 🔥 实现余额聚合：处理原生代币UTXO
	var availableBalance uint64 = 0
	var lockedBalance uint64 = 0
	var utxoCount uint32 = 0

	for _, utxoObj := range assetUTXOs {
		// 只处理Asset类型的UTXO
		if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 从UTXO内容中提取原生代币金额
		amount, err := m.extractNativeCoinAmount(utxoObj)
		if err != nil {
			m.logger.Warnf("无法提取UTXO金额，跳过: %v", err)
			continue
		}

		// 如果金额为0，跳过（这是非原生代币UTXO）
		if amount == 0 {
			continue
		}

		// 根据UTXO状态分类统计
		switch utxoObj.GetStatus() {
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE:
			availableBalance += amount
			utxoCount++
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED:
			// 被引用的UTXO暂时不可用，算作锁定余额
			lockedBalance += amount
			utxoCount++
		default:
			// 其他状态（如CONSUMED）不计入余额
			continue
		}
	}

	// 🔥 修正：查询待确认余额变动（支出和收入都要考虑）
	pendingEntries, err := m.getPendingBalances(ctx, address, nil)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("获取待确认余额失败: %v", err)
		}
		// 继续执行，不影响已确认余额查询
	}

	// 🔥 简化：使用简化版pending余额计算
	pendingBalance := m.calculateSimplePendingBalance(pendingEntries)

	// 🔥 修正：余额总计公式 - Total = available + locked （不包含pending）
	balanceInfo := &types.BalanceInfo{
		Address:     addressObj,
		TokenID:     nil, // nil表示原生币
		Available:   availableBalance,
		Locked:      lockedBalance,
		Pending:     pendingBalance,
		Total:       availableBalance + lockedBalance,
		UTXOCount:   utxoCount,
		LastUpdated: getCurrentTime(),
	}

	if m.logger != nil {
		m.logger.Debugf("平台主币余额查询完成 - address: %x, available: %d, locked: %d, utxos: %d",
			address, availableBalance, lockedBalance, utxoCount)
	}

	return balanceInfo, nil
}

// ============================================================================
//                              指定代币余额查询
// ============================================================================

// getTokenBalance 获取指定代币余额
//
// 🎯 **特定代币余额查询核心实现**
//
// 实现流程：
// 1. 查询地址相关的指定代币UTXO
// 2. 按状态分类统计（可用/锁定/待确认）
// 3. 查询代币元信息（名称、符号、精度）
// 4. 构建完整的代币余额信息
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//	tokenID: 代币标识符
//
// 返回：
//
//	*types.BalanceInfo: 代币余额信息
//	error: 查询错误
func (m *Manager) getTokenBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询代币余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}
	if len(tokenID) == 0 {
		return nil, fmt.Errorf("代币ID不能为空")
	}

	// 构建地址对象
	addressObj := &transaction.Address{RawHash: address}

	// 🔥 实现核心逻辑：查询Asset类型的UTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	assetUTXOs, err := m.utxoManager.GetUTXOsByAddress(ctx, address, &assetCategory, true)
	if err != nil {
		m.logger.Errorf("查询Asset UTXO失败: %v", err)
		return nil, fmt.Errorf("查询Asset UTXO失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("找到 %d 个Asset UTXO", len(assetUTXOs))
	}

	// 🔥 实现余额聚合：处理指定代币UTXO
	var availableBalance uint64 = 0
	var lockedBalance uint64 = 0
	var utxoCount uint32 = 0

	for _, utxoObj := range assetUTXOs {
		// 只处理Asset类型的UTXO
		if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 从UTXO内容中提取指定代币金额
		extractedTokenID, amount, err := m.extractTokenAmount(utxoObj, tokenID)
		if err != nil {
			m.logger.Warnf("无法提取代币UTXO金额，跳过: %v", err)
			continue
		}

		// 如果不是目标代币或金额为0，跳过
		if extractedTokenID == nil || amount == 0 || !bytesEqual(extractedTokenID, tokenID) {
			continue
		}

		// 根据UTXO状态分类统计
		switch utxoObj.GetStatus() {
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE:
			availableBalance += amount
			utxoCount++
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED:
			// 被引用的UTXO暂时不可用，算作锁定余额
			lockedBalance += amount
			utxoCount++
		default:
			// 其他状态（如CONSUMED）不计入余额
			continue
		}
	}

	// 🔥 修正：查询待确认余额变动（支出和收入都要考虑）
	pendingEntries, err := m.getPendingBalances(ctx, address, tokenID)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("获取待确认余额失败: %v", err)
		}
		// 继续执行，不影响已确认余额查询
	}

	// 🔥 简化：使用简化版pending余额计算
	pendingBalance := m.calculateSimplePendingBalance(pendingEntries)

	// 🔥 修正：余额总计公式 - Total = available + locked （不包含pending）
	balanceInfo := &types.BalanceInfo{
		Address:     addressObj,
		TokenID:     tokenID,
		Available:   availableBalance,
		Locked:      lockedBalance,
		Pending:     pendingBalance,
		Total:       availableBalance + lockedBalance,
		UTXOCount:   utxoCount,
		LastUpdated: getCurrentTime(),
	}

	if m.logger != nil {
		m.logger.Debugf("代币余额查询完成 - address: %x, tokenID: %x, available: %d, locked: %d, utxos: %d",
			address, tokenID, availableBalance, lockedBalance, utxoCount)
	}

	return balanceInfo, nil
}

// ============================================================================
//                              全量代币余额查询
// ============================================================================

// getAllTokenBalances 获取账户所有代币余额
//
// 🎯 **全量代币资产视图构建**
//
// 实现流程：
// 1. 查询地址的所有UTXO
// 2. 按代币类型分组统计
// 3. 为每种代币构建余额信息
// 4. 构建完整的资产配置映射
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//
// 返回：
//
//	map[string]*types.BalanceInfo: 代币余额映射
//	  键: 代币标识符（""表示平台主币）
//	  值: 对应的余额信息
//	error: 查询错误
func (m *Manager) getAllTokenBalances(ctx context.Context, address []byte) (map[string]*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询所有代币余额 - address: %x", address)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 构建地址对象
	addressObj := &transaction.Address{RawHash: address}

	// 🔥 实现核心逻辑：查询Asset类型的UTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	assetUTXOs, err := m.utxoManager.GetUTXOsByAddress(ctx, address, &assetCategory, true)
	if err != nil {
		m.logger.Errorf("查询Asset UTXO失败: %v", err)
		return nil, fmt.Errorf("查询Asset UTXO失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("找到 %d 个Asset UTXO", len(assetUTXOs))
	}

	// 🔥 实现余额聚合：按代币类型分组统计
	tokenBalances := make(map[string]*tokenBalanceAccumulator)

	for _, utxoObj := range assetUTXOs {
		// 只处理Asset类型的UTXO
		if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 尝试提取原生币金额
		nativeAmount, err := m.extractNativeCoinAmount(utxoObj)
		if err == nil && nativeAmount > 0 {
			// 这是原生币UTXO
			nativeKey := "" // 原生币使用空字符串作key
			if tokenBalances[nativeKey] == nil {
				tokenBalances[nativeKey] = &tokenBalanceAccumulator{
					tokenID: nil, // 原生币tokenID为nil
				}
			}
			m.accumulateBalance(tokenBalances[nativeKey], utxoObj, nativeAmount)
			continue
		}

		// 尝试提取合约代币金额
		tokenID, tokenAmount, err := m.extractTokenAmount(utxoObj, nil) // nil表示查询所有代币
		if err == nil && tokenID != nil && tokenAmount > 0 {
			// 这是合约代币UTXO
			tokenKey := fmt.Sprintf("%x", tokenID) // 使用十六进制字符串作key
			if tokenBalances[tokenKey] == nil {
				tokenBalances[tokenKey] = &tokenBalanceAccumulator{
					tokenID: tokenID,
				}
			}
			m.accumulateBalance(tokenBalances[tokenKey], utxoObj, tokenAmount)
		}
	}

	// 🔥 构建最终余额映射
	balances := make(map[string]*types.BalanceInfo)

	for key, accumulator := range tokenBalances {
		// 🔥 修正：查询待确认余额变动（支出和收入都要考虑）
		pendingEntries, err := m.getPendingBalances(ctx, address, accumulator.tokenID)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("获取待确认余额失败: %v", err)
			}
			// 继续执行，不影响已确认余额查询
		}

		// 🔥 简化：使用简化版pending余额计算
		pendingBalance := m.calculateSimplePendingBalance(pendingEntries)

		// 🔥 修正：余额总计公式 - Total = available + locked （不包含pending）
		// pending仅作为用户参考，不影响实际总余额
		balanceInfo := &types.BalanceInfo{
			Address:     addressObj,
			TokenID:     accumulator.tokenID,
			Available:   accumulator.availableBalance,
			Locked:      accumulator.lockedBalance,
			Pending:     pendingBalance,
			Total:       accumulator.availableBalance + accumulator.lockedBalance,
			UTXOCount:   accumulator.utxoCount,
			LastUpdated: getCurrentTime(),
		}

		balances[key] = balanceInfo
	}

	if m.logger != nil {
		m.logger.Debugf("所有代币余额查询完成 - address: %x, tokenCount: %d",
			address, len(balances))
	}

	return balances, nil
}

// ============================================================================
//                              私有辅助方法实现
// ============================================================================

// tokenBalanceAccumulator 代币余额累加器
//
// 🔢 **余额聚合数据结构**
//
// 用于在遍历UTXO时累积同一代币的余额信息。
type tokenBalanceAccumulator struct {
	tokenID          []byte // 代币ID（nil表示原生币）
	availableBalance uint64 // 可用余额
	lockedBalance    uint64 // 锁定余额
	utxoCount        uint32 // UTXO数量
}

// extractNativeCoinAmount 从Asset UTXO中提取原生币金额
//
// 🔍 **原生币金额提取核心逻辑**
//
// 解析Asset UTXO的内容，提取原生币（平台主币）的金额。
// 只处理NativeCoinAsset类型，忽略ContractTokenAsset。
//
// 参数：
//
//	utxo: Asset类型的UTXO
//
// 返回：
//
//	uint64: 原生币金额（0表示非原生币UTXO）
//	error: 解析错误
func (m *Manager) extractNativeCoinAmount(utxoObj *utxo.UTXO) (uint64, error) {
	// 检查UTXO类别
	if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
		return 0, fmt.Errorf("UTXO不是Asset类型")
	}

	// 从UTXO获取TxOutput内容
	txOutput := utxoObj.GetCachedOutput()
	if txOutput == nil {
		return 0, fmt.Errorf("UTXO没有缓存的TxOutput内容")
	}

	// 检查是否为AssetOutput
	assetOutput, ok := txOutput.OutputContent.(*transaction.TxOutput_Asset)
	if !ok {
		return 0, fmt.Errorf("TxOutput不是Asset类型")
	}

	if assetOutput.Asset == nil {
		return 0, fmt.Errorf("AssetOutput内容为空")
	}

	// 检查是否为原生币
	nativeCoin, ok := assetOutput.Asset.AssetContent.(*transaction.AssetOutput_NativeCoin)
	if !ok {
		// 这是ContractToken，返回0表示非原生币
		return 0, nil
	}

	if nativeCoin.NativeCoin == nil {
		return 0, fmt.Errorf("NativeCoin内容为空")
	}

	// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
	amountStr := nativeCoin.NativeCoin.Amount
	if amountStr == "" {
		return 0, nil
	}

	amount, err := utils.ParseAmountSafely(amountStr)
	if err != nil {
		return 0, fmt.Errorf("解析原生币金额失败: %w", err)
	}

	return amount, nil
}

// extractTokenAmount 从Asset UTXO中提取合约代币金额和代币ID
//
// 🪙 **合约代币金额提取核心逻辑**
//
// 解析Asset UTXO的内容，提取合约代币的金额和代币标识。
// 只处理ContractTokenAsset类型，忽略NativeCoinAsset。
//
// 参数：
//
//	utxoObj: Asset类型的UTXO
//	targetTokenID: 目标代币ID（nil表示查询所有代币）
//
// 返回：
//
//	tokenID: 代币标识
//	amount: 代币金额
//	error: 解析错误
func (m *Manager) extractTokenAmount(utxoObj *utxo.UTXO, targetTokenID []byte) ([]byte, uint64, error) {
	// 检查UTXO类别
	if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
		return nil, 0, fmt.Errorf("UTXO不是Asset类型")
	}

	// 从UTXO获取TxOutput内容
	txOutput := utxoObj.GetCachedOutput()
	if txOutput == nil {
		return nil, 0, fmt.Errorf("UTXO没有缓存的TxOutput内容")
	}

	// 检查是否为AssetOutput
	assetOutput, ok := txOutput.OutputContent.(*transaction.TxOutput_Asset)
	if !ok {
		return nil, 0, fmt.Errorf("TxOutput不是Asset类型")
	}

	if assetOutput.Asset == nil {
		return nil, 0, fmt.Errorf("AssetOutput内容为空")
	}

	// 检查是否为合约代币
	contractToken, ok := assetOutput.Asset.AssetContent.(*transaction.AssetOutput_ContractToken)
	if !ok {
		// 这是NativeCoin，不是我们要找的
		return nil, 0, nil
	}

	if contractToken.ContractToken == nil {
		return nil, 0, fmt.Errorf("ContractToken内容为空")
	}

	// 提取代币标识符
	var tokenID []byte
	switch identifier := contractToken.ContractToken.GetTokenIdentifier().(type) {
	case *transaction.ContractTokenAsset_FungibleClassId:
		tokenID = identifier.FungibleClassId
	case *transaction.ContractTokenAsset_NftUniqueId:
		tokenID = identifier.NftUniqueId
	case *transaction.ContractTokenAsset_SemiFungibleId:
		if identifier.SemiFungibleId != nil {
			tokenID = identifier.SemiFungibleId.BatchId // 使用批次ID作为代币ID
		}
	default:
		return nil, 0, fmt.Errorf("未知的代币标识符类型")
	}

	// 如果指定了targetTokenID，检查是否匹配
	if targetTokenID != nil && !bytesEqual(tokenID, targetTokenID) {
		return nil, 0, nil // 不匹配，返回0金额
	}

	// 解析金额字符串
	amountStr := contractToken.ContractToken.Amount
	if amountStr == "" {
		return tokenID, 0, nil
	}

	// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
	amount, err := utils.ParseAmountSafely(amountStr)
	if err != nil {
		return tokenID, 0, fmt.Errorf("解析合约代币金额失败: %w", err)
	}

	return tokenID, amount, nil
}

// accumulateBalance 累积余额到累加器
//
// 🔢 **余额累积核心逻辑**
//
// 根据UTXO状态将金额累积到相应的余额分类中。
//
// 参数：
//
//	accumulator: 余额累加器
//	utxoObj: UTXO对象
//	amount: 金额
func (m *Manager) accumulateBalance(accumulator *tokenBalanceAccumulator, utxoObj *utxo.UTXO, amount uint64) {
	if amount == 0 {
		return
	}

	// 根据UTXO状态分类累积
	switch utxoObj.GetStatus() {
	case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE:
		accumulator.availableBalance += amount
		accumulator.utxoCount++
	case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED:
		// 被引用的UTXO暂时不可用，算作锁定余额
		accumulator.lockedBalance += amount
		accumulator.utxoCount++
	default:
		// 其他状态（如CONSUMED）不计入余额
		return
	}
}

// getCurrentTime 获取当前时间
//
// 🕒 **时间获取工具方法**
//
// 获取当前UTC时间，用于设置余额信息的更新时间。
//
// 返回：
//
//	time.Time: 当前UTC时间
func getCurrentTime() time.Time {
	return time.Now().UTC()
}

// calculateSimplePendingBalance 简化pending余额计算
//
// 🎯 **简化版pending计算实现**
//
// 将复杂的 pendingIn/pendingOut 计算简化为直接的净变化计算，
// 明确pending的语义：仅作为用户参考的"预估变化"，不影响总余额。
//
// 实现要点：
// - pending = 所有待确认变动的净值
// - 正数表示预期增加，负数表示预期减少
// - 负数时显示为0（实际可用余额通过 GetEffectiveBalance 获取）
// - 消除原来复杂的 pendingIn/pendingOut 分离逻辑
//
// 参数：
//   - pendingEntries: 待确认余额变动条目列表
//
// 返回：
//   - uint64: 简化计算的pending余额
func (m *Manager) calculateSimplePendingBalance(pendingEntries []*types.PendingBalanceEntry) uint64 {
	var netPending int64 = 0

	// 简单求和所有待确认变动
	for _, entry := range pendingEntries {
		netPending += entry.Amount
	}

	// 如果净变化为负数，显示为0（用户界面友好）
	// 实际的有效余额计算应使用 GetEffectiveBalance 接口
	if netPending < 0 {
		if m.logger != nil {
			m.logger.Debugf("待确认净变化为负数(%.6f)，pending余额显示为0",
				float64(netPending)/1e9)
		}
		return 0
	}

	return uint64(netPending)
}

// bytesEqual 比较两个字节数组是否相等
//
// 🔍 **字节数组比较工具方法**
//
// 安全比较两个字节数组，处理nil情况。
//
// 参数：
//
//	a, b: 要比较的字节数组
//
// 返回：
//
//	bool: 是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============================================================================
//                              辅助工具方法
// ============================================================================

// aggregateUTXOsByToken 按代币类型聚合UTXO
//
// 🛠️ **UTXO聚合工具方法**
//
// 将同一地址的UTXO按代币类型进行分组统计，为余额计算提供基础数据。
//
// 参数：
//
//	utxos: UTXO列表
//
// 返回：
//
//	map[string]interface{}: 按代币分组的UTXO集合
//	error: 聚合错误
func (m *Manager) aggregateUTXOsByToken(utxos interface{}) (map[string]interface{}, error) {
	// TODO: 实现UTXO按代币类型聚合逻辑
	// 1. 遍历所有UTXO
	// 2. 按tokenID进行分组
	// 3. 计算每种代币的数量统计
	// 4. 返回聚合结果

	if m.logger != nil {
		m.logger.Debugf("开始聚合UTXO按代币类型")
	}

	// 临时实现
	aggregated := make(map[string]interface{})

	return aggregated, nil
}

// calculateBalanceStates 计算余额状态
//
// 🧮 **余额状态计算工具**
//
// 分析UTXO的锁定状态，计算可用余额、锁定余额等不同状态的金额。
//
// 参数：
//
//	tokenUTXOs: 特定代币的UTXO列表
//
// 返回：
//
//	available: 可用余额
//	locked: 锁定余额
//	pending: 待确认余额
//	error: 计算错误
func (m *Manager) calculateBalanceStates(tokenUTXOs interface{}) (string, string, string, error) {
	// TODO: 实现余额状态计算逻辑
	// 1. 遍历代币UTXO
	// 2. 检查每个UTXO的锁定条件
	// 3. 根据锁定状态分类累加
	// 4. 查询内存池获取待确认金额
	// 5. 返回各状态余额

	if m.logger != nil {
		m.logger.Debugf("开始计算余额状态")
	}

	// 临时实现 - 返回零值
	return "0", "0", "0", nil
}
