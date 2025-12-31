// Package chain 提供链状态查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 ChainQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 链信息查询
// - 高度查询
// - 区块哈希查询
// - 节点模式查询
// - 数据新鲜度检查
// - 就绪状态检查
// - 同步状态查询
// - 查询指标
package chain

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, logger, nil) // blockQuery 为 nil 表示使用备用修复策略

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsError 测试使用 nil storage 创建服务
func TestNewService_WithNilStorage_ReturnsError(t *testing.T) {
	// Arrange
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(nil, logger, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "storage 不能为空")
}

// ==================== 链信息查询测试 ====================

// TestGetChainInfo_WithValidTipData_ReturnsChainInfo 测试获取链信息
func TestGetChainInfo_WithValidTipData_ReturnsChainInfo(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据（格式：height(8字节) + blockHash(32字节)）
	height := uint64(100)
	blockHash := testutil.RandomHash()
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], blockHash)

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	chainInfo, err := service.GetChainInfo(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, chainInfo)
	assert.Equal(t, height, chainInfo.Height)
	assert.Equal(t, blockHash, chainInfo.BestBlockHash)
	assert.True(t, chainInfo.IsReady)
}

// TestGetChainInfo_WithInvalidTipData_AutoRepairs 测试无效链尖数据时自动修复
func TestGetChainInfo_WithInvalidTipData_AutoRepairs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置无效的链尖数据（长度不足）
	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, []byte{1, 2, 3})
	require.NoError(t, err)

	// Act - 应该自动修复（通过创世区块初始化）
	chainInfo, err := service.GetChainInfo(ctx)

	// Assert - 修复成功，不返回错误
	assert.NoError(t, err)
	assert.NotNil(t, chainInfo)
	assert.Equal(t, uint64(0), chainInfo.Height)
	assert.Equal(t, "genesis_initialized", chainInfo.Status)
}

// ==================== 高度查询测试 ====================

// TestGetCurrentHeight_WithValidTipData_ReturnsHeight 测试获取当前高度
func TestGetCurrentHeight_WithValidTipData_ReturnsHeight(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据
	height := uint64(200)
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], testutil.RandomHash())

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	currentHeight, err := service.GetCurrentHeight(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, height, currentHeight)
}

// TestGetCurrentHeight_WithMissingTipData_AutoRepairs 测试缺失链尖数据时自动修复
func TestGetCurrentHeight_WithMissingTipData_AutoRepairs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// Act - 应该自动修复（通过创世区块初始化）
	height, err := service.GetCurrentHeight(ctx)

	// Assert - 修复成功，返回创世高度0
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), height)
}

// TestGetCurrentHeight_EmptyChainWithoutGenesisHash_DoesNotAutoRepair
// 空链首次启动：state:chain:tip 为空，且 metadata 中不存在 genesis_hash
// 期望：直接返回高度 0，但不触发“修复/创世兜底”写入 chain tip
func TestGetCurrentHeight_EmptyChainWithoutGenesisHash_DoesNotAutoRepair(t *testing.T) {
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// Act
	height, err := service.GetCurrentHeight(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uint64(0), height)

	// 不应由 QueryService 抢跑写入 chain tip
	exists, err := storage.Exists(ctx, []byte("state:chain:tip"))
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestValidateAndRepairOnStartup_EmptyChainWithoutGenesisHash_SkipsRepair
// 空链首次启动：ValidateAndRepairOnStartup 不应执行强制修复（更不能走策略3-创世兜底写入 tip）
func TestValidateAndRepairOnStartup_EmptyChainWithoutGenesisHash_SkipsRepair(t *testing.T) {
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	svc, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	impl, ok := svc.(*Service)
	require.True(t, ok)

	err = impl.ValidateAndRepairOnStartup(ctx)
	require.NoError(t, err)

	exists, err := storage.Exists(ctx, []byte("state:chain:tip"))
	require.NoError(t, err)
	assert.False(t, exists)
}

// ==================== 区块哈希查询测试 ====================

// TestGetBestBlockHash_WithValidTipData_ReturnsHash 测试获取最佳区块哈希
func TestGetBestBlockHash_WithValidTipData_ReturnsHash(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据
	blockHash := testutil.RandomHash()
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], 100)
	copy(tipData[8:40], blockHash)

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	hash, err := service.GetBestBlockHash(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, blockHash, hash)
}

// ==================== 节点模式查询测试 ====================

// TestGetNodeMode_WithStoredMode_ReturnsMode 测试从存储读取节点模式
func TestGetNodeMode_WithStoredMode_ReturnsMode(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置节点模式
	nodeModeKey := []byte("config:node:mode")
	err = storage.Set(ctx, nodeModeKey, []byte("light"))
	require.NoError(t, err)

	// Act
	mode, err := service.GetNodeMode(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, types.NodeModeLight, mode)
}

// TestGetNodeMode_WithNoStoredMode_ReturnsDefault 测试无存储模式时返回默认值
func TestGetNodeMode_WithNoStoredMode_ReturnsDefault(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// Act
	mode, err := service.GetNodeMode(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, types.NodeModeFull, mode, "应该返回默认的全节点模式")
}

// ==================== 数据新鲜度检查测试 ====================

// TestIsDataFresh_DeprecatedAlwaysReturnsFalse 测试废弃后的 IsDataFresh 始终返回 false
func TestIsDataFresh_DeprecatedAlwaysReturnsFalse(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// Act
	isFresh, err := service.IsDataFresh(ctx)

	// Assert
	assert.NoError(t, err)
	assert.False(t, isFresh, "废弃后的 IsDataFresh 应始终返回 false（保守策略）")
}

// ==================== 就绪状态检查测试 ====================

// TestIsReady_WithHeightGreaterThanZero_ReturnsTrue 测试高度大于0时系统就绪
func TestIsReady_WithHeightGreaterThanZero_ReturnsTrue(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据（高度 > 0）
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], 1)
	copy(tipData[8:40], testutil.RandomHash())

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	isReady, err := service.IsReady(ctx)

	// Assert
	assert.NoError(t, err)
	assert.True(t, isReady)
}

// TestIsReady_WithZeroHeight_ReturnsFalse 测试高度为0时系统未就绪
func TestIsReady_WithZeroHeight_ReturnsTrue(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据（高度 = 0）
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], 0)
	copy(tipData[8:40], testutil.RandomHash())

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	isReady, err := service.IsReady(ctx)

	// Assert
	assert.NoError(t, err)
	assert.True(t, isReady, "高度为0（仅创世块）时系统应视为就绪")
}

// ==================== 同步状态查询测试 ====================

// TestGetSyncStatus_ReturnsBasicStatus 测试获取同步状态
func TestGetSyncStatus_ReturnsBasicStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置链尖数据
	height := uint64(50)
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], testutil.RandomHash())

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	syncStatus, err := service.GetSyncStatus(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, syncStatus)
	assert.Equal(t, height, syncStatus.CurrentHeight)
	assert.Equal(t, types.SyncStatusSyncing, syncStatus.Status)
}

// ==================== 查询指标测试 ====================

// TestGetQueryMetrics_ReturnsMetrics 测试获取查询指标
func TestGetQueryMetrics_ReturnsMetrics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 预置链尖数据，确保查询不会因为“缺少 tip”而计入错误指标
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], 0)
	copy(tipData[8:40], testutil.RandomHash())
	require.NoError(t, storage.Set(ctx, []byte("state:chain:tip"), tipData))

	// 执行一些查询以更新指标
	_, _ = service.GetCurrentHeight(ctx)
	_, _ = service.GetBestBlockHash(ctx)

	// Act
	metrics, err := service.GetQueryMetrics(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.QueryCount, uint64(0))
	assert.True(t, metrics.IsHealthy)
}

// ==================== 链尖修复测试 ====================

// TestRepairChainTipFallback_WithValidIndices_RebuildsChainTip 测试备用修复策略（从索引重建）
func TestRepairChainTipFallback_WithValidIndices_RebuildsChainTip(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置高度索引数据（模拟已有区块）
	height := uint64(100)
	blockHash := testutil.RandomHash()

	// 写入索引：indices:height:100 = blockHash
	indexKey := []byte("indices:height:100")
	err = storage.Set(ctx, indexKey, blockHash)
	require.NoError(t, err)

	// 删除链尖数据（模拟损坏）
	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, []byte{})
	require.NoError(t, err)

	// Act - 调用内部修复方法
	if svc, ok := service.(*Service); ok {
		repaired, err := svc.repairChainTipFallback(ctx)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, repaired)
		assert.Equal(t, height, repaired.Height)
		assert.Equal(t, "repaired_fallback", repaired.Status)

		// 验证链尖数据已写入
		tipData, err := storage.Get(ctx, tipKey)
		assert.NoError(t, err)
		assert.Equal(t, 40, len(tipData))
	}
}

// TestValidateAndRepairOnStartup_WithMissingTip_PerformsRepair 测试启动时检查（链尖不存在）
func TestValidateAndRepairOnStartup_WithMissingTip_PerformsRepair(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()

	// 创建服务但不设置链尖数据（模拟首次启动或数据丢失）
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 标记“链已创建”：存在 genesis_hash 元数据时，启动自愈才允许介入修复 chain tip
	// （否则空链首次启动应由启动流程创建创世区块，而不是 QueryService 抢跑写入 tip）
	err = storage.Set(ctx, []byte("system:chain_identity:genesis_hash"), []byte("dummy_genesis_hash"))
	require.NoError(t, err)

	// 设置一些高度索引，让备用修复策略能够工作
	blockHash := testutil.RandomHash()
	indexKey := []byte("indices:height:1")
	err = storage.Set(ctx, indexKey, blockHash)
	require.NoError(t, err)

	// Act - 调用启动验证
	if svc, ok := service.(*Service); ok {
		err := svc.ValidateAndRepairOnStartup(ctx)

		// Assert - 应该成功修复
		assert.NoError(t, err)

		// 验证链尖已被创建
		tipKey := []byte("state:chain:tip")
		tipData, err := storage.Get(ctx, tipKey)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(tipData), 40)
	}
}

// TestValidateAndRepairOnStartup_WithValidTip_PassesCheck 测试启动时检查（链尖正常）
func TestValidateAndRepairOnStartup_WithValidTip_PassesCheck(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置有效的链尖数据
	height := uint64(100)
	blockHash := testutil.RandomHash()
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], blockHash)

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act - 调用启动验证
	if svc, ok := service.(*Service); ok {
		err := svc.ValidateAndRepairOnStartup(ctx)

		// Assert - 应该通过检查
		assert.NoError(t, err)
	}
}

// TestGetChainInfo_WithCorruptedTip_UsesMultiLayerRepair 测试多层修复降级
func TestGetChainInfo_WithCorruptedTip_UsesMultiLayerRepair(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 设置损坏的链尖数据（长度不足）
	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, []byte{1, 2, 3})
	require.NoError(t, err)

	// 设置高度索引，让备用修复策略能够工作
	height := uint64(50)
	blockHash := testutil.RandomHash()
	indexKey := []byte("indices:height:50")
	err = storage.Set(ctx, indexKey, blockHash)
	require.NoError(t, err)

	// Act - 调用 GetChainInfo，应该触发多层修复
	chainInfo, err := service.GetChainInfo(ctx)

	// Assert - 应该成功修复并返回链信息
	assert.NoError(t, err)
	assert.NotNil(t, chainInfo)
	assert.Equal(t, height, chainInfo.Height)
	// 状态应该是 repaired_fallback（策略2）或 genesis_initialized（策略3）
	assert.Contains(t, []string{"repaired_fallback", "genesis_initialized"}, chainInfo.Status)
}

// TestGetChainInfoWithFallback_WithNoData_UsesGenesisInit 测试降级查询（无数据时使用创世区块初始化）
func TestGetChainInfoWithFallback_WithNoData_UsesGenesisInit(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, logger, nil)
	require.NoError(t, err)

	// 不设置任何数据，修复策略3（创世区块初始化）会成功

	// Act - 调用降级查询
	if svc, ok := service.(*Service); ok {
		chainInfo, err := svc.GetChainInfoWithFallback(ctx)

		// Assert - 应该返回创世区块初始化的信息（不是错误）
		assert.NoError(t, err)
		assert.NotNil(t, chainInfo)
		assert.Equal(t, uint64(0), chainInfo.Height)
		assert.False(t, chainInfo.IsReady)
		assert.Equal(t, "genesis_initialized", chainInfo.Status)
	}
}
