# 🌟 Hello World - WES 入门示例

【示例定位】
　　这是WES区块链系统的第一个入门示例，通过一个简单的"Hello World"智能合约帮助初学者理解WES的基本概念、开发流程和部署过程。作为WES学习的起点，提供最基础但完整的区块链应用开发体验。

【学习目标】
- 理解WES智能合约的基本结构
- 掌握合约的编译和部署流程
- 学会与合约进行基本交互
- 熟悉WES开发工具的使用
- 建立区块链开发的基础概念

## 📁 文件结构

```
hello-world/
├── src/
│   └── hello_world.go          # 主合约代码
├── tests/
│   └── hello_world_test.go     # 测试代码
├── config/
│   └── deploy.yaml             # 部署配置
├── scripts/
│   ├── build.sh               # 构建脚本
│   ├── deploy.sh              # 部署脚本
│   ├── interact.sh            # 交互脚本
│   └── clean.sh               # 清理脚本
├── docs/
│   └── explanation.md         # 详细说明文档
└── README.md                  # 本文档
```

## 🚀 快速开始

### 前置要求
- 已安装WES节点和开发工具
- 已启动本地测试节点
- 基本的Go语言知识

### 运行示例
```bash
# 1. 进入示例目录
cd examples/hello-world

# 2. 构建合约
./scripts/build.sh

# 3. 部署合约
./scripts/deploy.sh

# 4. 与合约交互
./scripts/interact.sh
```

## 📝 合约代码解析

### 主合约文件 (src/hello_world.go)
```go
package main

import (
    "contracts/sdk/go/framework"
)

// HelloWorld 合约主函数
func SayHello() uint32 {
    // 获取调用者地址
    caller := framework.GetCaller()
    
    // 构造问候消息
    message := "Hello, WES World! Caller: " + caller
    
    // 发送事件
    framework.EmitEvent("HelloWorld", map[string]interface{}{
        "message": message,
        "caller": caller,
        "timestamp": framework.GetTimestamp(),
    })
    
    // 返回成功状态
    return framework.SUCCESS
}

// 获取问候消息
func GetGreeting() uint32 {
    greeting := "Welcome to WES Blockchain!"
    
    // 将结果存储到返回值
    framework.SetReturnValue(greeting)
    
    return framework.SUCCESS
}

// 设置自定义消息
func SetMessage() uint32 {
    params := framework.GetContractParams()
    customMessage := params.GetString("message")
    
    if customMessage == "" {
        return framework.ERROR_INVALID_PARAMS
    }
    
    // 存储自定义消息
    framework.SetState("custom_message", customMessage)
    
    // 发送事件
    framework.EmitEvent("MessageSet", map[string]interface{}{
        "message": customMessage,
        "setter": framework.GetCaller(),
    })
    
    return framework.SUCCESS
}

// 获取自定义消息
func GetMessage() uint32 {
    message := framework.GetState("custom_message")
    if message == "" {
        message = "No custom message set"
    }
    
    framework.SetReturnValue(message)
    return framework.SUCCESS
}
```

### 关键概念解释

1. **框架导入**：使用WES提供的Go SDK框架
2. **函数导出**：所有公共函数都导出为合约接口
3. **事件发送**：使用`EmitEvent`记录合约活动
4. **状态存储**：使用`SetState`和`GetState`管理合约状态
5. **返回值**：使用`SetReturnValue`返回查询结果

## 🔧 构建和部署

### 构建脚本 (scripts/build.sh)
```bash
#!/bin/bash

echo "🔨 构建 Hello World 合约..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 创建输出目录
mkdir -p build

# 使用 TinyGo 编译为 WASM
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

if [ $? -eq 0 ]; then
    echo "✅ 构建成功: build/hello_world.wasm"
    echo "📊 文件大小: $(wc -c < build/hello_world.wasm) bytes"
else
    echo "❌ 构建失败"
    exit 1
fi
```

### 部署脚本 (scripts/deploy.sh)
```bash
#!/bin/bash

echo "🚀 部署 Hello World 合约..."

# 检查 WASM 文件是否存在
if [ ! -f "build/hello_world.wasm" ]; then
    echo "❌ 找不到 WASM 文件，请先运行 build.sh"
    exit 1
fi

# 读取 WASM 文件并转换为 hex
WASM_HEX=$(hexdump -ve '1/1 "%.2x"' build/hello_world.wasm)

# 部署合约
curl -X POST http://localhost:8080/api/v1/contract/deploy \
    -H "Content-Type: application/json" \
    -d '{
        "wasm_code": "'$WASM_HEX'",
        "owner": "CHelloWorldOwner...",
        "owner_public_key": "02...",
        "init_params": "",
        "执行费用_limit": 1000000
    }' | jq .

echo "✅ 部署完成"
```

## 🎮 交互示例

### 基本交互脚本 (scripts/interact.sh)
```bash
#!/bin/bash

CONTRACT_ADDRESS="CHelloWorld123..."  # 替换为实际的合约地址

echo "🎮 与 Hello World 合约交互..."

# 1. 调用 SayHello 函数
echo "📞 调用 SayHello 函数..."
curl -X POST http://localhost:8080/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "SayHello",
        "params": {},
        "caller": "CUser123...",
        "执行费用_limit": 100000
    }' | jq .

echo ""

# 2. 查询 GetGreeting 函数
echo "🔍 查询 GetGreeting 函数..."
curl -X POST http://localhost:8080/api/v1/contract/query \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "GetGreeting",
        "params": {}
    }' | jq .

echo ""

# 3. 设置自定义消息
echo "📝 设置自定义消息..."
curl -X POST http://localhost:8080/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "SetMessage",
        "params": {
            "message": "Hello from WES Example!"
        },
        "caller": "CUser123...",
        "执行费用_limit": 100000
    }' | jq .

echo ""

# 4. 获取自定义消息
echo "📖 获取自定义消息..."
curl -X POST http://localhost:8080/api/v1/contract/query \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "GetMessage",
        "params": {}
    }' | jq .
```

## 📊 预期输出

### 成功的交互输出示例
```json
{
  "status": "success",
  "transaction_hash": "0x1234...",
  "events": [
    {
      "event_name": "HelloWorld",
      "data": {
        "message": "Hello, WES World! Caller: CUser123...",
        "caller": "CUser123...",
        "timestamp": 1703123456
      }
    }
  ]
}
```

## 🧪 测试

### 运行测试
```bash
# 运行单元测试
go test ./tests/... -v

# 输出示例
=== RUN   TestHelloWorld
    TestHelloWorld: hello_world_test.go:15: ✅ SayHello function works correctly
=== RUN   TestGreeting
    TestGreeting: hello_world_test.go:25: ✅ GetGreeting returns expected message
--- PASS: TestHelloWorld (0.00s)
--- PASS: TestGreeting (0.00s)
PASS
```

## 🎓 学习要点

### 通过这个示例你将学到：

1. **WES智能合约结构**
   - Go语言合约开发方式
   - 函数导出和接口定义
   - SDK框架的使用方法

2. **合约生命周期**
   - 编译：Go代码 → WASM字节码
   - 部署：上传合约到区块链
   - 交互：调用合约函数

3. **基础功能使用**
   - 事件发送和监听
   - 状态存储和查询
   - 参数传递和返回值

4. **开发工具链**
   - TinyGo编译器使用
   - WES API接口调用
   - 测试框架使用

## 🔗 下一步学习

完成这个示例后，建议继续学习：

1. **[简单代币示例](../simple-examples/token-transfer/)** - 学习代币转账
2. **[NFT示例](../simple-examples/nft-minting/)** - 了解NFT操作
3. **[数据存储示例](../simple-examples/data-storage/)** - 掌握数据管理

## ❓ 常见问题

**Q: 编译失败怎么办？**
A: 检查TinyGo是否正确安装，确保Go版本兼容。

**Q: 部署失败怎么办？**
A: 确认节点正在运行，检查API地址是否正确。

**Q: 无法调用合约函数？**
A: 检查合约地址是否正确，确认函数名拼写无误。

---

**示例难度**：🟢 简单  
**预计学习时间**：30分钟  
**前置知识**：基础Go语言编程
