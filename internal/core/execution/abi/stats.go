package abi

import (
	"sync/atomic"
	"time"

	interfaces "github.com/weisyn/v1/internal/core/execution/interfaces"
)

// ABIStats ABI操作统计信息的内部实现。
//
// 收集和维护ABI管理器的各项操作统计数据，用于性能监控、问题诊断和系统优化。
// 该结构为内部使用，包含比接口暴露版本更详细的统计信息。
//
// 统计维度：
//   - 注册统计：ABI注册次数和成功率
//   - 操作统计：编解码操作次数和性能
//   - 时间统计：最后更新时间和操作耗时
//
// 线程安全性：
//   - 使用原子操作更新计数器，确保并发安全
//   - 时间字段在单线程环境下更新
type ABIStats struct {
	// TotalABIs 已注册的ABI总数。
	// 记录系统中当前活跃的ABI定义数量，用于资源使用监控。
	TotalABIs uint64

	// EncodingOperations 编码操作总次数。
	// 包括函数调用编码、参数编码和值编码等所有编码操作。
	// 用于评估编码器的使用频率和性能需求。
	EncodingOperations uint64

	// DecodingOperations 解码操作总次数。
	// 包括返回值解码、参数解码和值解码等所有解码操作。
	// 用于评估解码器的使用频率和性能需求。
	DecodingOperations uint64

	// LastUpdated 统计信息的最后更新时间。
	// 用于判断统计数据的新鲜度和监控系统活跃状态。
	LastUpdated time.Time

	// RegistrationErrors 注册失败次数（扩展统计）。
	// 记录ABI注册过程中发生的错误次数，用于质量监控。
	RegistrationErrors uint64

	// EncodingErrors 编码失败次数（扩展统计）。
	// 记录编码操作中发生的错误次数，用于稳定性监控。
	EncodingErrors uint64

	// DecodingErrors 解码失败次数（扩展统计）。
	// 记录解码操作中发生的错误次数，用于稳定性监控。
	DecodingErrors uint64
}

// NewABIStats 创建新的ABI统计实例。
//
// 初始化统计结构，设置初始时间戳，为统计数据收集做准备。
// 所有计数器初始化为0，时间戳设置为当前时间。
//
// 返回值：
//   - *ABIStats: 初始化完成的统计实例
func NewABIStats() *ABIStats {
	return &ABIStats{
		TotalABIs:          0,
		EncodingOperations: 0,
		DecodingOperations: 0,
		LastUpdated:        time.Now(),
		RegistrationErrors: 0,
		EncodingErrors:     0,
		DecodingErrors:     0,
	}
}

// ToInterfaceStats 转换为接口暴露的精简统计信息。
//
// 将内部详细统计转换为对外接口定义的简化版本，隐藏内部实现细节。
// 只暴露最核心的统计指标，保持接口的简洁性。
//
// 返回值：
//   - *interfaces.ABIStats: 符合接口定义的精简统计信息
//
// 转换规则：
//   - 保留核心操作统计（注册、编码、解码次数）
//   - 隐藏错误统计和时间信息
//   - 确保数据一致性和原子性
func (s *ABIStats) ToInterfaceStats() *interfaces.ABIStats {
	return &interfaces.ABIStats{
		TotalABIs:          atomic.LoadUint64(&s.TotalABIs),
		EncodingOperations: atomic.LoadUint64(&s.EncodingOperations),
		DecodingOperations: atomic.LoadUint64(&s.DecodingOperations),
	}
}

// IncrementRegistrations 增加注册统计计数。
//
// 线程安全地增加ABI注册次数，并更新最后修改时间。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementRegistrations(delta uint64) {
	atomic.AddUint64(&s.TotalABIs, delta)
	s.LastUpdated = time.Now()
}

// IncrementEncodingOps 增加编码操作统计计数。
//
// 线程安全地增加编码操作次数，用于性能监控。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementEncodingOps(delta uint64) {
	atomic.AddUint64(&s.EncodingOperations, delta)
	s.LastUpdated = time.Now()
}

// IncrementDecodingOps 增加解码操作统计计数。
//
// 线程安全地增加解码操作次数，用于性能监控。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementDecodingOps(delta uint64) {
	atomic.AddUint64(&s.DecodingOperations, delta)
	s.LastUpdated = time.Now()
}

// IncrementRegistrationErrors 增加注册错误统计计数。
//
// 记录ABI注册过程中的错误次数，用于质量监控。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementRegistrationErrors(delta uint64) {
	atomic.AddUint64(&s.RegistrationErrors, delta)
	s.LastUpdated = time.Now()
}

// IncrementEncodingErrors 增加编码错误统计计数。
//
// 记录编码操作中的错误次数，用于稳定性监控。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementEncodingErrors(delta uint64) {
	atomic.AddUint64(&s.EncodingErrors, delta)
	s.LastUpdated = time.Now()
}

// IncrementDecodingErrors 增加解码错误统计计数。
//
// 记录解码操作中的错误次数，用于稳定性监控。
//
// 参数：
//   - delta: 增加的数量，通常为1
func (s *ABIStats) IncrementDecodingErrors(delta uint64) {
	atomic.AddUint64(&s.DecodingErrors, delta)
	s.LastUpdated = time.Now()
}

// GetSuccessRate 计算操作成功率。
//
// 根据成功操作次数和错误次数计算成功率，用于系统健康度评估。
//
// 返回值：
//   - encodingSuccessRate: 编码操作成功率（0-1）
//   - decodingSuccessRate: 解码操作成功率（0-1）
//   - registrationSuccessRate: 注册操作成功率（0-1）
func (s *ABIStats) GetSuccessRate() (encodingSuccessRate, decodingSuccessRate, registrationSuccessRate float64) {
	totalEncodings := atomic.LoadUint64(&s.EncodingOperations)
	encodingErrors := atomic.LoadUint64(&s.EncodingErrors)

	totalDecodings := atomic.LoadUint64(&s.DecodingOperations)
	decodingErrors := atomic.LoadUint64(&s.DecodingErrors)

	totalRegistrations := atomic.LoadUint64(&s.TotalABIs)
	registrationErrors := atomic.LoadUint64(&s.RegistrationErrors)

	// 计算编码成功率
	if totalEncodings > 0 {
		encodingSuccessRate = float64(totalEncodings-encodingErrors) / float64(totalEncodings)
	}

	// 计算解码成功率
	if totalDecodings > 0 {
		decodingSuccessRate = float64(totalDecodings-decodingErrors) / float64(totalDecodings)
	}

	// 计算注册成功率
	if totalRegistrations > 0 {
		registrationSuccessRate = float64(totalRegistrations-registrationErrors) / float64(totalRegistrations)
	}

	return encodingSuccessRate, decodingSuccessRate, registrationSuccessRate
}

// Reset 重置所有统计计数器。
//
// 将所有统计数据重置为初始状态，用于统计周期重新开始或系统重启后的清理。
// 注意：此操作不是原子的，应在确保没有并发访问的情况下调用。
func (s *ABIStats) Reset() {
	atomic.StoreUint64(&s.TotalABIs, 0)
	atomic.StoreUint64(&s.EncodingOperations, 0)
	atomic.StoreUint64(&s.DecodingOperations, 0)
	atomic.StoreUint64(&s.RegistrationErrors, 0)
	atomic.StoreUint64(&s.EncodingErrors, 0)
	atomic.StoreUint64(&s.DecodingErrors, 0)
	s.LastUpdated = time.Now()
}
