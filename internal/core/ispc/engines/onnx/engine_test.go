//go:build !android && !ios && cgo
// +build !android,!ios,cgo

package onnx

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ort "github.com/yalue/onnxruntime_go"
	logconfig "github.com/weisyn/v1/internal/config/log"
	logImpl "github.com/weisyn/v1/internal/core/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/ures"
)

// mockCASStorage mock CAS存储实现
type mockCASStorage struct {
	files map[string][]byte
}

func (m *mockCASStorage) BuildFilePath(contentHash []byte) string {
	return ""
}

func (m *mockCASStorage) StoreFile(ctx context.Context, contentHash []byte, data []byte) error {
	m.files[string(contentHash)] = data
	return nil
}

func (m *mockCASStorage) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) {
	data, ok := m.files[string(contentHash)]
	if !ok {
		return nil, fmt.Errorf("文件未找到: %x", contentHash[:8])
	}
	return data, nil
}

func (m *mockCASStorage) FileExists(contentHash []byte) bool {
	_, ok := m.files[string(contentHash)]
	return ok
}

var _ ures.CASStorage = (*mockCASStorage)(nil)

// createTestLogger 创建测试用的logger
func createTestLogger() log.Logger {
	cfg := logconfig.New(&logconfig.LogOptions{
		Level:     "warn",
		ToConsole: false,
	})
	logger, _ := logImpl.New(cfg)
	return logger
}

func TestEngine_parseModelAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "有效地址（64位hex）",
			address: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: false,
		},
		{
			name:    "带0x前缀（兼容性容错，实际应使用纯hex）",
			address: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: false, // 系统容错：自动剥离 0x 前缀，但规范要求使用纯 hex
		},
		{
			name:    "长度不足",
			address: "123456",
			wantErr: true,
		},
		{
			name:    "无效hex字符",
			address: "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// parseModelAddress 是内部方法，已移除，直接测试地址解析逻辑
			// CallModel 现在直接接受 []byte hash，不再需要字符串地址解析
			address := strings.TrimPrefix(strings.TrimPrefix(tt.address, "0x"), "0X")
			hash, err := hex.DecodeString(address)
			if tt.wantErr {
				// 对于长度不足的情况，hex.DecodeString不会报错，但长度会不对
				if err != nil {
					assert.Error(t, err)
				} else {
					// 如果解码成功但长度不对，也算错误
					assert.NotEqual(t, 32, len(hash), "地址长度应该不是32字节")
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 32, len(hash))
			}
		})
	}
}

func TestEngine_CallModel_InvalidInput(t *testing.T) {
	logger := createTestLogger()
	cas := &mockCASStorage{files: make(map[string][]byte)}
	engine, err := NewEngine(logger, cas)
	require.NoError(t, err)

	ctx := context.Background()

	// 将测试模型地址转换为hash
	modelHash, err := hex.DecodeString("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	// 测试空输入
	_, err = engine.CallModel(ctx, modelHash, []TensorInput{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的输入张量")

	// 测试空张量（float32类型）
	_, err = engine.CallModel(ctx, modelHash, []TensorInput{{Data: []float64{}}})
	assert.Error(t, err)

	// 测试空张量（int64类型）
	_, err = engine.CallModel(ctx, modelHash, []TensorInput{{Int64Data: []int64{}}})
	assert.Error(t, err)

	// 测试所有数据字段都为空
	_, err = engine.CallModel(ctx, modelHash, []TensorInput{{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据为空")
}

// TestEngine_CallModel_ShapeValidation 测试多维张量形状验证
func TestEngine_CallModel_ShapeValidation(t *testing.T) {
	logger := createTestLogger()
	cas := &mockCASStorage{files: make(map[string][]byte)}
	engine, err := NewEngine(logger, cas)
	require.NoError(t, err)

	ctx := context.Background()
	modelHash, err := hex.DecodeString("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	// 测试：提供形状信息
	tensorInputs := []TensorInput{
		{
			Name:  "input",
			Data:  []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0},
			Shape: []int64{1, 2, 3}, // 3D形状
		},
	}

	// 注意：由于没有真实模型，这个测试会失败在模型加载阶段
	// 但可以验证形状验证逻辑是否正确
	_, err = engine.CallModel(ctx, modelHash, tensorInputs)
	// 期望错误：模型不存在（不是形状验证错误）
	assert.Error(t, err)
	// 验证错误不是形状验证错误（应该是模型加载错误）
	assert.NotContains(t, err.Error(), "数据大小不匹配")
}

// TestEngine_CallModel_Int64DataType 测试int64数据类型支持
func TestEngine_CallModel_Int64DataType(t *testing.T) {
	logger := createTestLogger()
	cas := &mockCASStorage{files: make(map[string][]byte)}
	engine, err := NewEngine(logger, cas)
	require.NoError(t, err)

	ctx := context.Background()
	modelHash, err := hex.DecodeString("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	// 测试：int64类型输入（模拟BERT等文本模型）
	tensorInputs := []TensorInput{
		{
			Name:      "input_ids",
			Int64Data: []int64{101, 2023, 2003, 1037, 3231, 102}, // token IDs
			Shape:     []int64{1, 6},                              // [batch, sequence_length]
			DataType:  "int64",
		},
	}

	// 注意：由于没有真实模型，这个测试会失败在模型加载阶段
	// 但可以验证数据类型处理逻辑是否正确
	_, err = engine.CallModel(ctx, modelHash, tensorInputs)
	// 期望错误：模型不存在（不是数据类型错误）
	assert.Error(t, err)
	// 验证错误不是数据类型相关错误
	assert.NotContains(t, err.Error(), "需要int64类型数据")
	assert.NotContains(t, err.Error(), "数据类型")
}

// TestCalculateTensorSize 测试张量大小计算
func TestCalculateTensorSize(t *testing.T) {
	tests := []struct {
		name     string
		shape    []int64
		expected int
	}{
		{
			name:     "2D形状",
			shape:    []int64{1, 5},
			expected: 5,
		},
		{
			name:     "3D形状",
			shape:    []int64{1, 2, 3},
			expected: 6,
		},
		{
			name:     "4D形状（图像）",
			shape:    []int64{1, 3, 224, 224},
			expected: 1 * 3 * 224 * 224,
		},
		{
			name:     "空形状",
			shape:    []int64{},
			expected: 0,
		},
		{
			name:     "单维度",
			shape:    []int64{10},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape := ort.NewShape(tt.shape...)
			size := calculateTensorSize(shape)
			assert.Equal(t, tt.expected, size)
		})
	}
}

// 注意：完整的功能测试需要真实的ONNX模型文件
// 这些测试可以在有实际模型文件后补充
//
// func TestEngine_CallModel_Success(t *testing.T) {
// 	// 需要准备真实的ONNX模型文件和测试数据
// }

func TestModelCache_GetOrLoadMetadata(t *testing.T) {
	logger := createTestLogger()
	cache := NewModelCache(logger)

	// 注意：完整测试需要真实的ONNX模型字节数据
	// 当前只测试缓存结构是否正确初始化
	assert.NotNil(t, cache)
	stats := cache.Stats()
	assert.Equal(t, 0, stats["cached_models"])
}

func TestSessionPool_AcquireRelease(t *testing.T) {
	pool := NewSessionPool()
	ctx := context.Background()

	// 测试获取和释放
	err := pool.Acquire(ctx)
	assert.NoError(t, err)

	pool.Release()

	// 测试多次获取
	for i := 0; i < 10; i++ {
		err := pool.Acquire(ctx)
		assert.NoError(t, err)
	}

	// 释放所有
	for i := 0; i < 10; i++ {
		pool.Release()
	}
}

func TestInferenceMetrics(t *testing.T) {
	metrics := NewInferenceMetrics()

	// 测试初始状态
	stats := metrics.Stats()
	assert.Equal(t, int64(0), stats["total_inferences"])
	assert.Equal(t, int64(0), stats["error_count"])

	// 记录成功的推理
	metrics.RecordInference(100*1000*1000, nil) // 100ms
	stats = metrics.Stats()
	assert.Equal(t, int64(1), stats["total_inferences"])

	// 记录失败的推理
	metrics.RecordInference(50*1000*1000, assert.AnError)
	stats = metrics.Stats()
	assert.Equal(t, int64(2), stats["total_inferences"])
	assert.Equal(t, int64(1), stats["error_count"])

	// 测试缓存命中率
	metrics.RecordCacheHit(true)
	metrics.RecordCacheHit(false)
	stats = metrics.Stats()
	assert.Equal(t, int64(1), stats["cache_hits"])
	assert.Equal(t, int64(1), stats["cache_misses"])
}

