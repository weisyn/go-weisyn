# 智能合约开发入门

---

## 🎯 入门概览

本指南帮助您快速开始 WES 智能合约开发，从零开始编写和部署您的第一个合约。

---

## 📋 开始前准备

### 开发环境

**必需工具**：
- Go 1.21+ 或 TinyGo
- WES 节点（本地或测试网）
- 代码编辑器（VS Code / GoLand）

**推荐工具**：
- WES SDK
- 合约开发工具链

---

## 🚀 第一个合约

### Step 1: 创建项目

```bash
# 创建项目目录
mkdir my-first-contract
cd my-first-contract

# 初始化 Go 模块
go mod init my-first-contract
```

### Step 2: 编写合约

创建 `contract.go`：

```go
package main

import (
    "github.com/weisyn/sdk/contract"
)

// HelloWorld 合约
type HelloWorld struct {
    contract.BaseContract
}

// SayHello 方法
func (c *HelloWorld) SayHello(name string) string {
    return "Hello, " + name + "!"
}

func main() {
    contract.Deploy(&HelloWorld{})
}
```

### Step 3: 编译合约

```bash
# 使用 TinyGo 编译为 WASM
tinygo build -o contract.wasm -target wasm contract.go
```

### Step 4: 部署合约

```bash
# 部署合约到测试网
wes dev contract deploy contract.wasm
```

### Step 5: 调用合约

```bash
# 调用合约方法
wes dev contract call <contract-hash> SayHello "World"
```

---

## 📚 学习路径

### 基础阶段

1. **理解合约概念**
   - 什么是智能合约
   - WES 合约的特点
   - 合约执行流程

2. **掌握开发工具**
   - 项目脚手架
   - 编译工具（WASM）
   - 部署工具

3. **编写简单合约**
   - Hello World 合约
   - EUTXO 状态管理（三层输出架构）
   - 事件发出

### 进阶阶段

1. **合约模式**
   - 代币合约模式
   - NFT 合约模式
   - DAO 合约模式

2. **最佳实践**
   - 安全编程
   - 资源消耗优化（内部资源计量）
   - 错误处理

3. **测试与调试**
   - 单元测试
   - 集成测试
   - 调试技巧

---

## 💡 推荐实践

### 安全实践

1. **输入验证**：始终验证用户输入
2. **重入攻击防护**：WES 通过 ISPC 单次执行+多点验证机制，天然避免传统区块链的重入问题
3. **权限控制**：实现适当的权限检查

### 性能优化

1. **CU（算力）优化**：减少不必要的计算，降低 CU（Compute Units，计算单位）消耗（注意：微迅链使用 CU 作为统一的算力计量单位，用户无需理解，但开发者应关注 CU 消耗以优化性能）
2. **状态优化**：合理设计 EUTXO 三层输出结构（AssetOutput/ResourceOutput/StateOutput）
3. **事件优化**：只发出必要的事件，大数据通过 URES 统一资源管理

---

## 📚 相关文档

- [合约模式](./patterns.md) - 推荐实践模式
- [故障排查](./troubleshooting.md) - 常见问题解决
- [API 参考](../../reference/api/) - API 接口文档

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [快速开始](../quickstart/) - 快速上手 WES

