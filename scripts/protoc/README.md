# Protocol Buffers 代码生成工具

　　本目录包含用于生成 Protocol Buffers Go 代码的脚本工具。

## 功能说明

　　`generate_proto.sh` 脚本用于**自动发现并生成**项目中所有 `.proto` 文件对应的 Go 语言绑定代码。

### 🤖 自动发现机制

　　脚本使用 `find` 命令自动搜索 `pb/` 目录下的所有 `.proto` 文件，无需手动维护文件列表。执行时会显示发现的文件：

```text
[proto] Discovering .proto files in pb/ directory...
[proto] Found 13 .proto files:
  - pb/blockchain/core/block.proto
  - pb/blockchain/core/transaction.proto
  - pb/blockchain/execution/contract_execution.proto
  - pb/blockchain/execution/core_execution.proto
  - pb/blockchain/resource/resource.proto
  - pb/blockchain/sync/sync.proto
  - pb/blockchain/utxo/utxo.proto
  - pb/common/common.proto
  - pb/common/types.proto
  - pb/consensus/network.proto
  - pb/network/envelope.proto
  - pb/p2p/message.proto
  - pb/p2p/node.proto
```

### 📂 处理的文件类型

　　脚本会自动处理以下分类的 protobuf 文件：

- **核心协议**：区块、交易、网络、同步
- **通用类型**：公共数据结构和类型定义
- **网络层**：P2P 通信和共识网络协议
- **执行层**：智能合约、资源管理、UTXO 系统

### ✨ 优势特性

- **零维护**：新增 `.proto` 文件时无需修改脚本
- **防遗漏**：自动发现所有文件，避免人为遗漏
- **可见性**：清楚显示处理的文件和进度
- **容错性**：对不存在的文件给出警告而不中断

## 前置条件

　　在执行脚本之前，请确保已安装以下工具：

### 1. Protocol Buffers 编译器

**macOS (使用 Homebrew):**

```bash
brew install protobuf
```

**Ubuntu/Debian:**

```bash
sudo apt-get install protobuf-compiler
```

**Windows:**

- 从 [Protocol Buffers releases](https://github.com/protocolbuffers/protobuf/releases) 下载预编译的二进制文件
- 将 `protoc.exe` 添加到系统 PATH 环境变量

### 2. Go Protocol Buffers 插件

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

　　确保 `$GOPATH/bin` 已添加到系统 PATH 环境变量中。

### 3. 验证安装

```bash
# 检查 protoc 是否正确安装
protoc --version

# 检查 Go 插件是否正确安装
protoc-gen-go --version
```

## 执行方法

### 方法一：在项目根目录执行

```bash
cd /path/to/your/weisyn
./scripts/protoc/generate_proto.sh
```

### 方法二：直接执行脚本

```bash
cd /path/to/your/weisyn/scripts/protoc
./generate_proto.sh
```

### 方法三：使用 bash 执行

```bash
bash /path/to/your/weisyn/scripts/protoc/generate_proto.sh
```

## 输出结果

　　脚本执行成功后会：

### 1. 显示发现和处理进度

```text
[proto] Discovering .proto files in pb/ directory...
[proto] Found 13 .proto files:
  - pb/blockchain/core/block.proto
  - pb/blockchain/core/transaction.proto
  - ... (所有发现的文件)

[proto] Generating Go code...
[proto] Processing: pb/blockchain/core/block.proto
[proto] Processing: pb/blockchain/core/transaction.proto
  ... (每个文件的处理进度)
```

### 2. 自动生成 Go 代码文件

　　在每个 `.proto` 文件的同级目录下生成对应的 `.pb.go` 文件。由于使用自动发现，生成的文件会根据实际存在的 `.proto` 文件动态确定。

**典型生成文件包括**：

- `pb/blockchain/core/*.pb.go` - 区块链核心数据结构
- `pb/blockchain/execution/*.pb.go` - 执行引擎相关
- `pb/common/*.pb.go` - 通用类型定义  
- `pb/p2p/*.pb.go` - P2P 网络协议
- `pb/network/*.pb.go` - 网络层协议
- 以及其他模块的 protobuf 文件

### 3. 输出完成统计

```text
[proto] Generated successfully!
[proto] Total files processed: 13
```

## 注意事项

### 重要提醒

⚠️ **不要手动编辑生成的 `.pb.go` 文件！**

　　所有 `.pb.go` 文件都是从对应的 `.proto` 文件自动生成的。如需修改，请：

1. 修改相应的 `.proto` 文件
2. 重新运行此脚本生成新的 Go 代码

### 文件权限

　　如果在 Linux/macOS 上遇到权限问题，请给脚本添加执行权限：

```bash
chmod +x scripts/protoc/generate_proto.sh
```

### 常见错误排查

**错误：`protoc: command not found`**

- 解决：请确保已正确安装 Protocol Buffers 编译器并添加到 PATH

**错误：`protoc-gen-go: program not found or is not executable`**

- 解决：请确保已安装 Go 插件并将 `$GOPATH/bin` 添加到 PATH

**错误：`no such file or directory`**

- 解决：请确保从项目根目录执行脚本，或检查 `.proto` 文件是否存在

## 开发工作流

　　在以下情况下需要运行此脚本：

### 🔄 自动化工作流

1. **修改 .proto 文件后**：更新数据结构定义
2. **新增 .proto 文件后**：✨ **无需手动配置** - 脚本会自动发现新文件！
3. **构建项目前**：确保所有 protobuf 代码都是最新的  
4. **代码审查前**：确保提交的代码包含最新的生成文件

### 💡 零维护优势

　　与之前需要手动维护文件列表的版本相比，新版本具有以下优势：

- **✅ 自动发现**：新增 `.proto` 文件时无需修改脚本
- **✅ 防遗漏**：不会因为忘记添加文件到脚本而遗漏
- **✅ 即时反馈**：清楚显示发现了哪些文件和处理进度
- **✅ 更简单**：开发者只需关注 protobuf 设计，不需要维护构建脚本

## 相关文档

- [Protocol Buffers 官方文档](https://developers.google.com/protocol-buffers)
- [Go Protocol Buffers 教程](https://developers.google.com/protocol-buffers/docs/gotutorial)
- [项目 API 文档](../../api/README.md)
