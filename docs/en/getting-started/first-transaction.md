# First Transaction

---

## Overview

This guide explains how to send your first transaction on WES, including creating accounts, querying balances, and sending transfers.

---

## Prerequisites

- WES node started (refer to [Local Quick Start](./quickstart-local.md) or [Docker Quick Start](./quickstart-docker.md))
- Node is in development mode or mining is enabled

---

## Create Account

### Using CLI to Create

```bash
# Create new account
wes-node account create

# Example output:
# Address: wes1abc123...
# Private Key: (saved to keystore)
# 
# ⚠️ Please keep your private key safe!
```

### Import Existing Private Key

```bash
# Import private key
wes-node account import --private-key <your_private_key>
```

### View Account List

```bash
# List all accounts
wes-node account list

# Example output:
# 0: wes1abc123... (default)
# 1: wes1def456...
```

---

## Get Test Tokens

### In Development Mode

In development mode, the first account automatically receives test tokens.

```bash
# View balance
wes-node account balance --address wes1abc123...

# Example output:
# Balance: 1000000 WES
```

### Using Faucet

If connected to test network, you can use faucet to get test tokens:

```bash
curl -X POST https://faucet.testnet.weisyn.io/api/claim \
  -H "Content-Type: application/json" \
  -d '{"address": "wes1abc123..."}'
```

---

## Send Transaction

### Using CLI to Send

```bash
# Send transfer transaction
wes-node tx send \
  --from wes1abc123... \
  --to wes1def456... \
  --amount 100

# Example output:
# Transaction submitted
# TxHash: 0x789...
# Status: Pending
```

### Using API to Send

```bash
curl -X POST http://localhost:8545/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "from": "wes1abc123...",
    "to": "wes1def456...",
    "amount": "100"
  }'

# Example response:
# {
#   "txHash": "0x789...",
#   "status": "pending"
# }
```

---

## Query Transaction

### Query Transaction Status

```bash
# Using CLI
wes-node tx get --hash 0x789...

# Using API
curl http://localhost:8545/api/v1/tx/0x789...
```

### Transaction Status Description

| Status | Description |
|--------|-------------|
| `pending` | Submitted, waiting for confirmation |
| `confirmed` | Included in block |
| `finalized` | Reached final confirmation |
| `failed` | Transaction failed |

---

## Query Balance Changes

### Sender Balance

```bash
wes-node account balance --address wes1abc123...

# Example output:
# Balance: 999900 WES (reduced by 100 + fees)
```

### Receiver Balance

```bash
wes-node account balance --address wes1def456...

# Example output:
# Balance: 100 WES
```

---

## Complete Example

### Step 1: Start Node

```bash
wes-node start --dev --mine --api
```

### Step 2: Create Two Accounts

```bash
# Create sender account
wes-node account create
# Address: wes1sender...

# Create receiver account
wes-node account create
# Address: wes1receiver...
```

### Step 3: Check Sender Balance

```bash
wes-node account balance --address wes1sender...
# Balance: 1000000 WES
```

### Step 4: Send Transaction

```bash
wes-node tx send \
  --from wes1sender... \
  --to wes1receiver... \
  --amount 100
# TxHash: 0x789...
```

### Step 5: Wait for Confirmation

```bash
# Query after waiting a few seconds
wes-node tx get --hash 0x789...
# Status: confirmed
```

### Step 6: Verify Balance

```bash
# Sender
wes-node account balance --address wes1sender...
# Balance: 999900 WES

# Receiver
wes-node account balance --address wes1receiver...
# Balance: 100 WES
```

---

## FAQ

### Q: Transaction stays in pending status

A: Check if mining is enabled:
```bash
wes-node start --dev --mine
```

### Q: Insufficient balance error

A: Ensure sender has sufficient balance (including fees):
```bash
wes-node account balance --address <from_address>
```

### Q: Transaction failed

A: View transaction details and error information:
```bash
wes-node tx get --hash <tx_hash> --verbose
```

---

## Next Steps

- [Core Concepts](../concepts/) - Deep dive into WES technical architecture
- [Contract Development Tutorial](../tutorials/contracts/) - Learn smart contract development
- [API Reference](../reference/api/) - Complete API documentation

