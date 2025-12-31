# ONNX Runtime 库文件目录说明

本目录包含所有平台的 ONNX Runtime 库文件和对应的嵌入代码。

## 📁 目录结构

每个平台目录包含：
- **库文件**：`libonnxruntime.so` / `libonnxruntime.dylib` / `onnxruntime.dll`
- **嵌入代码**：`embedded.go` - 使用条件编译嵌入库文件

```
libs/
├── darwin_amd64/
│   ├── libonnxruntime.dylib  ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── darwin_arm64/
│   ├── libonnxruntime.dylib  ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── linux_amd64/
│   ├── libonnxruntime.so     ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── linux_arm64/
│   ├── libonnxruntime.so     ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── windows_amd64/
│   ├── onnxruntime.dll        ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── windows_arm64/
│   ├── onnxruntime.dll        ✅ 已下载
│   └── embedded.go            ✅ 已启用
├── linux_386/
│   └── embedded.go            ⚠️ 待启用（需从源码编译）
├── linux_arm/
│   └── embedded.go            ⚠️ 待启用（需从源码编译）
└── ... (其他平台)
```

## ✅ 预编译库支持（6个平台）

以下平台有 ONNX Runtime 官方提供的预编译库，已下载并启用：

| 目录名 | 平台 | 架构 | 库文件名 | 状态 |
|--------|------|------|---------|------|
| `darwin_amd64/` | macOS | Intel (x86_64) | `libonnxruntime.dylib` | ✅ 已启用 |
| `darwin_arm64/` | macOS | Apple Silicon | `libonnxruntime.dylib` | ✅ 已启用 |
| `linux_amd64/` | Linux | x86_64 | `libonnxruntime.so` | ✅ 已启用 |
| `linux_arm64/` | Linux | ARM64 (aarch64) | `libonnxruntime.so` | ✅ 已启用 |
| `windows_amd64/` | Windows | x64 | `onnxruntime.dll` | ✅ 已启用 |
| `windows_arm64/` | Windows | ARM64 | `onnxruntime.dll` | ✅ 已启用 |

## ⚠️ 需从源码编译（10个平台）

以下平台受 ONNX Runtime 官方支持，但**无预编译库**，需要从源码编译：

| 目录名 | 平台 | 架构 | 库文件名 | 状态 | 说明 |
|--------|------|------|---------|------|------|
| `linux_386/` | Linux | x86_32 | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `linux_arm/` | Linux | ARM32v7 | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `linux_ppc64le/` | Linux | PPC64LE | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `linux_riscv64/` | Linux | RISCV64 | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `linux_s390x/` | Linux | S390X | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `windows_386/` | Windows | x86_32 | `onnxruntime.dll` | ⚠️ 待启用 | 无预编译库 |
| `windows_arm/` | Windows | ARM32v7 | `onnxruntime.dll` | ⚠️ 待启用 | 无预编译库 |
| `android_arm/` | Android | ARM32v7 | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `android_arm64/` | Android | ARM64 | `libonnxruntime.so` | ⚠️ 待启用 | 无预编译库 |
| `ios_arm64/` | iOS | ARM64 | `libonnxruntime.dylib` | ⚠️ 待启用 | 无预编译库 |

## 🔧 如何添加手动编译的平台支持

### 步骤 1: 从源码编译 ONNX Runtime

参考 ONNX Runtime 官方文档：[Building ONNX Runtime](https://onnxruntime.ai/docs/build/)

**基本编译命令示例**（Linux x86_32）：
```bash
# 克隆 ONNX Runtime 仓库
git clone --recursive https://github.com/microsoft/onnxruntime.git
cd onnxruntime

# 配置构建（Linux x86_32 示例）
./build.sh --config Release --build_shared_lib --parallel

# 编译后的库文件位置
# Linux: build/Linux/Release/libonnxruntime.so
# Windows: build/Windows/Release/Release/onnxruntime.dll
# macOS: build/MacOS/Release/libonnxruntime.dylib
```

**交叉编译示例**（Android ARM64）：
```bash
# Android ARM64 交叉编译
./build.sh --config Release --build_shared_lib \
  --android --android_abi arm64-v8a \
  --android_api 29
```

### 步骤 2: 复制库文件到对应目录

将编译好的库文件复制到对应的平台目录：

```bash
# Linux x86_32 示例
cp build/Linux/Release/libonnxruntime.so pkg/build/deps/onnx/libs/linux_386/libonnxruntime.so

# Linux ARM32v7 示例
cp build/Linux/Release/libonnxruntime.so pkg/build/deps/onnx/libs/linux_arm/libonnxruntime.so

# Windows x86_32 示例
cp build/Windows/Release/Release/onnxruntime.dll pkg/build/deps/onnx/libs/windows_386/onnxruntime.dll

# Android ARM64 示例
cp build/Android/Release/libonnxruntime.so pkg/build/deps/onnx/libs/android_arm64/libonnxruntime.so

# iOS ARM64 示例
cp build/iOS/Release/libonnxruntime.dylib pkg/build/deps/onnx/libs/ios_arm64/libonnxruntime.dylib
```

### 步骤 3: 启用嵌入代码

编辑对应平台的 `embedded.go` 文件，取消注释：

**示例：启用 Linux x86_32 支持**

编辑 `pkg/build/deps/onnx/libs/linux_386/embedded.go`：

```go
//go:build linux && 386
// +build linux,386

package onnx

import _ "embed"

// 取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
var embeddedLibLinux386 []byte

func init() {
	libLinux386 = embeddedLibLinux386
}
```

**修改为：**

```go
//go:build linux && 386
// +build linux,386

package onnx

import _ "embed"

//go:embed libonnxruntime.so
var embeddedLibLinux386 []byte

func init() {
	libLinux386 = embeddedLibLinux386
}
```

### 步骤 4: 更新主文件

编辑 `pkg/build/deps/onnx/embedded.go`，在 `getEmbeddedLibrary()` 函数中添加对应的 `case` 分支：

```go
case "linux_386":
    if len(libLinux386) == 0 {
        return nil, fmt.Errorf("嵌入的库文件为空 (linux_386)")
    }
    return libLinux386, nil
```

### 步骤 5: 验证

```bash
# 重新构建
go build ./cmd/weisyn

# 测试运行
go run ./cmd/weisyn
```

## 📋 平台支持总结

| 类型 | 数量 | 状态 |
|------|------|------|
| **预编译库** | 6个 | ✅ 已下载并启用 |
| **需从源码编译** | 10个 | ⚠️ 目录和 embedded.go 已创建，等待手动编译 |
| **总计** | 16个 | 所有 ONNX Runtime 官方支持的平台 |

## 📚 相关文档

- [ONNX Runtime 官方文档](https://onnxruntime.ai/docs/)
- [ONNX Runtime GitHub](https://github.com/microsoft/onnxruntime)
- [构建指南](https://onnxruntime.ai/docs/build/)
- [平台支持说明](../README.md)
- [部署说明](../DEPLOYMENT.md)

## ⚠️ 注意事项

1. **版本一致性**：手动编译的库文件版本应与预编译库版本一致（当前为 v1.23.2）
2. **文件命名**：确保库文件名与表格中的名称一致
3. **依赖项**：编译前确保安装了所有必要的依赖项（CMake、编译器工具链等）
4. **测试验证**：添加新平台后，务必在对应平台上进行完整测试
5. **嵌入路径**：`embedded.go` 中的 `//go:embed` 路径是相对于当前目录的，例如 `libonnxruntime.so` 表示当前目录下的文件

## 🔍 文件结构说明

每个平台目录的结构：

```
{platform}/
├── libonnxruntime.{so|dylib|dll}  # 库文件（预编译或手动编译）
└── embedded.go                     # 嵌入代码（使用条件编译）
```

**embedded.go 文件说明**：
- 使用 `//go:build` 条件编译标签，只在对应平台编译时生效
- 使用 `//go:embed` 嵌入当前目录下的库文件
- 在 `init()` 函数中将嵌入的数据赋值给全局变量

**优势**：
- ✅ 库文件和嵌入代码在一起，结构清晰
- ✅ 每个平台独立管理，易于维护
- ✅ 添加新平台只需在对应目录操作，不影响其他平台
