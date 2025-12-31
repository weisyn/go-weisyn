# 真实内存测试实施总结

## ✅ 已完成的工作

### 1. 清理旧测试内容

- ✅ 删除 `scripts/test_memory_full/` - 旧的无效测试脚本
- ✅ 删除 `scripts/test_rss_memory/` - 旧的无效测试脚本

这些脚本只是验证 MemoryDoctor 自身功能，没有真实节点/挖矿/交易，已彻底移除。

---

### 2. 观测层：MemoryDoctor 集成与日志输出

**文件**: `internal/core/infrastructure/metrics/memory_doctor.go`

**修改内容**:
- ✅ 在 `sampleOnce()` 方法中添加统一的结构化日志输出
- ✅ 使用 `logger.Info("memory_sample", ...)` 格式，包含所有关键指标：
  - `time`: 采样时间
  - `rss_mb` / `rss_bytes`: 真实物理内存（主判定指标）
  - `heap_mb` / `heap_alloc_bytes`: 堆内存分配
  - `heap_inuse_bytes`: 堆内存使用
  - `gc`: GC 次数
  - `goroutines`: Goroutine 数量
  - `modules_count`: 模块数量
  - `modules`: 各模块详细内存统计

**集成状态**:
- ✅ MemoryDoctor 已通过 Fx 模块集成到节点进程（`internal/core/infrastructure/metrics/module.go`）
- ✅ 节点启动时自动启动 MemoryDoctor，采样周期 5-10 秒
- ✅ HTTP 接口已可用：`GET /api/v1/system/memory`

**日志位置**:
- 系统日志：`{data_dir}/logs/node-system.log`
- 业务日志：`{data_dir}/logs/node-business.log`

**提取内存采样**:
```bash
# 从日志中提取所有 memory_sample 记录
grep "memory_sample" ./data/*/logs/node-system.log | jq .
```

---

### 3. 工作负载层：场景驱动脚本

**目录**: `scripts/memory_scenarios/`

#### 3.1 public_sync.sh - 公链同步测试

**功能**:
- 启动 `--chain public` 节点，连接到 WES 主网
- 观察长时间同步 + 收/发区块 + GossipSub 流量下的内存曲线
- 主要压力在：P2P、Sync、Block/Tx 存储索引等

**使用方法**:
```bash
./scripts/memory_scenarios/public_sync.sh
./scripts/memory_scenarios/public_sync.sh --clean  # 清理旧数据
```

**运行时长建议**:
- 快速验证：10-30 分钟
- 稳定性测试：2-24 小时

#### 3.2 private_mining.sh - 私链挖矿测试

**功能**:
- 自动生成私链配置（如果不存在）
- 启动 `--chain private` 节点
- 覆盖：TxPool、BlockBuilder、Consensus POW、EUTXO Writer、Snapshot Writer 等

**使用方法**:
```bash
./scripts/memory_scenarios/private_mining.sh
./scripts/memory_scenarios/private_mining.sh --clean  # 清理旧数据
```

**启动挖矿**（在另一个终端）:
```bash
go run ./cmd/cli mining start
# 或
./bin/weisyn-cli mining start
```

#### 3.3 tx_stress.sh - 高并发交易压测

**功能**:
- 专门压 TxPool / Mempool / Executor / UTXO 索引等模块
- 可控制 TPS 和压测时长

**使用方法**:
```bash
./scripts/memory_scenarios/tx_stress.sh <from_address> <to_address> [tps] [duration_minutes]

# 示例：10 TPS，持续 10 分钟
./scripts/memory_scenarios/tx_stress.sh \
  CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR \
  CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG \
  10 10
```

**前置条件**:
- 节点已启动（私链或公链）
- 至少有两个账户（from 和 to）
- from 账户有足够余额

#### 3.4 auto_full_test.sh - 全自动测试（推荐）

**功能**:
- ✅ 完全自动化：一条命令完成"启动节点 + 初始化钱包 + 启动挖矿 + 发送交易"
- ✅ 自动从 genesis 配置提取账户信息
- ✅ 自动创建 Profile 和导入账户
- ✅ 自动启动挖矿
- ✅ 可选自动发送交易

**使用方法**:
```bash
# 基本用法：启动节点 + 挖矿（30 分钟）
./scripts/memory_scenarios/auto_full_test.sh

# 包含交易压测（30 分钟）
./scripts/memory_scenarios/auto_full_test.sh --with-tx

# 自定义时长（60 分钟，包含交易）
./scripts/memory_scenarios/auto_full_test.sh --with-tx --duration 60

# 清理旧数据
./scripts/memory_scenarios/auto_full_test.sh --clean
```

**前置条件**:
- 配置文件 `data/memory-test/auto-full-config.json` 中必须包含 `genesis.accounts[0].private_key` 和 `genesis.accounts[0].address`
- 如果使用 `--with-tx`，建议配置文件中至少有两个 genesis 账户

**工作流程**:
1. 自动生成私链配置（如果不存在）
2. 从配置文件中提取 genesis 账户私钥和地址
3. 后台启动节点
4. 等待节点就绪（检查 HTTP 端口）
5. 创建/切换 CLI Profile
6. 导入 genesis 账户到 keystore
7. 启动挖矿
8. （可选）启动交易压测
9. 等待指定时长后自动清理

---

### 4. 评估层：日志分析工具

**文件**: `scripts/analyze_memory_from_logs.py`

**功能**:
- 从节点日志中提取 `memory_sample` 记录
- 计算每小时内存增长
- 生成 CSV 报告、文本摘要报告、趋势图（可选）

**使用方法**:
```bash
# 基本用法：生成 CSV 报告和文本摘要
python3 scripts/analyze_memory_from_logs.py \
  --log ./data/memory-test/public-sync/logs/node-system.log \
  --output ./memory-report.csv \
  --summary ./memory-summary.txt

# 生成趋势图（需要 matplotlib）
python3 scripts/analyze_memory_from_logs.py \
  --log ./data/memory-test/public-sync/logs/node-system.log \
  --output ./memory-report.csv \
  --plot memory-trend.png
```

**输出内容**:
- CSV 报告：包含所有采样点的详细数据
- 文本摘要：总体变化、每小时增长统计、评估结果
- 趋势图（可选）：RSS、Heap、Goroutines 的时间序列图

**评估标准**:
- ✅ **正常**: RSS 每小时增长 < 1-2%，或绝对值 < 20MB/h
- ⚠️ **可疑**: RSS 每小时增长 > 5% 或 > 50MB/h
- ❌ **异常**: 线性/指数型持续增长

---

## 📋 使用流程示例

### 完整测试流程（公链同步场景）

```bash
# 1. 启动节点（后台运行）
./scripts/memory_scenarios/public_sync.sh > /dev/null 2>&1 &

# 2. 等待一段时间（例如 2 小时）
sleep 7200

# 3. 停止节点
pkill -f "weisyn-node.*--chain public"

# 4. 分析日志
python3 scripts/analyze_memory_from_logs.py \
  --log ./data/memory-test/public-sync/logs/node-system.log \
  --output ./memory-report.csv \
  --summary ./memory-summary.txt \
  --plot memory-trend.png

# 5. 查看报告
cat memory-summary.txt
```

### 完整测试流程（私链挖矿场景）

```bash
# 1. 启动节点（后台运行）
./scripts/memory_scenarios/private_mining.sh > /dev/null 2>&1 &

# 2. 等待节点启动（约 10 秒）
sleep 10

# 3. 在另一个终端启动挖矿
go run ./cmd/cli mining start

# 4. 可选：叠加交易压力
./scripts/memory_scenarios/tx_stress.sh \
  CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR \
  CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG \
  5 30

# 5. 等待一段时间后停止节点并分析
pkill -f "weisyn-node.*--chain private"
python3 scripts/analyze_memory_from_logs.py \
  --log ./data/memory-test/private-mining/logs/node-system.log \
  --output ./memory-report.csv \
  --summary ./memory-summary.txt
```

---

## 🔍 关键改进点

### 1. 真实节点集成
- ✅ MemoryDoctor 已集成到节点进程，不再是独立 demo
- ✅ 采样数据自动写入日志，无需额外配置

### 2. 真实工作负载
- ✅ 公链同步：真实网络流量和区块同步
- ✅ 私链挖矿：真实共识和区块生成
- ✅ 交易压测：真实交易处理和 UTXO 操作
- ✅ **全自动测试**：一条命令完成节点启动 + 钱包初始化 + 挖矿 + 交易

### 3. 自动化分析
- ✅ 日志解析工具自动提取内存采样
- ✅ 自动计算每小时增长和评估结果
- ✅ 支持 CSV 导出和图表生成

### 4. 钱包自动化（新增）
- ✅ **自动提取 genesis 账户**：从配置文件中提取私钥和地址
- ✅ **自动创建 Profile**：无需手动配置 CLI
- ✅ **自动导入账户**：使用 genesis 账户自动导入到 keystore
- ✅ **自动启动挖矿**：无需手动执行 `mining start`
- ✅ **自动发送交易**：可选自动交易压测，无需手动提供地址

---

## 📝 注意事项

1. **日志模式**: 所有场景脚本都设置 `WES_CLI_MODE=true`，日志只写入文件，不刷屏
2. **数据目录**: 每个场景使用独立的数据目录，避免相互干扰
3. **长时间测试**: 建议在稳定环境中运行 24h+ 以发现缓慢泄漏
4. **资源监控**: 建议同时监控 CPU、磁盘 I/O 等系统资源

---

## 🔗 相关文档

- **场景脚本说明**: `scripts/memory_scenarios/README.md`
- **内存分析指南**: `scripts/MEMORY_ANALYSIS_GUIDE.md`
- **节点启动说明**: `cmd/README.md`

