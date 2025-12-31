# Port Specifications

---

## Overview

This document defines network port specifications used in the WES system.

---

## Default Ports

| Port | Protocol | Service | Description |
|------|----------|---------|-------------|
| 30303 | TCP/UDP | P2P | Inter-node communication |
| 8545 | HTTP | API | REST/JSON-RPC API |
| 8546 | WebSocket | API | WebSocket API |
| 6060 | HTTP | Metrics | Prometheus metrics |
| 9090 | HTTP | Admin | Admin interface |

---

## Port Descriptions

### P2P Port (30303)

**Purpose**: P2P communication between nodes

**Protocols**:
- TCP: Node discovery, block synchronization
- UDP: Node discovery (Kademlia)

**Configuration**:
```yaml
network:
  listen_addr: "0.0.0.0:30303"
```

**Firewall**:
```bash
# Both inbound and outbound need to be open
sudo ufw allow 30303/tcp
sudo ufw allow 30303/udp
```

### API Port (8545)

**Purpose**: Provide REST and JSON-RPC API externally

**Protocol**: HTTP

**Configuration**:
```yaml
api:
  http:
    enabled: true
    addr: "0.0.0.0:8545"
```

**Security Recommendations**:
- Production environment should restrict access sources
- Enable HTTPS
- Configure authentication

### WebSocket Port (8546)

**Purpose**: Real-time event subscription

**Protocol**: WebSocket

**Configuration**:
```yaml
api:
  ws:
    enabled: true
    addr: "0.0.0.0:8546"
```

### Metrics Port (6060)

**Purpose**: Prometheus metrics collection

**Protocol**: HTTP

**Configuration**:
```yaml
metrics:
  enabled: true
  addr: "0.0.0.0:6060"
```

### Admin Port (9090)

**Purpose**: Node management and diagnostics

**Protocol**: HTTP

**Configuration**:
```yaml
admin:
  enabled: true
  addr: "127.0.0.1:9090"
```

**Security Recommendations**:
- Bind to local address only
- Disable or strictly restrict in production environment

---

## Custom Ports

### Configuration Example

```yaml
network:
  listen_addr: "0.0.0.0:31303"  # Custom P2P port

api:
  http:
    addr: "0.0.0.0:18545"  # Custom API port
  ws:
    addr: "0.0.0.0:18546"  # Custom WebSocket port

metrics:
  addr: "0.0.0.0:16060"  # Custom metrics port

admin:
  addr: "127.0.0.1:19090"  # Custom admin port
```

---

## Multi-Node Deployment

When deploying multiple nodes on the same machine, different ports must be used:

| Node | P2P | API | WebSocket | Metrics |
|------|-----|-----|-----------|---------|
| Node 1 | 30303 | 8545 | 8546 | 6060 |
| Node 2 | 30304 | 8547 | 8548 | 6061 |
| Node 3 | 30305 | 8549 | 8550 | 6062 |

---

## Related Documentation

- [Configuration Reference](./config/) - Complete configuration instructions
- [Deployment Operations](../how-to/deploy/) - Deployment guide
- [Network and Topology](../concepts/network-and-topology.md) - Network architecture

