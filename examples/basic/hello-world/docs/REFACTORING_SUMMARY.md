# Hello World 合约重构完成报告

## 📋 重构概述

本次重构彻底改造了 HelloWorld 合约示例，围绕三个核心交互场景构建，消除了事件示例的混淆，建立了清晰的返回值通道语义。

## ✅ 完成的工作

### 1. 合约层重构（examples/basic/hello-world/src/hello_world.go）

#### ✨ 新函数设计

**Hello() - 最简单的字符串返回**
- 功能：返回 "Hello, WES!" 字符串
- 输入：无
- 返回：
  - `Results[0] = 0`（成功状态码）
  - `ReturnData = "Hello, WES!"`（业务数据）
  - `Events = []`（无事件）

**ChainStatus() - 链上状态查询**
- 功能：查询并返回链上核心信息
- 输入：无
- 调用宿主函数：`get_block_height`, `get_timestamp`, `get_caller`, `get_balance`
- 返回：
  - `Results[0] = 0`
  - `ReturnData = JSON`:
    ```json
    {
      "block_height": 12345,
      "timestamp": 1700000000,
      "caller": "0x1234...",
      "caller_balance": 1000000
    }
    ```

**Inspect() - 带参数的动态查询**
- 功能：根据 payload 中的 action 执行不同查询
- 输入（通过 Payload/initParams）：
  - `{"action":"block_height"}` - 返回区块高度
  - `{"action":"balance"}` - 返回调用者余额
  - `{"action":"balance","address":"0x..."}` - 返回指定地址余额
- 返回：
  - `Results[0] = 0` 或 `1`（ERROR_INVALID_PARAMS）
  - `ReturnData = JSON`（根据 action 不同）

#### 🗑️ 移除的内容
- ❌ SayHello（事件示例，混淆返回vs广播概念）
- ❌ GetGreeting（冗余，被 Hello 替代）
- ❌ SetMessage/GetMessage（状态存储示例，留给进阶教程）
- ❌ GetContractInfo（元数据查询，留给独立示例）

### 2. SDK 层增强（contracts/sdk/go/framework/）

**contract_base.go - ContractParams 扩展**
```go
// 新增易用方法
MustGetString(key string) string          // 获取必需字符串
GetStringOr(key, default string) string   // 获取字符串（带默认值）
ParseJSONInt(key string) uint64           // 解析整数字段
GetIntOr(key, default uint64) uint64      // 获取整数（带默认值）
IsEmpty() bool                            // 检查参数是否为空
```

**host_functions.go - JSON 序列化完善**
```go
// 支持完整的 map/struct 序列化
serializeToJSON(obj interface{}) string      // 递归序列化
serializeMapToJSON(m map[string]interface{}) // Map序列化
serializeArrayToJSON(arr []interface{})      // 数组序列化
escapeJSONString(s string) string            // 字符串转义
```

支持的类型：
- ✅ string, uint64, int64, int, uint32, int32
- ✅ bool, nil
- ✅ map[string]interface{}, map[string]string, map[string]uint64
- ✅ []interface{}, []string, []uint64

### 3. 宿主层改造（internal/core/）

**ExecutionContext 新增 InitParams 支持**
- `SetInitParams(params []byte) error` - 设置合约调用参数
- `GetInitParams() ([]byte, error)` - 获取合约调用参数

**StandardInterface 实现**
- `GetContractInitParams()` 从 ExecutionContext 读取（不再返回空）
- `get_balance` 宿主函数已验证可用（UTXO 占位逻辑对示例充分）

### 4. TX 层接口扩展（pkg/interfaces/tx/ & internal/core/tx/）

**CallContract 接口新增 initParams 参数**
```go
CallContract(
    ctx context.Context,
    signingKey []byte,
    contentHash []byte,
    method string,
    params []uint64,
    initParams []byte,  // ← 新增
) (result *ContractCallResult, err error)
```

**ISPC 层透传**
- `ExecuteWASMContract` 接收 initParams 并注入 ExecutionContext
- initParams 在执行前设置，供宿主函数 `get_contract_init_params` 读取

### 5. CLI 层改造（internal/cli/）

**合约调用界面新增 Payload 输入**
- 步骤 5/6：输入方法参数（数值型，可选）
- 步骤 6/6：输入合约负载（JSON/文本，可选）

**Payload 格式支持**
- inline JSON：`{"action":"balance"}`
- 文件引用：`@/path/to/payload.json`

**CallContractRequest 扩展**
```go
type CallContractRequest struct {
    // ... 原有字段
    Payload []byte  // ← 新增：合约调用参数
}
```

**结果展示已完善**
- ✅ 状态码（Results）
- ✅ 业务返回数据（ReturnData）- UTF-8 字符串或二进制
- ✅ 事件列表（Events）- 类型、时间戳、数据

### 6. 接口语义统一（pkg/interfaces/tx/contract.go）

**明确的通道约定**
- **Results（[]uint64）**：状态码/数值返回（0=成功，非0=错误码）
- **ReturnData（[]byte）**：业务返回数据（字符串/JSON/二进制），通过 `set_return_data` 设置
- **Events（[]EventData）**：日志/广播（非返回路径），通过 `emit_event` 发射

**参数通道约定**
- **params（[]uint64）**：WASM函数形参，强类型ABI场景
- **initParams（[]byte）**：JSON/二进制负载，由合约通过 `get_contract_init_params` 读取

## 🔧 验收步骤（需手动执行）

### 前置要求
1. 安装 TinyGo：`brew install tinygo`（macOS）或参考 https://tinygo.org/getting-started/install/
2. 确保 WES 节点已启动

### 步骤 1：编译合约
```bash
cd /Users/qinglong/go/src/chaincodes/WES/weisyn.git/examples/basic/hello-world
tinygo build -o hello_world.wasm -target wasi src/hello_world.go
```

### 步骤 2：部署合约
```bash
# 方式 1：使用 CLI
weisyn contract deploy --wallet <钱包名> --file hello_world.wasm --name "HelloWorld-v2"

# 方式 2：使用部署工具
go run /Users/qinglong/go/src/chaincodes/WES/weisyn.git/contracts/tools/deployer/main.go \
    --wallet <钱包名> \
    --file hello_world.wasm
```

获得合约 ID（contentHash，64位十六进制）

### 步骤 3：验证 Hello 函数
**调用**：
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Hello
```

**预期输出**：
```
✅ 调用成功！交易已提交到区块链网络

📋 执行结果：
  • 交易哈希: <txHash>
  • 状态码: [0]

📦 业务返回数据：
  Hello, WES!
```

### 步骤 4：验证 ChainStatus 函数
**调用**：
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method ChainStatus
```

**预期输出**：
```
✅ 调用成功！交易已提交到区块链网络

📋 执行结果：
  • 交易哈希: <txHash>
  • 状态码: [0]

📦 业务返回数据：
  {"block_height":12345,"timestamp":1700000000,"caller":"0x...","caller_balance":1000000}
```

### 步骤 5：验证 Inspect 函数（block_height）
**调用**：
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"block_height"}'
```

**预期输出**：
```
📋 执行结果：
  • 状态码: [0]

📦 业务返回数据：
  {"action":"block_height","result":12345}
```

### 步骤 6：验证 Inspect 函数（balance）
**调用**：
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"balance"}'
```

**预期输出**：
```
📋 执行结果：
  • 状态码: [0]

📦 业务返回数据：
  {"action":"balance","address":"0x...","balance":1000000}
```

### 步骤 7：验证错误处理
**调用**（无效 action）：
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"invalid"}'
```

**预期输出**：
```
📋 执行结果：
  • 状态码: [1]  ← ERROR_INVALID_PARAMS

📦 业务返回数据：
  {"error":"unsupported action","action":"invalid","supported":["block_height","balance"]}
```

## 📊 架构改进总结

### 通道清晰化
- ✅ Results = 状态码（0成功，非0错误）
- ✅ ReturnData = 业务数据（字符串/JSON）
- ✅ Events = 日志/广播（非返回）

### 参数通道对齐
- ✅ params（[]uint64）：函数形参
- ✅ initParams（[]byte）：JSON负载（init params通道已打通）

### 用户体验提升
- ✅ 初学者第一眼看到返回字符串（Hello）
- ✅ 能确认与链交互（ChainStatus）
- ✅ 能体验带参数查询（Inspect）
- ✅ CLI 完整展示 Results/ReturnData/Events

## 🎯 后续改进建议

1. **完善 Inspect 的地址解析**
   - 当前 `address` 参数解析为占位实现
   - 需实现真实的 WES 地址字符串→Address 转换

2. **增加更多查询场景**
   - `{"action":"tx_by_hash","hash":"..."}` - 需宿主函数 `get_tx_by_hash`
   - `{"action":"block_by_height","height":123}` - 需宿主函数 `get_block_by_height`

3. **添加状态存储示例**
   - 作为独立进阶教程
   - 演示 UTXO 状态管理模式

4. **补充单元测试**
   - 合约单元测试（TinyGo + 模拟宿主环境）
   - SDK JSON 序列化测试

## 📚 相关文档

- SDK 使用指南：`/contracts/sdk/README.md`
- WASM 引擎文档：`/internal/core/engines/wasm/README.md`
- 接口定义：`/pkg/interfaces/tx/contract.go`
- ISPC 规范：`/_docs/specs/ispc/INTRINSIC_SELF_PROVING_COMPUTING_SPECIFICATION.md`

