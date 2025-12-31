package kbucket

import (
	"context"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	libhost "github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"

	apiconfig "github.com/weisyn/v1/internal/config/api"
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	candidatepoolconfig "github.com/weisyn/v1/internal/config/candidatepool"
	clockconfig "github.com/weisyn/v1/internal/config/clock"
	complianceconfig "github.com/weisyn/v1/internal/config/compliance"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/config/event"
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

	"github.com/weisyn/v1/pkg/constants/protocols"
	cfgiface "github.com/weisyn/v1/pkg/interfaces/config"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/zap"
)

type stubP2PService struct{ host libhost.Host }

func (s stubP2PService) Host() libhost.Host              { return s.host }
func (s stubP2PService) Swarm() p2pi.Swarm               { return nil }
func (s stubP2PService) Routing() p2pi.Routing           { return nil }
func (s stubP2PService) Discovery() p2pi.Discovery       { return nil }
func (s stubP2PService) Connectivity() p2pi.Connectivity { return nil }
func (s stubP2PService) Diagnostics() p2pi.Diagnostics   { return nil }

type nopLogger struct{}

func (nopLogger) Debug(string)                        {}
func (nopLogger) Debugf(string, ...interface{})       {}
func (nopLogger) Info(string)                         {}
func (nopLogger) Infof(string, ...interface{})        {}
func (nopLogger) Warn(string)                         {}
func (nopLogger) Warnf(string, ...interface{})        {}
func (nopLogger) Error(string)                        {}
func (nopLogger) Errorf(string, ...interface{})       {}
func (nopLogger) Fatal(string)                        {}
func (nopLogger) Fatalf(string, ...interface{})       {}
func (nopLogger) With(...interface{}) logiface.Logger { return nopLogger{} }
func (nopLogger) Sync() error                         { return nil }
func (nopLogger) GetZapLogger() *zap.Logger           { return zap.NewNop() }

// stubConfigProvider implements cfgiface.Provider for tests.
type stubConfigProvider struct{ ns string }

func (s stubConfigProvider) GetNode() *nodeconfig.NodeOptions                   { return nil }
func (s stubConfigProvider) GetAPI() *apiconfig.APIOptions                      { return nil }
func (s stubConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions { return nil }
func (s stubConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions    { return nil }
func (s stubConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions             { return nil }
func (s stubConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions {
	return nil
}
func (s stubConfigProvider) GetNetwork() *networkconfig.NetworkOptions { return nil }
func (s stubConfigProvider) GetSync() *syncconfig.SyncOptions          { return nil }
func (s stubConfigProvider) GetLog() *logconfig.LogOptions             { return nil }
func (s stubConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig {
	return nil
}
func (s stubConfigProvider) GetEvent() *event.EventOptions                      { return nil }
func (s stubConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions { return nil }
func (s stubConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions { return nil }
func (s stubConfigProvider) GetClock() *clockconfig.ClockOptions                { return nil }
func (s stubConfigProvider) GetEnvironment() string                             { return "test" }
func (s stubConfigProvider) GetChainMode() string                               { return "public" }
func (s stubConfigProvider) GetInstanceDataDir() string                         { return "./data/test" }
func (s stubConfigProvider) GetNetworkNamespace() string                        { return s.ns }
func (s stubConfigProvider) GetSecurity() *types.UserSecurityConfig             { return nil }
func (s stubConfigProvider) GetAccessControlMode() string                       { return "open" }
func (s stubConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig {
	return nil
}
func (s stubConfigProvider) GetPSK() *types.UserPSKConfig               { return nil }
func (s stubConfigProvider) GetPermissionModel() string                 { return "public" }
func (s stubConfigProvider) GetBadger() *badgerconfig.BadgerOptions     { return nil }
func (s stubConfigProvider) GetMemory() *memoryconfig.MemoryOptions     { return nil }
func (s stubConfigProvider) GetFile() *fileconfig.FileOptions           { return nil }
func (s stubConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions     { return nil }
func (s stubConfigProvider) GetTemporary() *temporaryconfig.TempOptions { return nil }
func (s stubConfigProvider) GetSigner() *signerconfig.SignerOptions     { return nil }
func (s stubConfigProvider) GetDraftStore() interface{}                 { return nil }
func (s stubConfigProvider) GetAppConfig() *types.AppConfig             { return nil }
func (s stubConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig {
	return nil
}

func TestSupportsProtocol_QualifiedFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("create host1: %v", err)
	}
	defer h1.Close()

	h2, err := libp2p.New()
	if err != nil {
		t.Fatalf("create host2: %v", err)
	}
	defer h2.Close()

	// connect so that peerstore/network are populated in realistic shape
	if err := h1.Connect(ctx, libpeer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		t.Fatalf("connect: %v", err)
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: "testns"})
	mgr := mgrIface.(*RoutingTableManager)

	// qualified only
	qualified := protocols.QualifyProtocol(protocols.ProtocolSyncHelloV2, "testns")
	_ = h1.Peerstore().AddProtocols(h2.ID(), libprotocol.ID(qualified))

	ok, err := mgr.SupportsProtocol(h2.ID(), protocols.ProtocolSyncHelloV2)
	if err != nil {
		t.Fatalf("SupportsProtocol err: %v", err)
	}
	if !ok {
		t.Fatalf("expected SupportsProtocol=true for qualified protocol")
	}
}

func TestFindClosestPeersForProtocol_FiltersByPeerstoreProtocols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("create host1: %v", err)
	}
	defer h1.Close()

	h2, err := libp2p.New()
	if err != nil {
		t.Fatalf("create host2: %v", err)
	}
	defer h2.Close()

	if err := h1.Connect(ctx, libpeer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		t.Fatalf("connect: %v", err)
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: ""})
	mgr := mgrIface.(*RoutingTableManager)
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	// Insert peer into routing table directly (avoid WES/chain identity admission logic for this unit test)
	mgr.tabLock.Lock()
	pid := h2.ID()
	dhtID := ConvertPeerID(pid)
	cpl := CommonPrefixLen(mgr.localID, dhtID)
	idx := cpl
	if idx >= len(mgr.buckets) {
		idx = len(mgr.buckets) - 1
	}
	mgr.ensureBucket(idx)
	mgr.buckets[idx].pushFront(&PeerInfo{Id: pid, AddedAt: time.Now(), dhtId: dhtID, peerState: PeerStateActive, healthScore: 100})
	mgr.tabLock.Unlock()

	// no protocol => filtered out
	peers := mgr.FindClosestPeersForProtocol([]byte(h1.ID()), 8, protocols.ProtocolSyncHelloV2)
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers without protocol, got=%d", len(peers))
	}

	_ = h1.Peerstore().AddProtocols(pid, libprotocol.ID(protocols.ProtocolSyncHelloV2))
	peers = mgr.FindClosestPeersForProtocol([]byte(h1.ID()), 8, protocols.ProtocolSyncHelloV2)
	if len(peers) == 0 || peers[0] != pid {
		t.Fatalf("expected peer %s to be selected, got=%v", pid, peers)
	}
}

var _ cfgiface.Provider = (*stubConfigProvider)(nil)
