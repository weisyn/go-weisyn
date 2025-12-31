// Package writegate 提供 WriteGate 接口的默认实现
//
// # 架构定位
//
// WriteGate 是 L2 基础设施层组件，位于：
//   - 接口定义：pkg/interfaces/infrastructure/writegate/
//   - 实现：internal/core/infrastructure/writegate/（本包）
//
// 与其他基础设施组件（Storage、EventBus、Logger）保持一致的架构模式。
//
// # 功能说明
//
// WriteGate 提供全局写控制能力，支持两种模式：
//
// 1. ReadOnly 模式（只读模式）
//   - 用途：系统级故障保护，完全禁止所有写操作
//   - 场景：不可恢复的数据损坏、磁盘故障等
//   - 行为：所有写操作调用 AssertWriteAllowed 都会失败
//
// 2. WriteFence 模式（写围栏）
//   - 用途：受控写入窗口，只允许特定操作写入
//   - 场景：REORG（链重组）期间需要阻止其他写操作
//   - 行为：只有携带有效 token 的操作才能通过检查
//
// # 使用方式
//
// 应用代码应依赖接口而非本实现包：
//
//	import "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
//
//	// 检查写操作是否允许
//	if err := writegate.Default().AssertWriteAllowed(ctx, "myOperation"); err != nil {
//	    return err
//	}
//
// # 设计决策
//
// 1. 为什么使用全局单例？
//   - WriteGate 提供系统级写控制，需要在所有模块间共享状态
//   - 只读模式和写围栏必须影响所有写操作，不能各自为政
//
// 2. 为什么放在基础设施层？
//   - 跨多个核心业务模块使用（Chain、Consensus、Mempool、EUTXO、URES）
//   - 提供横切关注点（写控制是所有写操作的共同需求）
//   - 无业务逻辑，纯基础设施能力
//
// 3. 为什么使用接口抽象？
//   - 解耦：各模块依赖接口，不依赖具体实现
//   - 可测试：支持 Mock 实现，便于单元测试
//   - 灵活性：支持多实例（测试场景）、不同策略
//
// # 线程安全
//
// gateImpl 使用 sync.RWMutex 保护内部状态，支持并发调用：
//   - 读操作（IsReadOnly、ReadOnlyReason、AssertWriteAllowed）使用 RLock
//   - 写操作（EnterReadOnly、ExitReadOnly、EnableWriteFence、DisableWriteFence）使用 Lock
//
// # 性能考虑
//
// AssertWriteAllowed 是热路径方法（每次写操作都会调用），性能至关重要：
//   - 使用 RWMutex.RLock，支持高并发读
//   - 只读模式检查：O(1)，单次布尔判断
//   - 写围栏检查：O(1)，字符串比较
//   - 总开销：< 100ns（现代 CPU）
//
// 编译器可能会内联接口调用，进一步降低开销。
package writegate
