// Package planner 提供交易规划服务（UTXO 选择 + 交易构建）
//
// 🎯 **设计定位**：TX 内部辅助组件，协调 Selector 和 Builder
//
// 📋 **核心职责**：
// - 根据业务需求（如转账），自动选择 UTXO
// - 生成找零输出
// - 调用 Builder 构建 ComposedTx
// - 保持 Builder 纯装配特性（Builder 不做业务逻辑）
//
// ⚠️ **核心约束**：
// - Planner 是辅助组件，不是正式 Type-state
// - Planner 处理"Plan"阶段（UTXO 选择 + 找零计算）
// - Builder 仍然保持纯装配（只做 Add* 操作）
//
// 🔄 **使用流程**：
// 1. PlanAndBuildTransfer(...) → 选择 UTXO + 构建交易
// 2. 返回 ComposedTx（可以继续 Type-state 流程）
package planner

import (
	"context"
	"fmt"
	"math/big"

	"github.com/weisyn/v1/internal/core/tx/builder"
	"github.com/weisyn/v1/internal/core/tx/selector"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// Service 交易规划服务
type Service struct {
	selector     *selector.Service
	draftService tx.TransactionDraftService
	logger       log.Logger
}

// NewService 创建交易规划服务
func NewService(
	selector *selector.Service,
	draftService tx.TransactionDraftService,
	logger log.Logger,
) *Service {
	return &Service{
		selector:     selector,
		draftService: draftService,
		logger:       logger,
	}
}

// TransferRequest 转账请求
type TransferRequest struct {
	// FromAddress 发送方地址
	FromAddress []byte

	// ToAddress 接收方地址
	ToAddress []byte

	// Amount 转账金额
	Amount string

	// ContractAddress 合约地址（可选，仅合约代币需要）
	ContractAddress []byte

	// ClassID 代币类别 ID（可选，仅合约代币需要）
	ClassID []byte

	// LockingCondition 输出锁定条件
	LockingCondition *transaction.LockingCondition

	// ChangeLockingCondition 找零锁定条件（可选，默认与输入相同）
	ChangeLockingCondition *transaction.LockingCondition

	// Nonce 账户 nonce（防重放攻击）
	Nonce uint64
}

// PlanAndBuildTransfer 规划并构建转账交易
//
// 参数：
//   - ctx: 上下文
//   - req: 转账请求
//
// 返回：
//   - *types.ComposedTx: 组装完成的交易（可以继续 Type-state 流程）
//   - error: 规划或构建失败的错误
func (s *Service) PlanAndBuildTransfer(
	ctx context.Context,
	req *TransferRequest,
) (*types.ComposedTx, error) {
	if req == nil {
		return nil, fmt.Errorf("转账请求不能为空")
	}

	if s.logger != nil {
		// 安全地截取地址前缀用于日志（避免数组越界）
		fromPrefix := safeSlicePrefix(req.FromAddress, 8)
		toPrefix := safeSlicePrefix(req.ToAddress, 8)
		s.logger.Infof("[Planner] 开始规划转账: from=%x, to=%x, amount=%s",
			fromPrefix, toPrefix, req.Amount)
	}

	// 1. 确定 TokenID
	var tokenID string
	if req.ContractAddress == nil {
		tokenID = "native"
	} else {
		tokenID = fmt.Sprintf("%x:%x", req.ContractAddress, req.ClassID)
	}

	// 2. 使用 Selector 选择 UTXO
	assetRequests := []*selector.AssetRequest{
		{
			TokenID:         tokenID,
			Amount:          req.Amount,
			ContractAddress: req.ContractAddress,
			ClassID:         req.ClassID,
		},
	}

	selectionResult, err := s.selector.SelectUTXOs(ctx, req.FromAddress, assetRequests)
	if err != nil {
		return nil, fmt.Errorf("UTXO 选择失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("[Planner] 选中 %d 个 UTXO，找零: %v",
			len(selectionResult.SelectedUTXOs), selectionResult.ChangeAmounts)
	}

	// 3. 使用 Builder 构建交易（每次调用创建独立的 Builder 实例，避免状态串扰）
	builderSvc := builder.NewService(s.draftService)
	// 3.1 设置 nonce
	builderSvc.SetNonce(req.Nonce)

	// 3.2 添加所有选中的 UTXO 作为输入
	for _, utxo := range selectionResult.SelectedUTXOs {
		builderSvc.AddInput(utxo.Outpoint, false)
	}

	// 3.3 添加转账输出
	builderSvc.AddAssetOutput(
		req.ToAddress,
		req.Amount,
		req.ContractAddress,
		req.LockingCondition,
	)

	// 3.4 添加找零输出（如果有）
	if changeAmount, ok := selectionResult.ChangeAmounts[tokenID]; ok {
		changeLock := req.ChangeLockingCondition
		if changeLock == nil {
			// 默认找零回发送方，使用相同的锁定条件
			changeLock = req.LockingCondition
		}

		builderSvc.AddAssetOutput(
			req.FromAddress, // 找零回发送方
			changeAmount,
			req.ContractAddress,
			changeLock,
		)

		if s.logger != nil {
			s.logger.Debugf("[Planner] 添加找零输出: amount=%s", changeAmount)
		}
	}

	// 3.5 构建 ComposedTx
	composedTx, err := builderSvc.Build()
	if err != nil {
		return nil, fmt.Errorf("构建交易失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("[Planner] 交易构建完成: inputs=%d, outputs=%d",
			len(composedTx.Tx.Inputs), len(composedTx.Tx.Outputs))
	}

	return composedTx, nil
}

// MultiAssetTransferRequest 多资产转账请求
type MultiAssetTransferRequest struct {
	// FromAddress 发送方地址
	FromAddress []byte

	// Outputs 多个输出（支持多资产、多接收方）
	Outputs []*TransferOutput

	// LockingCondition 默认锁定条件（用于找零）
	DefaultLockingCondition *transaction.LockingCondition

	// Nonce 账户 nonce
	Nonce uint64
}

// TransferOutput 单个转账输出
type TransferOutput struct {
	// ToAddress 接收方地址
	ToAddress []byte

	// Amount 金额
	Amount string

	// ContractAddress 合约地址（可选）
	ContractAddress []byte

	// ClassID 代币类别 ID（可选）
	ClassID []byte

	// LockingCondition 锁定条件
	LockingCondition *transaction.LockingCondition
}

// PlanAndBuildMultiAssetTransfer 规划并构建多资产转账交易
//
// 参数：
//   - ctx: 上下文
//   - req: 多资产转账请求
//
// 返回：
//   - *types.ComposedTx: 组装完成的交易
//   - error: 规划或构建失败的错误
func (s *Service) PlanAndBuildMultiAssetTransfer(
	ctx context.Context,
	req *MultiAssetTransferRequest,
) (*types.ComposedTx, error) {
	if req == nil {
		return nil, fmt.Errorf("转账请求不能为空")
	}

	if len(req.Outputs) == 0 {
		return nil, fmt.Errorf("输出列表不能为空")
	}

	if s.logger != nil {
		// 安全地截取地址前缀用于日志（避免数组越界）
		fromPrefix := safeSlicePrefix(req.FromAddress, 8)
		s.logger.Infof("[Planner] 开始规划多资产转账: from=%x, outputs=%d",
			fromPrefix, len(req.Outputs))
	}

	// 1. 按资产分组，计算每个资产的总需求
	assetRequests := make(map[string]*selector.AssetRequest)

	for _, output := range req.Outputs {
		var tokenID string
		if output.ContractAddress == nil {
			tokenID = "native"
		} else {
			tokenID = fmt.Sprintf("%x:%x", output.ContractAddress, output.ClassID)
		}

		// 累加同一资产的需求
		if existingReq, ok := assetRequests[tokenID]; ok {
			// ✅ 使用 big.Int 进行精确金额累加
			existingAmount, ok := new(big.Int).SetString(existingReq.Amount, 10)
			if !ok {
				return nil, fmt.Errorf("无效的金额格式: %s", existingReq.Amount)
			}
			outputAmount, ok := new(big.Int).SetString(output.Amount, 10)
			if !ok {
				return nil, fmt.Errorf("无效的金额格式: %s", output.Amount)
			}
			totalAmount := new(big.Int).Add(existingAmount, outputAmount)
			existingReq.Amount = totalAmount.String()
		} else {
			assetRequests[tokenID] = &selector.AssetRequest{
				TokenID:         tokenID,
				Amount:          output.Amount,
				ContractAddress: output.ContractAddress,
				ClassID:         output.ClassID,
			}
		}
	}

	// 2. 转换为数组
	assetRequestList := make([]*selector.AssetRequest, 0, len(assetRequests))
	for _, req := range assetRequests {
		assetRequestList = append(assetRequestList, req)
	}

	// 3. 使用 Selector 选择 UTXO
	selectionResult, err := s.selector.SelectUTXOs(ctx, req.FromAddress, assetRequestList)
	if err != nil {
		return nil, fmt.Errorf("UTXO 选择失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("[Planner] 选中 %d 个 UTXO", len(selectionResult.SelectedUTXOs))
	}

	// 4. 使用 Builder 构建交易（每次调用创建独立的 Builder 实例，避免状态串扰）
	builderSvc := builder.NewService(s.draftService)

	// 4.1 设置 nonce
	builderSvc.SetNonce(req.Nonce)

	// 4.2 添加所有选中的 UTXO 作为输入
	for _, utxo := range selectionResult.SelectedUTXOs {
		builderSvc.AddInput(utxo.Outpoint, false)
	}

	// 4.3 添加所有转账输出
	for _, output := range req.Outputs {
		builderSvc.AddAssetOutput(
			output.ToAddress,
			output.Amount,
			output.ContractAddress,
			output.LockingCondition,
		)
	}

	// 4.4 添加找零输出（为每个资产生成找零）
	for tokenID, changeAmount := range selectionResult.ChangeAmounts {
		// 根据 tokenID 确定资产类型
		var contractAddress []byte
		if tokenID != "native" {
			// ✅ 从 assetRequests 中查找对应的资产请求
			assetReq, ok := assetRequests[tokenID]
			if !ok || assetReq == nil {
				return nil, fmt.Errorf("找不到资产请求: tokenID=%s", tokenID)
			}
			contractAddress = assetReq.ContractAddress
		}

		builderSvc.AddAssetOutput(
			req.FromAddress, // 找零回发送方
			changeAmount,
			contractAddress,
			req.DefaultLockingCondition,
		)

		if s.logger != nil {
			s.logger.Debugf("[Planner] 添加找零输出: tokenID=%s, amount=%s", tokenID, changeAmount)
		}
	}

	// 4.5 构建 ComposedTx
	composedTx, err := builderSvc.Build()
	if err != nil {
		return nil, fmt.Errorf("构建交易失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("[Planner] 多资产交易构建完成: inputs=%d, outputs=%d",
			len(composedTx.Tx.Inputs), len(composedTx.Tx.Outputs))
	}

	return composedTx, nil
}

// safeSlicePrefix 安全地截取字节数组的前缀，避免数组越界
//
// 参数：
//   - data: 待截取的字节数组
//   - maxLen: 最大截取长度
//
// 返回：
//   - []byte: 截取的前缀（如果 data 长度不足，返回完整 data）
func safeSlicePrefix(data []byte, maxLen int) []byte {
	if len(data) == 0 {
		return []byte{}
	}
	if len(data) < maxLen {
		return data
	}
	return data[:maxLen]
}
