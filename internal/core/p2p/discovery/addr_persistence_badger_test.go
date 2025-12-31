package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBadgerStore 是一个最小可用的 BadgerStore mock（仅覆盖本测试用到的方法）
type mockBadgerStore struct {
	mu sync.RWMutex
	kv map[string][]byte
}

func newMockBadgerStore() *mockBadgerStore {
	return &mockBadgerStore{kv: make(map[string][]byte)}
}

// memAddrStore 用于测试 prune/回填等逻辑的内存 AddrStore
type memAddrStore struct {
	mu      sync.Mutex
	recs    map[string]*PeerAddrRecord
	deleted map[string]bool
}

var _ AddrStore = (*memAddrStore)(nil)

func (m *memAddrStore) LoadAll(_ context.Context) ([]*PeerAddrRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*PeerAddrRecord, 0, len(m.recs))
	for _, r := range m.recs {
		out = append(out, r)
	}
	return out, nil
}

func (m *memAddrStore) Get(_ context.Context, peerID string) (*PeerAddrRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[peerID]
	return r, ok, nil
}

func (m *memAddrStore) Upsert(_ context.Context, rec *PeerAddrRecord) error {
	if rec == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recs[rec.PeerID] = rec
	return nil
}

func (m *memAddrStore) Delete(_ context.Context, peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recs, peerID)
	m.deleted[peerID] = true
	return nil
}

func (m *memAddrStore) Close() error { return nil }

func (m *mockBadgerStore) Close() error { return nil }

func (m *mockBadgerStore) Get(_ context.Context, key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.kv[string(key)]
	if !ok {
		return nil, nil
	}
	cp := append([]byte{}, v...)
	return cp, nil
}

func (m *mockBadgerStore) Set(_ context.Context, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kv[string(key)] = append([]byte{}, value...)
	return nil
}

func (m *mockBadgerStore) SetWithTTL(_ context.Context, key, value []byte, _ time.Duration) error {
	return m.Set(context.Background(), key, value)
}

func (m *mockBadgerStore) Delete(_ context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.kv, string(key))
	return nil
}

func (m *mockBadgerStore) Exists(_ context.Context, key []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.kv[string(key)]
	return ok, nil
}

func (m *mockBadgerStore) GetMany(_ context.Context, keys [][]byte) (map[string][]byte, error) {
	out := make(map[string][]byte)
	for _, k := range keys {
		v, _ := m.Get(context.Background(), k)
		if v != nil {
			out[string(k)] = v
		}
	}
	return out, nil
}

func (m *mockBadgerStore) SetMany(_ context.Context, entries map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range entries {
		m.kv[k] = append([]byte{}, v...)
	}
	return nil
}

func (m *mockBadgerStore) DeleteMany(_ context.Context, keys [][]byte) error {
	for _, k := range keys {
		_ = m.Delete(context.Background(), k)
	}
	return nil
}

func (m *mockBadgerStore) PrefixScan(_ context.Context, prefix []byte) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]byte)
	p := string(prefix)
	for k, v := range m.kv {
		if len(k) >= len(p) && k[:len(p)] == p {
			out[k] = append([]byte{}, v...)
		}
	}
	return out, nil
}

func (m *mockBadgerStore) RangeScan(_ context.Context, _, _ []byte) (map[string][]byte, error) {
	return nil, errors.New("not implemented")
}

func (m *mockBadgerStore) RunInTransaction(_ context.Context, _ func(tx storageiface.BadgerTransaction) error) error {
	return errors.New("not implemented")
}

func TestBadgerAddrStore_UpsertAndGet(t *testing.T) {
	ms := newMockBadgerStore()
	s := &BadgerAddrStore{
		store:  ms,
		prefix: []byte("peer_addrs/v1/"),
		logger: nil,
	}

	ctx := context.Background()
	rec := &PeerAddrRecord{
		Version:    PeerAddrRecordVersion,
		PeerID:     "peer-1",
		Addrs:      []string{"/ip4/127.0.0.1/tcp/1"},
		LastSeenAt: time.Now(),
	}
	require.NoError(t, s.Upsert(ctx, rec))

	got, ok, err := s.Get(ctx, "peer-1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "peer-1", got.PeerID)
	assert.Equal(t, rec.Addrs, got.Addrs)
}

func TestAddrManager_PruneOnce(t *testing.T) {
	ms := &memAddrStore{
		recs:    make(map[string]*PeerAddrRecord),
		deleted: make(map[string]bool),
	}
	ms.Upsert(context.Background(), &PeerAddrRecord{
		Version:    PeerAddrRecordVersion,
		PeerID:     "old",
		LastSeenAt: time.Now().Add(-2 * time.Hour),
	})
	ms.Upsert(context.Background(), &PeerAddrRecord{
		Version:    PeerAddrRecordVersion,
		PeerID:     "recent",
		LastSeenAt: time.Now().Add(-10 * time.Minute),
	})
	ms.Upsert(context.Background(), &PeerAddrRecord{
		Version:         PeerAddrRecordVersion,
		PeerID:          "failed",
		LastSeenAt:      time.Now().Add(-10 * time.Minute),
		FailCount:       50,
		LastConnectedAt: time.Now().Add(-48 * time.Hour),
	})
	ms.Upsert(context.Background(), &PeerAddrRecord{
		Version:     PeerAddrRecordVersion,
		PeerID:      "boot",
		LastSeenAt:  time.Now().Add(-1000 * time.Hour),
		IsBootstrap: true,
	})

	am := &AddrManager{
		store:     ms,
		recordTTL: 1 * time.Hour,
	}
	am.pruneOnce()

	ms.mu.Lock()
	defer ms.mu.Unlock()
	assert.True(t, ms.deleted["old"])
	assert.False(t, ms.deleted["recent"])
	assert.True(t, ms.deleted["failed"])
	assert.False(t, ms.deleted["boot"])
}

// routingMock 用于测试 lookup 并发限流
type routingMock struct {
	activeMu sync.Mutex
	active   int
	max      int
	blockCh  chan struct{}
}

func (r *routingMock) AdvertiseAndFindPeers(_ context.Context, _ string) (<-chan libpeer.AddrInfo, error) {
	ch := make(chan libpeer.AddrInfo)
	close(ch)
	return ch, nil
}
func (r *routingMock) FindPeer(_ context.Context, id libpeer.ID) (libpeer.AddrInfo, error) {
	r.activeMu.Lock()
	r.active++
	if r.active > r.max {
		r.max = r.active
	}
	r.activeMu.Unlock()

	<-r.blockCh

	r.activeMu.Lock()
	r.active--
	r.activeMu.Unlock()
	return libpeer.AddrInfo{ID: id, Addrs: nil}, nil
}
func (r *routingMock) RoutingTableSize() int { return 0 }
func (r *routingMock) Offline() bool         { return false }

func TestAddrManager_LookupSemaphoreCapsConcurrency(t *testing.T) {
	// 用一个真实 host 以满足 AddrManager 依赖
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	rm := &routingMock{blockCh: make(chan struct{})}
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 3,
		LookupTimeout:        5 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, rm, cfg, nil)
	defer am.Stop()

	// 快速触发大量不同 peer 的 lookup
	for i := 0; i < 50; i++ {
		am.triggerAddrLookup(libpeer.ID(fmt.Sprintf("peer-%d", i)))
	}

	// 放行执行中的 FindPeer
	close(rm.blockCh)
	time.Sleep(50 * time.Millisecond)

	rm.activeMu.Lock()
	max := rm.max
	rm.activeMu.Unlock()

	assert.LessOrEqual(t, max, cfg.MaxConcurrentLookups)
}


