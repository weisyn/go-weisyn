# 开发环境搭建

---

## 概述

本指南介绍如何搭建 WES 开发环境，准备参与项目开发。

---

## 环境要求

### 必需软件

| 软件 | 版本要求 | 用途 |
|------|----------|------|
| Go | 1.21+ | 主要开发语言 |
| Git | 2.30+ | 版本控制 |
| Make | 4.0+ | 构建工具 |

### 可选软件

| 软件 | 版本要求 | 用途 |
|------|----------|------|
| Docker | 20.10+ | 容器化测试 |
| golangci-lint | 1.54+ | 代码检查 |
| protoc | 3.19+ | Protocol Buffers |

---

## 安装步骤

### 1. 安装 Go

```bash
# macOS
brew install go

# Ubuntu
sudo apt update
sudo apt install golang-go

# 验证安装
go version
```

### 2. 配置 Go 环境

```bash
# 设置 GOPATH（如果需要）
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# 设置 Go 代理（国内用户）
go env -w GOPROXY=https://goproxy.cn,direct
```

### 3. 克隆仓库

```bash
# 克隆主仓库
git clone https://github.com/weisyn/weisyn.git
cd weisyn

# 或使用 SSH
git clone git@github.com:weisyn/weisyn.git
```

### 4. 安装依赖

```bash
# 下载 Go 依赖
go mod download

# 安装开发工具
make install-tools
```

### 5. 验证环境

```bash
# 运行测试
make test

# 构建项目
make build
```

---

## IDE 配置

### VS Code

推荐扩展：
- Go（官方扩展）
- GitLens
- Error Lens

配置文件 `.vscode/settings.json`：
```json
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.formatTool": "gofmt",
    "editor.formatOnSave": true
}
```

### GoLand

1. 打开项目目录
2. 确认 GOROOT 和 GOPATH 配置正确
3. 启用 File Watcher 进行自动格式化

---

## 开发工作流

### 1. 创建分支

```bash
# 从 main 创建特性分支
git checkout main
git pull origin main
git checkout -b feature/your-feature-name
```

### 2. 开发和测试

```bash
# 运行单元测试
make test

# 运行特定测试
go test ./internal/core/tx/...

# 运行代码检查
make lint
```

### 3. 提交代码

```bash
# 添加更改
git add .

# 提交（遵循提交规范）
git commit -m "feat: add new feature"

# 推送
git push origin feature/your-feature-name
```

### 4. 创建 Pull Request

在 GitHub 上创建 PR，填写：
- 标题：简洁描述更改
- 描述：详细说明更改内容和原因
- 关联 Issue（如果有）

---

## 常用命令

| 命令 | 说明 |
|------|------|
| `make build` | 构建项目 |
| `make test` | 运行测试 |
| `make lint` | 代码检查 |
| `make fmt` | 格式化代码 |
| `make clean` | 清理构建产物 |
| `make run` | 运行节点 |

---

## 项目结构

```
weisyn/
├── cmd/                    # 命令行入口
│   ├── node/              # 节点程序
│   └── cli/               # CLI 工具
├── internal/              # 内部包
│   ├── core/              # 核心模块
│   │   ├── ispc/          # ISPC 可验证计算
│   │   ├── eutxo/         # EUTXO 状态管理
│   │   ├── ures/          # URES 资源管理
│   │   ├── consensus/     # 共识机制
│   │   ├── tx/            # 交易处理
│   │   ├── block/         # 区块管理
│   │   ├── chain/         # 链管理
│   │   ├── network/       # 网络层
│   │   └── ...
│   └── ...
├── pkg/                   # 公共包
├── api/                   # API 定义
├── docs/                  # 文档
├── _dev/                  # 内部设计文档
└── ...
```

---

## 常见问题

### Q: go mod download 很慢

A: 使用 Go 代理：
```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

### Q: 测试失败

A: 检查：
1. Go 版本是否满足要求
2. 依赖是否完整
3. 是否有环境变量冲突

### Q: 如何调试

A: 使用 delve：
```bash
# 安装 delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试
dlv debug ./cmd/node
```

---

## 相关文档

- [代码规范](./code-style.md) - 编码标准
- [文档规范](./docs-style.md) - 文档编写标准
- [设计文档说明](./design-docs.md) - 如何阅读设计文档

