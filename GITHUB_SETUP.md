# GitHub 仓库设置指南

本文档说明如何完善 [weisyn](https://github.com/weisyn/weisyn) GitHub 仓库的设置。

## 📝 Repository Details（仓库详情）

### Description（描述）

```
WES 区块链核心 - 定义区块链的可验证计算范式，支持 AI 等复杂计算在链上可信运行
```

**英文版本**（如果支持英文描述）：
```
WES Blockchain Core - Defining verifiable computation paradigm for blockchain, enabling trusted on-chain execution of AI and complex computations
```

### Website（网站）

```
https://weisyn.com
```

或者文档网站：
```
https://github.com/weisyn/weisyn#readme
```

### Topics（标签）

建议添加以下标签（用空格分隔）：

```
blockchain wes blockchain-core consensus utxo ispc verifiable-computation ai-on-chain zk-proof zero-knowledge-proof smart-contract wasm tinygo go golang p2p network dapp infrastructure
```

**标签说明**：
- `blockchain` - 区块链相关
- `wes` - WES 区块链
- `blockchain-core` - 区块链核心
- `consensus` - 共识机制
- `utxo` - UTXO 模型
- `ispc` - ISPC（Intrinsic Self-Proving Computing）
- `verifiable-computation` - 可验证计算
- `ai-on-chain` - 链上 AI
- `zk-proof` / `zero-knowledge-proof` - 零知识证明
- `smart-contract` - 智能合约
- `wasm` - WebAssembly
- `tinygo` - TinyGo
- `go` / `golang` - Go 语言
- `p2p` - 点对点网络
- `network` - 网络
- `dapp` - 去中心化应用
- `infrastructure` - 基础设施

## 🔧 其他需要完善的内容

### 1. LICENSE 文件

**当前状态**：README 中提到了 MIT License，但仓库中可能缺少 LICENSE 文件。

**操作步骤**：
1. 在 GitHub 仓库页面，点击 "Add file" → "Create new file"
2. 文件名输入 `LICENSE`
3. 选择 "Choose a license template" → 选择 "MIT License"
4. 填写版权信息（如：Copyright 2025 Weisyn）
5. 提交文件

**或者**：从其他 MIT 项目复制 LICENSE 文件并修改版权信息。

### 2. .github 目录结构

建议创建以下文件以提升仓库专业性：

#### 2.1 Issue 模板

创建 `.github/ISSUE_TEMPLATE/` 目录：

**bug_report.md**：
```markdown
---
name: Bug Report
about: 报告 WES 核心的 bug
title: '[BUG] '
labels: bug
assignees: ''
---

## 描述
简要描述 bug

## 复现步骤
1. 
2. 
3. 

## 预期行为
描述预期行为

## 实际行为
描述实际行为

## 环境信息
- Go 版本：
- WES 版本：
- 操作系统：
- 架构：

## 日志/错误信息
```
粘贴错误日志
```

## 附加信息
其他相关信息
```

**feature_request.md**：
```markdown
---
name: Feature Request
about: 提出新功能建议
title: '[FEATURE] '
labels: enhancement
assignees: ''
---

## 功能描述
简要描述新功能

## 使用场景
描述使用场景

## 建议实现
描述建议的实现方式

## 附加信息
其他相关信息
```

#### 2.2 Pull Request 模板

创建 `.github/pull_request_template.md`：
```markdown
## 变更描述
简要描述本次 PR 的变更

## 变更类型
- [ ] Bug 修复
- [ ] 新功能
- [ ] 文档更新
- [ ] 代码重构
- [ ] 测试相关
- [ ] 性能优化
- [ ] 其他

## 测试
- [ ] 已添加单元测试
- [ ] 已添加集成测试
- [ ] 已通过所有测试
- [ ] 已测试相关功能

## 检查清单
- [ ] 代码遵循项目规范
- [ ] 已更新相关文档
- [ ] 已添加必要的注释
- [ ] 无编译错误和警告
- [ ] 已考虑向后兼容性
```

#### 2.3 GitHub Actions Workflows

创建 `.github/workflows/` 目录：

**ci.yml**（持续集成）：
```yaml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Run tests
        run: go test ./... -v
      
      - name: Build
        run: go build ./...
```

**lint.yml**（代码检查）：
```yaml
name: Lint

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  golangci-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
```

### 3. CONTRIBUTING.md

创建贡献指南文件（详见 CONTRIBUTING.md）

### 4. CODE_OF_CONDUCT.md

创建行为准则文件（详见 CODE_OF_CONDUCT.md）

### 5. SECURITY.md

创建安全策略文件（详见 SECURITY.md）

### 6. 仓库设置检查清单

#### 6.1 General Settings（常规设置）

- [ ] **Repository name**: `weisyn` ✅
- [ ] **Description**: 填写上述描述
- [ ] **Website**: 填写上述网站
- [ ] **Topics**: 添加上述标签

#### 6.2 Features（功能）

- [ ] **Issues**: 启用（用于 bug 报告和功能请求）
- [ ] **Projects**: 可选启用（用于项目管理）
- [ ] **Wiki**: 可选启用（如果使用 Wiki 文档）
- [ ] **Discussions**: 可选启用（用于社区讨论）
- [ ] **Sponsorships**: 可选启用

#### 6.3 Branch Protection（分支保护）

建议为 `main` 分支设置保护规则：

- [ ] **Require a pull request before merging**
  - [ ] Require approvals: 1
  - [ ] Dismiss stale pull request approvals when new commits are pushed
- [ ] **Require status checks to pass before merging**
  - [ ] Require branches to be up to date before merging
  - [ ] 选择 CI 检查（如：test, lint）
- [ ] **Require conversation resolution before merging**
- [ ] **Do not allow bypassing the above settings**

#### 6.4 Pages（页面）

如果有文档网站：

- [ ] 启用 GitHub Pages
- [ ] 选择文档源（如：`/docs` 目录）

#### 6.5 Actions（Actions）

- [ ] 确保 Actions 已启用
- [ ] 检查 Actions 权限设置

### 7. README 徽章

README 中已有一些徽章，可以添加更多：

```markdown
[![Go Version](https://img.shields.io/badge/go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/weisyn/weisyn)](https://goreportcard.com/report/github.com/weisyn/weisyn)
[![CI](https://github.com/weisyn/weisyn/workflows/CI/badge.svg)](https://github.com/weisyn/weisyn/actions)
```

### 8. 仓库描述优化建议

**简短版本**（适合 GitHub 搜索）：
```
WES Blockchain Core - Verifiable computation paradigm enabling AI and complex computations on-chain
```

**详细版本**（适合 About 部分）：
```
WES 是一个定义区块链可验证计算范式的区块链核心。突破确定性共识限制，支持 AI 等复杂计算在链上可信运行。采用 ISPC（Intrinsic Self-Proving Computing）架构，通过零知识证明实现可验证计算，让区块链真正承载真实业务。
```

## 📋 完整检查清单

### 必须完成

- [ ] Description（描述）
- [ ] Website（网站）
- [ ] Topics（标签）
- [ ] LICENSE 文件
- [ ] README.md（已有 ✅）

### 推荐完成

- [ ] .github/ISSUE_TEMPLATE/（Issue 模板）
- [ ] .github/pull_request_template.md（PR 模板）
- [ ] .github/workflows/（CI/CD 工作流）
- [ ] CONTRIBUTING.md（贡献指南）
- [ ] CODE_OF_CONDUCT.md（行为准则）
- [ ] SECURITY.md（安全策略）

### 可选完成

- [ ] .github/FUNDING.yml（资助信息）
- [ ] .github/dependabot.yml（依赖更新）
- [ ] .github/CODEOWNERS（代码所有者）
- [ ] GitHub Pages（文档网站）
- [ ] Releases（发布版本）

---



