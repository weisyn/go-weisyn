# 代币转账应用问题排查指南

## 📖 概述

本文档提供代币转账应用常见问题的解决方案，帮助开发者快速定位和解决问题。

## 🚨 常见问题分类

### 1. 环境配置问题

#### 问题：Go版本不兼容

**症状**: 
```bash
go: cannot find main module, but found .git/config
```

**解决方案**:
```bash
# 检查Go版本
go version

# 更新到Go 1.19或更高版本
# macOS (使用Homebrew)
brew install go

# Ubuntu/Debian
sudo apt-get update
sudo apt-get install golang-go

# Windows
# 从 https://golang.org/dl/ 下载安装
```

#### 问题：项目路径错误

**症状**:
```bash
❌ 请在WES项目根目录下运行此脚本
```

**解决方案**:
```bash
# 找到正确的项目路径
find / -name "weisyn" -type d 2>/dev/null

# 进入项目根目录
cd /path/to/weisyn

# 确认项目结构
ls -la
# 应该看到 go.mod, contracts/, examples/ 等目录
```

#### 问题：权限不足

**症状**:
```bash
permission denied: ./scripts/setup.sh
```

**解决方案**:
```bash
# 给脚本添加执行权限
chmod +x scripts/*.sh

# 或者使用bash直接执行
bash scripts/setup.sh
```

### 2. 依赖问题

#### 问题：Go模块依赖错误

**症状**:
```go
cannot find package "github.com/weisyn/v1/pkg/types"
```

**解决方案**:
```bash
# 在项目根目录执行
go mod tidy

# 如果仍有问题，清理模块缓存
go clean -modcache
go mod download

# 检查go.mod文件
cat go.mod
```

#### 问题：TinyGo未安装

**症状**:
```bash
tinygo: command not found
```

**解决方案**:
```bash
# macOS (使用Homebrew)
brew tap tinygo-org/tools
brew install tinygo

# Ubuntu/Debian
wget https://github.com/tinygo-org/tinygo/releases/download/v0.28.1/tinygo_0.28.1_amd64.deb
sudo dpkg -i tinygo_0.28.1_amd64.deb

# Windows
# 从 https://tinygo.org/getting-started/install/ 下载安装

# 验证安装
tinygo version
```

### 3. 编译问题

#### 问题：WASM编译失败

**症状**:
```bash
wasm-ld: error: cannot open crt1.o
```

**解决方案**:
```bash
# 检查TinyGo目标支持
tinygo targets

# 使用正确的目标参数
tinygo build -target wasm -o contract.wasm main.go

# 如果仍有问题，尝试更新TinyGo
```

#### 问题：导入路径错误

**症状**:
```go
package contracts/sdk/go/framework is not in GOROOT
```

**解决方案**:
```go
// 错误的导入
import "contracts/sdk/go/framework"

// 正确的导入
import "github.com/weisyn/v1/contracts/sdk/go/framework"
```

### 4. 运行时问题

#### 问题：区块链节点未运行

**症状**:
```bash
⚠️ 区块链节点未运行
```

**解决方案**:
```bash
# 检查节点状态
curl http://localhost:28680/health

# 启动节点（在项目根目录）
./bin/node

# 或者使用配置文件启动
./bin/node -config configs/config.json

# 检查节点日志
tail -f data/logs/weisyn.log
```

#### 问题：端口被占用

**症状**:
```bash
listen tcp :28680: bind: address already in use
```

**解决方案**:
```bash
# 查找占用端口的进程
lsof -i :28680
# 或者
netstat -tulpn | grep 28680

# 终止占用进程
kill -9 <PID>

# 或者修改配置使用其他端口
```

#### 问题：余额查询失败

**症状**:
```bash
调用合约失败: connection refused
```

**解决方案**:
```bash
# 检查合约地址是否正确
cat deployed_contract.json

# 检查网络连接
ping localhost

# 验证合约是否部署成功
curl -X POST http://localhost:28680/contract/call \
  -H "Content-Type: application/json" \
  -d '{"address":"CONTRACT_ADDRESS","method":"GetContractInfo"}'
```

### 5. 交易问题

#### 问题：交易签名失败

**症状**:
```bash
签名交易失败: invalid private key format
```

**解决方案**:
```bash
# 检查私钥格式（应为64位十六进制）
echo $PRIVATE_KEY | wc -c  # 应输出65（64字符+换行）

# 重新生成钱包
rm -f wallets.json
./scripts/setup.sh  # 重新初始化
```

#### 问题：执行费用不足

**症状**:
```bash
交易执行失败: out of 执行费用
```

**解决方案**:
```go
// 在transaction_builder.go中增加执行费用限制
transaction.执行费用Limit = 2000000  // 增加到200万

// 或者优化合约代码减少执行费用消耗
```

#### 问题：余额不足

**症状**:
```bash
余额不足，当前余额: 0, 需要: 100
```

**解决方案**:
```bash
# 检查账户是否有初始代币
./scripts/check_balance.sh

# 如果是演示账户，运行初始化
./scripts/run_demo.sh

# 或者从其他账户转入代币
```

## 🔧 调试技巧

### 1. 启用详细日志

```bash
# 设置详细日志级别
export WES_LOG_LEVEL=debug

# 运行时查看日志
./scripts/run_demo.sh 2>&1 | tee debug.log
```

### 2. 使用调试工具

```bash
# 检查Go语法
go vet ./...

# 运行测试
go test ./...

# 检查代码格式
go fmt ./...
```

### 3. 手动测试API

```bash
# 测试余额查询
curl -X POST http://localhost:28680/contract/call \
  -H "Content-Type: application/json" \
  -d '{
    "address": "CONTRACT_ADDRESS",
    "method": "GetBalance",
    "params": {"address": "USER_ADDRESS"}
  }'

# 测试转账
curl -X POST http://localhost:28680/transaction/submit \
  -H "Content-Type: application/json" \
  -d '{
    "from": "SENDER_ADDRESS",
    "to": "CONTRACT_ADDRESS",
    "data": "{\"to\":\"RECEIVER_ADDRESS\",\"amount\":100}",
    "signature": "TRANSACTION_SIGNATURE"
  }'
```

## 🌍 跨平台问题

### Windows特有问题

```bash
# 路径分隔符问题
# 使用Git Bash或WSL替代CMD

# 脚本执行问题
bash scripts/setup.sh  # 而不是 ./scripts/setup.sh

# 权限问题
# 以管理员身份运行命令提示符
```

### macOS特有问题

```bash
# Homebrew权限问题
sudo chown -R $(whoami) /usr/local/Homebrew/

# Xcode命令行工具
xcode-select --install

# M1芯片兼容性
arch -x86_64 brew install tinygo
```

### Linux特有问题

```bash
# 缺少开发工具
sudo apt-get install build-essential

# 权限问题
sudo usermod -aG docker $USER  # 如果使用Docker

# 防火墙问题
sudo ufw allow 28680
```

## 📝 日志分析

### 常见错误模式

```bash
# 连接错误
"connection refused" -> 检查节点是否运行
"timeout" -> 检查网络和防火墙
"404 not found" -> 检查URL和路由

# 合约错误
"invalid method" -> 检查方法名是否正确
"invalid params" -> 检查参数格式和类型
"execution failed" -> 检查合约逻辑和执行费用

# 交易错误
"invalid signature" -> 检查私钥和签名算法
"nonce too low" -> 检查交易序号
"insufficient funds" -> 检查账户余额
```

### 日志级别说明

- **ERROR**: 严重错误，需要立即处理
- **WARN**: 警告信息，可能影响功能
- **INFO**: 一般信息，正常运行状态
- **DEBUG**: 调试信息，详细执行过程

## 🆘 获取帮助

### 自助诊断

```bash
# 运行诊断脚本
./scripts/diagnose.sh  # 如果存在

# 检查系统信息
uname -a
go version
node --version  # 如果使用
```

### 社区支持

- 📚 查看[WES文档](../../../docs/)
- 💬 加入开发者社区讨论
- 🐛 在GitHub上报告Bug
- 📧 联系技术支持团队

### 问题报告模板

```markdown
## 问题描述
简要描述遇到的问题

## 复现步骤
1. 步骤一
2. 步骤二
3. ...

## 期望结果
描述期望的正常行为

## 实际结果
描述实际发生的情况

## 环境信息
- 操作系统: 
- Go版本: 
- TinyGo版本: 
- WES版本: 

## 错误日志
```
粘贴相关错误日志
```

## 附加信息
其他可能有用的信息
```

---

🎯 通过本指南，您应该能够解决大部分常见问题。如果问题依然存在，请不要犹豫寻求社区支持！
