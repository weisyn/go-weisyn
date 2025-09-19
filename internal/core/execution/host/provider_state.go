package host

import (
	"fmt"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
)

// StateProvider 状态查询能力提供者
//
// # 核心功能：
// - 执行上下文查询：调用者地址、合约地址、初始化参数
// - 区块链状态访问：区块高度、交易数据、合约存储
// - 只读访问：所有查询操作都是只读的，不修改链状态
// - 安全封装：通过函数注入模式隐藏底层实现细节
//
// # 设计目标：
// - 安全访问：只读操作，防止意外的状态修改
// - 性能优化：直接访问底层数据，无不必要的缓存层
// - 灵活集成：通过函数注入支持不同的数据源
// - 标准接口：提供智能合约和AI模型需要的标准查询能力
//
// # 使用场景：
// - 智能合约获取执行上下文信息
// - AI模型访问区块链历史数据
// - 合约间调用的身份验证
// - 数据分析和审计功能
type StateProvider struct {
	// getCaller 调用者地址查询函数
	// 返回当前执行上下文中的调用者地址
	getCaller func() ([]byte, error)

	// getContractAddr 合约地址查询函数
	// 返回当前正在执行的合约地址
	getContractAddr func() ([]byte, error)

	// getInitParams 初始化参数获取函数
	// 返回合约部署时的初始化参数
	getInitParams func() ([]byte, error)

	// getBlockHeight 区块高度查询函数
	// 返回当前链的最新区块高度
	getBlockHeight func() (uint64, error)

	// getTransaction 交易查询函数
	// 根据交易哈希返回序列化的交易数据
	getTransaction func(hash []byte) ([]byte, error)

	// getStorage 合约存储查询函数
	// 查询指定合约地址下特定键的存储值
	getStorage func(contractAddr []byte, key []byte) ([]byte, error)
}

func NewStateProvider() *StateProvider { return &StateProvider{} }

func (p *StateProvider) CapabilityDomain() string { return "state" }

func (p *StateProvider) Register(r execiface.HostCapabilityRegistry) error {
	if err := r.RegisterProvider(p); err != nil {
		return fmt.Errorf("register state provider failed: %w", err)
	}
	return nil
}

// WithGetCaller 注入调用者查询函数
func (p *StateProvider) WithGetCaller(fn func() ([]byte, error)) *StateProvider {
	p.getCaller = fn
	return p
}

// WithGetContractAddress 注入合约地址查询函数
func (p *StateProvider) WithGetContractAddress(fn func() ([]byte, error)) *StateProvider {
	p.getContractAddr = fn
	return p
}

// WithGetInitParams 注入初始化参数获取函数
func (p *StateProvider) WithGetInitParams(fn func() ([]byte, error)) *StateProvider {
	p.getInitParams = fn
	return p
}

// WithGetBlockHeight 注入区块高度查询函数
func (p *StateProvider) WithGetBlockHeight(fn func() (uint64, error)) *StateProvider {
	p.getBlockHeight = fn
	return p
}

// WithGetTransaction 注入交易查询函数（返回序列化交易字节）
func (p *StateProvider) WithGetTransaction(fn func(hash []byte) ([]byte, error)) *StateProvider {
	p.getTransaction = fn
	return p
}

// WithGetStorage 注入合约存储查询函数
func (p *StateProvider) WithGetStorage(fn func(contractAddr []byte, key []byte) ([]byte, error)) *StateProvider {
	p.getStorage = fn
	return p
}
