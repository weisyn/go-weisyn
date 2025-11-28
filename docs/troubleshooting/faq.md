# WES 常见问题（FAQ）

---

## 🎯 FAQ 概览

本文档回答 WES 使用中的常见问题，特别关注：
- **从 Bitcoin/Ethereum 迁移过来的用户**：心智模型差异和常见陷阱
- **开发者典型失败场景**：交易不确认、状态不一致、合约调用失败等
- **运维快速诊断**：5 分钟内定位问题
- **环境差异问题**：本地测试正常，上测试网/主网就失败

---

## 📋 通用问题

### Q：WES 是什么？

A：本文件聚焦"使用与排障"问题。关于 WES 的完整定位和世界观，请先阅读 [产品总览](../overview.md)。

### Q：如果我只想"先把系统跑起来"，应该按什么顺序看文档？

A：推荐顺序：
- 1）先看 [快速开始](../tutorials/quickstart/) 把本地单节点跑起来
- 2）再看 [部署指南](../tutorials/deployment/) 按你的环境（本地 / Docker / 云）部署
- 3）出现问题时，优先看本目录下的：
  - [`operations.md`](./operations.md) - 节点 / 网络 / 同步等运维问题
  - [`performance.md`](./performance.md) - 性能与资源相关问题

### Q：WES 的核心价值是什么？

A：WES 的三大核心价值（突破外部副作用瓶颈、支持企业级长事务、零改造成本集成）在 [产品总览](../overview.md) 和 [`product/value-proposition.md`](../product/value-proposition.md) 中有系统阐述，这里不再展开。

### Q：如何使用 WES？

A：快速开始步骤：
1. 安装 WES 节点
2. 连接到网络
3. 配置网络与账户
4. 获取测试代币
5. 发送第一笔交易

详细步骤请参考 [快速开始](../tutorials/quickstart/)。

---

## 🧠 心智模型与差异（从 Bitcoin / Ethereum 迁移）

### Q：从以太坊迁移到微迅链，最容易踩的坑有哪些？

A：典型"心智模型不匹配"的坑主要有：
- **状态模型不同**：以太坊是账户模型，微迅链采用 **EUTXO 三层输出模型**，状态保存在 UTXO 中，而不是"合约里的一坨变量"。请先阅读：
  - [`components/eutxo.md`](../components/eutxo.md) - 账本与状态模型
  - [`components/tx.md`](../components/tx.md) - 交易如何消费 / 生成 UTXO
- **外部系统集成方式不同**：微迅链支持在合约中直接调用外部系统（HTTP、API等），但需要通过 **ISPC 受控外部交互机制**（声明+佐证+验证），而不是传统区块链的预言机模式。只有执行节点会调用一次外部系统，其他节点通过验证 ZK 证明来确认，无需重复调用。可参考：
  - [企业级流程场景](../tutorials/scenarios/enterprise-workflow.md)
  - [`components/ispc.md`](../components/ispc.md) - ISPC 受控外部交互机制
- **费用体验不同**：微迅链没有 Gas 概念，使用 CU（Compute Units，计算单位）作为内部算力计量。用户看不到 Gas，只看到极简的网络手续费；合约部署和大部分业务调用基本无感成本。详情见：
  - [费用与经济模型](../product/economics.md)

### Q：为什么我在微迅链里找不到"全局可变状态变量"那一套？

A：在微迅链中：
- **状态不是挂在"合约账户"下面的一坨变量**，而是被拆解为一组有类型的 UTXO（Asset / Resource / State 三层），每一次状态变更都是"旧 UTXO 被消费 + 新 UTXO 被创建"
- **读 / 写路径都通过 EUTXO + TX 组件**，而不是任意时刻从"全局状态树"读写

因此，当你遇到"状态和预期不一致"的问题时，排查顺序建议是：
- 1）用 CLI / API 查看相关地址的 UTXO 与资源：`wes account utxo`、`wes_getUTXO`、`/api/v1/utxos/{address}`
- 2）确认是哪一笔交易消费了旧状态、生成了新状态：`wes tx history` + `wes tx get`
- 3）如果还是不清楚，回到 [`components/eutxo.md`](../components/eutxo.md) 重新梳理状态流转模型

### Q：微迅链合约可以调用外部系统（HTTP、API 等）吗？和传统区块链有什么区别？

A：**可以！** 微迅链支持在合约中直接调用外部系统，这是 ISPC 的核心创新之一。

**传统区块链的限制**：
- ❌ 无法直接调用外部系统（HTTP、API、数据库等）
- ❌ 必须通过"预言机"将外部数据喂入链上
- ❌ 所有节点重复执行，外部系统会被调用 N 次（N = 节点数），导致系统崩溃

**微迅链 ISPC 的突破**：
- ✅ **支持直接调用外部系统**：合约可以通过 HostABI 调用 HTTP、API、数据库等
- ✅ **受控外部交互机制**：通过"声明+佐证+验证"机制，外部调用被控制和见证
- ✅ **单次执行+多点验证**：只有执行节点调用一次外部系统，其他节点通过验证 ZK 证明来确认，无需重复调用
- ✅ **外部副作用只发生一次**：彻底解决传统区块链的重复调用问题

**ISPC 受控外部交互工作原理**：
1. **声明外部状态预期**：告诉系统"我要调用这个外部数据源"
2. **提供验证佐证**：提供 API 数字签名、响应哈希等密码学佐证
3. **运行时验证**：ISPC 运行时验证佐证的有效性
4. **生成 ZK 证明**：执行轨迹自动生成 ZK 证明（包含外部交互验证）
5. **验证节点验证证明**：其他节点验证证明，无需重复调用外部 API

**使用示例**（Go 合约）：
```go
import "github.com/weisyn/contract-sdk-go/helpers/external"

// 调用外部 API
result, err := external.CallAPI(
    "https://api.example.com/price",
    "GET",
    `{}`,
    &framework.Evidence{
        APISignature: "...",  // API 数字签名
        ResponseHash: "...",  // 响应哈希
    },
)
```

**学习资源**：
- [`components/ispc.md`](../components/ispc.md) - ISPC 受控外部交互机制详解
- [企业级流程场景](../tutorials/scenarios/enterprise-workflow.md) - 完整的外部系统集成示例
- [合约 SDK 文档](../../../contract-sdk-go/helpers/external/README.md) - 外部交互 API 使用指南

### Q：为什么我在本地链上测试一切正常，上测试网就各种失败？

A：这是**环境差异**导致的典型问题，常见原因：

**1. 版本不匹配**
- 本地节点版本与测试网版本不一致
- 合约编译工具链版本不同
- **排查**：`wes node version` 对比测试网版本；检查合约编译器版本

**2. 网络配置差异**
- 本地单节点 vs 测试网多节点环境
- 本地可能没有启用某些验证规则
- **排查**：检查配置文件中的 `network_id`、`chain_id`、`consensus` 等参数

**3. 数据状态差异**
- 本地是全新链，测试网已有历史数据
- UTXO 状态、资源状态不同
- **排查**：`wes chain status` 对比链高度和状态根；`wes account utxo` 对比 UTXO 集合

**4. 兼容性问题**
- 本地测试时使用了未正式发布的 API
- 测试网可能已升级协议版本
- **排查**：参考 [兼容性说明](../standards/compatibility.md) 检查 API 版本

**快速诊断命令**：
```bash
# 对比版本
wes node version
wes chain status

# 对比配置
wes node config show | grep -E "network_id|chain_id|consensus"

# 对比状态
wes chain height
wes account utxo <your-address>
```

### Q：为什么费用几乎为 0，但矿工还能有激励？

A：这是对 WES **"费用即激励"模型**的常见误解：

**误解**：用户不付 Gas = 矿工没收入 = 网络无法运行

**事实**：
- **微迅链没有 Gas 概念**：使用 CU（Compute Units，计算单位）作为内部算力计量，用户无需面对
- **用户支付的网络手续费**（虽然很少）会聚合为矿工激励
- **业务方通过赞助池注入合约代币**，主动奖励参与该业务计算的矿工
- **ISPC 成本优化**：单次执行 + 多点验证，实际运行成本极低，使得手续费可以保持在极低水平

**关键理解**：
- 赞助池 ≠ "代付 Gas"（因为微迅链本身就没有 Gas 概念）
- 赞助池 = 业务方主动激励矿工，确保业务在网络高负载下依然有足够激励

详细说明请参考 [费用与经济模型](../product/economics.md)。

### Q：从 Bitcoin 迁移过来，UTXO 模型有什么不同？

A：微迅链的 **EUTXO（Extended UTXO）** 相比 Bitcoin 的 UTXO 有重大扩展：

**Bitcoin UTXO**：
- 只有一种输出类型：价值输出（BTC）
- 只能消费，不能引用
- 状态表达受限

**微迅链 EUTXO**：
- **三层输出**：Asset（价值）、Resource（能力）、State（证据）
- **引用型输入**：可以引用资源而不消费（例如引用合约代码）
- **状态表达丰富**：可以表达复杂业务状态

**迁移建议**：
- 不要用 Bitcoin 的"找零"思维理解微迅链的状态变更
- 理解"消费型输入"和"引用型输入"的区别
- 参考 [`components/eutxo.md`](../components/eutxo.md) 中的三层输出架构

---

## 🚨 开发者典型失败场景

### Q：交易提交后长时间 pending / 不确认，怎么办？

A：这是最常见的开发问题，按以下顺序排查：

**步骤 1：检查交易状态**
```bash
# 查看交易详情
wes tx get <tx-hash>

# 查看交易状态
wes tx status <tx-hash>

# 查看交易池状态
wes txpool status
```

**步骤 2：检查节点同步状态**
```bash
# 确认节点已同步
wes chain status | grep syncing

# 如果未同步，交易可能无法被打包
# 参考 [运维问题排查](./operations.md#问题-3区块同步失败)
```

**步骤 3：检查交易格式和验证**
```bash
# 验证交易格式
wes tx validate <tx-hash>

# 检查常见错误：
# - invalid signature（签名错误）
# - insufficient balance（余额不足）
# - invalid nonce（Nonce 错误）
```

**步骤 4：检查交易池拥堵**
```bash
# 如果交易池已满，可能需要等待或提高手续费
wes txpool status

# 查看待处理交易数
wes txpool pending
```

**详细排查流程**：参考 [`operations.md`](./operations.md#问题-4交易提交失败) 中的完整排查步骤。

### Q：从 EVM 合约迁移过来，总是逻辑不对 / 状态不对，怎么办？

A：这是 **EUTXO 模型 vs 账户模型** 的典型问题：

**问题根源**：
- EVM 合约：状态是"合约账户下的一坨变量"，可以随时读写
- 微迅链合约：状态是"UTXO 集合"，必须通过交易消费/创建 UTXO 来变更

**注意**：微迅链支持在合约中调用外部系统（HTTP、API等），这是 ISPC 的核心能力。如果遇到外部系统调用问题，参考上面的"微迅链合约可以调用外部系统吗？"问题。

**排查步骤**：

**1. 理解状态流转**
- EVM：`contract.storage[key] = value`（直接修改）
- WES：`消费旧 UTXO → 执行逻辑 → 创建新 UTXO`（状态变更）

**2. 检查 UTXO 状态**
```bash
# 查看地址的所有 UTXO
wes account utxo <address>

# 查看特定类型的 UTXO（Asset/Resource/State）
wes account utxo <address> --type asset
wes account utxo <address> --type state
```

**3. 追踪状态变更历史**
```bash
# 查看交易历史
wes account history <address>

# 查看每笔交易的输入输出
wes tx get <tx-hash> | grep -A 10 "inputs\|outputs"
```

**4. 重新理解合约逻辑**
- 确认合约是否正确消费了旧 UTXO
- 确认合约是否正确创建了新 UTXO
- 确认 UTXO 的类型（Asset/Resource/State）是否正确

**学习资源**：
- [`components/eutxo.md`](../components/eutxo.md) - EUTXO 模型详解
- [`components/tx.md`](../components/tx.md) - 交易如何消费/创建 UTXO
- [`tutorials/contracts/beginner.md`](../tutorials/contracts/beginner.md) - 合约开发入门

### Q：AI 模型部署 / 推理失败，怎么排查？

A：AI 模型与智能合约采用统一的经济模型和部署机制。AI 模型相关问题的排查顺序：

**步骤 1：检查模型格式**
```bash
# 确认模型是 ONNX 格式
file model.onnx

# 检查模型大小（是否超限）
ls -lh model.onnx
```

**步骤 2：检查资源部署**
```bash
# 查看资源部署状态
wes resource get <resource-hash>

# 检查资源是否已上链
wes resource status <resource-hash>
```

**步骤 3：检查推理调用**
```bash
# 查看推理交易详情
wes tx get <inference-tx-hash>

# 检查推理结果和证明
wes tx get <inference-tx-hash> | grep -A 20 "proof\|result"
```

**步骤 4：检查 ISPC 执行日志**
```bash
# 查看 ISPC 相关日志
tail -f ./logs/wes.log | grep -i "ispc\|onnx\|inference"

# 查看错误日志
grep -i "error.*onnx\|error.*inference" ./logs/error.log
```

**常见错误**：
- `invalid onnx format` - 模型格式错误
- `model too large` - 模型大小超限
- `inference timeout` - 推理超时
- `proof generation failed` - 证明生成失败

**学习资源**：
- [AI 推理场景](../tutorials/scenarios/ai-inference.md) - AI 模型部署和推理完整流程
- [`components/ures.md`](../components/ures.md) - 资源管理能力
- [`components/ispc.md`](../components/ispc.md) - 可验证计算引擎

### Q：合约调用返回错误，但错误信息不明确，怎么排查？

A：合约调用错误的排查方法：

**步骤 1：检查调用参数**
```bash
# 查看调用交易的详情
wes tx get <tx-hash>

# 确认参数格式正确
wes tx get <tx-hash> | grep -A 10 "params\|inputs"
```

**步骤 2：检查合约状态**
```bash
# 查看合约资源
wes resource get <contract-hash>

# 检查合约是否已部署
wes resource status <contract-hash>
```

**步骤 3：检查执行日志**
```bash
# 查看合约执行日志
tail -f ./logs/wes.log | grep -i "contract\|wasm"

# 查看错误详情
grep -i "error.*contract\|error.*wasm" ./logs/error.log | tail -20
```

**步骤 4：本地调试**
- 使用本地节点单步调试
- 检查合约代码逻辑
- 参考 [合约开发故障排查](../tutorials/contracts/troubleshooting.md)

**常见错误类型**：
- `method not found` - 方法不存在或未导出
- `invalid parameters` - 参数类型或格式错误
- `insufficient permissions` - 权限不足
- `execution failed` - 执行逻辑错误

---

## ⚡ 快速诊断场景（5 分钟定位问题）

### Q：新同事接手一个挂了的节点，5 分钟内能做什么？

A：**快速诊断清单**（按优先级排序）：

**1 分钟：检查节点是否运行**
```bash
# 检查进程
ps aux | grep wes

# 检查端口
netstat -tulpn | grep -E "8080|8081"

# 检查健康状态
curl http://localhost:8080/api/v1/health/liveness
```

**2 分钟：检查基本状态**
```bash
# 节点状态
wes node status

# 链状态
wes chain status

# 同步状态
wes chain status | grep syncing
```

**3 分钟：查看最近错误**
```bash
# 查看最近的错误日志
tail -50 ./logs/error.log

# 查看最近的警告
grep -i "warn\|error" ./logs/wes.log | tail -20

# 查看系统资源
free -h && df -h
```

**5 分钟：定位问题类型**
- 如果节点未运行 → 参考 [`operations.md`](./operations.md#问题-1节点无法启动)
- 如果节点运行但未同步 → 参考 [`operations.md`](./operations.md#问题-3区块同步失败)
- 如果节点运行但无连接 → 参考 [`operations.md`](./operations.md#问题-2节点无法连接到网络)
- 如果资源不足 → 参考 [`performance.md`](./performance.md)

### Q：线上节点 CPU 飙高但 TPS 不高，怎么分辨是合约、网络还是存储的问题？

A：**分层排查法**：

**步骤 1：定位 CPU 消耗来源**
```bash
# 查看进程 CPU 使用
top -p $(pgrep wes)

# 查看线程 CPU 使用
top -H -p $(pgrep wes)

# 查看各组件 CPU 使用（如果支持）
wes node metrics | grep -i cpu
```

**步骤 2：检查合约执行**
```bash
# 查看合约执行相关日志
tail -f ./logs/wes.log | grep -i "contract\|wasm\|ispc"

# 检查是否有大量合约调用
wes txpool status | grep pending

# 检查合约执行时间
wes node metrics | grep -i "execution\|contract"
```

**步骤 3：检查网络层**
```bash
# 检查网络连接数
wes node peers | wc -l

# 检查网络 I/O
netstat -an | grep ESTABLISHED | wc -l

# 检查网络延迟
wes node status | grep -i latency
```

**步骤 4：检查存储层**
```bash
# 检查磁盘 I/O
iostat -x 1 5

# 检查存储使用
df -h

# 检查数据库操作
wes node metrics | grep -i "storage\|database"
```

**判断标准**：
- **CPU 高 + 大量合约调用** → 合约执行问题（优化合约或增加资源）
- **CPU 高 + 网络连接数多** → 网络层问题（检查 P2P 连接）
- **CPU 高 + 磁盘 I/O 高** → 存储层问题（检查存储性能）
- **CPU 高 + TPS 低** → 可能是共识延迟或同步问题

**详细排查**：参考 [`performance.md`](./performance.md) 和 [`operations.md`](./operations.md#问题-5节点频繁崩溃)。

---

## 💰 费用问题

### Q：使用微迅链需要支付 Gas 费吗？

A：**微迅链没有 Gas 概念**。普通用户不会像在传统公链上一样手动设置 Gas，只会在**转账**等少数操作中看到简单的网络手续费；具体数值和策略由链上经济模型决定。

**说明**：
- 微迅链使用 **CU（Compute Units，计算单位）** 作为内部算力计量单位
- CU 用于系统内部资源计量（合约和 AI 模型），用户无需理解
- 用户通常只在转账时看到简单的网络手续费

关于费用与激励机制，请参考 [产品总览](../overview.md) 和 [费用与经济模型](../product/economics.md)。

### Q：WES 的经济模型是什么？

A：WES 采用"费用即激励"的零增发模型，交易手续费聚合为矿工激励。更深入的说明请参考 [费用与经济模型](../product/economics.md)。

### Q：为什么我的交易手续费和预期不一致？

A：WES 的手续费计算规则：

**影响因素**：
- **交易大小**：交易数据越大，手续费越高
- **网络拥堵程度**：网络繁忙时，手续费可能略有上升
- **交易类型**：转账类操作通常有固定手续费，合约调用可能由业务方承担

**排查方法**：
```bash
# 查看交易详情（包含手续费）
wes tx get <tx-hash> | grep -i fee

# 查看当前网络手续费率
wes chain status | grep -i fee

# 估算交易手续费
wes tx estimate <tx-data>
```

**如果手续费异常高**：
- 检查交易大小是否过大
- 检查网络是否拥堵
- 参考 [`performance.md`](./performance.md) 排查性能问题

---

## 🔧 技术问题

### Q：WES 支持哪些编程语言？

A：WES 支持多种编程语言开发智能合约：
- Go（推荐）
- Rust
- JavaScript/TypeScript
- Python（通过 TinyGo）

### Q：WES 支持 AI 模型吗？

A：是的。WES 支持 ONNX 格式的 AI 模型，可以在链上执行 AI 推理。详细说明请参考 [AI 推理场景](../tutorials/scenarios/ai-inference.md)。

### Q：WES 的性能如何？

A：宏观性能能力（TPS、验证效率、成本等）请参考 [产品总览](../overview.md) 和 [`product/positioning.md`](../product/positioning.md)。如果是具体的性能异常或瓶颈，请结合本章内容和 [`performance.md`](./performance.md) 进行排查。

### Q：如何调试合约执行？

A：合约调试方法：

**1. 本地单节点调试**
```bash
# 启动本地单节点
wes node start --config local

# 部署合约到本地节点
wes contract deploy --file contract.wasm

# 调用合约并查看日志
wes contract call <contract-hash> <method> <params>
tail -f ./logs/wes.log | grep -i contract
```

**2. 使用调试模式**
```bash
# 启用调试日志
wes node start --log-level debug

# 查看详细执行日志
tail -f ./logs/wes.log | grep -i "debug\|contract"
```

**3. 检查执行结果**
```bash
# 查看交易执行结果
wes tx get <tx-hash> | grep -A 20 "result\|outputs"

# 查看执行错误
wes tx get <tx-hash> | grep -i error
```

**学习资源**：
- [合约开发入门](../tutorials/contracts/beginner.md)
- [合约开发故障排查](../tutorials/contracts/troubleshooting.md)

---

## 🚀 部署问题

### Q：如何部署 WES 节点？

A：WES 支持多种部署方式：
- 本地单节点部署
- Docker 容器部署
- 云环境部署

详细步骤请参考 [部署指南](../tutorials/deployment/)。

### Q：WES 的系统要求是什么？

A：**生产环境**：
- 操作系统：Linux（推荐 Ubuntu 22.04+）
- 内存：至少 8GB RAM（推荐 16GB+）
- 存储：至少 100GB 可用空间（推荐 SSD）

**测试环境**：
- 操作系统：Linux/macOS/Windows
- 内存：至少 4GB RAM
- 存储：至少 10GB 可用空间

### Q：Docker 部署后无法访问节点，怎么办？

A：Docker 部署常见问题：

**1. 端口映射问题**
```bash
# 检查容器端口映射
docker ps | grep wes

# 确认端口映射正确
# 应该是：-p 8080:8080 -p 8081:8081
```

**2. 防火墙问题**
```bash
# 检查防火墙规则
sudo ufw status

# 开放端口
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
```

**3. 容器网络问题**
```bash
# 检查容器网络
docker network ls

# 检查容器日志
docker logs <container-id>
```

**详细排查**：参考 [`operations.md`](./operations.md#问题-2节点无法连接到网络)。

---

## 📚 相关文档

- [产品总览](../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [快速开始](../tutorials/quickstart/) - 快速上手 WES
- [运维问题](./operations.md) - 运维问题排查
- [性能问题](./performance.md) - 性能问题排查
- [合约开发故障排查](../tutorials/contracts/troubleshooting.md) - 合约开发相关问题
