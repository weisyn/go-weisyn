# Version Releases

---

## Overview

This document records WES version release history and plans.

---

## Version Matrix

| Version | Type | Release Date | Support Status | EOL Date |
|---------|------|--------------|----------------|----------|
| *To be released* | - | - | - | - |

---

## Release Channels

### Stable Releases

- GitHub Releases
- Docker Hub
- Package managers (planned)

### Preview Releases

- GitHub Releases (Pre-release)
- Docker Hub (dev tag)

---

## Release Frequency

| Type | Frequency |
|------|-----------|
| Major version | 1-2 times per year |
| Minor version | Once per month |
| Patch version | As needed |
| Security patches | Immediate |

---

## Release Process

### 1. Preparation Phase

- Code freeze
- Features completed
- Tests passed

### 2. Release Candidate

- Create RC version
- Community testing
- Collect feedback

### 3. Official Release

- Create Release
- Release announcement
- Update documentation

### 4. Post-Release

- Monitor issues
- Quick response
- Collect feedback

---

## Download

### Binary Files

Visit [GitHub Releases](https://github.com/weisyn/weisyn/releases) to download.

### Docker Images

```bash
docker pull weisyn/wes-node:latest
docker pull weisyn/wes-node:<version>
```

### Build from Source

```bash
git clone https://github.com/weisyn/weisyn.git
cd weisyn
make build
```

---

## Changelog

Detailed changelog for each version:
- [GitHub Releases](https://github.com/weisyn/weisyn/releases)
- `CHANGELOG.md` file

---

## Subscribe to Updates

### Release Notifications

- Watch GitHub repository
- Subscribe to mailing list
- Follow official blog

---

## Related Documentation

- [Compatibility Policy](./compatibility.md) - API compatibility
- [Support Policy](./support-policy.md) - Version support
- [Installation Guide](../getting-started/installation.md) - Installation instructions

