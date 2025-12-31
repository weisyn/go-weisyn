# Development Environment Setup

---

## Overview

This guide explains how to set up the WES development environment to participate in project development.

---

## Environment Requirements

### Required Software

| Software | Version Requirement | Purpose |
|----------|---------------------|---------|
| Go | 1.21+ | Main development language |
| Git | 2.30+ | Version control |
| Make | 4.0+ | Build tool |

### Optional Software

| Software | Version Requirement | Purpose |
|----------|---------------------|---------|
| Docker | 20.10+ | Containerized testing |
| golangci-lint | 1.54+ | Code checking |
| protoc | 3.19+ | Protocol Buffers |

---

## Installation Steps

### 1. Install Go

```bash
# macOS
brew install go

# Ubuntu
sudo apt update
sudo apt install golang-go

# Verify installation
go version
```

### 2. Configure Go Environment

```bash
# Set GOPATH (if needed)
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Set Go proxy (for users in China)
go env -w GOPROXY=https://goproxy.cn,direct
```

### 3. Clone Repository

```bash
# Clone main repository
git clone https://github.com/weisyn/weisyn.git
cd weisyn

# Or use SSH
git clone git@github.com:weisyn/weisyn.git
```

### 4. Install Dependencies

```bash
# Download Go dependencies
go mod download

# Install development tools
make install-tools
```

### 5. Verify Environment

```bash
# Run tests
make test

# Build project
make build
```

---

## IDE Configuration

### VS Code

Recommended extensions:
- Go (official extension)
- GitLens
- Error Lens

Configuration file `.vscode/settings.json`:
```json
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.formatTool": "gofmt",
    "editor.formatOnSave": true
}
```

### GoLand

1. Open project directory
2. Confirm GOROOT and GOPATH are configured correctly
3. Enable File Watcher for auto-formatting

---

## Development Workflow

### 1. Create Branch

```bash
# Create feature branch from main
git checkout main
git pull origin main
git checkout -b feature/your-feature-name
```

### 2. Develop and Test

```bash
# Run unit tests
make test

# Run specific tests
go test ./internal/core/tx/...

# Run code checks
make lint
```

### 3. Commit Code

```bash
# Add changes
git add .

# Commit (follow commit conventions)
git commit -m "feat: add new feature"

# Push
git push origin feature/your-feature-name
```

### 4. Create Pull Request

Create PR on GitHub, fill in:
- Title: Concise description of changes
- Description: Detailed explanation of changes and reasons
- Related Issue (if any)

---

## Common Commands

| Command | Description |
|---------|-------------|
| `make build` | Build project |
| `make test` | Run tests |
| `make lint` | Code checking |
| `make fmt` | Format code |
| `make clean` | Clean build artifacts |
| `make run` | Run node |

---

## Project Structure

```
weisyn/
├── cmd/                    # Command-line entry points
│   ├── node/              # Node program
│   └── cli/               # CLI tools
├── internal/              # Internal packages
│   ├── core/              # Core modules
│   │   ├── ispc/          # ISPC verifiable computing
│   │   ├── eutxo/         # EUTXO state management
│   │   ├── ures/          # URES resource management
│   │   ├── consensus/     # Consensus mechanism
│   │   ├── tx/            # Transaction processing
│   │   ├── block/         # Block management
│   │   ├── chain/         # Chain management
│   │   ├── network/       # Network layer
│   │   └── ...
│   └── ...
├── pkg/                   # Public packages
├── api/                   # API definitions
├── docs/                  # Documentation
├── _dev/                  # Internal design documents
└── ...
```

---

## FAQ

### Q: go mod download is slow

A: Use Go proxy:
```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

### Q: Tests fail

A: Check:
1. If Go version meets requirements
2. If dependencies are complete
3. If there are environment variable conflicts

### Q: How to debug

A: Use delve:
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/node
```

---

## Related Documentation

- [Code Standards](./code-style.md) - Coding standards
- [Documentation Standards](./docs-style.md) - Documentation writing standards
- [Design Document Guide](./design-docs.md) - How to read design documents

