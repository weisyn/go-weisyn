package context

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"sync"
	"time"
)

// DeterministicEnforcer 确定性执行增强器
//
// 🎯 **确定性保证**：
// - 时间戳固定：执行期间时间戳不变
// - 随机数种子固定：为每次执行设置固定的随机数种子
// - 执行结果一致性验证：验证相同输入产生相同输出
type DeterministicEnforcer struct {
	// 固定时间戳（执行期间不变）
	fixedTimestamp time.Time
	// 固定随机数种子（基于执行ID和输入参数生成）
	fixedRandomSeed int64
	// 执行结果哈希（用于一致性验证）
	executionResultHash []byte
	// 执行输入哈希（用于一致性验证）
	executionInputHash []byte
	mutex              sync.RWMutex
}

// NewDeterministicEnforcer 创建确定性执行增强器
//
// 🎯 **确定性初始化**：
// - 固定时间戳：使用创建时的时间（或从ExecutionContext获取）
// - 固定随机数种子：基于executionID和输入参数生成确定性种子
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - inputParams: 执行输入参数（用于生成确定性种子）
//   - fixedTimestamp: 固定时间戳（如果为nil，使用当前时间）
func NewDeterministicEnforcer(executionID string, inputParams []byte, fixedTimestamp *time.Time) *DeterministicEnforcer {
	// 确定固定时间戳
	var timestamp time.Time
	if fixedTimestamp != nil {
		timestamp = *fixedTimestamp
	} else {
		timestamp = time.Now()
	}

	// 生成确定性随机数种子
	// 基于executionID和inputParams生成SHA-256哈希，取前8字节作为int64种子
	seed := generateDeterministicSeed(executionID, inputParams)

	// 计算执行输入哈希（用于一致性验证）
	inputHash := computeInputHash(executionID, inputParams, timestamp)

	return &DeterministicEnforcer{
		fixedTimestamp:     timestamp,
		fixedRandomSeed:     seed,
		executionInputHash:  inputHash,
		executionResultHash: nil, // 执行完成后设置
	}
}

// generateDeterministicSeed 生成确定性随机数种子
func generateDeterministicSeed(executionID string, inputParams []byte) int64 {
	h := sha256.New()
	h.Write([]byte(executionID))
	if inputParams != nil {
		h.Write(inputParams)
	}
	hash := h.Sum(nil)

	// 取前8字节作为int64种子
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	return seed
}

// computeInputHash 计算执行输入哈希
func computeInputHash(executionID string, inputParams []byte, timestamp time.Time) []byte {
	h := sha256.New()
	h.Write([]byte(executionID))
	if inputParams != nil {
		h.Write(inputParams)
	}
	// 添加时间戳（确定性）
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(timestamp.UnixNano()))
	h.Write(timestampBytes)
	return h.Sum(nil)
}

// GetFixedTimestamp 获取固定时间戳
//
// 🎯 **时间戳固定**：
// - 执行期间返回相同的时间戳
// - 确保相同输入产生相同的时间相关结果
func (d *DeterministicEnforcer) GetFixedTimestamp() time.Time {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.fixedTimestamp
}

// GetFixedRandomSeed 获取固定随机数种子
//
// 🎯 **随机数种子固定**：
// - 基于executionID和输入参数生成确定性种子
// - 确保相同输入产生相同的随机数序列
func (d *DeterministicEnforcer) GetFixedRandomSeed() int64 {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.fixedRandomSeed
}

// SetExecutionResultHash 设置执行结果哈希
//
// 🎯 **结果哈希记录**：
// - 在执行完成后调用
// - 用于后续的一致性验证
func (d *DeterministicEnforcer) SetExecutionResultHash(resultHash []byte) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.executionResultHash = resultHash
}

// VerifyExecutionConsistency 验证执行结果一致性
//
// 🎯 **一致性验证**：
// - 比较当前执行结果哈希与预期哈希
// - 如果不同，说明执行结果不一致
//
// 📋 **参数**：
//   - currentResultHash: 当前执行结果哈希
//
// 🔧 **返回值**：
//   - consistent: 是否一致
//   - error: 验证过程中的错误
func (d *DeterministicEnforcer) VerifyExecutionConsistency(currentResultHash []byte) (consistent bool, err error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.executionResultHash == nil {
		// 第一次执行，记录结果哈希
		return true, nil
	}

	// 比较哈希
	if len(currentResultHash) != len(d.executionResultHash) {
		return false, fmt.Errorf("执行结果哈希长度不一致: 当前=%d, 预期=%d", len(currentResultHash), len(d.executionResultHash))
	}

	for i := range currentResultHash {
		if currentResultHash[i] != d.executionResultHash[i] {
			return false, fmt.Errorf("执行结果哈希不一致: 位置=%d, 当前=%x, 预期=%x", i, currentResultHash[i], d.executionResultHash[i])
		}
	}

	return true, nil
}

// GetExecutionInputHash 获取执行输入哈希
func (d *DeterministicEnforcer) GetExecutionInputHash() []byte {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.executionInputHash
}

// DeterministicRandomSource 确定性随机数源
//
// 🎯 **确定性随机数**：
// - 基于固定种子生成随机数
// - 确保相同种子产生相同的随机数序列
type DeterministicRandomSource struct {
	seed   int64
	hasher hash.Hash
	mutex  sync.Mutex
}

// NewDeterministicRandomSource 创建确定性随机数源
func NewDeterministicRandomSource(seed int64) *DeterministicRandomSource {
	hasher := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	hasher.Write(seedBytes)

	return &DeterministicRandomSource{
		seed:   seed,
		hasher: hasher,
	}
}

// Read 读取随机字节（确定性实现）
//
// 🎯 **确定性随机数生成**：
// - 使用SHA-256哈希链生成随机字节
// - 每次调用都会更新哈希状态，确保序列的确定性
func (r *DeterministicRandomSource) Read(p []byte) (n int, err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 使用当前哈希状态生成随机字节
	hash := r.hasher.Sum(nil)
	copy(p, hash)

	// 更新哈希状态（为下一次调用准备）
	r.hasher.Reset()
	r.hasher.Write(hash)

	return len(p), nil
}

// Int63 生成63位随机整数（确定性实现）
func (r *DeterministicRandomSource) Int63() int64 {
	var buf [8]byte
	r.Read(buf[:])
	// 取前7字节（63位），最高位设为0
	buf[7] &= 0x7F
	return int64(binary.BigEndian.Uint64(buf[:]))
}

// Seed 设置随机数种子
func (r *DeterministicRandomSource) Seed(seed int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.seed = seed
	r.hasher.Reset()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	r.hasher.Write(seedBytes)
}

// ExecutionResultVerifier 执行结果一致性验证器
type ExecutionResultVerifier struct {
	// 执行结果记录（按输入哈希索引）
	resultRecords map[string]*executionResultRecord
	mutex         sync.RWMutex
}

// executionResultRecord 执行结果记录
type executionResultRecord struct {
	inputHash      []byte
	resultHash     []byte
	executionCount uint64
	firstSeenAt    time.Time
	lastSeenAt     time.Time
}

// NewExecutionResultVerifier 创建执行结果一致性验证器
func NewExecutionResultVerifier() *ExecutionResultVerifier {
	return &ExecutionResultVerifier{
		resultRecords: make(map[string]*executionResultRecord),
	}
}

// RecordExecutionResult 记录执行结果
//
// 🎯 **结果记录**：
// - 记录输入哈希和结果哈希的映射
// - 跟踪执行次数和时间
func (v *ExecutionResultVerifier) RecordExecutionResult(inputHash, resultHash []byte) error {
	if inputHash == nil || resultHash == nil {
		return fmt.Errorf("输入哈希或结果哈希不能为nil")
	}

	v.mutex.Lock()
	defer v.mutex.Unlock()

	inputHashStr := fmt.Sprintf("%x", inputHash)
	record, exists := v.resultRecords[inputHashStr]

	if exists {
		// 验证结果一致性
		if !compareHashes(resultHash, record.resultHash) {
			return fmt.Errorf("执行结果不一致: 输入哈希=%x, 当前结果=%x, 预期结果=%x", inputHash, resultHash, record.resultHash)
		}
		// 更新记录
		record.executionCount++
		record.lastSeenAt = time.Now()
	} else {
		// 创建新记录
		v.resultRecords[inputHashStr] = &executionResultRecord{
			inputHash:      inputHash,
			resultHash:     resultHash,
			executionCount: 1,
			firstSeenAt:    time.Now(),
			lastSeenAt:     time.Now(),
		}
	}

	return nil
}

// VerifyExecutionResult 验证执行结果一致性
func (v *ExecutionResultVerifier) VerifyExecutionResult(inputHash, resultHash []byte) (consistent bool, err error) {
	if inputHash == nil || resultHash == nil {
		return false, fmt.Errorf("输入哈希或结果哈希不能为nil")
	}

	v.mutex.RLock()
	defer v.mutex.RUnlock()

	inputHashStr := fmt.Sprintf("%x", inputHash)
	record, exists := v.resultRecords[inputHashStr]

	if !exists {
		// 第一次执行，无法验证
		return true, nil
	}

	consistent = compareHashes(resultHash, record.resultHash)
	if !consistent {
		err = fmt.Errorf("执行结果不一致: 输入哈希=%x, 当前结果=%x, 预期结果=%x", inputHash, resultHash, record.resultHash)
	}

	return consistent, err
}

// compareHashes 比较两个哈希是否相等
func compareHashes(hash1, hash2 []byte) bool {
	if len(hash1) != len(hash2) {
		return false
	}
	for i := range hash1 {
		if hash1[i] != hash2[i] {
			return false
		}
	}
	return true
}

// GetExecutionStats 获取执行统计信息
func (v *ExecutionResultVerifier) GetExecutionStats() map[string]interface{} {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	totalExecutions := uint64(0)
	consistentExecutions := uint64(0)

	for _, record := range v.resultRecords {
		totalExecutions += record.executionCount
		if record.executionCount > 1 {
			consistentExecutions += record.executionCount - 1 // 第一次不算一致性验证
		}
	}

	return map[string]interface{}{
		"total_records":        len(v.resultRecords),
		"total_executions":     totalExecutions,
		"consistent_executions": consistentExecutions,
	}
}

// contextImpl 扩展：添加确定性增强字段
// 注意：这个扩展需要在contextImpl中添加字段，但由于contextImpl在manager.go中定义，
// 我们通过组合的方式在isolation.go中提供辅助功能

// EnsureDeterministicTimestamp 确保时间戳固定
//
// 🎯 **时间戳固定**：
// - 在ExecutionContext中固定时间戳
// - 确保执行期间时间戳不变
func EnsureDeterministicTimestamp(ctx *contextImpl, enforcer *DeterministicEnforcer) {
	if enforcer == nil {
		return
	}

	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// 如果ExecutionContext已经有固定时间戳，使用它
	// 否则使用enforcer的固定时间戳
	if ctx.createdAt.IsZero() {
		ctx.createdAt = enforcer.GetFixedTimestamp()
	}
}

// EnsureDeterministicRandomSeed 确保随机数种子固定
//
// 🎯 **随机数种子固定**：
// - 为ExecutionContext设置固定的随机数种子
// - 确保相同输入产生相同的随机数序列
func EnsureDeterministicRandomSeed(ctx *contextImpl, enforcer *DeterministicEnforcer) *DeterministicRandomSource {
	if enforcer == nil {
		return nil
	}

	seed := enforcer.GetFixedRandomSeed()
	return NewDeterministicRandomSource(seed)
}

// VerifyExecutionResultConsistency 验证执行结果一致性
//
// 🎯 **一致性验证**：
// - 比较当前执行结果与历史执行结果
// - 确保相同输入产生相同输出
func VerifyExecutionResultConsistency(
	ctx *contextImpl,
	enforcer *DeterministicEnforcer,
	verifier *ExecutionResultVerifier,
	resultHash []byte,
) error {
	if enforcer == nil || verifier == nil {
		return nil // 如果未启用确定性增强，跳过验证
	}

	// 获取执行输入哈希
	inputHash := enforcer.GetExecutionInputHash()

	// 记录执行结果
	if err := verifier.RecordExecutionResult(inputHash, resultHash); err != nil {
		return fmt.Errorf("记录执行结果失败: %w", err)
	}

	// 验证执行结果一致性
	consistent, err := enforcer.VerifyExecutionConsistency(resultHash)
	if err != nil {
		return fmt.Errorf("验证执行结果一致性失败: %w", err)
	}

	if !consistent {
		return fmt.Errorf("执行结果不一致: 输入哈希=%x, 结果哈希=%x", inputHash, resultHash)
	}

	return nil
}

