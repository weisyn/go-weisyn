# cmd/tools/ - WES 工具集

本目录包含 WES 项目的各种实用工具，每个工具都是独立的可执行程序。

## 📋 前置条件

在开始之前，请确保：

1. **已获取源代码**：克隆了 [GitHub 仓库](https://github.com/weisyn/go-weisyn)
2. **Go 环境**：Go 1.21 或更高版本（检查：`go version`）
3. **终端/命令行**：能够执行命令的终端窗口

## 📁 目录结构

```
cmd/tools/
├── calculate-genesis-hash/  # 计算创世哈希工具
├── cleanup/                 # 数据清理工具
├── keygen/                  # 密钥生成工具
├── param-encoder/           # 参数编码工具
├── validate-configs/        # 配置验证工具
└── README.md                # 本文件
```

## 🛠️ 工具列表

### cleanup - 数据清理工具

**用途**: 清理 WES 区块链数据目录和临时文件

**使用 go run（推荐用于开发验证）**：
```bash
# 在项目根目录下执行
# 预览模式（不会真正删除）
go run ./cmd/tools/cleanup --dry-run

# 实际清理
go run ./cmd/tools/cleanup
```

**先编译再运行（推荐用于生产环境）**：
```bash
# 编译
go build -o bin/wes-cleanup ./cmd/tools/cleanup

# 预览模式（不会真正删除）
./bin/wes-cleanup --dry-run

# 实际清理
./bin/wes-cleanup
```

**详细文档**: 参见 [cleanup/README.md](./cleanup/README.md)

---

### keygen - 密钥生成工具

**用途**: 生成 WES 区块链密钥对和创世块密钥

**使用 go run（推荐用于开发验证）**：
```bash
# 在项目根目录下执行
# 生成 5 个密钥对
go run ./cmd/tools/keygen generate 5

# 生成创世块密钥
go run ./cmd/tools/keygen genesis
```

**先编译再运行（推荐用于生产环境）**：
```bash
# 编译
go build -o bin/wes-keygen ./cmd/tools/keygen

# 生成 5 个密钥对
./bin/wes-keygen generate 5

# 生成创世块密钥
./bin/wes-keygen genesis
```

**详细文档**: 参见 [keygen/README.md](./keygen/README.md)

---

### param-encoder - 参数编码工具

**用途**: 将智能合约参数编码为十六进制格式

**使用 go run（推荐用于开发验证）**：
```bash
# 在项目根目录下执行
# 编码转账参数
go run ./cmd/tools/param-encoder transfer <address> <amount>
```

**先编译再运行（推荐用于生产环境）**：
```bash
# 编译
go build -o bin/wes-param-encoder ./cmd/tools/param-encoder

# 编码转账参数
./bin/wes-param-encoder transfer <address> <amount>
```

**详细文档**: 参见 [param-encoder/README.md](./param-encoder/README.md)

---

### calculate-genesis-hash - 计算创世哈希工具

**用途**: 从链配置文件计算确定性的创世区块哈希（genesis_hash）

**使用 go run（推荐用于开发验证）**：
```bash
# 在项目根目录下执行
# 计算单个配置文件的创世哈希
go run ./cmd/tools/calculate-genesis-hash/main.go configs/chains/test-public-demo.json

# 计算多个配置文件的创世哈希
go run ./cmd/tools/calculate-genesis-hash/main.go configs/chains/*.json
```

**先编译再运行（推荐用于生产环境）**：
```bash
# 编译
go build -o bin/wes-calculate-genesis-hash ./cmd/tools/calculate-genesis-hash

# 计算创世哈希
./bin/wes-calculate-genesis-hash configs/chains/test-public-demo.json
```

**详细文档**: 参见 [calculate-genesis-hash/README.md](./calculate-genesis-hash/README.md)

---

### validate-configs - 配置验证工具

**用途**: 验证链配置文件是否符合规范，防止配置/文档漂移

**使用 go run（推荐用于开发验证）**：
```bash
# 在项目根目录下执行
# 验证单个配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/test-public-demo.json

# 验证所有配置文件
go run ./cmd/tools/validate-configs/main.go configs/chains/*.json
```

**先编译再运行（推荐用于生产环境）**：
```bash
# 编译
go build -o bin/wes-validate-configs ./cmd/tools/validate-configs

# 验证配置文件
./bin/wes-validate-configs configs/chains/*.json
```

**详细文档**: 参见 [validate-configs/README.md](./validate-configs/README.md)

---

## 🔨 构建所有工具

### 使用 Makefile（推荐）

```bash
# 在项目根目录下执行
make build-tools
```

### 手动构建

```bash
# 在项目根目录下执行
mkdir -p bin

# 构建 calculate-genesis-hash
go build -o bin/wes-calculate-genesis-hash ./cmd/tools/calculate-genesis-hash

# 构建 cleanup
go build -o bin/wes-cleanup ./cmd/tools/cleanup

# 构建 keygen
go build -o bin/wes-keygen ./cmd/tools/keygen

# 构建 param-encoder
go build -o bin/wes-param-encoder ./cmd/tools/param-encoder

# 构建 validate-configs
go build -o bin/wes-validate-configs ./cmd/tools/validate-configs
```

构建完成后，所有二进制文件都在 `bin/` 目录下。

## ❓ 常见问题

### Q: 使用 go run 还是编译后运行？

**A:** 
- **开发验证**：使用 `go run ./cmd/tools/<tool-name>`，无需编译，修改代码后立即生效
- **生产环境**：先编译，然后运行编译后的二进制文件

### Q: 命令在哪里执行？

**A:** 在**终端/命令行**中执行。打开终端，进入项目根目录，然后执行命令。

### Q: 工具可以单独分发吗？

**A:** 可以。每个工具都是独立的可执行程序，编译后可以单独分发和使用。

## 📝 添加新工具

1. 在 `cmd/tools/` 下创建新目录，例如 `cmd/tools/my-tool/`
2. 创建 `main.go` 作为入口点
3. 创建 `README.md` 说明工具用途和使用方法
4. 更新本 README，添加工具说明

## 🎯 工具设计原则

- **独立性**: 每个工具都是独立的可执行程序，不依赖其他工具
- **简单性**: 工具功能单一，易于理解和维护
- **可复用性**: 工具可以被脚本、CI/CD 等自动化流程调用
- **文档完善**: 每个工具都有清晰的 README 和使用说明

## 📚 相关文档

- **[cmd/README.md](../README.md)** - cmd/ 目录总览
- **[node/README.md](../node/README.md)** - 节点启动说明
- **[cli/README.md](../cli/README.md)** - CLI 客户端说明
