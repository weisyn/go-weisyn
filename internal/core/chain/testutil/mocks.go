package testutil

import (
	"context"
	"io"
	"sync"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"

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
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// MockConfigProvider 模拟配置提供者
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
	return &types.AppConfig{}
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

// MockNetwork 模拟网络服务
type MockNetwork struct {
	mu sync.RWMutex
}

func NewMockNetwork() *MockNetwork {
	return &MockNetwork{}
}

// RegisterStreamHandler 实现 network.Network 接口
func (m *MockNetwork) RegisterStreamHandler(protoID string, handler network.MessageHandler, opts ...network.RegisterOption) error {
	return nil
}

// UnregisterStreamHandler 实现 network.Network 接口
func (m *MockNetwork) UnregisterStreamHandler(protoID string) error {
	return nil
}

// Subscribe 实现 network.Network 接口
func (m *MockNetwork) Subscribe(topic string, handler network.SubscribeHandler, opts ...network.SubscribeOption) (func() error, error) {
	return func() error { return nil }, nil
}

// Call 实现 network.Network 接口
func (m *MockNetwork) Call(ctx context.Context, to peer.ID, protoID string, req []byte, opts *types.TransportOptions) ([]byte, error) {
	return nil, nil
}

// OpenStream 实现 network.Network 接口
func (m *MockNetwork) OpenStream(ctx context.Context, to peer.ID, protoID string, opts *types.TransportOptions) (network.StreamHandle, error) {
	return nil, nil
}

// Publish 实现 network.Network 接口
func (m *MockNetwork) Publish(ctx context.Context, topic string, data []byte, opts *types.PublishOptions) error {
	return nil
}

// ListProtocols 实现 network.Network 接口
func (m *MockNetwork) ListProtocols() []types.ProtocolInfo {
	return nil
}

// GetProtocolInfo 实现 network.Network 接口
func (m *MockNetwork) GetProtocolInfo(protoID string) *types.ProtocolInfo {
	return nil
}

// GetTopicPeers 实现 network.Network 接口
func (m *MockNetwork) GetTopicPeers(topic string) []peer.ID {
	return nil
}

// IsSubscribed 实现 network.Network 接口
func (m *MockNetwork) IsSubscribed(topic string) bool {
	return false
}

// CheckProtocolSupport 实现 network.Network 接口
func (m *MockNetwork) CheckProtocolSupport(ctx context.Context, peerID peer.ID, protocol string) (bool, error) {
	return true, nil
}

// MockRoutingTableManager 模拟路由表管理器
type MockRoutingTableManager struct {
	mu sync.RWMutex
}

func NewMockRoutingTableManager() *MockRoutingTableManager {
	return &MockRoutingTableManager{}
}

// GetRoutingTable 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) GetRoutingTable() *kademlia.RoutingTable {
	return &kademlia.RoutingTable{}
}

// AddPeer 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) AddPeer(ctx context.Context, addrInfo peer.AddrInfo) (bool, error) {
	return true, nil
}

// RemovePeer 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) RemovePeer(peerID peer.ID) error {
	return nil
}

// FindClosestPeers 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) FindClosestPeers(target []byte, count int) []peer.ID {
	return nil
}

// RecordPeerSuccess 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) RecordPeerSuccess(peerID peer.ID) {
}

// IsReady 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) IsReady() bool {
	return true
}

// WaitForReady 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) WaitForReady(ctx context.Context) error {
	return nil
}

// RecordPeerFailure 实现 kademlia.RoutingTableManager 接口
func (m *MockRoutingTableManager) RecordPeerFailure(peerID peer.ID) {
}

// QuarantineIncompatiblePeer 实现 kademlia.RoutingTableManager 接口
// 🆕 2025-12-18: 直接隔离不兼容的节点
func (m *MockRoutingTableManager) QuarantineIncompatiblePeer(peerID peer.ID, reason string) {
	// Mock 实现：不做任何操作
}

// MockP2PService 模拟P2P服务
type MockP2PService struct {
	mu sync.RWMutex
}

func NewMockP2PService() *MockP2PService {
	return &MockP2PService{}
}

// Host 实现 p2pi.Service 接口
func (m *MockP2PService) Host() libhost.Host {
	return nil
}

// Swarm 实现 p2pi.Service 接口
func (m *MockP2PService) Swarm() p2pi.Swarm {
	return nil
}

// Routing 实现 p2pi.Service 接口
func (m *MockP2PService) Routing() p2pi.Routing {
	return nil
}

// Discovery 实现 p2pi.Service 接口
func (m *MockP2PService) Discovery() p2pi.Discovery {
	return nil
}

// Connectivity 实现 p2pi.Service 接口
func (m *MockP2PService) Connectivity() p2pi.Connectivity {
	return nil
}

// Diagnostics 实现 p2pi.Service 接口
func (m *MockP2PService) Diagnostics() p2pi.Diagnostics {
	return nil
}

// MockTempStore 模拟临时存储
type MockTempStore struct {
	data map[string][]byte
	mu   sync.RWMutex
}

func NewMockTempStore() *MockTempStore {
	return &MockTempStore{
		data: make(map[string][]byte),
	}
}

// Close 实现 storage.TempStore 接口
func (m *MockTempStore) Close() error {
	return nil
}

// CreateTempFile 实现 storage.TempStore 接口
func (m *MockTempStore) CreateTempFile(ctx context.Context, prefix, suffix string) (id string, file io.ReadWriteCloser, err error) {
	return "", nil, nil
}

// CreateTempFileWithContent 实现 storage.TempStore 接口
func (m *MockTempStore) CreateTempFileWithContent(ctx context.Context, prefix, suffix string, content []byte) (id string, err error) {
	return "", nil
}

// GetTempFile 实现 storage.TempStore 接口
func (m *MockTempStore) GetTempFile(ctx context.Context, id string) (content []byte, err error) {
	return nil, nil
}

// OpenTempFile 实现 storage.TempStore 接口
func (m *MockTempStore) OpenTempFile(ctx context.Context, id string) (file io.ReadWriteCloser, err error) {
	return nil, nil
}

// RemoveTempFile 实现 storage.TempStore 接口
func (m *MockTempStore) RemoveTempFile(ctx context.Context, id string) error {
	return nil
}

// CreateTempDir 实现 storage.TempStore 接口
func (m *MockTempStore) CreateTempDir(ctx context.Context, prefix string) (id string, err error) {
	return "", nil
}

// RemoveTempDir 实现 storage.TempStore 接口
func (m *MockTempStore) RemoveTempDir(ctx context.Context, id string) error {
	return nil
}

// ListTempFiles 实现 storage.TempStore 接口
func (m *MockTempStore) ListTempFiles(ctx context.Context, pattern string) ([]types.TempFileInfo, error) {
	return nil, nil
}

// CleanupExpired 实现 storage.TempStore 接口
func (m *MockTempStore) CleanupExpired(ctx context.Context) (int, error) {
	return 0, nil
}

// SetExpiration 实现 storage.TempStore 接口
func (m *MockTempStore) SetExpiration(ctx context.Context, id string, duration time.Duration) error {
	return nil
}

// MockRuntimeState 模拟运行时状态
type MockRuntimeState struct {
	mu            sync.RWMutex
	syncMode      p2pi.SyncMode
	syncStatus    p2pi.SyncStatus
	isFullySynced bool
	isOnline      bool
	miningEnabled bool
}

func NewMockRuntimeState() *MockRuntimeState {
	return &MockRuntimeState{
		syncMode:      p2pi.SyncModeFull,
		syncStatus:    p2pi.SyncStatusSynced,
		isFullySynced: true,
		isOnline:      true,
		miningEnabled: false,
	}
}

// GetSyncMode 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) GetSyncMode() p2pi.SyncMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncMode
}

// SetSyncMode 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetSyncMode(ctx context.Context, mode p2pi.SyncMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncMode = mode
	return nil
}

// GetSyncStatus 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) GetSyncStatus() p2pi.SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncStatus
}

// SetSyncStatus 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetSyncStatus(status p2pi.SyncStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncStatus = status
}

// GetIsFullySynced 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) GetIsFullySynced() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isFullySynced
}

// SetIsFullySynced 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetIsFullySynced(synced bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isFullySynced = synced
}

// IsOnline 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) IsOnline() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isOnline
}

// SetIsOnline 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetIsOnline(online bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isOnline = online
}

// IsMiningEnabled 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) IsMiningEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.miningEnabled
}

// SetMiningEnabled 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetMiningEnabled(ctx context.Context, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.miningEnabled = enabled
	return nil
}

// IsConsensusEligible 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) IsConsensusEligible() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncMode == p2pi.SyncModeFull && m.isFullySynced && m.isOnline
}

// IsVoterInRound 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) IsVoterInRound() bool {
	return m.IsConsensusEligible()
}

// IsProposerCandidate 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) IsProposerCandidate() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.IsConsensusEligible() && m.miningEnabled
}

// GetSnapshot 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) GetSnapshot() p2pi.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return p2pi.Snapshot{
		SyncMode:            m.syncMode,
		SyncStatus:          m.syncStatus,
		IsFullySynced:       m.isFullySynced,
		IsOnline:            m.isOnline,
		MiningEnabled:       m.miningEnabled,
		IsConsensusEligible: m.IsConsensusEligible(),
		IsVoterInRound:      m.IsVoterInRound(),
		IsProposerCandidate: m.IsProposerCandidate(),
	}
}

// SetOnSyncModeChanged 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetOnSyncModeChanged(callback func(oldMode, newMode p2pi.SyncMode)) {
	// Mock implementation - no-op
}

// SetOnMiningEnabledChanged 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetOnMiningEnabledChanged(callback func(enabled bool)) {
	// Mock implementation - no-op
}

// SetOnSyncStatusChanged 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) SetOnSyncStatusChanged(callback func(oldStatus, newStatus p2pi.SyncStatus)) {
	// Mock implementation - no-op
}

// UpdateSyncStatusFromSyncService 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) UpdateSyncStatusFromSyncService(
	currentHeight uint64,
	networkLatestHeight uint64,
	syncLagThreshold uint64,
	isSyncing bool,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if isSyncing {
		m.syncStatus = p2pi.SyncStatusSyncing
	} else if currentHeight >= networkLatestHeight {
		m.syncStatus = p2pi.SyncStatusSynced
		m.isFullySynced = true
	} else if networkLatestHeight-currentHeight > syncLagThreshold {
		m.syncStatus = p2pi.SyncStatusLagging
	} else {
		m.syncStatus = p2pi.SyncStatusSynced
	}
}

// StartPeriodicSyncStatusUpdate 实现 p2pi.RuntimeState 接口
func (m *MockRuntimeState) StartPeriodicSyncStatusUpdate(
	ctx context.Context,
	getCurrentHeight func() uint64,
	getNetworkLatestHeight func() uint64,
	syncLagThreshold uint64,
	updateInterval time.Duration,
) {
	// Mock implementation - no-op
}
