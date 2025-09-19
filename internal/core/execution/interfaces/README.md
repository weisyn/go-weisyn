# Execution内部接口层

## 📋 模块定位

　　本目录定义了`internal/core/execution`模块内部各子目录之间相互调用的接口契约。这些接口仅供execution内部使用，不对外暴露。

## 🎯 设计原则

1. **内部专用**：这些接口仅供execution内部子目录相互调用
2. **业务驱动**：每个接口都对应明确的业务需求
3. **职责单一**：每个接口专注于特定的功能领域
4. **松耦合**：通过接口隔离各子目录的具体实现

## 📊 接口架构

```
coordinator（协调器）
    ├── 依赖 → monitoring.MetricsCollector
    ├── 依赖 → monitoring.AuditEventEmitter
    ├── 依赖 → security.SecurityValidator
    ├── 依赖 → security.QuotaManager
    ├── 依赖 → effects.SideEffectProcessor
    └── 依赖 → env.MLAdvisor

其他子目录之间无直接依赖（星型架构）
```

## 📁 接口文件说明

| 文件 | 职责 | 主要接口 |
|------|------|----------|
| monitoring.go | 监控审计接口 | MetricsCollector, AuditEventEmitter, AuditTracker |
| security.go | 安全验证接口 | SecurityValidator, QuotaManager, ThreatDetector |
| effects.go | 副作用处理接口 | SideEffectProcessor, UTXOHandler, StateHandler |
| env.go | 环境顾问接口 | MLAdvisor, ResourceOptimizer, PerformanceAnalyzer |
| manager.go | 引擎管理接口 | EngineRegistry, Dispatcher |
| host.go | 宿主能力接口 | CapabilityProvider, HostBinding |

## ⚠️ 使用约束

1. **不对外暴露**：这些接口不应被execution以外的模块引用
2. **稳定性要求**：接口变更需要评估对所有内部子目录的影响
3. **向后兼容**：修改接口时需要保持向后兼容或提供迁移路径

## 🔄 与公共接口的关系

```
pkg/interfaces/execution/        # 公共接口（跨组件调用）
    ↓ 实现
internal/core/execution/coordinator/
    ↓ 依赖
internal/core/execution/interfaces/  # 内部接口（本目录）
    ↓ 实现
internal/core/execution/monitoring/
internal/core/execution/security/
internal/core/execution/effects/
... 等子目录
```

## ✅ 接口设计检查清单

- [ ] 接口是否有明确的业务需求？
- [ ] 接口方法是否职责单一？
- [ ] 接口参数是否使用了正确的类型（pkg/types）？
- [ ] 接口是否避免了不必要的耦合？
- [ ] 接口文档是否完整清晰？
