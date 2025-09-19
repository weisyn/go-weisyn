// Package temp 提供基于文件系统的临时存储实现
package temp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	temporaryconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// tempFileRecord 临时文件记录
type tempFileRecord struct {
	ID         string
	Path       string
	Size       int64
	CreateTime time.Time
	ExpireTime time.Time
}

// Store 实现TempStore接口
type Store struct {
	config     *temporaryconfig.Config
	logger     log.Logger
	tempDir    string
	mu         sync.RWMutex
	files      map[string]*tempFileRecord // 临时文件记录映射
	dirs       map[string]*tempFileRecord // 临时目录记录映射
	closed     bool
	cancelFunc context.CancelFunc // 用于取消清理协程
}

// New 创建新的TempStore实例
func New(config *temporaryconfig.Config, logger log.Logger) storage.TempStore {
	tempDir := config.GetTempDir()

	// 确保临时目录存在
	if err := os.MkdirAll(tempDir, os.FileMode(config.GetDirectoryPermissions())); err != nil {
		logger.Errorf("无法创建临时存储目录 %s: %v", tempDir, err)
		return nil
	}

	store := &Store{
		config:  config,
		logger:  logger,
		tempDir: tempDir,
		files:   make(map[string]*tempFileRecord),
		dirs:    make(map[string]*tempFileRecord),
	}

	// 启动清理协程
	if config.IsAutoCleanupEnabled() {
		ctx, cancel := context.WithCancel(context.Background())
		store.cancelFunc = cancel
		go store.cleanupRoutine(ctx)
	}

	// 恢复已存在的临时文件记录
	store.restoreExistingFiles()

	logger.Infof("临时存储初始化成功，目录: %s", tempDir)
	return store
}

// CreateTempFile 创建临时文件
func (s *Store) CreateTempFile(ctx context.Context, prefix, suffix string) (string, io.ReadWriteCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return "", nil, fmt.Errorf("临时存储已关闭")
	}

	// 检查临时文件数量限制
	if len(s.files) >= s.config.GetMaxTempFiles() {
		return "", nil, fmt.Errorf("临时文件数量已达上限 %d", s.config.GetMaxTempFiles())
	}

	// 生成唯一ID
	id, err := s.generateUniqueID()
	if err != nil {
		return "", nil, fmt.Errorf("生成临时文件ID失败: %w", err)
	}

	// 构建文件名和路径
	filename := fmt.Sprintf("%s_%s_%s", prefix, id, suffix)
	fullPath := filepath.Join(s.tempDir, filename)

	// 创建文件
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR, os.FileMode(s.config.GetFilePermissions()))
	if err != nil {
		return "", nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 记录文件信息
	now := time.Now()
	expireTime := now.Add(s.config.GetDefaultTTL())
	record := &tempFileRecord{
		ID:         id,
		Path:       fullPath,
		Size:       0,
		CreateTime: now,
		ExpireTime: expireTime,
	}
	s.files[id] = record

	s.logger.Debugf("创建临时文件成功: %s (ID: %s)", filename, id)
	return id, file, nil
}

// CreateTempFileWithContent 创建临时文件并写入内容
func (s *Store) CreateTempFileWithContent(ctx context.Context, prefix, suffix string, content []byte) (string, error) {
	// 检查文件大小限制
	sizeMB := int64(len(content)) / (1024 * 1024)
	if sizeMB > s.config.GetMaxTempFileSize() {
		return "", fmt.Errorf("临时文件大小 %dMB 超过限制 %dMB", sizeMB, s.config.GetMaxTempFileSize())
	}

	// 创建临时文件
	id, file, err := s.CreateTempFile(ctx, prefix, suffix)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 写入内容
	if _, err := file.Write(content); err != nil {
		// 如果写入失败，删除已创建的文件
		_ = s.RemoveTempFile(ctx, id)
		return "", fmt.Errorf("写入临时文件内容失败: %w", err)
	}

	// 更新文件大小记录
	s.mu.Lock()
	if record, exists := s.files[id]; exists {
		record.Size = int64(len(content))
	}
	s.mu.Unlock()

	s.logger.Debugf("创建带内容的临时文件成功: ID: %s, 大小: %d", id, len(content))
	return id, nil
}

// GetTempFile 获取临时文件内容
func (s *Store) GetTempFile(ctx context.Context, id string) ([]byte, error) {
	s.mu.RLock()
	record, exists := s.files[id]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("临时文件不存在: %s", id)
	}

	// 检查文件是否过期
	if time.Now().After(record.ExpireTime) {
		// 文件过期，删除它
		_ = s.RemoveTempFile(ctx, id)
		return nil, fmt.Errorf("临时文件已过期: %s", id)
	}

	// 读取文件内容
	data, err := os.ReadFile(record.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，从记录中移除
			s.mu.Lock()
			delete(s.files, id)
			s.mu.Unlock()
			return nil, fmt.Errorf("临时文件不存在: %s", id)
		}
		return nil, fmt.Errorf("读取临时文件失败: %w", err)
	}

	return data, nil
}

// OpenTempFile 打开临时文件
func (s *Store) OpenTempFile(ctx context.Context, id string) (io.ReadWriteCloser, error) {
	s.mu.RLock()
	record, exists := s.files[id]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("临时文件不存在: %s", id)
	}

	// 检查文件是否过期
	if time.Now().After(record.ExpireTime) {
		// 文件过期，删除它
		_ = s.RemoveTempFile(ctx, id)
		return nil, fmt.Errorf("临时文件已过期: %s", id)
	}

	// 打开文件
	file, err := os.OpenFile(record.Path, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，从记录中移除
			s.mu.Lock()
			delete(s.files, id)
			s.mu.Unlock()
			return nil, fmt.Errorf("临时文件不存在: %s", id)
		}
		return nil, fmt.Errorf("打开临时文件失败: %w", err)
	}

	return file, nil
}

// RemoveTempFile 删除临时文件
func (s *Store) RemoveTempFile(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.files[id]
	if !exists {
		// 文件不存在，不返回错误
		return nil
	}

	// 删除物理文件
	if err := os.Remove(record.Path); err != nil && !os.IsNotExist(err) {
		s.logger.Warnf("删除临时文件失败 %s: %v", record.Path, err)
	}

	// 从记录中移除
	delete(s.files, id)

	s.logger.Debugf("删除临时文件成功: ID: %s", id)
	return nil
}

// CreateTempDir 创建临时目录
func (s *Store) CreateTempDir(ctx context.Context, prefix string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return "", fmt.Errorf("临时存储已关闭")
	}

	// 生成唯一ID
	id, err := s.generateUniqueID()
	if err != nil {
		return "", fmt.Errorf("生成临时目录ID失败: %w", err)
	}

	// 构建目录名和路径
	dirname := fmt.Sprintf("%s_%s", prefix, id)
	fullPath := filepath.Join(s.tempDir, dirname)

	// 创建目录
	if err := os.Mkdir(fullPath, os.FileMode(s.config.GetDirectoryPermissions())); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 记录目录信息
	now := time.Now()
	expireTime := now.Add(s.config.GetDefaultTTL())
	record := &tempFileRecord{
		ID:         id,
		Path:       fullPath,
		Size:       0,
		CreateTime: now,
		ExpireTime: expireTime,
	}
	s.dirs[id] = record

	s.logger.Debugf("创建临时目录成功: %s (ID: %s)", dirname, id)
	return id, nil
}

// RemoveTempDir 删除临时目录
func (s *Store) RemoveTempDir(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.dirs[id]
	if !exists {
		// 目录不存在，不返回错误
		return nil
	}

	// 删除物理目录和内容
	if err := os.RemoveAll(record.Path); err != nil && !os.IsNotExist(err) {
		s.logger.Warnf("删除临时目录失败 %s: %v", record.Path, err)
	}

	// 从记录中移除
	delete(s.dirs, id)

	s.logger.Debugf("删除临时目录成功: ID: %s", id)
	return nil
}

// ListTempFiles 列出所有临时文件
func (s *Store) ListTempFiles(ctx context.Context, pattern string) ([]types.TempFileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []types.TempFileInfo
	now := time.Now()

	for id, record := range s.files {
		// 检查是否过期
		if now.After(record.ExpireTime) {
			continue // 跳过过期文件（在清理时会被移除）
		}

		// 应用模式过滤
		if pattern != "" {
			filename := filepath.Base(record.Path)
			matched, err := filepath.Match(pattern, filename)
			if err != nil {
				s.logger.Warnf("模式匹配失败 %s: %v", pattern, err)
				continue
			}
			if !matched {
				continue
			}
		}

		// 获取当前文件大小
		size := record.Size
		if stat, err := os.Stat(record.Path); err == nil {
			size = stat.Size()
		}

		result = append(result, types.TempFileInfo{
			ID:         id,
			Size:       size,
			CreateTime: record.CreateTime,
			ExpireTime: record.ExpireTime,
		})
	}

	return result, nil
}

// CleanupExpired 清理所有过期的临时文件和目录
func (s *Store) CleanupExpired(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanedCount := 0
	now := time.Now()

	// 清理过期文件
	for id, record := range s.files {
		if now.After(record.ExpireTime) {
			// 删除物理文件
			if err := os.Remove(record.Path); err != nil && !os.IsNotExist(err) {
				s.logger.Warnf("删除过期临时文件失败 %s: %v", record.Path, err)
			} else {
				cleanedCount++
				s.logger.Debugf("清理过期临时文件: ID: %s", id)
			}
			delete(s.files, id)
		}
	}

	// 清理过期目录
	for id, record := range s.dirs {
		if now.After(record.ExpireTime) {
			// 删除物理目录
			if err := os.RemoveAll(record.Path); err != nil && !os.IsNotExist(err) {
				s.logger.Warnf("删除过期临时目录失败 %s: %v", record.Path, err)
			} else {
				cleanedCount++
				s.logger.Debugf("清理过期临时目录: ID: %s", id)
			}
			delete(s.dirs, id)
		}
	}

	if cleanedCount > 0 {
		s.logger.Infof("清理过期临时文件和目录 %d 个", cleanedCount)
	}

	return cleanedCount, nil
}

// SetExpiration 设置临时文件或目录的过期时间
func (s *Store) SetExpiration(ctx context.Context, id string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查文件记录
	if record, exists := s.files[id]; exists {
		if duration <= 0 {
			record.ExpireTime = time.Now().Add(s.config.GetDefaultTTL())
		} else {
			record.ExpireTime = time.Now().Add(duration)
		}
		s.logger.Debugf("更新临时文件过期时间: ID: %s, 过期时间: %v", id, record.ExpireTime)
		return nil
	}

	// 检查目录记录
	if record, exists := s.dirs[id]; exists {
		if duration <= 0 {
			record.ExpireTime = time.Now().Add(s.config.GetDefaultTTL())
		} else {
			record.ExpireTime = time.Now().Add(duration)
		}
		s.logger.Debugf("更新临时目录过期时间: ID: %s, 过期时间: %v", id, record.ExpireTime)
		return nil
	}

	return fmt.Errorf("临时文件或目录不存在: %s", id)
}

// generateUniqueID 生成唯一ID
func (s *Store) generateUniqueID() (string, error) {
	for i := 0; i < 10; i++ { // 最多尝试10次
		// 生成随机字节
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}

		// 转换为十六进制字符串
		id := hex.EncodeToString(bytes)

		// 检查ID是否已存在
		if _, exists := s.files[id]; !exists {
			if _, exists := s.dirs[id]; !exists {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("生成唯一ID失败")
}

// restoreExistingFiles 恢复已存在的临时文件记录
func (s *Store) restoreExistingFiles() {
	// 扫描临时目录中的现有文件
	entries, err := os.ReadDir(s.tempDir)
	if err != nil {
		s.logger.Warnf("扫描临时目录失败: %v", err)
		return
	}

	now := time.Now()
	defaultTTL := s.config.GetDefaultTTL()

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(s.tempDir, name)

		// 解析文件名以提取ID
		parts := strings.Split(name, "_")
		if len(parts) < 2 {
			continue // 跳过不符合命名规范的文件
		}

		var id string
		if len(parts) == 3 {
			id = parts[1] // prefix_id_suffix 格式
		} else if len(parts) == 2 {
			id = parts[1] // prefix_id 格式（目录）
		} else {
			continue
		}

		// 获取文件信息
		stat, err := entry.Info()
		if err != nil {
			continue
		}

		// 创建记录
		record := &tempFileRecord{
			ID:         id,
			Path:       fullPath,
			Size:       stat.Size(),
			CreateTime: stat.ModTime(),      // 使用修改时间作为创建时间的近似值
			ExpireTime: now.Add(defaultTTL), // 设置新的过期时间
		}

		if entry.IsDir() {
			s.dirs[id] = record
		} else {
			s.files[id] = record
		}
	}

	fileCount := len(s.files)
	dirCount := len(s.dirs)
	if fileCount > 0 || dirCount > 0 {
		s.logger.Infof("恢复临时存储记录: %d 个文件, %d 个目录", fileCount, dirCount)
	}
}

// cleanupRoutine 清理协程
func (s *Store) cleanupRoutine(ctx context.Context) {
	interval := s.config.GetCleanupInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Infof("启动临时存储清理协程，清理间隔: %v", interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("临时存储清理协程已停止")
			return
		case <-ticker.C:
			if count, err := s.CleanupExpired(ctx); err != nil {
				s.logger.Errorf("自动清理过期文件失败: %v", err)
			} else if count > 0 {
				s.logger.Infof("自动清理完成，清理了 %d 个过期项目", count)
			}
		}
	}
}

// Close 关闭临时存储
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// 停止清理协程
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// 执行最后一次清理（带超时机制）
	if s.config.IsAutoCleanupEnabled() {
		s.logger.Info("🔧 执行最后一次临时文件清理...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan bool, 1)
		go func() {
			_, _ = s.CleanupExpired(ctx)
			done <- true
		}()

		select {
		case <-done:
			s.logger.Info("🔧 临时文件清理完成")
		case <-time.After(2 * time.Second):
			s.logger.Warn("🔧 临时文件清理超时，跳过")
		}
	} else {
		s.logger.Info("🔧 自动清理已禁用，跳过最后清理")
	}

	s.logger.Info("🔧 临时存储已关闭")
	return nil
}
