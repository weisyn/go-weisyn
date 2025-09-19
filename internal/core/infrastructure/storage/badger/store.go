// Package badger 提供基于BadgerDB的存储实现
package badger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	log "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	interfaces "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/utils"
)

// Store 实现BadgerStore接口
type Store struct {
	db         *badgerdb.DB
	config     *badgerconfig.Config
	logger     log.Logger
	cancelFunc context.CancelFunc // 用于取消后台任务的函数
}

// New 创建新的BadgerStore实例
// 初始化数据库并启动维护任务
func New(config *badgerconfig.Config, logger log.Logger) interfaces.BadgerStore {
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

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Errorf("无法创建BadgerDB数据目录: %v", err)
		return nil
	}

	// 创建BadgerDB配置
	opts := badgerdb.DefaultOptions(dataDir)
	// 使用简化配置
	opts.SyncWrites = config.IsSyncWritesEnabled()
	opts.MemTableSize = config.GetMemTableSize()

	// 设置表现参数
	opts.NumCompactors = 2            // 后台整理工作线程数
	opts.NumLevelZeroTables = 5       // Level 0最大表数
	opts.NumLevelZeroTablesStall = 10 // Level 0表数触发压缩的阈值

	// 设置日志
	opts.Logger = newBadgerLogger(logger)

	// 声明数据库变量
	var db *badgerdb.DB

	// 检查是否强制使用内存模式
	if os.Getenv("WES_MEMORY_ONLY_MODE") == "true" {
		logger.Infof("🧠 检测到内存数据库模式标志，直接启用内存BadgerDB")
		fmt.Printf("🧠 正在启动内存数据库模式...\n")

		// 直接创建内存数据库
		memOpts := badgerdb.DefaultOptions("")
		memOpts = memOpts.WithInMemory(true)
		memOpts.Logger = newBadgerLogger(logger)
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
			logger.Errorf("无法打开BadgerDB: %v", err)

			// 🚨 显示明显的控制台警告
			fmt.Printf("\n")
			fmt.Printf("⚠️  ============ 重要警告 ============\n")
			fmt.Printf("❌ BadgerDB磁盘数据库打开失败\n")
			fmt.Printf("🔄 系统正在回退到内存数据库模式\n")
			fmt.Printf("📝 影响说明:\n")
			fmt.Printf("   • 所有数据仅存储在内存中\n")
			fmt.Printf("   • 程序退出后数据将丢失\n")
			fmt.Printf("   • 系统将创建新的Genesis区块\n")
			fmt.Printf("🛠️  建议操作:\n")
			fmt.Printf("   • 检查数据目录权限: %s\n", dataDir)
			fmt.Printf("   • 或使用 --memory-only 显式启用内存模式\n")
			fmt.Printf("=====================================\n")
			fmt.Printf("\n")

			// 记录详细的回退信息
			logger.Warnf("BadgerDB回退详情: 数据目录=%s, 错误=%v", dataDir, err)
			logger.Warn("⚠️ 回退到内存BadgerDB（数据不持久化，程序退出后丢失）")

			// 以内存模式回退，确保系统可启动（仍然是 BadgerStore 接口实例）
			memOpts := badgerdb.DefaultOptions("")
			memOpts = memOpts.WithInMemory(true)
			memOpts.Logger = newBadgerLogger(logger)
			memDB, memErr := badgerdb.Open(memOpts)
			if memErr != nil {
				logger.Errorf("无法打开内存BadgerDB: %v", memErr)
				fmt.Printf("❌ 严重错误: 内存数据库也无法启动: %v\n", memErr)
				return nil
			}
			db = memDB

			// 记录成功回退信息
			logger.Infof("✅ 内存BadgerDB启动成功（临时模式）")
			fmt.Printf("✅ 内存数据库模式已启用\n\n")
		}
	}

	// 设置数据库实例
	store.db = db

	// 启动维护例程
	ctx, cancel := context.WithCancel(context.Background())
	store.cancelFunc = cancel
	store.StartMaintenanceRoutines(ctx)

	// 如果启用自动压缩，设置备份目录并启动自动备份
	if config.IsAutoCompactionEnabled() {
		// 备份目录配置
		backupDir := filepath.Join(dataDir, "backups")
		// 确保备份目录存在
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			logger.Warnf("无法创建备份目录: %v", err)
		} else {
			store.StartAutomaticBackups(ctx, backupDir, 1*time.Hour, 24) // 每小时备份，保留24个（1天）
		}
	}

	logger.Info("BadgerDB存储初始化完成")
	return store
}

// Close 关闭存储并释放资源
func (s *Store) Close() error {
	s.logger.Info("🔧 开始关闭BadgerDB存储...")

	// 取消所有后台任务
	s.logger.Info("🔧 取消后台任务...")
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.logger.Info("🔧 后台任务已取消")
	}

	// 删除运行标记
	s.logger.Info("🔧 删除运行标记...")
	markerPath := filepath.Join(s.config.GetPath(), "BADGER_RUNNING")
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		s.logger.Warnf("无法删除数据库运行标记: %v", err)
	} else {
		s.logger.Info("🔧 运行标记已删除")
	}

	if s.db == nil {
		s.logger.Info("🔧 数据库连接为空，无需关闭")
		return nil
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

	s.logger.Info("🔧 BadgerDB存储已安全关闭")
	return nil
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
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Set(key, value)
	})
}

// SetWithTTL 设置键值对并指定过期时间
func (s *Store) SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error {
	return s.db.Update(func(txn *badgerdb.Txn) error {
		entry := badgerdb.NewEntry(key, value).WithTTL(ttl)
		return txn.SetEntry(entry)
	})
}

// Delete 删除指定键的值
func (s *Store) Delete(ctx context.Context, key []byte) error {
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
		defer it.Close()

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
		defer it.Close()

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
	// 创建BadgerDB事务
	txn := s.db.NewTransaction(true)

	// 创建我们的事务包装
	tx := &Transaction{
		txn:   txn,
		state: int32(TxActive),
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
	// 检查是否存在未完成标记
	markerPath := filepath.Join(dataDir, "BADGER_RUNNING")
	_, err := os.Stat(markerPath)

	if err == nil {
		// 存在标记，可能是异常关闭
		logger.Warn("检测到数据库可能未正常关闭，尝试修复...")

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

			// 修复失败，先尝试创建缺失的vlog文件，然后再考虑备份恢复
			logger.Warn("尝试创建缺失的000000.vlog文件...")
			if createErr := createMissingVLogFile(dataDir, logger); createErr == nil {
				logger.Info("成功创建缺失的vlog文件，重新尝试打开数据库")
				// 重新尝试打开数据库
				if retryDB, retryErr := badgerdb.Open(opts); retryErr == nil {
					retryDB.Close()
					logger.Info("数据库修复成功，继续正常启动")
					return safeOpenDB(dataDir, opts, logger) // 递归重试
				}
			}

			// 如果创建vlog文件也失败，检查是否有可用备份
			backupDir := filepath.Join(dataDir, "backups")
			if latestBackup := findLatestBackup(backupDir); latestBackup != "" {
				logger.Warnf("⚠️ 警告：即将从备份恢复，这将丢失备份时间点之后的所有数据！")
				logger.Infof("发现可用备份，尝试恢复: %s", latestBackup)

				// 备份当前损坏的数据
				corruptedBackupDir := filepath.Join(dataDir, "corrupted_backup_"+time.Now().Format("20060102_150405"))
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
					return nil, fmt.Errorf("数据库损坏且无法修复: %w", forceErr)
				}
			}
		} else {
			logger.Info("数据库自动修复成功")
		}
	}

	// 创建运行标记
	if err := os.WriteFile(markerPath, []byte("1"), 0644); err != nil {
		logger.Warn("无法创建数据库运行标记")
	}

	// 尝试打开数据库
	db, err := badgerdb.Open(opts)
	if err != nil {
		// 如果还是失败，进行最后的修复尝试
		logger.Errorf("常规打开失败，进行最后修复尝试: %v", err)

		if lastErr := forceRepairDatabase(dataDir, opts, logger); lastErr != nil {
			return nil, fmt.Errorf("打开数据库失败，所有修复尝试都失败: 原始错误=%v, 修复错误=%v", err, lastErr)
		}

		// 再次尝试打开
		db, err = badgerdb.Open(opts)
		if err != nil {
			return nil, fmt.Errorf("修复后仍无法打开数据库: %w", err)
		}

		logger.Info("强制修复后数据库打开成功")
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

// backupCorruptedData 备份损坏的数据
func backupCorruptedData(sourceDir, backupDir string, logger log.Logger) error {
	logger.Infof("备份损坏数据到: %s", backupDir)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
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
		if err := copyFile(sourcePath, backupPath); err != nil {
			logger.Warnf("复制文件失败 %s: %v", file.Name(), err)
		}
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

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
			if err := os.Remove(filePath); err != nil {
				logger.Warnf("删除文件失败 %s: %v", file, err)
			} else {
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
	file, err := os.OpenFile(vlogPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

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

// createMissingVLogFile 创建缺失的000000.vlog文件
// 当数据库文件损坏缺少vlog文件时，创建一个空的vlog文件
func createMissingVLogFile(dataDir string, logger log.Logger) error {
	vlogPath := filepath.Join(dataDir, "000000.vlog")

	// 检查文件是否已存在
	if _, err := os.Stat(vlogPath); err == nil {
		logger.Info("000000.vlog文件已存在，无需创建")
		return nil
	}

	logger.Infof("正在创建缺失的vlog文件: %s", vlogPath)

	// 创建一个空的vlog文件
	// BadgerDB的vlog文件有特定的头部结构
	file, err := os.Create(vlogPath)
	if err != nil {
		return fmt.Errorf("创建vlog文件失败: %w", err)
	}
	defer file.Close()

	// 写入vlog文件的基本头部
	// 这是根据BadgerDB v3的文件格式
	// 空的vlog文件需要有正确的头部标识
	vlogHeader := make([]byte, 32) // BadgerDB vlog头部通常是32字节
	// 设置魔数和版本号（简化实现）
	copy(vlogHeader[0:4], []byte{0xCA, 0xFE, 0xBA, 0xBE}) // 魔数
	vlogHeader[4] = 3                                     // BadgerDB版本3

	if _, err := file.Write(vlogHeader); err != nil {
		return fmt.Errorf("写入vlog头部失败: %w", err)
	}

	// 确保文件被写入到磁盘
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步vlog文件失败: %w", err)
	}

	logger.Info("成功创建空的000000.vlog文件")
	return nil
}

// badgerLogger 实现BadgerDB的日志接口
type badgerLogger struct {
	logger log.Logger
}

// newBadgerLogger 创建BadgerDB日志适配器
func newBadgerLogger(logger log.Logger) *badgerLogger {
	return &badgerLogger{logger: logger}
}

// Errorf 输出错误日志
func (l *badgerLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf("[BadgerDB] "+format, args...)
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
