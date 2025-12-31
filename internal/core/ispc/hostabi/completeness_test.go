package hostabi

import (
	"context"
	"testing"

	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// HostABI原语完整性验证测试
// ============================================================================
//
// 🎯 **目的**：
//   - 验证所有17个原语都已实现
//   - 验证包装器（Stats和Security）正确包装所有原语
//   - 验证原语使用统计功能
//   - 验证原语完整性检查功能
//
// 📋 **测试范围**：
//   - 17个原语的完整性检查
//   - HostRuntimePortsWithStats包装器
//   - HostRuntimePortsWithSecurity包装器
//   - PrimitiveCompletenessChecker
//   - PrimitiveUsageStats
//
// ============================================================================

// mockHostABI Mock的HostABI实现，用于测试包装器
type mockHostABI struct{}

func (m *mockHostABI) GetBlockHeight(ctx context.Context) (uint64, error) {
	return 100, nil
}

func (m *mockHostABI) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	return 1234567890, nil
}

func (m *mockHostABI) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *mockHostABI) GetChainID(ctx context.Context) ([]byte, error) {
	return []byte("test-chain"), nil
}

func (m *mockHostABI) GetCaller(ctx context.Context) ([]byte, error) {
	return make([]byte, 20), nil
}

func (m *mockHostABI) GetContractAddress(ctx context.Context) ([]byte, error) {
	return make([]byte, 20), nil
}

func (m *mockHostABI) GetTransactionID(ctx context.Context) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *mockHostABI) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	return &pb.TxOutput{}, nil
}

func (m *mockHostABI) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	return true, nil
}

func (m *mockHostABI) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	return &pbresource.Resource{}, nil
}

func (m *mockHostABI) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	return true, nil
}

func (m *mockHostABI) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockHostABI) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}

func (m *mockHostABI) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}

func (m *mockHostABI) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

func (m *mockHostABI) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	return nil
}

func (m *mockHostABI) LogDebug(ctx context.Context, message string) error {
	return nil
}

// 确保实现接口
var _ publicispc.HostABI = (*mockHostABI)(nil)

// ============================================================================
// 测试：PrimitiveCompletenessChecker
// ============================================================================

// TestPrimitiveCompletenessChecker_CheckCompleteness 测试原语完整性检查
func TestPrimitiveCompletenessChecker_CheckCompleteness(t *testing.T) {
	checker := NewPrimitiveCompletenessChecker()
	mockABI := &mockHostABI{}

	missingPrimitives, err := checker.CheckCompleteness(mockABI)
	if err != nil {
		t.Fatalf("检查原语完整性失败: %v", err)
	}

	if len(missingPrimitives) > 0 {
		t.Errorf("发现缺失的原语: %v", missingPrimitives)
	}
}

// TestPrimitiveCompletenessChecker_All17Primitives 测试所有17个原语都已定义
func TestPrimitiveCompletenessChecker_All17Primitives(t *testing.T) {
	checker := NewPrimitiveCompletenessChecker()

	expectedCount := 17
	actualCount := len(checker.requiredPrimitives)

	if actualCount != expectedCount {
		t.Errorf("期望17个原语，实际有%d个", actualCount)
	}

	// 验证所有17个原语都在列表中
	expectedPrimitives := []string{
		"GetBlockHeight", "GetBlockTimestamp", "GetBlockHash", "GetChainID",
		"GetCaller", "GetContractAddress", "GetTransactionID",
		"UTXOLookup", "UTXOExists",
		"ResourceLookup", "ResourceExists",
		"TxAddInput", "TxAddAssetOutput", "TxAddResourceOutput", "TxAddStateOutput",
		"EmitEvent", "LogDebug",
	}

	primitiveMap := make(map[string]bool)
	for _, p := range checker.requiredPrimitives {
		primitiveMap[p] = true
	}

	for _, expected := range expectedPrimitives {
		if !primitiveMap[expected] {
			t.Errorf("缺失原语: %s", expected)
		}
	}
}

// ============================================================================
// 测试：HostRuntimePortsWithStats包装器
// ============================================================================

// TestHostRuntimePortsWithStats_AllPrimitives 测试所有17个原语都被包装
func TestHostRuntimePortsWithStats_AllPrimitives(t *testing.T) {
	mockABI := &mockHostABI{}
	wrapper := NewHostRuntimePortsWithStats(mockABI)
	ctx := context.Background()

	// 测试所有17个原语
	// 类别 A：确定性区块视图（4个）
	_, err := wrapper.GetBlockHeight(ctx)
	if err != nil {
		t.Errorf("GetBlockHeight失败: %v", err)
	}

	_, err = wrapper.GetBlockTimestamp(ctx)
	if err != nil {
		t.Errorf("GetBlockTimestamp失败: %v", err)
	}

	_, err = wrapper.GetBlockHash(ctx, 0)
	if err != nil {
		t.Errorf("GetBlockHash失败: %v", err)
	}

	_, err = wrapper.GetChainID(ctx)
	if err != nil {
		t.Errorf("GetChainID失败: %v", err)
	}

	// 类别 B：执行上下文（3个）
	_, err = wrapper.GetCaller(ctx)
	if err != nil {
		t.Errorf("GetCaller失败: %v", err)
	}

	_, err = wrapper.GetContractAddress(ctx)
	if err != nil {
		t.Errorf("GetContractAddress失败: %v", err)
	}

	_, err = wrapper.GetTransactionID(ctx)
	if err != nil {
		t.Errorf("GetTransactionID失败: %v", err)
	}

	// 类别 C：UTXO查询（2个）
	_, err = wrapper.UTXOLookup(ctx, nil)
	if err != nil {
		t.Errorf("UTXOLookup失败: %v", err)
	}

	_, err = wrapper.UTXOExists(ctx, nil)
	if err != nil {
		t.Errorf("UTXOExists失败: %v", err)
	}

	// 类别 D：资源查询（2个）
	_, err = wrapper.ResourceLookup(ctx, nil)
	if err != nil {
		t.Errorf("ResourceLookup失败: %v", err)
	}

	_, err = wrapper.ResourceExists(ctx, nil)
	if err != nil {
		t.Errorf("ResourceExists失败: %v", err)
	}

	// 类别 E：交易草稿构建（4个）
	_, err = wrapper.TxAddInput(ctx, nil, false, nil)
	if err != nil {
		t.Errorf("TxAddInput失败: %v", err)
	}

	_, err = wrapper.TxAddAssetOutput(ctx, nil, 0, nil, nil)
	if err != nil {
		t.Errorf("TxAddAssetOutput失败: %v", err)
	}

	_, err = wrapper.TxAddResourceOutput(ctx, nil, "", nil, nil, nil)
	if err != nil {
		t.Errorf("TxAddResourceOutput失败: %v", err)
	}

	_, err = wrapper.TxAddStateOutput(ctx, nil, 0, nil, nil, nil)
	if err != nil {
		t.Errorf("TxAddStateOutput失败: %v", err)
	}

	// 类别 G：执行追踪（2个）
	err = wrapper.EmitEvent(ctx, "", nil)
	if err != nil {
		t.Errorf("EmitEvent失败: %v", err)
	}

	err = wrapper.LogDebug(ctx, "")
	if err != nil {
		t.Errorf("LogDebug失败: %v", err)
	}
}

// TestHostRuntimePortsWithStats_UsageStats 测试使用统计功能
func TestHostRuntimePortsWithStats_UsageStats(t *testing.T) {
	mockABI := &mockHostABI{}
	wrapper := NewHostRuntimePortsWithStats(mockABI)
	ctx := context.Background()

	// 调用几个原语
	wrapper.GetBlockHeight(ctx)
	wrapper.GetBlockTimestamp(ctx)
	wrapper.GetCaller(ctx)

	// 获取统计信息
	stats := wrapper.GetUsageStats()

	// 验证统计信息
	callCounts, ok := stats["call_counts"].(map[string]uint64)
	if !ok {
		t.Fatalf("call_counts类型错误")
	}

	if callCounts["GetBlockHeight"] != 1 {
		t.Errorf("GetBlockHeight调用计数错误: 期望1，实际%d", callCounts["GetBlockHeight"])
	}

	if callCounts["GetBlockTimestamp"] != 1 {
		t.Errorf("GetBlockTimestamp调用计数错误: 期望1，实际%d", callCounts["GetBlockTimestamp"])
	}

	if callCounts["GetCaller"] != 1 {
		t.Errorf("GetCaller调用计数错误: 期望1，实际%d", callCounts["GetCaller"])
	}
}

// TestHostRuntimePortsWithStats_CheckCompleteness 测试完整性检查功能
func TestHostRuntimePortsWithStats_CheckCompleteness(t *testing.T) {
	mockABI := &mockHostABI{}
	wrapper := NewHostRuntimePortsWithStats(mockABI)

	missingPrimitives, err := wrapper.CheckCompleteness()
	if err != nil {
		t.Fatalf("检查原语完整性失败: %v", err)
	}

	if len(missingPrimitives) > 0 {
		t.Errorf("发现缺失的原语: %v", missingPrimitives)
	}
}

// ============================================================================
// 测试：HostRuntimePortsWithSecurity包装器
// ============================================================================

// TestHostRuntimePortsWithSecurity_AllPrimitives 测试所有17个原语都被包装
func TestHostRuntimePortsWithSecurity_AllPrimitives(t *testing.T) {
	mockABI := &mockHostABI{}
	callerAddress := make([]byte, 20)
	wrapper := NewHostRuntimePortsWithSecurity(mockABI, callerAddress)
	ctx := context.Background()

	// 测试所有17个原语
	// 类别 A：确定性区块视图（4个）
	_, err := wrapper.GetBlockHeight(ctx)
	if err != nil {
		t.Errorf("GetBlockHeight失败: %v", err)
	}

	_, err = wrapper.GetBlockTimestamp(ctx)
	if err != nil {
		t.Errorf("GetBlockTimestamp失败: %v", err)
	}

	_, err = wrapper.GetBlockHash(ctx, 0)
	if err != nil {
		t.Errorf("GetBlockHash失败: %v", err)
	}

	_, err = wrapper.GetChainID(ctx)
	if err != nil {
		t.Errorf("GetChainID失败: %v", err)
	}

	// 类别 B：执行上下文（3个）
	_, err = wrapper.GetCaller(ctx)
	if err != nil {
		t.Errorf("GetCaller失败: %v", err)
	}

	_, err = wrapper.GetContractAddress(ctx)
	if err != nil {
		t.Errorf("GetContractAddress失败: %v", err)
	}

	_, err = wrapper.GetTransactionID(ctx)
	if err != nil {
		t.Errorf("GetTransactionID失败: %v", err)
	}

	// 类别 C：UTXO查询（2个）
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}
	_, err = wrapper.UTXOLookup(ctx, outpoint)
	if err != nil {
		t.Errorf("UTXOLookup失败: %v", err)
	}

	_, err = wrapper.UTXOExists(ctx, outpoint)
	if err != nil {
		t.Errorf("UTXOExists失败: %v", err)
	}

	// 类别 D：资源查询（2个）
	contentHash := make([]byte, 32)
	_, err = wrapper.ResourceLookup(ctx, contentHash)
	if err != nil {
		t.Errorf("ResourceLookup失败: %v", err)
	}

	_, err = wrapper.ResourceExists(ctx, contentHash)
	if err != nil {
		t.Errorf("ResourceExists失败: %v", err)
	}

	// 类别 E：交易草稿构建（4个）
	_, err = wrapper.TxAddInput(ctx, outpoint, false, nil)
	if err != nil {
		t.Errorf("TxAddInput失败: %v", err)
	}

	owner := make([]byte, 20)
	_, err = wrapper.TxAddAssetOutput(ctx, owner, 0, nil, nil)
	if err != nil {
		t.Errorf("TxAddAssetOutput失败: %v", err)
	}

	_, err = wrapper.TxAddResourceOutput(ctx, contentHash, "", owner, nil, nil)
	if err != nil {
		t.Errorf("TxAddResourceOutput失败: %v", err)
	}

	executionResultHash := make([]byte, 32)
	_, err = wrapper.TxAddStateOutput(ctx, nil, 0, executionResultHash, nil, nil)
	if err != nil {
		t.Errorf("TxAddStateOutput失败: %v", err)
	}

	// 类别 G：执行追踪（2个）
	err = wrapper.EmitEvent(ctx, "", nil)
	if err != nil {
		t.Errorf("EmitEvent失败: %v", err)
	}

	err = wrapper.LogDebug(ctx, "")
	if err != nil {
		t.Errorf("LogDebug失败: %v", err)
	}
}

// TestHostRuntimePortsWithSecurity_RateLimit 测试频率限制功能
func TestHostRuntimePortsWithSecurity_RateLimit(t *testing.T) {
	mockABI := &mockHostABI{}
	callerAddress := make([]byte, 20)
	wrapper := NewHostRuntimePortsWithSecurity(mockABI, callerAddress)
	ctx := context.Background()

	// 设置频率限制：每秒最多1次
	wrapper.SetMaxRate("GetBlockHeight", 1)

	// 第一次调用应该成功
	_, err := wrapper.GetBlockHeight(ctx)
	if err != nil {
		t.Errorf("第一次调用失败: %v", err)
	}

	// 立即第二次调用应该失败（频率限制）
	_, err = wrapper.GetBlockHeight(ctx)
	if err == nil {
		t.Error("第二次调用应该失败（频率限制），但没有返回错误")
	}
}

// ============================================================================
// 测试：PrimitiveUsageStats
// ============================================================================

// TestPrimitiveUsageStats_RecordCall 测试记录调用
func TestPrimitiveUsageStats_RecordCall(t *testing.T) {
	stats := NewPrimitiveUsageStats()

	stats.RecordCall("GetBlockHeight")
	stats.RecordCall("GetBlockHeight")
	stats.RecordCall("GetBlockTimestamp")

	result := stats.GetStats()
	callCounts := result["call_counts"].(map[string]uint64)

	if callCounts["GetBlockHeight"] != 2 {
		t.Errorf("GetBlockHeight调用计数错误: 期望2，实际%d", callCounts["GetBlockHeight"])
	}

	if callCounts["GetBlockTimestamp"] != 1 {
		t.Errorf("GetBlockTimestamp调用计数错误: 期望1，实际%d", callCounts["GetBlockTimestamp"])
	}
}

// TestPrimitiveUsageStats_RecordError 测试记录错误
func TestPrimitiveUsageStats_RecordError(t *testing.T) {
	stats := NewPrimitiveUsageStats()

	stats.RecordCall("GetBlockHeight")
	stats.RecordError("GetBlockHeight")
	stats.RecordCall("GetBlockHeight")
	stats.RecordError("GetBlockHeight")

	result := stats.GetStats()
	errorCounts := result["error_counts"].(map[string]uint64)

	if errorCounts["GetBlockHeight"] != 2 {
		t.Errorf("GetBlockHeight错误计数错误: 期望2，实际%d", errorCounts["GetBlockHeight"])
	}
}

// ============================================================================
// 集成测试：完整流程
// ============================================================================

// TestIntegration_StatsAndSecurity 测试统计和安全包装器的集成
func TestIntegration_StatsAndSecurity(t *testing.T) {
	mockABI := &mockHostABI{}
	callerAddress := make([]byte, 20)

	// 创建带统计的包装器
	statsWrapper := NewHostRuntimePortsWithStats(mockABI)

	// 创建带安全的包装器（包装统计包装器）
	securityWrapper := NewHostRuntimePortsWithSecurity(statsWrapper, callerAddress)

	ctx := context.Background()

	// 调用原语
	_, err := securityWrapper.GetBlockHeight(ctx)
	if err != nil {
		t.Errorf("GetBlockHeight失败: %v", err)
	}

	// 验证统计信息
	stats := statsWrapper.GetUsageStats()
	callCounts := stats["call_counts"].(map[string]uint64)

	if callCounts["GetBlockHeight"] != 1 {
		t.Errorf("GetBlockHeight调用计数错误: 期望1，实际%d", callCounts["GetBlockHeight"])
	}
}

