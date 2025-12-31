# Simple Token - 资源级合约示例

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-15
- **最后审核**：2025-11-15
- **所有者**：合约平台组
- **适用范围**：WES 资源级合约示例（可执行合约 + 测试用例）

---

## 🎯 示例定位

**路径**：`contracts/examples/simple-token/`

**目的**：提供一个**固定行为、可直接运行**的代币合约资源级示例，用于功能验证与回归测试。

**与模板的区别**：
- `contracts/templates/learning/simple-token/`：**学习模板**，侧重教学，代码中有大量注释，可复制后修改
- `contracts/examples/simple-token/`：**资源级示例**，行为固定，测试完备，用于验证平台能力

---

## 📐 目录结构

```
simple-token/
├── README.md                    # 本文档
├── src/                         # 合约源码
│   └── main.go
├── go.mod                       # 合约模块定义
└── testcases/                   # 资源级测试用例
    └── default.json             # 标准测试场景
```

---

## 🔧 合约功能

### 导出函数

| 函数名 | 功能 | 参数 | 返回值 |
|--------|------|------|--------|
| `GetContractInfo` | 获取合约基本信息 | 无 | JSON（name, symbol, decimals 等） |
| `GetBalance` | 查询地址余额 | `{"address": "..."}` | JSON（balance, token_info） |
| `GetTotalSupply` | 查询总发行量 | 无 | JSON（total_supply） |
| `Transfer` | 转账代币 | `{"to": "...", "amount": "100"}` | 状态码 + 事件 |

### 代币信息

- **名称**：Simple Token
- **符号**：STK
- **小数位**：18
- **初始发行量**：1,000,000

---

## 🧪 测试规范

### 测试用例

本示例包含标准测试用例：`testcases/default.json`

**用例结构**：
- 部署合约
- 查询合约信息
- 查询初始余额
- 执行转账并验证

**测试环境**：
- 节点环境：`testing`（单节点共识）
- JSON-RPC 端点：`http://127.0.0.1:28680/jsonrpc`

### 运行测试

```bash
# 1. 编译合约
cd contracts/examples/simple-token
tinygo build -o simple-token.wasm -target=wasi -scheduler=none -no-debug -opt=2 ./src

# 2. 运行测试（需要测试节点运行中）
# 测试脚本会读取 testcases/default.json 并自动执行
```

---

## 📚 相关文档

- `contracts/examples/README.md`：资源级合约示例库总览
- `contracts/templates/learning/simple-token/README.md`：学习模板说明
- `models/examples/basic/sklearn_randomforest/`：模型资源级示例（结构对齐）

---

## 🔗 与其他资源的关系

- **资源级示例**：本示例是合约资源级示例，与 `models/examples/` 中的模型资源级示例结构一致
- **场景级示例**：根 `examples/` 中的场景级示例可以组合使用本合约与其他资源

