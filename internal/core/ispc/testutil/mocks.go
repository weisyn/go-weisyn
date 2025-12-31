// Package testutil 提供 ISPC 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"crypto/sha256"
	"hash"
	"sync"
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
	temporaryconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	syncconfig "github.com/weisyn/v1/internal/config/sync"
	signerconfig "github.com/weisyn/v1/internal/config/tx/signer"
	txpoolconfig "github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Mock 对象 ====================

// MockLogger 统一的日志Mock实现
//
// ✅ **设计原则**：最小实现，所有方法返回空值，不记录日志
// 📋 **使用场景**：80%的测试用例，不需要验证日志调用
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
func (m *MockLogger) With(args ...interface{}) log.Logger       { return m }
func (m *MockLogger) Sync() error                               { return nil }
func (m *MockLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// BehavioralMockLogger 行为Mock日志（记录调用）
//
// ✅ **设计原则**：记录所有日志调用，用于验证日志行为
// 📋 **使用场景**：需要验证日志调用的测试（5%的测试用例）
type BehavioralMockLogger struct {
	logs  []string
	mutex sync.Mutex
}

func (m *BehavioralMockLogger) Debug(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *BehavioralMockLogger) Debugf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "DEBUG: "+format)
}

func (m *BehavioralMockLogger) Info(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *BehavioralMockLogger) Infof(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "INFO: "+format)
}

func (m *BehavioralMockLogger) Warn(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *BehavioralMockLogger) Warnf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "WARN: "+format)
}

func (m *BehavioralMockLogger) Error(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *BehavioralMockLogger) Errorf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "ERROR: "+format)
}

func (m *BehavioralMockLogger) Fatal(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "FATAL: "+msg)
}

func (m *BehavioralMockLogger) Fatalf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "FATAL: "+format)
}

func (m *BehavioralMockLogger) With(args ...interface{}) log.Logger { return m }
func (m *BehavioralMockLogger) Sync() error                         { return nil }
func (m *BehavioralMockLogger) GetZapLogger() *zap.Logger           { return zap.NewNop() }

// GetLogs 获取所有日志记录
func (m *BehavioralMockLogger) GetLogs() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return append([]string{}, m.logs...)
}

// ClearLogs 清空日志记录
func (m *BehavioralMockLogger) ClearLogs() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = m.logs[:0]
}

// MockHashManager 统一的哈希管理器Mock实现
//
// ✅ **设计原则**：使用真实的SHA256算法，确保哈希计算正确
// 📋 **使用场景**：所有需要哈希计算的测试
type MockHashManager struct{}

func (m *MockHashManager) SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func (m *MockHashManager) SHA3_256(data []byte) []byte {
	return m.SHA256(data) // 简化实现，使用SHA256
}

func (m *MockHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data) // 简化实现，使用SHA256
}

func (m *MockHashManager) Blake2b_256(data []byte) []byte {
	return m.SHA256(data) // 简化实现，使用SHA256
}

func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	hash := make([]byte, 20)
	copy(hash, m.SHA256(data)[:20])
	return hash
}

func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := m.SHA256(data)
	return m.SHA256(first)
}

func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	return sha256.New() // 简化实现，返回SHA256的hasher
}

// MockSignatureManager 统一的签名管理器Mock实现
//
// ✅ **设计原则**：最小实现，所有验证返回true，签名返回固定值
// 📋 **使用场景**：不需要真实签名验证的测试
type MockSignatureManager struct{}

func (m *MockSignatureManager) SignTransaction(txHash []byte, privateKey []byte, sigHashType crypto.SignatureHashType) ([]byte, error) {
	return []byte("mock_signature"), nil
}

func (m *MockSignatureManager) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType crypto.SignatureHashType) bool {
	return string(signature) == "mock_signature"
}

func (m *MockSignatureManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	return []byte("mock_signature"), nil
}

func (m *MockSignatureManager) Verify(data, signature, publicKey []byte) bool {
	return string(signature) == "mock_signature"
}

func (m *MockSignatureManager) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	return []byte("mock_signature"), nil
}

func (m *MockSignatureManager) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	return string(signature) == "mock_signature"
}

func (m *MockSignatureManager) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	return []byte("mock_public_key"), nil
}

func (m *MockSignatureManager) RecoverAddress(hash []byte, signature []byte) (string, error) {
	return "mock_address", nil
}

func (m *MockSignatureManager) SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error) {
	signatures := make([][]byte, len(dataList))
	for i := range dataList {
		signatures[i] = []byte("mock_signature")
	}
	return signatures, nil
}

func (m *MockSignatureManager) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	results := make([]bool, len(dataList))
	for i := range dataList {
		results[i] = true
	}
	return results, nil
}

func (m *MockSignatureManager) NormalizeSignature(signature []byte) ([]byte, error) {
	return signature, nil
}

func (m *MockSignatureManager) ValidateSignature(signature []byte) error {
	return nil
}

// MockConfigProvider 统一的配置提供者Mock实现
//
// ✅ **设计原则**：实现所有config.Provider接口方法（20+方法），返回nil或默认值
// 📋 **使用场景**：80%的测试用例，不需要特定配置
type MockConfigProvider struct{}

func (m *MockConfigProvider) GetNode() *nodeconfig.NodeOptions {
	return nil
}

func (m *MockConfigProvider) GetAPI() *apiconfig.APIOptions {
	return nil
}

func (m *MockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions {
	return nil
}

func (m *MockConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions {
	return nil
}

func (m *MockConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions {
	return nil
}

func (m *MockConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions {
	return nil
}

func (m *MockConfigProvider) GetNetwork() *networkconfig.NetworkOptions {
	return nil
}

func (m *MockConfigProvider) GetSync() *syncconfig.SyncOptions {
	return nil
}

func (m *MockConfigProvider) GetLog() *logconfig.LogOptions {
	return nil
}

func (m *MockConfigProvider) GetEvent() *eventconfig.EventOptions {
	return nil
}

func (m *MockConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions {
	return nil
}

func (m *MockConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions {
	return nil
}

func (m *MockConfigProvider) GetClock() *clockconfig.ClockOptions {
	return nil
}

func (m *MockConfigProvider) GetEnvironment() string {
	return "test"
}

func (m *MockConfigProvider) GetChainMode() string {
	return "private"
}

func (m *MockConfigProvider) GetInstanceDataDir() string {
	return "./data/test/test-mock"
}

func (m *MockConfigProvider) GetNetworkNamespace() string {
	return "test"
}

func (m *MockConfigProvider) GetSecurity() *types.UserSecurityConfig {
	return nil
}

func (m *MockConfigProvider) GetAccessControlMode() string {
	return "open"
}

func (m *MockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig {
	return nil
}

func (m *MockConfigProvider) GetPSK() *types.UserPSKConfig {
	return nil
}

func (m *MockConfigProvider) GetPermissionModel() string {
	return "private"
}

func (m *MockConfigProvider) GetBadger() *badgerconfig.BadgerOptions {
	return nil
}

func (m *MockConfigProvider) GetMemory() *memoryconfig.MemoryOptions {
	return nil
}

func (m *MockConfigProvider) GetFile() *fileconfig.FileOptions {
	return nil
}

func (m *MockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions {
	return nil
}

func (m *MockConfigProvider) GetTemporary() *temporaryconfig.TempOptions {
	return nil
}

func (m *MockConfigProvider) GetSigner() *signerconfig.SignerOptions {
	return nil
}

func (m *MockConfigProvider) GetAppConfig() *types.AppConfig {
	return nil
}

func (m *MockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig {
	return nil
}

func (m *MockConfigProvider) GetDraftStore() interface{} {
	return nil
}

func (m *MockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig {
	return nil
}

// ConfigurableMockConfigProvider 可配置的Mock配置提供者
//
// ✅ **设计原则**：支持设置特定配置项的返回值
// 📋 **使用场景**：需要特定配置值的测试（15%的测试用例）
type ConfigurableMockConfigProvider struct {
	apiOptions       *apiconfig.APIOptions
	logOptions       *logconfig.LogOptions
	clockOptions     *clockconfig.ClockOptions
	networkNamespace string
}

func (m *ConfigurableMockConfigProvider) GetAPI() *apiconfig.APIOptions {
	if m.apiOptions != nil {
		return m.apiOptions
	}
	return nil
}

func (m *ConfigurableMockConfigProvider) SetAPI(options *apiconfig.APIOptions) {
	m.apiOptions = options
}

func (m *ConfigurableMockConfigProvider) GetLog() *logconfig.LogOptions {
	if m.logOptions != nil {
		return m.logOptions
	}
	return nil
}

func (m *ConfigurableMockConfigProvider) SetLog(options *logconfig.LogOptions) {
	m.logOptions = options
}

func (m *ConfigurableMockConfigProvider) GetClock() *clockconfig.ClockOptions {
	if m.clockOptions != nil {
		return m.clockOptions
	}
	return nil
}

func (m *ConfigurableMockConfigProvider) SetClock(options *clockconfig.ClockOptions) {
	m.clockOptions = options
}

func (m *ConfigurableMockConfigProvider) GetEnvironment() string {
	return "test"
}

func (m *ConfigurableMockConfigProvider) GetChainMode() string {
	return "private"
}

func (m *ConfigurableMockConfigProvider) GetInstanceDataDir() string {
	return "./data/test/test-mock"
}

func (m *ConfigurableMockConfigProvider) GetNetworkNamespace() string {
	if m.networkNamespace != "" {
		return m.networkNamespace
	}
	return "test"
}

func (m *ConfigurableMockConfigProvider) SetNetworkNamespace(namespace string) {
	m.networkNamespace = namespace
}

func (m *ConfigurableMockConfigProvider) GetSecurity() *types.UserSecurityConfig {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetAccessControlMode() string {
	return "open"
}

func (m *ConfigurableMockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetPSK() *types.UserPSKConfig {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetPermissionModel() string {
	return "private"
}

// 实现其他config.Provider方法（委托给基础Mock）
func (m *ConfigurableMockConfigProvider) GetNode() *nodeconfig.NodeOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetNetwork() *networkconfig.NetworkOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetSync() *syncconfig.SyncOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetEvent() *eventconfig.EventOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetBadger() *badgerconfig.BadgerOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetMemory() *memoryconfig.MemoryOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetFile() *fileconfig.FileOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetTemporary() *temporaryconfig.TempOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetSigner() *signerconfig.SignerOptions {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetAppConfig() *types.AppConfig {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetDraftStore() interface{} {
	return nil
}

func (m *ConfigurableMockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig {
	return nil
}

// MockClock 统一的时钟Mock实现
//
// ✅ **设计原则**：可配置时间，支持时间推进
// 📋 **使用场景**：需要确定性时间的测试
type MockClock struct {
	now   time.Time
	mutex sync.Mutex
}

// NewMockClock 创建Mock时钟
func NewMockClock(now time.Time) *MockClock {
	return &MockClock{now: now}
}

// Now 返回当前时间
func (m *MockClock) Now() time.Time {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.now
}

// Since 返回自指定时间以来的持续时间
func (m *MockClock) Since(t time.Time) time.Duration {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.now.Sub(t)
}

// Unix 返回Unix时间戳
func (m *MockClock) Unix() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.now.Unix()
}

// UnixNano 返回Unix纳秒时间戳
func (m *MockClock) UnixNano() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.now.UnixNano()
}

// Advance 推进时间
func (m *MockClock) Advance(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.now = m.now.Add(duration)
}

// SetTime 设置时间
func (m *MockClock) SetTime(t time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.now = t
}
