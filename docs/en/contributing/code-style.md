# Code Standards

---

## Overview

This document defines code style and coding standards for the WES project.

---

## General Principles

### Readability First

- Code should be self-explanatory
- Complex logic must have comments
- Use meaningful names

### Simplicity

- Prefer simple solutions
- Avoid over-engineering
- Follow KISS principle

### Consistency

- Follow existing project style
- Use standard formatting tools
- Unified error handling patterns

---

## Go Code Standards

### Formatting

Use `gofmt` or `goimports` to format code:

```bash
gofmt -w .
# or
goimports -w .
```

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package name | lowercase, single word | `tx`, `block`, `consensus` |
| Exported types | PascalCase | `Transaction`, `BlockHeader` |
| Unexported types | camelCase | `txPool`, `blockCache` |
| Constants | PascalCase or ALL_CAPS | `MaxBlockSize`, `DEFAULT_TIMEOUT` |
| Interfaces | Verb + er suffix | `Reader`, `Writer`, `Validator` |

### Error Handling

```go
// Good: Check and handle errors
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Bad: Ignore errors
result, _ := doSomething()
```

### Comments

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

### Code Organization

```go
// 1. Package declaration
package tx

// 2. Imports (standard library, third-party, this project)
import (
    "context"
    "fmt"
    
    "github.com/pkg/errors"
    
    "github.com/weisyn/weisyn/internal/core/eutxo"
)

// 3. Constants
const (
    MaxTxSize = 1 << 20 // 1MB
)

// 4. Variables
var (
    ErrInvalidTx = errors.New("invalid transaction")
)

// 5. Type definitions
type Transaction struct {
    // ...
}

// 6. Constructors
func NewTransaction() *Transaction {
    // ...
}

// 7. Methods
func (tx *Transaction) Validate() error {
    // ...
}

// 8. Helper functions
func validateInputs(inputs []Input) error {
    // ...
}
```

---

## Testing Standards

### Test Files

- Test files end with `_test.go`
- Placed in the same package as the code being tested

### Test Functions

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

## Code Checking

### golangci-lint

Use golangci-lint for code checking:

```bash
golangci-lint run
```

### Configuration

The project uses `.golangci.yml` configuration file, including:
- Enabled linters
- Exclusion rules
- Severity settings

---

## Git Commit Standards

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

| Type | Description |
|------|-------------|
| feat | New feature |
| fix | Bug fix |
| docs | Documentation update |
| style | Formatting changes |
| refactor | Refactoring |
| test | Test related |
| chore | Build/tool related |

### Example

```
feat(tx): add RBF support

Add Replace-By-Fee support for transaction replacement.

- Add fee comparison logic
- Update mempool handling
- Add RBF flag to transaction

Closes #123
```

---

## Related Documentation

- [Development Environment Setup](./development-setup.md) - Environment configuration
- [Documentation Standards](./docs-style.md) - Documentation writing standards
- [`_dev/04-工程标准-standards/`](../../../_dev/04-工程标准-standards/) - Complete engineering standards

