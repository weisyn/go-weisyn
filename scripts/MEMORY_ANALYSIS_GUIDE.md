# WES 内存问题定位指南

## 📋 概述

本指南介绍如何使用内存监控系统定位 WES 节点的内存问题。

## 🚀 快速开始

### 1. 启动节点

```bash
cd /Users/qinglong/go/src/chaincodes/WES/weisyn.git

# 开发环境
go run cmd/weisyn/main.go --env development

# 或后台运行
go run cmd/weisyn/main.go --env development --daemon
```

节点启动后，默认监听：
- **HTTP API**: `http://localhost:28680`
- **内存监控接口**: `http://localhost:28680/api/v1/system/memory`

### 2. 运行内存分析工具

#### 方式 1：使用 Python 脚本（推荐）

```bash
# 使用默认地址 (http://localhost:28680)
python3 scripts/memory_analysis.py

# 或指定自定义地址
python3 scripts/memory_analysis.py http://localhost:28680
```

#### 方式 2：使用 Shell 脚本

```bash
bash scripts/analyze_memory.sh
```

#### 方式 3：直接访问 API

```bash
# 获取内存监控数据
curl http://localhost:28680/api/v1/system/memory | jq .

# 或使用 Python 格式化
curl -s http://localhost:28680/api/v1/system/memory | python3 -m json.tool
```

## 📊 分析结果解读

### 运行时统计

- **堆分配 (heap_alloc)**: Go runtime 分配的堆内存总量
- **堆使用 (heap_inuse)**: 当前正在使用的堆内存
- **GC 次数 (num_gc)**: 垃圾回收次数
- **Goroutine 数 (num_goroutine)**: 当前运行的 goroutine 数量

### 模块统计

每个模块提供以下指标：

- **module**: 模块名称（如 `mempool.txpool`、`consensus.miner`）
- **layer**: 架构层级（`L2-Infrastructure`、`L3-Coordination`、`L4-CoreBusiness`）
- **objects**: 主要对象数量（如交易数、区块数、连接数）
- **approx_bytes**: 模块估算的内存使用（字节）
- **cache_items**: 缓存条目数
- **queue_length**: 队列/通道/pending 列表长度

## 🔍 问题定位流程

### 1. 识别内存使用异常的模块

运行分析工具后，关注：
- 内存使用超过 100MB 的模块
- 对象数量异常增长的模块
- 队列长度异常增长的模块
- 缓存条目异常增长的模块

### 2. 分析内存增长趋势

```bash
# 定期运行分析工具，记录结果
for i in {1..10}; do
    echo "=== 第 $i 次采样 ===" >> memory_trend.log
    python3 scripts/memory_analysis.py >> memory_trend.log
    sleep 30
done
```

### 3. 检查特定模块

如果发现某个模块内存使用异常，可以：

1. **查看模块日志**：
   ```bash
   tail -f data/logs/node-system.log | grep "模块名"
   ```

2. **检查模块配置**：
   - 查看模块的配置参数（如缓存大小、队列长度限制）
   - 确认是否有内存限制设置

3. **分析模块代码**：
   - 检查是否有未释放的资源
   - 检查是否有循环引用
   - 检查是否有 goroutine 泄漏

### 4. 常见内存问题

#### 问题 1：交易池内存持续增长

**症状**：
- `mempool.txpool` 模块的 `objects` 和 `approx_bytes` 持续增长
- `queue_length` 异常高

**可能原因**：
- 交易未及时清理
- 交易池大小限制未生效
- 交易验证失败但未清理

**排查步骤**：
1. 检查交易池配置：`configs/development/txpool.json`
2. 查看交易池日志：`grep "txpool" data/logs/node-system.log`
3. 检查交易清理逻辑

#### 问题 2：区块缓存内存泄漏

**症状**：
- `block.builder` 或 `chain.sync` 模块内存持续增长
- `cache_items` 异常高

**可能原因**：
- 区块缓存未设置过期时间
- 区块缓存大小限制未生效
- 区块引用未释放

**排查步骤**：
1. 检查区块缓存配置
2. 查看区块处理日志
3. 检查区块引用计数

#### 问题 3：网络连接内存泄漏

**症状**：
- `network.facade` 模块的 `objects`（连接数）持续增长
- `queue_length`（消息队列）异常高

**可能原因**：
- 连接未正确关闭
- 消息队列未及时处理
- 连接池未限制大小

**排查步骤**：
1. 检查网络连接日志
2. 查看连接生命周期管理
3. 检查消息队列处理逻辑

## 🛠️ 高级分析

### 使用 MemoryDoctor 历史数据

MemoryDoctor 会保留最近 N 次采样数据（默认 30 次），可以通过以下方式访问：

```go
// 在代码中访问
memoryDoctor := // 获取 MemoryDoctor 实例
history := memoryDoctor.GetHistory()
for _, sample := range history {
    // 分析历史数据
}
```

### 内存趋势检测

MemoryDoctor 会自动检测内存增长趋势，并在日志中记录警告：

```bash
# 查看内存趋势警告
grep "内存趋势警告" data/logs/node-system.log
```

### 生成内存报告

```bash
# 生成详细的内存报告
python3 scripts/memory_analysis.py > memory_report_$(date +%Y%m%d_%H%M%S).txt
```

## 📝 最佳实践

1. **定期监控**：建议每小时运行一次内存分析
2. **设置告警**：当内存使用超过阈值时发送告警
3. **记录基线**：记录正常情况下的内存使用基线
4. **对比分析**：对比不同时间段的内存使用情况
5. **及时处理**：发现异常后及时排查和处理

## 🔗 相关资源

- [内存监控实现文档](../_dev/11-历史与里程碑-history/implementation/MEMORY_MONITORING_IMPLEMENTATION.md)
- [内存监控测试方案](../_dev/07-测试方案-testing/SYSTEM_MEMORY_AUDIT.md)
- [内存监控验证脚本](./verify_memory_monitoring.sh)

