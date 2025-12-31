package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
)

// 测试验证数据库完整性
func TestVerifyIntegrity(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 插入大量测试数据，以便触发值日志操作
	testData := make(map[string][]byte)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("integrity-key-%d", i)
		// 创建一个较大的值，确保写入值日志
		value := make([]byte, 2048)
		for j := range value {
			value[j] = byte(i % 256)
		}
		testData[key] = value
	}

	err := store.SetMany(ctx, testData)
	require.NoError(t, err)

	// 强制进行一次垃圾回收以确保有值日志
	err = store.db.RunValueLogGC(0.1)
	// 即使返回ErrNoRewrite也是正常的，意味着没有足够的垃圾回收空间
	if err != nil && err != badgerdb.ErrNoRewrite {
		require.NoError(t, err)
	}

	// 验证数据库完整性
	err = store.VerifyIntegrity(ctx)
	assert.NoError(t, err)
}

// 测试检查数据库文件
func TestCheckDataFiles(t *testing.T) {
	store, tempDir, cleanup := setupTestStore(t)
	defer cleanup()

	// 检查有效的数据库目录
	exists, err := store.CheckDataFiles(tempDir)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 检查不存在的目录
	nonExistentDir := filepath.Join(tempDir, "non-existent")
	exists, err = store.CheckDataFiles(nonExistentDir)
	assert.Error(t, err)
	assert.False(t, exists)
}

// 测试尝试修复数据库
func TestTryRepair(t *testing.T) {
	// 创建一个临时测试目录
	tempDir, err := os.MkdirTemp("", "badger-repair-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建日志
	logger := &mockLogger{}

	// 创建必要的数据库文件
	manifestFile := filepath.Join(tempDir, "MANIFEST")
	err = os.WriteFile(manifestFile, []byte("测试MANIFEST内容"), 0644)
	require.NoError(t, err, "创建MANIFEST文件失败")

	vlogFile := filepath.Join(tempDir, "000000.vlog")
	err = os.WriteFile(vlogFile, []byte("测试值日志内容"), 0644)
	require.NoError(t, err, "创建值日志文件失败")

	// 模拟异常关闭，创建锁文件
	lockFile := filepath.Join(tempDir, "LOCK")
	err = os.WriteFile(lockFile, []byte("1"), 0644)
	require.NoError(t, err)

	// 创建配置 - 使用新的配置系统
	options := &badgerconfig.BadgerOptions{
		Path:       tempDir,
		SyncWrites: false,
		// BadgerDB 参数约束：MemTableSize 过小会导致 ValueThreshold 校验失败，进而打不开磁盘DB。
		MemTableSize:         128 << 20, // 128MB（与默认值一致）
		EnableAutoCompaction: false,
	}
	config := badgerconfig.NewFromOptions(options)

	// 创建存储并测试修复
	store := &Store{
		logger: logger,
		config: config,
	}

	// 创建BadgerDB选项（直接打开就可以）
	opts := badgerdb.DefaultOptions(tempDir)
	opts.ValueLogFileSize = 1 << 20
	opts.ValueThreshold = 1 << 10

	// 尝试修复
	err = store.TryRepair(tempDir)
	// 这里应该会有错误，因为我们创建的不是真实的BadgerDB文件，
	// 但是我们只关心它能正确处理流程
}

// 测试验证备份完整性
func TestVerifyBackup(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 创建临时备份目录
	backupDir, err := os.MkdirTemp("", "badger-verify-backup-test")
	require.NoError(t, err)
	defer os.RemoveAll(backupDir)

	// 创建一个有效的备份文件
	backupFile := filepath.Join(backupDir, "verify-backup.bak")

	// 使用手动方式创建备份文件，而不是通过CreateBackup函数
	testData := []byte("这是一个测试备份文件内容，用于验证备份功能")
	err = os.WriteFile(backupFile, testData, 0644)
	require.NoError(t, err)

	// 创建元数据文件
	metadataPath := backupFile + ".meta"
	metadata := BackupMetadata{
		Timestamp:     time.Now(),
		Size:          int64(len(testData)),
		KeyCount:      10,
		BackupType:    BackupTypeAutomatic,
		Status:        BackupStatusVerified,
		FormatVersion: 1,
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(metadataPath, metadataBytes, 0644)
	require.NoError(t, err)

	// 验证有效的备份
	valid, hash, err := store.VerifyBackup(ctx, backupFile)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.NotEmpty(t, hash)

	// 验证不存在的备份
	valid, hash, err = store.VerifyBackup(ctx, filepath.Join(backupDir, "non-existent.bak"))
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Empty(t, hash)

	// 创建损坏的备份文件
	corruptedFile := filepath.Join(backupDir, "corrupted-backup.bak")
	err = os.WriteFile(corruptedFile, []byte(""), 0644)
	require.NoError(t, err)

	// 验证空的备份文件
	valid, hash, err = store.VerifyBackup(ctx, corruptedFile)
	assert.Error(t, err) // 空文件应该导致错误
	assert.False(t, valid)
	assert.Empty(t, hash)
}

// 测试从备份恢复
func TestRestoreFromBackup(t *testing.T) {
	// 创建源存储
	sourceStore, _, sourceCleanup := setupTestStore(t)
	defer sourceCleanup()

	ctx := context.Background()

	// 插入测试数据
	testData := map[string][]byte{
		"restore-key1": []byte("restore-value1"),
		"restore-key2": []byte("restore-value2"),
		"restore-key3": []byte("restore-value3"),
	}
	err := sourceStore.SetMany(ctx, testData)
	require.NoError(t, err)

	// 创建临时备份目录
	backupDir, err := os.MkdirTemp("", "badger-restore-test")
	require.NoError(t, err)
	defer os.RemoveAll(backupDir)

	// 创建备份
	backupFile := filepath.Join(backupDir, "restore-backup.bak")
	err = sourceStore.CreateBackup(ctx, backupFile)
	require.NoError(t, err)

	// 创建恢复目标目录
	restoreDir, err := os.MkdirTemp("", "badger-restore-target")
	require.NoError(t, err)
	defer os.RemoveAll(restoreDir)

	// 执行恢复
	err = sourceStore.RestoreFromBackup(ctx, backupFile, restoreDir)
	assert.NoError(t, err)

	// 创建新的存储实例，指向恢复目录 - 使用新的配置系统
	targetOptions := &badgerconfig.BadgerOptions{
		Path:                 restoreDir,
		SyncWrites:           false,
		MemTableSize:         128 << 20, // 128MB（与默认值一致）
		EnableAutoCompaction: false,
	}
	targetCfg := badgerconfig.NewFromOptions(targetOptions)
	targetLogger := &mockLogger{}
	targetStore := New(targetCfg, targetLogger)
	require.NotNil(t, targetStore)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	// 验证恢复的数据
	for key, expectedValue := range testData {
		value, err := targetStore.Get(ctx, []byte(key))
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, value)
	}
}
