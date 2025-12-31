# Docker 快速开始

---

## 概述

本指南介绍如何使用 Docker 快速启动 WES 节点，适合快速体验和测试。

---

## 前置条件

- 已安装 Docker（版本 20.10+）
- 已安装 Docker Compose（可选，用于多节点部署）

---

## 单节点启动

### 步骤 1：拉取镜像

```bash
docker pull weisyn/wes-node:latest
```

### 步骤 2：启动容器

```bash
# 启动开发模式节点
docker run -d \
  --name wes-node \
  -p 30303:30303 \
  -p 8545:8545 \
  -v ~/.wes-docker:/root/.wes \
  weisyn/wes-node:latest \
  start --dev --mine --api
```

### 步骤 3：验证运行

```bash
# 查看容器状态
docker ps

# 查看日志
docker logs -f wes-node

# 检查节点状态
docker exec wes-node wes-node status
```

---

## 使用 Docker Compose

### 创建 docker-compose.yaml

```yaml
version: '3.8'

services:
  wes-node:
    image: weisyn/wes-node:latest
    container_name: wes-node
    ports:
      - "30303:30303"
      - "8545:8545"
    volumes:
      - wes-data:/root/.wes
    command: start --dev --mine --api
    restart: unless-stopped

volumes:
  wes-data:
```

### 启动

```bash
docker-compose up -d
```

### 停止

```bash
docker-compose down
```

---

## 多节点集群

### docker-compose-cluster.yaml

```yaml
version: '3.8'

services:
  wes-node-1:
    image: weisyn/wes-node:latest
    container_name: wes-node-1
    ports:
      - "30303:30303"
      - "8545:8545"
    volumes:
      - wes-data-1:/root/.wes
    command: start --mine --api
    networks:
      - wes-network

  wes-node-2:
    image: weisyn/wes-node:latest
    container_name: wes-node-2
    ports:
      - "30304:30303"
      - "8546:8545"
    volumes:
      - wes-data-2:/root/.wes
    command: start --bootstrap wes-node-1:30303 --api
    networks:
      - wes-network
    depends_on:
      - wes-node-1

  wes-node-3:
    image: weisyn/wes-node:latest
    container_name: wes-node-3
    ports:
      - "30305:30303"
      - "8547:8545"
    volumes:
      - wes-data-3:/root/.wes
    command: start --bootstrap wes-node-1:30303 --api
    networks:
      - wes-network
    depends_on:
      - wes-node-1

networks:
  wes-network:
    driver: bridge

volumes:
  wes-data-1:
  wes-data-2:
  wes-data-3:
```

### 启动集群

```bash
docker-compose -f docker-compose-cluster.yaml up -d
```

---

## 常用命令

### 容器管理

```bash
# 查看运行中的容器
docker ps

# 停止容器
docker stop wes-node

# 启动容器
docker start wes-node

# 删除容器
docker rm wes-node
```

### 日志管理

```bash
# 查看实时日志
docker logs -f wes-node

# 查看最近 100 行日志
docker logs --tail 100 wes-node
```

### 进入容器

```bash
# 进入容器 shell
docker exec -it wes-node /bin/sh

# 执行单个命令
docker exec wes-node wes-node status
```

---

## 数据持久化

### 使用命名卷

```bash
docker run -d \
  --name wes-node \
  -v wes-data:/root/.wes \
  weisyn/wes-node:latest \
  start --dev
```

### 使用本地目录

```bash
docker run -d \
  --name wes-node \
  -v /path/to/local/data:/root/.wes \
  weisyn/wes-node:latest \
  start --dev
```

---

## 常见问题

### Q: 容器无法启动

A: 检查端口是否被占用：
```bash
lsof -i :30303
lsof -i :8545
```

### Q: 数据如何备份

A: 备份数据卷：
```bash
docker run --rm \
  -v wes-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/wes-backup.tar.gz /data
```

### Q: 如何升级镜像

A: 
```bash
docker pull weisyn/wes-node:latest
docker stop wes-node
docker rm wes-node
# 重新创建容器（使用相同的数据卷）
```

---

## 下一步

- [第一笔交易](./first-transaction.md) - 发起第一笔交易
- [部署教程](../tutorials/deployment/) - 更多部署选项
- [配置指南](../how-to/configure/) - 详细配置说明

