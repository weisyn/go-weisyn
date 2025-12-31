# 故障排除快速指南

本文档专门帮助你快速解决Hello World合约开发中遇到的常见问题。

## 🏗️ 重要提示：独立Go模块

本合约使用**独立的 go.mod 文件**（Go 1.23），与主项目（Go 1.25）分离。这是有意的设计，确保：
- ✅ TinyGo兼容性
- ✅ 零外部依赖
- ✅ WASM文件最小化（141KB vs 1.6MB）

如果遇到Go版本相关问题，请检查是否在合约目录中操作，以及环境变量 `GOTOOLCHAIN=local` 是否设置。

💡 **详细了解**：参见 [MODULE_DESIGN.md](MODULE_DESIGN.md)

---

## 🎯 快速诊断流程图

```
遇到问题了？
    ↓
第一步：确认TinyGo是否安装？
    ├─ 未安装 → 跳转到"问题1"
    └─ 已安装 → 继续
    ↓
第二步：检查WASM文件大小？
    ├─ > 1MB → 跳转到"问题2"
    └─ < 200KB → 继续
    ↓
第三步：部署时报错？
    ├─ "未找到导出函数" → 跳转到"问题3"
    ├─ "connection refused" → 跳转到"问题4"
    └─ 其他错误 → 跳转到"问题5"
```

---

## 🔧 问题速查表

| 错误关键词 | 跳转 | 紧急程度 |
|----------|-----|---------|
| `tinygo: command not found` | [问题1](#问题1tinygo未安装) | 🔴 必须解决 |
| `WASM文件 > 1MB` | [问题2](#问题2wasm文件太大) | 🟡 影响部署 |
| `未找到业务导出函数` | [问题3](#问题3部署失败---未找到导出函数) | 🟡 无法部署 |
| `connection refused` | [问题4](#问题4节点连接失败) | 🟢 环境问题 |
| `function 'XXX' not found` | [问题6](#问题6合约调用失败---函数未找到) | 🟢 调用问题 |

---

## 问题0：Go版本冲突

### 🎯 错误信息
```
requires go version 1.19 through 1.23, got go1.24
或
go: ../../../go.mod requires go >= 1.24 (running go 1.23.x)
```

### 🔍 快速诊断

**情况1**：在合约目录编译，报错"got go1.24"
```bash
cd examples/basic/hello-world
tinygo build ...
# 错误：TinyGo 不支持 Go 1.24
```

**原因**：TinyGo找到了主项目的go.mod（Go 1.24要求）

**解决方案**：
```bash
# 确保合约目录有自己的go.mod
ls go.mod  # 应该存在

# 设置环境变量使用本地模块
export GOTOOLCHAIN=local

# 重新编译
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go
```

**情况2**：系统Go版本太新，TinyGo不支持

**解决方案**：
```bash
# 安装Go 1.23
cd /tmp
curl -L https://go.dev/dl/go1.23.5.darwin-arm64.tar.gz -o go1.23.tar.gz
tar -xzf go1.23.tar.gz
mv go ~/go1.23

# 临时使用
export PATH="$HOME/go1.23/bin:$PATH"
export GOTOOLCHAIN=local

# 验证
go version
# 应该显示：go version go1.23.5
```

### ✅ 预防措施

在 `~/.zshrc` 或 `~/.bashrc` 中添加：
```bash
# 智能合约开发环境
alias tinygo-env='export PATH="$HOME/go1.23/bin:$HOME/tinygo/bin:$PATH" && export GOTOOLCHAIN=local'
```

使用时：
```bash
tinygo-env
cd examples/basic/hello-world
./scripts/build.sh
```

---

## 问题1：TinyGo未安装

### 💊 5分钟快速修复

#### macOS用户
```bash
# 方法1：Homebrew（推荐）
brew install tinygo

# 方法2：手动安装
curl -L https://github.com/tinygo-org/tinygo/releases/download/v0.33.0/tinygo0.33.0.darwin-amd64.tar.gz -o tinygo.tar.gz
tar -xzf tinygo.tar.gz
sudo mv tinygo /usr/local/
echo 'export PATH="/usr/local/tinygo/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

#### Linux用户
```bash
# Ubuntu/Debian
wget https://github.com/tinygo-org/tinygo/releases/download/v0.33.0/tinygo_0.33.0_amd64.deb
sudo dpkg -i tinygo_0.33.0_amd64.deb

# 其他发行版
wget https://github.com/tinygo-org/tinygo/releases/download/v0.33.0/tinygo0.33.0.linux-amd64.tar.gz
tar -xzf tinygo0.33.0.linux-amd64.tar.gz
sudo mv tinygo /usr/local/
echo 'export PATH="/usr/local/tinygo/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Windows用户
```powershell
# Chocolatey
choco install tinygo

# 或手动下载安装：
# https://github.com/tinygo-org/tinygo/releases
```

### ✅ 验证安装
```bash
tinygo version
# 期望输出：tinygo version 0.30.0 或更高
```

---

## 问题2：WASM文件太大

### 🔍 快速诊断
```bash
ls -lh examples/basic/hello-world/build/hello_world.wasm
```

| 文件大小 | 编译器 | 状态 |
|---------|-------|------|
| 100-200 KB | TinyGo | ✅ 正确 |
| 1.5-2.0 MB | 标准Go | ❌ 错误 |

### 💊 立即修复
```bash
cd examples/basic/hello-world

# 1. 删除错误的文件
rm build/hello_world.wasm

# 2. 使用TinyGo重新编译
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

# 3. 验证文件大小
ls -lh build/hello_world.wasm
# ✅ 应该显示 100K-200K
```

### 📊 大小对比
```
标准Go编译：
├─ 文件大小：1.6 MB
├─ 启动时间：慢
├─ Gas消耗：高
└─ 部署状态：可能失败 ❌

TinyGo编译：
├─ 文件大小：150 KB
├─ 启动时间：快
├─ Gas消耗：低
└─ 部署状态：正常 ✅
```

---

## 问题3：部署失败 - 未找到导出函数

### 🎯 错误信息
```
ERROR ❌ 部署失败: 服务端解析WASM导出函数失败：
未找到业务导出函数(WASM文件可能未使用//export标记导出函数)
```

### 🔍 根本原因排查

#### 检查点1：编译器类型（90%的情况）
```bash
# 检查文件大小
ls -lh build/hello_world.wasm

# 如果 > 1MB：
#   原因：使用了标准Go编译器
#   解决：跳转到"问题2"
```

#### 检查点2：导出指令
```bash
# 检查源代码中的导出指令
grep "//export" src/hello_world.go

# 应该看到至少这些：
# //export SayHello
# //export GetGreeting
# //export SetMessage
# //export GetMessage
# //export GetContractInfo
# //export invoke
```

#### 检查点3：WASM文件有效性
```bash
# 如果有wasm-objdump工具
wasm-objdump -x build/hello_world.wasm | grep "export"

# 应该看到导出的函数列表
```

### 💊 标准修复流程
```bash
# 步骤1：使用TinyGo重新编译
cd examples/basic/hello-world
rm build/hello_world.wasm
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

# 步骤2：验证文件
ls -lh build/hello_world.wasm
# ✅ 100-200 KB

# 步骤3：重新部署
# 在WES CLI中执行部署操作
```

---

## 问题4：节点连接失败

### 🎯 错误信息
```
connection refused
或
dial tcp 127.0.0.1:28680: connect: connection refused
```

### 🔍 快速检查
```bash
# 检查节点是否运行
curl http://localhost:28680/health

# 如果连接失败，说明节点未启动
```

### 💊 启动节点
```bash
# 方法1：测试环境（推荐）
go run ./cmd/testing --cli-only

# 方法2：开发环境
go run ./cmd/development

# 方法3：生产环境
go run ./cmd/production
```

---

## 问题5：其他编译错误

### 错误类型1：Go版本过低
```
Error: Go 1.21 or higher is required
```

**解决方案**：
```bash
# 检查Go版本
go version

# 如果 < 1.21，升级Go
# macOS: brew upgrade go
# Linux: 从官网下载 https://golang.org/dl/
```

### 错误类型2：依赖包缺失
```
Error: cannot find package "github.com/weisyn/v1/..."
```

**解决方案**：
```bash
# 在项目根目录
go mod download
go mod tidy
```

### 错误类型3：权限问题
```
Permission denied
```

**解决方案**：
```bash
# 给脚本添加执行权限
chmod +x scripts/*.sh

# 或使用bash直接运行
bash scripts/build.sh
```

---

## 问题6：合约调用失败 - 函数未找到

### 🎯 错误信息
```
ERROR ❌ 合约调用失败
错误信息：执行函数失败: 函数 'SayHello' 未找到
```

### 🔍 原因分析
1. **拼写错误**：函数名大小写敏感
2. **未导出**：函数缺少 `//export` 指令
3. **旧WASM**：使用了旧版本的WASM文件

### 💊 解决方案

#### 步骤1：检查函数名拼写
```bash
# 正确的函数名（区分大小写）
SayHello      ✅
sayHello      ❌ 错误（小写s）
SAYHELLO      ❌ 错误（全大写）
say_hello     ❌ 错误（下划线）
```

#### 步骤2：验证导出函数列表
```bash
# 查看源代码中的导出函数
grep "//export" src/hello_world.go

# 可调用的函数：
# - SayHello
# - GetGreeting
# - SetMessage
# - GetMessage
# - GetContractInfo
```

#### 步骤3：重新编译和部署
```bash
# 如果修改了源代码，必须重新编译和部署
cd examples/basic/hello-world

# 1. 重新编译
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

# 2. 重新部署（在WES CLI中执行）
```

---

## 问题7：编译很慢

### 💊 优化建议

#### 使用增量编译
```bash
# TinyGo默认已经优化，但可以：
# 1. 不要每次都删除build目录
# 2. 使用编译缓存

# 清理不必要的文件
go clean -cache
```

#### 减少编译时间
```bash
# 开发时可以使用开发模式
tinygo build -o build/hello_world.wasm \
    -target wasi \
    -no-debug \
    src/hello_world.go

# 这会跳过一些调试信息生成
```

---

## 🆘 紧急救援联系

如果以上所有方法都无法解决问题：

1. **查看日志**：
   ```bash
   cat data/logs/weisyn.log
   ```

2. **完整的错误报告**：
   - TinyGo版本：`tinygo version`
   - Go版本：`go version`
   - 操作系统：`uname -a`（macOS/Linux）或系统信息（Windows）
   - 完整错误信息
   - 已尝试的解决方案

3. **提交Issue**：
   - GitHub：https://github.com/weisyn/weisyn/issues
   - 包含上述所有信息

---

## ✅ 预防性检查清单

在开始开发前，确保：

- [ ] ✅ TinyGo已安装（`tinygo version`）
- [ ] ✅ Go版本 >= 1.21（`go version`）
- [ ] ✅ 环境变量配置正确（PATH包含TinyGo）
- [ ] ✅ WES节点正在运行
- [ ] ✅ 脚本有执行权限（`chmod +x scripts/*.sh`）

在每次编译前，确保：

- [ ] ✅ 使用TinyGo编译器（不是标准Go）
- [ ] ✅ 所有业务函数都有 `//export` 指令
- [ ] ✅ `main()` 函数保持为空
- [ ] ✅ `invoke()` 函数已导出

在每次部署前，确保：

- [ ] ✅ WASM文件大小 < 300KB
- [ ] ✅ WES节点正在运行
- [ ] ✅ 钱包已创建且已解锁

---

## 📚 相关文档

- [BEGINNER_README.md](BEGINNER_README.md) - 完整的新手指南
- [WASM_FUNCTION_DESIGN.md](WASM_FUNCTION_DESIGN.md) - 技术文档
- [CONCEPTS.md](CONCEPTS.md) - 区块链基础概念

---

**更新日期**：2025-11-13  
**文档版本**：v1.0.0

