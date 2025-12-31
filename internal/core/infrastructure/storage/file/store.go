// Package file 提供基于文件系统的存储实现
package file

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// Store 实现FileStore接口
type Store struct {
	config   *fileconfig.Config
	logger   log.Logger
	rootPath string
	mu       sync.RWMutex
	closed   bool
}

// Prometheus 指标：观测 FileStore.Load 调用情况
var (
	filestoreLoadRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "storage_filestore_load_requests_total",
		Help: "Total number of FileStore.Load calls.",
	})
	filestoreLoadErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "storage_filestore_load_errors_total",
		Help: "Total number of FileStore.Load errors.",
	})
	filestoreLoadDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "storage_filestore_load_duration_seconds",
		Help:    "Duration of FileStore.Load calls.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(
		filestoreLoadRequests,
		filestoreLoadErrors,
		filestoreLoadDuration,
	)
}

// New 创建新的FileStore实例
func New(config *fileconfig.Config, logger log.Logger) storage.FileStore {
	rootPath := config.GetRootPath()
	// 统一为绝对路径，避免相对路径导致的边界校验误判
	if abs, err := filepath.Abs(rootPath); err == nil {
		rootPath = abs
	}

	// 确保根目录存在
	if err := os.MkdirAll(rootPath, os.FileMode(config.GetDirectoryPermissions())); err != nil {
		logger.Errorf("无法创建文件存储根目录 %s: %v", rootPath, err)
		return nil
	}

	store := &Store{
		config:   config,
		logger:   logger,
		rootPath: rootPath,
	}

	logger.Infof("文件存储初始化成功，根目录: %s", rootPath)
	return store
}

// Save 保存数据到指定路径
func (s *Store) Save(ctx context.Context, path string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	// 检查文件大小限制
	sizeMB := int64(len(data)) / (1024 * 1024)
	if sizeMB > s.config.GetMaxFileSize() {
		return fmt.Errorf("文件大小 %dMB 超过限制 %dMB", sizeMB, s.config.GetMaxFileSize())
	}

	// 获取完整路径
	fullPath, err := s.getFullPath(path)
	if err != nil {
		return err
	}

	// 确保父目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, os.FileMode(s.config.GetDirectoryPermissions())); err != nil {
		s.logger.Errorf("创建目录失败 %s: %v", dir, err)
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, data, os.FileMode(s.config.GetFilePermissions())); err != nil {
		s.logger.Errorf("保存文件失败 %s: %v", fullPath, err)
		return fmt.Errorf("保存文件失败: %w", err)
	}

	// 如果启用了文件校验，计算并记录校验和
	if s.config.IsFileVerificationEnabled() {
		if err := s.saveChecksum(fullPath, data); err != nil {
			s.logger.Warnf("保存文件校验和失败 %s: %v", fullPath, err)
		}
	}

	s.logger.Debugf("文件保存成功: %s", path)
	return nil
}

// Load 从指定路径加载数据
func (s *Store) Load(ctx context.Context, path string) ([]byte, error) {
	filestoreLoadRequests.Inc()
	start := time.Now()
	defer filestoreLoadDuration.Observe(time.Since(start).Seconds())

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		filestoreLoadErrors.Inc()
		metricsutil.RecordFileLoad(time.Since(start), true)
		return nil, fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		filestoreLoadErrors.Inc()
		metricsutil.RecordFileLoad(time.Since(start), true)
		return nil, err
	}

	// 🔧 调试：打印详细路径信息
	if s.logger != nil {
		s.logger.Debugf("🔍 [FileStore] 读取文件: path=%s, rootPath=%s, fullPath=%s", path, s.rootPath, fullPath)
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if s.logger != nil {
			s.logger.Errorf("❌ [FileStore] 文件不存在: path=%s, fullPath=%s", path, fullPath)
		}
		filestoreLoadErrors.Inc()
		metricsutil.RecordFileLoad(time.Since(start), true)
		return nil, fmt.Errorf("文件不存在: %s (完整路径: %s)", path, fullPath)
	}

	// 读取文件
	data, err := os.ReadFile(fullPath)
	if err != nil {
		s.logger.Errorf("读取文件失败 %s: %v", fullPath, err)
		filestoreLoadErrors.Inc()
		metricsutil.RecordFileLoad(time.Since(start), true)
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 如果启用了文件校验，验证校验和
	if s.config.IsFileVerificationEnabled() {
		if err := s.verifyChecksum(fullPath, data); err != nil {
			s.logger.Errorf("文件校验失败 %s: %v", fullPath, err)
			filestoreLoadErrors.Inc()
			metricsutil.RecordFileLoad(time.Since(start), true)
			return nil, fmt.Errorf("文件校验失败: %w", err)
		}
	}

	s.logger.Debugf("文件读取成功: %s", path)
	metricsutil.RecordFileLoad(time.Since(start), false)
	return data, nil
}

// Delete 删除指定路径的文件
func (s *Store) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		return err
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", path)
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		s.logger.Errorf("删除文件失败 %s: %v", fullPath, err)
		return fmt.Errorf("删除文件失败: %w", err)
	}

	// 删除校验和文件（如果存在）
	checksumPath := fullPath + ".sha256"
	if _, err := os.Stat(checksumPath); err == nil {
		if err := os.Remove(checksumPath); err != nil {
			s.logger.Warnf("删除校验和文件失败: %v", err)
		}
	}

	s.logger.Debugf("文件删除成功: %s", path)
	return nil
}

// Exists 检查指定路径的文件是否存在
func (s *Store) Exists(ctx context.Context, path string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查文件存在性失败: %w", err)
	}

	return true, nil
}

// FileInfo 获取文件信息
func (s *Store) FileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return types.FileInfo{}, fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		return types.FileInfo{}, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.FileInfo{}, fmt.Errorf("文件不存在: %s", path)
		}
		return types.FileInfo{}, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return types.FileInfo{
		Size:       stat.Size(),
		CreateTime: getCreateTime(stat),
		ModTime:    stat.ModTime(),
		IsDir:      stat.IsDir(),
	}, nil
}

// ListFiles 列出指定目录下的所有文件
func (s *Store) ListFiles(ctx context.Context, dirPath string, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("文件存储已关闭")
	}

	fullDirPath, err := s.getFullPath(dirPath)
	if err != nil {
		return nil, err
	}

	// 检查目录是否存在
	if _, err := os.Stat(fullDirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("目录不存在: %s", dirPath)
	}

	// 读取目录内容
	entries, err := os.ReadDir(fullDirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue // 跳过目录，只返回文件
		}

		filename := entry.Name()
		// 过滤掉校验和文件
		if strings.HasSuffix(filename, ".sha256") {
			continue
		}

		// 应用模式过滤
		if pattern != "" {
			matched, err := filepath.Match(pattern, filename)
			if err != nil {
				s.logger.Warnf("模式匹配失败 %s: %v", pattern, err)
				continue
			}
			if !matched {
				continue
			}
		}

		files = append(files, filepath.Join(dirPath, filename))
	}

	return files, nil
}

// MakeDir 创建目录
func (s *Store) MakeDir(ctx context.Context, dirPath string, recursive bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	fullDirPath, err := s.getFullPath(dirPath)
	if err != nil {
		return err
	}

	if recursive {
		err := os.MkdirAll(fullDirPath, os.FileMode(s.config.GetDirectoryPermissions()))
		if err != nil {
			return fmt.Errorf("递归创建目录失败: %w", err)
		}
	} else {
		err := os.Mkdir(fullDirPath, os.FileMode(s.config.GetDirectoryPermissions()))
		if err != nil {
			if os.IsExist(err) {
				return nil // 目录已存在，不返回错误
			}
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	s.logger.Debugf("目录创建成功: %s", dirPath)
	return nil
}

// DeleteDir 删除目录
func (s *Store) DeleteDir(ctx context.Context, dirPath string, recursive bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	fullDirPath, err := s.getFullPath(dirPath)
	if err != nil {
		return err
	}

	// 检查目录是否存在
	if _, err := os.Stat(fullDirPath); os.IsNotExist(err) {
		return fmt.Errorf("目录不存在: %s", dirPath)
	}

	if recursive {
		err := os.RemoveAll(fullDirPath)
		if err != nil {
			return fmt.Errorf("递归删除目录失败: %w", err)
		}
	} else {
		err := os.Remove(fullDirPath)
		if err != nil {
			return fmt.Errorf("删除目录失败: %w", err)
		}
	}

	s.logger.Debugf("目录删除成功: %s", dirPath)
	return nil
}

// OpenReadStream 打开文件的读取流
func (s *Store) OpenReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("打开文件读取流失败: %w", err)
	}

	return file, nil
}

// OpenWriteStream 打开文件的写入流
func (s *Store) OpenWriteStream(ctx context.Context, path string) (io.WriteCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("文件存储已关闭")
	}

	fullPath, err := s.getFullPath(path)
	if err != nil {
		return nil, err
	}

	// 确保父目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, os.FileMode(s.config.GetDirectoryPermissions())); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(s.config.GetFilePermissions()))
	if err != nil {
		return nil, fmt.Errorf("打开文件写入流失败: %w", err)
	}

	return file, nil
}

// Copy 复制文件
func (s *Store) Copy(ctx context.Context, sourcePath, destPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	sourceFullPath, err := s.getFullPath(sourcePath)
	if err != nil {
		return err
	}
	destFullPath, err := s.getFullPath(destPath)
	if err != nil {
		return err
	}

	// 检查源文件是否存在
	if _, err := os.Stat(sourceFullPath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 确保目标文件的父目录存在
	dir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(dir, os.FileMode(s.config.GetDirectoryPermissions())); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 打开源文件
	sourceFile, err := os.Open(sourceFullPath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			s.logger.Warnf("关闭源文件失败: %v", err)
		}
	}()

	// 创建目标文件
	destFile, err := os.OpenFile(destFullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(s.config.GetFilePermissions()))
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			s.logger.Warnf("关闭目标文件失败: %v", err)
		}
	}()

	// 复制数据
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件数据失败: %w", err)
	}

	// 同步到磁盘
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("同步文件到磁盘失败: %w", err)
	}

	s.logger.Debugf("文件复制成功: %s -> %s", sourcePath, destPath)
	return nil
}

// Move 移动文件
func (s *Store) Move(ctx context.Context, sourcePath, destPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("文件存储已关闭")
	}

	sourceFullPath, err := s.getFullPath(sourcePath)
	if err != nil {
		return err
	}
	destFullPath, err := s.getFullPath(destPath)
	if err != nil {
		return err
	}

	// 检查源文件是否存在
	if _, err := os.Stat(sourceFullPath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 确保目标文件的父目录存在
	dir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(dir, os.FileMode(s.config.GetDirectoryPermissions())); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 移动文件
	if err := os.Rename(sourceFullPath, destFullPath); err != nil {
		return fmt.Errorf("移动文件失败: %w", err)
	}

	// 移动校验和文件（如果存在）
	sourceChecksumPath := sourceFullPath + ".sha256"
	destChecksumPath := destFullPath + ".sha256"
	if _, err := os.Stat(sourceChecksumPath); err == nil {
		if err := os.Rename(sourceChecksumPath, destChecksumPath); err != nil {
			s.logger.Warnf("移动校验和文件失败: %v", err)
		}
	}

	s.logger.Debugf("文件移动成功: %s -> %s", sourcePath, destPath)
	return nil
}

func isWithinRoot(root, fullPath string) bool {
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

// getFullPath 获取完整路径（带边界校验）
//
// 约束：
// - 禁止绝对路径（避免绕过根目录约束）
// - 禁止任何形式的 ".." 越界
// - 特殊支持 blocks/ 前缀：blocks 目录与 files 目录同级，允许通过 blocks/... 访问区块文件
func (s *Store) getFullPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("非法路径：空路径")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("非法路径：不允许绝对路径: %s", path)
	}

	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return "", fmt.Errorf("非法路径：%s", path)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法路径：禁止越界访问: %s", path)
	}

	// blocks/ 特殊前缀：映射到 {instance_data_dir}/blocks/*
	if cleaned == "blocks" || strings.HasPrefix(cleaned, "blocks"+string(os.PathSeparator)) {
		instanceRoot := filepath.Dir(s.rootPath) // {instance_data_dir}
		full := filepath.Clean(filepath.Join(instanceRoot, cleaned))
		if !isWithinRoot(instanceRoot, full) {
			return "", fmt.Errorf("非法路径：blocks 越界访问: %s", path)
		}
		return full, nil
	}

	full := filepath.Clean(filepath.Join(s.rootPath, cleaned))
	if !isWithinRoot(s.rootPath, full) {
		return "", fmt.Errorf("非法路径：越界访问: %s", path)
	}
	return full, nil
}

// saveChecksum 保存文件校验和
func (s *Store) saveChecksum(filePath string, data []byte) error {
	hash := sha256.Sum256(data)
	checksumPath := filePath + ".sha256"
	if err := os.WriteFile(checksumPath, []byte(fmt.Sprintf("%x", hash)), 0600); err != nil {
		return fmt.Errorf("写入校验和文件失败: %w", err)
	}
	return nil
}

// verifyChecksum 验证文件校验和
func (s *Store) verifyChecksum(filePath string, data []byte) error {
	checksumPath := filePath + ".sha256"

	// 如果校验和文件不存在，跳过验证
	if _, err := os.Stat(checksumPath); os.IsNotExist(err) {
		return nil
	}

	// 读取存储的校验和
	storedChecksum, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("读取校验和文件失败: %w", err)
	}

	// 计算当前数据的校验和
	hash := sha256.Sum256(data)
	currentChecksum := fmt.Sprintf("%x", hash)

	// 比较校验和
	if string(storedChecksum) != currentChecksum {
		return fmt.Errorf("文件校验和不匹配")
	}

	return nil
}

// getCreateTime 获取文件创建时间（跨平台兼容）
func getCreateTime(stat os.FileInfo) time.Time {
	// 在不同平台上，创建时间的获取方式可能不同
	// 这里使用修改时间作为创建时间的近似值
	return stat.ModTime()
}
