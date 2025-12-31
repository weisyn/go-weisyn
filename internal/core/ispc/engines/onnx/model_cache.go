//go:build !android && !ios && cgo
// +build !android,!ios,cgo

package onnx

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ModelCache 模型元数据缓存
//
// 🎯 **核心职责**：
// - 缓存模型元数据（输入/输出名称、形状信息）
// - 避免重复解析模型文件
// - 注意：由于onnxruntime_go API限制，无法预创建会话
//   会话需要在执行时动态创建（需要实际的输入/输出张量）
type ModelCache struct {
	metadata map[string]*ModelMetadata // modelAddress -> metadata
	mu       sync.RWMutex
	logger   log.Logger
}

// NewModelCache 创建模型缓存
func NewModelCache(logger log.Logger) *ModelCache {
	return &ModelCache{
		metadata: make(map[string]*ModelMetadata),
		logger:   logger,
	}
}

// GetOrLoadMetadata 获取或加载模型元数据
//
// 流程：
// 1. 检查缓存是否存在
// 2. 缓存未命中时，从modelBytes提取元数据
// 3. 加入缓存并返回
//
// 返回：
//   - *ModelMetadata: 模型元数据
//   - bool: 是否为缓存命中
//   - error: 错误信息
func (mc *ModelCache) GetOrLoadMetadata(
	ctx context.Context,
	modelAddress string,
	modelBytes []byte,
	logger log.Logger,
) (*ModelMetadata, bool, error) {
	// 1. 尝试从缓存获取
	mc.mu.RLock()
	if metadata, ok := mc.metadata[modelAddress]; ok {
		mc.mu.RUnlock()

		if logger != nil {
			logger.Debugf("使用缓存的ONNX模型元数据 model=%s", modelAddress)
		}

		return metadata, true, nil
	}
	mc.mu.RUnlock()

	// 2. 缓存未命中,提取元数据
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 双重检查(避免并发重复提取)
	if metadata, ok := mc.metadata[modelAddress]; ok {
		return metadata, true, nil
	}

	// 3. 提取模型元数据（输入/输出名称）
	fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] 调用 extractModelMetadata()...\n")
	metadata, err := extractModelMetadata(modelBytes)
	if err != nil {
		errMsg := err.Error()
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ extractModelMetadata() 失败\n")
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 原始错误信息: %q\n", errMsg)
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 原始错误信息长度: %d\n", len(errMsg))
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 原始错误信息是否包含'且初始化失败': %v\n", 
			strings.Contains(errMsg, "且初始化失败"))
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 原始错误信息是否包含'且': %v\n", 
			strings.Contains(errMsg, "且"))
		
		// 如果元数据提取失败，返回错误而不是使用默认值
		// 因为缺少 InputInfos 和 OutputInfos 会导致后续验证失败
		if logger != nil {
			logger.Errorf("提取ONNX模型元数据失败: %v", err)
		}
		wrappedErr := fmt.Errorf("提取ONNX模型元数据失败: %w", err)
		wrappedErrMsg := wrappedErr.Error()
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 包装后的错误信息: %q\n", wrappedErrMsg)
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 包装后的错误信息长度: %d\n", len(wrappedErrMsg))
		fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ❌ 包装后的错误信息是否包含'且初始化失败': %v\n", 
			strings.Contains(wrappedErrMsg, "且初始化失败"))
		return nil, false, wrappedErr
	}
	fmt.Fprintf(os.Stderr, "[TRACE GetOrLoadMetadata] ✅ extractModelMetadata() 成功\n")

	if logger != nil {
		logger.Debugf("ONNX模型元数据已提取并缓存 model=%s input_names=%v output_names=%v",
			modelAddress, metadata.InputNames, metadata.OutputNames)
	}

	// 4. 加入缓存
	mc.metadata[modelAddress] = metadata

	return metadata, false, nil
}

// Clear 清理所有缓存
func (mc *ModelCache) Clear() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metadata = make(map[string]*ModelMetadata)

	if mc.logger != nil {
		mc.logger.Info("ONNX模型元数据缓存已清空")
	}

	return nil
}

// Stats 获取缓存统计
func (mc *ModelCache) Stats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return map[string]interface{}{
		"cached_models": len(mc.metadata),
	}
}

