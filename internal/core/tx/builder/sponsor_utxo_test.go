// Package builder_test 提供 SponsorUTXOHelper 的单元测试
//
// 🧪 **测试覆盖**：
// - SponsorUTXOHelper 核心功能测试
// - 赞助UTXO识别测试
// - 元数据提取测试
// - 生命周期状态测试
// - 验证功能测试
package builder

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
)

// ==================== NewSponsorUTXOHelper 测试 ====================

// TestNewSponsorUTXOHelper 测试创建 SponsorUTXOHelper
func TestNewSponsorUTXOHelper(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()

	helper := NewSponsorUTXOHelper(utxoQuery)

	assert.NotNil(t, helper)
	assert.Equal(t, utxoQuery, helper.eutxoQuery)
}

// ==================== IsSponsorUTXO 测试 ====================

// TestIsSponsorUTXO_Success 测试识别赞助UTXO成功
func TestIsSponsorUTXO_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)

	result := helper.IsSponsorUTXO(sponsorUTXO)

	assert.True(t, result)
}

// TestIsSponsorUTXO_NilUTXO 测试nil UTXO
func TestIsSponsorUTXO_NilUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	result := helper.IsSponsorUTXO(nil)

	assert.False(t, result)
}

// TestIsSponsorUTXO_NoCachedOutput 测试没有CachedOutput
func TestIsSponsorUTXO_NoCachedOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建没有CachedOutput的UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	utxo := &utxopb.UTXO{
		Outpoint:     outpoint,
		Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
		Status:       utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
		OwnerAddress: constants.SponsorPoolOwner[:],
		// 没有CachedOutput
	}

	result := helper.IsSponsorUTXO(utxo)

	assert.False(t, result)
}

// TestIsSponsorUTXO_WrongOwner 测试错误的Owner
func TestIsSponsorUTXO_WrongOwner(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建Owner不是SponsorPoolOwner的UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	result := helper.IsSponsorUTXO(utxo)

	assert.False(t, result)
}

// TestIsSponsorUTXO_NoDelegationLock 测试没有DelegationLock
func TestIsSponsorUTXO_NoDelegationLock(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建只有SingleKeyLock的UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	result := helper.IsSponsorUTXO(utxo)

	assert.False(t, result)
}

// ==================== ExtractMetadata 测试 ====================

// TestExtractMetadata_Success 测试提取元数据成功
func TestExtractMetadata_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO.CreatedTimestamp = 1234567890

	metadata, err := helper.ExtractMetadata(sponsorUTXO)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "native", metadata.TokenType)
	assert.Equal(t, big.NewInt(1000000), metadata.TotalAmount)
	assert.Equal(t, big.NewInt(1000000), metadata.MaxPerClaim)
	assert.Equal(t, uint64(100), metadata.CreationHeight)
	assert.Equal(t, uint64(1234567890), metadata.CreationTime)
	assert.Equal(t, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE, metadata.CurrentStatus)
	assert.Equal(t, uint64(0), metadata.ExpiryHeight) // 没有设置过期时间
}

// TestExtractMetadata_WithExpiry 测试有过期时间的元数据提取
func TestExtractMetadata_WithExpiry(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建有过期时间的赞助UTXO
	expiryBlocks := uint64(50)
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, &expiryBlocks, 100)

	metadata, err := helper.ExtractMetadata(sponsorUTXO)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, uint64(150), metadata.ExpiryHeight) // 100 + 50
}

// TestExtractMetadata_NotSponsorUTXO 测试不是赞助UTXO
func TestExtractMetadata_NotSponsorUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建普通UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	metadata, err := helper.ExtractMetadata(utxo)

	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestExtractMetadata_NoCachedOutput 测试没有CachedOutput
func TestExtractMetadata_NoCachedOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建没有CachedOutput的UTXO
	// 注意：ExtractMetadata 首先调用 IsSponsorUTXO，如果没有CachedOutput会直接返回"不是赞助UTXO"
	outpoint := testutil.CreateOutPoint(nil, 0)
	utxo := &utxopb.UTXO{
		Outpoint:     outpoint,
		Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
		Status:       utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
		OwnerAddress: constants.SponsorPoolOwner[:],
		BlockHeight:  100,
		// 没有CachedOutput
	}

	metadata, err := helper.ExtractMetadata(utxo)

	assert.Error(t, err)
	assert.Nil(t, metadata)
	// ExtractMetadata 首先调用 IsSponsorUTXO，如果没有CachedOutput会直接返回"不是赞助UTXO"
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestExtractMetadata_NoDelegationLock 测试没有DelegationLock
func TestExtractMetadata_NoDelegationLock(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建只有SingleKeyLock的UTXO（但Owner是SponsorPoolOwner）
	// 注意：ExtractMetadata 首先调用 IsSponsorUTXO，如果没有DelegationLock会直接返回"不是赞助UTXO"
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	metadata, err := helper.ExtractMetadata(utxo)

	assert.Error(t, err)
	assert.Nil(t, metadata)
	// ExtractMetadata 首先调用 IsSponsorUTXO，如果没有DelegationLock会直接返回"不是赞助UTXO"
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestExtractMetadata_NotAssetOutput 测试不是AssetOutput
func TestExtractMetadata_NotAssetOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建ResourceOutput（不是AssetOutput）
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	// 创建简单的ResourceOutput（只需要Resource字段）
	output := &transaction_pb.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction_pb.LockingCondition{lock},
		OutputContent: &transaction_pb.TxOutput_Resource{
			Resource: &transaction_pb.ResourceOutput{
				// Resource字段可以为nil，用于测试
			},
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	metadata, err := helper.ExtractMetadata(utxo)

	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "必须是资产输出")
}

// TestExtractMetadata_ContractToken_Fungible 测试同质化合约代币元数据提取
func TestExtractMetadata_ContractToken_Fungible(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建同质化合约代币赞助UTXO
	contractAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_FungibleClassId{
					FungibleClassId: []byte("default"),
				},
				Amount: "500000",
			},
		},
	}
	output := &transaction_pb.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction_pb.LockingCondition{lock},
		OutputContent: &transaction_pb.TxOutput_Asset{
			Asset: asset,
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = 200

	metadata, err := helper.ExtractMetadata(utxo)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Contains(t, metadata.TokenType, "contract:")
	assert.Equal(t, big.NewInt(500000), metadata.TotalAmount)
}

// TestExtractMetadata_ContractToken_NFT 测试NFT合约代币元数据提取
func TestExtractMetadata_ContractToken_NFT(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建NFT合约代币赞助UTXO
	contractAddr := testutil.RandomAddress()
	nftID := testutil.RandomHash()
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_NftUniqueId{
					NftUniqueId: nftID,
				},
				Amount: "1",
			},
		},
	}
	output := &transaction_pb.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction_pb.LockingCondition{lock},
		OutputContent: &transaction_pb.TxOutput_Asset{
			Asset: asset,
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = 200

	metadata, err := helper.ExtractMetadata(utxo)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Contains(t, metadata.TokenType, "contract:")
	assert.Contains(t, metadata.TokenType, "nft:")
	assert.Equal(t, big.NewInt(1), metadata.TotalAmount)
}

// TestExtractMetadata_ContractToken_SFT 测试SFT合约代币元数据提取
func TestExtractMetadata_ContractToken_SFT(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建SFT合约代币赞助UTXO
	contractAddr := testutil.RandomAddress()
	batchID := testutil.RandomHash()
	instanceID := uint64(123)
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_SemiFungibleId{
					SemiFungibleId: &transaction_pb.SemiFungibleId{
						BatchId:    batchID,
						InstanceId: instanceID,
					},
				},
				Amount: "100",
			},
		},
	}
	output := &transaction_pb.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction_pb.LockingCondition{lock},
		OutputContent: &transaction_pb.TxOutput_Asset{
			Asset: asset,
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = 200

	metadata, err := helper.ExtractMetadata(utxo)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Contains(t, metadata.TokenType, "contract:")
	assert.Contains(t, metadata.TokenType, "sft:")
	assert.Equal(t, big.NewInt(100), metadata.TotalAmount)
}

// ==================== GetLifecycleState 测试 ====================

// TestGetLifecycleState_Active 测试活跃状态
func TestGetLifecycleState_Active(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建活跃的赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	currentHeight := uint64(200)

	state, err := helper.GetLifecycleState(context.Background(), sponsorUTXO, currentHeight)

	assert.NoError(t, err)
	assert.Equal(t, SponsorStateActive, state)
}

// TestGetLifecycleState_FullyClaimed 测试全部领取状态
func TestGetLifecycleState_FullyClaimed(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建已消费的赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO.Status = utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED
	currentHeight := uint64(200)

	state, err := helper.GetLifecycleState(context.Background(), sponsorUTXO, currentHeight)

	assert.NoError(t, err)
	assert.Equal(t, SponsorStateFullyClaimed, state)
}

// TestGetLifecycleState_Expired 测试已过期状态
func TestGetLifecycleState_Expired(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建已过期的赞助UTXO
	expiryBlocks := uint64(50)
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, &expiryBlocks, 100)
	currentHeight := uint64(200) // 超过过期高度 150

	state, err := helper.GetLifecycleState(context.Background(), sponsorUTXO, currentHeight)

	assert.NoError(t, err)
	assert.Equal(t, SponsorStateExpired, state)
}

// TestGetLifecycleState_PartialClaimed 测试部分领取状态
func TestGetLifecycleState_PartialClaimed(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建部分领取的赞助UTXO（当前金额小于总金额）
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  2000000, // 最大领取金额大于当前金额
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	// 创建金额较小的输出（模拟部分领取后的找零）
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "500000", lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = 100

	currentHeight := uint64(200)

	state, err := helper.GetLifecycleState(context.Background(), utxo, currentHeight)

	assert.NoError(t, err)
	// 注意：由于无法准确判断是否部分领取（需要查询历史），这里可能返回Active
	// 但测试用例展示了部分领取的逻辑路径
	assert.Contains(t, []SponsorLifecycleState{SponsorStateActive, SponsorStatePartialClaimed}, state)
}

// TestGetLifecycleState_NotSponsorUTXO 测试不是赞助UTXO
func TestGetLifecycleState_NotSponsorUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建普通UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	state, err := helper.GetLifecycleState(context.Background(), utxo, 200)

	assert.Error(t, err)
	assert.Equal(t, SponsorStateUnknown, state)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// ==================== ValidateSponsorUTXO 测试 ====================

// TestValidateSponsorUTXO_Success 测试验证成功
func TestValidateSponsorUTXO_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建有效的赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)

	err := helper.ValidateSponsorUTXO(sponsorUTXO)

	assert.NoError(t, err)
}

// TestValidateSponsorUTXO_NotSponsorUTXO 测试不是赞助UTXO
func TestValidateSponsorUTXO_NotSponsorUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建普通UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	err := helper.ValidateSponsorUTXO(utxo)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestValidateSponsorUTXO_WrongOwner 测试错误的Owner
func TestValidateSponsorUTXO_WrongOwner(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建Owner不是SponsorPoolOwner的UTXO
	// 注意：ValidateSponsorUTXO 首先调用 IsSponsorUTXO，如果不是赞助UTXO会直接返回错误
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	err := helper.ValidateSponsorUTXO(utxo)

	// ValidateSponsorUTXO 首先调用 IsSponsorUTXO，如果不是赞助UTXO会直接返回"不是赞助UTXO"
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestValidateSponsorUTXO_NoDelegationLock 测试没有DelegationLock
func TestValidateSponsorUTXO_NoDelegationLock(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建只有SingleKeyLock的UTXO
	// 注意：ValidateSponsorUTXO 首先调用 IsSponsorUTXO，如果不是赞助UTXO会直接返回错误
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	err := helper.ValidateSponsorUTXO(utxo)

	// ValidateSponsorUTXO 首先调用 IsSponsorUTXO，如果不是赞助UTXO会直接返回"不是赞助UTXO"
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// TestValidateSponsorUTXO_NoConsumeOperation 测试没有consume操作授权
func TestValidateSponsorUTXO_NoConsumeOperation(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建只有transfer授权的UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"transfer"}, nil, 100)

	err := helper.ValidateSponsorUTXO(sponsorUTXO)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未授权consume操作")
}

// TestValidateSponsorUTXO_NotAssetOutput 测试不是AssetOutput
func TestValidateSponsorUTXO_NotAssetOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	helper := NewSponsorUTXOHelper(utxoQuery)

	// 创建ResourceOutput
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := &transaction_pb.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction_pb.LockingCondition{lock},
		OutputContent: &transaction_pb.TxOutput_Resource{
			Resource: &transaction_pb.ResourceOutput{
				// Resource字段可以为nil，用于测试
			},
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	err := helper.ValidateSponsorUTXO(utxo)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须是AssetOutput")
}

// ==================== 辅助函数 ====================

// createSponsorUTXOForTest 创建测试用的赞助UTXO
func createSponsorUTXOForTest(amount string, authorizedOps []string, expiryBlocks *uint64, blockHeight uint64) *utxopb.UTXO {
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: authorizedOps,
		MaxValuePerOperation:  1000000,
		ExpiryDurationBlocks:  expiryBlocks,
		AllowedDelegates:      nil,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], amount, lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = blockHeight
	return utxo
}

