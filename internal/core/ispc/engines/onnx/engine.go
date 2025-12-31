//go:build !android && !ios && cgo
// +build !android,!ios,cgo

package onnx

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	onnxdeps "github.com/weisyn/v1/pkg/build/deps/onnx"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/ures"
	ort "github.com/yalue/onnxruntime_go"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// TensorInput 张量输入（支持多维张量）
//
// 🎯 **设计目的**：
// - 支持多维张量输入（如 [1, 3, 224, 224]）
// - 提供形状信息，确保与模型要求匹配
// - 支持未来扩展（数据类型等）
//
// 📋 **字段说明**：
//   - Name: 输入名称（可选，按顺序匹配时可为空）
//   - Data: 展平的数据（float64数组）
//   - Shape: 形状信息（如 [1, 3, 224, 224]）
//
// TensorInput 类型别名，使用interfaces包中的定义
type TensorInput = ispcInterfaces.TensorInput

// Engine ONNX推理引擎核心实现
//
// 🎯 **核心职责**：
// - 集成ONNX Runtime进行模型推理
// - 管理模型会话缓存
// - 处理张量转换
// - 错误处理和日志记录
// - 推理性能监控
type Engine struct {
	logger      log.Logger
	casStorage  ures.CASStorage   // 内容寻址存储（用于加载模型文件）
	modelCache  *ModelCache       // 模型会话缓存
	sessionPool *SessionPool      // 会话池（并发控制）
	memoryPool  *TensorMemoryPool // 张量内存池
	metrics     *InferenceMetrics // 推理监控指标
	once        sync.Once         // 确保ONNX Runtime只初始化一次
	initDone    bool              // 标记初始化是否完成（用于双重检查）
	initErr     error             // 初始化错误（如果初始化失败，记录错误以便后续检查）
	initMutex   sync.RWMutex      // 保护 initDone 和 initErr 的并发访问
}

// float32ToFloat16 将 IEEE754 float32 转换为 IEEE754 binary16 (float16) 的 16 位编码。
//
// ⚠️ 说明：
// - 这是一个独立实现，用于在不引入额外依赖的情况下支持 float16 张量
// - 实现参考 IEEE754 标准，覆盖 Inf / NaN / 正常数 / 次正规数 / 下溢 场景
// - 返回值为 uint16，小端序写入时低字节在前
func float32ToFloat16(f float32) uint16 {
	bits := math.Float32bits(f)

	sign := uint16((bits >> 16) & 0x8000) // 符号位
	exp := int32((bits >> 23) & 0xff)     // 指数部分（8 位）
	mantissa := bits & 0x7fffff           // 尾数部分（23 位）

	switch exp {
	case 0:
		// 零或次正规数：直接下溢为 0（对本项目测试场景足够）
		if mantissa == 0 {
			return sign
		}
		// 将 subnormal 近似为 0
		return sign
	case 0xff:
		// Inf 或 NaN
		if mantissa == 0 {
			// ±Inf
			return sign | 0x7c00
		}
		// NaN：保留一个 quiet NaN 模式
		return sign | 0x7e00
	}

	// 规格化数：重新偏移指数
	exp32 := exp - 127  // 去掉 float32 偏移
	exp16 := exp32 + 15 // 应用 float16 偏移

	if exp16 >= 0x1f {
		// 溢出：映射为无穷大
		return sign | 0x7c00
	}

	if exp16 <= 0 {
		// 下溢到次正规数或 0
		if exp16 < -10 {
			// 太小，直接视为 0
			return sign
		}

		// 生成次正规数（保留部分精度）
		// 将隐含的最高位 1 加回 mantissa
		mant32 := mantissa | 0x00800000
		shift := uint32(1 - exp16 + 13) // 23 - 10 = 13
		halfMant := uint16(mant32 >> shift)

		// 简单舍入：查看被移除位的最高位
		if (mant32>>(shift-1))&1 == 1 {
			halfMant++
		}

		return sign | halfMant
	}

	// 正常范围的规格化数
	halfExp := uint16(exp16) << 10
	halfMant := uint16(mantissa >> 13) // 保留 10 位小数

	// 四舍五入：检查第 11 位
	if (mantissa>>12)&1 == 1 {
		halfMant++
		if halfMant&0x03ff == 0 {
			// 尾数进位导致指数 +1
			halfExp += 0x0400
			if halfExp >= 0x7c00 {
				// 溢出为 Inf
				halfExp = 0x7c00
				halfMant = 0
			}
		}
	}

	return sign | halfExp | (halfMant & 0x03ff)
}

// NewEngine 创建ONNX推理引擎
func NewEngine(logger log.Logger, casStorage ures.CASStorage) (*Engine, error) {
	return &Engine{
		logger:      logger,
		casStorage:  casStorage,
		modelCache:  NewModelCache(logger),
		sessionPool: NewSessionPool(),
		memoryPool:  NewTensorMemoryPool(),
		metrics:     NewInferenceMetrics(),
	}, nil
}

// initializeONNXRuntime 初始化ONNX Runtime环境
//
// 使用嵌入的ONNX Runtime库文件，无需用户手动安装依赖。
// 库文件在构建时通过 go generate 自动下载并嵌入到二进制中。
func (e *Engine) initializeONNXRuntime() error {
	fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] 开始初始化ONNX Runtime\n")

	// 先检查是否已经初始化成功
	e.initMutex.RLock()
	initDone := e.initDone
	initErr := e.initErr
	e.initMutex.RUnlock()

	if initDone {
		// 已经初始化成功，直接返回
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ 已初始化，直接返回\n")
		return nil
	}

	if initErr != nil {
		// 之前初始化失败，返回错误（sync.Once 不会再次执行）
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 之前初始化失败，返回错误: %q\n", initErr.Error())
		return initErr
	}

	// 执行初始化（sync.Once 确保只执行一次）
	var doInitErr error
	e.once.Do(func() {
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] sync.Once 执行，调用 LoadEmbeddedLibrary()\n")

		// 优先使用嵌入的库文件
		if err := onnxdeps.LoadEmbeddedLibrary(); err != nil {
			// 如果嵌入的库加载失败，记录错误
			errMsg := fmt.Sprintf("加载嵌入的ONNX Runtime库失败: %v", err)
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ LoadEmbeddedLibrary() 失败: %s\n", errMsg)
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 错误信息完整内容: %q\n", err.Error())
			if e.logger != nil {
				e.logger.Warnf("加载嵌入的ONNX Runtime库失败: %v", err)
			}
			doInitErr = fmt.Errorf(
				"ONNX Runtime初始化失败。\n"+
					"这通常是因为构建时未运行 go generate 下载库文件。\n"+
					"解决方法：\n"+
					"  1. 运行: go generate ./pkg/build/deps/onnx\n"+
					"  2. 然后重新构建: go build\n"+
					"原始错误: %w", err)
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 包装后的错误信息: %q\n", doInitErr.Error())

			// 记录初始化失败
			e.initMutex.Lock()
			e.initErr = doInitErr
			e.initMutex.Unlock()
			return
		}
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ LoadEmbeddedLibrary() 成功\n")

		// ⚠️ 关键：验证初始化是否真正成功
		// LoadEmbeddedLibrary() 可能返回成功，但 InitializeEnvironment() 可能失败
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] 检查 IsInitialized()...\n")
		if !ort.IsInitialized() {
			doInitErr = fmt.Errorf("ONNX Runtime初始化失败：LoadEmbeddedLibrary() 成功但 IsInitialized() 返回 false")
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ IsInitialized() = false\n")
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 错误信息: %q\n", doInitErr.Error())
			if e.logger != nil {
				e.logger.Errorf("ONNX Runtime初始化验证失败: IsInitialized() = false")
			}

			// 记录初始化失败
			e.initMutex.Lock()
			e.initErr = doInitErr
			e.initMutex.Unlock()
			return
		}
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ IsInitialized() = true\n")

		// 标记初始化成功
		e.initMutex.Lock()
		e.initDone = true
		e.initErr = nil
		e.initMutex.Unlock()

		if e.logger != nil {
			e.logger.Info("✅ ONNX Runtime环境初始化成功（使用嵌入的库文件）")
		}
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ sync.Once 内部初始化成功\n")
	})

	// 检查初始化结果
	e.initMutex.RLock()
	initDone = e.initDone
	initErr = e.initErr
	e.initMutex.RUnlock()

	// 如果初始化失败，返回错误
	if initErr != nil {
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 返回错误: %q\n", initErr.Error())
		return initErr
	}

	// 双重检查：确保初始化真正成功
	fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] 双重检查: initDone=%v, IsInitialized()=%v\n", initDone, ort.IsInitialized())
	if !initDone {
		// sync.Once 已执行但 initDone 仍为 false，说明初始化失败
		err := fmt.Errorf("ONNX Runtime初始化失败：sync.Once 已执行但初始化未完成")
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 双重检查失败: %q\n", err.Error())
		return err
	}

	// 如果 sync.Once 已执行但 IsInitialized() 返回 false，可能是状态被破坏
	// 尝试重新设置库路径并重新初始化
	if !ort.IsInitialized() {
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ⚠️ sync.Once已执行但IsInitialized()=false，尝试恢复...\n")
		// 重新加载库路径（可能状态被破坏）
		if err := onnxdeps.LoadEmbeddedLibrary(); err != nil {
			errMsg := fmt.Errorf("ONNX Runtime初始化失败：IsInitialized() 返回 false，且恢复尝试失败: %w", err)
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 恢复失败: %q\n", errMsg.Error())
			return errMsg
		}
		// 再次检查
		if !ort.IsInitialized() {
			err := fmt.Errorf("ONNX Runtime初始化失败：IsInitialized() 返回 false（状态可能被破坏）")
			fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ❌ 恢复后仍失败: %q\n", err.Error())
			return err
		}
		fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ 恢复成功，IsInitialized() = true\n")
	}

	fmt.Fprintf(os.Stderr, "[TRACE initializeONNXRuntime] ✅ 初始化成功，返回 nil\n")
	return nil
}

// CallModel 执行ONNX模型推理（支持多维张量输入）
//
// 🎯 **统一接口**：使用TensorInput格式，支持多维张量
//
// 实现InternalONNXEngine接口
//
// 实现流程：
// 1. 参数验证
// 2. 初始化ONNX Runtime（如果尚未初始化）
// 3. 从CAS存储加载模型文件
// 4. 获取模型元数据（带缓存）
// 5. 预处理输入张量
// 6. 创建输出张量（处理动态形状）
// 7. 创建DynamicAdvancedSession并执行推理
// 8. 后处理输出张量
//
// 📌 **设计决策**：
// - 统一使用 DynamicAdvancedSession（而非 AdvancedSession）
// - 原因：DynamicAdvancedSession 支持固定和动态输出形状，与官方库保持一致
// - 优势：简化实现逻辑，避免条件分支，降低测试复杂度
//
// 参数：
//   - ctx: 执行上下文
//   - modelHash: 模型内容哈希（32字节）
//   - tensorInputs: 张量输入列表（包含数据和形状信息）
//
// 返回值：
//   - [][]float64: 推理结果
//   - error: 推理错误
//
// 使用示例：
//
//	tensorInputs := []TensorInput{
//	    {
//	        Name:  "input",
//	        Data:  []float64{0.1, 0.2, ...}, // 展平的图像数据
//	        Shape: []int64{1, 3, 224, 224},   // 4D形状
//	    },
//	}
//	outputs, err := engine.CallModel(ctx, modelHash, tensorInputs)
func (e *Engine) CallModel(
	ctx context.Context,
	modelHash []byte,
	tensorInputs []TensorInput,
) ([]ispcInterfaces.TensorOutput, error) {
	// 将hash转换为hex string（供后续使用）
	modelAddress := hex.EncodeToString(modelHash)
	startTime := time.Now()
	var inferenceErr error
	defer func() {
		if inferenceErr == nil {
			duration := time.Since(startTime)
			e.metrics.RecordInference(duration, nil)
		}
	}()

	// 1. 参数验证
	if len(tensorInputs) == 0 {
		inferenceErr = ErrInvalidInput
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, ErrInvalidInput)
	}

	for i, tensorInput := range tensorInputs {
		// 验证至少有一个数据字段不为空
		hasData := len(tensorInput.Data) > 0 || len(tensorInput.Int64Data) > 0 || len(tensorInput.Int32Data) > 0 || len(tensorInput.Int16Data) > 0 || len(tensorInput.Uint8Data) > 0
		if !hasData {
			inferenceErr = fmt.Errorf("输入张量[%d]数据为空（需要提供Data、Int64Data、Int32Data、Int16Data或Uint8Data）", i)
			duration := time.Since(startTime)
			e.metrics.RecordInference(duration, inferenceErr)
			return nil, WrapError("CallModel", modelAddress, inferenceErr)
		}
		// 添加调试日志：检查每个数据字段（用于诊断问题）
		if e.logger != nil {
			e.logger.Debugf("输入张量[%d]数据检查: Data=%d, Int64Data=%d, Int32Data=%d, Int16Data=%d, Uint8Data=%d",
				i, len(tensorInput.Data), len(tensorInput.Int64Data), len(tensorInput.Int32Data), len(tensorInput.Int16Data), len(tensorInput.Uint8Data))
		}
	}

	// 2. 初始化ONNX Runtime
	fmt.Fprintf(os.Stderr, "[TRACE CallModel] 调用 initializeONNXRuntime()...\n")
	if err := e.initializeONNXRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "[TRACE CallModel] ❌ initializeONNXRuntime() 失败: %q\n", err.Error())
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		wrappedErr := WrapError("CallModel", modelAddress, err)
		fmt.Fprintf(os.Stderr, "[TRACE CallModel] ❌ 包装后的错误: %q\n", wrappedErr.Error())
		return nil, wrappedErr
	}
	fmt.Fprintf(os.Stderr, "[TRACE CallModel] ✅ initializeONNXRuntime() 成功\n")

	// 3. 加载模型文件（从CAS存储）
	modelBytes, err := e.loadModelFromCAS(ctx, modelHash)
	if err != nil {
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, err)
	}

	// 4. 获取模型元数据（带缓存）
	fmt.Fprintf(os.Stderr, "[TRACE CallModel] 调用 GetOrLoadMetadata()...\n")
	metadata, cacheHit, err := e.modelCache.GetOrLoadMetadata(ctx, modelAddress, modelBytes, e.logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[TRACE CallModel] ❌ GetOrLoadMetadata() 失败: %q\n", err.Error())
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		wrappedErr := WrapError("CallModel", modelAddress, err)
		fmt.Fprintf(os.Stderr, "[TRACE CallModel] ❌ 包装后的错误: %q\n", wrappedErr.Error())
		return nil, wrappedErr
	}
	fmt.Fprintf(os.Stderr, "[TRACE CallModel] ✅ GetOrLoadMetadata() 成功\n")
	e.metrics.RecordCacheHit(cacheHit)

	// 5. 获取推理执行权限（并发控制）
	if err := e.sessionPool.Acquire(ctx); err != nil {
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, err)
	}
	defer e.sessionPool.Release()

	// 6. 预处理输入张量（TensorInput -> ONNX张量）
	onnxInputs, err := e.preprocessInputsFromTensors(tensorInputs, metadata.InputNames, metadata.InputInfos)
	if err != nil {
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, err)
	}
	defer e.releaseInputs(onnxInputs)

	// 7. 准备输出占位（统一交给 ONNX Runtime 自动分配）
	// ⚠️ 统一策略：**所有输出一律传递 nil**
	//    由 onnxruntime_go / ONNX Runtime 根据模型元数据自动分配正确类型与形状。
	//    这样可以避免 float16 / bfloat16 等特殊类型在预分配阶段出现类型不匹配。
	// 📚 官方参考：onnxruntime_test.go:1409 (sklearn_randomforest 示例)
	//    outputs := []Value{nil, nil}
	onnxOutputs := make([]ort.Value, len(metadata.OutputInfos))
	for i, info := range metadata.OutputInfos {
		// 添加调试日志（同时输出到 stderr 和 logger），但不再在这里预分配具体张量类型
		typeValue := int(info.OrtValueType)
		fmt.Fprintf(os.Stderr, "[TRACE CallModel] 输出[%d](%s): OrtValueType=%v(值=%d), DataType=%v —— 统一传递 nil 由 ONNX Runtime 自动分配\n",
			i, info.Name, info.OrtValueType, typeValue, info.DataType)
		if e.logger != nil {
			e.logger.Infof("准备输出[%d](%s): OrtValueType=%v(值=%d), DataType=%v —— 统一传递 nil 由 ONNX Runtime 自动分配",
				i, info.Name, info.OrtValueType, typeValue, info.DataType)
		}
		onnxOutputs[i] = nil
	}
	defer e.releaseOutputs(onnxOutputs)

	// 8. 统一使用 DynamicAdvancedSession（支持固定和动态输出形状）
	// 与官方库保持一致，简化实现逻辑，避免条件分支
	session, err := ort.NewDynamicAdvancedSessionWithONNXData(
		modelBytes,
		metadata.InputNames,
		metadata.OutputNames,
		nil, // SessionOptions
	)
	if err != nil {
		inferenceErr = fmt.Errorf("创建ONNX会话失败: %w", err)
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, inferenceErr)
	}
	defer session.Destroy()

	// 9. 执行推理
	if err := session.Run(onnxInputs, onnxOutputs); err != nil {
		inferenceErr = fmt.Errorf("推理执行失败: %w", err)
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, inferenceErr)
	}

	// 10. 后处理输出张量（ONNX张量 -> []TensorOutput）
	outputs, err := e.postprocessOutputs(onnxOutputs, metadata.OutputInfos)
	if err != nil {
		inferenceErr = err
		duration := time.Since(startTime)
		e.metrics.RecordInference(duration, inferenceErr)
		return nil, WrapError("CallModel", modelAddress, err)
	}

	duration := time.Since(startTime)
	if e.logger != nil {
		totalValues := 0
		for _, out := range outputs {
			totalValues += len(out.Values)
		}
		e.logger.Debugf("ONNX推理完成 model=%s latency_ms=%d outputs=%d total_values=%d",
			modelAddress,
			duration.Milliseconds(),
			len(outputs),
			totalValues,
		)
	}

	return outputs, nil
}

// 确保Engine实现InternalONNXEngine接口
var _ ispcInterfaces.InternalONNXEngine = (*Engine)(nil)

// parseModelAddress 解析模型标识为内容哈希
//
// 🎯 **标准化标识解析**：
// 严格要求64位十六进制字符串（32字节哈希），不允许0x前缀
//
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// - 参数名虽为 "address"，但实际语义是 ResourceCodeId（内容哈希），属于对象标识命名空间
// - 不是"账户地址"（Address），而是"资源代码标识"（ResourceCodeId）
// - 对外表示：64位hex字符串（不带0x），符合承诺类哈希命名空间的展示规范
//
// 参数：
//   - address: 模型标识字符串（实际为 ResourceCodeId 的 hex 表示）
//
// 返回：
//   - []byte: 解析后的内容哈希（32字节）
//   - error: 解析错误
func (e *Engine) parseModelAddress(address string) ([]byte, error) {
	// 移除可能的空白字符
	address = strings.TrimSpace(address)

	// 严格拒绝 0x 前缀（ETH地址格式）
	// ⚠️ 符合标识协议：共识层只认原始 bytes，0x 前缀属于 UI 层，不应出现在协议层
	if len(address) >= 2 && (address[:2] == "0x" || address[:2] == "0X") {
		return nil, fmt.Errorf("模型标识不允许0x前缀，请使用纯十六进制字符串: %s（注意：这是资源代码标识 ResourceCodeId，不是账户地址）", address)
	}

	// 验证长度（32字节 = 64个十六进制字符）
	if len(address) != 64 {
		return nil, fmt.Errorf("模型地址长度必须为64位十六进制字符，实际长度: %d", len(address))
	}

	// 解析为字节数组
	contentHash, err := hex.DecodeString(address)
	if err != nil {
		return nil, fmt.Errorf("模型地址必须是有效的十六进制字符串: %w", err)
	}

	// 再次验证解析后的长度
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("解析后的哈希长度必须为32字节，实际: %d", len(contentHash))
	}

	return contentHash, nil
}

// loadModelFromCAS 从CAS存储加载ONNX模型文件
func (e *Engine) loadModelFromCAS(ctx context.Context, contentHash []byte) ([]byte, error) {
	if e.casStorage == nil {
		return nil, fmt.Errorf("CAS存储未初始化")
	}

	// 从CAS存储读取文件
	modelBytes, err := e.casStorage.ReadFile(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("从CAS存储加载模型失败: %w", err)
	}

	if len(modelBytes) == 0 {
		return nil, fmt.Errorf("模型文件为空")
	}

	if e.logger != nil {
		e.logger.Debugf("从CAS存储加载ONNX模型成功 hash=%x size=%d", contentHash[:8], len(modelBytes))
	}

	return modelBytes, nil
}

// preprocessInputsFromTensors 将TensorInput转换为ONNX张量
//
// 🎯 **改进**：支持多维张量输入，使用模型元数据或用户提供的形状信息
//
// 参数：
//   - tensorInputs: 张量输入列表（包含数据和形状信息）
//   - inputNames: 输入名称列表
//   - inputInfos: 输入信息（包含形状、类型等元数据）
//
// 返回：
//   - []ort.Value: ONNX张量列表
//   - error: 处理错误
func (e *Engine) preprocessInputsFromTensors(
	tensorInputs []TensorInput,
	inputNames []string,
	inputInfos []ort.InputOutputInfo,
) ([]ort.Value, error) {
	// 验证输入数量匹配
	if len(tensorInputs) != len(inputNames) {
		return nil, fmt.Errorf("输入张量数量(%d)与模型输入名称数量(%d)不匹配",
			len(tensorInputs), len(inputNames))
	}
	if len(tensorInputs) != len(inputInfos) {
		return nil, fmt.Errorf("输入张量数量(%d)与模型输入信息数量(%d)不匹配",
			len(tensorInputs), len(inputInfos))
	}

	onnxInputs := make([]ort.Value, 0, len(tensorInputs))

	for i, tensorInput := range tensorInputs {
		info := inputInfos[i]
		inputName := inputNames[i]

		// 添加详细日志追踪 Shape 解析
		if e.logger != nil {
			e.logger.Infof("处理输入[%d](%s): tensorInput.Shape=%v (len=%d), info.Dimensions=%v (len=%d)",
				i, inputName, tensorInput.Shape, len(tensorInput.Shape), info.Dimensions, len(info.Dimensions))
		}

		// 确定输入形状：优先级 用户提供 > 模型元数据 > 默认推断
		var shape ort.Shape
		var dataLength int
		// 确定数据长度（根据数据类型）
		if len(tensorInput.Data) > 0 {
			dataLength = len(tensorInput.Data)
		} else if len(tensorInput.Int64Data) > 0 {
			dataLength = len(tensorInput.Int64Data)
		} else if len(tensorInput.Int32Data) > 0 {
			dataLength = len(tensorInput.Int32Data)
		} else if len(tensorInput.Int16Data) > 0 {
			dataLength = len(tensorInput.Int16Data)
		} else if len(tensorInput.Uint8Data) > 0 {
			dataLength = len(tensorInput.Uint8Data)
		}

		if len(tensorInput.Shape) > 0 {
			// ✅ 优先使用用户提供的形状（支持多维张量）
			shape = ort.NewShape(tensorInput.Shape...)
			if e.logger != nil {
				e.logger.Infof("输入[%d](%s)使用用户提供的形状: %v", i, inputName, shape)
			}
		} else if len(info.Dimensions) > 0 {
			// ✅ 使用模型元数据中的实际形状（支持多维张量）
			shape = info.Dimensions
			if e.logger != nil {
				e.logger.Infof("输入[%d](%s)使用模型元数据形状: %v", i, inputName, shape)
			}
		} else {
			// 回退：如果没有形状信息，使用 [1, N]
			if e.logger != nil {
				e.logger.Warnf("输入[%d](%s)没有形状信息，使用默认形状[1, %d]",
					i, inputName, dataLength)
			}
			shape = ort.NewShape(1, int64(dataLength))
		}

		// 确定数据类型：优先级 用户指定 > 模型元数据
		//
		// 📚 **官方实现参考** (github.com/yalue/onnxruntime_go@v1.22.0):
		// - tensor_type_constraints.go: IntData 接口定义包含 ~int32 | ~int16 | ~int64 等
		// - onnxruntime_test.go:396: 使用 NewTensor(shape, []int32{...}) 创建 int32 输入
		// - onnxruntime_test.go:572: 使用 NewEmptyTensor[int16](shape) 创建 int16 输出
		// - onnxruntime_test.go:1161: float16 使用 NewCustomDataTensor(shape, []byte{...}, TensorElementDataTypeFloat16)
		//
		dataType := info.DataType
		if tensorInput.DataType != "" {
			// 用户指定了数据类型，转换为ort类型
			switch tensorInput.DataType {
			case "float32", "float":
				dataType = ort.TensorElementDataTypeFloat
			case "float64", "double":
				dataType = ort.TensorElementDataTypeDouble
			case "int64":
				dataType = ort.TensorElementDataTypeInt64
			case "int32":
				// ✅ onnxruntime_go 完全支持 int32，直接使用
				// 📚 官方参考: onnxruntime_test.go:396-397
				//    inputData := []int32{12, 21}
				//    input, e := NewTensor(NewShape(1, 2), inputData)
				dataType = ort.TensorElementDataTypeInt32
			case "int16":
				// ✅ onnxruntime_go 完全支持 int16，直接使用
				// 📚 官方参考: onnxruntime_test.go:572
				//    outputA := newTestTensor[int16](t, NewShape(1, 2, 2))
				//    其中 newTestTensor[int16] 内部调用 NewEmptyTensor[int16](shape)
				dataType = ort.TensorElementDataTypeInt16
			case "uint8":
				dataType = ort.TensorElementDataTypeUint8
			case "float16":
				// ⚠️ float16 需要使用 NewCustomDataTensor（Go 没有原生 float16 类型）
				// 📚 官方参考: onnxruntime_test.go:1161-1162
				//    inputTensor, e := NewCustomDataTensor(NewShape(1, 2, 2, 2), inputData,
				//        TensorElementDataTypeFloat16)
				//    其中 inputData 是 []byte 类型（字节格式）
				dataType = ort.TensorElementDataTypeFloat16
			case "bfloat16":
				// ⚠️ bfloat16 需要使用 NewCustomDataTensor（Go 没有原生 bfloat16 类型）
				// 📚 官方参考: onnxruntime_test.go:1167-1168
				//    outputTensor, e := NewCustomDataTensor(NewShape(1, 2, 2, 2), outputData,
				//        TensorElementDataTypeBFloat16)
				dataType = ort.TensorElementDataTypeBFloat16
			default:
				// 如果无法识别，使用模型元数据中的类型
				if e.logger != nil {
					e.logger.Warnf("输入[%d](%s)数据类型%s无法识别，使用模型元数据类型", i, inputName, tensorInput.DataType)
				}
			}
		}

		// 计算期望的数据大小
		expectedSize := calculateTensorSize(shape)
		if expectedSize < 0 {
			// 动态维度暂不支持
			for _, t := range onnxInputs {
				t.Destroy()
			}
			return nil, fmt.Errorf("输入[%d](%s)包含动态维度，暂不支持", i, inputName)
		}

		// 根据数据类型创建对应的ONNX张量
		var onnxTensor ort.Value
		var err error

		switch dataType {
		case ort.TensorElementDataTypeFloat:
			// float32类型：使用Data字段
			if len(tensorInput.Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要float32类型数据，但Data为空", i, inputName)
			}
			if len(tensorInput.Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际数据大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Data),
				)
			}
			// 转换为float32
			data := make([]float32, len(tensorInput.Data))
			for j, val := range tensorInput.Data {
				data[j] = float32(val)
			}
			onnxTensor, err = ort.NewTensor(shape, data)

		case ort.TensorElementDataTypeInt64:
			// int64类型：使用Int64Data字段
			if len(tensorInput.Int64Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Int64Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Int64Data),
				)
			}
			onnxTensor, err = ort.NewTensor(shape, tensorInput.Int64Data)

		case ort.TensorElementDataTypeInt32:
			// int32类型：使用Int32Data字段（onnxruntime_go 完全支持）
			// 📚 官方实现参考: onnxruntime_test.go:396-397
			//    inputData := []int32{12, 21}
			//    input, e := NewTensor(NewShape(1, 2), inputData)
			//    直接使用 []int32 创建 *Tensor[int32]，无需类型转换
			if len(tensorInput.Int32Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要int32类型数据，但Int32Data为空", i, inputName)
			}
			if len(tensorInput.Int32Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Int32Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Int32Data),
				)
			}
			onnxTensor, err = ort.NewTensor(shape, tensorInput.Int32Data)

		case ort.TensorElementDataTypeInt16:
			// int16类型：使用Int16Data字段（onnxruntime_go 完全支持）
			// 📚 官方实现参考: onnxruntime_test.go:572
			//    outputA := newTestTensor[int16](t, NewShape(1, 2, 2))
			//    其中 newTestTensor[int16] 内部调用 NewEmptyTensor[int16](shape)
			//    对于输入，使用 NewTensor(shape, []int16{...}) 创建 *Tensor[int16]
			if len(tensorInput.Int16Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Int16Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Int16Data),
				)
			}
			onnxTensor, err = ort.NewTensor(shape, tensorInput.Int16Data)

		case ort.TensorElementDataTypeUint8:
			// uint8类型：使用Uint8Data字段
			if len(tensorInput.Uint8Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要uint8类型数据，但Uint8Data为空", i, inputName)
			}
			if len(tensorInput.Uint8Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Uint8Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Uint8Data),
				)
			}
			onnxTensor, err = ort.NewTensor(shape, tensorInput.Uint8Data)

		case ort.TensorElementDataTypeDouble:
			// float64/double类型：使用Data字段，转换为float64数组
			if len(tensorInput.Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要float64类型数据，但Data为空", i, inputName)
			}
			if len(tensorInput.Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际数据大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Data),
				)
			}
			// 转换为float64数组（onnxruntime_go 支持 float64）
			// 注意：tensorInput.Data 已经是 []float64，直接使用即可
			onnxTensor, err = ort.NewTensor(shape, tensorInput.Data)

		case ort.TensorElementDataTypeBFloat16:
			// bfloat16 类型：使用 Data 字段（float64）作为近似的 float32 来源，转换为 bfloat16 字节格式
			//
			// 📚 官方实现参考: onnxruntime_test.go:1167-1168
			// 使用 NewCustomDataTensor(shape, []byte{...}, TensorElementDataTypeBFloat16)
			if len(tensorInput.Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要bfloat16类型数据，但Data为空", i, inputName)
			}
			if len(tensorInput.Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Data),
				)
			}

			// 将 float64 → float32 → bfloat16（取 IEEE754 float32 高 16 位）并按小端序写入字节切片
			bfBytes := make([]byte, expectedSize*2)
			for idx, v := range tensorInput.Data {
				f32 := float32(v)
				bits := math.Float32bits(f32)
				bf := uint16(bits >> 16) // bfloat16 使用 float32 的高 16 位
				// 小端序写入
				bfBytes[2*idx] = byte(bf)
				bfBytes[2*idx+1] = byte(bf >> 8)
			}

			onnxTensor, err = ort.NewCustomDataTensor(shape, bfBytes, ort.TensorElementDataTypeBFloat16)

		case ort.TensorElementDataTypeFloat16:
			// float16 类型：使用 Data 字段（float64）作为近似来源，转换为 IEEE754 binary16 字节格式
			//
			// 📚 官方实现参考: onnxruntime_test.go:1161-1162
			// 使用 NewCustomDataTensor(shape, []byte{...}, TensorElementDataTypeFloat16)
			if len(tensorInput.Data) == 0 {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf("输入[%d](%s)需要float16类型数据，但Data为空", i, inputName)
			}
			if len(tensorInput.Data) != expectedSize {
				for _, t := range onnxInputs {
					t.Destroy()
				}
				return nil, fmt.Errorf(
					"输入[%d](%s)数据大小不匹配: 期望形状%v(大小%d), 实际Data大小%d",
					i, inputName, shape, expectedSize, len(tensorInput.Data),
				)
			}

			// 将 float64 → float32 → float16（IEEE754 binary16 编码）并按小端序写入字节切片
			halfBytes := make([]byte, expectedSize*2)
			for idx, v := range tensorInput.Data {
				f32 := float32(v)
				h := float32ToFloat16(f32)
				// 小端序写入
				halfBytes[2*idx] = byte(h)
				halfBytes[2*idx+1] = byte(h >> 8)
			}

			onnxTensor, err = ort.NewCustomDataTensor(shape, halfBytes, ort.TensorElementDataTypeFloat16)

		default:
			// 不支持的数据类型
			for _, t := range onnxInputs {
				t.Destroy()
			}
			return nil, fmt.Errorf("输入[%d](%s)数据类型%v暂不支持", i, inputName, dataType)
		}

		if err != nil {
			// 清理已创建的张量
			for _, t := range onnxInputs {
				t.Destroy()
			}
			return nil, fmt.Errorf("创建ONNX输入张量[%d](%s)失败: 形状%v, 类型%v, 错误: %w",
				i, inputName, shape, dataType, err)
		}

		onnxInputs = append(onnxInputs, onnxTensor)
	}

	return onnxInputs, nil
}

// calculateTensorSize 计算张量的总元素数
//
// 参数：
//   - shape: 张量形状（如 [1, 3, 224, 224]）
//
// 返回：
//   - int: 总元素数
func calculateTensorSize(shape ort.Shape) int {
	if len(shape) == 0 {
		return 0
	}
	size := 1
	for _, dim := range shape {
		if dim <= 0 {
			// 动态维度（-1）暂不支持，需要明确指定
			return -1
		}
		size *= int(dim)
	}
	return size
}

// inferOutputShape 推断动态输出形状
// 对于包含 -1 的维度，使用输入形状的第一个维度或合理的默认值
func (e *Engine) inferOutputShape(outputInfo ort.InputOutputInfo, onnxInputs []ort.Value) ort.Shape {
	shape := make(ort.Shape, len(outputInfo.Dimensions))

	// 获取第一个输入的形状（用于推断动态维度）
	var firstInputShape ort.Shape
	if len(onnxInputs) > 0 {
		if tensor, ok := onnxInputs[0].(*ort.Tensor[float32]); ok {
			firstInputShape = tensor.GetShape()
		} else if tensor, ok := onnxInputs[0].(*ort.Tensor[int64]); ok {
			firstInputShape = tensor.GetShape()
		} else if tensor, ok := onnxInputs[0].(*ort.Tensor[uint8]); ok {
			firstInputShape = tensor.GetShape()
		}
	}

	if e.logger != nil {
		e.logger.Infof("推断动态输出形状: 输出维度=%v, 输入形状=%v", outputInfo.Dimensions, firstInputShape)
	}

	for i, dim := range outputInfo.Dimensions {
		if dim <= 0 {
			// 动态维度：尝试从输入形状推断，或使用默认值
			// 对于大多数ONNX模型，动态维度通常是批次维度（第0维），使用输入的第0维
			if len(firstInputShape) > 0 && i < len(firstInputShape) {
				shape[i] = firstInputShape[i]
				if e.logger != nil {
					e.logger.Infof("  维度[%d]: -1 -> %d (从输入形状推断)", i, shape[i])
				}
			} else {
				// 使用默认值 1（对于大多数情况，这是一个合理的默认值）
				shape[i] = 1
				if e.logger != nil {
					e.logger.Infof("  维度[%d]: -1 -> 1 (使用默认值)", i)
				}
			}
		} else {
			shape[i] = dim
		}
	}

	if e.logger != nil {
		e.logger.Infof("推断后的输出形状: %v", shape)
	}

	return shape
}

// createTensorOutput 为单个 Tensor 类型的输出创建输出张量
// 基于官方 API：只处理 Tensor 类型，Map/Sequence 类型由调用方传递 nil
//
// 参数：
//   - info: 输出信息（必须是 ONNXTypeTensor 类型）
//   - onnxInputs: 输入张量列表（用于推断动态形状）
//
// 返回值：
//   - ort.Value: 创建的输出张量
//   - error: 处理错误
func (e *Engine) createTensorOutput(info ort.InputOutputInfo, onnxInputs []ort.Value) (ort.Value, error) {
	// 验证：必须是 Tensor 类型
	if info.OrtValueType != ort.ONNXTypeTensor {
		return nil, fmt.Errorf("createTensorOutput 只能处理 Tensor 类型，收到 %v 类型", info.OrtValueType)
	}

	// 根据元数据创建空输出张量
	var shape ort.Shape

	// 添加详细日志
	if e.logger != nil {
		e.logger.Infof("创建输出张量(%s): Dimensions=%v, len=%d", info.Name, info.Dimensions, len(info.Dimensions))
	}

	if len(info.Dimensions) > 0 {
		// 检查是否有动态维度（-1）
		hasDynamicDim := false
		for _, dim := range info.Dimensions {
			if dim <= 0 {
				hasDynamicDim = true
				if e.logger != nil {
					e.logger.Infof("检测到动态维度: %d", dim)
				}
				break
			}
		}

		if hasDynamicDim {
			// 对于动态形状，根据输入形状推断
			if e.logger != nil {
				e.logger.Infof("检测到动态输出形状(%s): %v", info.Name, info.Dimensions)
			}
			shape = e.inferOutputShape(info, onnxInputs)
			if e.logger != nil {
				e.logger.Infof("推断后的输出形状(%s): %v", info.Name, shape)
			}
		} else {
			if e.logger != nil {
				e.logger.Infof("没有动态维度，直接使用: %v", info.Dimensions)
			}
			shape = info.Dimensions
		}
	} else {
		// 如果 Dimensions 为空，返回错误（不应该出现，因为已经验证是 Tensor 类型）
		return nil, fmt.Errorf("tensor 类型输出(%s)的 Dimensions 为空，无法创建张量", info.Name)
	}

	// 验证形状不包含无效值
	for j, dim := range shape {
		if dim <= 0 {
			return nil, fmt.Errorf("创建ONNX输出张量(%s)失败: 推断后的形状仍然包含无效维度[%d]=%d, 完整形状=%v, 原始Dimensions=%v",
				info.Name, j, dim, shape, info.Dimensions)
		}
	}

	// 根据模型元数据中的输出数据类型创建对应类型的输出张量
	// 📚 **官方实现参考** (github.com/yalue/onnxruntime_go@v1.22.0):
	// - onnxruntime_test.go:572: 使用 NewEmptyTensor[int16](shape) 创建 int16 输出
	// - onnxruntime_test.go:402: 使用 NewEmptyTensor[int32](shape) 创建 int32 输出
	// - onnxruntime_test.go:1167: float16/bfloat16 使用 NewCustomDataTensor(shape, []byte{}, TensorElementDataTypeBFloat16)
	//
	// ⚠️ 重要：info.DataType 是 ONNX 模型定义中的数据类型，必须与模型完全匹配
	// 如果模型输出是 int16，info.DataType 应该是 TensorElementDataTypeInt16
	if e.logger != nil {
		e.logger.Infof("尝试创建输出张量(%s): 形状=%v, 数据类型=%v (值=%d)", info.Name, shape, info.DataType, int(info.DataType))
		// 输出所有可能的 TensorElementDataType 常量值，用于调试
		e.logger.Infof("数据类型常量: Int16=%d, Int32=%d, Int64=%d, Float=%d, Double=%d",
			int(ort.TensorElementDataTypeInt16),
			int(ort.TensorElementDataTypeInt32),
			int(ort.TensorElementDataTypeInt64),
			int(ort.TensorElementDataTypeFloat),
			int(ort.TensorElementDataTypeDouble))
	}

	var outputTensor ort.Value
	var err error

	// 根据输出数据类型创建对应类型的张量（onnxruntime_go 完全支持这些类型）
	switch info.DataType {
	case ort.TensorElementDataTypeInt64:
		outputTensor, err = ort.NewEmptyTensor[int64](shape)
	case ort.TensorElementDataTypeInt32:
		// ✅ onnxruntime_go 完全支持 int32
		// 📚 官方参考: onnxruntime_test.go:402
		//    output := newTestTensor[int32](t, NewShape(1))
		//    其中 newTestTensor[int32] 内部调用 NewEmptyTensor[int32](shape)
		outputTensor, err = ort.NewEmptyTensor[int32](shape)
	case ort.TensorElementDataTypeInt16:
		// ✅ onnxruntime_go 完全支持 int16
		// 📚 官方参考: onnxruntime_test.go:572
		//    outputA := newTestTensor[int16](t, NewShape(1, 2, 2))
		//    其中 newTestTensor[int16] 内部调用 NewEmptyTensor[int16](shape)
		outputTensor, err = ort.NewEmptyTensor[int16](shape)
	case ort.TensorElementDataTypeUint8:
		outputTensor, err = ort.NewEmptyTensor[uint8](shape)
	case ort.TensorElementDataTypeFloat:
		outputTensor, err = ort.NewEmptyTensor[float32](shape)
	case ort.TensorElementDataTypeDouble:
		// float64/double类型：onnxruntime_go 支持 float64
		outputTensor, err = ort.NewEmptyTensor[float64](shape)
	default:
		// 其他类型（如 float16/bfloat16）暂不在此处预分配，由 CallModel 传递 nil 让 ONNX Runtime 自动分配
		return nil, fmt.Errorf("创建输出张量(%s)失败: 不支持的输出数据类型%v，请在 CallModel 中传递 nil 让 ONNX Runtime 自动分配",
			info.Name, info.DataType)
	}

	if err != nil {
		return nil, fmt.Errorf("创建ONNX输出张量(%s)失败: 形状%v, 数据类型%v, 错误: %w", info.Name, shape, info.DataType, err)
	}

	return outputTensor, nil
}

// releaseInputs 释放输入张量
func (e *Engine) releaseInputs(inputs []ort.Value) {
	for _, tensor := range inputs {
		if tensor != nil {
			tensor.Destroy()
		}
	}
}

// encodeValuesToRaw 将数值视图编码为原始字节视图
// dtype 使用 jsonrpc_advanced_tensor_types.md 中的字符串枚举，例如 "float32"、"float64"、"int64" 等
func encodeValuesToRaw(dtype string, vals []float64) []byte {
	switch dtype {
	case "float32":
		raw := make([]byte, len(vals)*4)
		for i, v := range vals {
			bits := math.Float32bits(float32(v))
			binary.LittleEndian.PutUint32(raw[i*4:], bits)
		}
		return raw
	case "int64":
		raw := make([]byte, len(vals)*8)
		for i, v := range vals {
			binary.LittleEndian.PutUint64(raw[i*8:], uint64(int64(v)))
		}
		return raw
	case "uint64":
		raw := make([]byte, len(vals)*8)
		for i, v := range vals {
			binary.LittleEndian.PutUint64(raw[i*8:], uint64(v))
		}
		return raw
	case "int32":
		raw := make([]byte, len(vals)*4)
		for i, v := range vals {
			binary.LittleEndian.PutUint32(raw[i*4:], uint32(int32(v)))
		}
		return raw
	case "uint32":
		raw := make([]byte, len(vals)*4)
		for i, v := range vals {
			binary.LittleEndian.PutUint32(raw[i*4:], uint32(v))
		}
		return raw
	case "int16":
		raw := make([]byte, len(vals)*2)
		for i, v := range vals {
			binary.LittleEndian.PutUint16(raw[i*2:], uint16(int16(v)))
		}
		return raw
	case "uint16":
		raw := make([]byte, len(vals)*2)
		for i, v := range vals {
			binary.LittleEndian.PutUint16(raw[i*2:], uint16(v))
		}
		return raw
	case "int8":
		raw := make([]byte, len(vals))
		for i, v := range vals {
			raw[i] = byte(int8(v))
		}
		return raw
	case "uint8":
		raw := make([]byte, len(vals))
		for i, v := range vals {
			raw[i] = byte(uint8(v))
		}
		return raw
	case "bool":
		raw := make([]byte, len(vals))
		for i, v := range vals {
			if v != 0 {
				raw[i] = 1
			} else {
				raw[i] = 0
			}
		}
		return raw
	default:
		// 默认使用 float64 小端编码
		raw := make([]byte, len(vals)*8)
		for i, v := range vals {
			bits := math.Float64bits(v)
			binary.LittleEndian.PutUint64(raw[i*8:], bits)
		}
		return raw
	}
}

// postprocessOutputs 将ONNX张量转换为富张量结构 []TensorOutput
// 支持多种数据类型的输出（float32, int64, uint8等），并为 JSON-RPC tensor_outputs 提供基础数据
func (e *Engine) postprocessOutputs(onnxOutputs []ort.Value, outputInfos []ort.InputOutputInfo) ([]ispcInterfaces.TensorOutput, error) {
	outputs := make([]ispcInterfaces.TensorOutput, 0, len(onnxOutputs))

	for i, onnxValue := range onnxOutputs {
		info := outputInfos[i]

		// 跳过 nil（Map/Sequence 类型，由 ONNX Runtime 自动分配）
		if onnxValue == nil {
			if e.logger != nil {
				e.logger.Warnf("输出[%d]是 Map/Sequence 类型，跳过处理", i)
			}
			// 对于 Map/Sequence 类型，返回占位空张量
			outputs = append(outputs, ispcInterfaces.TensorOutput{
				Name:    info.Name,
				DType:   "",
				Shape:   nil,
				Layout:  "",
				Values:  []float64{},
				RawData: nil,
			})
			continue
		}

		// 检查类型：只处理 Tensor 类型
		if onnxValue.GetONNXType() != ort.ONNXTypeTensor {
			if e.logger != nil {
				e.logger.Warnf("输出[%d]是 %v 类型，跳过处理（当前只支持 Tensor 类型）", i, onnxValue.GetONNXType())
			}
			// 非 Tensor 类型，返回占位空张量
			outputs = append(outputs, ispcInterfaces.TensorOutput{
				Name:    info.Name,
				DType:   "",
				Shape:   nil,
				Layout:  "",
				Values:  []float64{},
				RawData: nil,
			})
			continue
		}

		var tensorData []float64

		// 尝试不同的数据类型转换（onnxruntime_go 支持的类型）
		switch tensor := onnxValue.(type) {
		case *ort.Tensor[float32]:
			// float32类型
			data := tensor.GetData()
			tensorData = make([]float64, len(data))
			for j, val := range data {
				tensorData[j] = float64(val)
			}
		case *ort.Tensor[float64]:
			// float64/double类型：直接使用，无需转换
			tensorData = tensor.GetData()
		case *ort.Tensor[int64]:
			// int64类型（如 sklearn_randomforest 的 output_label）
			data := tensor.GetData()
			tensorData = make([]float64, len(data))
			for j, val := range data {
				tensorData[j] = float64(val)
			}
		case *ort.Tensor[int32]:
			// int32类型（onnxruntime_go 完全支持）
			// 📚 官方参考: onnxruntime_test.go:415
			//    result := output.GetData()[0]  // output 是 *Tensor[int32]
			//    直接使用 GetData() 获取 []int32 数据
			data := tensor.GetData()
			tensorData = make([]float64, len(data))
			for j, val := range data {
				tensorData[j] = float64(val)
			}
		case *ort.Tensor[int16]:
			// int16类型（onnxruntime_go 完全支持）
			// 📚 官方参考: onnxruntime_test.go:591
			//    verifyTensorData(t, outputA, expectedA)  // outputA 是 *Tensor[int16]
			//    其中 verifyTensorData 内部使用 tensor.GetData() 获取 []int16 数据
			data := tensor.GetData()
			tensorData = make([]float64, len(data))
			for j, val := range data {
				tensorData[j] = float64(val)
			}
		case *ort.Tensor[uint8]:
			// uint8类型
			data := tensor.GetData()
			tensorData = make([]float64, len(data))
			for j, val := range data {
				tensorData[j] = float64(val)
			}
		default:
			// 对于 float16 / bfloat16 等特殊类型，当前不强制解析为数值，只返回空数组并记录日志
			if e.logger != nil {
				e.logger.Warnf("输出[%d]使用了当前不支持直接解析的数据类型: %T，返回空结果占位", i, onnxValue)
			}
			outputs = append(outputs, ispcInterfaces.TensorOutput{
				Name:    info.Name,
				DType:   "",
				Shape:   nil,
				Layout:  "",
				Values:  []float64{},
				RawData: nil,
			})
			continue
		}

		// 映射 ONNX/ORT 数据类型到 dtype 字符串
		dtype := ""
		switch info.DataType {
		case ort.TensorElementDataTypeFloat:
			dtype = "float32"
		case ort.TensorElementDataTypeDouble:
			dtype = "float64"
		case ort.TensorElementDataTypeInt64:
			dtype = "int64"
		case ort.TensorElementDataTypeInt32:
			dtype = "int32"
		case ort.TensorElementDataTypeInt16:
			dtype = "int16"
		case ort.TensorElementDataTypeUint8:
			dtype = "uint8"
		default:
			dtype = "float64"
		}

		// 形状优先使用模型元数据，如果存在动态轴则回退到实际张量形状或 [N]
		var shape []int64
		hasDynamic := false
		for _, d := range info.Dimensions {
			if d <= 0 {
				hasDynamic = true
				break
			}
		}
		if !hasDynamic && len(info.Dimensions) > 0 {
			shape = make([]int64, len(info.Dimensions))
			for idx, d := range info.Dimensions {
				shape[idx] = int64(d)
			}
		} else {
			// 尝试从张量本身获取形状
			switch tensor := onnxValue.(type) {
			case *ort.Tensor[float32]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			case *ort.Tensor[float64]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			case *ort.Tensor[int64]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			case *ort.Tensor[int32]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			case *ort.Tensor[int16]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			case *ort.Tensor[uint8]:
				s := tensor.GetShape()
				shape = make([]int64, len(s))
				for idx, d := range s {
					shape[idx] = int64(d)
				}
			default:
				// 回退为一维 [N]
				shape = []int64{int64(len(tensorData))}
			}
		}

		raw := encodeValuesToRaw(dtype, tensorData)

		outputs = append(outputs, ispcInterfaces.TensorOutput{
			Name:    info.Name,
			DType:   dtype,
			Shape:   shape,
			Layout:  "",
			Values:  tensorData,
			RawData: raw,
		})
	}

	return outputs, nil
}

// releaseOutputs 释放输出张量
func (e *Engine) releaseOutputs(outputs []ort.Value) {
	for _, tensor := range outputs {
		if tensor != nil {
			tensor.Destroy()
		}
	}
}

// Shutdown 关闭引擎
func (e *Engine) Shutdown() error {
	if e.modelCache != nil {
		if err := e.modelCache.Clear(); err != nil {
			if e.logger != nil {
				e.logger.Errorf("清理模型缓存失败: %v", err)
			}
		}
	}

	if e.sessionPool != nil {
		if err := e.sessionPool.Close(); err != nil {
			if e.logger != nil {
				e.logger.Errorf("关闭会话池失败: %v", err)
			}
		}
	}

	// 清理ONNX Runtime环境
	ort.DestroyEnvironment()

	if e.logger != nil {
		e.logger.Info("✅ ONNX引擎已关闭")
	}

	return nil
}
