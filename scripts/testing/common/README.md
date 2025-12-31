# 公共测试工具

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-13
- **最后审核**：2025-11-13
- **所有者**：测试团队
- **适用范围**：所有测试脚本的公共工具和初始化逻辑

---

## 🎯 目录定位

**路径**：`scripts/testing/common/`

**核心职责**：提供统一的测试环境初始化和管理工具，包括：
- 测试数据清理策略（避免测试污染）
- 单节点 / 网络共识模式开关
- 日志与数据目录的统一归集

**关键文件**：
- `test_init.sh` - 统一测试环境初始化（⭐核心）
- `verify_scripts.sh` - 测试脚本验证工具

---

## 🔧 核心工具

### `test_init.sh` - 统一测试环境初始化

**用途**：根据 `configs/testing/config.json` 中的 `test` 配置统一初始化测试环境

**功能**：
- ✅ 读取测试配置（`cleanup_on_start`, `keep_recent_logs`, `cleanup_wrong_locations`, `single_node_mode`）
- ✅ 停止所有相关节点进程
- ✅ 根据配置清理测试数据（可选）
- ✅ 管理测试日志（保留最近N个）
- ✅ 清理错误位置的数据目录
- ✅ 预留统一入口扩展网络检查（如 peers 数量、aggregator 状态）

**使用方法**：
```bash
#!/usr/bin/env bash

# 加载统一的测试初始化脚本
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../common/test_init.sh"

# 初始化测试环境（会根据配置自动处理）
init_test_environment
```

**环境变量**（初始化后设置）：
- `TEST_CLEANUP_ON_START` - 是否清理数据
- `TEST_KEEP_RECENT_LOGS` - 保留日志数量
- `TEST_CLEANUP_WRONG_LOCATIONS` - 是否清理错误位置
- `TEST_SINGLE_NODE_MODE` - 是否单节点模式（true=本地自挖，false=依赖外部网络共识）

---

### `verify_scripts.sh` - 测试脚本验证工具

**用途**：验证所有测试脚本的语法和基本功能

**验证内容**：
- ✅ 语法检查（使用 `bash -n`）
- ✅ 权限检查（确保脚本可执行）
- ✅ 依赖检查（检查 Go 环境和必要命令）
- ✅ 端口检查（检查常用端口是否被占用）

**使用方法**：
```bash
bash scripts/testing/common/verify_scripts.sh
```

---

## 📋 配置驱动架构

```
configs/testing/config.json
    ↓
    test.cleanup_on_start
    ↓
    ├── cmd/weisyn (节点启动时读取)
    └── scripts/testing/common/test_init.sh (测试脚本初始化时读取)
```

**原则**：
- ✅ 配置驱动，而非代码驱动
- ✅ 所有测试脚本都可以读取
- ✅ 易于维护和修改

---

## 🔗 相关文档

- [测试脚本总入口](../README.md) - 测试脚本目录总览
- [测试环境配置](../../../configs/testing/config.json) - 测试环境配置文件
- [文档规范](../../../docs/system/standards/principles/documentation.md) - 文档编写规范

---

## 📝 变更历史

| 版本 | 日期 | 变更内容 | 作者 |
|-----|------|---------|------|
| 1.0 | 2025-11-13 | 初始版本，统一测试初始化 | 测试团队 |

---

