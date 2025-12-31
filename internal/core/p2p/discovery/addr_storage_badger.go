package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	badgercore "github.com/weisyn/v1/internal/core/infrastructure/storage/badger"
	badgercfg "github.com/weisyn/v1/internal/config/storage/badger"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// BadgerAddrStore 使用专用 BadgerDB 作为 peer_addrs 的持久化后端
//
// Key 设计：
// - key = prefix + peerID（ASCII）
// - prefix 默认 "peer_addrs/v1/"
type BadgerAddrStore struct {
	store  storageiface.BadgerStore
	prefix []byte
	logger logiface.Logger
}

type badgerAddrStoreConfig struct {
	Dir            string
	NamespacePrefix string
}

func newBadgerAddrStore(cfg badgerAddrStoreConfig, logger logiface.Logger) (*BadgerAddrStore, error) {
	if strings.TrimSpace(cfg.Dir) == "" {
		return nil, fmt.Errorf("badger dir is required")
	}
	ns := strings.TrimSpace(cfg.NamespacePrefix)
	if ns == "" {
		ns = "peer_addrs/v1/"
	}

	// 使用内部 badger 基础设施封装创建专用DB
	bcfg := badgercfg.NewFromOptions(&badgercfg.BadgerOptions{
		Path:                 cfg.Dir,
		SyncWrites:           true,
		MemTableSize:         64 << 20, // 64MB 对 addr 记录已足够
		EnableAutoCompaction: true,
	})
	bstore := badgercore.New(bcfg, logger)
	if bstore == nil {
		return nil, fmt.Errorf("create badger store failed")
	}

	return &BadgerAddrStore{
		store:  bstore,
		prefix: []byte(ns),
		logger: logger,
	}, nil
}

func (s *BadgerAddrStore) key(peerID string) []byte {
	return append(append([]byte{}, s.prefix...), []byte(peerID)...)
}

func (s *BadgerAddrStore) LoadAll(ctx context.Context) ([]*PeerAddrRecord, error) {
	m, err := s.store.PrefixScan(ctx, s.prefix)
	if err != nil {
		return nil, err
	}
	out := make([]*PeerAddrRecord, 0, len(m))
	for _, v := range m {
		var rec PeerAddrRecord
		if err := json.Unmarshal(v, &rec); err != nil {
			// 容忍单条损坏，跳过并记录
			if s.logger != nil {
				s.logger.Warnf("peer_addrs badger unmarshal failed: %v", err)
			}
			continue
		}
		out = append(out, &rec)
	}
	return out, nil
}

func (s *BadgerAddrStore) Get(ctx context.Context, peerID string) (*PeerAddrRecord, bool, error) {
	val, err := s.store.Get(ctx, s.key(peerID))
	if err != nil {
		return nil, false, err
	}
	if val == nil {
		return nil, false, nil
	}
	var rec PeerAddrRecord
	if err := json.Unmarshal(val, &rec); err != nil {
		return nil, false, err
	}
	return &rec, true, nil
}

func (s *BadgerAddrStore) Upsert(ctx context.Context, rec *PeerAddrRecord) error {
	if rec == nil {
		return nil
	}
	if rec.Version == 0 {
		rec.Version = PeerAddrRecordVersion
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	
	// 注意：BadgerDB的ValueLogGC由底层存储引擎自动管理
	// 通过pruneLoop定期清理过期记录已足够控制存储大小
	return s.store.Set(ctx, s.key(rec.PeerID), b)
}

func (s *BadgerAddrStore) Delete(ctx context.Context, peerID string) error {
	return s.store.Delete(ctx, s.key(peerID))
}

func (s *BadgerAddrStore) Close() error {
	if s.store == nil {
		return nil
	}
	return s.store.Close()
}


