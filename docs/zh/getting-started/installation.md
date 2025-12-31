# 安装指南

---

## 概述

本指南介绍如何准备 WES 开发和运行环境。

---

## 系统要求

### 硬件要求

| 配置 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 4 核 | 8 核+ |
| 内存 | 8 GB | 16 GB+ |
| 磁盘 | 100 GB SSD | 500 GB+ SSD |
| 网络 | 10 Mbps | 100 Mbps+ |

### 软件要求

| 软件 | 版本要求 |
|------|----------|
| 操作系统 | Linux (Ubuntu 20.04+) / macOS 12+ / Windows 10+ |
| Go | 1.21+ |
| Docker | 20.10+ (可选) |
| Git | 2.30+ |

---

## 安装方式

### 方式 1：从源码编译

#### 1. 安装 Go

```bash
# macOS (使用 Homebrew)
brew install go

# Ubuntu
sudo apt update
sudo apt install golang-go

# 验证安装
go version
```

#### 2. 克隆仓库

```bash
git clone https://github.com/weisyn/weisyn.git
cd weisyn
```

#### 3. 编译

```bash
# 编译所有组件
make build

# 或只编译节点
go build -o wes-node ./cmd/node
```

#### 4. 验证安装

```bash
./wes-node --version
```

### 方式 2：使用 Docker

#### 1. 安装 Docker

```bash
# macOS
brew install --cask docker

# Ubuntu
sudo apt update
sudo apt install docker.io docker-compose
sudo systemctl enable docker
sudo systemctl start docker
```

#### 2. 拉取镜像

```bash
docker pull weisyn/wes-node:latest
```

#### 3. 验证安装

```bash
docker run --rm weisyn/wes-node:latest --version
```

### 方式 3：下载预编译二进制

#### 1. 下载

访问 [Releases 页面](https://github.com/weisyn/weisyn/releases) 下载对应平台的二进制文件。

#### 2. 解压并安装

```bash
# macOS / Linux
tar -xzf wes-node-linux-amd64.tar.gz
sudo mv wes-node /usr/local/bin/

# Windows
# 解压后添加到 PATH
```

#### 3. 验证安装

```bash
wes-node --version
```

---

## 配置文件

### 默认配置位置

| 平台 | 配置文件位置 |
|------|-------------|
| Linux | `~/.wes/config.yaml` |
| macOS | `~/Library/Application Support/WES/config.yaml` |
| Windows | `%APPDATA%\WES\config.yaml` |

### 创建配置文件

```bash
# 创建配置目录
mkdir -p ~/.wes

# 生成默认配置
wes-node config init
```

### 基本配置示例

```yaml
# ~/.wes/config.yaml
node:
  data_dir: ~/.wes/data
  log_level: info

network:
  listen_addr: "0.0.0.0:30303"
  bootstrap_nodes: []

consensus:
  enable_mining: false
```

---

## 验证安装

### 启动节点

```bash
# 启动节点
wes-node start

# 或使用自定义配置
wes-node start --config /path/to/config.yaml
```

### 检查状态

```bash
# 检查节点状态
wes-node status

# 检查网络连接
wes-node peers
```

---

## 常见问题

### Q: 编译时报错 "go: command not found"

A: 请确保 Go 已正确安装并添加到 PATH：
```bash
export PATH=$PATH:/usr/local/go/bin
```

### Q: Docker 镜像拉取失败

A: 检查网络连接，或尝试使用镜像加速：
```bash
docker pull registry.cn-hangzhou.aliyuncs.com/weisyn/wes-node:latest
```

### Q: 节点启动后无法连接网络

A: 检查防火墙设置，确保 30303 端口开放：
```bash
sudo ufw allow 30303
```

---

## 下一步

- [本地快速开始](./quickstart-local.md) - 启动本地单节点
- [Docker 快速开始](./quickstart-docker.md) - 使用 Docker 启动
- [第一笔交易](./first-transaction.md) - 发起第一笔交易

