// Package abi 提供合约应用二进制接口（ABI）的管理、编解码、验证与兼容性检查功能。
//
// 本包遵循接口集中化架构约束，将扩展性接口集中到 internal/core/execution/interfaces，
// 实现包专注于具体功能实现，通过 fx 依赖注入支持策略组件的灵活替换和扩展。
//
// 主要特性：
//   - ABI 注册管理：合约 ABI 的注册、存储和检索
//   - 参数编解码：函数参数和返回值的标准化编解码
//   - 类型验证：ABI 定义的结构完整性和类型安全验证
//   - 版本兼容性：ABI 版本间的兼容性检查和迁移支持
//   - 统计监控：ABI 使用统计和性能监控
//
// 设计原则：
//   - 类型复用：复用 pkg/types 统一类型定义，避免重复定义和类型转换
//   - 高内聚低耦合：专注 ABI 管理职责，不涉及执行调度、网络通信等跨域功能
//   - 依赖倒置：通过 fx 依赖注入提供服务，支持策略组件的灵活替换
//   - 可测试性：接口驱动设计，支持单元测试和集成测试
package abi

import (
	typespkg "github.com/weisyn/v1/pkg/types"
)

// 类型别名定义说明：
// 为与项目统一类型保持一致，ABI 相关模型统一复用 pkg/types 中的定义。
// 下述类型为别名，禁止在本子包重复定义结构字段，避免类型漂移和维护成本。

// ContractABI 合约应用二进制接口定义的类型别名。
// 复用 pkg/types.ContractABI，确保类型一致性和向后兼容性。
// 包含合约的完整接口描述，包括函数、事件、错误等定义。
type ContractABI = typespkg.ContractABI

// ContractFunction 合约函数定义的类型别名。
// 复用 pkg/types.ContractFunction，描述合约中单个函数的接口规范。
// 包含函数名称、参数列表、返回值类型、可见性等信息。
type ContractFunction = typespkg.ContractFunction

// ABIParam ABI 参数定义的类型别名。
// 复用 pkg/types.ABIParam，描述函数参数或返回值的类型信息。
// 包含参数名称、数据类型、索引状态等属性。
type ABIParam = typespkg.ABIParam

// ContractEvent 合约事件定义的类型别名。
// 复用 pkg/types.ContractEvent，描述合约事件的接口规范。
// 包含事件名称、参数列表、匿名状态等信息。
type ContractEvent = typespkg.ContractEvent
