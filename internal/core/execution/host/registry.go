package host

import (
	"fmt"
	"sync"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Registry 宿主能力注册器
//
// # 核心功能：
// - 聚合各领域的宿主能力提供者（IO、状态、事件、UTXO）
// - 构建统一的标准宿主接口，供智能合约和AI模型调用
// - 提供域名唯一性保证和类型安全的能力访问
// - 支持动态注册和能力发现
//
// # 设计目标：
// - 模块化：各领域能力独立实现，松耦合组织
// - 可扩展：支持新领域能力的动态注册
// - 类型安全：编译时和运行时的类型检查
// - 高性能：读写锁优化，便捷引用避免频繁查找
//
// # 使用场景：
// - 智能合约执行环境的宿主函数提供
// - AI模型推理过程中的区块链状态访问
// - 执行引擎与区块链核心组件的桥接
// - 沙箱环境中的受控能力暴露
type Registry struct {
	// mu 读写锁
	// 保护byDomain映射和便捷引用字段的并发访问安全
	// 使用读写锁优化读多写少的访问模式
	mu sync.RWMutex

	// byDomain 按领域名称分组的能力提供者映射
	// 键：领域名称（如"io"、"state"、"events"、"utxo"）
	// 值：对应领域的能力提供者实现
	// 设计：确保每个领域只能注册一个提供者
	byDomain map[string]execiface.HostCapabilityProvider

	// ==================== 便捷引用（类型化访问） ====================
	// 以下字段提供类型安全的直接访问，避免频繁的类型断言和映射查找

	// io IO能力提供者
	// 提供文件读写、日志输出、返回数据设置等IO相关功能
	io *IOProvider

	// state 状态查询提供者
	// 提供调用者信息、合约地址、区块高度、存储查询等状态访问功能
	state *StateProvider

	// events 事件发射提供者已移除
	// execution模块使用同步操作，不需要事件系统

	// utxo UTXO操作提供者
	// 提供UTXO创建、转移、余额查询等区块链资产操作功能
	utxo *UTXOProvider
}

// NewRegistry 创建宿主能力注册器
//
// 返回初始化完成的空注册器实例，准备接受能力提供者注册
//
// 返回值：
//   - *Registry: 新创建的注册器实例
//
// 初始状态：
//   - 领域映射为空，无已注册的能力提供者
//   - 便捷引用字段为nil，需要通过RegisterProvider填充
//   - 读写锁已初始化，可安全并发使用
//
// 使用示例：
//
//	registry := NewRegistry()
//	registry.RegisterProvider(NewIOProvider())
//	registry.RegisterProvider(NewStateProvider())
//	hostInterface := registry.BuildStandardInterface()
//
// 设计考虑：
//   - 零配置创建，所有组件都有安全的默认状态
//   - 延迟初始化，能力提供者按需注册
//   - 支持分阶段构建，灵活的组装模式
func NewRegistry() *Registry {
	return &Registry{byDomain: make(map[string]execiface.HostCapabilityProvider)}
}

// RegisterProvider 注册能力提供者
//
// 将能力提供者注册到指定领域，确保领域名称唯一性
//
// 参数：
//   - p: 要注册的能力提供者实现
//
// 返回值：
//   - error: 注册错误，包括参数校验和重复注册检查
//
// 注册流程：
//  1. 参数校验：提供者不能为nil，领域名称不能为空
//  2. 重复检查：同一领域只能注册一个提供者
//  3. 映射注册：将提供者添加到领域映射中
//  4. 便捷引用：为已知类型设置类型化引用
//
// 支持的领域：
//   - "io": IO操作能力（文件读写、日志、返回数据）
//   - "state": 状态查询能力（调用者、合约地址、区块信息）
//   - "events": 事件发射能力（结构化事件、队列管理）
//   - "utxo": UTXO操作能力（创建、转移、余额查询）
//
// 并发安全：
//   - 使用写锁保护注册过程的原子性
//   - 避免并发注册导致的数据竞争
//
// 错误处理：
//   - 提供者为nil时返回明确错误
//   - 领域名称为空时返回参数错误
//   - 重复注册时返回冲突错误，保护已注册的提供者
func (r *Registry) RegisterProvider(p execiface.HostCapabilityProvider) error {
	// 参数校验
	if p == nil {
		return fmt.Errorf("host provider is nil")
	}
	domain := p.CapabilityDomain()
	if domain == "" {
		return fmt.Errorf("host provider domain is empty")
	}

	// 加写锁保护注册过程
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查重复注册
	if _, exists := r.byDomain[domain]; exists {
		return fmt.Errorf("host provider domain already registered: %s", domain)
	}

	// 注册到领域映射
	r.byDomain[domain] = p

	// 设置便捷引用，提供类型安全的直接访问
	switch domain {
	case "io":
		if v, ok := p.(*IOProvider); ok {
			r.io = v
		}
	case "state":
		if v, ok := p.(*StateProvider); ok {
			r.state = v
		}
	// "events" case removed - execution不需要事件系统
	case "utxo":
		if v, ok := p.(*UTXOProvider); ok {
			r.utxo = v
		}
	}
	return nil
}

// BuildStandardInterface 构建标准宿主接口
//
// 将所有已注册的能力提供者聚合为统一的标准宿主接口
//
// 返回值：
//   - execiface.HostStandardInterface: 聚合后的标准宿主接口实现
//
// 功能说明：
//   - 聚合各领域能力：将IO、状态、事件、UTXO等能力统一暴露
//   - 提供标准接口：符合pkg/interfaces/execution定义的接口规范
//   - 能力检查：每个方法调用时会检查对应领域是否已注册
//   - 错误处理：未注册的能力会返回明确的错误信息
//
// 使用场景：
//   - 智能合约执行时的宿主函数提供
//   - AI模型推理过程中的区块链能力访问
//   - 执行引擎与区块链核心组件的桥接
//
// 并发安全：
//   - 使用读锁保护构建过程，允许并发读取
//   - 返回的接口实例是线程安全的
//
// 设计考虑：
//   - 延迟检查：构建时不检查能力完整性，调用时才检查
//   - 优雅降级：缺失的能力返回明确错误而非崩溃
//   - 类型安全：利用便捷引用提供编译时类型检查
func (r *Registry) BuildStandardInterface() execiface.HostStandardInterface {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return &standardHost{io: r.io, state: r.state, utxo: r.utxo}
}

// ==================== 标准宿主实现 ====================

// standardHost 标准宿主接口的具体实现
//
// # 功能说明：
// - 实现execiface.HostStandardInterface接口
// - 聚合各领域能力提供者，提供统一的宿主函数访问
// - 每个方法调用时检查对应能力是否可用
// - 提供优雅的错误处理和能力缺失提示
//
// # 设计特点：
// - 组合模式：通过组合各Provider实现完整功能
// - 延迟检查：运行时检查能力可用性，而非构建时
// - 错误透明：将底层Provider的错误直接传播给调用方
// - 类型安全：利用类型化字段避免运行时类型断言
//
// # 使用场景：
// - 作为智能合约和AI模型的宿主环境
// - 桥接执行引擎与区块链核心功能
// - 提供沙箱化的区块链能力访问
type standardHost struct {
	// io IO能力提供者
	// 处理文件读写、日志输出、返回数据设置等IO操作
	// nil时相关方法将返回能力不可用错误
	io *IOProvider

	// state 状态查询提供者
	// 处理调用者信息、合约地址、区块高度等状态查询
	// nil时相关方法将返回能力不可用错误
	state *StateProvider

	// events 事件发射提供者已移除
	// execution模块使用同步操作，不需要事件系统

	// utxo UTXO操作提供者
	// 处理UTXO创建、转移、余额查询等区块链资产操作
	// nil时相关方法将返回能力不可用错误
	utxo *UTXOProvider
}

func (s *standardHost) SetReturnData(data []byte) error {
	if s.io == nil || s.io.setReturnData == nil {
		return newCapabilityErr("io", "SetReturnData")
	}
	return s.io.setReturnData(data)
}

// EmitEvent 提供空实现以满足接口要求
// execution模块不支持事件系统，返回不支持错误
func (s *standardHost) EmitEvent(eventType string, payload []byte) error {
	return fmt.Errorf("event emission not supported: execution module uses synchronous operations only")
}

func (s *standardHost) GetCaller() ([]byte, error) {
	if s.state == nil || s.state.getCaller == nil {
		return nil, newCapabilityErr("state", "GetCaller")
	}
	return s.state.getCaller()
}

func (s *standardHost) GetContractAddress() ([]byte, error) {
	if s.state == nil || s.state.getContractAddr == nil {
		return nil, newCapabilityErr("state", "GetContractAddress")
	}
	return s.state.getContractAddr()
}

func (s *standardHost) CreateUTXOOutput(recipient []byte, amount uint64) error {
	if s.utxo == nil || s.utxo.createOutput == nil {
		return newCapabilityErr("utxo", "CreateUTXOOutput")
	}
	return s.utxo.createOutput(recipient, amount)
}

func (s *standardHost) ExecuteUTXOTransfer(from, to []byte, amount uint64) error {
	if s.utxo == nil || s.utxo.transfer == nil {
		return newCapabilityErr("utxo", "ExecuteUTXOTransfer")
	}
	return s.utxo.transfer(from, to, amount)
}

func (s *standardHost) QueryUTXOBalance(address []byte) (uint64, error) {
	if s.utxo == nil || s.utxo.queryBalance == nil {
		return 0, newCapabilityErr("utxo", "QueryUTXOBalance")
	}
	return s.utxo.queryBalance(address)
}

func (s *standardHost) GetContractInitParams() ([]byte, error) {
	if s.state == nil || s.state.getInitParams == nil {
		return nil, newCapabilityErr("state", "GetContractInitParams")
	}
	return s.state.getInitParams()
}

// GetBlockHeight 获取当前链高度
func (s *standardHost) GetBlockHeight() (uint64, error) {
	if s.state == nil || s.state.getBlockHeight == nil {
		return 0, newCapabilityErr("state", "GetBlockHeight")
	}
	return s.state.getBlockHeight()
}

// GetTransaction 获取序列化交易字节
func (s *standardHost) GetTransaction(txHash []byte) ([]byte, error) {
	if s.state == nil || s.state.getTransaction == nil {
		return nil, newCapabilityErr("state", "GetTransaction")
	}
	return s.state.getTransaction(txHash)
}

// GetBalance 获取账户主币余额（通过 UTXO 聚合）
func (s *standardHost) GetBalance(address []byte) (uint64, error) {
	if s.utxo == nil || s.utxo.queryBalance == nil {
		return 0, newCapabilityErr("utxo", "GetBalance")
	}
	return s.utxo.queryBalance(address)
}

// GetStorage 获取合约存储键对应的值
func (s *standardHost) GetStorage(contractAddr []byte, key []byte) ([]byte, error) {
	if s.state == nil || s.state.getStorage == nil {
		return nil, newCapabilityErr("state", "GetStorage")
	}
	return s.state.getStorage(contractAddr, key)
}

func newCapabilityErr(domain, method string) error {
	return fmt.Errorf("host capability unavailable: domain=%s method=%s", domain, method)
}

// NewHostCapabilityRegistryWrapper 创建宿主能力注册器的包装器
// 从module.go迁移而来，用于fx依赖注入
func NewHostCapabilityRegistryWrapper(logger log.Logger) execiface.HostCapabilityRegistry {
	registry := NewRegistry()
	if logger != nil {
		logger.Info("HostCapabilityRegistry已创建")
	}
	return registry
}
