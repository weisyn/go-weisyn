# ABI 验证包

**版本**: 1.0  
**状态**: ✅ 稳定  
**最后更新**: 2025-11-24

---

## 📋 概述

`internal/core/ispc/abi` 包提供 ABI 验证和辅助函数，用于确保 Payload 和 Draft JSON 符合 WES ABI 规范。

**规范来源**：`docs/components/core/ispc/abi-and-payload.md`

---

## 🎯 核心功能

### ValidatePayload

验证 Payload JSON 是否符合 WES ABI 规范：

```go
import "github.com/weisyn/v1/internal/core/ispc/abi"
import "github.com/weisyn/v1/pkg/types"

schema := types.GetDefaultABISchema()
payloadJSON := `{"from":"0x1234...","amount":"1000000"}`

err := abi.ValidatePayload(payloadJSON, schema)
if err != nil {
    // 处理验证错误
}
```

**检查项**：
- ✅ 保留字段类型是否正确
- ✅ 扩展字段名是否与保留字段冲突
- ✅ 字段值格式是否符合规范

### ValidateDraftJSON

验证 Draft JSON 是否符合 WES ABI 规范：

```go
draftJSON := `{
    "sign_mode": "defer_sign",
    "outputs": [{
        "type": "state",
        "state_id": "base64...",
        "state_version": 1,
        "execution_result_hash": "base64..."
    }]
}`

err := abi.ValidateDraftJSON(draftJSON, schema)
```

**检查项**：
- ✅ 必需字段是否存在（sign_mode, outputs）
- ✅ State Output 字段名是否正确（state_version, execution_result_hash）
- ✅ Asset Output 字段名是否正确（owner, amount, token_id）
- ✅ Intent 参数是否符合规范

### ConvertContractABIToSchema

将 ContractABI 转换为中立 Schema 表示：

```go
contractABI := &types.ContractABI{
    Version: "1.0",
    Functions: []types.ContractFunction{...},
}

schema, err := abi.ConvertContractABIToSchema(contractABI)
```

---

## 📝 使用示例

### 验证 Payload

```go
package main

import (
    "fmt"
    "github.com/weisyn/v1/internal/core/ispc/abi"
    "github.com/weisyn/v1/pkg/types"
)

func main() {
    schema := types.GetDefaultABISchema()
    
    // 正确的 payload
    payload1 := `{"from":"0x1234...","amount":"1000000"}`
    if err := abi.ValidatePayload(payload1, schema); err != nil {
        fmt.Printf("验证失败: %v\n", err)
    } else {
        fmt.Println("验证通过")
    }
    
    // 错误的 payload（字段冲突）
    payload2 := `{"from":"0x1234...","from_custom":"value"}`
    if err := abi.ValidatePayload(payload2, schema); err != nil {
        fmt.Printf("验证失败（预期）: %v\n", err)
    }
}
```

### 验证 Draft JSON

```go
draftJSON := `{
    "sign_mode": "defer_sign",
    "outputs": [{
        "type": "asset",
        "owner": "0x1234...",
        "amount": "1000",
        "token_id": "0x0000..."
    }]
}`

err := abi.ValidateDraftJSON(draftJSON, schema)
if err != nil {
    fmt.Printf("Draft JSON 验证失败: %v\n", err)
}
```

---

## 🧪 测试

运行测试：

```bash
cd internal/core/ispc/abi
go test -v
```

---

## 🔗 相关文档

- [WES ABI & Payload 规范](../../../../docs/components/core/ispc/abi-and-payload.md)
- [ABI Schema 定义](../../../../pkg/types/abi_schema.go)
- [ABI Conformance 工具](../../../../tools/abi-conformance/README.md)

