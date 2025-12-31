package badger

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	
	"github.com/weisyn/v1/internal/core/diagnostics"
)

// BackupStatus 备份状态
type BackupStatus string

const (
	// BackupStatusCompleted 表示备份已成功完成
	BackupStatusCompleted BackupStatus = "completed"
	// BackupStatusFailed 表示备份失败
	BackupStatusFailed BackupStatus = "failed"
	// BackupStatusVerified 表示备份已验证
	BackupStatusVerified BackupStatus = "verified"
)

// BackupType 备份类型
type BackupType string

const (
	// BackupTypeAutomatic 表示自动备份
	BackupTypeAutomatic BackupType = "automatic"
	// BackupTypeManual 表示手动备份
	BackupTypeManual BackupType = "manual"
	// BackupTypePreUpdate 表示更新前备份
	BackupTypePreUpdate BackupType = "pre_update"
)

// BackupMetadata 备份元数据
// 包含备份的关键信息，用于验证和恢复
type BackupMetadata struct {
	Timestamp     time.Time    `json:"timestamp"`      // 备份创建时间
	Size          int64        `json:"size"`           // 备份文件大小
	KeyCount      int          `json:"key_count"`      // 键的数量
	DBVersion     string       `json:"db_version"`     // 数据库版本
	AppVersion    string       `json:"app_version"`    // 应用程序版本
	MachineName   string       `json:"machine_name"`   // 机器名称
	BackupReason  string       `json:"backup_reason"`  // 备份原因
	BackupType    BackupType   `json:"backup_type"`    // 备份类型
	Status        BackupStatus `json:"status"`         // 备份状态
	Hash          string       `json:"hash,omitempty"` // 备份文件的哈希值(可选)
	FormatVersion int          `json:"format_version"` // 备份格式版本
}

// backupManager 管理备份操作
type backupManager struct {
	store     *Store
	backupDir string
	mutex     sync.Mutex
	logger    log.Logger
}

// newBackupManager 创建新的备份管理器
func newBackupManager(store *Store, backupDir string) *backupManager {
	return &backupManager{
		store:     store,
		backupDir: backupDir,
		logger:    store.logger,
	}
}

// CreateBackup 创建数据库备份
// 将数据库内容保存到指定路径，并创建元数据文件
func (s *Store) CreateBackup(ctx context.Context, destPath string) error {
	s.logger.Infof("创建备份到: %s", destPath)

	// 确保备份目录存在
	backupDir := filepath.Dir(destPath)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 创建临时备份文件
	tempPath := destPath + ".tmp"
	backupFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("创建临时备份文件失败: %w", err)
	}

	// 准备元数据
	metadata := BackupMetadata{
		Timestamp:     time.Now(),
		DBVersion:     "3",             // BadgerDB v3版本
		AppVersion:    getAppVersion(), // 从配置或环境获取
		MachineName:   getHostname(),
		BackupReason:  "定期备份",
		BackupType:    BackupTypeAutomatic,
		Status:        BackupStatusCompleted,
		FormatVersion: 1, // 当前备份格式版本
	}

	// 获取数据库统计信息
	count := 0
	err = s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close() // Badger Iterator.Close() 无返回值

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})

	if err != nil {
		backupFile.Close()
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Warnf("获取键数量失败: %v", err)
		return fmt.Errorf("获取键数量失败: %w", err)
	}

	metadata.KeyCount = count

	// 执行备份
	s.logger.Info("开始执行数据备份...")
	startTime := time.Now()

	// 🔧 优化：使用带缓冲的写入，减少系统调用，提升性能
	bufferedWriter := bufio.NewWriterSize(backupFile, 2*1024*1024) // 2MB缓冲区
	_, err = s.db.Backup(bufferedWriter, 0)
	if err != nil {
		bufferedWriter.Flush()
		backupFile.Close()
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("执行备份失败: %v", err)
		return fmt.Errorf("执行备份失败: %w", err)
	}

	// 刷新缓冲区
	if err := bufferedWriter.Flush(); err != nil {
		backupFile.Close()
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("刷新备份缓冲区失败: %v", err)
		return fmt.Errorf("刷新备份缓冲区失败: %w", err)
	}

	backupDuration := time.Since(startTime)

	// 关闭文件以确保写入完成
	if err := backupFile.Close(); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("关闭备份文件失败: %v", err)
		return fmt.Errorf("关闭备份文件失败: %w", err)
	}

	// 🔧 优化：备份完成后立即触发GC和内存释放，加速内存回收
	s.logger.Info("备份完成，开始释放内存...")

	// 记录备份前的内存状态
	var beforeGC runtime.MemStats
	runtime.ReadMemStats(&beforeGC)
	beforeRSS := getRSSBytesForBackup()

	// 执行GC和内存释放
	runtime.GC()
	runtime.GC()         // 执行两次GC确保充分回收
	debug.FreeOSMemory() // 将内存返还给操作系统

	// 等待GC完成
	time.Sleep(100 * time.Millisecond)

	// 记录备份后的内存状态
	var afterGC runtime.MemStats
	runtime.ReadMemStats(&afterGC)
	afterRSS := getRSSBytesForBackup()

	freedHeapMB := int64(beforeGC.HeapAlloc-afterGC.HeapAlloc) / 1024 / 1024
	freedRSSMB := int64(beforeRSS-afterRSS) / 1024 / 1024

	if freedHeapMB > 0 || freedRSSMB > 0 {
		s.logger.Infof("备份后内存释放: HeapAlloc %dMB → %dMB (释放 %dMB), RSS %dMB → %dMB (释放 %dMB)",
			beforeGC.HeapAlloc/1024/1024, afterGC.HeapAlloc/1024/1024, freedHeapMB,
			beforeRSS/1024/1024, afterRSS/1024/1024, freedRSSMB)
	} else {
		s.logger.Debugf("备份后内存状态: HeapAlloc %dMB, RSS %dMB",
			afterGC.HeapAlloc/1024/1024, afterRSS/1024/1024)
	}

	// 计算备份文件哈希
	hash, err := calculateFileHash(tempPath)
	if err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Warnf("计算备份文件哈希失败: %v", err)
	} else {
		metadata.Hash = hash
	}

	// 获取文件大小
	fileInfo, err := os.Stat(tempPath)
	if err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("获取备份文件信息失败: %v", err)
		return fmt.Errorf("获取备份文件信息失败: %w", err)
	}
	metadata.Size = fileInfo.Size()

	// 验证备份文件
	if err := verifyBackupFile(tempPath); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("备份验证失败: %v", err)
		return fmt.Errorf("备份验证失败: %w", err)
	}

	metadata.Status = BackupStatusVerified

	// 重命名临时文件为目标文件
	if err := os.Rename(tempPath, destPath); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warnf("删除临时备份文件失败: %v", removeErr)
		}
		s.logger.Errorf("重命名备份文件失败: %v", err)
		return fmt.Errorf("重命名备份文件失败: %w", err)
	}

	// 保存元数据到文件
	metadataPath := destPath + ".meta"
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		s.logger.Warnf("序列化备份元数据失败: %v", err)
	} else {
		if err := os.WriteFile(metadataPath, metadataJSON, 0600); err != nil {
			s.logger.Warnf("写入备份元数据失败: %v", err)
		}
	}

	s.logger.Infof("数据库备份成功: %s (大小: %d 字节, 键数量: %d, 耗时: %v)",
		destPath, metadata.Size, metadata.KeyCount, backupDuration)
	return nil
}

// StartAutomaticBackups 启动自动备份
// 根据指定的时间间隔定期备份数据库，并保留指定数量的备份
func (s *Store) StartAutomaticBackups(ctx context.Context, backupDir string, interval time.Duration, keepCount int) {
	s.logger.Infof("启动自动备份任务，间隔：%v，保留数量：%d", interval, keepCount)

	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		s.logger.Errorf("创建备份目录失败: %v", err)
		return
	}

	// 创建备份管理器
	manager := newBackupManager(s, backupDir)

	// 启动定期备份任务
	go func() {
		// 首次备份延迟1分钟，避免启动时立即执行
		initialDelay := time.NewTimer(1 * time.Minute)

		select {
		case <-initialDelay.C:
			// 执行首次备份
			if err := manager.performBackup(BackupTypeAutomatic, "启动后首次自动备份"); err != nil {
				s.logger.Errorf("首次自动备份失败: %v", err)
			}
		case <-ctx.Done():
			initialDelay.Stop()
			return
		}

		// 设置定期备份
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := manager.performBackup(BackupTypeAutomatic, "定期自动备份"); err != nil {
					s.logger.Errorf("自动备份失败: %v", err)
				}

				// 清理旧备份
				if err := manager.cleanOldBackups(keepCount); err != nil {
					s.logger.Errorf("清理旧备份失败: %v", err)
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// performBackup 执行备份操作
func (bm *backupManager) performBackup(backupType BackupType, reason string) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("badger_backup_%s_%s.bak", timestamp, string(backupType))
	backupPath := filepath.Join(bm.backupDir, backupName)

	bm.logger.Infof("执行%s备份: %s", backupType, backupPath)

	// 创建备份
	return bm.store.CreateBackup(context.Background(), backupPath)
}

// cleanOldBackups 清理旧备份，只保留指定数量的最新备份
func (bm *backupManager) cleanOldBackups(keepCount int) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	bm.logger.Infof("清理旧备份，保留最新的%d个备份", keepCount)

	// 获取所有备份文件
	backups, err := bm.listBackupFiles()
	if err != nil {
		return fmt.Errorf("列出备份文件失败: %w", err)
	}

	// 如果备份数量不超过保留数量，不需要清理
	if len(backups) <= keepCount {
		bm.logger.Infof("当前备份数量(%d)不超过保留数量(%d)，无需清理", len(backups), keepCount)
		return nil
	}

	// 需要删除的备份数量
	deleteCount := len(backups) - keepCount
	bm.logger.Infof("将删除%d个旧备份", deleteCount)

	// 删除旧备份
	for i := 0; i < deleteCount; i++ {
		backupPath := backups[i]
		metadataPath := backupPath + ".meta"

		// 删除备份文件
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			bm.logger.Warnf("删除旧备份文件失败: %s, %v", backupPath, err)
		}

		// 删除元数据文件
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			bm.logger.Warnf("删除旧备份元数据文件失败: %s, %v", metadataPath, err)
		}

		bm.logger.Infof("已删除旧备份: %s", backupPath)
	}

	return nil
}

// listBackupFiles 列出所有备份文件，按时间戳排序（从旧到新）
func (bm *backupManager) listBackupFiles() ([]string, error) {
	// 读取备份目录
	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	// 过滤出备份文件
	var backups []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "badger_backup_") &&
			strings.HasSuffix(file.Name(), ".bak") {
			backups = append(backups, filepath.Join(bm.backupDir, file.Name()))
		}
	}

	// 按文件名排序（含时间戳，所以这实际上是按时间排序）
	sort.Strings(backups)

	return backups, nil
}

// ListBackups 列出所有可用的备份
func (s *Store) ListBackups(backupDir string) ([]BackupMetadata, error) {
	s.logger.Infof("列出备份目录中的所有备份: %s", backupDir)

	// 创建备份管理器
	manager := newBackupManager(s, backupDir)

	// 获取所有备份文件
	backupFiles, err := manager.listBackupFiles()
	if err != nil {
		return nil, fmt.Errorf("获取备份文件列表失败: %w", err)
	}

	// 读取元数据
	var backups []BackupMetadata
	for _, backupFile := range backupFiles {
		metadataPath := backupFile + ".meta"

		// 检查元数据文件是否存在
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			// 元数据文件不存在，尝试创建一个基本的元数据
			fileInfo, err := os.Stat(backupFile)
			if err != nil {
				s.logger.Warnf("无法获取备份文件信息: %s, %v", backupFile, err)
				continue
			}

			// 从文件名中提取时间戳
			fileName := filepath.Base(backupFile)
			timestamp, backupType := extractBackupInfo(fileName)

			metadata := BackupMetadata{
				Timestamp:     timestamp,
				Size:          fileInfo.Size(),
				BackupType:    backupType,
				Status:        BackupStatusCompleted,
				FormatVersion: 1,
			}

			backups = append(backups, metadata)
		} else {
			// 读取元数据文件
			data, err := os.ReadFile(metadataPath)
			if err != nil {
				s.logger.Warnf("读取备份元数据失败: %s, %v", metadataPath, err)
				continue
			}

			var metadata BackupMetadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				s.logger.Warnf("解析备份元数据失败: %s, %v", metadataPath, err)
				continue
			}

			backups = append(backups, metadata)
		}
	}

	// 按时间戳排序，最新的在前
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// CreateManualBackup 创建手动备份
func (s *Store) CreateManualBackup(ctx context.Context, backupDir, reason string) (string, error) {
	s.logger.Infof("创建手动备份，原因: %s", reason)

	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 设置备份名称
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("badger_backup_%s_manual.bak", timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// 创建备份
	if err := s.CreateBackup(ctx, backupPath); err != nil {
		return "", fmt.Errorf("创建手动备份失败: %w", err)
	}

	// 更新元数据
	metadataPath := backupPath + ".meta"
	if _, err := os.Stat(metadataPath); err == nil {
		data, err := os.ReadFile(metadataPath)
		if err == nil {
			var metadata BackupMetadata
			if err := json.Unmarshal(data, &metadata); err == nil {
				metadata.BackupType = BackupTypeManual
				metadata.BackupReason = reason

				// 写回更新后的元数据
				if updatedData, err := json.MarshalIndent(metadata, "", "  "); err == nil {
					if err := os.WriteFile(metadataPath, updatedData, 0600); err != nil {
						s.logger.Warnf("更新备份元数据失败: %v", err)
					}
				} else {
					s.logger.Warnf("序列化备份元数据失败: %v", err)
				}
			} else {
				s.logger.Warnf("解析备份元数据失败: %v", err)
			}
		} else {
			s.logger.Warnf("读取备份元数据失败: %v", err)
		}
	}

	return backupPath, nil
}

// getAppVersion 获取应用程序版本
func getAppVersion() string {
	// 这里应该从应用配置或环境变量中获取实际版本
	return "1.0.0"
}

// getHostname 获取主机名
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// calculateFileHash 计算文件的SHA256哈希值
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close() // 文件关闭错误通常可以忽略

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("计算哈希失败: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// verifyBackupFile 验证备份文件格式
func verifyBackupFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer file.Close() // 文件关闭错误通常可以忽略

	// 获取文件信息 - 确保文件存在并可访问
	_, err = file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 空数据库备份也是合法的，只检查文件可读性
	// 不再检查文件大小是否为0

	// 读取文件头部只是为了验证文件可读性
	header := make([]byte, 4)
	_, err = file.Read(header)
	if err != nil && err != io.EOF {
		return fmt.Errorf("读取备份文件头失败: %w", err)
	}

	return nil
}

// extractBackupInfo 从备份文件名中提取信息
func extractBackupInfo(fileName string) (time.Time, BackupType) {
	// 默认值
	defaultTime := time.Now()
	defaultType := BackupTypeAutomatic

	// 尝试从文件名中提取时间戳
	parts := strings.Split(fileName, "_")
	if len(parts) >= 3 {
		// 格式：badger_backup_YYYYMMDD_HHMMSS_type.bak
		dateStr := parts[2]
		timeStr := parts[3]

		// 尝试解析时间戳
		if len(dateStr) == 8 && len(timeStr) >= 6 {
			year, _ := strconv.Atoi(dateStr[0:4])
			month, _ := strconv.Atoi(dateStr[4:6])
			day, _ := strconv.Atoi(dateStr[6:8])

			hour, _ := strconv.Atoi(timeStr[0:2])
			minute, _ := strconv.Atoi(timeStr[2:4])
			second, _ := strconv.Atoi(timeStr[4:6])

			if year > 0 && month > 0 && day > 0 {
				parsedTime := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
				defaultTime = parsedTime
			}
		}

		// 尝试解析备份类型
		if len(parts) >= 5 {
			typeStr := strings.Split(parts[4], ".")[0]
			switch typeStr {
			case "manual":
				defaultType = BackupTypeManual
			case "pre_update":
				defaultType = BackupTypePreUpdate
			}
		}

		// 特殊处理 - 检查文件名中是否包含"pre_update"字符串
		fileNameLower := strings.ToLower(fileName)
		if strings.Contains(fileNameLower, "pre_update") {
			defaultType = BackupTypePreUpdate
		}
	}

	return defaultTime, defaultType
}

// getRSSBytesForBackup 获取进程 RSS（Resident Set Size）字节数
// 用于备份后的内存监控
//
// ✅ 修复：使用 diagnostics.GetRSSBytes() 获取当前 RSS（而不是峰值）
// 在 macOS 上，该函数会使用启发式估算方法，返回更接近实际当前 RSS 的值
func getRSSBytesForBackup() uint64 {
	// 使用 diagnostics 包中的 GetRSSBytes() 函数
	// 该函数在 macOS 上会使用启发式估算，返回更准确的当前 RSS
	return diagnostics.GetRSSBytes()
}
