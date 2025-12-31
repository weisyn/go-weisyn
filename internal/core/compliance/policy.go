// policy.go
// 合规策略决策引擎实现
//
// 主要功能：
// 1. 多信源融合的合规决策
// 2. 地理限制和操作限制检查
// 3. 决策结果缓存和性能优化
// 4. 配置热更新支持
//
// 决策逻辑：
// 1. 优先级：身份凭证 > GeoIP查询 > P2P地理特征
// 2. 国家判定：多信源融合，取最可信的结果
// 3. 操作检查：支持精确匹配和模式匹配
// 4. 缓存策略：决策结果短期缓存，减少重复计算
//
// 作者：WES开发团队
// 创建时间：2025-09-15

package compliance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/internal/config/compliance"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// CompliancePolicyService 合规策略服务实现
//
// 🛡️ **纯地理位置合规服务 (Pure Geolocation Compliance Service)**
//
// 基于IP地理位置的简化合规检查服务，使用DB-IP免费数据库。
// 不依赖外部身份验证服务，完全开源自包含。
type CompliancePolicyService struct {
	logger       log.Logger                    // 日志记录器
	config       *compliance.ComplianceOptions // 合规配置
	geoipService complianceIfaces.GeoIPService // 地理位置查询服务（唯一依赖）

	// 配置热更新支持
	configMutex   sync.RWMutex // 配置读写锁
	configVersion int64        // 配置版本号（原子操作）

	// 决策结果缓存
	decisionCache    map[string]*cachedDecision // 决策结果缓存
	cacheMutex       sync.RWMutex               // 缓存读写锁
	cacheCleanupTick *time.Ticker               // 缓存清理定时器

	// 性能统计
	stats *policyStats // 策略执行统计
}

// cachedDecision 缓存的决策结果
type cachedDecision struct {
	decision  *complianceIfaces.Decision // 决策结果
	expiresAt time.Time                  // 过期时间
}

// policyStats 策略执行统计信息
type policyStats struct {
	totalChecks   int64 // 总检查次数
	allowedChecks int64 // 允许通过次数
	deniedChecks  int64 // 拒绝次数
	cacheHits     int64 // 缓存命中次数
	cacheMisses   int64 // 缓存未命中次数
}

// NewCompliancePolicyService 创建合规策略服务
//
// 🏗️ **纯地理位置合规服务构造器 (Pure Geolocation Compliance Constructor)**
//
// 创建基于DB-IP地理位置的简化合规服务，完全开源自包含。
//
// 参数：
// - config: 合规配置选项
// - logger: 日志记录器
// - geoipService: DB-IP地理位置查询服务
//
// 返回：
// - complianceIfaces.Policy: 合规策略接口实现
func NewCompliancePolicyService(
	config *compliance.ComplianceOptions,
	logger log.Logger,
	geoipService complianceIfaces.GeoIPService,
) complianceIfaces.Policy {
	service := &CompliancePolicyService{
		logger:       logger,
		config:       config,
		geoipService: geoipService,
		configVersion:    1,
		decisionCache:    make(map[string]*cachedDecision),
		stats:            &policyStats{},
	}

	// 启动缓存清理协程
	service.startCacheCleanup()

	return service
}

// CheckTransaction 检查交易的合规性
func (s *CompliancePolicyService) CheckTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	source *complianceIfaces.TransactionSource,
) (*complianceIfaces.Decision, error) {
	// 增加统计计数
	atomic.AddInt64(&s.stats.totalChecks, 1)

	// 如果合规功能未启用，直接允许
	s.configMutex.RLock()
	if !s.config.Enabled {
		s.configMutex.RUnlock()
		decision := &complianceIfaces.Decision{
			Allowed:   true,
			Source:    complianceIfaces.DecisionSourceConfig,
			Timestamp: time.Now(),
		}
		atomic.AddInt64(&s.stats.allowedChecks, 1)
		return decision, nil
	}
	s.configMutex.RUnlock()

	// 检查缓存
	cacheKey := s.buildTransactionCacheKey(tx, source)
	if cached := s.getCachedDecision(cacheKey); cached != nil {
		atomic.AddInt64(&s.stats.cacheHits, 1)
		return cached, nil
	}
	atomic.AddInt64(&s.stats.cacheMisses, 1)

	// 解析交易操作类型
	operation := s.extractOperationType(tx)

	// 提取发起地址
	address := s.extractSenderAddress(tx)

	// 执行合规检查
	decision, err := s.performComplianceCheck(ctx, operation, address, source)
	if err != nil {
		s.logger.Errorf("合规检查执行失败: %v", err)
		return nil, err
	}

	// 缓存决策结果
	s.cacheDecision(cacheKey, decision, 5*time.Minute)

	// 更新统计
	if decision.Allowed {
		atomic.AddInt64(&s.stats.allowedChecks, 1)
	} else {
		atomic.AddInt64(&s.stats.deniedChecks, 1)
	}

	return decision, nil
}

// CheckOperation 检查特定操作的合规性
func (s *CompliancePolicyService) CheckOperation(
	ctx context.Context,
	operation string,
	address string,
	source *complianceIfaces.TransactionSource,
) (*complianceIfaces.Decision, error) {
	// 增加统计计数
	atomic.AddInt64(&s.stats.totalChecks, 1)

	// 如果合规功能未启用，直接允许
	s.configMutex.RLock()
	if !s.config.Enabled {
		s.configMutex.RUnlock()
		decision := &complianceIfaces.Decision{
			Allowed:   true,
			Source:    complianceIfaces.DecisionSourceConfig,
			Timestamp: time.Now(),
		}
		atomic.AddInt64(&s.stats.allowedChecks, 1)
		return decision, nil
	}
	s.configMutex.RUnlock()

	// 检查缓存
	cacheKey := s.buildOperationCacheKey(operation, address, source)
	if cached := s.getCachedDecision(cacheKey); cached != nil {
		atomic.AddInt64(&s.stats.cacheHits, 1)
		return cached, nil
	}
	atomic.AddInt64(&s.stats.cacheMisses, 1)

	// 执行合规检查
	decision, err := s.performComplianceCheck(ctx, operation, address, source)
	if err != nil {
		s.logger.Errorf("操作合规检查执行失败: %v", err)
		return nil, err
	}

	// 缓存决策结果
	s.cacheDecision(cacheKey, decision, 5*time.Minute)

	// 更新统计
	if decision.Allowed {
		atomic.AddInt64(&s.stats.allowedChecks, 1)
	} else {
		atomic.AddInt64(&s.stats.deniedChecks, 1)
	}

	return decision, nil
}

// UpdatePolicy 更新合规策略配置
func (s *CompliancePolicyService) UpdatePolicy(ctx context.Context, config interface{}) error {
	newConfig, ok := config.(*compliance.ComplianceOptions)
	if !ok {
		return fmt.Errorf("配置类型错误，期望 *compliance.ComplianceOptions")
	}

	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 更新配置
	s.config = newConfig
	atomic.AddInt64(&s.configVersion, 1)

	// 清空决策缓存，确保使用新配置
	s.clearDecisionCache()

	s.logger.Infof("合规策略配置已更新，版本: %d", atomic.LoadInt64(&s.configVersion))
	return nil
}

// ============================================================================
//                           核心决策逻辑
// ============================================================================

// performComplianceCheck 执行核心合规检查逻辑
func (s *CompliancePolicyService) performComplianceCheck(
	ctx context.Context,
	operation string,
	address string,
	source *complianceIfaces.TransactionSource,
) (*complianceIfaces.Decision, error) {
	// 1. 检查操作是否被禁用
	if s.isOperationBanned(operation) {
		return &complianceIfaces.Decision{
			Allowed:      false,
			Reason:       complianceIfaces.ReasonOperationBanned,
			ReasonDetail: fmt.Sprintf("操作类型 '%s' 被配置禁用", operation),
			Source:       complianceIfaces.DecisionSourceConfig,
			Timestamp:    time.Now(),
		}, nil
	}

	// 2. 获取国家信息（多信源融合）
	country, decisionSource, err := s.determineCountry(ctx, address, source)
	if err != nil {
		s.logger.Warnf("确定国家信息失败: %v", err)
		// 继续执行，使用未知国家处理逻辑
	}

	// 3. 检查国家是否被禁用
	if s.isCountryBanned(country) {
		return &complianceIfaces.Decision{
			Allowed:      false,
			Reason:       complianceIfaces.ReasonCountryBanned,
			ReasonDetail: fmt.Sprintf("国家 '%s' 被配置禁用", country),
			Country:      country,
			Source:       decisionSource,
			Timestamp:    time.Now(),
		}, nil
	}

	// 4. 处理未知国家情况
	if country == "" && s.config.RejectOnUnknownCountry {
		return &complianceIfaces.Decision{
			Allowed:      false,
			Reason:       complianceIfaces.ReasonUnknownCountry,
			ReasonDetail: "无法确定来源国家且配置拒绝未知来源",
			Source:       complianceIfaces.DecisionSourceUnknown,
			Timestamp:    time.Now(),
		}, nil
	}

	// 5. 所有检查通过，允许操作
	return &complianceIfaces.Decision{
		Allowed:   true,
		Country:   country,
		Source:    decisionSource,
		Timestamp: time.Now(),
	}, nil
}

// determineCountry 基于IP地理位置确定国家信息
//
// 🌍 **纯地理位置国家判定 (Pure Geolocation Country Detection)**
//
// 简化的国家信息确定逻辑，只依赖IP地理位置查询。
// 使用DB-IP免费数据库，完全开源自包含。
//
// 查询优先级：
// 1. IP地址GeoIP查询（主要方式）
// 2. 已知地理位置信息（备用方式）
func (s *CompliancePolicyService) determineCountry(
	ctx context.Context,
	address string,
	source *complianceIfaces.TransactionSource,
) (string, complianceIfaces.DecisionSource, error) {
	// 优先级1：GeoIP查询（主要方式）
	if source != nil && source.IPAddress != "" {
		if country, err := s.geoipService.GetCountryByIP(ctx, source.IPAddress); err == nil && country != "" {
			if s.logger != nil {
				s.logger.Debugf("通过GeoIP确定地址 %s 来自国家: %s", address, country)
			}
			return country, complianceIfaces.DecisionSourceGeoIP, nil
		}
	}

	// 优先级2：已知地理位置信息（备用方式）
	if source != nil && source.GeoLocation != nil && source.GeoLocation.Country != "" {
		return source.GeoLocation.Country, complianceIfaces.DecisionSourceP2P, nil
	}

	// 无法确定国家
	return "", complianceIfaces.DecisionSourceUnknown, fmt.Errorf("无法从任何信源确定国家信息")
}

// isOperationBanned 检查操作是否被禁用
func (s *CompliancePolicyService) isOperationBanned(operation string) bool {
	s.configMutex.RLock()
	defer s.configMutex.RUnlock()

	for _, bannedOp := range s.config.BannedOperations {
		// 精确匹配
		if bannedOp == operation {
			return true
		}

		// 模式匹配（支持通配符）
		if strings.HasSuffix(bannedOp, "*") {
			prefix := strings.TrimSuffix(bannedOp, "*")
			if strings.HasPrefix(operation, prefix) {
				return true
			}
		}
	}

	return false
}

// isCountryBanned 检查国家是否被禁用
func (s *CompliancePolicyService) isCountryBanned(country string) bool {
	if country == "" {
		return false
	}

	s.configMutex.RLock()
	defer s.configMutex.RUnlock()

	for _, bannedCountry := range s.config.BannedCountries {
		if bannedCountry == country {
			return true
		}
	}

	return false
}

// ============================================================================
//                           辅助工具方法
// ============================================================================

// extractOperationType 从交易中提取操作类型
func (s *CompliancePolicyService) extractOperationType(tx *transaction.Transaction) string {
	if tx == nil || len(tx.Outputs) == 0 {
		return "unknown"
	}

	// 根据输出类型判断操作类型
	for _, output := range tx.Outputs {
		switch output.OutputContent.(type) {
		case *transaction.TxOutput_Asset:
			return "transfer" // 资产转账
		case *transaction.TxOutput_Resource:
			// 资源相关操作，可能是合约部署或调用
			if tx.Metadata != nil {
				// 简化处理：基于Metadata判断合约方法类型
				// 实际实现中可能需要更复杂的解析逻辑
				return "contract.call"
			}
			return "contract.*"
		case *transaction.TxOutput_State:
			return "state.update" // 状态更新操作
		}
	}

	return "unknown"
}

// extractSenderAddress 从交易中提取发送方地址
func (s *CompliancePolicyService) extractSenderAddress(tx *transaction.Transaction) string {
	if tx == nil || len(tx.Outputs) == 0 {
		return ""
	}

	// 从第一个输出的owner字段获取地址
	// 注意：在UTXO模型中，发送方通常是输出的owner
	return string(tx.Outputs[0].Owner)
}

// ============================================================================
//                           缓存管理
// ============================================================================

// buildTransactionCacheKey 构建交易的缓存键
func (s *CompliancePolicyService) buildTransactionCacheKey(
	tx *transaction.Transaction,
	source *complianceIfaces.TransactionSource,
) string {
	operation := s.extractOperationType(tx)
	address := s.extractSenderAddress(tx)
	ipAddress := ""
	if source != nil {
		ipAddress = source.IPAddress
	}

	return fmt.Sprintf("tx:%s:%s:%s:%d", operation, address, ipAddress, atomic.LoadInt64(&s.configVersion))
}

// buildOperationCacheKey 构建操作的缓存键
func (s *CompliancePolicyService) buildOperationCacheKey(
	operation string,
	address string,
	source *complianceIfaces.TransactionSource,
) string {
	ipAddress := ""
	if source != nil {
		ipAddress = source.IPAddress
	}

	return fmt.Sprintf("op:%s:%s:%s:%d", operation, address, ipAddress, atomic.LoadInt64(&s.configVersion))
}

// getCachedDecision 获取缓存的决策结果
func (s *CompliancePolicyService) getCachedDecision(cacheKey string) *complianceIfaces.Decision {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	if cached, exists := s.decisionCache[cacheKey]; exists {
		if cached.expiresAt.After(time.Now()) {
			return cached.decision
		}
		// 过期的缓存将在清理时移除
	}

	return nil
}

// cacheDecision 缓存决策结果
func (s *CompliancePolicyService) cacheDecision(
	cacheKey string,
	decision *complianceIfaces.Decision,
	ttl time.Duration,
) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.decisionCache[cacheKey] = &cachedDecision{
		decision:  decision,
		expiresAt: time.Now().Add(ttl),
	}
}

// clearDecisionCache 清空决策缓存
func (s *CompliancePolicyService) clearDecisionCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.decisionCache = make(map[string]*cachedDecision)
}

// startCacheCleanup 启动缓存清理协程
func (s *CompliancePolicyService) startCacheCleanup() {
	s.cacheCleanupTick = time.NewTicker(10 * time.Minute)
	go func() {
		for range s.cacheCleanupTick.C {
			s.cleanupExpiredCache()
		}
	}()
}

// cleanupExpiredCache 清理过期的缓存条目
func (s *CompliancePolicyService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for key, cached := range s.decisionCache {
		if cached.expiresAt.Before(now) {
			delete(s.decisionCache, key)
		}
	}
}

// Stop 停止服务并清理资源
func (s *CompliancePolicyService) Stop() {
	if s.cacheCleanupTick != nil {
		s.cacheCleanupTick.Stop()
	}
}
