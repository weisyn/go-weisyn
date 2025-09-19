// Package resource - 文件处理工具类
//
// 🎯 **文件处理工具 (File Processing Utils)**
//
// 本文件提供静态资源部署相关的文件处理功能：
// - 文件读取和验证
// - 文件哈希计算
// - 文件大小智能处理
//
// 🏗️ **设计原则**：
// - 统一文件处理：所有文件操作的统一入口
// - 智能处理策略：根据文件大小选择不同处理方式
// - 内存高效：避免大文件全部加载到内存
package resource

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// FileUtils 文件处理工具
type FileUtils struct {
	logger log.Logger
}

// NewFileUtils 创建文件处理工具实例
func NewFileUtils(logger log.Logger) *FileUtils {
	return &FileUtils{
		logger: logger,
	}
}

// ReadFileWithValidation 读取文件并进行验证
//
// 🎯 **智能文件读取**：
// 根据文件大小自动选择最优的处理策略
//
// 参数：
//   - ctx: 上下文对象
//   - filePath: 文件路径
//
// 返回：
//   - []byte: 文件内容（小文件）或文件头（大文件）
//   - error: 读取错误
func (fu *FileUtils) ReadFileWithValidation(ctx context.Context, filePath string) ([]byte, error) {
	// 检查上下文
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("上下文已取消: %w", err)
	}

	// 🔍 业务验证：检查文件是否存在和基本属性
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if fu.logger != nil {
				fu.logger.Debug(fmt.Sprintf("文件不存在: %s", filePath))
			}
			return nil, fmt.Errorf("文件不存在: %s", filePath)
		}
		if fu.logger != nil {
			fu.logger.Error(fmt.Sprintf("文件状态检查失败: %s, 错误: %v", filePath, err))
		}
		return nil, fmt.Errorf("文件状态检查失败: %w", err)
	}

	// 检查是否为常规文件
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("不是常规文件: %s", filePath)
	}

	fileSize := stat.Size()
	fileName := filepath.Base(filePath)

	// 检查空文件（业务决策：允许但警告）
	if fileSize == 0 {
		if fu.logger != nil {
			fu.logger.Warn(fmt.Sprintf("警告：文件为空 - %s", fileName))
		}
		return []byte{}, nil // 返回空字节切片，允许空文件
	}

	// 🎯 智能处理策略：根据文件大小选择不同的处理方式
	inMemoryThreshold := int64(maxInMemoryFileSize())

	if fileSize <= inMemoryThreshold {
		// 小文件：直接读取到内存
		return fu.ReadSmallFile(filePath, fileSize)
	} else {
		// 大文件：只读取文件头用于验证和MIME检测
		return fu.ReadLargeFileHeader(filePath, fileSize)
	}
}

// ReadSmallFile 读取小文件到内存
func (fu *FileUtils) ReadSmallFile(filePath string, fileSize int64) ([]byte, error) {
	if fu.logger != nil {
		fu.logger.Debug(fmt.Sprintf("读取小文件到内存: %s (大小: %d bytes)", filePath, fileSize))
	}

	// 直接读取整个文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取小文件失败: %w", err)
	}

	// 验证读取的大小是否与预期一致
	if int64(len(data)) != fileSize {
		fu.logger.Warn(fmt.Sprintf("文件大小不一致 - 预期: %d, 实际: %d", fileSize, len(data)))
	}

	return data, nil
}

// ReadLargeFileHeader 读取大文件头部
func (fu *FileUtils) ReadLargeFileHeader(filePath string, fileSize int64) ([]byte, error) {
	if fu.logger != nil {
		fu.logger.Debug(fmt.Sprintf("读取大文件头部: %s (大小: %d bytes)", filePath, fileSize))
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开大文件失败: %w", err)
	}
	defer file.Close()

	// 读取文件头部（用于MIME类型检测和基本验证）
	headerSize := int64(1024) // 读取前1KB
	if fileSize < headerSize {
		headerSize = fileSize
	}

	header := make([]byte, headerSize)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("读取大文件头部失败: %w", err)
	}

	return header[:n], nil
}

// ComputeFileHashDirect 直接计算文件哈希
func (fu *FileUtils) ComputeFileHashDirect(ctx context.Context, filePath string) ([]byte, error) {
	if fu.logger != nil {
		fu.logger.Debug(fmt.Sprintf("计算文件哈希: %s", filePath))
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 流式计算哈希
	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return nil, fmt.Errorf("计算文件哈希失败: %w", err)
	}

	hash := hasher.Sum(nil)
	if fu.logger != nil {
		fu.logger.Debug(fmt.Sprintf("✅ 文件哈希计算完成: %x", hash))
	}

	return hash, nil
}

// maxInMemoryFileSize 返回内存处理的文件大小阈值
func maxInMemoryFileSize() int64 {
	return 10 * 1024 * 1024 // 10MB
}
