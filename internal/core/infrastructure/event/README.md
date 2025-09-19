# 事件总线基础设施实现 (Event Bus Infrastructure Implementation)

## 【模块定位】

　　**事件总线基础设施模块**是WES区块链系统事件通信机制的核心实现。本模块基于asaskevich/EventBus构建，并扩展了域注册、智能路由、事件验证等企业级功能，为整个系统提供高性能、可靠的异步事件通信服务。

## 【设计理念】

### 基础设施职责

　　作为基础设施层，本模块遵循"**提供机制，不定义业务**"的原则：
- 提供事件发布、订阅、路由等核心机制
- 不硬编码任何业务域名称
- 支持组件动态注册和自管理
- 保持最大的灵活性和可扩展性

### 架构设计

```mermaid
graph TB
    subgraph "EventBus实现架构"
        subgraph "核心层"
            BASE["asaskevich/EventBus<br/>底层事件总线"]
            ENHANCED["EnhancedEventBus<br/>增强封装"]
        end
        
        subgraph "功能层"
            REGISTRY["DomainRegistry<br/>域注册管理"]
            ROUTER["EventRouter<br/>智能路由"]
            VALIDATOR["EventValidator<br/>事件验证"]
            MIDDLEWARE["Middleware<br/>中间件链"]
        end
        
        subgraph "管理层"
            COORDINATOR["EventCoordinator<br/>全局协调器"]
            LIFECYCLE["Lifecycle<br/>生命周期管理"]
            METRICS["Metrics<br/>内部指标"]
        end
    end
    
    BASE --> ENHANCED
    ENHANCED --> REGISTRY
    ENHANCED --> ROUTER
    ENHANCED --> VALIDATOR
    ROUTER --> MIDDLEWARE
    COORDINATOR --> ENHANCED
    COORDINATOR --> LIFECYCLE
    LIFECYCLE --> METRICS
    
    style BASE fill:#e3f2fd
    style ENHANCED fill:#fff9c4
    style COORDINATOR fill:#f3e5f5
```

## 【核心组件】

### EventBus - 事件总线核心

#### 基础功能
- **发布订阅**：支持同步/异步的事件发布和订阅
- **一次性订阅**：支持只响应一次的事件订阅
- **批量操作**：支持批量发布和订阅
- **历史记录**：可选的事件历史记录功能

#### 增强功能
- **域管理**：动态域注册和验证
- **智能路由**：基于规则的事件路由
- **优先级处理**：支持事件优先级队列
- **中间件支持**：可插拔的中间件机制

### DomainRegistry - 域注册中心

```mermaid
classDiagram
    class DomainRegistry {
        -domains map[string]*DomainInfo
        -routes map[string][]string
        -mu sync.RWMutex
        +RegisterDomain(domain, info) error
        +UnregisterDomain(domain) error
        +IsDomainRegistered(domain) bool
        +GetDomainInfo(domain) *DomainInfo
        +ListDomains() []string
        +ValidateEventName(name) error
    }
    
    class DomainInfo {
        +Name string
        +Component string
        +Description string
        +EventTypes []string
        +RegisteredAt time.Time
        +Active bool
    }
    
    DomainRegistry --> DomainInfo
```

### EventRouter - 智能路由器

```mermaid
graph LR
    subgraph "路由决策流程"
        EVENT["事件输入"]
        EXTRACT["提取域名"]
        VALIDATE["验证域"]
        MATCH["匹配订阅"]
        PRIORITY["优先级排序"]
        DISPATCH["分发执行"]
    end
    
    EVENT --> EXTRACT
    EXTRACT --> VALIDATE
    VALIDATE --> MATCH
    MATCH --> PRIORITY
    PRIORITY --> DISPATCH
```

#### 路由策略

| 策略 | 说明 | 应用场景 |
|------|------|---------|
| Direct | 直接路由到指定订阅者 | 点对点通信 |
| Broadcast | 广播到所有订阅者 | 状态同步 |
| RoundRobin | 轮询分发 | 负载均衡 |
| Priority | 按优先级分发 | 关键事件处理 |
| Filter | 基于条件过滤 | 选择性处理 |

### EventValidator - 事件验证器

```mermaid
sequenceDiagram
    participant Producer as 生产者
    participant Validator as 验证器
    participant Bus as EventBus
    participant Consumer as 消费者
    
    Producer->>Validator: 发布事件
    
    alt 验证通过
        Validator->>Bus: 传递事件
        Bus->>Consumer: 分发事件
        Consumer-->>Bus: 处理完成
    else 验证失败
        Validator-->>Producer: 返回错误
        Note over Producer: 处理验证错误
    end
```

## 【实现细节】

### 模块初始化

```mermaid
graph TB
    subgraph "初始化流程"
        CONFIG["加载配置"]
        CREATE["创建EventBus"]
        ENHANCE["增强功能"]
        REGISTER["注册到DI"]
        START["启动服务"]
    end
    
    CONFIG --> CREATE
    CREATE --> ENHANCE
    ENHANCE --> REGISTER
    REGISTER --> START
```

### 事件发布流程

```mermaid
sequenceDiagram
    participant Component as 组件
    participant Publisher as 发布器
    participant Validator as 验证器
    participant Router as 路由器
    participant Handler as 处理器
    
    Component->>Publisher: Publish(event)
    Publisher->>Validator: 验证事件
    Validator->>Router: 路由决策
    Router->>Handler: 异步分发
    Handler-->>Router: 确认
    Router-->>Publisher: 完成
```

### 事件订阅流程

```mermaid
graph TB
    subgraph "订阅注册流程"
        SUB_REQ["订阅请求"]
        CHECK_DOMAIN["检查域权限"]
        REGISTER_HANDLER["注册处理器"]
        CREATE_SUB["创建订阅信息"]
        STORE_SUB["存储订阅关系"]
        RETURN_ID["返回订阅ID"]
    end
    
    SUB_REQ --> CHECK_DOMAIN
    CHECK_DOMAIN --> REGISTER_HANDLER
    REGISTER_HANDLER --> CREATE_SUB
    CREATE_SUB --> STORE_SUB
    STORE_SUB --> RETURN_ID
```

## 【配置管理】

### 配置结构

```go
type Config struct {
    // 基础配置
    Enabled                bool   // 是否启用
    
    // 增强功能开关
    EnableEnhancedFeatures bool   // 启用增强功能
    EnableDomainRegistry   bool   // 启用域注册
    EnableSmartRouter      bool   // 启用智能路由
    EnableValidator        bool   // 启用验证器
    
    // 域管理配置
    StrictDomainCheck      bool   // 严格域检查
    WarnCrossDomain        bool   // 跨域警告
    AllowUnregisteredDomain bool  // 允许未注册域
    
    // 性能配置
    AsyncPublish          bool    // 异步发布
    WorkerPoolSize        int     // 工作池大小
    BufferSize            int     // 缓冲区大小
    MaxRetries            int     // 最大重试次数
    RetryInterval         time.Duration // 重试间隔
    
    // 监控配置
    EnableMetrics         bool    // 启用指标
    MetricsInterval       time.Duration // 指标更新间隔
}
```

### 配置优先级

```mermaid
graph LR
    subgraph "配置来源优先级"
        ENV["环境变量<br/>最高优先级"]
        FILE["配置文件<br/>次优先级"]
        DEFAULT["默认值<br/>最低优先级"]
    end
    
    ENV -->|覆盖| FILE
    FILE -->|覆盖| DEFAULT
```

## 【性能优化】

### 优化策略

```mermaid
graph TB
    subgraph "性能优化措施"
        subgraph "发布优化"
            ASYNC_PUB["异步发布"]
            BATCH_PUB["批量发布"]
            POOL_PUB["发布池"]
        end
        
        subgraph "订阅优化"
            SUB_CACHE["订阅缓存"]
            ROUTE_CACHE["路由缓存"]
            HANDLER_POOL["处理器池"]
        end
        
        subgraph "资源优化"
            BUFFER_REUSE["缓冲区复用"]
            GOROUTINE_POOL["协程池"]
            MEMORY_POOL["内存池"]
        end
    end
```

### 性能基准

| 操作 | 基准值 | 测试条件 |
|------|--------|---------|
| 单事件发布 | < 100μs | 1个订阅者 |
| 批量发布(100) | < 1ms | 10个订阅者 |
| 事件路由 | < 10μs | 100个订阅者 |
| 端到端延迟 | < 1ms | 正常负载 |

## 【错误处理】

### 错误分类和处理

```mermaid
graph TB
    subgraph "错误处理策略"
        E_TYPE["错误类型判断"]
        
        subgraph "可恢复错误"
            RETRY["自动重试"]
            BACKOFF["退避算法"]
            FALLBACK["降级处理"]
        end
        
        subgraph "不可恢复错误"
            LOG_ERROR["记录错误"]
            NOTIFY["通知组件"]
            SKIP["跳过处理"]
        end
        
        subgraph "严重错误"
            CIRCUIT["熔断机制"]
            DEGRADE["服务降级"]
            ALERT["告警通知"]
        end
    end
    
    E_TYPE --> RETRY
    E_TYPE --> LOG_ERROR
    E_TYPE --> CIRCUIT
```

### 重试机制

| 策略 | 说明 | 参数 |
|------|------|------|
| 线性退避 | 固定间隔重试 | interval=1s |
| 指数退避 | 指数增长间隔 | base=1s, factor=2 |
| 随机退避 | 随机间隔重试 | min=1s, max=10s |
| 有限重试 | 限制重试次数 | maxRetries=3 |

## 【监控指标】

### 内部指标

```mermaid
graph LR
    subgraph "监控指标体系"
        subgraph "吞吐量指标"
            TOTAL["总事件数"]
            RATE["事件速率"]
            PEAK["峰值QPS"]
        end
        
        subgraph "延迟指标"
            P50["P50延迟"]
            P95["P95延迟"]
            P99["P99延迟"]
        end
        
        subgraph "错误指标"
            ERROR_RATE["错误率"]
            RETRY_COUNT["重试次数"]
            FAILED_COUNT["失败数量"]
        end
        
        subgraph "资源指标"
            GOROUTINES["协程数"]
            MEMORY["内存占用"]
            BUFFER_USAGE["缓冲区使用率"]
        end
    end
```

## 【扩展机制】

### 中间件支持

```mermaid
sequenceDiagram
    participant Event as 事件
    participant MW1 as 中间件1
    participant MW2 as 中间件2
    participant MW3 as 中间件3
    participant Handler as 处理器
    
    Event->>MW1: PreProcess
    MW1->>MW2: PreProcess
    MW2->>MW3: PreProcess
    MW3->>Handler: Execute
    Handler-->>MW3: Result
    MW3-->>MW2: PostProcess
    MW2-->>MW1: PostProcess
    MW1-->>Event: Complete
```

### 插件接口

| 接口 | 功能 | 示例 |
|------|------|------|
| EventFilter | 事件过滤 | 权限过滤、内容过滤 |
| EventTransformer | 事件转换 | 格式转换、数据映射 |
| EventValidator | 事件验证 | Schema验证、业务规则 |
| EventInterceptor | 事件拦截 | 日志记录、审计追踪 |

## 【测试支持】

### 测试工具

```mermaid
graph TB
    subgraph "测试工具集"
        MOCK["MockEventBus<br/>模拟事件总线"]
        RECORDER["EventRecorder<br/>事件记录器"]
        GENERATOR["EventGenerator<br/>事件生成器"]
        VALIDATOR["TestValidator<br/>测试验证器"]
        BENCHMARK["Benchmark<br/>性能基准"]
    end
    
    subgraph "测试类型"
        UNIT["单元测试"]
        INTEGRATION["集成测试"]
        LOAD["负载测试"]
        CHAOS["混沌测试"]
    end
    
    MOCK --> UNIT
    RECORDER --> INTEGRATION
    GENERATOR --> LOAD
    VALIDATOR --> CHAOS
```

## 【故障处理】

### 故障场景

| 场景 | 处理策略 | 恢复机制 |
|------|---------|---------|
| 处理器panic | 捕获并记录 | 重启处理器 |
| 事件堆积 | 流量控制 | 扩容缓冲区 |
| 内存泄漏 | 监控告警 | 重启服务 |
| 死锁 | 超时检测 | 强制释放 |

## 【最佳实践】

### 实现建议

#### DO - 推荐做法

- ✅ 使用对象池减少GC压力
- ✅ 实现优雅关闭机制
- ✅ 添加必要的监控指标
- ✅ 使用context进行超时控制
- ✅ 实现幂等的事件处理

#### DON'T - 避免做法

- ❌ 在处理器中执行阻塞操作
- ❌ 忽略错误处理
- ❌ 创建过多的goroutine
- ❌ 使用全局变量存储状态
- ❌ 硬编码配置参数

## 【维护指南】

### 日常维护

- 定期检查错误日志
- 监控性能指标趋势
- 清理过期的订阅关系
- 更新依赖库版本
- 执行性能基准测试

### 故障排查

```mermaid
graph TB
    subgraph "故障排查流程"
        SYMPTOM["症状分析"]
        LOG["查看日志"]
        METRICS["检查指标"]
        TRACE["事件追踪"]
        PROFILE["性能分析"]
        FIX["修复问题"]
    end
    
    SYMPTOM --> LOG
    LOG --> METRICS
    METRICS --> TRACE
    TRACE --> PROFILE
    PROFILE --> FIX
```

## 【版本历史】

| 版本 | 变更内容 | 日期 |
|------|---------|------|
| v1.0.0 | 初始版本，基础EventBus | 2024-01 |
| v1.1.0 | 添加域注册功能 | 2024-02 |
| v1.2.0 | 实现智能路由 | 2024-03 |
| v1.3.0 | 添加中间件支持 | 2024-04 |

## 【附录】

### 依赖项

- github.com/asaskevich/EventBus - 底层事件总线
- go.uber.org/fx - 依赖注入框架
- github.com/google/uuid - UUID生成

### 相关模块

- pkg/interfaces/infrastructure/event - 接口定义
- internal/config/event - 配置管理
- internal/core/*/integration/event - 集成示例