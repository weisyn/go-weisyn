# hello-world - Hello World 合约示例

---

## 📌 版本信息

- **版本**：1.0
- **状态**：stable
- **最后更新**：2025-11-15
- **最后审核**：2025-11-15
- **所有者**：合约平台组
- **适用范围**：WES 项目中 hello-world 合约示例

---

## 📍 组件定位

最简单的合约示例，用于验证 WES 平台的基本合约执行能力。该合约只包含一个 `SayHello` 函数，返回固定的字符串。

**用途**：
- ✅ 验证合约编译和部署流程
- ✅ 验证合约调用机制
- ✅ 作为最基础的回归测试用例

---

## 📁 文件说明

- **main.go**: 合约源码
- **go.mod**: Go 模块定义
- **build.sh**: 编译脚本（将 Go 编译为 WASM）
- **testcases/default.json**: 标准测试用例

---

## 🔨 编译合约

```bash
cd hello-world
./build.sh
```

编译成功后会在当前目录生成 `hello-world.wasm` 文件。

---

## 🧪 测试用例

测试用例定义在 `testcases/default.json` 中，包含：

- **SayHello 调用测试**：验证合约可以正常调用并返回预期结果

---

## 📚 相关文档

- [合约资源级示例库 README](../README.md)
- [合约平台总览](../../README.md)

---
