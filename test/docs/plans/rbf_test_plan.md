#WES RBF (Replace-By-Fee) 功能测试计划 - 生产验证版

## 🎯 **测试目标与范围**

### 更新时间：2025年01月15日 (生产验证版)

**🎯 RBF功能完整性验证**：
- ✅ **架构验证**: Transaction Domain统一RBF处理，TxPool专注存储
- 🎯 **功能验证**: RBF合并机制在真实交易场景中的完整性测试
- 🎯 **场景验证**: 成功合并与失败回退两种核心场景
- 🎯 **一致性验证**: 余额计算、UTXO状态、区块链状态一致性

---

## 🚀 **测试执行准备阶段**

### Phase 0 - 环境准备与验证

#### T0.1 节点重建与清理
```bash
# 1. 重新编译节点
cd /Users/qinglong/go/src/chaincodes/TV/WES
go build -o bin/node cmd/node/main.go

# 2. 清理历史数据
rm -rf data/badger/*
rm -rf data/logs/*
mkdir -p data/badger data/logs

# 3. 验证配置文件
cat configs/genesis.json | jq '.'
cat configs/config.json | jq '.'
```

#### T0.2 区块链初始化验证
```bash
# 启动节点
./bin/node --config=configs/config.json

# 验证创世区块
curl -X POST http://localhost:8080/v1/blocks/height/0

# 验证创世地址余额
curl -X POST http://localhost:8080/v1/accounts/balance \
  -H "Content-Type: application/json" \
  -d '{"address": "GENESIS_ADDRESS_1"}'

curl -X POST http://localhost:8080/v1/accounts/balance \
  -H "Content-Type: application/json" \
  -d '{"address": "GENESIS_ADDRESS_2"}'
```

**预期结果**：
- ✅ 节点启动成功，无错误日志
- ✅ 创世区块正确生成，高度为0
- ✅ 创世地址余额符合配置预期
- ✅ RBF组件初始化成功

---

## 🔄 **RBF核心功能测试**

### Phase 1 - RBF成功合并场景测试

#### T1.1 第一笔交易提交（不挖矿）
```bash
# 停止挖矿（确保交易停留在内存池）
curl -X POST http://localhost:8080/v1/mining/stop

# 提交第一笔交易：A向B转账X金额
curl -X POST http://localhost:8080/v1/transactions/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "ADDRESS_A",
    "to_address": "ADDRESS_B", 
    "amount": 1000000000000,
    "fee": 1000000
  }'
```

**验证点**：
- ✅ 交易成功提交到API
- ✅ 交易进入内存池，状态为pending
- ✅ 交易哈希生成正确
- ✅ 挖矿未启动，交易保持在内存池

#### T1.2 第二笔交易提交（触发RBF）
```bash
# 提交第二笔交易：A向B转账Y金额（更大金额）
curl -X POST http://localhost:8080/v1/transactions/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "ADDRESS_A",
    "to_address": "ADDRESS_B",
    "amount": 2000000000000,
    "fee": 2000000
  }'
```

**RBF处理预期行为**：
1. **冲突检测**: Transaction Domain检测到UTXO冲突
2. **RBF触发**: 调用RBF处理器处理冲突
3. **合并策略**: 选择金额更大的交易（Y）
4. **原子替换**: 从内存池移除交易X，添加合并后的交易
5. **用户响应**: API返回成功，用户无感知

#### T1.3 内存池状态验证
```bash
# 查询内存池状态
curl -X GET http://localhost:8080/v1/txpool/status

# 查询特定交易状态
curl -X POST http://localhost:8080/v1/transactions/status \
  -H "Content-Type: application/json" \
  -d '{"tx_hash": "FIRST_TX_HASH"}'

curl -X POST http://localhost:8080/v1/transactions/status \
  -H "Content-Type: application/json" \
  -d '{"tx_hash": "SECOND_TX_HASH"}'
```

**验证点**：
- ✅ 内存池只包含一笔交易（合并后的交易）
- ✅ 第一笔交易状态为replaced
- ✅ 第二笔交易（或合并交易）状态为pending
- ✅ 合并交易的金额、费用符合预期

#### T1.4 挖矿验证与余额检查
```bash
# 启动挖矿
curl -X POST http://localhost:8080/v1/mining/start

# 等待出块
sleep 10

# 查询最新区块
curl -X GET http://localhost:8080/v1/blocks/latest

# 验证余额
curl -X POST http://localhost:8080/v1/accounts/balance \
  -H "Content-Type: application/json" \
  -d '{"address": "ADDRESS_A"}'

curl -X POST http://localhost:8080/v1/accounts/balance \
  -H "Content-Type: application/json" \
  -d '{"address": "ADDRESS_B"}'
```

**预期结果**：
- ✅ 新区块包含合并后的交易
- ✅ ADDRESS_A余额减少：原余额 - 2000000000000 - 2000000（金额+费用）
- ✅ ADDRESS_B余额增加：原余额 + 2000000000000
- ✅ 矿工地址余额增加挖矿奖励和交易费用

---

### Phase 2 - RBF失败回退场景测试

#### T2.1 环境重置
```bash
# 停止当前节点
pkill -f "bin/node"

# 清理数据，重新启动
rm -rf data/badger/* data/logs/*
./bin/node --config=configs/config.json

# 停止挖矿
curl -X POST http://localhost:8080/v1/mining/stop
```

#### T2.2 第一笔交易提交（消耗大部分余额）
```bash
# 提交第一笔交易：A向B转账接近全部余额
curl -X POST http://localhost:8080/v1/transactions/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "ADDRESS_A",
    "to_address": "ADDRESS_B",
    "amount": 9000000000000,
    "fee": 1000000
  }'
   ```

#### T2.3 第二笔交易提交（余额不足，RBF失败）
```bash
# 提交第二笔交易：A向C转账（余额不足）
curl -X POST http://localhost:8080/v1/transactions/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "ADDRESS_A", 
    "to_address": "ADDRESS_C",
    "amount": 8000000000000,
    "fee": 2000000
  }'
```

**RBF失败预期行为**：
1. **冲突检测**: 检测到UTXO冲突
2. **RBF尝试**: 尝试合并两笔交易
3. **余额验证**: 发现合并后余额不足
4. **失败回退**: RBF处理失败，维持原有交易
5. **错误处理**: 第二笔交易被拒绝，返回余额不足错误

#### T2.4 失败场景验证与恢复
```bash
# 验证内存池状态
curl -X GET http://localhost:8080/v1/txpool/status

# 验证第一笔交易仍在内存池
curl -X POST http://localhost:8080/v1/transactions/status \
  -H "Content-Type: application/json" \
  -d '{"tx_hash": "FIRST_TX_HASH"}'

# 启动挖矿，确认第一笔交易
curl -X POST http://localhost:8080/v1/mining/start

# 等待出块并验证余额
sleep 10
curl -X GET http://localhost:8080/v1/blocks/latest
```

**验证点**：
- ✅ 第二笔交易被拒绝，API返回余额不足错误
- ✅ 第一笔交易保持在内存池，状态为pending
- ✅ 挖矿后第一笔交易被确认
- ✅ 余额计算正确：ADDRESS_A减少9000000000000+费用，ADDRESS_B增加9000000000000

---

## 📊 **测试验证矩阵**

### 核心验证点

| 测试场景 | 验证项目 | 预期结果 | 状态 |
|---------|----------|----------|------|
| **成功合并** | RBF触发时机 | Transaction Domain处理冲突 | 🔄 待验证 |
| **成功合并** | 交易合并逻辑 | 选择更优交易，原子替换 | 🔄 待验证 |
| **成功合并** | 内存池状态 | 只保留合并后交易 | 🔄 待验证 |
| **成功合并** | 余额一致性 | 转账金额和费用正确扣除 | 🔄 待验证 |
| **失败回退** | 合并失败检测 | 余额不足时正确识别 | 🔄 待验证 |
| **失败回退** | 原交易保持 | 第一笔交易继续有效 | 🔄 待验证 |
| **失败回退** | 错误处理 | 用户收到明确错误信息 | 🔄 待验证 |
| **系统级** | 挖矿集成 | RBF不影响出块流程 | 🔄 待验证 |

### 技术指标验证

| 技术指标 | 验证方法 | 预期值 | 状态 |
|---------|----------|--------|------|
| **架构分离** | 代码审查 | TxPool无RBF业务逻辑 | ✅ 已验证 |
| **依赖注入** | 启动日志 | RBF处理器成功注入 | ✅ 已验证 |
| **接口统一** | 编译验证 | 无类型冲突错误 | ✅ 已验证 |
| **性能影响** | 交易延迟 | RBF处理<100ms | 🔄 待测试 |
| **内存使用** | 资源监控 | 内存池大小合理 | 🔄 待测试 |

---

## 🎯 **测试执行检查清单**

### 执行前检查
- [ ] bin/node重新编译完成
- [ ] 数据库和日志目录清理完成  
- [ ] 配置文件验证正确
- [ ] 创世区块和地址余额验证

### Phase 1 执行检查（成功场景）
- [ ] 第一笔交易成功提交到内存池
- [ ] 第二笔交易触发RBF处理
- [ ] 内存池状态符合预期（只有合并交易）
- [ ] 挖矿后余额计算正确

### Phase 2 执行检查（失败场景）
- [ ] 环境重置完成
- [ ] 第一笔交易（大额）成功提交
- [ ] 第二笔交易（余额不足）被正确拒绝
- [ ] 第一笔交易保持有效并成功出块

### 最终验证
- [ ] 两种场景的余额一致性验证
- [ ] 区块链状态完整性检查
- [ ] 系统日志无异常错误
- [ ] RBF架构职责边界清晰

---

## 🎉 **预期测试成果**

### 成功标准
1. **✅ 架构验证**: RBF处理完全在Transaction Domain，职责清晰
2. **✅ 功能验证**: 成功和失败两种场景都能正确处理
3. **✅ 一致性验证**: 余额、UTXO、区块状态完全一致
4. **✅ 用户体验**: API响应正确，错误信息清晰
5. **✅ 系统稳定**: 挖矿、同步等其他功能不受影响

### 技术收获
1. **生产级RBF**: 真实环境下的RBF合并机制验证
2. **架构优化**: Transaction Domain统一处理的正确性证明
3. **错误处理**: 完善的失败回退和错误提示机制
4. **系统集成**: RBF与其他区块链组件的无缝集成

**总体目标**: 📈 **构建生产就绪的RBF功能，确保区块链的交易处理能力达到企业级标准** 