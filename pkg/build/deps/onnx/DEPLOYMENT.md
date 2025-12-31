# ONNX Runtime 二进制打包与部署说明

## 📦 二进制打包情况

### ✅ ONNX Runtime 库文件已完全打包

**当前编译平台的 ONNX Runtime 库文件已嵌入到二进制中**，无需额外依赖。

- **二进制文件大小**：约 96-134MB（仅包含当前编译平台的库文件）
- **库文件大小**：约 14-38MB（单个平台的预编译库）
- **Go 代码大小**：约 82-96MB（包含其他依赖）

### 🔍 打包机制

**使用条件编译（build tags）**，只嵌入当前编译平台的库文件：

1. **构建时**：根据 `GOOS` 和 `GOARCH` 自动选择对应的嵌入文件
   - macOS Intel: `embedded_darwin_amd64.go` → `libonnxruntime.dylib` (38M)
   - macOS Apple Silicon: `embedded_darwin_arm64.go` → `libonnxruntime.dylib` (34M)
   - Linux x64: `embedded_linux_amd64.go` → `libonnxruntime.so` (21M)
   - Linux ARM64: `embedded_linux_arm64.go` → `libonnxruntime.so` (18M)
   - Windows x64: `embedded_windows_amd64.go` → `onnxruntime.dll` (14M)
   - Windows ARM64: `embedded_windows_arm64.go` → `onnxruntime.dll` (14M)

2. **优势**：
   - ✅ **减小体积**：只嵌入当前平台的库文件，二进制文件更小
   - ✅ **自动选择**：编译时自动选择对应平台的文件
   - ✅ **零配置**：无需手动指定，Go 编译器自动处理

2. **运行时**：
   - 自动检测当前运行平台
   - 从嵌入的二进制中提取对应平台的库文件
   - 提取到 `~/.weisyn/libs/` 目录（仅首次运行）
   - 后续运行直接使用缓存的库文件

## 🚀 跨平台使用

### ✅ 可以传输到其他电脑

**二进制文件可以传输到其他相同平台的电脑上直接运行**，无需安装任何依赖。

### ⚠️ 平台限制

**重要**：Go 二进制文件是平台特定的，只能在编译时的平台上运行。

| 编译平台 | 可运行平台 | 嵌入的库文件 | 二进制大小 |
|---------|-----------|-------------|-----------|
| macOS (darwin_arm64) | macOS (darwin_arm64) | `libonnxruntime.dylib` (34M) | ~130MB |
| macOS (darwin_amd64) | macOS (darwin_amd64) | `libonnxruntime.dylib` (38M) | ~134MB |
| Linux (linux_amd64) | Linux (linux_amd64) | `libonnxruntime.so` (21M) | ~117MB |
| Linux (linux_arm64) | Linux (linux_arm64) | `libonnxruntime.so` (18M) | ~114MB |
| Windows (windows_amd64) | Windows (windows_amd64) | `onnxruntime.dll` (14M) | ~110MB |
| Windows (windows_arm64) | Windows (windows_arm64) | `onnxruntime.dll` (14M) | ~110MB |

**跨平台运行**：
- ❌ macOS 编译的二进制不能在 Linux 上运行
- ❌ Linux 编译的二进制不能在 Windows 上运行
- ✅ 需要使用交叉编译为不同平台生成二进制

**优化说明**：
- ✅ 使用条件编译，只嵌入当前平台的库文件
- ✅ 二进制文件大小减少约 100MB（相比嵌入所有平台）
- ✅ 编译时自动选择对应平台的文件，无需手动配置

## 🔧 交叉编译

### 为不同平台编译二进制

使用 Go 的交叉编译功能，可以在一个平台上为其他平台编译：

```bash
# 为 Linux x64 编译（在 macOS 上）
GOOS=linux GOARCH=amd64 go build -o weisyn-linux-amd64 ./cmd/weisyn

# 为 Linux ARM64 编译（在 macOS 上）
GOOS=linux GOARCH=arm64 go build -o weisyn-linux-arm64 ./cmd/weisyn

# 为 Windows x64 编译（在 macOS/Linux 上）
GOOS=windows GOARCH=amd64 go build -o weisyn-windows-amd64.exe ./cmd/weisyn

# 为 Windows ARM64 编译（在 macOS/Linux 上）
GOOS=windows GOARCH=arm64 go build -o weisyn-windows-arm64.exe ./cmd/weisyn

# 为 macOS Intel 编译（在 macOS ARM 上）
GOOS=darwin GOARCH=amd64 go build -o weisyn-darwin-amd64 ./cmd/weisyn

# 为 macOS Apple Silicon 编译（在 macOS Intel 上）
GOOS=darwin GOARCH=arm64 go build -o weisyn-darwin-arm64 ./cmd/weisyn
```

### 交叉编译注意事项

1. **CGO 依赖**：如果代码使用了 CGO，交叉编译可能需要配置交叉编译工具链
2. **库文件嵌入**：所有平台的库文件都会嵌入，但运行时只会提取当前平台的
3. **文件大小**：交叉编译的二进制大小相同（都包含所有平台的库文件）

## 📋 部署场景

### 场景 1: 单平台部署

**适用**：只在一种平台上使用（如仅 Linux 服务器）

```bash
# 在目标平台上编译
go build -o weisyn ./cmd/weisyn

# 传输到其他相同平台的服务器
scp weisyn user@server:/path/to/
```

**优点**：
- 简单直接
- 二进制文件包含所有依赖

**缺点**：
- 二进制文件较大（234MB）
- 需要为每个平台单独编译

### 场景 2: 多平台部署

**适用**：需要支持多个平台

```bash
# 在一个平台上交叉编译所有平台
GOOS=linux GOARCH=amd64 go build -o weisyn-linux-amd64 ./cmd/weisyn
GOOS=linux GOARCH=arm64 go build -o weisyn-linux-arm64 ./cmd/weisyn
GOOS=windows GOARCH=amd64 go build -o weisyn-windows-amd64.exe ./cmd/weisyn
GOOS=darwin GOARCH=amd64 go build -o weisyn-darwin-amd64 ./cmd/weisyn
GOOS=darwin GOARCH=arm64 go build -o weisyn-darwin-arm64 ./cmd/weisyn
```

**优点**：
- 一次编译，多平台使用
- 统一版本管理

**缺点**：
- 每个二进制都包含所有平台的库文件（文件较大）

### 场景 3: 优化部署（可选）

如果希望减小二进制文件大小，可以：

1. **按平台分离构建**：修改 `embedded.go`，只嵌入当前平台的库文件
2. **使用构建标签**：使用 Go 的构建标签为不同平台创建不同的嵌入文件
3. **外部库文件**：不嵌入库文件，运行时从外部目录加载（需要额外分发库文件）

**注意**：当前设计使用条件编译，只嵌入当前编译平台的库文件，减小二进制体积。

## 🔍 验证二进制包含库文件

### 检查二进制文件大小

```bash
# 编译
go build -o weisyn ./cmd/weisyn

# 检查大小（应该约 234MB）
ls -lh weisyn

# 检查文件类型
file weisyn
```

### 验证运行时提取

```bash
# 首次运行（会提取库文件）
./weisyn

# 检查提取的库文件
ls -lh ~/.weisyn/libs/

# 应该看到对应平台的库文件，例如：
# ~/.weisyn/libs/libonnxruntime.dylib (macOS)
# ~/.weisyn/libs/libonnxruntime.so (Linux)
# ~/.weisyn/libs/onnxruntime.dll (Windows)
```

## 📝 总结

### ✅ 已实现的功能

1. **完全打包**：所有平台的 ONNX Runtime 库文件都已嵌入二进制
2. **自动提取**：运行时自动提取对应平台的库文件
3. **零依赖**：无需安装 ONNX Runtime 或任何其他依赖
4. **可传输**：二进制可以传输到其他相同平台的电脑直接运行

### ⚠️ 注意事项

1. **平台特定**：二进制只能在编译时的平台上运行（除非交叉编译）
2. **文件较大**：二进制文件约 234MB（包含所有平台的库文件）
3. **首次运行**：首次运行会提取库文件到 `~/.weisyn/libs/`（约 14-38MB）

### 🎯 推荐部署方式

1. **开发环境**：直接使用 `go run` 或 `go build`
2. **生产环境**：在目标平台上编译，或使用交叉编译
3. **分发**：单个二进制文件即可，无需额外依赖

## 🔗 相关文档

- [README.md](./README.md) - 基本使用说明
- [libs/README.md](./libs/README.md) - 库文件目录说明
- [platform.go](./platform.go) - 平台支持检测

