# WriteGate 接口

## 概述

本包定义了 WriteGate 的公共接口，供所有需要写控制的模块使用。

## 接口定义

### WriteGate 接口

全局写门闸接口，提供系统级写操作控制：

```go
type WriteGate interface {
    // 只读模式控制
    EnterReadOnly(reason string)
    ExitReadOnly()
    IsReadOnly() bool
    ReadOnlyReason() string
    
    // 写围栏控制（REORG 专用）
    EnableWriteFence(purpose string) (token string, err error)
    DisableWriteFence(token string) error
    
    // 写操作检查
    AssertWriteAllowed(ctx context.Context, operation string) error
}
```

### Context 辅助函数

```go
// WithWriteToken 将 token 绑定到 context
func WithWriteToken(ctx context.Context, token string) context.Context

// TokenFromContext 从 context 中读取 token
func TokenFromContext(ctx context.Context) string
```

### 全局访问函数

```go
// Default 返回全局默认实例
func Default() WriteGate

// SetDefault 设置全局默认实例（实现层使用）
func SetDefault(gate WriteGate)
```

## 使用示例

### 基本写操作检查

```go
import "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"

func (s *Service) WriteData(ctx context.Context, data []byte) error {
    // 检查写操作是否允许
    if err := writegate.Default().AssertWriteAllowed(ctx, "WriteData"); err != nil {
        return err
    }
    
    // 执行写操作
    return s.storage.Write(data)
}
```

### 只读模式

```go
// 进入只读模式（系统级故障保护）
writegate.Default().EnterReadOnly("corruption detected")
defer writegate.Default().ExitReadOnly()

// 检查状态
if writegate.Default().IsReadOnly() {
    reason := writegate.Default().ReadOnlyReason()
    log.Warnf("System is read-only: %s", reason)
}
```

### 写围栏（REORG 场景）

```go
// 开启写围栏
token, err := writegate.Default().EnableWriteFence("reorg")
if err != nil {
    return err
}
defer writegate.Default().DisableWriteFence(token)

// 创建携带 token 的 context
ctx = writegate.WithWriteToken(ctx, token)

// 使用 ctx 进行受控写操作
if err := executeReorg(ctx); err != nil {
    return err
}
```

## 设计原则

### 接口优先

- 应用代码只依赖本包的接口，不依赖具体实现
- 实现层位于 `internal/core/infrastructure/writegate/`
- 通过 `Default()` 获取全局实例

### 简洁清晰

- 接口方法语义明确，易于理解
- Context 辅助函数简化 token 操作
- 错误信息包含足够的调试信息

### 可测试性

- 接口易于 Mock
- 支持通过 `SetDefault()` 注入测试实现
- 测试可创建独立实例

## 实现注册

实现层通过 `init()` 函数注册默认实例：

```go
// internal/core/infrastructure/writegate/singleton.go
func init() {
    wgif.SetDefault(&gateImpl{})
}
```

## 依赖关系

```
应用代码
    ↓ 依赖
pkg/interfaces/infrastructure/writegate/（本包）
    ↑ 实现
internal/core/infrastructure/writegate/
```

## 参考资料

- 实现文档：[internal/core/infrastructure/writegate/README.md](../../../../internal/core/infrastructure/writegate/README.md)
- 重构方案：[09-WriteGate架构重构方案.md](../../../../_dev/14-实施任务-implementation-tasks/20251215-16-defect-reports-summary/09-WriteGate架构重构方案.md)

