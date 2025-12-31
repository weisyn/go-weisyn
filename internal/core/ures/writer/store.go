// Package writer 资源存储逻辑
package writer

import (
	"context"
	"fmt"
	"os"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// StoreResourceFile 存储资源文件
//
// 实现 ResourceWriter.StoreResourceFile
//
// 🎯 **核心流程**：
// 1. 读取源文件
// 2. 计算内容哈希（SHA256）
// 3. 检查文件是否已存在（幂等性）
// 4. 存储文件到CAS
//
// 参数：
//   - ctx: 上下文
//   - sourceFilePath: 源文件路径
//
// 返回：
//   - []byte: 内容哈希（32字节SHA256）
//   - error: 存储错误，nil表示成功
//
// 特性：
//   - 幂等性：相同内容的文件只存储一次
//   - 并发安全：使用 Lock 保护
//
// 示例：
//
//	contentHash, err := writer.StoreResourceFile(ctx, "/path/to/file.wasm")
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("文件已存储，哈希: %x\n", contentHash)
func (s *Service) StoreResourceFile(ctx context.Context, sourceFilePath string) ([]byte, error) {
	if err := writegate.Default().AssertWriteAllowed(ctx, "ures.StoreResourceFile"); err != nil {
		return nil, err
	}
	// 1. 读取源文件
	data, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFileFailed, err)
	}

	if s.logger != nil {
		s.logger.Debugf("📂 读取源文件: %s (size: %d bytes)", sourceFilePath, len(data))
	}

	// 2. 计算内容哈希（SHA256）
	contentHash := s.hasher.SHA256(data)

	if s.logger != nil {
		s.logger.Debugf("🔐 文件哈希: %x", contentHash)
	}

	// 3. 检查文件是否已存在（幂等性）
	if s.casStorage.FileExists(contentHash) {
		// 文件已存在，直接返回哈希
		if s.logger != nil {
			s.logger.Debugf("📦 文件已存在，跳过存储: %x", contentHash[:8])
		}
		return contentHash, nil
	}

	// 4. 存储文件到CAS
	if err := s.casStorage.StoreFile(ctx, contentHash, data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreFileFailed, err)
	}

	// 5. 日志记录
	if s.logger != nil {
		s.logger.Infof("✅ 资源文件已存储: %x (size: %d bytes)", contentHash[:8], len(data))
	}

	return contentHash, nil
}

