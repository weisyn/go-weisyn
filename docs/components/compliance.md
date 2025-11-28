# Compliance 组件能力视图

---

## 🎯 组件定位

Compliance 组件是 WES 系统的监管合规核心，负责在交易处理和区块生产的关键节点实施合规策略检查。

**在三层模型中的位置**：协调层（Coordination Layer）

> **战略背景**：Compliance 组件通过多信息源融合的决策引擎，在不影响现有业务接口的前提下，为系统提供透明、高效的监管合规能力。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 合规策略引擎

**能力描述**：
- 基于配置规则的智能决策判定
- 多信息源融合决策（身份凭证、GeoIP、P2P特征）
- 决策结果缓存（提升性能和一致性）
- 配置驱动的策略规则执行

**决策优先级排序**：
```
身份凭证 > 配置规则 > GeoIP查询 > P2P特征 > 默认策略
```

**使用约束**：
- 策略配置支持热重载
- 决策结果有缓存机制
- 支持优雅降级（合规服务异常时）

### 2. 多信息源融合

**能力描述**：
- **身份凭证优先**：基于数字签名的可信身份属地证明
- **GeoIP辅助判定**：支持MaxMind、IPInfo等主流GeoIP数据库
- **P2P特征分析**：基于连接模式的地理特征推断
- **配置规则覆盖**：支持手动配置的地址白名单/黑名单

**使用约束**：
- 信息源查询有超时限制
- 查询结果有缓存机制
- 支持信息源降级

### 3. 精准操作控制

**能力描述**：
- **操作类型识别**：普通转账、合约调用、治理操作、支付相关
- **地理区域限制**：国家级封禁、操作级授权、未知地区处理
- **三阶段合规控制**：边界网关拦截 + 节点内策略准入 + 打包阶段排除

**使用约束**：
- 操作类型识别基于交易输出类型和合约地址
- 地理限制基于ISO-3166-1 alpha-2国家代码
- 合规检查在哈希计算前进行

---

## 🔧 接口能力

### Policy（合规策略接口）

**能力**：
- `CheckTransaction(ctx, tx)` - 检查交易合规性
- `CheckOperation(ctx, operation, address)` - 检查操作合规性
- `GetPolicyMetrics()` - 获取策略性能指标

**约束**：
- 检查是只读操作
- 检查结果有缓存机制
- 支持并发检查

### IdentityRegistry（身份验证接口）

**能力**：
- `VerifyAddress(ctx, address)` - 验证地址身份
- `GetAddressCountry(ctx, address)` - 获取地址国家
- `RegisterIdentity(ctx, address, credential)` - 注册身份凭证

**约束**：
- 身份验证支持外部服务集成
- 验证结果有缓存机制

### GeoIPService（地理位置接口）

**能力**：
- `GetCountryByIP(ctx, ip)` - 根据IP获取国家
- `UpdateDatabase()` - 更新GeoIP数据库

**约束**：
- 支持多种GeoIP数据源
- 查询结果有缓存机制

---

## ⚙️ 配置说明

### 合规策略配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 启用合规检查 |
| `banned_countries` | []string | [] | 封禁国家列表（ISO-3166-1 alpha-2） |
| `banned_operations` | []string | [] | 封禁操作列表 |
| `reject_on_unknown_country` | bool | false | 未知国家是否拒绝 |

### 身份服务配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `identity_provider.url` | string | "" | 外部身份服务URL |
| `identity_provider.cache_ttl` | duration | 5m | 身份验证缓存TTL |
| `identity_provider.request_timeout` | duration | 10s | 请求超时时间 |

### GeoIP服务配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `geoip.database_path` | string | "" | GeoIP数据库路径 |
| `geoip.cache_ttl` | duration | 1h | GeoIP查询缓存TTL |

---

## 📋 使用约束

### 合规检查约束

1. **检查时机约束**：
   - 合规检查在哈希计算前进行
   - 三阶段控制：边界网关 → 内存池 → 共识层

2. **检查性能约束**：
   - 决策延迟 < 10ms（缓存命中）
   - 决策延迟 < 100ms（外部查询）
   - 支持 > 1000 TPS 的合规检查吞吐量

3. **降级策略约束**：
   - 合规服务异常时支持降级
   - 降级策略可配置

### 信息源约束

1. **身份凭证约束**：
   - 身份凭证优先于其他信息源
   - 支持外部身份服务集成

2. **GeoIP查询约束**：
   - GeoIP查询有缓存机制
   - 支持多种GeoIP数据源

---

## 🎯 典型使用场景

### 场景 1：检查交易合规性

```go
policy := compliance.NewPolicy()
err := policy.CheckTransaction(ctx, tx)
if err != nil {
    return err // 交易不合规
}
// 交易通过合规检查
```

### 场景 2：验证地址身份

```go
identityRegistry := compliance.NewIdentityRegistry()
country, err := identityRegistry.GetAddressCountry(ctx, address)
if err != nil {
    return err
}
// 获取地址国家信息
```

### 场景 3：配置合规策略

```go
config := &compliance.ComplianceConfig{
    Enabled: true,
    BannedCountries: []string{"US", "CN"},
    BannedOperations: []string{"transfer"},
}
policy.SetConfig(config)
// 合规策略已更新
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [TX 能力视图](./tx.md) - 了解交易能力
- [Mempool 能力视图](./mempool.md) - 了解内存池能力


