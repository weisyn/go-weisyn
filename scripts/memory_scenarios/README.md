# 内存测试场景脚本

本目录包含用于真实节点内存测试的场景驱动脚本。

## 📋 概述

这些脚本用于测试**真实节点 + 真实工作负载**下的内存行为，而不是简单的 MemoryDoctor demo。

## 🎯 测试目标

- 验证节点在长时间运行下的内存行为是否健康
- 检测是否存在内存泄漏或异常增长
- 观察不同工作负载下的内存曲线

## 📁 场景脚本

### 1. public_sync.sh - 公链同步测试

**目的**：观察长时间同步 + 收/发区块 + GossipSub 流量下的内存曲线

**特点**：
- 连接到 WES 主网
- 主要压力在：P2P、Sync、Block/Tx 存储索引等
- 适合长时间运行（数小时~数天）

**使用方法**：
```bash
# 快速验证（10-30 分钟）
./scripts/memory_scenarios/public_sync.sh

# 清理旧数据后重新开始
./scripts/memory_scenarios/public_sync.sh --clean
```

**预期结果**：
- RSS 在启动 10-30 分钟后逐渐趋于稳定或缓慢上升
- 不应出现线性/指数型持续上升

---

### 2. private_mining.sh - 私链挖矿测试

**目的**：在可控环境中跑"挖矿 + 全工作流"，避免主网因素干扰

**特点**：
- 覆盖：TxPool、BlockBuilder、Consensus POW、EUTXO Writer、Snapshot Writer 等
- 交易量不高，但区块不断生成

**使用方法**：
```bash
# 方式 A：手动挖矿（前台模式）
./scripts/memory_scenarios/private_mining.sh

# 方式 B：自动挖矿（后台模式，自动导入 genesis 账户并启动挖矿）
./scripts/memory_scenarios/private_mining.sh --auto-mining

# 清理后重新开始
./scripts/memory_scenarios/private_mining.sh --clean
```

**手动启动挖矿**（仅方式 A，在另一个终端）：
```bash
# 使用 go run
go run ./cmd/cli mining start

# 或使用编译后的二进制
./bin/weisyn-cli mining start
```

**可选：叠加交易压力**：
```bash
# 每隔 1s 发一笔小额转账
./scripts/memory_scenarios/tx_stress.sh <from> <to> 1 600
```

---

### 2.5. auto_full_test.sh - 全自动测试（推荐）

**目的**：一条命令完成"启动节点 + 初始化钱包 + 启动挖矿 + 发送交易"的完整流程

**特点**：
- ✅ 完全自动化，无需手动操作
- ✅ 自动从 genesis 配置提取账户信息
- ✅ 自动创建 Profile 和导入账户
- ✅ 自动启动挖矿
- ✅ 可选自动发送交易

**使用方法**：
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

**前置条件**：
- 配置文件 `data/memory-test/auto-full-config.json` 中必须包含 `genesis.accounts[0].private_key` 和 `genesis.accounts[0].address`
- 如果使用 `--with-tx`，建议配置文件中至少有两个 genesis 账户

**工作流程**：
1. 自动生成私链配置（如果不存在）
2. 从配置文件中提取 genesis 账户私钥和地址
3. 后台启动节点
4. 等待节点就绪（检查 HTTP 端口）
5. 创建/切换 CLI Profile
6. 导入 genesis 账户到 keystore
7. 启动挖矿
8. （可选）启动交易压测
9. 等待指定时长后自动清理

**优势**：
- 🚀 一键启动，无需手动操作
- 🔒 使用 genesis 账户，无需额外配置钱包
- 📊 自动记录内存采样，便于后续分析

---

### 3. tx_stress.sh - 高并发交易压测

**目的**：专门压 TxPool / Mempool / Executor / UTXO 索引等模块

**特点**：
- 可控制 TPS 和压测时长
- 适合短时间高强度测试

**使用方法**：
```bash
# 基本用法
./scripts/memory_scenarios/tx_stress.sh <from_address> <to_address> [tps] [duration_minutes]

# 示例：10 TPS，持续 10 分钟
./scripts/memory_scenarios/tx_stress.sh \
  CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR \
  CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG \
  10 10

# 示例：20 TPS，持续 30 分钟
./scripts/memory_scenarios/tx_stress.sh \
  CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR \
  CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG \
  20 30
```

**前置条件**：
- 节点已启动（私链或公链）
- 至少有两个账户（from 和 to）
- from 账户有足够余额

---

## 📊 内存采样

所有场景都会自动启用 MemoryDoctor，每 5-10 秒采样一次内存状态。

**采样指标**：
- `rss_mb` / `rss_bytes`：真实物理内存（主判定指标）
- `heap_mb` / `heap_alloc_bytes`：堆内存分配
- `heap_inuse_bytes`：堆内存使用
- `gc`：GC 次数
- `goroutines`：Goroutine 数量
- `modules`：各模块内存统计

**日志位置**：
- 系统日志：`{data_dir}/logs/node-system.log`
- 业务日志：`{data_dir}/logs/node-business.log`

**提取内存采样**：
```bash
# 从系统日志中提取所有 memory_sample 记录
grep "memory_sample" ./data/memory-test/*/logs/node-system.log | jq .

# 或使用分析工具
python3 scripts/analyze_memory_from_logs.py \
  --log ./data/memory-test/public-sync/logs/node-system.log \
  --output ./memory-report.csv
```

---

## 🔍 评估标准

### 正常行为
- 预热期（前 10-30 分钟）内 RSS 可能快速上升，然后趋于稳定
- 之后 RSS 每小时增长 < 1-2%，或绝对值 < 20MB/h

### 可疑行为
- 稳定期之后，RSS 每小时持续增长 > 5% 或 > 50MB/h，且无明显波动/回落
- `goroutines` 持续上升且不回落

### 明显泄漏
- 线性或近线性增长（画图可以明显看到一条斜线）
- 例如：连续 4 小时内从 500MB 涨到 1.5GB 以上

---

## 📝 注意事项

1. **日志模式**：所有脚本都会设置 `WES_CLI_MODE=true`，日志只写入文件，不刷屏
2. **数据目录**：每个场景使用独立的数据目录，避免相互干扰
3. **长时间测试**：建议在稳定环境中运行 24h+ 以发现缓慢泄漏
4. **资源监控**：建议同时监控 CPU、磁盘 I/O 等系统资源

---

## 🔗 相关工具

- **日志分析**：`scripts/analyze_memory_from_logs.py` - 从日志中提取并分析内存趋势
- **内存监控指南**：`scripts/MEMORY_ANALYSIS_GUIDE.md` - 详细的内存分析文档

