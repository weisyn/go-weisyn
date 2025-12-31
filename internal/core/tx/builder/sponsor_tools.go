package builder

import (
	"context"
	"fmt"
	"math/big"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo_pb "github.com/weisyn/v1/pb/blockchain/utxo"
	cryptoface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// SponsorTools 赞助UTXO工具集
//
// 🎯 **核心职责**：提供赞助UTXO的创建、查询、管理工具
//
// 💡 **设计理念**：
// - 提供统一的工具接口，简化赞助UTXO的使用
// - 封装底层查询和构建逻辑
// - 支持多种锁定方式（DelegationLock、ContractLock等）
type SponsorTools struct {
	eutxoQuery persistence.UTXOQuery
	helper     *SponsorUTXOHelper
	audit      *SponsorAuditService
}

// NewSponsorTools 创建赞助工具集
func NewSponsorTools(
	eutxoQuery persistence.UTXOQuery,
	txQuery persistence.TxQuery,
	chainQuery persistence.ChainQuery,
	hashManager cryptoface.HashManager,
) *SponsorTools {
	return &SponsorTools{
		eutxoQuery: eutxoQuery,
		helper:     NewSponsorUTXOHelper(eutxoQuery),
		audit:      NewSponsorAuditService(eutxoQuery, txQuery, chainQuery, hashManager),
	}
}

// SponsorUTXOInfo 赞助UTXO信息
type SponsorUTXOInfo struct {
	UTXO            *utxo_pb.UTXO        // UTXO对象
	Metadata        *SponsorMetadata     // 元数据
	LifecycleState  SponsorLifecycleState // 生命周期状态
}

// ListSponsorUTXOs 列出所有赞助UTXO
//
// **功能**：
// - 查询所有赞助池UTXO
// - 提取元数据
// - 计算生命周期状态
//
// **参数**：
//   - ctx: 上下文对象
//   - currentHeight: 当前区块高度（用于计算过期状态）
//   - onlyAvailable: 是否只返回可用状态的UTXO
//
// **返回**：
//   - []*SponsorUTXOInfo: 赞助UTXO信息列表
//   - error: 查询错误
func (t *SponsorTools) ListSponsorUTXOs(
	ctx context.Context,
	currentHeight uint64,
	onlyAvailable bool,
) ([]*SponsorUTXOInfo, error) {
	// 1. 查询所有赞助池UTXO
	utxos, err := t.eutxoQuery.GetSponsorPoolUTXOs(ctx, onlyAvailable)
	if err != nil {
		return nil, fmt.Errorf("查询赞助池UTXO失败: %w", err)
	}

	// 2. 提取元数据和状态
	var result []*SponsorUTXOInfo
	for _, utxo := range utxos {
		// 验证是否为赞助UTXO
		if !t.helper.IsSponsorUTXO(utxo) {
			continue
		}

		// 提取元数据
		metadata, err := t.helper.ExtractMetadata(utxo)
		if err != nil {
			continue // 提取失败，跳过
		}

		// 计算生命周期状态
		state, err := t.helper.GetLifecycleState(ctx, utxo, currentHeight)
		if err != nil {
			state = SponsorStateUnknown // 状态计算失败，标记为未知
		}

		result = append(result, &SponsorUTXOInfo{
			UTXO:           utxo,
			Metadata:       metadata,
			LifecycleState: state,
		})
	}

	return result, nil
}

// SponsorUTXODetail 赞助UTXO详细信息
type SponsorUTXODetail struct {
	Info       *SponsorUTXOInfo
	ClaimHistory []*ClaimRecord
}

// GetSponsorUTXOInfo 获取单个赞助UTXO的详细信息
//
// **功能**：
// - 查询指定UTXO
// - 提取元数据
// - 计算生命周期状态
// - 查询领取历史
//
// **参数**：
//   - ctx: 上下文对象
//   - outpoint: UTXO的OutPoint
//   - currentHeight: 当前区块高度
//
// **返回**：
//   - *SponsorUTXODetail: 赞助UTXO详细信息
//   - error: 查询错误
func (t *SponsorTools) GetSponsorUTXOInfo(
	ctx context.Context,
	outpoint *transaction_pb.OutPoint,
	currentHeight uint64,
) (*SponsorUTXODetail, error) {
	// 1. 查询UTXO
	utxo, err := t.eutxoQuery.GetUTXO(ctx, outpoint)
	if err != nil {
		return nil, fmt.Errorf("查询UTXO失败: %w", err)
	}

	// 2. 验证是否为赞助UTXO
	if !t.helper.IsSponsorUTXO(utxo) {
		return nil, fmt.Errorf("不是赞助UTXO")
	}

	// 3. 提取元数据
	metadata, err := t.helper.ExtractMetadata(utxo)
	if err != nil {
		return nil, fmt.Errorf("提取元数据失败: %w", err)
	}

	// 4. 计算生命周期状态
	state, err := t.helper.GetLifecycleState(ctx, utxo, currentHeight)
	if err != nil {
		state = SponsorStateUnknown
	}

	// 5. 查询领取历史
	claimHistory, err := t.audit.GetSponsorClaimHistory(ctx, outpoint)
	if err != nil {
		claimHistory = []*ClaimRecord{} // 查询失败，返回空列表
	}

	return &SponsorUTXODetail{
		Info: &SponsorUTXOInfo{
			UTXO:           utxo,
			Metadata:       metadata,
			LifecycleState: state,
		},
		ClaimHistory: claimHistory,
	}, nil
}

// ValidateSponsorUTXO 验证赞助UTXO是否符合标准
//
// **功能**：
// - 验证UTXO结构
// - 验证DelegationLock配置
// - 验证金额和代币类型
func (t *SponsorTools) ValidateSponsorUTXO(utxo *utxo_pb.UTXO) error {
	return t.helper.ValidateSponsorUTXO(utxo)
}

// GetStatistics 获取赞助统计信息
//
// **功能**：
// - 统计总赞助数、总金额、已领取金额等
func (t *SponsorTools) GetStatistics(ctx context.Context) (*SponsorStats, error) {
	return t.audit.GetSponsorStatistics(ctx)
}

// GetMinerClaimHistory 查询矿工的领取历史
//
// **功能**：
// - 查询指定矿工的所有领取记录
func (t *SponsorTools) GetMinerClaimHistory(
	ctx context.Context,
	minerAddr []byte,
) ([]*ClaimRecord, error) {
	return t.audit.GetMinerClaimHistory(ctx, minerAddr)
}

// SponsorUTXOConfig 赞助UTXO配置（用于创建）
//
// **功能**：
// - 封装创建赞助UTXO所需的配置
// - 支持多种锁定方式
type SponsorUTXOConfig struct {
	// 资产信息
	TokenType string   // 代币类型（native/contract:xxx:yyy）
	Amount    *big.Int // 金额

	// 锁定方式（三选一）
	UseDelegationLock bool                   // 使用DelegationLock（当前默认）
	UseContractLock   bool                   // 使用ContractLock（需要智能合约）
	UseHeightLock     bool                   // 使用HeightLock嵌套DelegationLock

	// DelegationLock配置
	MaxValuePerOperation  uint64   // 单次最大领取金额
	ExpiryDurationBlocks  *uint64  // 过期高度（可选）
	AllowedDelegates      [][]byte // 允许的委托地址（空=任意矿工）

	// ContractLock配置（如果使用）
	ContractAddress []byte // 合约地址
	RequiredMethod  string  // 必需的方法名

	// HeightLock配置（如果使用）
	UnlockHeight       uint64  // 解锁高度
	ConfirmationBlocks uint32  // 确认区块数（可选，默认0）
	GraceBlocks        *uint64 // 宽限区块数（可选）
	// 注意：HeightLock本身没有过期字段，过期通过DelegationLock的ExpiryDurationBlocks实现

	// 元数据（可选，当前无法存储到UTXO）
	Description string // 描述信息
	Purpose     string // 目的信息
}

// ValidateConfig 验证配置有效性
func (c *SponsorUTXOConfig) ValidateConfig() error {
	// 1. 验证锁定方式选择
	lockCount := 0
	if c.UseDelegationLock {
		lockCount++
	}
	if c.UseContractLock {
		lockCount++
	}
	if c.UseHeightLock {
		lockCount++
	}
	if lockCount != 1 {
		return fmt.Errorf("必须且只能选择一种锁定方式")
	}

	// 2. 验证金额
	if c.Amount == nil || c.Amount.Sign() <= 0 {
		return fmt.Errorf("金额必须大于0")
	}

	// 3. 验证代币类型
	if c.TokenType == "" {
		return fmt.Errorf("代币类型不能为空")
	}

	// 4. 验证ContractLock配置（如果使用）
	if c.UseContractLock {
		if len(c.ContractAddress) == 0 {
			return fmt.Errorf("ContractLock需要合约地址")
		}
		if c.RequiredMethod == "" {
			return fmt.Errorf("ContractLock需要方法名")
		}
	}

	// 5. 验证HeightLock配置（如果使用）
	if c.UseHeightLock {
		if c.UnlockHeight == 0 {
			return fmt.Errorf("UnlockHeight必须大于0")
		}
		// 注意：过期通过DelegationLock的ExpiryDurationBlocks实现
	}

	return nil
}

// ToLockingConditions 将配置转换为LockingConditions
//
// **功能**：
// - 根据配置生成LockingConditions
// - 支持多种锁定方式
func (c *SponsorUTXOConfig) ToLockingConditions() ([]*transaction_pb.LockingCondition, error) {
	if err := c.ValidateConfig(); err != nil {
		return nil, err
	}

	var conditions []*transaction_pb.LockingCondition

	// 根据选择的锁定方式生成条件
	if c.UseDelegationLock {
		// DelegationLock方式
		delegationLock := &transaction_pb.DelegationLock{
			AuthorizedOperations: []string{"consume"},
			MaxValuePerOperation: c.MaxValuePerOperation,
			ExpiryDurationBlocks: c.ExpiryDurationBlocks,
		}
		// 转换AllowedDelegates
		if len(c.AllowedDelegates) > 0 {
			delegationLock.AllowedDelegates = make([][]byte, len(c.AllowedDelegates))
			copy(delegationLock.AllowedDelegates, c.AllowedDelegates)
		}

		conditions = append(conditions, &transaction_pb.LockingCondition{
			Condition: &transaction_pb.LockingCondition_DelegationLock{
				DelegationLock: delegationLock,
			},
		})
	} else if c.UseContractLock {
		// ContractLock方式
		contractLock := &transaction_pb.ContractLock{
			ContractAddress: c.ContractAddress,
			RequiredMethod:  c.RequiredMethod,
		}

		conditions = append(conditions, &transaction_pb.LockingCondition{
			Condition: &transaction_pb.LockingCondition_ContractLock{
				ContractLock: contractLock,
			},
		})
	} else if c.UseHeightLock {
		// HeightLock嵌套DelegationLock方式
		delegationLock := &transaction_pb.DelegationLock{
			AuthorizedOperations: []string{"consume"},
			MaxValuePerOperation: c.MaxValuePerOperation,
			ExpiryDurationBlocks: c.ExpiryDurationBlocks, // 过期通过DelegationLock实现
		}
		// 转换AllowedDelegates
		if len(c.AllowedDelegates) > 0 {
			delegationLock.AllowedDelegates = make([][]byte, len(c.AllowedDelegates))
			copy(delegationLock.AllowedDelegates, c.AllowedDelegates)
		}

		heightLock := &transaction_pb.HeightLock{
			UnlockHeight:       c.UnlockHeight,
			ConfirmationBlocks: c.ConfirmationBlocks,
			GraceBlocks:        c.GraceBlocks,
			BaseLock: &transaction_pb.LockingCondition{
				Condition: &transaction_pb.LockingCondition_DelegationLock{
					DelegationLock: delegationLock,
				},
			},
		}

		conditions = append(conditions, &transaction_pb.LockingCondition{
			Condition: &transaction_pb.LockingCondition_HeightLock{
				HeightLock: heightLock,
			},
		})
	}

	return conditions, nil
}

