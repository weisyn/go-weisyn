# WriteGate 实现

## 概述

WriteGate 是 WES 系统的全局写门闸组件，位于 L2 基础设施层，提供系统级的写操作控制能力。

## 架构定位

```
L4 核心业务层
  ├── Chain/Fork
  ├── Consensus (Miner/Aggregator)
  ├── Mempool
  └── Block
      ↓ 依赖接口
L3 执行与状态层
  ├── EUTXO
  └── URES
      ↓ 依赖接口
L2 基础设施层（本组件）
  ├── WriteGate ⭐
  ├── Storage
  ├── EventBus
  └── Logger
```

## 功能说明

WriteGate 提供三种写控制模式：

### 1. ReadOnly 模式（只读模式）

**用途**：系统级故障保护，完全禁止所有写操作

**场景**：
- 不可恢复的数据损坏
- 磁盘故障
- 系统紧急维护

**行为**：
- 所有写操作调用 `AssertWriteAllowed` 都会失败
- 返回包含原因的错误信息
- 写围栏会被自动清除（只读优先级最高）

**API**：
```go
gate.EnterReadOnly("corruption detected")
defer gate.ExitReadOnly()

// 检查状态
isReadOnly := gate.IsReadOnly()
reason := gate.ReadOnlyReason()
```

### 2. WriteFence 模式（写围栏）

**用途**：受控写入窗口，只允许持有特定 token 的操作写入

**场景**：
- REORG（链重组）期间需要阻止其他写操作
- 需要确保写操作的原子性和一致性
- 多步骤操作需要排他性写入

**行为**：
- 生成唯一的 token 作为写操作通行证
- 只有通过 `WithWriteToken` 携带该 token 的 context 才能通过检查
- 其他所有写操作都会被阻止

**API**：
```go
// REORG 场景示例
token, err := gate.EnableWriteFence("reorg")
if err != nil {
    return err
}
defer gate.DisableWriteFence(token)

// 将 token 绑定到 context
ctx = writegate.WithWriteToken(ctx, token)

// 使用 ctx 进行受控写操作
err = someWriteOperation(ctx)
```

### 3. RecoveryMode 模式（恢复模式）

**用途**：系统自动修复，允许在只读模式下执行受控的恢复操作

**场景**：
- 自省重建（Self-Introspection Rebuild）
- 链尖修复（Chain Tip Repair）
- 其他关键恢复操作

**行为**：
- 生成唯一的 recovery token 作为特权通行证
- 即使在只读模式下，携带该 token 的写操作也允许通过
- Recovery token 优先级高于 ReadOnly
- 同时只能有一个 recovery token 活跃

**优先级规则**：
```
RecoveryToken > ReadOnly > WriteFenceToken > Normal
```

**API**：
```go
// 自省修复场景示例
token, err := gate.EnableRecoveryMode("self-introspection-rebuild")
if err != nil {
    return err
}
defer gate.DisableRecoveryMode(token)

// 将 token 绑定到 context
ctx = writegate.WithWriteToken(ctx, token)

// 使用 ctx 进行恢复写操作（即使在只读模式下也允许）
err = blockProcessor.ProcessBlock(ctx, genesis)
```

**安全性**：
- Recovery token 有严格的生命周期控制
- 必须显式启用和禁用
- 同时只能有一个活跃
- 所有操作都会记录日志
- 不受只读模式限制，需要谨慎使用

**使用场景说明**：

Recovery Mode 设计用于解决"架构死锁"问题：当节点因严重错误（如 BadgerDB 事务超限）进入只读模式后，传统的修复机制（如自省重建）无法执行，因为它们也需要写入数据。Recovery Mode 通过提供特权写入通道，允许系统在只读模式下执行必要的修复操作，从而实现自动恢复。

**示例：自省修复中使用**：
```go
func (s *Service) rebuildChainByLocalPrefixAndForkProvider(ctx context.Context, ...) error {
    // 启用 Recovery Mode（允许在只读模式下执行修复）
    var recoveryToken string
    var recoveryEnabled bool
    if s.writeGate != nil {
        tok, err := s.writeGate.EnableRecoveryMode("self-introspection-rebuild")
        if err != nil {
            return fmt.Errorf("启用恢复模式失败: %w", err)
        }
        recoveryToken = tok
        recoveryEnabled = true
        defer func() {
            if recoveryEnabled {
                _ = s.writeGate.DisableRecoveryMode(recoveryToken)
            }
        }()
        
        // 将 recovery token 绑定到 context
        ctx = writegate.WithWriteToken(ctx, recoveryToken)
        
        s.logger.Infof("🔧 自省修复：已启用恢复模式")
    }
    
    // 执行修复操作（即使在只读模式下也能写入）
    // ...
    
    return nil
}
```

## 使用方式

### 基本使用

```go
import "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"

// 在写操作前检查
func (s *Service) WriteData(ctx context.Context, data []byte) error {
    // 检查写操作是否允许
    if err := writegate.Default().AssertWriteAllowed(ctx, "myService.WriteData"); err != nil {
        return err
    }
    
    // 执行实际的写操作
    return s.doWrite(data)
}
```

### REORG 场景使用

```go
func (s *Service) ExecuteReorg(ctx context.Context) error {
    // 1. 开启写围栏
    token, err := writegate.Default().EnableWriteFence("reorg")
    if err != nil {
        return err
    }
    defer writegate.Default().DisableWriteFence(token)
    
    // 2. 创建携带 token 的 context
    ctx = writegate.WithWriteToken(ctx, token)
    
    // 3. 执行 REORG 操作（其他写操作会被阻止）
    if err := s.rollbackBlocks(ctx); err != nil {
        return err
    }
    
    if err := s.applyNewBlocks(ctx); err != nil {
        return err
    }
    
    return nil
}
```

### 测试中使用

```go
import (
    _ "github.com/weisyn/v1/internal/core/infrastructure/writegate" // 导入实现包
)

func TestMyFunction(t *testing.T) {
    // 全局实例会自动注册
    // 测试代码可以直接使用
}
```

## 设计决策

### 为什么使用全局单例？

1. **系统级写控制**：WriteGate 提供系统级写控制，需要在所有模块间共享状态
2. **一致性保证**：只读模式和写围栏必须影响所有写操作，不能各自为政
3. **简化使用**：避免在各模块间传递 WriteGate 实例

### 为什么放在基础设施层？

1. **跨模块使用**：被多个核心业务模块使用（Chain、Consensus、Mempool、EUTXO、URES）
2. **横切关注点**：写控制是所有写操作的共同需求
3. **无业务逻辑**：纯基础设施能力，不包含业务逻辑
4. **系统级状态**：只读模式和写围栏影响整个节点

### 为什么使用接口抽象？

1. **解耦**：各模块依赖接口，不依赖具体实现
2. **可测试**：支持 Mock 实现，便于单元测试
3. **灵活性**：支持多实例（测试场景）、不同策略

## 实现细节

### 线程安全

`gateImpl` 使用 `sync.RWMutex` 保护内部状态，支持并发调用：
- 读操作（`IsReadOnly`、`ReadOnlyReason`、`AssertWriteAllowed`）使用 `RLock`
- 写操作（`EnterReadOnly`、`ExitReadOnly`、`EnableWriteFence`、`DisableWriteFence`）使用 `Lock`

### 性能考虑

`AssertWriteAllowed` 是热路径方法（每次写操作都会调用），性能至关重要：
- 使用 `RWMutex.RLock`，支持高并发读
- 只读模式检查：O(1)，单次布尔判断
- 写围栏检查：O(1)，字符串比较
- 总开销：< 100ns（现代 CPU）

编译器可能会内联接口调用，进一步降低开销。

### Token 生成

使用 `crypto/rand` 生成 128 位（16 字节）随机 token，编码为 32 字符十六进制字符串，确保安全性和唯一性。

## 文件结构

```
pkg/interfaces/infrastructure/writegate/  # 接口层
├── gate.go         # WriteGate 接口定义
├── context.go      # Context 辅助函数
└── singleton.go    # 全局访问函数

internal/core/infrastructure/writegate/   # 实现层（本包）
├── gate.go         # gateImpl 实现
├── gate_test.go    # 单元测试
├── singleton.go    # 全局单例注册
├── doc.go          # 包文档
└── README.md       # 本文件
```

## 测试

### 单元测试

```bash
go test ./internal/core/infrastructure/writegate/... -v
```

测试覆盖：
- ✅ ReadOnly 模式进入/退出
- ✅ ReadOnly 模式阻止写操作
- ✅ WriteFence 开启/关闭
- ✅ WriteFence 阻止无 token 写操作
- ✅ WriteFence 需要正确的 token
- ✅ ReadOnly 清除 WriteFence
- ✅ RecoveryMode 基本功能
- ✅ RecoveryMode 绕过 ReadOnly
- ✅ RecoveryMode Token 不匹配
- ✅ RecoveryMode 优先级
- ✅ 并发安全性
- ✅ Context token 操作

### 集成测试

REORG 相关的集成测试在 `internal/core/chain/fork/` 中。

## 迁移记录

**重构时间**：2024-12

**重构原因**：
- 旧位置：`internal/core/chain/writegate/`（架构违规）
- 问题：多个 L4 模块依赖 chain 模块的 internal 实现，违反分层架构原则
- 解决：移至 L2 基础设施层，提供接口抽象

**重构影响**：
- 迁移了 11 个文件跨 6 个模块
- 所有使用点更新为依赖接口
- 添加了完整的单元测试

## 参考资料

- [09-WriteGate架构重构方案.md](/_dev/14-实施任务-implementation-tasks/20251215-16-defect-reports-summary/09-WriteGate架构重构方案.md)
- [代码组织规范](/_dev/04-工程标准-standards/01-代码与接口标准-code-and-interfaces/01-CODE_ORGANIZATION_STANDARD.md)
- [分层架构模型](/_dev/02-架构设计-architecture/01-分层与模块架构-layers-and-modules/01-LAYERED_MODEL.md)

