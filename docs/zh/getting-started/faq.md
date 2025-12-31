# 常见问题

---

## 概述

本文档收集了 WES 入门过程中的常见问题和解答。

---

## 基础问题

### Q: WES 是什么？

A: WES（微迅链）是第三代区块链，通过 ISPC（本征自证计算）可验证计算范式，突破传统区块链的确定性共识限制，支持 AI 推理和企业级应用在链上运行。

详细了解：[WES 是什么](../concepts/what-is-wes.md)

### Q: WES 和以太坊有什么区别？

A: 主要区别：

| 特性 | WES | 以太坊 |
|------|-----|--------|
| 计算模式 | ISPC（单次执行+多点验证） | 所有节点重复执行 |
| AI 支持 | 原生支持链上 AI 推理 | 不支持 |
| 状态模型 | EUTXO（三层输出） | 账户模型 |
| 外部交互 | 支持可验证的外部调用 | 需要预言机 |

### Q: WES 支持哪些编程语言？

A: WES 智能合约支持：
- Rust
- Go
- JavaScript/TypeScript
- Python（实验性）

合约会编译成 WASM 格式在链上执行。

---

## 安装问题

### Q: 支持哪些操作系统？

A: WES 支持：
- Linux（Ubuntu 20.04+、CentOS 8+）
- macOS 12+
- Windows 10+（推荐使用 WSL2）

### Q: 需要什么硬件配置？

A: 最低要求：
- CPU：4 核
- 内存：8 GB
- 磁盘：100 GB SSD

推荐配置：
- CPU：8 核+
- 内存：16 GB+
- 磁盘：500 GB+ SSD

详细说明：[安装指南](./installation.md)

### Q: 编译时报错怎么办？

A: 常见解决方案：
1. 确保 Go 版本 >= 1.21
2. 运行 `go mod tidy` 更新依赖
3. 检查 CGO 环境配置
4. 查看详细错误信息 `go build -v`

---

## 运行问题

### Q: 节点无法启动

A: 检查清单：
1. 端口是否被占用：`lsof -i :30303`
2. 数据目录权限是否正确
3. 配置文件格式是否正确
4. 查看日志文件获取详细错误

### Q: 节点无法连接到网络

A: 检查清单：
1. 防火墙是否开放 30303 端口
2. 引导节点地址是否正确
3. 网络连接是否正常
4. 是否在 NAT 后面（尝试启用 UPnP）

### Q: 同步很慢怎么办？

A: 优化建议：
1. 使用 SSD 存储
2. 增加网络带宽
3. 使用快照同步
4. 连接更多节点

---

## 交易问题

### Q: 交易一直不确认

A: 可能原因：
1. 手续费过低
2. 节点未开启挖矿（开发模式）
3. 网络拥堵
4. 交易格式错误

解决方案：
- 检查交易状态
- 增加手续费重新提交
- 等待网络恢复

### Q: 余额不足错误

A: 检查：
1. 账户余额是否足够（包括手续费）
2. 是否有未确认的交易
3. 地址是否正确

### Q: 如何查询交易状态？

A: 使用 CLI 或 API：
```bash
# CLI
wes-node tx get --hash <tx_hash>

# API
curl http://localhost:8545/api/v1/tx/<tx_hash>
```

---

## 开发问题

### Q: 如何开发智能合约？

A: 步骤：
1. 安装合约 SDK
2. 编写合约代码
3. 编译为 WASM
4. 部署到链上

详细教程：[合约开发教程](../tutorials/contracts/)

### Q: 如何调用 API？

A: WES 提供 REST API 和 JSON-RPC：
```bash
# REST API
curl http://localhost:8545/api/v1/node/info

# JSON-RPC
curl -X POST http://localhost:8545/rpc \
  -H "Content-Type: application/json" \
  -d '{"method": "wes_nodeInfo", "params": []}'
```

详细说明：[API 参考](../reference/api/)

### Q: 如何使用 SDK？

A: WES 提供多语言 SDK：
- Go SDK：`go get github.com/weisyn/client-sdk-go`
- JavaScript SDK：`npm install @weisyn/client-sdk-js`

使用示例请参考各 SDK 的 README。

---

## 其他问题

### Q: 如何获取测试代币？

A: 
- 开发模式：首个账户自动有测试代币
- 测试网络：使用水龙头 https://faucet.testnet.weisyn.io

### Q: 如何参与社区？

A:
- GitHub：提交 Issue 和 PR
- Discord：加入社区讨论
- 文档：贡献文档改进

### Q: 如何报告 Bug？

A:
1. 在 GitHub 创建 Issue
2. 提供详细的复现步骤
3. 附上日志和环境信息
4. 描述期望行为和实际行为

---

## 更多帮助

如果以上内容没有解决你的问题：

1. 查阅 [完整文档](../README.md)
2. 搜索 GitHub Issues
3. 在社区提问
4. 联系技术支持

---

## 相关文档

- [安装指南](./installation.md)
- [本地快速开始](./quickstart-local.md)
- [第一笔交易](./first-transaction.md)
- [核心概念](../concepts/)

