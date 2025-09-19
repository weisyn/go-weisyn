# WES Blockchain启动流程设计

## 📊 **完整启动流程图**

```mermaid
graph TD
    %% 启动入口
    START[["🚀 节点启动<br/>cmd/node/main.go"]] 
    
    %% 应用层启动
    APP_BOOTSTRAP[["📱 应用引导<br/>internal/app/bootstrap.go"]]
    CONFIG_LOAD[["⚙️ 配置加载<br/>internal/app/app.go"]]
    
    %% fx模块装配
    FX_MODULES[["🔧 FX模块装配<br/>internal/app/bootstrap.go"]]
    
    %% 基础设施层
    INFRA_LAYER[["🏗️ 基础设施层启动<br/>config, log, crypto, storage..."]]
    
    %% 数据层
    DATA_LAYER[["💾 数据存储层启动<br/>repositories, mempool..."]]
    
    %% 业务层
    BUSINESS_LAYER[["⚙️ 业务逻辑层启动<br/>blockchain, consensus..."]]
    
    %% 区块链模块启动
    BC_MODULE[["⛓️ 区块链模块启动<br/>internal/core/blockchain/module.go"]]
    
    %% 服务创建阶段
    CHAIN_SERVICE[["🔗 链状态服务<br/>chain.NewManager()"]]
    TX_SERVICE[["💸 交易服务<br/>transaction.NewManager()"]]
    BLOCK_SERVICE[["⛏️ 区块服务<br/>block.NewManager()"]]
    SYNC_SERVICE[["🔄 同步服务<br/>sync.NewManager()"]]
    
    %% 创世区块检查阶段
    GENESIS_CHECK[["🌱 创世区块检查<br/>fx.Invoke Genesis Check"]]
    
    %% 创世检查决策
    NEEDS_GENESIS{{"🤔 需要创世区块？<br/>NeedsGenesisBlock()"}}
    
    %% 创世区块处理分支
    LOAD_CONFIG[["📄 加载创世配置<br/>genesis.json + blockchain config"]]
    CREATE_TX[["💰 创建创世交易<br/>CreateGenesisTransactions()"]]
    CREATE_BLOCK[["📦 创建创世区块<br/>CreateGenesisBlock()"]]
    VALIDATE_GENESIS[["✅ 验证创世区块<br/>ValidateGenesisBlock()"]]
    PROCESS_GENESIS[["🔄 处理创世区块<br/>ProcessGenesisBlock()"]]
    INIT_STATE[["🗂️ 初始化链状态<br/>initializeGenesisState()"]]
    
    %% 正常启动分支
    SKIP_GENESIS[["⏭️ 跳过创世处理<br/>链已初始化"]]
    
    %% 服务就绪
    SERVICES_READY[["✅ 区块链服务就绪"]]
    API_LAYER[["🌐 API层启动<br/>HTTP, gRPC, WebSocket"]]
    
    %% 运行状态
    RUNNING[["🎉 系统运行中<br/>等待交易和区块"]]
    
    %% 错误处理
    ERROR_HANDLE[["❌ 错误处理<br/>启动失败回滚"]]

    %% 主流程
    START --> APP_BOOTSTRAP
    APP_BOOTSTRAP --> CONFIG_LOAD
    CONFIG_LOAD --> FX_MODULES
    FX_MODULES --> INFRA_LAYER
    INFRA_LAYER --> DATA_LAYER
    DATA_LAYER --> BUSINESS_LAYER
    BUSINESS_LAYER --> BC_MODULE
    
    %% 区块链模块内部启动
    BC_MODULE --> CHAIN_SERVICE
    BC_MODULE --> TX_SERVICE  
    BC_MODULE --> BLOCK_SERVICE
    BC_MODULE --> SYNC_SERVICE
    
    %% 所有服务就绪后检查创世
    CHAIN_SERVICE --> GENESIS_CHECK
    TX_SERVICE --> GENESIS_CHECK
    BLOCK_SERVICE --> GENESIS_CHECK 
    SYNC_SERVICE --> GENESIS_CHECK
    
    %% 创世检查决策分支
    GENESIS_CHECK --> NEEDS_GENESIS
    
    %% 需要创世区块的处理流程
    NEEDS_GENESIS -->|"是"| LOAD_CONFIG
    LOAD_CONFIG --> CREATE_TX
    CREATE_TX --> CREATE_BLOCK
    CREATE_BLOCK --> VALIDATE_GENESIS
    VALIDATE_GENESIS --> PROCESS_GENESIS
    PROCESS_GENESIS --> INIT_STATE
    INIT_STATE --> SERVICES_READY
    
    %% 不需要创世区块的处理流程
    NEEDS_GENESIS -->|"否"| SKIP_GENESIS
    SKIP_GENESIS --> SERVICES_READY
    
    %% 继续启动流程
    SERVICES_READY --> API_LAYER
    API_LAYER --> RUNNING
    
    %% 错误分支
    LOAD_CONFIG -.->|"错误"| ERROR_HANDLE
    CREATE_TX -.->|"错误"| ERROR_HANDLE
    CREATE_BLOCK -.->|"错误"| ERROR_HANDLE
    VALIDATE_GENESIS -.->|"错误"| ERROR_HANDLE
    PROCESS_GENESIS -.->|"错误"| ERROR_HANDLE
    
    %% 样式定义
    classDef startNode fill:#e1f5fe,stroke:#01579b,stroke-width:3px,color:#000
    classDef processNode fill:#f3e5f5,stroke:#4a148c,stroke-width:2px,color:#000
    classDef decisionNode fill:#fff3e0,stroke:#e65100,stroke-width:2px,color:#000
    classDef genesisNode fill:#e8f5e8,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef errorNode fill:#ffebee,stroke:#c62828,stroke-width:2px,color:#000
    classDef readyNode fill:#e0f2f1,stroke:#00695c,stroke-width:3px,color:#000
    
    class START,RUNNING startNode
    class APP_BOOTSTRAP,CONFIG_LOAD,FX_MODULES,INFRA_LAYER,DATA_LAYER,BUSINESS_LAYER,BC_MODULE processNode
    class CHAIN_SERVICE,TX_SERVICE,BLOCK_SERVICE,SYNC_SERVICE processNode
    class GENESIS_CHECK processNode
    class NEEDS_GENESIS decisionNode
    class LOAD_CONFIG,CREATE_TX,CREATE_BLOCK,VALIDATE_GENESIS,PROCESS_GENESIS,INIT_STATE genesisNode
    class SKIP_GENESIS,SERVICES_READY,API_LAYER readyNode
    class ERROR_HANDLE errorNode
```

## 🔍 **关键流程节点详解**

### **1. 节点启动阶段**
```
cmd/node/main.go → app.Start() → BootstrapApp()
```
- **入口单一**：所有启动逻辑统一在main.go中
- **选项模式**：支持配置文件路径等启动参数
- **错误处理**：启动失败立即退出

### **2. 模块装配阶段** 
```
fx.Module("blockchain") → 依赖注入 → 服务创建
```
- **分层装配**：基础设施 → 数据 → 业务 → 应用
- **依赖解析**：fx自动解析循环依赖
- **生命周期**：统一管理服务启停

### **3. 创世检查阶段**
```go
// 在所有blockchain服务就绪后触发
fx.Invoke(genesisInitializationCheck)
```
- **时机精确**：所有依赖服务都已创建完成
- **配置驱动**：基于genesis.json和blockchain配置
- **原子操作**：创世初始化要么全成功要么全失败

### **4. 创世区块处理流程**

#### **4.1 创世交易创建**
```go
// internal/core/blockchain/interfaces/transaction.go
CreateGenesisTransactions(ctx, genesisConfig) → []*Transaction
```
- **配置解析**：从genesis.json读取初始账户
- **交易构造**：创建初始代币分配交易
- **确定性**：相同配置产生相同交易

#### **4.2 创世区块构建**
```go
// internal/core/blockchain/interfaces/block.go
CreateGenesisBlock(ctx, genesisTransactions, genesisConfig) → *Block
```
- **区块头构造**：Height=0, PreviousHash=全零
- **Merkle根计算**：基于创世交易计算
- **时间戳设置**：使用配置中的时间戳

#### **4.3 创世区块处理**
```go
// internal/core/blockchain/chain/genesis.go
ProcessGenesisBlock(ctx, genesisBlock) → error
```
- **最终验证**：确保创世区块格式正确
- **数据存储**：调用repository.StoreBlock()
- **状态初始化**：设置ChainInitializedKey等状态

## 🎯 **设计优势分析**

### **1. 延迟初始化策略**
- **优势**：系统可以在没有创世区块的情况下启动所有服务
- **适用场景**：开发测试、多节点部署、灾难恢复
- **实现**：通过fx.Invoke在服务就绪后执行创世检查

### **2. 配置驱动的确定性**
- **优势**：相同配置在任何节点都产生相同创世区块
- **实现**：完全基于genesis.json构建创世状态
- **验证**：创世区块可以通过配置重现和验证

### **3. 统一的处理管道**
- **优势**：创世区块和普通区块使用相同的处理逻辑
- **实现**：通过isGenesisBlock标志区分特殊处理
- **维护**：减少代码重复，降低维护复杂度

### **4. 原子性状态管理**
- **优势**：创世初始化要么全部成功要么全部失败
- **实现**：所有状态变更在BadgerDB事务中完成
- **恢复**：支持失败后的自动重试和状态清理

## 🔧 **核心文件职责**

### **创世处理相关文件**
```
internal/core/blockchain/
├── chain/genesis.go                 # 创世区块管理核心
├── interfaces/transaction.go       # 创世交易接口
├── interfaces/block.go             # 创世区块接口  
├── module.go                       # 启动时创世检查
└── repositories/repository/chain.go # 创世状态持久化
```

### **启动流程相关文件**
```
cmd/node/main.go                    # 启动入口
internal/app/bootstrap.go           # 模块装配
internal/app/app.go                 # 配置管理
configs/genesis.json                # 创世配置
```

## 🚀 **启动命令示例**

### **标准启动**
```bash
# 使用默认配置启动
./bin/node

# 使用指定配置启动  
./bin/node --config configs/config.json
```

### **创世区块相关日志**
```
INFO  开始创世区块初始化检查...
INFO  检查是否需要创建创世区块
INFO  链状态未初始化，需要创建创世区块
INFO  开始创建创世区块...
INFO  ✅ 创世区块创建成功，交易数: 2
INFO  开始处理创世区块...
INFO  ✅ 创世区块处理完成
INFO  🎉 创世区块初始化完成
INFO  区块链核心模块已加载
```

这个设计实现了**"配置驱动、延迟初始化、原子处理"**的创世区块管理策略，既保证了系统启动的灵活性，又确保了创世状态的确定性和一致性。
