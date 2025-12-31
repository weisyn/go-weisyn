// Package badger 提供基于BadgerDB的存储实现
package badger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	log "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	interfaces "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/utils"
	runtimeutil "github.com/weisyn/v1/pkg/utils/runtime"
	"go.uber.org/zap"
)

// Store 实现BadgerStore接口
type Store struct {
	db         *badgerdb.DB
	config     *badgerconfig.Config
	logger     log.Logger
	cancelFunc context.CancelFunc // 用于取消后台任务的函数

	// 彻底修复：避免 Close 过程中仍被写入，触发 Badger y.AssertTrue(db.mt != nil) 的 fatal 退出
	closing int32
	writeWg sync.WaitGroup
}

// New 创建新的BadgerStore实例
// 初始化数据库并启动维护任务
func New(config *badgerconfig.Config, logger log.Logger) interfaces.BadgerStore {
	if logger == nil {
		logger = nopLogger{}
	}
	store := &Store{
		config: config,
		logger: logger,
	}

	// 确保数据目录存在
	dataDir := config.GetPath()
	if dataDir == "" {
		// 使用默认路径作为备用，确保路径解析正确
		dataDir = utils.ResolveDataPath("./data/badger")
		logger.Warnf("BadgerDB数据目录路径未配置，使用默认路径: %s", dataDir)
	}

	logger.Infof("初始化BadgerDB存储，数据目录: %s", dataDir)

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		logger.Errorf("无法创建BadgerDB数据目录: %v", err)
		return nil
	}

	// 创建BadgerDB配置
	opts := badgerdb.DefaultOptions(dataDir)
	// 使用简化配置
	opts.SyncWrites = config.IsSyncWritesEnabled()
	opts.MemTableSize = config.GetMemTableSize()

	// 🆕 2025-12-18 修复：降低 ValueLogFileSize 减少 mmap 虚拟地址占用
	//
	// 问题：BadgerDB 使用 mmap 将 value log 文件映射到虚拟地址空间，
	// 默认 ValueLogFileSize=1GB，导致 runtime.MemStats.HeapAlloc 虚高（可达 100GB+）。
	//
	// 解决：将 ValueLogFileSize 从 1GB 降低到 512MB，减少单个文件的 mmap 占用。
	//
	// 权衡：
	// - 优点：减少虚拟地址空间占用，降低 HeapAlloc 统计误导
	// - 缺点：产生更多小文件，可能增加文件描述符占用（但影响较小）
	opts.ValueLogFileSize = 512 << 20 // 512MB（而非默认 1GB）

	// 🆕 P2 修复：统一降低 Badger block/index cache，防止 RSS 内存持续增长
	//
	// 问题：badger 默认 BlockCacheSize=256MB，与 P2P peerstore 叠加后容易导致 RSS 过高
	// 解决：所有环境统一使用 64MB 缓存，小内存容器进一步降低到 32MB
	//
	// 缓存大小选择依据：
	// - 64MB: 足够大多数链数据索引查询，同时保持合理的 RSS 占用
	// - 32MB: 小内存容器（<= 4GB）的保守配置
	limit, ok, _ := runtimeutil.GetCgroupMemoryLimitBytes()
	limitMB := uint64(0)
	if ok && limit > 0 {
		limitMB = limit / 1024 / 1024
	}

	if limitMB > 0 && limitMB <= 4096 {
		// 小内存容器（<= 4GB）：使用更保守的 32MB 缓存
		opts.BlockCacheSize = 32 << 20
		opts.IndexCacheSize = 32 << 20
		opts.NumMemtables = 2 // 减少 memtable 数量
	} else {
		// 所有其他环境（包括非容器）：统一使用 64MB 缓存
		opts.BlockCacheSize = 64 << 20
		opts.IndexCacheSize = 64 << 20
		opts.NumMemtables = 2 // 减少 memtable 数量
	}

	// 设置表现参数
	opts.NumCompactors = 2            // 后台整理工作线程数
	opts.NumLevelZeroTables = 5       // Level 0最大表数
	opts.NumLevelZeroTablesStall = 10 // Level 0表数触发压缩的阈值

	// 设置日志（带 dataDir，便于写入 BADGER_FATAL 标记用于下次启动自愈）
	opts.Logger = newBadgerLogger(logger, dataDir)

	// 声明数据库变量
	var db *badgerdb.DB

	// 检查是否强制使用内存模式
	if os.Getenv("WES_MEMORY_ONLY_MODE") == "true" {
		logger.Infof("🧠 检测到内存数据库模式标志，直接启用内存BadgerDB")
		fmt.Printf("🧠 正在启动内存数据库模式...\n")

		// 直接创建内存数据库
		memOpts := badgerdb.DefaultOptions("")
		memOpts = memOpts.WithInMemory(true)
		memOpts.Logger = newBadgerLogger(logger, "")
		// 🆕 P2 修复：与磁盘模式保持一致的缓存配置
		if limitMB > 0 && limitMB <= 4096 {
			memOpts.BlockCacheSize = 32 << 20
			memOpts.IndexCacheSize = 32 << 20
			memOpts.NumMemtables = 2
		} else {
			memOpts.BlockCacheSize = 64 << 20
			memOpts.IndexCacheSize = 64 << 20
			memOpts.NumMemtables = 2
		}
		memDB, memErr := badgerdb.Open(memOpts)
		if memErr != nil {
			logger.Errorf("无法打开内存BadgerDB: %v", memErr)
			fmt.Printf("❌ 严重错误: 内存数据库启动失败: %v\n", memErr)
			return nil
		}
		db = memDB
		logger.Infof("✅ 内存BadgerDB启动成功（用户显式选择）")
		fmt.Printf("✅ 内存数据库模式启动成功\n")
	} else {
		// 安全打开数据库（磁盘）
		var err error
		db, err = safeOpenDB(dataDir, opts, logger)
		if err != nil {
			logger.Errorf("无法打开BadgerDB(磁盘): %v", err)

			// 默认策略：Fail-fast（禁止隐式回退到内存DB）。
			// 原因：回退到内存DB会导致“索引/元数据不持久化”，但 FileStore/Block 文件仍可能写入磁盘，
			// 从而制造 blocks/ 与 Badger 索引不一致的致命状态（你当前遇到的 649 vs 512 就是典型）。
			//
			// 如确需兼容旧行为（仅建议 dev/test 临时使用），可显式设置：
			// - WES_ALLOW_BADGER_FALLBACK_TO_MEMORY=true
			if os.Getenv("WES_ALLOW_BADGER_FALLBACK_TO_MEMORY") != "true" {
			fmt.Printf("\n")
				fmt.Printf("❌ BadgerDB磁盘数据库打开失败，已拒绝自动回退到内存DB（Fail-fast）\n")
				fmt.Printf("📁 数据目录: %s\n", dataDir)
			fmt.Printf("🛠️  建议操作:\n")
				fmt.Printf("   • 检查是否有多进程占用/锁冲突、目录权限、磁盘空间\n")
				fmt.Printf("   • 如需“临时内存模式”，请显式设置 WES_MEMORY_ONLY_MODE=true\n")
				fmt.Printf("   • 如需“兼容旧行为(不推荐)”，请显式设置 WES_ALLOW_BADGER_FALLBACK_TO_MEMORY=true\n")
			fmt.Printf("\n")
				return nil
			}

			// 兼容旧行为：显式允许时才回退到内存DB
			logger.Warnf("BadgerDB打开失败但允许回退到内存DB: dataDir=%s err=%v", dataDir, err)
			logger.Warn("⚠️ 回退到内存BadgerDB（数据不持久化，程序退出后丢失）")

			memOpts := badgerdb.DefaultOptions("")
			memOpts = memOpts.WithInMemory(true)
			memOpts.Logger = newBadgerLogger(logger, "")
			if limit, ok, _ := runtimeutil.GetCgroupMemoryLimitBytes(); ok && limit > 0 {
				limitMB := limit / 1024 / 1024
				if limitMB <= 6144 {
					memOpts.BlockCacheSize = 64 << 20
					memOpts.IndexCacheSize = 64 << 20
				}
			}
			memDB, memErr := badgerdb.Open(memOpts)
			if memErr != nil {
				logger.Errorf("无法打开内存BadgerDB: %v", memErr)
				fmt.Printf("❌ 严重错误: 内存数据库也无法启动: %v\n", memErr)
				return nil
			}
			db = memDB

			logger.Infof("✅ 内存BadgerDB启动成功（临时模式，显式允许回退）")
			fmt.Printf("✅ 内存数据库模式已启用（显式允许回退）\n\n")
		}
	}

	// 设置数据库实例
	store.db = db

	// 🆕 记录启动时的BadgerDB vlog文件信息（用于内存分析）
	store.logBadgerVlogInfo(dataDir, logger)

	// 启动维护例程
	ctx, cancel := context.WithCancel(context.Background())
	store.cancelFunc = cancel
	store.StartMaintenanceRoutines(ctx)

	// 如果启用自动压缩，设置备份目录并启动自动备份
	if config.IsAutoCompactionEnabled() {
		// 备份目录配置
		backupDir := filepath.Join(dataDir, "backups")
		// 确保备份目录存在
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			logger.Warnf("无法创建备份目录: %v", err)
		} else {
			store.StartAutomaticBackups(ctx, backupDir, 1*time.Hour, 24) // 每小时备份，保留24个（1天）
		}
	}

	logger.Info("BadgerDB存储初始化完成")
	return store
}

// nopLogger 用于在测试/集成测试/工具链等 logger 未注入时，避免 nil 指针崩溃。
// 生产环境应通过 DI 注入真实 logger。
type nopLogger struct{}

func (nopLogger) Debug(string)                           {}
func (nopLogger) Debugf(string, ...interface{})          {}
func (nopLogger) Info(string)                            {}
func (nopLogger) Infof(string, ...interface{})           {}
func (nopLogger) Warn(string)                            {}
func (nopLogger) Warnf(string, ...interface{})           {}
func (nopLogger) Error(string)                           {}
func (nopLogger) Errorf(string, ...interface{})          {}
func (nopLogger) Fatal(string)                           {}
func (nopLogger) Fatalf(string, ...interface{})          {}
func (nopLogger) With(...interface{}) log.Logger         { return nopLogger{} }
func (nopLogger) Sync() error                            { return nil }
func (nopLogger) GetZapLogger() *zap.Logger              { return zap.NewNop() }

// Close 关闭存储并释放资源
func (s *Store) Close() error {
	// 进入关闭态：阻断后续写入，并等待 in-flight 写完成
	if !atomic.CompareAndSwapInt32(&s.closing, 0, 1) {
		return nil
	}

	s.logger.Info("🔧 开始关闭BadgerDB存储...")

	// 取消所有后台任务
	s.logger.Info("🔧 取消后台任务...")
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.logger.Info("🔧 后台任务已取消")
	}

	if s.db == nil {
		s.logger.Info("🔧 数据库连接为空，无需关闭")
		return nil
	}

	// 等待所有写事务退出，避免 Close 过程中仍有 Update/Txn 写入
	waitCh := make(chan struct{})
	go func() {
		s.writeWg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
	case <-time.After(30 * time.Second):
		s.logger.Warn("⚠️ 等待 in-flight 写事务超时（30s），仍继续关闭 BadgerDB（可能导致异常退出）")
	}

	// 快速关闭：跳过垃圾回收和同步，直接关闭数据库
	// 注意：启用了sync_writes=true，数据已经实时同步，无需额外同步
	s.logger.Info("🔧 开始快速关闭BadgerDB（跳过GC和额外同步）...")

	// 关闭数据库
	s.logger.Info("🔧 正在调用db.Close()...")
	if err := s.db.Close(); err != nil {
		// 如果是LOCK文件不存在的错误，只记录警告而不返回错误
		if strings.Contains(err.Error(), "LOCK: no such file or directory") {
			s.logger.Warn("BadgerDB LOCK文件已不存在，这通常是正常的关闭过程")
		} else {
			s.logger.Errorf("🔧 关闭BadgerDB失败: %v", err)
			return fmt.Errorf("关闭BadgerDB失败: %w", err)
		}
	} else {
		s.logger.Info("🔧 db.Close() 调用成功")
	}

	// ✅ 彻底修复：仅在 db.Close 成功后删除运行标记，避免“异常退出但 marker 已被提前删除”导致下次启动无法进入修复流程
	s.logger.Info("🔧 删除运行标记...")
	markerPath := filepath.Join(s.config.GetPath(), "BADGER_RUNNING")
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		s.logger.Warnf("无法删除数据库运行标记: %v", err)
	} else {
		s.logger.Info("🔧 运行标记已删除")
	}

	s.logger.Info("🔧 BadgerDB存储已安全关闭")
	return nil
}

func (s *Store) beginWrite() (func(), error) {
	// 关闭过程中拒绝写入，避免 Badger Close 与写入并发导致 fatal
	if atomic.LoadInt32(&s.closing) == 1 {
		return nil, fmt.Errorf("badger store is closing")
	}
	s.writeWg.Add(1)
	// double-check，避免在 Add 之后进入 closing
	if atomic.LoadInt32(&s.closing) == 1 {
		s.writeWg.Done()
		return nil, fmt.Errorf("badger store is closing")
	}
	return s.writeWg.Done, nil
}

// Get 获取指定键的值
func (s *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	var valCopy []byte
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badgerdb.ErrKeyNotFound {
				return nil // 键不存在时返回nil值和nil错误
			}
			return err
		}

		// 复制值
		valCopy, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("badger获取键失败: %w", err)
	}

	return valCopy, nil
}

// Set 设置键值对
func (s *Store) Set(ctx context.Context, key, value []byte) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Set(key, value)
	})
}

// SetWithTTL 设置键值对并指定过期时间
func (s *Store) SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		entry := badgerdb.NewEntry(key, value).WithTTL(ttl)
		return txn.SetEntry(entry)
	})
}

// Delete 删除指定键的值
func (s *Store) Delete(ctx context.Context, key []byte) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Delete(key)
	})
}

// Exists 检查键是否存在
func (s *Store) Exists(ctx context.Context, key []byte) (bool, error) {
	var exists bool
	err := s.db.View(func(txn *badgerdb.Txn) error {
		_, err := txn.Get(key)
		if err == badgerdb.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("badger检查键存在性失败: %w", err)
	}

	return exists, nil
}

// GetMany 批量获取多个键的值
func (s *Store) GetMany(ctx context.Context, keys [][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		for _, key := range keys {
			item, err := txn.Get(key)
			if err == badgerdb.ErrKeyNotFound {
				continue // 跳过不存在的键
			}
			if err != nil {
				return err
			}

			// 复制值
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			// 使用键的字符串表示作为map的键
			result[string(key)] = val
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("badger批量获取键值失败: %w", err)
	}

	return result, nil
}

// SetMany 批量设置多个键值对
func (s *Store) SetMany(ctx context.Context, entries map[string][]byte) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		for k, v := range entries {
			if err := txn.Set([]byte(k), v); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMany 批量删除多个键
func (s *Store) DeleteMany(ctx context.Context, keys [][]byte) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		for _, key := range keys {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// PrefixScan 按前缀扫描键值对
func (s *Store) PrefixScan(ctx context.Context, prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close() // Badger Iterator.Close() 无返回值

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()

			// 复制键
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)

			// 复制值
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			result[string(keyCopy)] = valCopy
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("badger前缀扫描失败: %w", err)
	}

	return result, nil
}

// RangeScan 范围扫描键值对
func (s *Store) RangeScan(ctx context.Context, startKey, endKey []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close() // Badger Iterator.Close() 无返回值

		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()

			// 如果键超过了endKey，则停止迭代
			if len(endKey) > 0 && compareBytes(k, endKey) >= 0 {
				break
			}

			// 复制键
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)

			// 复制值
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			result[string(keyCopy)] = valCopy
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("badger范围扫描失败: %w", err)
	}

	return result, nil
}

// RunInTransaction 在事务中执行操作
func (s *Store) RunInTransaction(ctx context.Context, fn func(tx interfaces.BadgerTransaction) error) error {
	done, err := s.beginWrite()
	if err != nil {
		return err
	}
	defer done()
	// 创建BadgerDB事务
	txn := s.db.NewTransaction(true)

	// 创建我们的事务包装（带大小估算器）
	tx := &Transaction{
		txn:     txn,
		state:   int32(TxActive),
		sizeEst: NewTxSizeEstimator(0), // 使用默认10MB限制
	}

	// 确保事务最终被关闭
	defer func() {
		// 只有在事务仍然活动的情况下才需要丢弃
		if tx.IsActive() {
			tx.Discard()
		}
	}()

	// 执行用户提供的事务函数
	if err := fn(tx); err != nil {
		// 如果函数返回错误，丢弃事务
		if tx.IsActive() {
			tx.Discard()
		}
		return fmt.Errorf("事务执行失败: %w", err)
	}

	// 如果事务仍处于活动状态，提交它
	if tx.IsActive() {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("事务提交失败: %w", err)
		}
	} else if tx.IsDiscarded() {
		// 如果事务已丢弃，返回错误
		return fmt.Errorf("事务已被丢弃")
	}
	// 如果事务已提交，不需要做什么

	return nil
}

// compareBytes 比较两个字节切片
func compareBytes(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	} else if len(a) > len(b) {
		return 1
	}

	return 0
}

// 安全启动逻辑
func safeOpenDB(dataDir string, opts badgerdb.Options, logger log.Logger) (*badgerdb.DB, error) {
	// 🆕 彻底修复：如果上次运行检测到 Badger 致命前兆（BADGER_FATAL），强制进入修复/恢复路径
	fatalMarkerPath := filepath.Join(dataDir, "BADGER_FATAL")
	if _, ferr := os.Stat(fatalMarkerPath); ferr == nil {
		logger.Warn("检测到 BADGER_FATAL 标记文件：上次运行可能触发了 Badger 致命错误前兆，开始执行真正自动自愈流程（修复/恢复/重建）")

		// 创建临时Store实例用于修复/恢复
		tempConfig := badgerconfig.New(nil)
		tempStore := &Store{
			logger: logger,
			config: tempConfig,
		}

		// 1) 先尝试自动修复（轻量优先）
		if repairErr := tempStore.TryRepair(dataDir); repairErr != nil {
			logger.Errorf("BADGER_FATAL 自动修复失败: %v", repairErr)

			// 2) 有备份则从最近备份恢复
			backupDir := filepath.Join(dataDir, "backups")
			if latestBackup := findLatestBackup(backupDir); latestBackup != "" {
				logger.Warnf("BADGER_FATAL 检测到可用备份，尝试从备份恢复: %s", latestBackup)

				// 备份当前损坏的数据
				recoveryDir := getRecoveryDir(dataDir)
				corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
				if err := backupCorruptedData(dataDir, corruptedBackupDir, logger); err != nil {
					logger.Warnf("备份损坏数据失败: %v", err)
				}

				if restoreErr := tempStore.RestoreFromBackup(context.Background(), latestBackup, dataDir); restoreErr != nil {
					logger.Errorf("BADGER_FATAL 从备份恢复失败: %v", restoreErr)
					// 继续走强制修复/重建
				} else {
					logger.Info("BADGER_FATAL 从备份恢复成功")
				}
			}

			// 3) 备份恢复不可用或失败：尝试强制修复
			if forceErr := forceRepairDatabase(dataDir, opts, logger); forceErr != nil {
				logger.Warnf("BADGER_FATAL 强制修复失败，将重建数据库目录（会丢失未备份的数据）: %v", forceErr)

				// 备份损坏的数据库目录
				recoveryDir := getRecoveryDir(dataDir)
				corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
				if backupErr := backupCorruptedData(dataDir, corruptedBackupDir, logger); backupErr != nil {
					logger.Warnf("备份损坏数据失败: %v", backupErr)
				} else {
					logger.Infof("已备份损坏的数据库到: %s", corruptedBackupDir)
				}

				// 删除并重建目录
				if rmErr := os.RemoveAll(dataDir); rmErr != nil {
					return nil, fmt.Errorf("BADGER_FATAL 无法删除损坏的数据库目录: %w", rmErr)
				}
				if mkErr := os.MkdirAll(dataDir, 0700); mkErr != nil {
					return nil, fmt.Errorf("BADGER_FATAL 无法创建新的数据库目录: %w", mkErr)
				}
				logger.Info("BADGER_FATAL 已重建数据库目录完成")
			} else {
				logger.Info("BADGER_FATAL 强制修复成功")
			}
		} else {
			logger.Info("BADGER_FATAL 自动修复成功")
		}

		// 注意：此处不直接删除标记，只有在成功打开数据库后再移除
	}

	// 检查是否存在未完成标记
	markerPath := filepath.Join(dataDir, "BADGER_RUNNING")
	_, err := os.Stat(markerPath)

	if err == nil {
		// 存在标记，可能是异常关闭
		// 但也可能只是标记文件没删除，先尝试直接删除标记并打开
		logger.Warn("检测到BADGER_RUNNING标记文件，可能是上次未正常关闭")
		logger.Info("先尝试删除标记文件并直接打开数据库...")

		// 删除标记文件
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("无法删除标记文件: %v", err)
		} else if err == nil {
			logger.Info("标记文件已删除，尝试直接打开数据库")
		}

		// 尝试直接打开数据库
		db, openErr := badgerdb.Open(opts)
		if openErr == nil {
			// 成功打开！说明数据库实际上是正常的，只是标记文件没删除
			logger.Info("✅ 数据库打开成功，上次关闭虽然不正常但数据完整")
			// 创建新的运行标记
			if err := os.WriteFile(markerPath, []byte("1"), 0600); err != nil {
				logger.Warnf("无法创建运行标记文件: %v", err)
			}
			return db, nil
		}

		// 直接打开失败，说明确实需要修复
		logger.Warnf("直接打开失败: %v，开始执行修复流程...", openErr)

		// 创建临时Store实例用于修复
		// 创建临时配置用于恢复
		tempConfig := badgerconfig.New(nil)
		tempStore := &Store{
			logger: logger,
			config: tempConfig,
		}

		// 首先尝试自动修复
		if repairErr := tempStore.TryRepair(dataDir); repairErr != nil {
			logger.Errorf("自动修复失败: %v", repairErr)

			// 修复失败，检查是否有可用备份
			backupDir := filepath.Join(dataDir, "backups")
			if latestBackup := findLatestBackup(backupDir); latestBackup != "" {
				logger.Warnf("⚠️ 警告：即将从备份恢复，这将丢失备份时间点之后的所有数据！")
				logger.Infof("发现可用备份，尝试恢复: %s", latestBackup)

				// 备份当前损坏的数据
				recoveryDir := getRecoveryDir(dataDir)
				corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
				if err := backupCorruptedData(dataDir, corruptedBackupDir, logger); err != nil {
					logger.Warnf("备份损坏数据失败: %v", err)
				}

				// 从备份恢复
				if restoreErr := tempStore.RestoreFromBackup(context.Background(), latestBackup, dataDir); restoreErr != nil {
					logger.Errorf("从备份恢复失败: %v", restoreErr)
					return nil, fmt.Errorf("数据库损坏且恢复失败: 修复错误=%v, 恢复错误=%v", repairErr, restoreErr)
				}

				logger.Info("从备份恢复成功")
			} else {
				// 没有备份，尝试强制修复
				logger.Warn("没有可用备份，尝试强制修复（可能丢失数据）")
				if forceErr := forceRepairDatabase(dataDir, opts, logger); forceErr != nil {
					// 如果强制修复也失败，删除整个数据库目录，让系统重新创建
					logger.Warnf("⚠️ 警告：强制修复失败，将删除损坏的数据库并重新创建（所有数据将丢失）")
					logger.Warnf("损坏的数据库路径: %s", dataDir)

					// 备份损坏的数据库到恢复目录
					recoveryDir := getRecoveryDir(dataDir)
					corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
					if backupErr := backupCorruptedData(dataDir, corruptedBackupDir, logger); backupErr != nil {
						logger.Warnf("备份损坏数据失败: %v", backupErr)
					} else {
						logger.Infof("已备份损坏的数据库到: %s", corruptedBackupDir)
					}

					// 删除损坏的数据库目录
					if rmErr := os.RemoveAll(dataDir); rmErr != nil {
						return nil, fmt.Errorf("无法删除损坏的数据库目录: %w", rmErr)
					}

					// 重新创建数据库目录
					if mkErr := os.MkdirAll(dataDir, 0700); mkErr != nil {
						return nil, fmt.Errorf("无法创建新的数据库目录: %w", mkErr)
					}

					logger.Info("已删除损坏的数据库，将重新创建")
				}
			}
		} else {
			logger.Info("数据库自动修复成功")
		}
	}

	// 创建运行标记
	if err := os.WriteFile(markerPath, []byte("1"), 0600); err != nil {
		logger.Warn("无法创建数据库运行标记")
	}

	// 尝试打开数据库
	db, err := badgerdb.Open(opts)
	if err != nil {
		// 如果还是失败，进行最后的修复尝试
		logger.Errorf("常规打开失败，进行最后修复尝试: %v", err)

		if lastErr := forceRepairDatabase(dataDir, opts, logger); lastErr != nil {
			// 强制修复失败，先检查是否有可用备份
			backupDir := filepath.Join(dataDir, "backups")
			if latestBackup := findLatestBackup(backupDir); latestBackup != "" {
				logger.Warnf("⚠️ 强制修复失败，发现可用备份，尝试从备份恢复")
				logger.Infof("备份文件: %s", latestBackup)

				// 备份当前损坏的数据
				recoveryDir := getRecoveryDir(dataDir)
				corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
				if backupErr := backupCorruptedData(dataDir, corruptedBackupDir, logger); backupErr != nil {
					logger.Warnf("备份损坏数据失败: %v", backupErr)
				} else {
					logger.Infof("已备份损坏的数据库到: %s", corruptedBackupDir)
				}

				// 创建临时Store实例用于恢复
				tempConfig := badgerconfig.New(nil)
				tempStore := &Store{
					logger: logger,
					config: tempConfig,
				}

				// 从备份恢复
				if restoreErr := tempStore.RestoreFromBackup(context.Background(), latestBackup, dataDir); restoreErr != nil {
					logger.Errorf("从备份恢复失败: %v", restoreErr)
					return nil, fmt.Errorf("数据库损坏且恢复失败: 修复错误=%v, 恢复错误=%v", lastErr, restoreErr)
				}

				logger.Info("从备份恢复成功，重新尝试打开数据库")
			} else {
				// 没有可用备份，删除数据库重新创建
				logger.Warnf("⚠️ 警告：强制修复失败且无可用备份，将删除数据库并重新创建（所有数据将丢失）")
				logger.Warnf("损坏的数据库路径: %s", dataDir)

				// 备份损坏的数据库
				recoveryDir := getRecoveryDir(dataDir)
				corruptedBackupDir := filepath.Join(recoveryDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
				if backupErr := backupCorruptedData(dataDir, corruptedBackupDir, logger); backupErr != nil {
					logger.Warnf("备份损坏数据失败: %v", backupErr)
				} else {
					logger.Infof("已备份损坏的数据库到: %s", corruptedBackupDir)
				}

				// 删除损坏的数据库目录
				if rmErr := os.RemoveAll(dataDir); rmErr != nil {
					return nil, fmt.Errorf("无法删除损坏的数据库目录: %w", rmErr)
				}

				// 重新创建数据库目录
				if mkErr := os.MkdirAll(dataDir, 0700); mkErr != nil {
					return nil, fmt.Errorf("无法创建新的数据库目录: %w", mkErr)
				}

				logger.Info("已删除损坏的数据库，正在重新创建...")
			}
		}

		// 再次尝试打开（可能是从备份恢复后的数据库，或全新的数据库）
		db, err = badgerdb.Open(opts)
		if err != nil {
			return nil, fmt.Errorf("最终打开数据库失败: %w", err)
		}

		logger.Info("数据库成功打开")
	}

	// 成功打开后，清理 BADGER_FATAL（如果存在）
	if _, ferr := os.Stat(fatalMarkerPath); ferr == nil {
		if rmErr := os.Remove(fatalMarkerPath); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warnf("无法删除 BADGER_FATAL 标记文件: %v", rmErr)
		} else {
			logger.Info("已清理 BADGER_FATAL 标记文件")
		}
	}

	return db, nil
}

// tempBadgerConfig 临时配置，用于修复过程
type tempBadgerConfig struct {
	path string
}

func (c *tempBadgerConfig) GetPath() string               { return c.path }
func (c *tempBadgerConfig) GetValueLogFileSize() int64    { return 67108864 }
func (c *tempBadgerConfig) GetValueThreshold() int64      { return 128 }
func (c *tempBadgerConfig) IsSyncWritesEnabled() bool     { return true }
func (c *tempBadgerConfig) IsAutoCompactionEnabled() bool { return false }

// getRecoveryDir 获取恢复备份目录的统一路径
// 所有恢复相关的备份（corrupted_backup_*、existing_backup_*）都统一放在 recovery/ 子目录下
func getRecoveryDir(dataDir string) string {
	return filepath.Join(dataDir, "recovery")
}

// backupCorruptedData 备份损坏的数据
func backupCorruptedData(sourceDir, backupDir string, logger log.Logger) error {
	logger.Infof("备份损坏数据到: %s", backupDir)

	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 列出源目录中的所有文件
	files, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		sourcePath := filepath.Join(sourceDir, file.Name())
		backupPath := filepath.Join(backupDir, file.Name())

		// 复制文件
		if err := copyFile(sourcePath, backupPath, logger); err != nil {
			logger.Warnf("复制文件失败 %s: %v", file.Name(), err)
		}
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string, logger log.Logger) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			if logger != nil {
				logger.Warnf("关闭源文件失败: %v", err)
			}
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			if logger != nil {
				logger.Warnf("关闭目标文件失败: %v", err)
			}
		}
	}()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// forceRepairDatabase 强制修复数据库
func forceRepairDatabase(dataDir string, opts badgerdb.Options, logger log.Logger) error {
	logger.Warn("开始强制修复数据库（可能丢失部分数据）")

	// 1. 删除可能损坏的文件
	corruptedFiles := []string{"LOCK", "DISCARD"}
	for _, file := range corruptedFiles {
		filePath := filepath.Join(dataDir, file)
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				logger.Warnf("删除文件失败 %s: %v", file, err)
			} else if err == nil {
				logger.Infof("删除了可能损坏的文件: %s", file)
			}
		}
	}

	// 2. 尝试截断值日志文件
	vlogFiles, err := filepath.Glob(filepath.Join(dataDir, "*.vlog"))
	if err == nil {
		for _, vlogFile := range vlogFiles {
			if err := truncateCorruptedVLog(vlogFile, logger); err != nil {
				logger.Warnf("截断值日志文件失败 %s: %v", vlogFile, err)
			}
		}
	}

	// 3. 尝试以检测模式打开，让BadgerDB自动处理损坏
	repairOpts := opts
	repairOpts.DetectConflicts = false // 禁用冲突检测，提高容错性
	repairOpts.CompactL0OnClose = true // 关闭时压缩L0层

	db, err := badgerdb.Open(repairOpts)
	if err != nil {
		return fmt.Errorf("修复模式打开失败: %w", err)
	}

	// 尝试运行垃圾回收来清理可能的损坏数据
	if gcErr := db.RunValueLogGC(0.1); gcErr != nil && gcErr != badgerdb.ErrNoRewrite {
		logger.Warnf("修复过程中垃圾回收失败: %v", gcErr)
	}

	// 立即关闭，这会触发必要的修复和压缩
	db.Close()

	logger.Info("强制修复完成")
	return nil
}

// truncateCorruptedVLog 截断损坏的值日志文件
func truncateCorruptedVLog(vlogPath string, logger log.Logger) error {
	file, err := os.OpenFile(vlogPath, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warnf("关闭文件失败: %v", err)
		}
	}()

	// 获取文件信息
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// 如果文件很小，可能不需要截断
	if info.Size() < 1024 {
		return nil
	}

	// 尝试找到有效的结束位置
	// 这是一个简化的实现，实际应该解析BadgerDB的文件格式
	validSize := findValidVLogSize(file, logger)

	if validSize > 0 && validSize < info.Size() {
		logger.Infof("截断值日志文件 %s: %d -> %d", vlogPath, info.Size(), validSize)
		return file.Truncate(validSize)
	}

	return nil
}

// findValidVLogSize 找到值日志文件的有效大小
func findValidVLogSize(file *os.File, logger log.Logger) int64 {
	// 这是一个简化的实现
	// 实际应该解析BadgerDB的值日志格式来找到有效的结束位置

	info, err := file.Stat()
	if err != nil {
		return 0
	}

	// 简单策略：如果文件很大但开头很小，可能是写入中断
	// 尝试保留前面的有效部分
	if info.Size() > 1024*1024 { // 1MB
		// 读取文件开头检查
		buffer := make([]byte, 1024)
		n, err := file.ReadAt(buffer, 0)
		if err != nil || n == 0 {
			return 0
		}

		// 如果开头有数据，尝试保留前面的部分
		// 这里使用一个保守的策略
		return min(info.Size()/2, 1024*1024) // 保留一半或1MB，取较小值
	}

	return 0
}

// min 返回两个int64中的较小值
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// badgerLogger 实现BadgerDB的日志接口
type badgerLogger struct {
	logger log.Logger
	dataDir string
}

// newBadgerLogger 创建BadgerDB日志适配器
func newBadgerLogger(logger log.Logger, dataDir string) *badgerLogger {
	return &badgerLogger{logger: logger, dataDir: dataDir}
}

// Errorf 输出错误日志
func (l *badgerLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf("[BadgerDB] "+format, args...)

	// 🆕 彻底修复：捕获 Badger 关键致命前兆，写入 BADGER_FATAL 标记，确保下次启动强制走修复流程
	// 典型前兆：
	// - while deleting file: ... .mem ... no such file or directory
	// - Assert failed（Badger 内部 fatal 可能直接走 stderr；这里尽量提前标记）
	if strings.Contains(format, "while deleting file") || strings.Contains(format, "Assert failed") {
		if strings.TrimSpace(l.dataDir) != "" {
			_ = os.WriteFile(filepath.Join(l.dataDir, "BADGER_FATAL"), []byte(time.Now().Format(time.RFC3339Nano)), 0600)
		}
	}
}

// Warningf 输出警告日志
func (l *badgerLogger) Warningf(format string, args ...interface{}) {
	l.logger.Warnf("[BadgerDB] "+format, args...)
}

// Infof 输出信息日志
func (l *badgerLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof("[BadgerDB] "+format, args...)
}

// Debugf 输出调试日志
func (l *badgerLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf("[BadgerDB] "+format, args...)
}

// logBadgerVlogInfo 记录BadgerDB vlog文件信息（用于内存分析）
func (s *Store) logBadgerVlogInfo(dataDir string, logger log.Logger) {
	vlogFiles, err := filepath.Glob(filepath.Join(dataDir, "*.vlog"))
	if err != nil {
		return
	}

	totalSize := int64(0)
	fileInfo := make([]string, 0, len(vlogFiles))
	for _, vlogFile := range vlogFiles {
		if info, err := os.Stat(vlogFile); err == nil {
			size := info.Size()
			totalSize += size
			fileInfo = append(fileInfo, fmt.Sprintf("%s(%.2fMB)", filepath.Base(vlogFile), float64(size)/(1024*1024)))
		}
	}

	// 转换为MB
	totalSizeMB := float64(totalSize) / (1024 * 1024)
	
	// 🆕 获取数据库统计信息（用于分析内存使用）
	var dbSizeMB float64
	if s.db != nil {
		lsmSize, vlogSize := s.db.Size()
		dbSize := lsmSize + vlogSize
		dbSizeMB = float64(dbSize) / (1024 * 1024)
	}
	
	if logger != nil {
		if dbSizeMB > 0 {
			logger.Infof("📊 [BadgerDB启动] vlog文件统计: 数量=%d, 总大小=%.2fMB, 文件列表=[%s], DB总大小=%.2fMB",
				len(vlogFiles), totalSizeMB, strings.Join(fileInfo, ", "), dbSizeMB)
		} else {
			logger.Infof("📊 [BadgerDB启动] vlog文件统计: 数量=%d, 总大小=%.2fMB, 文件列表=[%s]",
				len(vlogFiles), totalSizeMB, strings.Join(fileInfo, ", "))
		}
	}
}
