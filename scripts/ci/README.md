# CI/CD持续集成脚本（scripts/ci）

【模块定位】
　　本模块是WES系统中持续集成和持续部署的自动化脚本集合，负责提供代码质量检查、自动化测试和部署流程。通过标准化的CI/CD流水线和质量门禁机制，提供全自动的代码集成体验，支撑代码提交、测试验证和发布部署的自动化需求，确保代码质量、构建稳定性和发布可靠性。

【设计原则】
- **自动化优先**：实现从代码提交到部署发布的全流程自动化，减少人工干预和操作错误
- **质量门禁**：建立多层次的质量检查机制，确保只有高质量代码能够通过流水线
- **快速反馈**：提供快速的构建和测试反馈，帮助开发者及时发现和修复问题
- **环境一致性**：保证开发、测试、生产环境的一致性和可重复性
- **可观测性**：提供详细的构建日志、测试报告和部署状态追踪

【核心职责】
1. **静态代码检查**：执行代码风格、安全性和质量检查
2. **自动化测试**：运行单元测试、集成测试和端到端测试
3. **构建验证**：验证代码能够成功构建和打包
4. **安全扫描**：检查依赖漏洞和代码安全问题
5. **部署自动化**：自动部署到测试和生产环境
6. **监控报警**：监控CI/CD流水线状态和质量指标

【实现架构】

　　采用**流水线编排**的多阶段处理架构，实现从代码检查到部署发布的完整CI/CD流程。

```mermaid
graph TB
    subgraph "CI/CD流水线架构设计"
        subgraph "代码质量检查层"
            STATIC_CHECK["静态检查器<br/>🔍 static_checks.sh"]
            SECURITY_SCAN["安全扫描器<br/>🛡️ Security Scanner"]
            LINT_CHECK["代码规范检查<br/>📏 Linter"]
        end
        
        subgraph "测试执行层"
            UNIT_TEST["单元测试<br/>🧪 Unit Tests"]
            INTEGRATION_TEST["集成测试<br/>🔗 Integration Tests"]
            E2E_TEST["端到端测试<br/>🌍 run-e2e-tests.sh"]
        end
        
        subgraph "构建部署层"
            BUILD_VERIFY["构建验证<br/>🏗️ Build Verification"]
            ARTIFACT_GEN["产物生成<br/>📦 Artifact Generation"]
            DEPLOY_AUTO["自动部署<br/>🚀 Auto Deployment"]
        end
        
        subgraph "质量门禁层"
            QUALITY_GATE["质量门禁<br/>🚪 Quality Gate"]
            APPROVAL_GATE["审批门禁<br/>✅ Approval Gate"]
            RELEASE_GATE["发布门禁<br/>🎯 Release Gate"]
        end
    end
    
    STATIC_CHECK --> SECURITY_SCAN
    SECURITY_SCAN --> LINT_CHECK
    
    LINT_CHECK --> UNIT_TEST
    UNIT_TEST --> INTEGRATION_TEST
    INTEGRATION_TEST --> E2E_TEST
    
    E2E_TEST --> BUILD_VERIFY
    BUILD_VERIFY --> ARTIFACT_GEN
    ARTIFACT_GEN --> DEPLOY_AUTO
    
    DEPLOY_AUTO --> QUALITY_GATE
    QUALITY_GATE --> APPROVAL_GATE
    APPROVAL_GATE --> RELEASE_GATE
    
    style STATIC_CHECK fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    style E2E_TEST fill:#e8f5e8,stroke:#388e3c,stroke-width:2px
    style RELEASE_GATE fill:#fff3e0,stroke:#f57c00,stroke-width:2px
```

【核心业务流程】

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## 🔄 CI/CD流水线流程
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

```mermaid
sequenceDiagram
    participant Dev as 开发者
    participant Git as Git仓库
    participant CI as CI流水线
    participant Test as 测试环境
    participant Prod as 生产环境
    
    Dev->>Git: 1. 提交代码
    Git->>CI: 2. 触发CI流水线
    
    CI->>CI: 3. 静态代码检查
    CI->>CI: 4. 安全扫描
    CI->>CI: 5. 单元测试
    CI->>CI: 6. 集成测试
    
    CI->>Test: 7. 部署到测试环境
    CI->>CI: 8. 端到端测试
    
    alt 测试通过
        CI->>CI: 9. 构建生产镜像
        CI->>Prod: 10. 部署到生产环境
        CI->>Dev: 11. 通知部署成功
    else 测试失败
        CI->>Dev: 11. 通知构建失败
    end
```

【模块组织】

```
scripts/ci/
├── static_checks.sh         # 静态代码检查脚本
├── guards.sh               # CI守护检查脚本
├── run-e2e-tests.sh        # 端到端测试执行脚本
├── architecture-check.sh   # 架构守护检查脚本（从根目录移入）
├── verify_api.sh           # API网关验证脚本（从根目录移入）
└── README.md              # 本模块说明文档
```

**脚本功能说明**：

| 脚本文件 | 主要功能 | 执行频率 | 依赖条件 |
|----------|----------|----------|----------|
| `static_checks.sh` | 代码风格、语法检查 | 每次提交 | Go环境 |
| `guards.sh` | 构建保护、质量门禁 | 每次PR | Git hooks |
| `run-e2e-tests.sh` | 完整功能测试 | 每日/发布前 | 测试环境 |
| `architecture-check.sh` | 架构守护检查 | 每次提交 | Go环境 |
| `verify_api.sh` | API网关验证 | 每次部署 | 运行中的节点 |

【使用示例】

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## 🚀 本地CI检查
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

```bash
# 1. 运行静态检查
./scripts/ci/static_checks.sh

# 2. 运行守护检查  
./scripts/ci/guards.sh

# 3. 运行端到端测试
./scripts/ci/run-e2e-tests.sh

# 4. 完整CI流程
./scripts/ci/static_checks.sh && \
./scripts/ci/guards.sh && \
./scripts/ci/run-e2e-tests.sh
```

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## ⚙️ GitHub Actions集成
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

```yaml
# .github/workflows/ci.yml
name: WES CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  static-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.19
      - name: Run Static Checks
        run: ./scripts/ci/static_checks.sh

  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run Guards
        run: ./scripts/ci/guards.sh

  e2e-test:
    runs-on: ubuntu-latest
    needs: [static-check, security-scan]
    steps:
      - uses: actions/checkout@v3
      - name: Run E2E Tests
        run: ./scripts/ci/run-e2e-tests.sh
```

【质量门禁配置】

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## 📊 质量标准
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

| 质量指标 | 阈值要求 | 检查方式 | 失败处理 |
|----------|----------|----------|----------|
| **代码覆盖率** | ≥ 80% | 单元测试报告 | 阻止合并 |
| **静态检查** | 0 错误 | golangci-lint | 阻止构建 |
| **安全漏洞** | 0 高危 | gosec扫描 | 阻止部署 |
| **性能基准** | ≤ 10% 回退 | 基准测试 | 警告通知 |
| **依赖更新** | 30天内 | 依赖检查 | 建议更新 |

【CI/CD环境配置】

```bash
# CI环境变量配置
export CI=true
export GO_VERSION=1.19
export NODE_ENV=test
export DATABASE_URL=postgres://test:test@localhost:28790/weisyn_test
export REDIS_URL=redis://localhost:28791
```

【监控和告警】

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## 📈 CI/CD指标监控  
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

| 监控指标 | 正常范围 | 告警阈值 | 处理方式 |
|----------|----------|----------|----------|
| 构建成功率 | > 95% | < 90% | 邮件告警 |
| 构建时长 | < 10分钟 | > 15分钟 | Slack通知 |
| 测试通过率 | > 98% | < 95% | 阻止发布 |
| 部署频率 | 每日1次 | 连续3天无部署 | 流程检查 |

【故障排除】

## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
## 🔧 常见问题解决
## ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**1. 静态检查失败**
```bash
# 错误：golangci-lint报告问题
# 解决：修复代码风格问题
golangci-lint run --fix
```

**2. 测试环境不稳定**
```bash  
# 错误：E2E测试间歇性失败
# 解决：清理测试环境
./scripts/maintenance/clean-environment.sh
```

**3. 依赖下载慢**
```bash
# 错误：模块下载超时
# 解决：使用代理
export GOPROXY=https://goproxy.cn,direct
```

【开发指南】

1. **添加新检查项**：在相应脚本中添加检查逻辑和错误处理
2. **集成新工具**：更新脚本依赖和环境配置
3. **配置告警**：设置合适的阈值和通知方式
4. **优化性能**：并行执行检查项，减少CI时间

---

## 🔗 相关文档

- **构建脚本**：[`../build/README.md`](../build/README.md) - 项目构建和编译流程
- **测试框架**：[`../../test/README.md`](../../test/README.md) - 自动化测试体系
- **部署脚本**：[`../deploy/README.md`](../deploy/README.md) - 自动化部署流程
