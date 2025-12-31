# Compatibility Policy

---

## Overview

This document defines WES API and protocol compatibility policies.

---

## Version Number Standards

WES uses Semantic Versioning:

```
Major.Minor.Patch
MAJOR.MINOR.PATCH
```

| Version Number | Change Type | Compatibility |
|----------------|-------------|---------------|
| MAJOR | Major changes | May be incompatible |
| MINOR | New features | Backward compatible |
| PATCH | Bug fixes | Backward compatible |

---

## API Compatibility

### HTTP/REST API

- Maintain backward compatibility within same major version
- New fields don't affect existing fields
- Deprecated fields notified one minor version in advance
- Deprecated fields retained for at least two minor versions

### JSON-RPC API

- Method signatures remain stable within major version
- New parameters added as optional
- Return values only increase, never decrease

### WebSocket API

- Event formats remain stable within major version
- New event types don't affect existing subscriptions

---

## Protocol Compatibility

### P2P Protocol

- Protocol version negotiation mechanism
- Support multiple versions coexisting
- Old version nodes can participate in network

### Consensus Protocol

- Consensus rule changes through on-chain governance
- Soft fork: Old nodes can still verify
- Hard fork: Requires coordinated upgrade

---

## Data Format Compatibility

### Transaction Format

- Backward compatible: Old format transactions can be processed
- Forward compatible: New fields use default values

### Block Format

- Block version number identifies format
- Version upgrades have transition period

---

## Upgrade Strategy

### Software Upgrade

1. Release new version
2. Provide upgrade documentation
3. Users upgrade independently
4. Old versions supported for a period

### Protocol Upgrade

1. Proposal and discussion
2. Testnet validation
3. Mainnet activation (height or time)
4. Monitoring and rollback preparation

---

## Deprecation Strategy

### Deprecation Process

1. **Mark Deprecated**: Mark in documentation and code
2. **Warning Period**: At least one minor version
3. **Removal**: Remove in next major version

### Deprecation Notices

- List deprecated items in release notes
- Include warning headers in API responses
- Output warnings in logs

---

## Related Documentation

- [Support Policy](./support-policy.md) - Version support cycles
- [Version Releases](./releases.md) - Release history

