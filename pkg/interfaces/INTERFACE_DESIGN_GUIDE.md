# 接口设计指南 (Interface Design Guide)

## 【核心原则】

### **1. 最小接口原则**
```go
// ❌ 错误：接口过大
type Service interface {
    Method1()
    Method2()
    Method3()
    // ... 20个方法
}

// ✅ 正确：接口精简
type Reader interface {
    Read(ctx context.Context, id string) (*Data, error)
}

type Writer interface {
    Write(ctx context.Context, data *Data) error
}
```

### **2. 单一职责原则**
每个接口应该只负责一个明确的功能领域。

### **3. 依赖倒置原则**
高层模块不应依赖低层模块，两者都应依赖抽象（接口）。

## 【接口分类】

### **公共接口 (pkg/interfaces)**
- **用途**：跨组件通信
- **位置**：`pkg/interfaces/*`
- **命名**：`XxxService`
- **示例**：`BlockService`, `ChainService`, `ConsensusService`

### **内部接口 (internal/core/*/interfaces)**
- **用途**：组件内部使用
- **位置**：`internal/core/*/interfaces/*`
- **命名**：根据具体功能命名
- **示例**：`BlockManager`, `TransactionValidator`

## 【设计规范】

### **1. 接口命名**
```go
// 服务接口：XxxService
type BlockService interface {}

// 管理器接口：XxxManager
type RepositoryManager interface {}

// 提供者接口：XxxProvider
type StorageProvider interface {}

// 处理器接口：XxxHandler
type EventHandler interface {}
```

### **2. 方法命名**
```go
type Service interface {
    // 查询方法：Get/List/Find
    GetBlock(ctx context.Context, hash []byte) (*Block, error)
    ListBlocks(ctx context.Context, offset, limit int) ([]*Block, error)
    
    // 创建方法：Create/Build
    CreateBlock(ctx context.Context, txs []*Transaction) (*Block, error)
    
    // 更新方法：Update/Modify
    UpdateState(ctx context.Context, state *State) error
    
    // 删除方法：Delete/Remove
    DeleteTransaction(ctx context.Context, hash []byte) error
    
    // 验证方法：Validate/Verify
    ValidateBlock(ctx context.Context, block *Block) error
    
    // 执行方法：Execute/Process
    ProcessTransaction(ctx context.Context, tx *Transaction) error
}
```

### **3. 参数设计**
```go
// ✅ 使用 context.Context 作为第一个参数
func (s *Service) GetData(ctx context.Context, id string) (*Data, error)

// ✅ 使用结构体传递多个参数
type QueryParams struct {
    Offset int
    Limit  int
    Filter string
}
func (s *Service) Query(ctx context.Context, params QueryParams) ([]*Data, error)

// ❌ 避免过多的参数
func (s *Service) BadMethod(ctx context.Context, a, b, c, d, e string) error
```

### **4. 返回值设计**
```go
// ✅ 返回具体类型和 error
func GetBlock(ctx context.Context, hash []byte) (*Block, error)

// ✅ 对于可能不存在的资源，返回 nil 和 nil
func FindBlock(ctx context.Context, hash []byte) (*Block, error) {
    // 如果不存在，返回 (nil, nil)
    // 如果出错，返回 (nil, error)
}

// ❌ 避免返回 interface{}
func GetSomething() interface{}
```

## 【依赖管理】

### **1. 正确的依赖方向**
```
外部调用者
    ↓
pkg/interfaces（公共接口）
    ↑
internal/core/adapters（适配层）
    ↓
internal/core/implementations（实现层）
```

### **2. 禁止的依赖**
```go
// ❌ 禁止：跨组件内部依赖
import "github.com/weisyn/v1/internal/core/blockchain"  // 在 consensus 中

// ❌ 禁止：pkg 依赖 internal
import "github.com/weisyn/v1/internal/core/*"  // 在 pkg/interfaces 中

// ✅ 正确：通过公共接口
import "github.com/weisyn/v1/pkg/interfaces/blockchain"
```

## 【版本管理】

### **1. 向后兼容**
```go
// v1 接口
type ServiceV1 interface {
    Method1() error
}

// v2 接口（向后兼容）
type ServiceV2 interface {
    ServiceV1  // 嵌入 v1 接口
    Method2() error  // 新增方法
}
```

### **2. 废弃标记**
```go
// Deprecated: 使用 NewMethod 代替
func OldMethod() error {
    return NewMethod()
}
```

## 【测试指南】

### **1. Mock 接口**
```go
type MockBlockService struct {
    mock.Mock
}

func (m *MockBlockService) GetBlock(ctx context.Context, hash []byte) (*Block, error) {
    args := m.Called(ctx, hash)
    return args.Get(0).(*Block), args.Error(1)
}
```

### **2. 接口测试**
```go
func TestBlockService(t *testing.T) {
    // 测试所有接口实现
    var service BlockService = NewBlockServiceImpl()
    
    // 测试接口方法
    block, err := service.GetBlock(context.Background(), []byte("hash"))
    assert.NoError(t, err)
    assert.NotNil(t, block)
}
```

## 【检查清单】

在添加或修改接口前，请确认：

- [ ] 接口是否符合单一职责原则？
- [ ] 接口大小是否合理（建议不超过5-7个方法）？
- [ ] 方法命名是否清晰一致？
- [ ] 是否所有方法都使用 context.Context？
- [ ] 返回值是否包含 error？
- [ ] 是否避免了 interface{} 类型？
- [ ] 是否有适当的文档注释？
- [ ] 是否考虑了向后兼容性？
- [ ] 是否在正确的位置（pkg/interfaces vs internal）？
- [ ] 是否运行了依赖检查脚本？

## 【常见错误】

### **1. 循环依赖**
```go
// ❌ package A 依赖 B，B 又依赖 A
// 解决：提取公共接口到 pkg/interfaces
```

### **2. 接口污染**
```go
// ❌ 为每个结构体都创建接口
// 正确：只在需要抽象时创建接口
```

### **3. 泄露实现**
```go
// ❌ 接口方法返回具体类型
func GetImpl() *ConcreteType

// ✅ 返回接口类型
func GetService() Service
```

## 【工具支持】

### **依赖检查**
```bash
./scripts/check_dependencies.sh
```

### **接口生成**
```bash
# 使用 mockery 生成 mock
mockery --name=BlockService --dir=pkg/interfaces/blockchain

# 使用 ifacemaker 从实现生成接口
ifacemaker -f block_service.go -s BlockServiceImpl -i BlockService
```

## 【参考资源】

- [Effective Go - Interfaces](https://golang.org/doc/effective_go#interfaces)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [SOLID Principles in Go](https://dave.cheney.net/2016/08/20/solid-go-design)

---

*最后更新：2024-12-xx*
*维护者：WES Team*
