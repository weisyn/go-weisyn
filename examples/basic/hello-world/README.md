# 🌟 Hello World - WES 场景级应用示例

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-15
- **最后审核**：2025-11-15
- **所有者**：合约平台组
- **适用范围**：WES 场景级应用示例（端到端场景演示）

---

## 🎯 示例定位

**路径**：`examples/basic/hello-world/`

**目的**：这是一个**场景级应用示例**，展示如何构建一个完整的 WES 应用（合约 + 客户端 + 部署脚本），而不仅仅是单个合约资源。

**与资源级示例的区别**：
- **资源级示例**（`contracts/examples/`、`models/examples/`）：单个可执行资源（.wasm/.onnx）+ 测试用例
- **场景级示例**（本目录）：完整的应用场景，包含合约、客户端代码、部署脚本、交互脚本等

---

## 📋 简介

这是 WES 区块链的 Hello World 场景级示例，展示三个核心交互场景：
1. **Hello()** - 最简单的字符串返回
2. **ChainStatus()** - 查询链上状态（区块高度/时间戳/调用者/余额）
3. **Inspect()** - 带参数的动态查询（block_height/balance）

## 🚀 快速开始

### 1. 环境要求

- **Go 1.23**（主项目可用 Go 1.25，但合约编译需要 1.23）
- **TinyGo 0.34.0**（将 Go 编译为 WASM）
- **WES 节点**（运行区块链）

### 2. 编译合约

```bash
cd /Users/qinglong/go/src/chaincodes/WES/weisyn.git/examples/basic/hello-world

# 方式1：使用构建脚本（需要系统 PATH 中有 tinygo 和 go1.23）
bash scripts/build.sh

# 方式2：直接使用 TinyGo（如果不在 PATH 中，使用绝对路径）
/path/to/tinygo build -o build/hello_world.wasm -target wasm src/hello_world.go

# 方式3：使用编译工具
go run ../../contracts/tools/compiler/main.go -v src/hello_world.go
```

**重要**：本合约使用独立的 `go.mod`（Go 1.23）以兼容 TinyGo 0.34.0。

### 3. 部署合约

```bash
# 使用 CLI
weisyn contract deploy --wallet <钱包名> --file build/hello_world.wasm --name "HelloWorld-v2"

# 或使用部署脚本
bash scripts/deploy.sh
```

### 4. 调用合约

#### Hello() - 返回字符串
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Hello
```

**预期输出**：
```
📦 业务返回数据：
  Hello, WES!
```

#### ChainStatus() - 查询链状态
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method ChainStatus
```

**预期输出**：
```
📦 业务返回数据：
  {"block_height":12345,"timestamp":1700000000,"caller":"0x...","caller_balance":1000000}
```

#### Inspect() - 动态查询（block_height）
```bash
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"block_height"}'
```

**预期输出**：
```
📦 业务返回数据：
  {"action":"block_height","result":12345}
```

#### Inspect() - 动态查询（balance）
```bash
# 查询调用者余额
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"balance"}'

# 查询指定地址余额
weisyn contract call \
    --wallet <钱包名> \
    --contract <合约ID> \
    --method Inspect \
    --payload '{"action":"balance","address":"0x1234...abcd"}'
```

## 📁 项目结构

```
examples/basic/hello-world/
├── src/
│   └── hello_world.go          # 合约源码
├── build/
│   └── hello_world.wasm        # 编译产物
├── scripts/
│   ├── build.sh                # 构建脚本
│   ├── build_beginner.sh       # 新手友好版构建脚本
│   ├── deploy.sh               # 部署脚本
│   └── interact.sh             # 交互脚本
├── config/
│   ├── deploy_response.json    # 部署响应示例
│   └── deploy_tx_hash.txt      # 部署交易哈希
├── go.mod                      # 独立模块配置（Go 1.23）
├── README.md                   # 本文件
└── docs/                       # 详细文档目录
    ├── REFACTORING_SUMMARY.md  # 重构总结
    ├── HOST_FUNCTIONS_VERIFICATION.md  # 宿主函数验证报告
    └── TROUBLESHOOTING.md      # 故障排除指南
```

## 📖 核心概念

### 1. 返回值通道

WES 合约有三种返回通道：

- **Results ([]uint64)**：WASM 函数返回值，用于状态码（0=成功，非0=错误）
- **ReturnData ([]byte)**：业务返回数据，通过 `SetReturnString()`/`SetReturnJSON()` 设置
- **Events ([]EventData)**：事件日志，通过 `EmitEvent()` 发射（非返回通道）

### 2. 参数通道

- **params ([]uint64)**：WASM 函数形参，用于数值型参数
- **initParams ([]byte)**：JSON/文本负载，通过 `GetContractParams()` 读取（CLI 的 `--payload` 参数）

### 3. 宿主函数

合约可调用的链上功能（所有宿主函数均为真实实现，非占位）：

| 函数 | 功能 | 实现状态 |
|------|------|----------|
| `GetBlockHeight()` | 获取区块高度 | ✅ 真实（调用链服务） |
| `GetTimestamp()` | 获取时间戳 | ✅ 真实（区块时间或当前时间） |
| `GetCaller()` | 获取调用者地址 | ✅ 真实（从执行上下文） |
| `QueryBalance()` | 查询余额 | ⚠️ UTXO查询真实，金额提取占位（固定1000/UTXO） |
| `SetReturnString()` | 设置返回字符串 | ✅ 真实（设置到执行上下文） |
| `SetReturnJSON()` | 设置返回JSON | ✅ 真实（完整递归序列化） |
| `GetContractParams()` | 获取合约参数 | ✅ 真实（从执行上下文读取initParams） |

## 🔧 开发指南

### 编写新函数

1. 在 `src/hello_world.go` 中添加导出函数：
```go
//export MyFunction
func MyFunction() uint32 {
    // 业务逻辑
    result := "My Result"
    
    // 设置返回数据
    if err := framework.SetReturnString(result); err != nil {
        return framework.ERROR_EXECUTION_FAILED
    }
    
    return framework.SUCCESS
}
```

2. 重新编译：`bash scripts/build.sh`
3. 重新部署：`bash scripts/deploy.sh`

### 使用 JSON 参数

```go
//export ProcessData
func ProcessData() uint32 {
    // 获取 JSON 参数（来自 --payload）
    params := framework.GetContractParams()
    
    // 解析字段
    name := params.MustGetString("name")
    age := params.GetIntOr("age", 0)
    
    // 构建响应
    response := map[string]interface{}{
        "message": "Hello, " + name,
        "age": age,
    }
    
    framework.SetReturnJSON(response)
    return framework.SUCCESS
}
```

调用：
```bash
weisyn contract call \
    --contract <ID> \
    --method ProcessData \
    --payload '{"name":"Alice","age":30}'
```

### SDK 常用方法

**参数解析**：
- `GetContractParams()` - 获取参数对象
- `ParseJSON(key)` - 解析字符串字段
- `MustGetString(key)` - 获取必需字符串
- `GetStringOr(key, default)` - 获取字符串（带默认值）
- `ParseJSONInt(key)` - 解析整数字段
- `GetIntOr(key, default)` - 获取整数（带默认值）
- `IsEmpty()` - 检查参数是否为空

**返回数据**：
- `SetReturnString(str)` - 返回字符串
- `SetReturnJSON(obj)` - 返回 JSON（支持 map/struct/array）

**链查询**：
- `GetBlockHeight()` - 区块高度
- `GetTimestamp()` - 时间戳
- `GetCaller()` - 调用者地址
- `QueryBalance(addr, assetType)` - 查询余额

**地址工具**：
- `ParseAddressFromHex(hexStr)` - 解析十六进制地址字符串

## ⚠️ 常见问题

### 1. 编译错误：`requires go version 1.19 through 1.23, got go1.25`

**原因**：TinyGo 0.34.0 不支持 Go 1.25。

**解决**：
- 安装 Go 1.23：`go install golang.org/dl/go1.23.4@latest && ~/go/bin/go1.23.4 download`
- 或临时切换 Go 版本（如果已安装多版本）

### 2. CLI 看不到业务返回数据

**原因**：旧版本 CLI 或只显示 `Results` 字段。

**解决**：
- 确保使用最新 CLI（支持显示 `ReturnData`）
- 检查输出格式：应包含 `📦 业务返回数据：` 部分

### 3. `SetMessage` 调用失败（JSON 参数）

**原因**：旧版本不支持 `--payload` 参数。

**解决**：
- 使用 `--payload '{"key":"value"}'` 传递 JSON
- 或更新 CLI 到最新版本

### 4. 余额总是 0 或固定值

**原因**：`QueryBalance` 的 UTXO 金额提取是占位实现（固定 1000/UTXO）。

**说明**：这是已知的占位逻辑，对示例合约足够；生产环境需完善 UTXO 金额解析。

## 📚 延伸阅读

- **[REFACTORING_SUMMARY.md](docs/REFACTORING_SUMMARY.md)** - 合约重构总结，了解架构改进
- **[HOST_FUNCTIONS_VERIFICATION.md](docs/HOST_FUNCTIONS_VERIFICATION.md)** - 宿主函数验证报告，确认非占位实现
- **[TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)** - 故障排除指南，解决常见问题
- **[合约 SDK 文档](../../../contracts/sdk/README.md)** - 完整的 SDK API 参考
- **[WASM 引擎文档](../../../internal/core/engines/wasm/README.md)** - WASM 执行引擎原理

## 🎯 下一步

1. **修改合约**：尝试添加新函数或修改现有逻辑
2. **学习进阶示例**：查看 `contracts/token`、`contracts/nft` 等复杂合约
3. **理解 UTXO 模型**：阅读 WES 的状态管理机制
4. **参与开发**：为 WES 项目贡献代码或文档

---

*最后更新*: 2025-10-13  
*WES 版本*: v1.0.0  
*合约版本*: v2.0.0（重构版）
