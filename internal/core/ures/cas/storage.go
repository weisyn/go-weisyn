// Package cas 文件存储逻辑
package cas

import (
	"context"
	"fmt"
)

// StoreFile 存储文件
//
// 实现 CASStorage.StoreFile
//
// 🎯 **核心流程**：
// 1. 验证参数（哈希长度、数据非空）
// 2. 构建文件路径
// 3. 检查文件是否已存在（幂等性）
// 4. 存储文件到 FileStore
//
// 参数：
//   - ctx: 上下文
//   - contentHash: 内容哈希（32字节SHA256）
//   - data: 文件数据
//
// 返回：
//   - error: 存储错误，nil表示成功
//
// 特性：
//   - 幂等性：相同内容的文件只存储一次
//   - 并发安全：使用 Lock 保护
func (s *Service) StoreFile(ctx context.Context, contentHash []byte, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 验证参数
	if len(contentHash) != 32 {
		return fmt.Errorf("%w: %d", ErrInvalidHashLength, len(contentHash))
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	// 2. 构建文件路径
	// 注意：FileStore 的根目录由配置决定（在节点场景下通常为 {instance_data_dir}/files），
	// 因此这里不需要也不应该再添加 "files/" 前缀，只构建相对路径。
	// 路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	fullPath := s.buildFilePathInternal(contentHash)
	if fullPath == "" {
		return ErrBuildPathFailed
	}

	// 3. 检查文件是否已存在（幂等性）
	exists, err := s.fileStore.Exists(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("检查文件存在失败: %w", err)
	}
	if exists {
		// 文件已存在，直接返回（幂等性）
		if s.logger != nil {
			s.logger.Debugf("📦 文件已存在，跳过存储: %s", fullPath)
		}
		return nil
	}

	// 4. 存储文件
	if err := s.fileStore.Save(ctx, fullPath, data); err != nil {
		return fmt.Errorf("存储文件失败: %w", err)
	}

	// 5. 日志记录
	if s.logger != nil {
		s.logger.Debugf("✅ 文件已存储: %s (size: %d bytes)", fullPath, len(data))
	}

	return nil
}

// ReadFile 读取文件
//
// 实现 CASStorage.ReadFile
//
// 🎯 **核心流程**：
// 1. 验证参数（哈希长度）
// 2. 构建文件路径
// 3. 从 FileStore 读取文件
//
// 参数：
//   - ctx: 上下文
//   - contentHash: 内容哈希（32字节SHA256）
//
// 返回：
//   - []byte: 文件数据
//   - error: 读取错误，nil表示成功
func (s *Service) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. 验证参数
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidHashLength, len(contentHash))
	}

	// 2. 构建文件路径
	// 注意：FileStore 的根目录已经是 ./data/files，所以不需要再添加 "files/" 前缀
	// 路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	fullPath := s.buildFilePathInternal(contentHash)
	if fullPath == "" {
		return nil, ErrBuildPathFailed
	}

	// 3. 读取文件
	data, err := s.fileStore.Load(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 4. 日志记录
	if s.logger != nil {
		s.logger.Debugf("📖 文件已读取: %s (size: %d bytes)", fullPath, len(data))
	}

	return data, nil
}

// FileExists 检查文件存在
//
// 实现 CASStorage.FileExists
//
// 参数：
//   - contentHash: 内容哈希（32字节SHA256）
//
// 返回：
//   - bool: 文件是否存在
func (s *Service) FileExists(contentHash []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. 验证参数
	if len(contentHash) != 32 {
		if s.logger != nil {
			s.logger.Warnf("CAS.FileExists: 无效的内容哈希长度: %d（期望32字节）", len(contentHash))
		}
		return false
	}

	// 2. 构建文件路径
	// 注意：FileStore 的根目录已经是 ./data/files，所以不需要再添加 "files/" 前缀
	// 路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	fullPath := s.buildFilePathInternal(contentHash)
	if fullPath == "" {
		if s.logger != nil {
			s.logger.Warnf("CAS.FileExists: 构建文件路径失败（contentHash=%x）", contentHash)
		}
		return false
	}

	// 4. 检查文件存在
	exists, err := s.fileStore.Exists(context.Background(), fullPath)
	if err != nil {
		// 检查失败：记录告警日志，返回 false（保持接口语义）
		if s.logger != nil {
			s.logger.Warnf("CAS.FileExists: 底层 FileStore.Exists 失败, path=%s, err=%v", fullPath, err)
		}
		return false
	}
	return exists
}

// buildFilePathInternal 内部路径构建（不加锁）
//
// 供 StoreFile、ReadFile、FileExists 内部使用
func (s *Service) buildFilePathInternal(contentHash []byte) string {
	// 调用公共接口方法（不加锁，由调用方加锁）
	return s.BuildFilePath(contentHash)
}
