# FAQ

---

## Overview

This document collects common questions and answers during WES getting started.

---

## Basic Questions

### Q: What is WES?

A: WES (Weisyn) is a third-generation blockchain that breaks through traditional blockchain's deterministic consensus limitations through ISPC (Intrinsic Self-Proving Computing) verifiable computing paradigm, supporting AI inference and enterprise applications running on-chain.

Learn more: [What is WES](../concepts/what-is-wes.md)

### Q: What's the difference between WES and Ethereum?

A: Main differences:

| Feature | WES | Ethereum |
|---------|-----|----------|
| Computing Model | ISPC (single execution + multi-point verification) | All nodes repeat execution |
| AI Support | Native support for on-chain AI inference | Not supported |
| State Model | EUTXO (three-layer output) | Account model |
| External Interaction | Supports verifiable external calls | Requires oracles |

### Q: What programming languages does WES support?

A: WES smart contracts support:
- Rust
- Go
- JavaScript/TypeScript
- Python (experimental)

Contracts are compiled to WASM format for on-chain execution.

---

## Installation Questions

### Q: What operating systems are supported?

A: WES supports:
- Linux (Ubuntu 20.04+, CentOS 8+)
- macOS 12+
- Windows 10+ (WSL2 recommended)

### Q: What hardware configuration is needed?

A: Minimum requirements:
- CPU: 4 cores
- Memory: 8 GB
- Disk: 100 GB SSD

Recommended configuration:
- CPU: 8+ cores
- Memory: 16+ GB
- Disk: 500+ GB SSD

Details: [Installation Guide](./installation.md)

### Q: What to do if build fails?

A: Common solutions:
1. Ensure Go version >= 1.21
2. Run `go mod tidy` to update dependencies
3. Check CGO environment configuration
4. View detailed error information `go build -v`

---

## Runtime Questions

### Q: Node cannot start

A: Checklist:
1. Check if port is occupied: `lsof -i :30303`
2. Check if data directory permissions are correct
3. Check if configuration file format is correct
4. View log files for detailed errors

### Q: Node cannot connect to network

A: Checklist:
1. Check if firewall allows port 30303
2. Check if bootstrap node addresses are correct
3. Check if network connection is normal
4. Check if behind NAT (try enabling UPnP)

### Q: Synchronization is slow

A: Optimization suggestions:
1. Use SSD storage
2. Increase network bandwidth
3. Use snapshot synchronization
4. Connect to more nodes

---

## Transaction Questions

### Q: Transaction not confirming

A: Possible reasons:
1. Fee too low
2. Node not mining (development mode)
3. Network congestion
4. Transaction format error

Solutions:
- Check transaction status
- Increase fee and resubmit
- Wait for network recovery

### Q: Insufficient balance error

A: Check:
1. If account balance is sufficient (including fees)
2. If there are unconfirmed transactions
3. If address is correct

### Q: How to query transaction status?

A: Use CLI or API:
```bash
# CLI
wes-node tx get --hash <tx_hash>

# API
curl http://localhost:8545/api/v1/tx/<tx_hash>
```

---

## Development Questions

### Q: How to develop smart contracts?

A: Steps:
1. Install contract SDK
2. Write contract code
3. Compile to WASM
4. Deploy to chain

Detailed tutorial: [Contract Development Tutorial](../tutorials/contracts/)

### Q: How to call API?

A: WES provides REST API and JSON-RPC:
```bash
# REST API
curl http://localhost:8545/api/v1/node/info

# JSON-RPC
curl -X POST http://localhost:8545/rpc \
  -H "Content-Type: application/json" \
  -d '{"method": "wes_nodeInfo", "params": []}'
```

Details: [API Reference](../reference/api/)

### Q: How to use SDK?

A: WES provides multi-language SDKs:
- Go SDK: `go get github.com/weisyn/client-sdk-go`
- JavaScript SDK: `npm install @weisyn/client-sdk-js`

See each SDK's README for usage examples.

---

## Other Questions

### Q: How to get test tokens?

A: 
- Development mode: First account automatically has test tokens
- Test network: Use faucet https://faucet.testnet.weisyn.io

### Q: How to participate in community?

A:
- GitHub: Submit Issues and PRs
- Discord: Join community discussions
- Documentation: Contribute documentation improvements

### Q: How to report bugs?

A:
1. Create Issue on GitHub
2. Provide detailed reproduction steps
3. Attach logs and environment information
4. Describe expected and actual behavior

---

## More Help

If the above doesn't solve your problem:

1. Check [Complete Documentation](../README.md)
2. Search GitHub Issues
3. Ask in community
4. Contact technical support

---

## Related Documentation

- [Installation Guide](./installation.md)
- [Local Quick Start](./quickstart-local.md)
- [First Transaction](./first-transaction.md)
- [Core Concepts](../concepts/)

