package zkproof

import (
	"context"
	"fmt"
	"sync"
	"time"

	// 内部接口
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ProofGenerationRetryConfig 证明生成重试配置
type ProofGenerationRetryConfig struct {
	MaxRetries      int           // 最大重试次数
	InitialDelay    time.Duration // 初始延迟
	MaxDelay        time.Duration // 最大延迟
	BackoffFactor   float64       // 退避因子（指数退避）
	RetryableErrors []string      // 可重试的错误类型
}

// DefaultProofGenerationRetryConfig 默认重试配置
func DefaultProofGenerationRetryConfig() *ProofGenerationRetryConfig {
	return &ProofGenerationRetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"timeout",
			"temporary",
			"circuit compilation",
			"witness building",
		},
	}
}

// ProofGenerationErrorLog 证明生成错误日志
type ProofGenerationErrorLog struct {
	Timestamp      time.Time
	CircuitID      string
	CircuitVersion uint32
	Error          error
	Attempt        int
	Retryable      bool
	Context        map[string]interface{}
}

// ProofReliabilityEnforcer 证明生成可靠性增强器
//
// 🎯 **可靠性保证**：
// - 证明生成重试机制：自动重试可恢复的错误
// - 证明验证自检：生成后立即验证
// - 错误日志记录：详细记录所有错误用于故障排查
type ProofReliabilityEnforcer struct {
	logger         log.Logger
	prover         *Prover
	validator      *Validator
	retryConfig    *ProofGenerationRetryConfig
	errorLogs      []ProofGenerationErrorLog
	errorLogsMutex sync.RWMutex
	maxErrorLogs   int // 最大错误日志数量
}

// NewProofReliabilityEnforcer 创建证明生成可靠性增强器
func NewProofReliabilityEnforcer(
	logger log.Logger,
	prover *Prover,
	validator *Validator,
	retryConfig *ProofGenerationRetryConfig,
) *ProofReliabilityEnforcer {
	if retryConfig == nil {
		retryConfig = DefaultProofGenerationRetryConfig()
	}

	return &ProofReliabilityEnforcer{
		logger:       logger,
		prover:       prover,
		validator:    validator,
		retryConfig:  retryConfig,
		errorLogs:    make([]ProofGenerationErrorLog, 0),
		maxErrorLogs: 1000, // 最多保存1000条错误日志
	}
}

// GenerateProofWithRetry 带重试机制的证明生成
//
// 🎯 **重试机制**：
// - 自动重试可恢复的错误（如超时、临时错误）
// - 使用指数退避策略
// - 记录每次重试的错误日志
//
// 📋 **参数**：
//   - ctx: 上下文（支持超时控制）
//   - input: ZK证明输入
//
// 🔧 **返回值**：
//   - *interfaces.ZKProofResult: 证明结果
//   - error: 生成过程中的错误
func (e *ProofReliabilityEnforcer) GenerateProofWithRetry(
	ctx context.Context,
	input *interfaces.ZKProofInput,
) (*interfaces.ZKProofResult, error) {
	var lastErr error
	delay := e.retryConfig.InitialDelay

	for attempt := 0; attempt <= e.retryConfig.MaxRetries; attempt++ {
		// 记录尝试次数
		if attempt > 0 {
			e.logger.Warnf("ZK证明生成重试: circuitID=%s, attempt=%d/%d, delay=%v",
				input.CircuitID, attempt, e.retryConfig.MaxRetries, delay)
			
			// 等待退避时间
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			case <-time.After(delay):
				// 继续重试
			}
		}

		// 尝试生成证明
		result, err := e.prover.GenerateProof(ctx, input)
		if err == nil {
			// 生成成功，进行验证自检
			if err := e.verifyProofSelfCheck(ctx, input, result); err != nil {
				// 验证失败，记录错误但继续重试
				e.logError(input, err, attempt, false, map[string]interface{}{
					"error_type": "self_check_failed",
					"attempt":     attempt,
				})
				lastErr = fmt.Errorf("证明验证自检失败: %w", err)
				// 继续重试
			} else {
				// 验证成功，返回结果
				e.logger.Infof("ZK证明生成成功: circuitID=%s, attempt=%d, size=%d字节",
					input.CircuitID, attempt+1, len(result.ProofData))
				return result, nil
			}
		} else {
			// 生成失败，检查是否可重试
			retryable := e.isRetryableError(err)
			e.logError(input, err, attempt, retryable, map[string]interface{}{
				"error_type": "generation_failed",
				"attempt":     attempt,
			})

			if !retryable || attempt >= e.retryConfig.MaxRetries {
				// 不可重试或已达到最大重试次数
				return nil, fmt.Errorf("ZK证明生成失败（尝试%d次）: %w", attempt+1, err)
			}

			lastErr = err
		}

		// 计算下一次重试的延迟（指数退避）
		delay = time.Duration(float64(delay) * e.retryConfig.BackoffFactor)
		if delay > e.retryConfig.MaxDelay {
			delay = e.retryConfig.MaxDelay
		}
	}

	return nil, fmt.Errorf("ZK证明生成失败（已重试%d次）: %w", e.retryConfig.MaxRetries, lastErr)
}

// GenerateStateProofWithRetry 带重试机制的状态证明生成
//
// 🎯 **功能**：
// - 调用GenerateProofWithRetry生成基础证明
// - 构建StateProof结构
// - 进行验证自检
func (e *ProofReliabilityEnforcer) GenerateStateProofWithRetry(
	ctx context.Context,
	input *interfaces.ZKProofInput,
) (*transaction.ZKStateProof, error) {
	// 生成基础证明（带重试）
	result, err := e.GenerateProofWithRetry(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("生成基础证明失败: %w", err)
	}

	// 构建StateProof
	stateProof := &transaction.ZKStateProof{
		Proof:               result.ProofData,
		PublicInputs:        input.PublicInputs,
		ProvingScheme:       e.prover.config.DefaultProvingScheme,
		Curve:               e.prover.config.DefaultCurve,
		VerificationKeyHash: result.VKHash,
		CircuitId:           input.CircuitID,
		CircuitVersion:      input.CircuitVersion,
		ConstraintCount:     result.ConstraintCount,
	}

	// 验证自检（使用StateProof）
	if err := e.verifyStateProofSelfCheck(ctx, stateProof); err != nil {
		e.logError(input, err, 0, false, map[string]interface{}{
			"error_type": "state_proof_self_check_failed",
		})
		return nil, fmt.Errorf("状态证明验证自检失败: %w", err)
	}

	e.logger.Infof("状态证明生成成功: circuitID=%s, size=%d字节",
		input.CircuitID, len(stateProof.Proof))
	return stateProof, nil
}

// verifyProofSelfCheck 证明验证自检
//
// 🎯 **验证自检**：
// - 生成证明后立即进行本地验证
// - 确保证明的正确性
// - 如果验证失败，记录错误并返回错误
func (e *ProofReliabilityEnforcer) verifyProofSelfCheck(
	ctx context.Context,
	input *interfaces.ZKProofInput,
	result *interfaces.ZKProofResult,
) error {
	// 构建StateProof用于验证
	stateProof := &transaction.ZKStateProof{
		Proof:               result.ProofData,
		PublicInputs:        input.PublicInputs,
		ProvingScheme:       e.prover.config.DefaultProvingScheme,
		Curve:               e.prover.config.DefaultCurve,
		VerificationKeyHash: result.VKHash,
		CircuitId:           input.CircuitID,
		CircuitVersion:      input.CircuitVersion,
		ConstraintCount:     result.ConstraintCount,
	}

	// 执行验证
	valid, err := e.validator.ValidateProof(ctx, stateProof)
	if err != nil {
		return fmt.Errorf("验证过程出错: %w", err)
	}

	if !valid {
		return fmt.Errorf("证明验证失败: 生成的证明无法通过验证")
	}

	e.logger.Debugf("证明验证自检通过: circuitID=%s", input.CircuitID)
	return nil
}

// verifyStateProofSelfCheck 状态证明验证自检
func (e *ProofReliabilityEnforcer) verifyStateProofSelfCheck(
	ctx context.Context,
	stateProof *transaction.ZKStateProof,
) error {
	valid, err := e.validator.ValidateProof(ctx, stateProof)
	if err != nil {
		return fmt.Errorf("验证过程出错: %w", err)
	}

	if !valid {
		return fmt.Errorf("状态证明验证失败: 生成的证明无法通过验证")
	}

	e.logger.Debugf("状态证明验证自检通过: circuitID=%s", stateProof.CircuitId)
	return nil
}

// isRetryableError 判断错误是否可重试
//
// 🎯 **可重试错误**：
// - 超时错误
// - 临时错误
// - 电路编译错误（可能是资源问题）
// - Witness构建错误（可能是资源问题）
func (e *ProofReliabilityEnforcer) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, retryablePattern := range e.retryConfig.RetryableErrors {
		if contains(errStr, retryablePattern) {
			return true
		}
	}

	// 检查上下文取消错误（不可重试）
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// 默认情况下，某些错误可以重试（如资源不足、临时故障）
	// 但明确的业务逻辑错误不应重试
	return false
}

// contains 检查字符串是否包含子字符串（不区分大小写）
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalsIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalsIgnoreCase 不区分大小写的字符串比较
func equalsIgnoreCase(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1 := s1[i]
		c2 := s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// logError 记录错误日志
//
// 🎯 **错误日志记录**：
// - 记录所有证明生成和验证错误
// - 包含详细的上下文信息
// - 用于故障排查和问题分析
func (e *ProofReliabilityEnforcer) logError(
	input *interfaces.ZKProofInput,
	err error,
	attempt int,
	retryable bool,
	context map[string]interface{},
) {
	errorLog := ProofGenerationErrorLog{
		Timestamp:      time.Now(),
		CircuitID:      input.CircuitID,
		CircuitVersion: input.CircuitVersion,
		Error:          err,
		Attempt:        attempt,
		Retryable:      retryable,
		Context:        context,
	}

	e.errorLogsMutex.Lock()
	defer e.errorLogsMutex.Unlock()

	// 添加错误日志
	e.errorLogs = append(e.errorLogs, errorLog)

	// 限制日志数量（FIFO）
	if len(e.errorLogs) > e.maxErrorLogs {
		e.errorLogs = e.errorLogs[1:]
	}

	// 记录到日志系统
	e.logger.Errorf("ZK证明生成错误: circuitID=%s, version=%d, attempt=%d, retryable=%v, error=%v, context=%v",
		input.CircuitID, input.CircuitVersion, attempt, retryable, err, context)
}

// GetErrorLogs 获取错误日志（用于故障排查）
//
// 🎯 **用途**：
// - 开发阶段的问题诊断
// - 生产环境的故障排查
// - 性能分析和优化
func (e *ProofReliabilityEnforcer) GetErrorLogs(limit int) []ProofGenerationErrorLog {
	e.errorLogsMutex.RLock()
	defer e.errorLogsMutex.RUnlock()

	if limit <= 0 || limit > len(e.errorLogs) {
		limit = len(e.errorLogs)
	}

	// 返回最近的错误日志
	start := len(e.errorLogs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ProofGenerationErrorLog, limit)
	copy(result, e.errorLogs[start:])
	return result
}

// GetErrorStats 获取错误统计信息
func (e *ProofReliabilityEnforcer) GetErrorStats() map[string]interface{} {
	e.errorLogsMutex.RLock()
	defer e.errorLogsMutex.RUnlock()

	totalErrors := len(e.errorLogs)
	retryableErrors := 0
	nonRetryableErrors := 0
	circuitErrorCounts := make(map[string]int)

	for _, log := range e.errorLogs {
		if log.Retryable {
			retryableErrors++
		} else {
			nonRetryableErrors++
		}
		circuitErrorCounts[log.CircuitID]++
	}

	return map[string]interface{}{
		"total_errors":        totalErrors,
		"retryable_errors":    retryableErrors,
		"non_retryable_errors": nonRetryableErrors,
		"circuit_error_counts": circuitErrorCounts,
		"max_error_logs":      e.maxErrorLogs,
	}
}

// ClearErrorLogs 清空错误日志
func (e *ProofReliabilityEnforcer) ClearErrorLogs() {
	e.errorLogsMutex.Lock()
	defer e.errorLogsMutex.Unlock()

	e.errorLogs = make([]ProofGenerationErrorLog, 0)
	e.logger.Infof("已清空所有错误日志")
}

