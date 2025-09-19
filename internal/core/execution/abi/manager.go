package abi

import (
	"fmt"
	"sync"
	"time"

	interfaces "github.com/weisyn/v1/internal/core/execution/interfaces"
	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	typespkg "github.com/weisyn/v1/pkg/types"
)

// 文件说明（中文）：
// 本文件实现 ABI 管理器（ABIManager），提供对外 ABIService（公共接口）与
// InternalABIService（内部接口）的标准实现，负责：
// 1) 合约 ABI 的注册与版本管理；
// 2) 基于 ABI 的参数编码与返回值解码；
// 3) 版本兼容性报告生成；
// 4) 基础统计信息的采集与暴露（内部）。

// ABIManager ABI管理器的核心实现，负责统一管理合约ABI的生命周期。
//
// ABIManager 作为 ABI 子模块的门面，整合了存储、编解码、验证、兼容性检查
// 等各项功能，为上层提供统一的 ABI 服务接口。它同时实现了公共接口
// (pkg/interfaces/execution.ABIService) 和内部接口
// (internal/core/execution/interfaces.InternalABIService)。
//
// 设计特点：
//   - 线程安全：使用读写锁保护并发访问
//   - 策略可配置：支持通过依赖注入替换编解码、验证等策略
//   - 统计监控：内置性能统计和监控支持
//   - 错误处理：完整的错误处理和日志记录
type ABIManager struct {
	// abiStore ABI存储实例，负责ABI定义的持久化和检索
	abiStore *ABIStore

	// encoder 编码器实例，实现 interfaces.Encoder 接口
	// 负责将函数调用参数编码为字节序列
	encoder interfaces.Encoder

	// decoder 解码器实例，实现 interfaces.Decoder 接口
	// 负责将字节序列解码为函数返回值
	decoder interfaces.Decoder

	// compatSvc 兼容性服务实例，实现 interfaces.CompatibilityService 接口
	// 负责检查不同版本ABI间的兼容性
	compatSvc interfaces.CompatibilityService

	// typeSystem 类型系统实例，负责ABI类型的管理和转换
	typeSystem *TypeSystem

	// validator ABI验证器实例，负责验证ABI定义的正确性
	validator *ABIValidator

	// config 管理器配置实例，控制管理器的行为和性能参数
	config *ABIManagerConfig

	// mutex 读写互斥锁，保护并发访问的数据安全
	// 使用读写锁以支持并发读取，提高性能
	mutex sync.RWMutex

	// stats 统计信息实例，收集ABI操作的性能指标
	stats *ABIStats
}

// 编译时接口实现检查，确保 ABIManager 实现了所需的接口
var _ execiface.ABIService = (*ABIManager)(nil)
var _ interfaces.InternalABIService = (*ABIManager)(nil)

// NewABIManager 创建新的ABI管理器实例。
//
// 该函数使用提供的配置创建一个完整的ABI管理器，包括所有必要的组件：
// 存储、编解码器、验证器、兼容性服务、类型系统和统计收集。
// 如果未提供配置，将使用默认配置。
//
// 参数：
//   - config: ABI管理器配置，可以为nil（使用默认配置）
//
// 返回值：
//   - *ABIManager: 初始化完成的ABI管理器实例
//
// 初始化的组件：
//   - ABIStore: 使用默认存储配置的内存存储
//   - Encoder/Decoder: 默认的JSON编解码实现
//   - CompatibilityService: 默认的兼容性检查服务
//   - TypeSystem: 默认的类型系统配置
//   - Validator: 默认的ABI验证器
//   - Stats: 性能统计收集器
func NewABIManager(config *ABIManagerConfig) *ABIManager {
	if config == nil {
		config = DefaultABIManagerConfig()
	}
	return &ABIManager{
		abiStore:   NewABIStore(DefaultABIStoreConfig()),
		encoder:    newDefaultEncoder(),
		decoder:    newDefaultDecoder(),
		compatSvc:  newDefaultCompatibilityService(),
		typeSystem: NewTypeSystem(DefaultTypeSystemConfig()),
		validator:  NewABIValidator(), // 零配置，使用智能默认策略
		config:     config,
		stats:      NewABIStats(),
	}
}

// RegisterABI 注册合约的ABI定义。
//
// 该方法将合约的ABI定义存储到管理器中，并进行必要的验证和兼容性检查。
// 注册过程包括ABI结构验证、版本兼容性检查（如果启用）、存储操作和统计更新。
//
// 参数：
//   - contractID: 合约的唯一标识符，通常是合约地址
//   - abi: 合约的ABI定义，包含函数、事件等接口信息
//
// 返回值：
//   - error: 注册失败时返回错误信息，成功时返回nil
//
// 执行流程：
//  1. 获取写锁，确保线程安全
//  2. 使用验证器验证ABI定义的正确性
//  3. 如果启用兼容性检查，与现有版本进行兼容性验证
//  4. 将ABI定义存储到存储层
//  5. 更新注册统计信息
//
// 可能的错误：
//   - ABI验证失败：ABI定义不符合规范
//   - 兼容性检查失败：新版本与现有版本不兼容
//   - 存储失败：底层存储操作出错
func (am *ABIManager) RegisterABI(contractID string, abi *typespkg.ContractABI) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// 验证ABI定义的正确性
	if errors := am.validator.ValidateABI(abi); len(errors) > 0 {
		return fmt.Errorf("ABI validation failed: %v", errors)
	}

	// 兼容性检查（自运行节点始终启用，确保版本安全）
	if existingABI, err := am.abiStore.GetABI(contractID, ""); err == nil && existingABI != nil {
		if !am.compatSvc.IsCompatible(existingABI, abi) {
			return fmt.Errorf("ABI is not compatible with existing version")
		}
	}

	// 存储ABI定义
	if err := am.abiStore.StoreABI(contractID, abi); err != nil {
		return fmt.Errorf("failed to store ABI: %w", err)
	}

	// 更新统计信息
	am.updateRegistrationStats()
	return nil
}

// EncodeParameters 编码函数参数为字节序列。
//
// 该方法根据合约ABI定义将函数调用参数编码为标准化的字节序列，
// 用于网络传输、存储或执行引擎处理。
//
// 参数：
//   - contractID：合约的唯一标识符，通常是合约地址
//   - method：要调用的函数名称
//   - args：函数参数列表，按ABI定义顺序排列
//
// 返回值：
//   - []byte：编码后的字节序列
//   - error：编码过程中的错误，nil表示成功
//
// 执行流程：
//  1. 自动推断最新版本的ABI定义
//  2. 查找指定的函数定义
//  3. 验证参数数量和类型匹配
//  4. 处理参数值（类型转换等）
//  5. 调用编码器生成字节序列
//  6. 更新编码操作统计
func (am *ABIManager) EncodeParameters(contractID, method string, args []interface{}) ([]byte, error) {
	startTime := time.Now()
	defer func() { am.updateEncodingStats(time.Since(startTime)) }()
	// 自动推断最新版本（自运行节点的智能默认行为）
	abi, err := am.abiStore.GetABI(contractID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI: %w", err)
	}
	var fn *typespkg.ContractFunction
	for i := range abi.Functions {
		if abi.Functions[i].Name == method {
			fn = &abi.Functions[i]
			break
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("function %s not found in ABI", method)
	}
	if len(args) != len(fn.Params) {
		return nil, fmt.Errorf("argument count mismatch: expected %d, got %d", len(fn.Params), len(args))
	}
	processedArgs, err := am.processArguments(fn.Params, args)
	if err != nil {
		return nil, fmt.Errorf("failed to process arguments: %w", err)
	}
	encoded, err := am.encoder.EncodeFunctionCall(fn, processedArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode function call: %w", err)
	}
	return encoded, nil
}

// DecodeResult 解码函数返回值。
//
// 该方法根据合约ABI定义将字节序列解码为函数返回值列表，
// 支持单返回值和多返回值函数的解码。
//
// 参数：
//   - contractID：合约的唯一标识符
//   - method：函数名称
//   - data：待解码的字节序列
//
// 返回值：
//   - []interface{}：解码后的返回值列表
//   - error：解码过程中的错误，nil表示成功
//
// 执行流程：
//  1. 自动推断最新版本的ABI定义
//  2. 查找指定的函数定义
//  3. 调用解码器将字节序列解码为返回值
//  4. 更新解码操作统计
func (am *ABIManager) DecodeResult(contractID, method string, data []byte) ([]interface{}, error) {
	startTime := time.Now()
	defer func() { am.updateDecodingStats(time.Since(startTime)) }()
	// 自动推断最新版本（自运行节点的智能默认行为）
	abi, err := am.abiStore.GetABI(contractID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI: %w", err)
	}
	var fn *typespkg.ContractFunction
	for i := range abi.Functions {
		if abi.Functions[i].Name == method {
			fn = &abi.Functions[i]
			break
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("function %s not found in ABI", method)
	}
	result, err := am.decoder.DecodeFunctionResult(fn, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode function result: %w", err)
	}
	return result, nil
}

// ValidateCompatibility 验证ABI版本间的兼容性。
//
// 该方法对比两个版本的ABI定义，生成详细的兼容性报告，
// 包括破坏性变更、新增功能、移除功能等信息。
//
// 参数：
//   - contractID：合约的唯一标识符
//   - fromVersion：源版本号
//   - toVersion：目标版本号
//
// 返回值：
//   - *interfaces.CompatibilityReport：详细的兼容性分析报告
//   - error：验证过程中的错误，nil表示成功
//
// 报告内容：
//   - 兼容性状态：是否向后兼容
//   - 破坏性变更：不兼容的修改列表
//   - 新增功能：新版本新增的函数
//   - 移除功能：新版本移除的函数
//   - 修改功能：签名变更的函数
//   - 升级建议：版本升级的建议
func (am *ABIManager) ValidateCompatibility(contractID, fromVersion, toVersion string) (*interfaces.CompatibilityReport, error) {
	fromABI, err := am.abiStore.GetABI(contractID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get source ABI: %w", err)
	}
	toABI, err := am.abiStore.GetABI(contractID, toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target ABI: %w", err)
	}
	return am.compatSvc.GenerateCompatibilityReport(fromABI, toABI), nil
}

// GetABIStats 获取ABI管理器的统计信息。
//
// 该方法返回当前管理器的操作统计数据，用于性能监控、
// 问题诊断和系统优化。统计数据包括操作次数和大致的性能指标。
//
// 返回值：
//   - *interfaces.ABIStats：符合接口定义的统计信息
//
// 统计内容：
//   - 注册ABI总数：当前管理器中活跃的ABI定义数量
//   - 编码操作次数：累计执行的编码操作数量
//   - 解码操作次数：累计执行的解码操作数量
//
// 线程安全性：
//   - 使用读锁保护，支持并发访问
//   - 原子操作读取计数器，确保数据一致性
func (am *ABIManager) GetABIStats() *interfaces.ABIStats {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.stats.ToInterfaceStats()
}

// processArguments 处理函数调用参数。
//
// 该方法对函数调用参数进行预处理，包括类型验证、格式转换、
// 值校验等操作，确保参数符合ABI定义的要求。
//
// 参数：
//   - params：ABI参数定义列表，描述参数的类型和约束
//   - args：实际参数值列表，与定义一一对应
//
// 返回值：
//   - []interface{}：处理后的参数列表
//   - error：处理过程中的错误，nil表示成功
//
// 处理逻辑：
//
//	当前实现为简化版本，直接复制参数列表
//	生产环境中可扩展为完整的类型验证和转换逻辑
func (am *ABIManager) processArguments(params []typespkg.ABIParam, args []interface{}) ([]interface{}, error) {
	processedArgs := make([]interface{}, len(args))
	copy(processedArgs, args)
	_ = params
	return processedArgs, nil
}

// updateRegistrationStats 更新ABI注册统计信息。
//
// 该方法在ABI注册成功后调用，用于更新管理器的统计计数器。
// 包括增加注册的ABI总数和更新最后修改时间。
func (am *ABIManager) updateRegistrationStats() {
	am.stats.TotalABIs++
	am.stats.LastUpdated = time.Now()
}

// updateEncodingStats 更新编码操作统计信息。
//
// 该方法在编码操作完成后调用，用于更新性能统计数据。
//
// 参数：
//   - duration：编码操作的耗时（当前实现中未使用，保留接口兼容性）
func (am *ABIManager) updateEncodingStats(duration time.Duration) {
	am.stats.EncodingOperations++
	am.stats.LastUpdated = time.Now()
	_ = duration
}

// updateDecodingStats 更新解码操作统计信息。
//
// 该方法在解码操作完成后调用，用于更新性能统计数据。
//
// 参数：
//   - duration：解码操作的耗时（当前实现中未使用，保留接口兼容性）
func (am *ABIManager) updateDecodingStats(duration time.Duration) {
	am.stats.DecodingOperations++
	am.stats.LastUpdated = time.Now()
	_ = duration
}
