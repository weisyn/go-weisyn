# 🔧 参数编码工具 (Param Encoder)

> **工具功能**: 将智能合约参数编码为十六进制格式

## 📋 快速开始

```bash
# 编码转账参数
go run ./cmd/tools/param-encoder transfer CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG 1000

# 编码余额查询参数
go run ./cmd/tools/param-encoder balance CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR

# 编码授权参数
go run ./cmd/tools/param-encoder approve CSpenderAddress123456789012345678901234567890 5000
```

## 功能说明

`param-encoder` 工具用于将人类可读的交易参数转换为区块链可识别的十六进制编码格式，包括：

- ✅ 转账参数编码
- ✅ 余额查询编码
- ✅ 授权参数编码
- ✅ 代理转账编码

### 主要特性

1. **类型安全**: 严格的参数类型检查和转换
2. **标准兼容**: 遵循区块链行业标准的编码规范
3. **易于使用**: 直观的命令行接口
4. **错误友好**: 详细的错误提示和参数验证

## 使用方法

### 基本用法

```bash
# 编译工具
go build -o bin/wes-param-encoder ./cmd/tools/param-encoder

# 查看帮助
./bin/wes-param-encoder
```

### 支持的操作

| 操作 | 命令格式 | 参数说明 |
|------|----------|----------|
| `transfer` | `transfer <to_address> <amount>` | 接收地址 + 转账金额 |
| `balance` | `balance <address>` | 查询地址 |
| `approve` | `approve <spender> <amount>` | 授权地址 + 授权额度 |
| `transfer_from` | `transfer_from <from> <to> <amount>` | 代理转账 |

## 使用示例

### 转账参数编码

```bash
go run ./cmd/tools/param-encoder transfer CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG 1000
```

输出示例：
```
🔄 编码转账参数...
🔍 解码地址: CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG
地址字节: 742d35cc61b8882921493b5c03e69ff9c555b5ce (20字节)
✅ 转账参数编码完成
操作: 转账 1000 WES 到 CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG
十六进制参数: 742d35cc61b8882921493b5c03e69ff9c555b5ce00000000000003e8
参数长度: 28 字节 (地址20字节 + 金额8字节)

📋 可用于API调用的参数:
"parameters": "742d35cc61b8882921493b5c03e69ff9c555b5ce00000000000003e8"
```

### 余额查询编码

```bash
go run ./cmd/tools/param-encoder balance CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR
```

## 相关文档

- **[tools/README.md](../README.md)** - 工具集总览
- **[keygen/README.md](../keygen/README.md)** - 密钥生成工具

