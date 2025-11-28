# WES 快速开始

---

## 🎯 快速开始指南

本指南帮助您在 5 分钟内快速上手 WES 区块链平台。

---

## 📋 开始前准备

### 系统要求

- **操作系统**: Linux/macOS/Windows
- **内存**: 至少 4GB RAM
- **存储**: 至少 10GB 可用空间
- **网络**: 稳定的互联网连接

---

## 🛠️ 安装 WES 节点

### 从源码编译（推荐开发者）

```bash
# 1. 克隆代码库
git clone https://github.com/weisyn/weisyn.git
cd weisyn

# 2. 编译节点
make build-dev

# 3. 验证安装
./bin/weisyn-development --version
```

### 使用 go run（开发环境推荐）

```bash
# 直接运行，无需编译
go run cmd/weisyn/main.go --version
```

---

## 🔧 初始化节点配置

```bash
# 1. 启动开发环境（会自动检测本地节点，未运行时显示引导菜单）
go run cmd/weisyn/main.go --env development

# 或使用编译后的二进制
./bin/weisyn-development --env development

# 2. 创建钱包
# 节点启动后会自动进入 CLI 界面，选择"账户管理" → "创建钱包"

# 3. 查看账户余额
# 在 CLI 界面中选择"账户管理" → "查询余额"
```

---

## 🌐 连接到网络

### 启动开发环境（推荐新手）

```bash
# 启动开发环境节点（默认模式：启动节点 + CLI界面）
go run cmd/weisyn/main.go --env development

# 或后台模式（只启动节点，不显示CLI）
go run cmd/weisyn/main.go --env development --daemon
```

### 启动测试环境

```bash
# 启动测试环境节点
go run cmd/weisyn/main.go --env testing

# 或后台模式
go run cmd/weisyn/main.go --env testing --daemon
```

### 启动生产环境

```bash
# 启动生产环境节点（后台模式推荐）
go run cmd/weisyn/main.go --env production --daemon
```

---

## 💰 获取测试代币

### 通过挖矿获取测试币

```bash
# 1. 启动节点（开发或测试环境）
go run cmd/weisyn/main.go --env development

# 2. 在 CLI 界面中：
#    - 选择"账户管理" → "创建钱包"（如果还没有）
#    - 选择"挖矿控制" → "开始挖矿"（输入你的地址）

# 3. 检查余额
#    在 CLI 界面中选择"账户管理" → "查询余额"
```

---

## 💸 发送第一笔交易

### 通过 CLI

```bash
# 启动节点（如果还未启动）
go run cmd/weisyn/main.go --env development

# 在 CLI 界面中：
#   选择"转账操作" → "单笔转账"
#   按照提示输入接收方地址、金额等信息
```

### 通过 JSON-RPC API

```bash
# 发送已签名交易（需要先在客户端签名）
curl -X POST http://localhost:8080/jsonrpc \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_sendRawTransaction",
    "params": ["0x..."],
    "id": 1
  }'
```

**注意**：WES 采用客户端签名模式，交易需要在客户端签名后再提交。完整的转账流程请参考 [CLI 参考](../../reference/cli/index.md)。

---

## 🎉 恭喜！你已经成功

- ✅ 安装了 WES 节点
- ✅ 连接到了 WES 网络
- ✅ 获取了测试代币
- ✅ 发送了第一笔交易

---

## 🎯 下一步做什么？

### 🔰 初学者路径

1. [用户指南](../README.md) - 深入了解所有功能
2. [CLI 快速开始](../cli-quickstart.md) - 管理你的数字资产
3. [常见问题](../../troubleshooting/faq.md) - 解决使用中的问题

### 👨‍💻 开发者路径

1. [智能合约快速开始](../contracts/beginner.md) - 编写第一个合约
2. [开发文档](../../README.md) - 开始开发应用
3. [应用示例](../../examples/README.md) - 学习最佳实践

### 🏗️ 运维路径

1. [环境配置](../deployment/) - 部署生产环境节点
2. [运维指南](../../troubleshooting/operations.md) - 设置系统监控
3. [配置优化](../../reference/config/) - 优化节点性能

---

## 🆘 遇到问题？

- 📖 查看 [完整故障排查](../../troubleshooting/faq.md)
- 💬 加入 [Discord 社区](https://discord.gg/weisyn)
- 🐛 在 [GitHub Issues](https://github.com/weisyn/weisyn/issues) 上提交问题

---

**欢迎来到 WES 生态系统！** 🌟

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [部署指南](../deployment/) - 了解生产环境部署

