package abi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// Service ABI 服务实现
//
// 🎯 **设计理念**：薄实现，严格遵循WES三层架构
// 📋 **架构原则**：Service负责ABI相关的具体业务逻辑实现，不是管理器
//
// 实现 pkg/interfaces/engines.ABIService 公共接口
// 提供合约 ABI 的注册、编码、解码等核心功能
//
// 🔗 **依赖关系**：
// - log.Logger：日志记录服务
type Service struct {
	// ==================== 基础设施服务 ====================
	logger log.Logger // 日志服务

	// ==================== ABI 存储 ====================
	// 使用内存存储，key 为 contractID，value 为 ContractABI
	abis map[string]*types.ContractABI
	mu   sync.RWMutex // 读写锁保护并发访问
}

// 确保Service实现ispcInterfaces.ABIService接口
var _ ispcInterfaces.ABIService = (*Service)(nil)

// NewService 创建 ABI 服务
//
// 🎯 **依赖注入构造器**：接收必要的依赖服务
// 📋 **服务实现原则**：实现ABI相关的具体业务逻辑
//
// 📋 **参数说明**：
//   - logger: 日志服务，用于记录操作过程和错误信息
//
// 🔧 **初始化内容**：
//   - abis: 初始化空的 ABI 存储映射
//   - mu: 初始化读写锁
func NewService(logger log.Logger) *Service {
	return &Service{
		// 基础设施服务
		logger: logger,

		// ABI 存储初始化
		abis: make(map[string]*types.ContractABI),
	}
}

// ==================== 公共接口实现 ====================

// RegisterABI 注册合约 ABI 定义（公共接口实现）
//
// 🎯 **ABI 注册功能**：
// 将合约的 ABI 定义注册到管理器中，供后续编码解码使用
//
// 📋 **参数说明**：
//   - contractID: 合约标识符，通常是合约地址或哈希
//   - abi: 合约 ABI 定义，包含函数签名、参数类型等信息
//
// 🔧 **注册流程**：
//  1. 验证输入参数的有效性
//  2. 使用写锁保护并发安全
//  3. 将 ABI 存储到内存映射中
//  4. 记录操作日志
//
// ⚠️ **线程安全**：
// 使用读写锁确保并发注册和查询的安全性
func (s *Service) RegisterABI(contractID string, abi *types.ContractABI) error {
	if s.logger != nil {
		s.logger.Debug("开始注册合约 ABI")
	}

	// 基础验证
	if contractID == "" {
		return fmt.Errorf("合约ID不能为空")
	}
	if abi == nil {
		return fmt.Errorf("ABI定义不能为空")
	}

	// 使用写锁保护并发安全
	s.mu.Lock()
	defer s.mu.Unlock()

	// 存储 ABI 定义
	s.abis[contractID] = abi

	if s.logger != nil {
		s.logger.Debugf("合约 ABI 注册成功: contractID=%s, functions=%d, events=%d",
			contractID, len(abi.Functions), len(abi.Events))
	}

	return nil
}

// EncodeParameters 编码函数参数（公共接口实现）
//
// 🎯 **参数编码功能**：
// 根据合约 ABI 定义，将函数调用参数编码为字节序列
//
// 📋 **参数说明**：
//   - contractID: 合约标识符
//   - method: 函数名称
//   - args: 函数参数数组
//
// 🔧 **编码流程**：
//  1. 根据 contractID 查找对应的 ABI 定义
//  2. 在 ABI 中查找指定的函数定义
//  3. 验证参数数量和类型
//  4. 使用 JSON 编码参数（简化实现）
//  5. 返回编码后的字节数据
//
// ⚠️ **当前实现**：
// 使用 JSON 编码作为简化实现，生产环境可能需要更高效的编码方式
func (s *Service) EncodeParameters(contractID, method string, args []interface{}) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("开始编码函数参数")
	}

	// 基础验证
	if contractID == "" {
		return nil, fmt.Errorf("合约ID不能为空")
	}
	if method == "" {
		return nil, fmt.Errorf("方法名不能为空")
	}

	// 使用读锁查找 ABI
	s.mu.RLock()
	abi, exists := s.abis[contractID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未找到合约 %s 的 ABI 定义", contractID)
	}

	// 查找函数定义
	var targetFunction *types.ContractFunction
	for i := range abi.Functions {
		if abi.Functions[i].Name == method {
			targetFunction = &abi.Functions[i]
			break
		}
	}

	if targetFunction == nil {
		return nil, fmt.Errorf("未找到方法 %s 的定义", method)
	}

	// 验证参数数量
	if len(args) != len(targetFunction.Params) {
		return nil, fmt.Errorf("参数数量不匹配: 期望 %d 个，实际 %d 个",
			len(targetFunction.Params), len(args))
	}

	// 使用 JSON 编码（简化实现）
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("参数编码失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("函数参数编码成功: contractID=%s, method=%s, args_count=%d, encoded_size=%d",
			contractID, method, len(args), len(encoded))
	}

	return encoded, nil
}

// DecodeResult 解码函数返回值（公共接口实现）
//
// 🎯 **返回值解码功能**：
// 根据合约 ABI 定义，将字节序列解码为函数返回值
//
// 📋 **参数说明**：
//   - contractID: 合约标识符
//   - method: 函数名称
//   - data: 待解码的字节数据
//
// 🔧 **解码流程**：
//  1. 根据 contractID 查找对应的 ABI 定义
//  2. 在 ABI 中查找指定的函数定义
//  3. 使用 JSON 解码数据（简化实现）
//  4. 返回解码后的结果数组
//
// ⚠️ **当前实现**：
// 使用 JSON 解码作为简化实现，生产环境可能需要更精确的类型解码
func (s *Service) DecodeResult(contractID, method string, data []byte) ([]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug("开始解码函数返回值")
	}

	// 基础验证
	if contractID == "" {
		return nil, fmt.Errorf("合约ID不能为空")
	}
	if method == "" {
		return nil, fmt.Errorf("方法名不能为空")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("解码数据不能为空")
	}

	// 使用读锁查找 ABI
	s.mu.RLock()
	abi, exists := s.abis[contractID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未找到合约 %s 的 ABI 定义", contractID)
	}

	// 查找函数定义
	var targetFunction *types.ContractFunction
	for i := range abi.Functions {
		if abi.Functions[i].Name == method {
			targetFunction = &abi.Functions[i]
			break
		}
	}

	if targetFunction == nil {
		return nil, fmt.Errorf("未找到方法 %s 的定义", method)
	}

	// 使用 JSON 解码（简化实现）
	var result []interface{}
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("返回值解码失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("函数返回值解码成功: contractID=%s, method=%s, result_count=%d, data_size=%d",
			contractID, method, len(result), len(data))
	}

	return result, nil
}
