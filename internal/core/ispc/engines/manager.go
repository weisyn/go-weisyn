package engines

import (
	"context"
	"encoding/hex"
	"fmt"
	"runtime"
	"sync"
	"time"

	hostabi "github.com/weisyn/v1/internal/core/ispc/hostabi"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Manager 引擎统一管理器
//
// 🎯 **设计原则**：
// - 统一管理WASM和ONNX引擎
// - 实现InternalEngineManager接口
// - 作为coordinator和具体引擎之间的桥梁
// - 提供优雅关闭机制，确保资源正确释放
// - 提供错误隔离和健康检查机制
// - P1: 提供执行结果缓存，提升性能
// - P1: 支持引擎注册机制，实现可扩展性增强
type Manager struct {
	logger     log.Logger
	wasmEngine ispcInterfaces.InternalWASMEngine
	onnxEngine ispcInterfaces.InternalONNXEngine

	// P1: 引擎注册表（可扩展性增强）
	registry *Registry // 引擎注册表

	// 生命周期管理 - 优雅关闭
	shutdownOnce   sync.Once          // 确保只关闭一次
	shutdownMutex  sync.RWMutex       // 保护关闭状态
	isShutdown     bool               // 是否已关闭
	shutdownCtx    context.Context    // 关闭上下文（用于取消正在执行的请求）
	shutdownCancel context.CancelFunc // 取消函数

	// 执行请求跟踪
	activeRequests sync.WaitGroup // 跟踪正在执行的请求数量

	// P0: 错误处理和恢复
	wasmHealth  *EngineHealth // WASM引擎健康信息
	onnxHealth  *EngineHealth // ONNX引擎健康信息
	healthMutex sync.RWMutex  // 保护健康信息的并发访问

	// P1: 执行结果缓存
	executionCache *ExecutionResultCache // 执行结果缓存
	cacheEnabled   bool                  // 是否启用缓存

	// 健康检查配置
	healthCheckConfig HealthCheckConfig // 健康检查配置
}

type wasmCacheValue struct {
	ReturnValues []uint64
	ReturnData   []byte
	Events       []*ispcInterfaces.Event
}

// NewManager 创建引擎统一管理器
func NewManager(
	logger log.Logger,
	wasmEngine ispcInterfaces.InternalWASMEngine,
	onnxEngine ispcInterfaces.InternalONNXEngine,
) (*Manager, error) {
	return NewManagerWithCache(logger, wasmEngine, onnxEngine, true, 1000, 5*time.Minute)
}

// NewManagerWithHealthCheck 创建引擎统一管理器（带健康检查配置）
//
// 📋 **参数**：
//   - logger: 日志记录器
//   - wasmEngine: WASM引擎实例
//   - onnxEngine: ONNX引擎实例
//   - healthCheckConfig: 健康检查配置
//
// 🎯 **健康检查配置说明**：
//   - 参考 onnxruntime_go 的错误处理模式
//   - 错误应该被记录和统计，但不应该阻止后续请求
//   - 健康检查应该用于监控和告警，而不是阻止执行
func NewManagerWithHealthCheck(
	logger log.Logger,
	wasmEngine ispcInterfaces.InternalWASMEngine,
	onnxEngine ispcInterfaces.InternalONNXEngine,
	healthCheckConfig HealthCheckConfig,
) (*Manager, error) {
	manager, err := NewManager(logger, wasmEngine, onnxEngine)
	if err != nil {
		return nil, err
	}
	manager.healthCheckConfig = healthCheckConfig
	return manager, nil
}

// NewManagerWithCache 创建引擎统一管理器（带缓存配置）
//
// 📋 **参数**：
//   - logger: 日志记录器
//   - wasmEngine: WASM引擎实例
//   - onnxEngine: ONNX引擎实例
//   - enableCache: 是否启用执行结果缓存
//   - cacheSize: 缓存最大条目数
//   - cacheTTL: 缓存生存时间
func NewManagerWithCache(
	logger log.Logger,
	wasmEngine ispcInterfaces.InternalWASMEngine,
	onnxEngine ispcInterfaces.InternalONNXEngine,
	enableCache bool,
	cacheSize int,
	cacheTTL time.Duration,
) (*Manager, error) {
	if wasmEngine == nil {
		return nil, fmt.Errorf("wasmEngine cannot be nil")
	}
	// ⚠️ 允许 onnxEngine 为 nil（平台不支持时）
	// 如果为 nil，ONNX 功能将不可用，但系统可以正常运行
	if onnxEngine == nil {
		if logger != nil {
			logger.Warn("⚠️ ONNX 引擎为 nil，ONNX AI 推理功能将不可用")
		}
	}

	// 创建关闭上下文（初始时不会被取消）
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	manager := &Manager{
		logger:         logger,
		wasmEngine:     wasmEngine,
		onnxEngine:     onnxEngine,
		registry:       NewRegistry(), // P1: 初始化引擎注册表
		isShutdown:     false,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		wasmHealth: &EngineHealth{
			Status:      EngineHealthHealthy,
			LastCheck:   time.Now(),
			ErrorStats:  NewEngineErrorStats(),
			IsAvailable: true,
		},
		onnxHealth: &EngineHealth{
			Status:      EngineHealthHealthy,
			LastCheck:   time.Now(),
			ErrorStats:  NewEngineErrorStats(),
			IsAvailable: true,
		},
		cacheEnabled: enableCache,
		// 默认健康检查配置：参考 onnxruntime_go 的错误处理模式
		// 错误应该被记录和统计，但不应该阻止后续请求
		healthCheckConfig: HealthCheckConfig{
			Enabled:            false, // 默认禁用健康检查阻止机制（仅用于监控）
			UnhealthyThreshold: 10,
			DegradedThreshold:  3,
		},
	}

	// P1: 注册WASM和ONNX引擎到注册表
	wasmAdapter := NewWASMEngineAdapter(wasmEngine)
	if err := manager.registry.Register(wasmAdapter); err != nil {
		return nil, fmt.Errorf("failed to register WASM engine: %w", err)
	}

	// 仅在 ONNX 引擎存在时注册
	if onnxEngine != nil {
		onnxAdapter := NewONNXEngineAdapter(onnxEngine)
		if err := manager.registry.Register(onnxAdapter); err != nil {
			return nil, fmt.Errorf("failed to register ONNX engine: %w", err)
		}
	} else {
		// ONNX 引擎不可用，设置健康状态为不可用
		manager.onnxHealth.Status = EngineHealthUnhealthy
		manager.onnxHealth.IsAvailable = false
		if logger != nil {
			logger.Info("ℹ️ ONNX 引擎未注册（平台不支持），ONNX AI 推理功能不可用")
		}
	}

	// 初始化执行结果缓存
	if enableCache {
		manager.executionCache = NewExecutionResultCache(logger, cacheSize, cacheTTL)
		if logger != nil {
			logger.Infof("✅ 执行结果缓存已启用: size=%d, ttl=%v", cacheSize, cacheTTL)
		}
	}

	return manager, nil
}

// mergeContexts 合并执行上下文和关闭上下文
//
// 🎯 **优雅合并**：
// - 如果关闭上下文被取消，执行上下文也会被取消
// - 如果原始上下文被取消，执行上下文也会被取消
// - 确保goroutine正确清理，不会泄漏
func (m *Manager) mergeContexts(ctx context.Context) (context.Context, context.CancelFunc) {
	// 创建一个可取消的上下文
	mergedCtx, cancel := context.WithCancel(ctx)

	// 启动goroutine监听两个上下文的取消信号
	go func() {
		select {
		case <-m.shutdownCtx.Done():
			// 关闭信号：取消执行
			cancel()
		case <-ctx.Done():
			// 原始上下文取消：取消执行
			cancel()
		case <-mergedCtx.Done():
			// 执行完成：goroutine自动退出
		}
	}()

	return mergedCtx, cancel
}

// ExecuteWASM 执行WASM合约
//
// 🎯 **错误隔离**：
// - WASM引擎的错误不会影响ONNX引擎
// - 错误会被记录到WASM引擎的错误统计中
// - 连续错误会导致引擎状态降级
func (m *Manager) ExecuteWASM(
	ctx context.Context,
	hash []byte,
	method string,
	params []uint64,
) ([]uint64, error) {
	// 阶段1: 检查是否已关闭（快速路径）
	m.shutdownMutex.RLock()
	if m.isShutdown {
		m.shutdownMutex.RUnlock()
		return nil, fmt.Errorf("引擎管理器已关闭，无法执行WASM合约")
	}
	m.shutdownMutex.RUnlock()

	// 阶段2: 检查WASM引擎健康状态
	m.healthMutex.RLock()
	wasmAvailable := m.wasmHealth.IsAvailable
	m.healthMutex.RUnlock()

	if !wasmAvailable {
		return nil, fmt.Errorf("WASM引擎当前不可用，请检查引擎状态")
	}

	// 阶段3: 增加正在执行的请求计数
	// 注意：必须在检查关闭状态之后增加，避免关闭时增加计数
	m.activeRequests.Add(1)
	defer m.activeRequests.Done()

	// 阶段4: 合并执行上下文和关闭上下文
	execCtx, cancel := m.mergeContexts(ctx)
	defer cancel()

	// P1: 执行结果缓存
	if m.cacheEnabled && m.executionCache != nil {
		// 构建缓存键
		contractID := hex.EncodeToString(hash)
		cacheKey := BuildCacheKey("wasm", contractID, method, params)

		// 尝试从缓存获取
		if cachedResult, cachedErr, found := m.executionCache.Get(cacheKey); found {
			if m.logger != nil {
				m.logger.Debugf("✅ WASM执行结果缓存命中: contract=%s, method=%s", contractID, method)
			}
			if cachedErr != nil {
				return nil, cachedErr
			}
			switch cached := cachedResult.(type) {
			case *wasmCacheValue:
				m.restoreCachedWASMResult(execCtx, cached)
				return cloneUint64Slice(cached.ReturnValues), nil
			case []uint64:
				// 向后兼容：早期缓存只包含返回值
				return cloneUint64Slice(cached), nil
			default:
				if m.logger != nil {
					m.logger.Warnf("⚠️ WASM缓存命中但类型不匹配: %T", cachedResult)
				}
			}
		}
	}

	// 阶段5: 执行WASM合约（带错误记录）
	result, err := m.wasmEngine.CallFunction(execCtx, hash, method, params)

	// P1: 缓存执行结果（仅缓存成功的结果）
	if m.cacheEnabled && m.executionCache != nil && err == nil {
		contractID := hex.EncodeToString(hash)
		cacheKey := BuildCacheKey("wasm", contractID, method, params)
		cacheValue := &wasmCacheValue{
			ReturnValues: cloneUint64Slice(result),
		}

		if exec := hostabi.GetExecutionContext(execCtx); exec != nil {
			if data, dataErr := exec.GetReturnData(); dataErr == nil && len(data) > 0 {
				cacheValue.ReturnData = cloneBytes(data)
			} else if dataErr != nil && m.logger != nil {
				m.logger.Warnf("⚠️ 获取执行返回数据失败（缓存跳过）: %v", dataErr)
			}

			if events, eventsErr := exec.GetEvents(); eventsErr == nil && len(events) > 0 {
				cacheValue.Events = cloneEvents(events)
			} else if eventsErr != nil && m.logger != nil {
				m.logger.Warnf("⚠️ 获取执行事件失败（缓存跳过）: %v", eventsErr)
			}
		} else if m.logger != nil {
			m.logger.Warn("⚠️ 无法从上下文获取 ExecutionContext，缓存仅包含返回值")
		}

		m.executionCache.Set(cacheKey, cacheValue, nil, 0) // 使用默认TTL
	}

	// 阶段6: 记录执行结果（用于健康检查）
	if err != nil {
		m.healthMutex.Lock()
		m.wasmHealth.ErrorStats.RecordError(err)
		updateWASMHealthStatus(m.wasmHealth, m.healthCheckConfig, m.logger)
		m.healthMutex.Unlock()
	} else {
		m.healthMutex.Lock()
		m.wasmHealth.ErrorStats.RecordSuccess()
		updateWASMHealthStatus(m.wasmHealth, m.healthCheckConfig, m.logger)
		m.healthMutex.Unlock()
	}

	return result, err
}

// ExecuteONNX 执行ONNX模型推理
//
// 🎯 **错误隔离**：
// - ONNX引擎的错误不会影响WASM引擎
// - 错误会被记录到ONNX引擎的错误统计中
// - 连续错误会导致引擎状态降级
func (m *Manager) ExecuteONNX(
	ctx context.Context,
	hash []byte,
	tensorInputs []ispcInterfaces.TensorInput,
) ([]ispcInterfaces.TensorOutput, error) {
	// 阶段1: 检查是否已关闭（快速路径）
	m.shutdownMutex.RLock()
	if m.isShutdown {
		m.shutdownMutex.RUnlock()
		return nil, fmt.Errorf("引擎管理器已关闭，无法执行ONNX模型")
	}
	m.shutdownMutex.RUnlock()

	// 阶段2: 检查ONNX引擎是否存在和健康状态
	if m.onnxEngine == nil {
		return nil, fmt.Errorf("ONNX引擎不可用：当前平台 (%s_%s) 不支持 ONNX Runtime", runtime.GOOS, runtime.GOARCH)
	}

	m.healthMutex.RLock()
	onnxAvailable := m.onnxHealth.IsAvailable
	m.healthMutex.RUnlock()

	if !onnxAvailable {
		return nil, fmt.Errorf("ONNX引擎当前不可用，请检查引擎状态")
	}

	// 阶段3: 增加正在执行的请求计数
	// 注意：必须在检查关闭状态之后增加，避免关闭时增加计数
	m.activeRequests.Add(1)
	defer m.activeRequests.Done()

	// 阶段4: 合并执行上下文和关闭上下文
	execCtx, cancel := m.mergeContexts(ctx)
	defer cancel()

	// P1: 执行结果缓存
	if m.cacheEnabled && m.executionCache != nil {
		// 构建缓存键
		modelID := hex.EncodeToString(hash)
		// 将TensorInput转换为[][]float64用于缓存键（临时方案）
		inputsForCache := make([][]float64, len(tensorInputs))
		for i, ti := range tensorInputs {
			inputsForCache[i] = ti.Data
		}
		cacheKey := BuildCacheKey("onnx", modelID, "", inputsForCache)

		// 尝试从缓存获取
		if cachedResult, cachedErr, found := m.executionCache.Get(cacheKey); found {
			if m.logger != nil {
				m.logger.Debugf("✅ ONNX执行结果缓存命中: model=%s", modelID)
			}
			if cachedErr != nil {
				return nil, cachedErr
			}
			if result, ok := cachedResult.([]ispcInterfaces.TensorOutput); ok {
				return result, nil
			}
		}
	}

	// 阶段5: 执行ONNX模型推理（带错误记录）
	result, err := m.onnxEngine.CallModel(execCtx, hash, tensorInputs)

	// P1: 缓存执行结果（仅缓存成功的结果）
	if m.cacheEnabled && m.executionCache != nil && err == nil {
		modelID := hex.EncodeToString(hash)
		// 将TensorInput转换为[][]float64用于缓存键（临时方案）
		inputsForCache := make([][]float64, len(tensorInputs))
		for i, ti := range tensorInputs {
			inputsForCache[i] = ti.Data
		}
		cacheKey := BuildCacheKey("onnx", modelID, "", inputsForCache)
		m.executionCache.Set(cacheKey, result, nil, 0) // 使用默认TTL
	}

	// 阶段6: 记录执行结果（用于健康检查）
	if err != nil {
		m.healthMutex.Lock()
		m.onnxHealth.ErrorStats.RecordError(err)
		updateONNXHealthStatus(m.onnxHealth, m.healthCheckConfig, m.logger)
		m.healthMutex.Unlock()
	} else {
		m.healthMutex.Lock()
		m.onnxHealth.ErrorStats.RecordSuccess()
		updateONNXHealthStatus(m.onnxHealth, m.healthCheckConfig, m.logger)
		m.healthMutex.Unlock()
	}

	return result, err
}

// CheckHealth 检查引擎健康状态
//
// 🎯 **健康检查**：
// - 返回WASM和ONNX引擎的健康状态
// - 用于开发/调试阶段验证引擎状态
// - 不用于生产监控（区块链系统不需要生产监控）
//
// 📋 **返回值**：
//   - wasmHealth: WASM引擎健康信息
//   - onnxHealth: ONNX引擎健康信息
func (m *Manager) CheckHealth() (wasmHealth *EngineHealth, onnxHealth *EngineHealth) {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()

	// 返回健康信息的副本（避免外部修改）
	wasmCopy := &EngineHealth{
		Status:      m.wasmHealth.Status,
		LastCheck:   m.wasmHealth.LastCheck,
		ErrorStats:  m.wasmHealth.ErrorStats, // ErrorStats内部有mutex保护
		IsAvailable: m.wasmHealth.IsAvailable,
	}

	onnxCopy := &EngineHealth{
		Status:      m.onnxHealth.Status,
		LastCheck:   m.onnxHealth.LastCheck,
		ErrorStats:  m.onnxHealth.ErrorStats, // ErrorStats内部有mutex保护
		IsAvailable: m.onnxHealth.IsAvailable,
	}

	return wasmCopy, onnxCopy
}

// GetErrorStats 获取错误统计信息
//
// 🎯 **错误统计**：
// - 返回WASM和ONNX引擎的错误统计
// - 用于问题诊断和性能分析
//
// 📋 **返回值**：
//   - wasmStats: WASM引擎错误统计
//   - onnxStats: ONNX引擎错误统计
func (m *Manager) GetErrorStats() (wasmStats map[string]interface{}, onnxStats map[string]interface{}) {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()

	totalWASM, errorByTypeWASM, lastErrorTimeWASM, consecutiveWASM, lastErrorWASM := m.wasmHealth.ErrorStats.GetStats()
	totalONNX, errorByTypeONNX, lastErrorTimeONNX, consecutiveONNX, lastErrorONNX := m.onnxHealth.ErrorStats.GetStats()

	wasmStats = map[string]interface{}{
		"total_errors":       totalWASM,
		"error_by_type":      errorByTypeWASM,
		"last_error_time":    lastErrorTimeWASM,
		"last_error":         nil,
		"consecutive_errors": consecutiveWASM,
		"status":             string(m.wasmHealth.Status),
		"is_available":       m.wasmHealth.IsAvailable,
	}
	if lastErrorWASM != nil {
		wasmStats["last_error"] = lastErrorWASM.Error()
	}

	onnxStats = map[string]interface{}{
		"total_errors":       totalONNX,
		"error_by_type":      errorByTypeONNX,
		"last_error_time":    lastErrorTimeONNX,
		"last_error":         nil,
		"consecutive_errors": consecutiveONNX,
		"status":             string(m.onnxHealth.Status),
		"is_available":       m.onnxHealth.IsAvailable,
	}
	if lastErrorONNX != nil {
		onnxStats["last_error"] = lastErrorONNX.Error()
	}

	return wasmStats, onnxStats
}

// GetRegistry 获取引擎注册表
//
// 🎯 **可扩展性增强**：
// - 提供访问引擎注册表的接口
// - 允许外部代码注册新的引擎类型
// - 支持动态引擎管理
//
// 📋 **返回值**：
//   - *Registry: 引擎注册表实例
func (m *Manager) GetRegistry() *Registry {
	return m.registry
}

// RegisterEngine 注册新引擎
//
// 🎯 **可扩展性增强**：
// - 允许动态注册新的引擎类型
// - 支持插件化架构
//
// 📋 **参数**：
//   - engine: 引擎实例（必须实现Engine接口）
//
// 📋 **返回值**：
//   - error: 注册错误（如引擎类型已存在）
func (m *Manager) RegisterEngine(engine ispcInterfaces.Engine) error {
	return m.registry.Register(engine)
}

// GetEngine 获取指定类型的引擎
//
// 🎯 **可扩展性增强**：
// - 通过引擎类型查找引擎实例
// - 支持动态引擎查找
//
// 📋 **参数**：
//   - engineType: 引擎类型
//
// 📋 **返回值**：
//   - ispcInterfaces.Engine: 引擎实例（如果存在）
//   - bool: 是否存在
func (m *Manager) GetEngine(engineType ispcInterfaces.EngineType) (ispcInterfaces.Engine, bool) {
	return m.registry.Get(engineType)
}

// ListEngines 列出所有已注册的引擎
//
// 🎯 **可扩展性增强**：
// - 返回所有已注册引擎的元数据
// - 支持引擎发现
//
// 📋 **返回值**：
//   - []ispcInterfaces.EngineMetadata: 所有引擎的元数据列表
func (m *Manager) ListEngines() []ispcInterfaces.EngineMetadata {
	return m.registry.List()
}

// GetCacheStats 获取执行结果缓存统计信息
//
// 🎯 **缓存统计**：
// - 返回执行结果缓存的统计信息
// - 用于性能分析和缓存优化
//
// 📋 **返回值**：
//   - map[string]interface{}: 缓存统计信息（如果缓存未启用则返回nil）
func (m *Manager) GetCacheStats() map[string]interface{} {
	if !m.cacheEnabled || m.executionCache == nil {
		return nil
	}
	return m.executionCache.GetStats()
}

// ClearCache 清空执行结果缓存
//
// 🎯 **缓存清理**：
// - 清空所有缓存的执行结果
// - 重置缓存统计信息
func (m *Manager) ClearCache() {
	if m.executionCache != nil {
		m.executionCache.Clear()
		if m.logger != nil {
			m.logger.Info("✅ 执行结果缓存已清空")
		}
	}
}

// ShrinkCache 主动裁剪执行结果缓存（供 MemoryDoctor 调用）
func (m *Manager) ShrinkCache(targetSize int) {
	if !m.cacheEnabled || m.executionCache == nil {
		return
	}

	if targetSize <= 0 {
		targetSize = 1
	}

	if m.logger != nil {
		m.logger.Warnf("MemoryDoctor 触发 ISPC Engines 执行结果缓存收缩: targetSize=%d", targetSize)
	}

	// 当前 ExecutionResultCache 尚未暴露精细容量控制接口，这里采用快速清空方式：
	// - 清空所有缓存条目
	// - 保留容量和 TTL 配置，后续按需重新填充热点执行结果
	m.executionCache.Clear()
}

// Shutdown 关闭引擎管理器，释放所有资源
//
// 🎯 **优雅关闭流程**（6个阶段）：
// 1. 设置 isShutdown = true，停止接受新的执行请求
// 2. 取消关闭上下文，通知正在执行的请求尽快完成
// 3. 等待所有正在执行的请求完成（通过 WaitGroup 和超时控制）
// 4. 关闭WASM引擎，释放资源
// 5. 关闭ONNX引擎，释放资源
// 6. 清理资源，确保没有泄漏
//
// 📋 **参数**：
//   - ctx: 关闭上下文（用于控制关闭超时，建议至少30秒）
//
// 🔧 **返回值**：
//   - error: 关闭过程中的错误（如果有）
//
// ⚠️ **注意**：
//   - 关闭后管理器不能再使用
//   - 多次调用是安全的（使用sync.Once保证只执行一次）
//   - 如果超时，会强制关闭，但会记录警告日志
//   - 关闭过程中的错误不会阻止关闭流程
func (m *Manager) Shutdown(ctx context.Context) error {
	var shutdownErr error

	m.shutdownOnce.Do(func() {
		if m.logger != nil {
			m.logger.Info("🔄 开始关闭引擎管理器（优雅关闭）...")
		}

		// 阶段1: 设置关闭标志，停止接受新的执行请求
		m.shutdownMutex.Lock()
		m.isShutdown = true
		m.shutdownMutex.Unlock()

		if m.logger != nil {
			m.logger.Info("📋 阶段1/6: 已停止接受新的执行请求")
		}

		// 阶段2: 取消关闭上下文，通知正在执行的请求尽快完成
		if m.shutdownCancel != nil {
			m.shutdownCancel()
			if m.logger != nil {
				m.logger.Info("📢 阶段2/6: 已发送关闭信号，通知正在执行的请求尽快完成")
			}
		}

		// 阶段3: 等待所有正在执行的请求完成（带超时）
		// 使用传入的ctx超时，如果没有超时则使用默认30秒
		waitCtx := ctx
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
		}

		done := make(chan struct{})
		go func() {
			m.activeRequests.Wait()
			close(done)
		}()

		select {
		case <-done:
			if m.logger != nil {
				m.logger.Info("✅ 阶段3/6: 所有正在执行的请求已完成")
			}
		case <-waitCtx.Done():
			if m.logger != nil {
				m.logger.Warnf("⚠️ 阶段3/6: 等待请求完成超时（%v），强制关闭引擎", waitCtx.Err())
			}
		}

		// 阶段4: 关闭WASM引擎
		if m.wasmEngine != nil {
			if m.logger != nil {
				m.logger.Info("🔄 阶段4/6: 正在关闭WASM引擎...")
			}
			if err := m.wasmEngine.Close(); err != nil {
				if shutdownErr == nil {
					shutdownErr = fmt.Errorf("关闭WASM引擎失败: %w", err)
				} else {
					shutdownErr = fmt.Errorf("%w; 关闭WASM引擎失败: %w", shutdownErr, err)
				}
				if m.logger != nil {
					m.logger.Errorf("❌ 关闭WASM引擎失败: %v", err)
				}
			} else {
				if m.logger != nil {
					m.logger.Info("✅ WASM引擎已关闭")
				}
			}
		}

		// 阶段5: 关闭ONNX引擎
		if m.onnxEngine != nil {
			if m.logger != nil {
				m.logger.Info("🔄 阶段5/6: 正在关闭ONNX引擎...")
			}
			if err := m.onnxEngine.Shutdown(); err != nil {
				if shutdownErr == nil {
					shutdownErr = fmt.Errorf("关闭ONNX引擎失败: %w", err)
				} else {
					shutdownErr = fmt.Errorf("%w; 关闭ONNX引擎失败: %w", shutdownErr, err)
				}
				if m.logger != nil {
					m.logger.Errorf("❌ 关闭ONNX引擎失败: %v", err)
				}
			} else {
				if m.logger != nil {
					m.logger.Info("✅ ONNX引擎已关闭")
				}
			}
		}

		// 阶段6: 清理资源
		if m.shutdownCancel != nil {
			m.shutdownCancel = nil
		}
		m.shutdownCtx = nil

		// P1: 停止执行结果缓存清理goroutine
		if m.executionCache != nil {
			m.executionCache.Stop()
			if m.logger != nil {
				m.logger.Info("✅ 执行结果缓存已停止")
			}
		}

		if shutdownErr == nil {
			if m.logger != nil {
				m.logger.Info("✅ 阶段6/6: 引擎管理器已成功关闭，所有资源已释放")
			}
		} else {
			if m.logger != nil {
				m.logger.Errorf("⚠️ 阶段6/6: 引擎管理器关闭完成，但有错误: %v", shutdownErr)
			}
		}
	})

	return shutdownErr
}

// 确保Manager实现InternalEngineManager接口
var _ ispcInterfaces.InternalEngineManager = (*Manager)(nil)

func (m *Manager) restoreCachedWASMResult(ctx context.Context, entry *wasmCacheValue) {
	if entry == nil {
		return
	}

	execCtx := hostabi.GetExecutionContext(ctx)
	if execCtx == nil {
		if m.logger != nil {
			m.logger.Warn("⚠️ WASM缓存命中但 ExecutionContext 不可用，返回数据无法恢复")
		}
		return
	}

	if len(entry.ReturnData) > 0 {
		if err := execCtx.SetReturnData(cloneBytes(entry.ReturnData)); err != nil && m.logger != nil {
			m.logger.Warnf("⚠️ 恢复缓存返回数据失败: %v", err)
		}
	}

	if len(entry.Events) > 0 {
		for _, evt := range entry.Events {
			if evt == nil {
				continue
			}
			if err := execCtx.AddEvent(cloneEvent(evt)); err != nil && m.logger != nil {
				m.logger.Warnf("⚠️ 恢复缓存事件失败: %v", err)
			}
		}
	}
}

func cloneUint64Slice(src []uint64) []uint64 {
	if len(src) == 0 {
		return nil
	}
	dst := make([]uint64, len(src))
	copy(dst, src)
	return dst
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func cloneEvents(src []*ispcInterfaces.Event) []*ispcInterfaces.Event {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]*ispcInterfaces.Event, 0, len(src))
	for _, evt := range src {
		if evt == nil {
			continue
		}
		cloned = append(cloned, cloneEvent(evt))
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func cloneEvent(evt *ispcInterfaces.Event) *ispcInterfaces.Event {
	if evt == nil {
		return nil
	}
	cloned := &ispcInterfaces.Event{
		Type:      evt.Type,
		Timestamp: evt.Timestamp,
	}
	if evt.Data != nil {
		cloned.Data = cloneEventData(evt.Data)
	}
	return cloned
}

func cloneEventData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(data))
	for k, v := range data {
		cloned[k] = cloneEventValue(v)
	}
	return cloned
}

func cloneEventValue(v interface{}) interface{} {
	switch value := v.(type) {
	case nil:
		return nil
	case string, bool, int, int32, int64, uint, uint32, uint64, float32, float64:
		return value
	case []byte:
		return cloneBytes(value)
	case map[string]interface{}:
		return cloneEventData(value)
	case []interface{}:
		if len(value) == 0 {
			return []interface{}{}
		}
		cloned := make([]interface{}, len(value))
		for i, item := range value {
			cloned[i] = cloneEventValue(item)
		}
		return cloned
	default:
		return value
	}
}
