# Local Quick Start

---

## Overview

This guide explains how to quickly start a WES single-node environment locally, completing the experience in 5 minutes.

---

## Prerequisites

- Completed installation steps in [Installation Guide](./installation.md)
- Sufficient local disk space (at least 10 GB)

---

## Quick Start

### Step 1: Initialize Node

```bash
# Create data directory
mkdir -p ~/.wes/data

# Initialize configuration
wes-node init --datadir ~/.wes/data
```

### Step 2: Start Node

```bash
# Start single node (development mode)
wes-node start --dev

# Or start with mining enabled
wes-node start --dev --mine
```

### Step 3: Verify Running

```bash
# Check node status
wes-node status

# Expected output:
# Node Status: Running
# Height: 0
# Peers: 0
# Mining: true/false
```

---

## Development Mode Description

The `--dev` flag starts a pre-configured development environment:

- **Single-node mode**: No need to connect to other nodes
- **Fast block generation**: Block time set to 1 second
- **Preset accounts**: Automatically creates test accounts
- **Auto mining**: Optionally enable mining

---

## Using CLI Interaction

### View Blocks

```bash
# View latest block
wes-node block latest

# View block at specified height
wes-node block get --height 1
```

### View Accounts

```bash
# List accounts
wes-node account list

# View account balance
wes-node account balance --address <address>
```

### Send Transaction

```bash
# Send transfer transaction
wes-node tx send \
  --from <from_address> \
  --to <to_address> \
  --amount 100
```

---

## Using API Interaction

### Start API Service

API service starts by default at `http://localhost:8545`.

### Query Node Information

```bash
curl http://localhost:8545/api/v1/node/info
```

### Query Latest Block

```bash
curl http://localhost:8545/api/v1/block/latest
```

### Submit Transaction

```bash
curl -X POST http://localhost:8545/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "from": "<from_address>",
    "to": "<to_address>",
    "amount": "100"
  }'
```

---

## Stop Node

```bash
# Graceful stop
wes-node stop

# Or directly Ctrl+C
```

---

## Data Directory Structure

```
~/.wes/
├── config.yaml     # Configuration file
└── data/
    ├── blocks/     # Block data
    ├── state/      # State data
    ├── resources/  # Resource data
    └── logs/       # Log files
```

---

## FAQ

### Q: No new blocks after node startup

A: Ensure mining is enabled:
```bash
wes-node start --dev --mine
```

### Q: API cannot be accessed

A: Check if API is enabled:
```bash
wes-node start --dev --api
```

### Q: How to reset data

A: Delete data directory and reinitialize:
```bash
rm -rf ~/.wes/data
wes-node init --datadir ~/.wes/data
```

---

## Next Steps

- [Docker Quick Start](./quickstart-docker.md) - Start using Docker
- [First Transaction](./first-transaction.md) - Send your first transaction
- [Core Concepts](../concepts/) - Deep dive into WES

