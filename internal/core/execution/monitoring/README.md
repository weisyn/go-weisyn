# 监控与审计（internal/core/execution/monitoring）

【模块定位（MVP）】
　　本模块为“辅助性”能力，服务于故障可观测与问题定位。在区块链节点“自运行、无人值守”的业务前提下，默认仅保留最小可行能力（日志 + 基础执行快照），其余重型能力（告警、可靠性管理、历史查询等）一律按需启用，不作为默认装配。

【默认行为（开箱即用）】
- 标准日志输出：关键路径信息、错误日志
- 基础执行快照（轻量内存统计）：
  - 总执行次数（TotalExecutions）
  - 成功率（SuccessRate）
  - 平均耗时（AverageLatency）
- 无任何后台 goroutine、无持久化、无外部系统依赖，常驻内存接近零

【可选扩展（按需启用）】
- 高级监控：历史聚合、趋势分析、持久化存储
- 告警通知：阈值检查、规则匹配、外部通知
- 审计查询：历史查询、报表生成、合规稽核
- 可靠性管理：熔断/限流/降级/健康检查

说明：这些扩展功能已从核心模块移除，需要时可在应用层独立实现。启用条件：存在明确消费方（外部告警/运营平台/合规稽核）且评估性价比后再开启。

【装配与配置】
- DI 装配：
  - 默认：协调器使用基础实现（BasicMetricsCollector、BasicAuditEmitter），无后台任务
  - 扩展：应用层可根据需要替换为高级实现，但不再作为core模块的一部分
- 配置极简：
  - 无配置文件依赖，开箱即用
  - 基础实现通过简单构造函数创建（NewBasicMetricsCollector、NewBasicAuditEmitter）
  - 扩展功能在应用层独立配置和管理

【接口归口原则】
- 跨子模块扩展的契约统一归口 `internal/core/execution/interfaces/monitoring.go`
- 仅当确需跨子模块扩展时才暴露接口；策略/插拔点在实现包内一律降级为非导出或具体类型，避免接口污染（已在本目录内完成收敛）

【与协调器的关系】
- 协调器默认注入基础监控/审计实现（BasicMetricsCollector、BasicAuditEmitter）
- 基础实现保证业务主路径零干扰：无后台任务、无阻塞操作、内存常驻近零
- 通过集中化接口（interfaces.MetricsCollector、interfaces.AuditEventEmitter）交互，保持高内聚、低耦合

【性能与资源】
- 基础模式（当前实现）：无后台任务、无持久化；仅原子计数器级内存占用
- 性能指标（基准测试验证）：
  - RecordExecutionStart: 7ns/op, 0B/op, 0 allocs/op
  - EmitSecurityEvent: 3ns/op, 0B/op, 0 allocs/op
  - 并发安全、线程无锁、适合高频调用
- 扩展模式：应用层独立实现，成本和复杂性自行评估

【使用建议】
- 自运行区块链节点：直接使用当前基础实现，零配置、零维护
- 企业级部署：在应用层添加外部监控系统（如Prometheus、ELK等）
- 合规稽核需求：在应用层实现审计日志收集和存储
- 高可用部署：在基础设施层实现负载均衡、熔断等策略

【文件导览（当前实现）】
- metrics_collector.go：BasicMetricsCollector 基础执行指标收集，原子计数器实现
- audit_emitter.go：BasicAuditEmitter 基础审计事件发射，标准日志输出
- monitoring_basic_test.go：完整的单元测试和性能基准测试
- README.md：本说明文档

【已移除文件（MVP简化）】
- reliability_manager.go、metrics_monitor.go、audit_query_service.go 等重型组件已删除
- 需要相关功能时，建议在应用层独立实现

【验收标准（已达成）】
1. ✅ 启动无后台任务、无持久化对象、无外部依赖
2. ✅ 性能验证：单次操作3-7纳秒，零内存分配，并发安全
3. ✅ 测试覆盖：完整单元测试 + 性能基准测试 + MVP原则验证

【变更记录（MVP极简化完成）】
- 2024.01: 完成MVP极简化改造，删除重型监控组件（metrics_monitor等）
- 策略型接口在实现包内降级为非导出；跨子模块契约归口 interfaces
- 协调器默认注入基础实现（BasicMetricsCollector、BasicAuditEmitter）
- 实现真正的"默认极简、零配置、零开销"目标

