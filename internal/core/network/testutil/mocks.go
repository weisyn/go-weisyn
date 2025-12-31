// Package testutil 提供 network 模块测试的统一 Mock 对象和辅助函数
//
// 🎯 **设计原则**：
// - 统一管理：所有 Mock 对象集中在此，避免重复定义
// - 最小实现：Mock 对象只实现必要的方法，返回合理的默认值
// - 可配置：支持设置特定返回值（如需要）
package testutil

import (
	"hash"
	"time"

	"go.uber.org/zap"

	apiconfig "github.com/weisyn/v1/internal/config/api"
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	candidatepoolconfig "github.com/weisyn/v1/internal/config/candidatepool"
	clockconfig "github.com/weisyn/v1/internal/config/clock"
	complianceconfig "github.com/weisyn/v1/internal/config/compliance"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	eventconfig "github.com/weisyn/v1/internal/config/event"
	logconfig "github.com/weisyn/v1/internal/config/log"
	networkconfig "github.com/weisyn/v1/internal/config/network"
	nodeconfig "github.com/weisyn/v1/internal/config/node"
	repositoryconfig "github.com/weisyn/v1/internal/config/repository"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	sqliteconfig "github.com/weisyn/v1/internal/config/storage/sqlite"
	syncconfig "github.com/weisyn/v1/internal/config/sync"
	signerconfig "github.com/weisyn/v1/internal/config/tx/signer"
	temporaryconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	txpoolconfig "github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== MockLogger ====================

// MockLogger 统一的日志 Mock 实现
type MockLogger struct{}

func (m *MockLogger) Debug(msg string)                          {}
func (m *MockLogger) Debugf(format string, args ...interface{}) {}
func (m *MockLogger) Info(msg string)                           {}
func (m *MockLogger) Infof(format string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string)                           {}
func (m *MockLogger) Warnf(format string, args ...interface{})  {}
func (m *MockLogger) Error(msg string)                          {}
func (m *MockLogger) Errorf(format string, args ...interface{}) {}
func (m *MockLogger) Fatal(msg string)                          {}
func (m *MockLogger) Fatalf(format string, args ...interface{}) {}
func (m *MockLogger) With(keyvals ...interface{}) logiface.Logger { return m }
func (m *MockLogger) Sync() error                               { return nil }
func (m *MockLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// ==================== MockConfigProvider ====================

// MockConfigProvider 统一的配置 Mock 实现
type MockConfigProvider struct {
	networkOptions *networkconfig.NetworkOptions
}

// NewMockConfigProvider 创建 Mock 配置提供者
func NewMockConfigProvider() *MockConfigProvider {
	return &MockConfigProvider{
		networkOptions: &networkconfig.NetworkOptions{
			MaxMessageSize:           1024 * 1024, // 1MB
			MessageTimeout:           30 * time.Second,
			DeduplicationCacheTTL:    5 * time.Minute,
			RetryAttempts:            3,
			RetryBackoffBase:         100 * time.Millisecond,
			RetryBackoffMax:          5 * time.Second,
			ConnectTimeout:           10 * time.Second,
			WriteTimeout:             5 * time.Second,
			ReadTimeout:              5 * time.Second,
			MaxConnections:           1000,
			MaxConnectionsPerIP:       50,
			MaxMessagesPerWindow:      100,
			MessageRateLimitWindow:    1 * time.Minute,
		},
	}
}

// SetNetworkOptions 设置网络配置选项
func (m *MockConfigProvider) SetNetworkOptions(opts *networkconfig.NetworkOptions) {
	m.networkOptions = opts
}


// GetAPI 获取API配置
func (m *MockConfigProvider) GetAPI() *apiconfig.APIOptions { return nil }

// GetBlockchain 获取区块链配置
func (m *MockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions { return nil }

// GetConsensus 获取共识配置
func (m *MockConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions { return nil }

// GetTxPool 获取交易池配置
func (m *MockConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions { return nil }

// GetCandidatePool 获取候选池配置
func (m *MockConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions { return nil }

// GetNetwork 获取网络配置
func (m *MockConfigProvider) GetNetwork() *networkconfig.NetworkOptions {
	return m.networkOptions
}

// GetSync 获取同步配置
func (m *MockConfigProvider) GetSync() *syncconfig.SyncOptions { return nil }

// GetLog 获取日志配置
func (m *MockConfigProvider) GetLog() *logconfig.LogOptions { return nil }

// GetEvent 获取事件配置
func (m *MockConfigProvider) GetEvent() *eventconfig.EventOptions { return nil }

// GetRepository 获取资源仓库配置
func (m *MockConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions { return nil }

// GetCompliance 获取合规配置
func (m *MockConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions { return nil }

// GetClock 获取时钟配置
func (m *MockConfigProvider) GetClock() *clockconfig.ClockOptions { return nil }

// GetNetworkNamespace 获取网络命名空间
func (m *MockConfigProvider) GetInstanceDataDir() string { return "./data/test/test-mock" }

func (m *MockConfigProvider) GetNetworkNamespace() string { return "testnet" }

// GetBadger 获取BadgerDB配置
func (m *MockConfigProvider) GetBadger() *badgerconfig.BadgerOptions { return nil }

// GetMemory 获取内存存储配置
func (m *MockConfigProvider) GetMemory() *memoryconfig.MemoryOptions { return nil }

// GetFile 获取文件存储配置
func (m *MockConfigProvider) GetFile() *fileconfig.FileOptions { return nil }

// GetSQLite 获取SQLite配置
func (m *MockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions { return nil }

// GetTemporary 获取临时存储配置
func (m *MockConfigProvider) GetTemporary() *temporaryconfig.TempOptions { return nil }

// GetSigner 获取签名器配置
func (m *MockConfigProvider) GetSigner() *signerconfig.SignerOptions { return nil }

// GetDraftStore 获取草稿存储配置
func (m *MockConfigProvider) GetDraftStore() interface{} { return nil }

// GetAppConfig 获取原始应用配置
func (m *MockConfigProvider) GetAppConfig() *types.AppConfig { return nil }

// GetUnifiedGenesisConfig 获取统一格式的创世配置
func (m *MockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig { return nil }

// GetMemoryMonitoring 获取内存监控配置
func (m *MockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig { return nil }

// GetNode 获取节点配置
func (m *MockConfigProvider) GetNode() *nodeconfig.NodeOptions { return nil }

// GetEnvironment 获取运行环境
func (m *MockConfigProvider) GetEnvironment() string { return "test" }

// GetChainMode 获取链模式
func (m *MockConfigProvider) GetChainMode() string { return "private" }

// GetSecurity 获取安全配置
func (m *MockConfigProvider) GetSecurity() *types.UserSecurityConfig { return nil }

// GetAccessControlMode 获取接入控制模式
func (m *MockConfigProvider) GetAccessControlMode() string { return "open" }

// GetCertificateManagement 获取证书管理配置
func (m *MockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig { return nil }

// GetPSK 获取PSK配置
func (m *MockConfigProvider) GetPSK() *types.UserPSKConfig { return nil }

// GetPermissionModel 获取权限模型
func (m *MockConfigProvider) GetPermissionModel() string { return "private" }


// ==================== MockHashManager ====================

// MockHashManager 统一的哈希管理器 Mock 实现
type MockHashManager struct{}

// SHA256 计算SHA-256哈希
func (m *MockHashManager) SHA256(data []byte) []byte {
	// 返回固定哈希值用于测试
	return make([]byte, 32)
}

// Keccak256 计算Keccak-256哈希
func (m *MockHashManager) Keccak256(data []byte) []byte {
	return make([]byte, 32)
}

// RIPEMD160 计算RIPEMD-160哈希
func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	return make([]byte, 20)
}

// DoubleSHA256 计算双重SHA-256哈希
func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	return make([]byte, 32)
}

// NewSHA256Hasher 创建SHA-256流式哈希器
func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return nil // 返回 nil，测试中需要时再实现
}

// NewRIPEMD160Hasher 创建RIPEMD-160流式哈希器
func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	return nil // 返回 nil，测试中需要时再实现
}

// ==================== MockSigManager ====================

// MockSigManager 统一的签名管理器 Mock 实现
type MockSigManager struct{}

// SignTransaction 签名交易
func (m *MockSigManager) SignTransaction(txHash []byte, privateKey []byte, sigHashType crypto.SignatureHashType) ([]byte, error) {
	return make([]byte, 64), nil
}

// VerifyTransactionSignature 验证交易签名
func (m *MockSigManager) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType crypto.SignatureHashType) bool {
	return true
}

// Sign 签名任意数据
func (m *MockSigManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	return make([]byte, 64), nil
}

// Verify 验证数据签名
func (m *MockSigManager) Verify(data, signature, publicKey []byte) bool {
	return true
}

// SignMessage 签名消息
func (m *MockSigManager) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	return make([]byte, 65), nil
}

// VerifyMessage 验证消息签名
func (m *MockSigManager) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	return true
}

// RecoverPublicKey 从签名恢复公钥
func (m *MockSigManager) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	return make([]byte, 33), nil
}

// RecoverAddress 从签名恢复地址
func (m *MockSigManager) RecoverAddress(hash []byte, signature []byte) (string, error) {
	return "test_address", nil
}

// SignBatch 批量签名
func (m *MockSigManager) SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error) {
	return make([][]byte, len(dataList)), nil
}

// VerifyBatch 批量验证签名
func (m *MockSigManager) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	result := make([]bool, len(dataList))
	for i := range result {
		result[i] = true
	}
	return result, nil
}

// NormalizeSignature 规范化签名
func (m *MockSigManager) NormalizeSignature(signature []byte) ([]byte, error) {
	return signature, nil
}

// ValidateSignature 验证签名格式
func (m *MockSigManager) ValidateSignature(signature []byte) error {
	return nil
}

// ==================== MockEventBus ====================

// MockEventBus 统一的事件总线 Mock 实现
type MockEventBus struct {
	subscriptions map[event.EventType][]interface{}
}

// NewMockEventBus 创建 Mock 事件总线
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		subscriptions: make(map[event.EventType][]interface{}),
	}
}

// Subscribe 订阅事件
func (m *MockEventBus) Subscribe(eventType event.EventType, handler interface{}) error {
	if m.subscriptions == nil {
		m.subscriptions = make(map[event.EventType][]interface{})
	}
	m.subscriptions[eventType] = append(m.subscriptions[eventType], handler)
	return nil
}

// SubscribeAsync 异步订阅事件
func (m *MockEventBus) SubscribeAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return m.Subscribe(eventType, handler)
}

// SubscribeOnce 一次性订阅事件
func (m *MockEventBus) SubscribeOnce(eventType event.EventType, handler interface{}) error {
	return m.Subscribe(eventType, handler)
}

// SubscribeOnceAsync 异步一次性订阅事件
func (m *MockEventBus) SubscribeOnceAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return m.Subscribe(eventType, handler)
}

// Publish 发布事件
func (m *MockEventBus) Publish(eventType event.EventType, args ...interface{}) {
	// Mock 实现
}

// PublishEvent 发布Event接口类型事件
func (m *MockEventBus) PublishEvent(ev event.Event) {
	// Mock 实现
}

// Unsubscribe 取消订阅
func (m *MockEventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// WaitAsync 等待所有异步处理完成
func (m *MockEventBus) WaitAsync() {
	// Mock 实现
}

// HasCallback 检查是否有回调函数
func (m *MockEventBus) HasCallback(eventType event.EventType) bool {
	return len(m.subscriptions[eventType]) > 0
}

// GetEventHistory 获取事件历史
func (m *MockEventBus) GetEventHistory(eventType event.EventType) []interface{} {
	return nil
}

// PublishWESEvent 发布WES事件
func (m *MockEventBus) PublishWESEvent(event *types.WESEvent) error {
	return nil
}

// SubscribeWithFilter 带过滤器的订阅
func (m *MockEventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	return "", nil
}

// SubscribeWESEvents 订阅WES消息事件
func (m *MockEventBus) SubscribeWESEvents(protocols []types.ProtocolType, handler types.WESEventHandler) (types.SubscriptionID, error) {
	return "", nil
}

// UnsubscribeByID 通过订阅ID取消订阅
func (m *MockEventBus) UnsubscribeByID(id types.SubscriptionID) error {
	return nil
}

// EnableEventHistory 启用事件历史记录
func (m *MockEventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	return nil
}

// DisableEventHistory 禁用事件历史记录
func (m *MockEventBus) DisableEventHistory(eventType event.EventType) error {
	return nil
}

// GetActiveSubscriptions 获取活跃订阅列表
func (m *MockEventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	return nil, nil
}

// UpdateConfig 更新事件总线配置
func (m *MockEventBus) UpdateConfig(config *types.EventBusConfig) error {
	return nil
}

// GetConfig 获取当前配置
func (m *MockEventBus) GetConfig() (*types.EventBusConfig, error) {
	return nil, nil
}

// RegisterEventInterceptor 注册事件拦截器
func (m *MockEventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	return nil
}

// UnregisterEventInterceptor 注销事件拦截器
func (m *MockEventBus) UnregisterEventInterceptor(interceptorID string) error {
	return nil
}

