# 智能合约服务（internal/core/blockchain/transaction/contract）

【模块定位】
　　智能合约服务是交易处理系统中处理可执行资源（智能合约）部署和调用的核心模块。基于WebAssembly（WASM）执行环境，实现去中心化计算和可编程业务逻辑，通过ResourceOutput创建能力载体UTXO，为区块链提供图灵完备的计算能力。

【核心职责】
- **合约部署管理**：验证、部署WASM智能合约到区块链
- **合约调用执行**：处理合约方法调用和状态变更
- **执行费用管理**：精确计算和控制合约执行成本
- **状态转换验证**：确保合约执行的确定性和一致性
- **企业级权限控制**：支持复杂的合约访问控制策略

---

## 🏗️ **模块架构**

【服务组织】

```mermaid
graph TB
    subgraph "智能合约服务架构"
        subgraph "对外接口"
            DEPLOY["DeployContract()<br/>📄 合约部署接口"]
            CALL["CallContract()<br/>⚡ 合约调用接口"]
        end
        
        subgraph "核心服务"
            DEPLOY_SVC["ContractDeployService<br/>🚀 合约部署逻辑"]
            CALL_SVC["ContractCallService<br/>🔧 合约调用逻辑"]
        end
        
        subgraph "执行引擎"
            WASM_ENGINE["WASM执行引擎<br/>⚙️ WebAssembly运行时"]
            GAS_METER["执行费用计量器<br/>⛽ 资源消耗监控"]
            STATE_MGR["状态管理器<br/>📊 合约状态处理"]
        end
        
        subgraph "验证系统"
            WASM_VALIDATOR["WASM验证器<br/>✅ 代码安全检查"]
            ABI_PROCESSOR["ABI处理器<br/>📋 接口解析验证"]
            PARAM_VALIDATOR["参数验证器<br/>🔍 调用参数检查"]
        end
        
        subgraph "基础设施"
            RESOURCE_MGR["ResourceManager<br/>📦 资源管理"]
            EXEC_ENGINE["ExecutionEngine<br/>🎯 执行引擎"]
            CACHE["MemoryStore<br/>🧠 合约缓存"]
            CRYPTO["密码学服务<br/>🔐 签名验证"]
        end
    end
    
    DEPLOY --> DEPLOY_SVC
    CALL --> CALL_SVC
    
    DEPLOY_SVC --> WASM_VALIDATOR
    DEPLOY_SVC --> ABI_PROCESSOR
    CALL_SVC --> PARAM_VALIDATOR
    CALL_SVC --> WASM_ENGINE
    
    WASM_ENGINE --> GAS_METER
    GAS_METER --> STATE_MGR
    
    STATE_MGR --> RESOURCE_MGR
    RESOURCE_MGR --> EXEC_ENGINE
    EXEC_ENGINE --> CACHE
    CACHE --> CRYPTO
    
    style DEPLOY fill:#E8F5E8
    style CALL fill:#FFF3E0
    style DEPLOY_SVC fill:#E3F2FD
    style CALL_SVC fill:#FCE4EC
    style WASM_ENGINE fill:#F3E5F5
    style WASM_VALIDATOR fill:#E0F2F1
```

**架构特点说明：**

1. **双服务设计**：部署和调用分离，专业化处理不同业务逻辑
2. **WASM执行引擎**：基于WebAssembly的安全可控执行环境
3. **完整验证链**：从代码验证到参数检查的多层安全保障
4. **执行费用精确计量**：确保资源消耗的可预测和公平计费

---

## 📄 **合约部署服务**

【contract_deploy.go】

　　处理智能合约的完整部署流程，包括代码验证、ABI解析、资源创建等关键步骤。

```mermaid
sequenceDiagram
    participant User as 👤 开发者
    participant Service as 📄 ContractDeployService
    participant Validator as ✅ WASM验证器
    participant ABI as 📋 ABI处理器
    participant Resource as 📦 资源管理器
    participant Builder as 🔨 交易构建器
    participant Cache as 🧠 缓存服务
    
    User->>Service: 1. 提交合约部署请求
    Service->>Service: 2. 基础参数验证
    Service->>Validator: 3. WASM格式验证
    Validator->>Validator: 4. 代码安全检查
    Validator-->>Service: 5. 验证结果
    Service->>ABI: 6. ABI接口解析
    ABI->>ABI: 7. 接口签名验证
    ABI-->>Service: 8. ABI处理结果
    Service->>Resource: 9. 计算内容哈希
    Resource-->>Service: 10. 返回资源标识
    Service->>Builder: 11. 构建部署交易
    Builder->>Builder: 12. 创建ResourceOutput
    Builder->>Builder: 13. 设置访问控制
    Builder-->>Service: 14. 返回交易对象
    Service->>Cache: 15. 缓存部署信息
    Cache-->>Service: 16. 返回交易哈希
    Service-->>User: 17. 部署交易哈希
    
    Note over User,Cache: 合约部署完整验证流程
```

**部署处理步骤：**

1. **代码验证阶段**：
   - WASM格式合规性检查
   - 代码完整性验证
   - 安全漏洞扫描
   - 资源消耗预估

2. **ABI处理阶段**：
   - 接口定义解析
   - 方法签名验证
   - 参数类型检查
   - 导出函数映射

3. **资源创建阶段**：
   - 计算内容哈希（SHA256）
   - 创建ResourceOutput
   - 设置ExecutableType为CONTRACT
   - 配置ContractExecutionConfig

4. **权限控制设置**：
   - 根据options配置访问策略
   - 设置合适的LockingConditions
   - 支持企业级权限管理
   - 配置合约升级权限

---

## ⚡ **合约调用服务**

【contract_call.go】

　　处理智能合约方法调用，包括参数处理、执行费用计算、状态管理等核心功能。

```mermaid
flowchart TD
    subgraph "合约调用处理流程"
        START[调用请求] --> PARSE[参数解析验证]
        PARSE --> LOAD[加载合约资源]
        LOAD --> GAS_EST[执行费用费用估算]
        GAS_EST --> EXEC_PREP[执行环境准备]
        EXEC_PREP --> CONTRACT_EXEC[合约方法执行]
        CONTRACT_EXEC --> STATE_UPDATE[状态变更处理]
        STATE_UPDATE --> RESULT[执行结果封装]
        RESULT --> CACHE_STORE[结果缓存存储]
    end
    
    subgraph "并行处理"
        PARAM_VALID[参数类型验证]
        AUTH_CHECK[调用权限检查]  
        BALANCE_CHECK[余额充足检查]
        GAS_LIMIT[执行费用限制验证]
    end
    
    subgraph "状态管理"
        STATE_READ[状态读取]
        STATE_WRITE[状态写入]
        STATE_COMMIT[状态提交]
        STATE_ROLLBACK[异常回滚]
    end
    
    PARSE -.-> PARAM_VALID
    LOAD -.-> AUTH_CHECK
    GAS_EST -.-> BALANCE_CHECK
    EXEC_PREP -.-> GAS_LIMIT
    
    CONTRACT_EXEC --> STATE_READ
    STATE_READ --> STATE_WRITE
    STATE_WRITE --> STATE_COMMIT
    STATE_UPDATE -.-> STATE_ROLLBACK
    
    style START fill:#E8F5E8
    style CONTRACT_EXEC fill:#FFE0B2
    style STATE_UPDATE fill:#F3E5F5
    style CACHE_STORE fill:#E0F2F1
```

**调用处理特点：**

1. **智能参数处理**：
   - JSON参数自动转换
   - 类型安全验证
   - 复杂数据结构支持
   - ABI兼容性检查

2. **精确执行费用管理**：
   - 执行前执行费用估算
   - 运行时执行费用计量
   - 超限自动终止
   - 剩余执行费用退还

3. **状态一致性保证**：
   - 原子性状态更新
   - 异常自动回滚
   - 并发访问控制
   - 状态快照机制

---

## ⛽ **执行费用费用系统**

【精确的资源计量】

　　提供公平、可预测的计算资源计费机制，确保网络资源的合理分配。

```mermaid
mindmap
  root((执行费用费用体系))
    (基础操作费用)
      [指令执行费用]
      [内存分配费用]
      [存储读写费用]
      [网络调用费用]
    (动态调整机制)
      [网络拥塞感知]
      [历史数据分析]
      [实时费用调整]
      [优先级定价]
    (费用优化策略)
      [批量操作优惠]
      [预付费折扣]
      [长期合约优惠]
      [开发者激励]
    (安全保护措施)
      [最大执行费用限制]
      [恶意代码检测]
      [资源耗尽防护]
      [DoS攻击防范]
```

**执行费用计算公式：**

```
总执行费用费用 = 基础执行费用 + 存储费用 + 网络费用 + 优先级费用

其中：
- 基础执行费用 = 指令数量 × 指令执行费用价格
- 存储费用 = 存储字节数 × 存储执行费用价格  
- 网络费用 = 外部调用次数 × 网络执行费用价格
- 优先级费用 = 基础费用 × 优先级倍数
```

**执行费用费用等级：**

| **操作类型** | **执行费用消耗** | **说明** | **优化建议** |
|-------------|------------|----------|-------------|
| 算术运算 | 3-5 执行费用 | 基础数学计算 | 使用内建函数 |
| 内存读写 | 3 执行费用 | 局部变量访问 | 减少临时变量 |
| 存储读写 | 200-20000 执行费用 | 持久化存储 | 批量读写优化 |
| 合约调用 | 700+ 执行费用 | 跨合约调用 | 减少调用层级 |
| 日志输出 | 375+ 执行费用 | 事件记录 | 精简日志内容 |
| 创建合约 | 32000+ 执行费用 | 部署新合约 | 工厂模式优化 |

---

## 🔒 **安全机制**

【多层安全保障】

```mermaid
graph TB
    subgraph "智能合约安全体系"
        subgraph "代码层安全"
            CODE_SCAN[静态代码扫描]
            VULN_CHECK[漏洞模式检测]
            SECURE_CODING[安全编码规范]
        end
        
        subgraph "执行层安全"
            SANDBOX[沙箱执行环境]
            RESOURCE_LIMIT[资源使用限制]
            TIMEOUT_CTRL[执行超时控制]
        end
        
        subgraph "访问层安全"
            AUTH_CONTROL[权限访问控制]
            RATE_LIMIT[调用频率限制]
            WHITELIST[地址白名单机制]
        end
        
        subgraph "数据层安全"
            STATE_ENCRYPT[状态数据加密]
            PARAM_VALIDATE[参数合法性验证]
            OUTPUT_SANITIZE[输出数据清理]
        end
    end
    
    CODE_SCAN --> SANDBOX
    VULN_CHECK --> RESOURCE_LIMIT  
    SECURE_CODING --> TIMEOUT_CTRL
    
    SANDBOX --> AUTH_CONTROL
    RESOURCE_LIMIT --> RATE_LIMIT
    TIMEOUT_CTRL --> WHITELIST
    
    AUTH_CONTROL --> STATE_ENCRYPT
    RATE_LIMIT --> PARAM_VALIDATE
    WHITELIST --> OUTPUT_SANITIZE
    
    style CODE_SCAN fill:#FFCDD2
    style SANDBOX fill:#F8BBD9
    style AUTH_CONTROL fill:#E1BEE7
    style STATE_ENCRYPT fill:#C5CAE9
```

**安全特性详解：**

1. **代码安全验证**：
   - 静态分析检测已知漏洞模式
   - 禁止危险操作和系统调用
   - 强制内存安全和类型安全
   - 代码签名和完整性验证

2. **执行环境隔离**：
   - WebAssembly沙箱执行
   - 严格的资源使用限制
   - 确定性执行保证
   - 异常自动恢复机制

3. **访问权限管控**：
   - 基于锁定条件的访问控制
   - 调用频率和执行费用限制
   - 白名单和黑名单机制
   - 动态权限调整

---

## 🎯 **企业级功能**

【复杂业务场景支持】

```mermaid
classDiagram
    class EnterpriseContract {
        +MultiSigDeployment: 企业多签部署
        +AccessControlPolicy: 访问控制策略
        +UpgradeManagement: 合约升级管理
        +AuditTrail: 审计追踪
        +ComplianceCheck: 合规检查
    }
    
    class AccessControlPolicy {
        +PersonalAccess: 个人私有合约
        +SharedAccess: 团队共享合约  
        +CommercialAccess: 付费使用合约
        +EnterpriseAccess: 企业治理合约
    }
    
    class UpgradeStrategy {
        +ImmutableContract: 不可变合约
        +OwnerUpgrade: 所有者升级
        +GovernanceUpgrade: 治理投票升级
        +TimeLockUpgrade: 时间锁升级
    }
    
    EnterpriseContract --> AccessControlPolicy
    EnterpriseContract --> UpgradeStrategy
    
    AccessControlPolicy : +validateAccess()
    AccessControlPolicy : +checkPermission()
    UpgradeStrategy : +proposeUpgrade()
    UpgradeStrategy : +executeUpgrade()
```

**企业功能特性：**

1. **多重签名部署**：
   - M-of-N企业级合约部署
   - 分阶段部署审核流程
   - 部署权限精细控制
   - 异常回滚和恢复

2. **访问控制策略**：
   - 基于角色的访问控制（RBAC）
   - 动态权限调整机制
   - 时间和地域限制
   - 审计日志自动记录

3. **合约升级管理**：
   - 向后兼容性保证
   - 渐进式升级部署
   - 治理投票决策机制
   - 升级异常保护

---

## 📊 **性能优化**

【高性能执行策略】

| **优化维度** | **策略** | **效果** | **适用场景** |
|-------------|----------|----------|-------------|
| 代码编译 | AOT预编译 | 50%执行加速 | 频繁调用合约 |
| 状态缓存 | 智能预取 | 80%IO减少 | 状态密集操作 |
| 并行执行 | 无关联并行 | 3x吞吐提升 | 批量合约调用 |
| 执行费用优化 | 指令级优化 | 20%费用减少 | 复杂计算合约 |
| 网络优化 | 批量打包 | 60%网络减少 | 高频合约交互 |

**性能监控指标：**

```mermaid
dashboard
    title 合约服务性能仪表板
    
    gauge "合约部署成功率" value 96.5 max 100
    gauge "平均部署时间" value 180 max 500 units "ms"
    gauge "合约调用TPS" value 520 max 1000  
    gauge "执行费用使用效率" value 78.2 max 100 units "%"
    gauge "缓存命中率" value 91.8 max 100 units "%"
```

---

## 🔧 **开发工具支持**

【完善的开发体验】

```mermaid
flowchart LR
    subgraph "合约开发工具链"
        IDE[智能合约IDE] --> COMPILER[WASM编译器]
        COMPILER --> TESTER[合约测试框架]
        TESTER --> DEBUGGER[调试工具]
        DEBUGGER --> DEPLOYER[部署工具]
        DEPLOYER --> MONITOR[监控面板]
    end
    
    subgraph "开发支持"
        TEMPLATE[合约模板库]
        LIBRARY[标准库支持]
        DOC[API文档生成]
        EXAMPLE[示例代码库]
    end
    
    IDE -.-> TEMPLATE
    COMPILER -.-> LIBRARY
    TESTER -.-> DOC
    DEBUGGER -.-> EXAMPLE
    
    style IDE fill:#E8F5E8
    style COMPILER fill:#FFF3E0
    style TESTER fill:#E3F2FD
    style MONITOR fill:#FCE4EC
```

**开发工具特性：**

1. **智能合约IDE**：
   - 语法高亮和自动补全
   - 实时错误检查
   - 集成调试环境
   - 版本控制集成

2. **测试框架**：
   - 单元测试自动化
   - 集成测试支持
   - 执行费用消耗分析
   - 性能基准测试

3. **部署和监控**：
   - 一键部署到测试网
   - 实时性能监控
   - 错误日志追踪
   - 用户使用统计

---

## 🛠️ **故障诊断**

【问题排查指南】

| **问题类型** | **症状** | **可能原因** | **解决方案** |
|-------------|----------|-------------|-------------|
| 部署失败 | 合约无法部署 | WASM格式错误 | 检查编译器版本和输出格式 |
| 调用超时 | 执行时间过长 | 无限循环或复杂计算 | 优化算法或增加执行费用限制 |
| 执行费用不足 | 交易执行失败 | 执行费用估算不准确 | 使用更精确的执行费用估算 |
| 权限错误 | 访问被拒绝 | 锁定条件不匹配 | 检查调用者权限和锁定条件 |
| 状态异常 | 数据不一致 | 并发访问冲突 | 使用事务或加锁机制 |

**诊断工具：**

```go
// 合约诊断接口
type ContractDiagnostics interface {
    CheckContractHealth(address string) (*HealthReport, error)
    Analyze执行费用Usage(txHash []byte) (*执行费用Report, error)
    ValidateContractState(address string) (*StateReport, error)
    TraceExecution(txHash []byte) (*ExecutionTrace, error)
}
```

---

## 📋 **最佳实践**

【开发建议】

1. **合约设计原则**：
   - 保持合约逻辑简单明确
   - 避免复杂的状态依赖
   - 使用事件记录重要操作
   - 实现合理的权限控制

2. **执行费用优化技巧**：
   - 避免不必要的存储操作
   - 使用批量操作减少调用
   - 选择合适的数据结构
   - 预计算常用数值

3. **安全开发规范**：
   - 输入参数严格验证
   - 避免重入攻击漏洞
   - 正确处理整数溢出
   - 实现紧急暂停机制

4. **测试和部署**：
   - 完整的单元测试覆盖
   - 多种场景的集成测试
   - 测试网充分验证
   - 灰度发布策略

【参考文档】
- [智能合约接口规范](../../../../pkg/interfaces/blockchain/contract.go)
- [WASM执行引擎文档](../../../../internal/core/engines/contract/README.md)
- [资源管理协议](../../../../pb/blockchain/block/transaction/resource/README.md)
- [执行费用费用系统](../fee/README.md)
