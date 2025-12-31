# Docker Quick Start

---

## Overview

This guide explains how to quickly start a WES node using Docker, suitable for quick experience and testing.

---

## Prerequisites

- Docker installed (version 20.10+)
- Docker Compose installed (optional, for multi-node deployment)

---

## Single Node Startup

### Step 1: Pull Image

```bash
docker pull weisyn/wes-node:latest
```

### Step 2: Start Container

```bash
# Start development mode node
docker run -d \
  --name wes-node \
  -p 30303:30303 \
  -p 8545:8545 \
  -v ~/.wes-docker:/root/.wes \
  weisyn/wes-node:latest \
  start --dev --mine --api
```

### Step 3: Verify Running

```bash
# View container status
docker ps

# View logs
docker logs -f wes-node

# Check node status
docker exec wes-node wes-node status
```

---

## Using Docker Compose

### Create docker-compose.yaml

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

### Start

```bash
docker-compose up -d
```

### Stop

```bash
docker-compose down
```

---

## Multi-Node Cluster

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

### Start Cluster

```bash
docker-compose -f docker-compose-cluster.yaml up -d
```

---

## Common Commands

### Container Management

```bash
# View running containers
docker ps

# Stop container
docker stop wes-node

# Start container
docker start wes-node

# Delete container
docker rm wes-node
```

### Log Management

```bash
# View real-time logs
docker logs -f wes-node

# View last 100 lines of logs
docker logs --tail 100 wes-node
```

### Enter Container

```bash
# Enter container shell
docker exec -it wes-node /bin/sh

# Execute single command
docker exec wes-node wes-node status
```

---

## Data Persistence

### Using Named Volumes

```bash
docker run -d \
  --name wes-node \
  -v wes-data:/root/.wes \
  weisyn/wes-node:latest \
  start --dev
```

### Using Local Directory

```bash
docker run -d \
  --name wes-node \
  -v /path/to/local/data:/root/.wes \
  weisyn/wes-node:latest \
  start --dev
```

---

## FAQ

### Q: Container cannot start

A: Check if ports are occupied:
```bash
lsof -i :30303
lsof -i :8545
```

### Q: How to backup data

A: Backup data volume:
```bash
docker run --rm \
  -v wes-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/wes-backup.tar.gz /data
```

### Q: How to upgrade image

A: 
```bash
docker pull weisyn/wes-node:latest
docker stop wes-node
docker rm wes-node
# Recreate container (using same data volume)
```

---

## Next Steps

- [First Transaction](./first-transaction.md) - Send your first transaction
- [Deployment Tutorial](../tutorials/deployment/) - More deployment options
- [Configuration Guide](../how-to/configure/) - Detailed configuration instructions

