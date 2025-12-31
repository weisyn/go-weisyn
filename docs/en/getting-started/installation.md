# Installation Guide

---

## Overview

This guide explains how to prepare the WES development and runtime environment.

---

## System Requirements

### Hardware Requirements

| Configuration | Minimum | Recommended |
|---------------|---------|-------------|
| CPU | 4 cores | 8+ cores |
| Memory | 8 GB | 16+ GB |
| Disk | 100 GB SSD | 500+ GB SSD |
| Network | 10 Mbps | 100+ Mbps |

### Software Requirements

| Software | Version Requirement |
|----------|---------------------|
| Operating System | Linux (Ubuntu 20.04+) / macOS 12+ / Windows 10+ |
| Go | 1.21+ |
| Docker | 20.10+ (Optional) |
| Git | 2.30+ |

---

## Installation Methods

### Method 1: Build from Source

#### 1. Install Go

```bash
# macOS (using Homebrew)
brew install go

# Ubuntu
sudo apt update
sudo apt install golang-go

# Verify installation
go version
```

#### 2. Clone Repository

```bash
git clone https://github.com/weisyn/weisyn.git
cd weisyn
```

#### 3. Build

```bash
# Build all components
make build

# Or build node only
go build -o wes-node ./cmd/node
```

#### 4. Verify Installation

```bash
./wes-node --version
```

### Method 2: Using Docker

#### 1. Install Docker

```bash
# macOS
brew install --cask docker

# Ubuntu
sudo apt update
sudo apt install docker.io docker-compose
sudo systemctl enable docker
sudo systemctl start docker
```

#### 2. Pull Image

```bash
docker pull weisyn/wes-node:latest
```

#### 3. Verify Installation

```bash
docker run --rm weisyn/wes-node:latest --version
```

### Method 3: Download Pre-compiled Binary

#### 1. Download

Visit the [Releases page](https://github.com/weisyn/weisyn/releases) to download the binary for your platform.

#### 2. Extract and Install

```bash
# macOS / Linux
tar -xzf wes-node-linux-amd64.tar.gz
sudo mv wes-node /usr/local/bin/

# Windows
# Extract and add to PATH
```

#### 3. Verify Installation

```bash
wes-node --version
```

---

## Configuration Files

### Default Configuration Location

| Platform | Configuration File Location |
|----------|----------------------------|
| Linux | `~/.wes/config.yaml` |
| macOS | `~/Library/Application Support/WES/config.yaml` |
| Windows | `%APPDATA%\WES\config.yaml` |

### Create Configuration File

```bash
# Create configuration directory
mkdir -p ~/.wes

# Generate default configuration
wes-node config init
```

### Basic Configuration Example

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

## Verify Installation

### Start Node

```bash
# Start node
wes-node start

# Or use custom configuration
wes-node start --config /path/to/config.yaml
```

### Check Status

```bash
# Check node status
wes-node status

# Check network connections
wes-node peers
```

---

## FAQ

### Q: Build error "go: command not found"

A: Ensure Go is correctly installed and added to PATH:
```bash
export PATH=$PATH:/usr/local/go/bin
```

### Q: Docker image pull failed

A: Check network connection, or try using mirror acceleration:
```bash
docker pull registry.cn-hangzhou.aliyuncs.com/weisyn/wes-node:latest
```

### Q: Node cannot connect to network after startup

A: Check firewall settings, ensure port 30303 is open:
```bash
sudo ufw allow 30303
```

---

## Next Steps

- [Local Quick Start](./quickstart-local.md) - Start local single node
- [Docker Quick Start](./quickstart-docker.md) - Start using Docker
- [First Transaction](./first-transaction.md) - Send your first transaction

