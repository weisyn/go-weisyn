# 智能合约开发故障排查

---

## 🎯 故障排查概览

本文档帮助您解决智能合约开发中的常见问题，包括编译、部署、调用、性能优化等。

---

## ❓ 常见问题

### Q：合约编译失败

**问题**：合约编译时出现错误

**可能原因**：
- Go 版本不兼容
- 依赖包缺失
- 代码语法错误
- TinyGo 配置问题

**解决方案**：

**步骤 1：检查 Go 版本**
```bash
go version
# 需要 Go 1.21+ 或 TinyGo
```

**步骤 2：检查依赖**
```bash
go mod download
go mod tidy
```

**步骤 3：检查代码语法**
```bash
go build ./...
go vet ./...
```

**步骤 4：检查 TinyGo 配置**
```bash
# 确认 TinyGo 已安装
tinygo version

# 检查编译目标
tinygo build -o contract.wasm -target wasm -scheduler=none -no-debug .
```

**常见编译错误**：
- `undefined: xxx` - 函数或变量未定义
- `cannot use xxx` - 类型不匹配
- `import cycle` - 循环依赖
- `WASM 编译失败` - TinyGo 配置错误

---

### Q：合约部署失败

**问题**：合约部署到链上失败

**可能原因**：
- WASM 文件格式错误
- 合约大小超限
- 网络连接问题
- 权限不足
- 资源哈希冲突

**解决方案**：

**步骤 1：验证 WASM 文件**
```bash
# 检查文件格式
file contract.wasm
# 应该显示：WebAssembly (wasm) binary

# 检查文件大小
ls -lh contract.wasm
# 确认未超过限制（通常 < 10MB）
```

**步骤 2：检查网络连接**
```bash
# 检查节点状态
wes node status

# 检查节点是否同步
wes chain status | grep syncing
```

**步骤 3：检查部署权限**
```bash
# 确认账户有足够余额
wes account balance <your-address>

# 确认账户有部署权限
wes account info <your-address>
```

**步骤 4：查看部署日志**
```bash
# 查看部署交易详情
wes tx get <deploy-tx-hash>

# 查看错误日志
tail -f ./logs/wes.log | grep -i "deploy\|contract"
```

**常见部署错误**：
- `invalid wasm format` - WASM 文件格式错误
- `contract too large` - 合约大小超限
- `insufficient balance` - 余额不足
- `resource hash conflict` - 资源哈希冲突（已存在相同资源）

---

### Q：合约调用失败

**问题**：调用合约方法时失败

**可能原因**：
- 方法不存在或未导出
- 参数格式错误
- 权限不足
- 合约执行失败
- CU（算力）不足

**解决方案**：

**步骤 1：检查方法是否存在**
```bash
# 查看合约信息
wes resource get <contract-hash>

# 查看合约方法列表
wes contract methods <contract-hash>
```

**步骤 2：检查参数格式**
```bash
# 验证参数格式
wes tx validate <tx-hash>

# 查看调用交易详情
wes tx get <tx-hash> | grep -A 10 "params\|inputs"
```

**步骤 3：检查权限**
```bash
# 确认调用者有权限
wes account info <caller-address>

# 检查合约权限配置
wes resource get <contract-hash> | grep -i "owner\|permission"
```

**步骤 4：查看执行日志**
```bash
# 查看合约执行日志
tail -f ./logs/wes.log | grep -i "contract\|wasm\|ispc"

# 查看执行错误
wes tx get <tx-hash> | grep -i error
```

**常见调用错误**：
- `method not found` - 方法不存在或未导出
- `invalid parameters` - 参数类型或格式错误
- `insufficient permissions` - 权限不足
- `execution failed` - 执行逻辑错误
- `compute units exceeded` - CU（算力）消耗超限

---

### Q：合约执行的 CU（算力）消耗过高，应该怎么优化？

**说明**：在微迅链中，**没有 Gas 概念**，而是使用 **CU（Compute Units，计算单位）** 作为统一的算力计量单位。CU 用于计量合约和 AI 模型的算力消耗，是系统内部的资源计量标准，**用户无需理解或手动设置 CU 参数**。

**CU 的作用**：
- **内部资源计量**：系统内部用于计量算力消耗
- **节点限流/配额管理**：节点可以根据 CU 进行资源限制
- **统计报表**：记录谁消耗了多少算力
- **定价策略**：资源所有者可以基于 CU 设置定价（可选）

**问题**：某些合约在执行时，CU 消耗过高（例如本地模拟执行、profiling 或链上监控指标显示 CU 消耗偏高）。

**CU 计算公式**（智能合约）：

```
CU = base_cu + (input_size_bytes / 1024) * input_factor + (exec_time_ms / 100) * time_factor + ops_contribution

其中：
- base_cu: 基础 CU（合约类型相关，默认值）
- input_factor: 输入大小因子（默认 0.1）
- time_factor: 执行时间因子（默认 1.0）
- ops_contribution: 操作统计贡献
  - 存储操作：storage_ops * storage_op_factor
  - 跨合约调用：cross_contract_calls * cross_call_factor
- 复杂度系数：资源特定的调整因子（默认 1.0）
```

**常见原因**：
- **循环计算过多**：在合约内做大规模枚举/聚合（例如 O(n²) 复杂度）
- **状态读写频繁**：反复读写同一批 UTXO / 状态
- **事件发出过多**：过度依赖事件传输大量数据
- **跨合约调用过多**：频繁调用其他合约
- **输入数据过大**：调用参数包含大量数据

**优化方向**：

**1. 优化循环逻辑**
- 将 O(n²) 降为 O(n) / O(log n)
- 避免在合约内做大规模计算
- 考虑将复杂计算移到链下，合约只做验证

**示例**：
```go
// ❌ 低效：O(n²) 复杂度
for i := 0; i < len(items); i++ {
    for j := 0; j < len(items); j++ {
        // 复杂计算
    }
}

// ✅ 高效：O(n) 复杂度
for i := 0; i < len(items); i++ {
    // 单次遍历完成计算
}
```

**2. 减少状态读写次数**
- 合理设计 EUTXO（Extended UTXO）三层输出结构（AssetOutput/ResourceOutput/StateOutput）
- 把相关状态聚合在少量 UTXO 中，一次交易完成必要变更
- 避免重复读取相同状态

**示例**：
```go
// ❌ 低效：多次状态读写
for i := 0; i < len(items); i++ {
    state := getState(items[i])  // 多次读取
    updateState(items[i], newValue)  // 多次写入
}

// ✅ 高效：批量状态操作
states := batchGetState(items)  // 一次批量读取
batchUpdateState(items, newValues)  // 一次批量写入
```

**3. 精简事件输出**
- 事件只承载必要的索引信息
- 大数据通过 URES（Universal Resource State）统一资源管理机制管理
- 避免在事件中传输大量数据

**示例**：
```go
// ❌ 低效：事件包含大量数据
emitEvent("DataUpdated", largeDataStruct)  // 事件数据过大

// ✅ 高效：事件只包含索引信息
emitEvent("DataUpdated", dataHash)  // 事件只包含哈希
// 实际数据通过 URES 存储，通过哈希引用
```

**4. 优化跨合约调用**
- 减少不必要的跨合约调用
- 合并多个调用为一次调用
- 考虑将相关逻辑合并到同一合约

**5. 优化输入数据**
- 减少调用参数的数据量
- 使用引用（哈希）而不是完整数据
- 考虑分批处理大数据

**查看 CU 消耗**：
```bash
# 查看交易执行详情（包含 CU）
wes tx get <tx-hash> | grep -i "compute_units\|cu"

# 查看合约执行统计
wes node metrics | grep -i "compute_units\|cu"

# 查看资源消耗报表
wes resource stats <contract-hash>
```

**进一步说明**：
- **用户视角**：是否需要付费、付多少费，请参考 [费用与经济模型](../../product/economics.md)
- **开发者视角**：应关注在实现相同业务逻辑的前提下，尽量减少不必要的计算和状态操作，降低 CU 消耗
- **CU 与费用**：CU 是内部计量单位，费用由资源所有者设置的定价策略决定（可选），用户通常无需直接面对 CU 概念

---

### Q：合约执行超时

**问题**：合约执行时间过长，超过最大执行时间限制

**可能原因**：
- 执行逻辑过于复杂
- 循环计算过多
- 外部调用延迟高
- 网络问题

**解决方案**：

**步骤 1：检查执行时间配置**
```bash
# 查看当前配置
wes node config show | grep -i "max_execution_time"

# 检查默认超时时间（通常 30s）
```

**步骤 2：优化执行逻辑**
- 参考上面的"CU 消耗优化"方法
- 减少循环计算
- 优化算法复杂度

**步骤 3：检查外部调用**
```bash
# 查看外部调用日志
tail -f ./logs/wes.log | grep -i "external\|api"

# 检查外部系统响应时间
```

**步骤 4：使用异步处理**
- 对于耗时操作，考虑拆分为多个交易
- 使用事件机制通知后续处理

---

### Q：合约状态不一致

**问题**：合约执行后，状态与预期不一致

**可能原因**：
- EUTXO 状态模型理解错误
- 交易未正确消费/创建 UTXO
- 并发执行导致状态冲突
- 状态查询时机错误

**解决方案**：

**步骤 1：理解 EUTXO 状态模型**
- 状态不是"合约账户下的变量"，而是 UTXO 集合
- 状态变更 = 消费旧 UTXO + 创建新 UTXO
- 参考 [`components/eutxo.md`](../../components/eutxo.md)

**步骤 2：检查 UTXO 状态**
```bash
# 查看地址的所有 UTXO
wes account utxo <address>

# 查看特定类型的 UTXO
wes account utxo <address> --type asset
wes account utxo <address> --type state
```

**步骤 3：追踪状态变更**
```bash
# 查看交易历史
wes account history <address>

# 查看每笔交易的输入输出
wes tx get <tx-hash> | grep -A 20 "inputs\|outputs"
```

**步骤 4：检查并发冲突**
- 确认 UTXO 是否被其他交易消费
- 检查交易顺序和依赖关系

---

## 📚 相关文档

- [开发入门](./beginner.md) - 合约开发入门
- [推荐模式](./patterns.md) - 推荐实践模式
- [API 参考](../../reference/api/) - API 接口文档
- [EUTXO 能力视图](../../components/eutxo.md) - 理解 EUTXO 状态模型
- [ISPC 能力视图](../../components/ispc.md) - 理解可验证计算机制

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [开发入门](./beginner.md) - 合约开发入门
- [费用与经济模型](../../product/economics.md) - 了解费用机制
