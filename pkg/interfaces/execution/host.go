package execution

// 宿主能力提供者抽象：按能力域（返回负载/事件/上下文/UTXO/资源）提供宿主函数
// 由区块链计算层实现并注册到宿主能力注册器
// 引擎仅通过 HostBinding 暴露的函数指针调用，不感知区块链实现细节

type HostCapabilityProvider interface {
	// CapabilityDomain 返回能力域名称（如 return_data、events、context、utxo、resource）
	CapabilityDomain() string

	// Register 将本能力域的函数注册进宿主标准接口
	Register(registry HostCapabilityRegistry) error
}

// 宿主能力注册器：聚合各能力提供者，构建标准宿主接口
// 由区块链计算层实现

type HostCapabilityRegistry interface {
	// RegisterProvider 注册能力提供者
	RegisterProvider(p HostCapabilityProvider) error

	// BuildStandardInterface 构建并返回标准宿主接口
	BuildStandardInterface() HostStandardInterface
}

// 标准宿主接口：引擎侧仅依赖本接口进行回调
// 由区块链计算层汇总实现，不暴露区块链实现细节

type HostStandardInterface interface {
	// 返回负载能力
	SetReturnData(data []byte) error

	// 事件发射能力
	EmitEvent(eventType string, payload []byte) error

	// 上下文查询能力
	GetCaller() ([]byte, error)
	GetContractAddress() ([]byte, error)

	// UTXO 副作用能力（以“请求”形式收集，最终由链侧落账）
	CreateUTXOOutput(recipient []byte, amount uint64) error
	ExecuteUTXOTransfer(from, to []byte, amount uint64) error

	// 资源只读查询能力
	QueryUTXOBalance(address []byte) (uint64, error)
	GetContractInitParams() ([]byte, error)
	GetStorage(contractAddr []byte, key []byte) ([]byte, error)

	// 区块状态只读能力
	GetBlockHeight() (uint64, error)

	// 账户余额只读能力
	GetBalance(address []byte) (uint64, error)

	// 交易只读能力（返回序列化交易字节，由引擎侧自行解码）
	GetTransaction(txHash []byte) ([]byte, error)
}

// 宿主绑定：将标准宿主接口绑定为特定引擎可调用的函数集合
// 由区块链计算层实现，供引擎在初始化阶段绑定

type HostBinding interface {
	// Standard 返回标准宿主接口
	Standard() HostStandardInterface
}
