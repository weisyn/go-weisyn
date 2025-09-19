// Package node 提供节点网络服务工厂实现
package node

import (
	nodeconfig "github.com/weisyn/v1/internal/config/node"
	discpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/discovery"
	hostpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/host"
	cfgprovider "github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ServiceInput 定义节点服务工厂的输入参数
type ServiceInput struct {
	Provider cfgprovider.Provider  `optional:"true"`
	Logger   logiface.Logger       `optional:"true"`
	Event    eventiface.EventBus   `optional:"true"`
	Storage  storageiface.Provider `optional:"true"`
}

// ServiceOutput 定义节点服务工厂的输出结果
type ServiceOutput struct {
	HostRuntime *hostpkg.Runtime
	DiscRuntime *discpkg.Runtime
	Host        nodeiface.Host
}

// CreateNodeServices 创建节点网络服务
//
// 🏭 **节点服务工厂**：
// 该函数负责创建节点网络相关的所有服务，包括host和discovery运行时。
// 将复杂的服务创建逻辑从module.go中分离出来，保持module.go的薄实现。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - ServiceOutput: 创建的服务实例集合
//   - error: 创建过程中的错误
func CreateNodeServices(input ServiceInput) (ServiceOutput, error) {
	// 获取节点选项：优先Provider；否则使用默认
	var nodeOpts *nodeconfig.NodeOptions
	if input.Provider != nil {
		nodeOpts = input.Provider.GetNode()
	}
	if nodeOpts == nil {
		nodeOpts = nodeconfig.New(nil).GetOptions()
	}

	// 创建host运行时
	hostRuntime, err := hostpkg.NewRuntime(nodeOpts, input.Logger)
	if err != nil {
		return ServiceOutput{}, err
	}

	// 创建discovery运行时
	discRuntime, err := discpkg.NewRuntime(nodeOpts, input.Logger, hostRuntime, input.Event, input.Storage)
	if err != nil {
		return ServiceOutput{}, err
	}

	// 创建host服务
	hostService := newHostService(hostRuntime)

	return ServiceOutput{
		HostRuntime: hostRuntime,
		DiscRuntime: discRuntime,
		Host:        hostService,
	}, nil
}
