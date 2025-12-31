# ABI 一致性测试工具

**版本**: 1.0  
**状态**: ✅ 可用  
**最后更新**: 2025-11-24

---

## 📋 概述

跨仓库 ABI 一致性测试工具，用于验证所有 SDK 的 payload 构建和 Draft JSON 生成是否符合 WES ABI 规范。

**规范来源**：`docs/components/core/ispc/abi-and-payload.md`

---

## 🎯 功能

1. **Payload 一致性检查**：
   - 验证 payload JSON 是否只包含允许的保留字段和扩展字段
   - 检查字段名是否符合规范（下划线命名）
   - 验证 Base64 编码是否正确

2. **Draft JSON 一致性检查**：
   - 验证 Draft JSON 字段名是否符合规范
   - 检查 State Output 字段名（`state_version`, `execution_result_hash`）
   - 验证字段类型和格式

3. **跨语言一致性检查**：
   - 对比 Go/TS TransactionBuilder 生成的 Draft JSON
   - 确保字段名和结构一致（允许字段顺序不同）

---

## 📦 使用方法

### 手动运行

```bash
cd tools/abi-conformance

# 基本检查
go run main.go

# 扫描 SDK fixtures（可选）
go run main.go --scan-fixtures
```

### 编译为可执行文件

```bash
cd tools/abi-conformance
go build -o abi-conformance main.go
./abi-conformance
```

### 集成到 CI

```yaml
# .github/workflows/abi-conformance.yml
name: ABI Conformance Check
on: [push, pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run ABI Conformance
        run: |
          cd tools/abi-conformance
          go run main.go
```

---

## 🔧 配置

工具会自动扫描以下目录：

- `sdk/client-sdk-go.git/tests/fixtures/` - Go Client SDK 测试用例
- `sdk/client-sdk-js.git/tests/fixtures/` - JS Client SDK 测试用例
- `sdk/contract-sdk-go.git/tests/fixtures/` - Go Contract SDK 测试用例
- `sdk/contract-sdk-js.git/tests/fixtures/` - JS Contract SDK 测试用例

---

## 📝 测试用例格式

测试用例应为 JSON 文件，格式如下：

```json
{
  "name": "transfer_payload",
  "type": "payload",
  "input": {
    "from": "0x1234...",
    "to": "0xabcd...",
    "amount": "1000000"
  },
  "expected": {
    "from": "0x1234...",
    "to": "0xabcd...",
    "amount": "1000000",
    "token_id": "0x0000..."
  }
}
```

---

## ✅ 检查项

1. ✅ Payload 字段名符合规范（`token_id` 而非 `tokenID`）
2. ✅ Draft JSON 字段名符合规范（`state_version`, `execution_result_hash`）
3. ✅ 扩展字段不与保留字段冲突
4. ✅ Base64 编码正确
5. ✅ 跨语言 Draft JSON 字段一致

---

## 🔗 相关文档

- [WES ABI & Payload 规范](../../docs/components/core/ispc/abi-and-payload.md)
- [Client SDK Go ABI Helper](../../sdk/client-sdk-go.git/utils/abi.go)
- [Client SDK JS ABI Helper](../../sdk/client-sdk-js.git/src/utils/abi.ts)

