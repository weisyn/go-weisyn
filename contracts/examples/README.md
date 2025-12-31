# WES 合约资源级示例库（contracts/examples）

---

## 📌 版本信息

- **版本**：1.0
- **状态**：draft
- **最后更新**：2025-11-15
- **最后审核**：2025-11-15
- **所有者**：合约平台组
- **适用范围**：WES 项目中「资源级」合约示例（可执行合约 + 测试用例）

---

## 🎯 目录定位

**路径**：`contracts/examples/`

**目的**：存放经过固定化的、可直接运行的 **合约可执行资源（Executable Contract Resource）**，**专门用于平台功能验证与回归测试**。

**⚠️ 重要说明**：
- ❌ **不用于开发**：本目录的示例行为固定，不鼓励修改
- ✅ **用于测试验证**：证明测试合约跑通，验证平台能力
- ✅ **固定行为**：每个示例有明确的业务语义和行为定义
- ✅ **完整测试**：提供可重复执行的测试用例（`testcases/default.json`）

**与其他目录的关系**：

- **合约 SDK 模块**（**@go.mod `github.com/weisyn/contract-sdk-go`**）：提供「如何构建合约」的开发模板（`templates/learning/` 和 `templates/standard/`），开发者通过 `go get` 即可获得
- **`contracts/examples/`**：**资源级示例**，提供「合约如何表现」的固定示例（用于测试验证）
- **`models/examples/`**：模型的资源级示例库（结构与本目录对齐）
- **`examples/`**（仓库根）：场景级示例库，组合使用模型、合约等多种资源

---

## 📐 目录结构

```text
contracts/examples/
├── README.md                    # 本文档（目录总览和规范）
├── basic/                       # 基础合约示例（基本功能测试）
│   ├── README.md               # 基础示例说明
│   ├── hello-world/            # Hello World 示例
│   │   ├── README.md
│   │   ├── main.go
│   │   ├── go.mod
│   │   ├── build.sh            # 编译脚本
│   │   └── testcases/
│   │       └── default.json
│   └── simple-token/           # Simple Token 示例
│       ├── README.md
│       ├── src/main.go
│       ├── go.mod
│       ├── build.sh            # 编译脚本
│       └── testcases/
│           └── default.json
└── edge_cases/                  # 边缘情况示例（边界条件测试）
    ├── README.md
    └── （待补充）
```

### 当前示例

| 分类 | 示例名称 | 路径 | 功能描述 | 状态 |
|------|---------|------|----------|------|
| **basic** | hello-world | `basic/hello-world/` | 最简单的合约示例（SayHello） | ✅ 已就绪 |
| **basic** | simple-token | `basic/simple-token/` | 基础代币合约示例（转账、余额查询、总量查询） | ✅ 已就绪 |
| **edge_cases** | （待补充） | - | - | 📝 规划中 |

**约束要求**：

- 每个示例目录必须包含：
  - `README.md`：说明业务背景、导出函数、输入参数、预期行为
  - `main.go` 或 `src/main.go`：合约源码，使用 `github.com/weisyn/contract-sdk-go` 作为 SDK 依赖
  - `go.mod`：模块独立，便于编译和测试
  - `build.sh`：编译脚本（将 Go 编译为 WASM），必须可执行
  - `testcases/default.json`：标准测试场景（与 `models/examples/*/testcases/default.json` 风格一致）
- 测试工具应支持：
  - 读取 `testcases/*.json`
  - 部署对应合约
  - 按用例描述自动调用并核对状态/事件

### 🔨 编译示例

每个示例都提供了 `build.sh` 脚本，用于将 Go 合约编译为 WASM：

```bash
# 编译单个示例
cd basic/hello-world
./build.sh

# 编译所有基础示例
cd basic
for dir in */; do
    if [ -f "$dir/build.sh" ]; then
        echo "编译 $dir"
        cd "$dir"
        ./build.sh
        cd ..
    fi
done
```

---

## 🧪 资源级 vs 场景级

- **资源级示例（本目录、`models/examples/`）**：
  - 聚焦单个合约或单个模型
  - 定义其「输入 → 输出」和「状态/事件变化」的规范
  - 用于单组件的功能验证与回归测试

- **场景级示例（`examples/` 根目录）**：
  - 聚焦业务场景（例如「AI+DeFi」、「RWA+治理」）
  - 组合使用多个可执行资源（模型 + 合约 + 客户端）
  - 用于端到端流程演示与性能/集成测试

---

## 📚 相关文档

- `contracts/README.md`：合约平台总览
- **合约 SDK 模块**（**@go.mod `github.com/weisyn/contract-sdk-go`**）：SDK 模板库说明（开发模板）
- `models/README.md`：模型资源与示例库总览
- `examples/README.md`：场景级应用示例库

---

> **备注**：本目录目前作为规范入口，具体示例会在后续迭代中逐步补充。所有新增示例都应遵守本文档的结构与测试约束。

