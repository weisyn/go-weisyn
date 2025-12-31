# 代码规范

---

## 概述

本文档定义了 WES 项目的代码风格和编码标准。

---

## 通用原则

### 可读性优先

- 代码应该自解释
- 复杂逻辑必须有注释
- 使用有意义的命名

### 简单性

- 优先选择简单的解决方案
- 避免过度工程
- 遵循 KISS 原则

### 一致性

- 遵循项目现有风格
- 使用标准工具格式化
- 统一的错误处理模式

---

## Go 代码规范

### 格式化

使用 `gofmt` 或 `goimports` 格式化代码：

```bash
gofmt -w .
# 或
goimports -w .
```

### 命名约定

| 类型 | 约定 | 示例 |
|------|------|------|
| 包名 | 小写，单词 | `tx`, `block`, `consensus` |
| 导出类型 | PascalCase | `Transaction`, `BlockHeader` |
| 非导出类型 | camelCase | `txPool`, `blockCache` |
| 常量 | PascalCase 或 ALL_CAPS | `MaxBlockSize`, `DEFAULT_TIMEOUT` |
| 接口 | 动词+er 后缀 | `Reader`, `Writer`, `Validator` |

### 错误处理

```go
// 好：检查并处理错误
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// 不好：忽略错误
result, _ := doSomething()
```

### 注释

```go
// Package tx provides transaction handling functionality.
package tx

// Transaction represents a blockchain transaction.
// It contains inputs, outputs, and execution information.
type Transaction struct {
    // ID is the unique identifier of the transaction.
    ID TxID
    // Inputs contains the transaction inputs.
    Inputs []Input
    // ...
}

// Validate checks if the transaction is valid.
// It returns an error if validation fails.
func (tx *Transaction) Validate() error {
    // ...
}
```

### 代码组织

```go
// 1. 包声明
package tx

// 2. 导入（标准库、第三方、本项目）
import (
    "context"
    "fmt"
    
    "github.com/pkg/errors"
    
    "github.com/weisyn/weisyn/internal/core/eutxo"
)

// 3. 常量
const (
    MaxTxSize = 1 << 20 // 1MB
)

// 4. 变量
var (
    ErrInvalidTx = errors.New("invalid transaction")
)

// 5. 类型定义
type Transaction struct {
    // ...
}

// 6. 构造函数
func NewTransaction() *Transaction {
    // ...
}

// 7. 方法
func (tx *Transaction) Validate() error {
    // ...
}

// 8. 辅助函数
func validateInputs(inputs []Input) error {
    // ...
}
```

---

## 测试规范

### 测试文件

- 测试文件以 `_test.go` 结尾
- 放在与被测试代码相同的包中

### 测试函数

```go
func TestTransaction_Validate(t *testing.T) {
    tests := []struct {
        name    string
        tx      *Transaction
        wantErr bool
    }{
        {
            name:    "valid transaction",
            tx:      newValidTx(),
            wantErr: false,
        },
        {
            name:    "empty inputs",
            tx:      newTxWithEmptyInputs(),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.tx.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 代码检查

### golangci-lint

使用 golangci-lint 进行代码检查：

```bash
golangci-lint run
```

### 配置

项目使用 `.golangci.yml` 配置文件，包含：
- 启用的 linter
- 排除规则
- 严重性设置

---

## Git 提交规范

### 提交消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 类型

| 类型 | 说明 |
|------|------|
| feat | 新功能 |
| fix | 修复 bug |
| docs | 文档更新 |
| style | 格式调整 |
| refactor | 重构 |
| test | 测试相关 |
| chore | 构建/工具相关 |

### 示例

```
feat(tx): add RBF support

Add Replace-By-Fee support for transaction replacement.

- Add fee comparison logic
- Update mempool handling
- Add RBF flag to transaction

Closes #123
```

---

## 相关文档

- [开发环境搭建](./development-setup.md) - 环境配置
- [文档规范](./docs-style.md) - 文档编写标准
- [`_dev/04-工程标准-standards/`](../../../_dev/04-工程标准-standards/) - 完整工程标准

