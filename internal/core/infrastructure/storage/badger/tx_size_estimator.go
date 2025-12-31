// Package badger 提供 BadgerDB 事务大小估算器
package badger

import (
	"sync/atomic"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 确保 TxSizeEstimator 实现了 storage.TxSizeEstimator 接口
var _ storage.TxSizeEstimator = (*TxSizeEstimator)(nil)

// TxSizeEstimator 事务大小估算器
//
// 🎯 **设计目的**：
// - 估算 BadgerDB 事务的大小，避免超过10MB限制
// - 提供事务大小监控和预警
//
// 💡 **使用场景**：
// - 批量写入操作（如UTXO恢复）
// - 大量索引更新
// - 需要精确控制事务大小的场景
//
// ⚠️ **注意事项**：
// - 估算值是近似值，实际大小可能有所不同
// - BadgerDB 默认限制约为 10MB（可配置）
// - 建议在80%阈值时停止添加新操作
type TxSizeEstimator struct {
	currentSize atomic.Uint64
	maxSize     uint64
}

// NewTxSizeEstimator 创建估算器
//
// 参数：
//   - maxSize: BadgerDB事务大小限制（字节），默认10MB
//
// 返回：
//   - *TxSizeEstimator: 估算器实例
func NewTxSizeEstimator(maxSize uint64) *TxSizeEstimator {
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024 // 10MB默认值
	}
	return &TxSizeEstimator{
		maxSize: maxSize,
	}
}

// AddWrite 记录写入操作
//
// 估算规则：
// - 键长度 + 值长度 + 元数据开销（约20字节）
// - 元数据包括：LSM树结构、版本信息等
//
// 参数：
//   - keyLen: 键的长度（字节）
//   - valueLen: 值的长度（字节）
func (e *TxSizeEstimator) AddWrite(keyLen, valueLen int) {
	overhead := 20 // BadgerDB每个条目的元数据开销
	size := uint64(keyLen + valueLen + overhead)
	e.currentSize.Add(size)
}

// AddDelete 记录删除操作
//
// 估算规则：
// - 删除操作在LSM树中也需要写入墓碑标记
// - 开销约为键长度 + 10字节元数据
//
// 参数：
//   - keyLen: 键的长度（字节）
func (e *TxSizeEstimator) AddDelete(keyLen int) {
	overhead := 10 // 删除操作的元数据开销
	size := uint64(keyLen + overhead)
	e.currentSize.Add(size)
}

// GetCurrentSize 获取当前事务大小估算值
//
// 返回：
//   - uint64: 当前估算的事务大小（字节）
func (e *TxSizeEstimator) GetCurrentSize() uint64 {
	return e.currentSize.Load()
}

// IsNearLimit 检查是否接近限制
//
// 阈值：80%
// 当达到80%时，建议停止添加新操作并提交当前事务
//
// 返回：
//   - bool: true表示接近限制，false表示还有空间
func (e *TxSizeEstimator) IsNearLimit() bool {
	return e.GetCurrentSize() >= (e.maxSize * 80 / 100)
}

// Reset 重置估算器
//
// 使用场景：
// - 事务提交后，准备开始新事务
func (e *TxSizeEstimator) Reset() {
	e.currentSize.Store(0)
}

// GetUsagePercent 获取使用百分比
//
// 返回：
//   - float64: 使用百分比（0-100）
func (e *TxSizeEstimator) GetUsagePercent() float64 {
	return float64(e.GetCurrentSize()) * 100 / float64(e.maxSize)
}

// GetMaxSize 获取最大事务大小限制
//
// 返回：
//   - uint64: 最大事务大小（字节）
func (e *TxSizeEstimator) GetMaxSize() uint64 {
	return e.maxSize
}

// GetRemainingSize 获取剩余可用空间
//
// 返回：
//   - uint64: 剩余空间（字节）
func (e *TxSizeEstimator) GetRemainingSize() uint64 {
	current := e.GetCurrentSize()
	if current >= e.maxSize {
		return 0
	}
	return e.maxSize - current
}

