# HTTP 类型定义（internal/api/http/types）

> **📌 模块类型**：`[ ] 实现模块` `[ ] 接口定义` `[X] 数据结构` `[ ] 工具/其他`

---

## 📍 **模块定位**

　　本模块定义 HTTP REST API 的**响应类型、错误码和分页结构**，为 REST 接口提供统一的数据格式和错误处理机制。

**解决什么问题**：
- **错误标准化**：区块链特有错误码体系
- **响应统一**：标准的 JSON 响应格式
- **分页支持**：大数据集的分页查询

**不解决什么问题**（边界）：
- ❌ 不处理业务逻辑（由 handlers 负责）
- ❌ 不实现序列化（由 Gin 负责）

---

## 🎯 **核心约束**

**严格遵守**：
- ✅ **HTTP 状态码标准**：正确使用 2xx/4xx/5xx
- ✅ **错误码唯一**：每种错误有独立错误码
- ✅ **响应格式统一**：所有成功响应包含 `data` 字段

**严格禁止**：
- ❌ **混淆状态码**：不得 200 返回错误信息
- ❌ **错误码重复**：不得多种错误使用同一错误码

---

## 📦 **类型体系**

### **类型全景**

```mermaid
classDiagram
    class ErrorResponse {
        +ErrorCode Code
        +string Message
        +interface{} Details
    }
    
    class SuccessResponse {
        +interface{} Data
        +*Pagination Meta
    }
    
    class Pagination {
        +int Page
        +int PageSize
        +int Total
        +int TotalPages
    }
    
    SuccessResponse --> Pagination : contains
```

### **错误码定义**

| 错误码 | HTTP状态 | 含义 | 使用场景 |
|-------|---------|------|---------|
| `INVALID_PARAMETER` | 400 | 参数错误 | 参数缺失/格式错误 |
| `NOT_FOUND` | 404 | 资源不存在 | 区块/交易不存在 |
| `UNAUTHORIZED` | 401 | 未授权 | 签名验证失败 |
| `INTERNAL_ERROR` | 500 | 内部错误 | 服务异常 |
| `SERVICE_UNAVAILABLE` | 503 | 服务不可用 | 同步中/维护中 |

---

## 📁 **目录结构**

```
types/
├── error.go            # ✅ 错误码定义
├── response.go         # ✅ 响应结构
├── pagination.go       # ✅ 分页类型
└── README.md           # 本文档
```

---

## 📊 **核心机制**

### **机制1：区块链错误码体系**

**实现示例**：
```go
// 错误码枚举
type ErrorCode string

const (
    ErrInvalidParameter    ErrorCode = "INVALID_PARAMETER"
    ErrNotFound            ErrorCode = "NOT_FOUND"
    ErrInvalidSignature    ErrorCode = "INVALID_SIGNATURE"
    ErrInvalidBlockParam   ErrorCode = "INVALID_BLOCK_PARAM"
)

// 错误响应
type ErrorResponse struct {
    Code    ErrorCode   `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}
```

---

## 🎓 **使用指南**

### **典型场景：返回错误**

```go
func (h *Handler) GetBlock(c *gin.Context) {
    height := c.Param("height")
    if height == "" {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Code:    ErrInvalidParameter,
            Message: "height is required",
        })
        return
    }
    
    block, err := h.repo.GetBlockByHeight(ctx, height)
    if err != nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Code:    ErrNotFound,
            Message: "block not found",
            Details: map[string]interface{}{"height": height},
        })
        return
    }
    
    c.JSON(http.StatusOK, SuccessResponse{Data: block})
}
```

---

## 📚 **相关文档**

- **Handlers**：[../handlers/README.md](../handlers/README.md) - 使用这些类型

---

## 📋 **文档变更记录**

| 日期 | 变更内容 | 原因 |
|------|---------|------|
| 2025-10-24 | 创建本文档 | 补全子目录 README，符合模板 v3.0 |

---

> 📝 **文档说明**  
> 本文档遵循 `_docs/templates/README_TEMPLATE.md` v3.0 规范

