# Execution模块文件迁移清单

## 📋 迁移分类汇总

### 🔴 类别1：越界实现 → 迁移到engines/wasm/
这些文件包含具体的引擎实现逻辑，违背了execution协调层的职责边界

| 当前位置 | 目标位置 | 文件大小 | 迁移原因 |
|----------|----------|----------|----------|
| `execution/abi_manager.go` | `engines/wasm/abi/manager.go` | 1038行 | ABI处理属于WASM引擎具体实现 |
| `execution/contract_manager.go` | `engines/wasm/contract/manager.go` | ~400行 | 合约管理属于引擎实现层 |
| `execution/contract_converters.go` | `engines/wasm/contract/converters.go` | ~200行 | 合约转换属于引擎实现层 |
| `execution/contract_utils.go` | `engines/wasm/contract/utils.go` | ~150行 | 合约工具属于引擎实现层 |

**迁移影响评估：**
- 总计迁移代码：~1788行
- 需要更新的import路径：约15-20个文件
- 依赖关系调整：engines/wasm/module.go需要注册这些组件

### 🟡 类别2：架构重组 → 在execution内重新组织
这些文件职责正确但目录结构不符合项目规范

| 当前位置 | 目标位置 | 文件大小 | 重组原因 |
|----------|----------|----------|----------|
| `execution/metrics_monitor.go` | `execution/monitoring/metrics.go` | 769行 | 按功能分组，符合项目规范 |
| `execution/audit_manager.go` | `execution/monitoring/audit_manager.go` | ~300行 | 审计管理归入监控组 |
| `execution/audit_query_service.go` | `execution/monitoring/audit_query.go` | ~200行 | 审计查询归入监控组 |
| `execution/audit_tracker.go` | `execution/monitoring/audit_tracker.go` | ~400行 | 审计追踪归入监控组 |
| `execution/security_integrator.go` | `execution/security/integrator.go` | ~150行 | 安全集成器独立成组 |
| `execution/quota_manager.go` | `execution/security/quota_manager.go` | ~200行 | 配额管理归入安全组 |
| `execution/side_effect_archiver.go` | `execution/effects/archiver.go` | ~300行 | 副作用归档器独立成组 |
| `execution/reliability_manager.go` | `execution/monitoring/reliability.go` | ~545行 | 可靠性管理归入监控组 |

**重组影响评估：**
- 总计重组代码：~2864行
- 需要创建的新目录：monitoring/, security/, effects/
- 内部import调整：约10-15个文件

### 🟢 类别3：module.go分离 → 分离到对应目录
module.go中的具体实现代码需要分离

| 当前位置 | 目标位置 | 代码行数 | 分离原因 |
|----------|----------|----------|----------|
| `ProductionMetricsCollector` | `monitoring/metrics_collector.go` | ~300行 | 具体实现不应在module.go中 |
| `ProductionAuditEmitter` | `monitoring/audit_emitter.go` | ~80行 | 具体实现不应在module.go中 |
| `ProductionSideEffectProcessor` | `effects/side_effect_processor.go` | ~100行 | 具体实现不应在module.go中 |

**分离影响评估：**
- module.go瘦身：从751行减少到~50行
- 新增实现文件：3个
- 构造函数调整：需要更新module.go中的Provider

### ✅ 类别4：保留不变 → 职责正确，位置合理
这些文件符合协调层职责，目录结构也合理

| 文件位置 | 保留原因 |
|----------|----------|
| `execution/host/registry.go` | 宿主能力注册表，协调层职责 |
| `execution/host/binding.go` | 宿主绑定实现，协调层职责 |
| `execution/host/provider_*.go` | 宿主能力提供者，协调层职责 |
| `execution/manager/engine_manager.go` | 引擎管理器，协调层职责 |
| `execution/manager/dispatcher.go` | 请求分发器，协调层职责 |
| `execution/resource_execution_coordinator.go` | 执行协调器，协调层核心 |
| `execution/resource_coordinator_impl.go` | 协调器实现，协调层核心 |
| `execution/env/advisor.go` | 环境顾问，协调层职责 |
| `execution/env/ml.go` | 机器学习环境，协调层职责 |
| `execution/migration/` | 迁移工具，协调层职责 |

## 📊 迁移统计总览

| 迁移类别 | 文件数量 | 代码行数 | 目标位置 |
|----------|----------|----------|----------|
| 🔴 越界实现 | 4个文件 | ~1788行 | engines/wasm/* |
| 🟡 架构重组 | 8个文件 | ~2864行 | execution子目录 |
| 🟢 module.go分离 | 3个实现 | ~480行 | execution子目录 |
| ✅ 保留不变 | 15+个文件 | ~2000行 | execution/保持 |

**总体效果：**
- execution/ 从~8000行减少到~3000行（协调层）
- engines/wasm/ 从~2000行增加到~4000行（实现层）
- 目录结构清晰，职责边界明确

## 🚀 迁移执行顺序

### 第一优先级：越界实现迁移
1. 创建 `engines/wasm/abi/` 目录
2. 创建 `engines/wasm/contract/` 目录  
3. 迁移 `abi_manager.go` → `engines/wasm/abi/manager.go`
4. 迁移 `contract_*.go` → `engines/wasm/contract/`
5. 更新 `engines/wasm/module.go` 注册新组件
6. 修复所有import路径

### 第二优先级：module.go瘦身
1. 创建 `execution/monitoring/` 目录
2. 创建 `execution/effects/` 目录
3. 分离 `ProductionMetricsCollector` 到 `monitoring/metrics_collector.go`
4. 分离 `ProductionAuditEmitter` 到 `monitoring/audit_emitter.go`  
5. 分离 `ProductionSideEffectProcessor` 到 `effects/side_effect_processor.go`
6. 重构 `module.go` 为标准装配文件

### 第三优先级：架构重组
1. 创建 `execution/security/` 目录
2. 重组监控相关文件到 `monitoring/`
3. 重组安全相关文件到 `security/`
4. 重组副作用相关文件到 `effects/`
5. 更新内部import路径

## ✅ 验收检查项

### 编译验证
- [ ] `go build ./internal/core/execution` 成功
- [ ] `go build ./internal/core/engines/wasm` 成功
- [ ] `go build ./...` 整体编译成功

### 架构验证  
- [ ] execution/ 不包含具体引擎实现
- [ ] engines/wasm/ 包含完整的WASM实现
- [ ] module.go 仅包含依赖注入装配
- [ ] 目录结构符合项目规范

### 功能验证
- [ ] 依赖注入正常工作
- [ ] 接口调用路径正确
- [ ] 不影响现有功能

这个迁移清单确保了架构重构的系统性和可追踪性。
