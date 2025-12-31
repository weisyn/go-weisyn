# Windows ARM32v7 (arm) 平台支持

本目录用于存放 Windows ARM32v7 平台的 ONNX Runtime 库文件和嵌入代码。

## 📋 平台信息

- **操作系统**: Windows
- **架构**: ARM32v7 (arm)
- **库文件名**: `onnxruntime.dll`
- **状态**: ⚠️ 需从源码编译（无预编译库）

## 🔧 如何启用支持

### 步骤 1: 从源码编译 ONNX Runtime

ONNX Runtime 官方不提供此平台的预编译库，需要从源码编译。

**编译环境要求**：
- Windows ARM32v7 系统（或交叉编译环境）
- Visual Studio 2019+ 或 Visual Studio Build Tools（支持 ARM）
- CMake 3.18+
- Python 3.6+（用于构建脚本）

**编译命令**（使用 Visual Studio）：
```bash
# 克隆 ONNX Runtime 仓库
git clone --recursive https://github.com/microsoft/onnxruntime.git
cd onnxruntime

# 使用 CMake 配置构建（Windows ARM32v7）
cmake -G "Visual Studio 16 2019" -A ARM -DCMAKE_BUILD_TYPE=Release -Donnxruntime_BUILD_SHARED_LIB=ON .

# 编译
cmake --build . --config Release --parallel

# 编译后的库文件位置
# build/Windows/Release/Release/onnxruntime.dll
```

**使用 PowerShell**：
```powershell
# 配置构建
cmake -G "Visual Studio 16 2019" -A ARM -DCMAKE_BUILD_TYPE=Release -Donnxruntime_BUILD_SHARED_LIB=ON .

# 编译
cmake --build . --config Release --parallel
```

### 步骤 2: 复制库文件

将编译好的库文件复制到此目录：

```bash
# 在 Git Bash 或 PowerShell 中
cp build/Windows/Release/Release/onnxruntime.dll pkg/build/deps/onnx/libs/windows_arm/onnxruntime.dll
```

### 步骤 3: 启用嵌入代码

编辑 `embedded.go` 文件，取消注释：

**修改前**：
```go
// 需要从源码编译 ONNX Runtime，编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed onnxruntime.dll
// var embeddedLibWindowsARM []byte

// func init() {
// 	libWindowsARM = embeddedLibWindowsARM
// }
```

**修改后**：
```go
//go:embed onnxruntime.dll
var embeddedLibWindowsARM []byte

func init() {
	libWindowsARM = embeddedLibWindowsARM
}
```

### 步骤 4: 更新主文件

编辑 `pkg/build/deps/onnx/embedded.go`，在 `getEmbeddedLibrary()` 函数中添加对应的 `case` 分支：

```go
case "windows_arm":
    if len(libWindowsARM) == 0 {
        return nil, fmt.Errorf("嵌入的库文件为空 (windows_arm)。请参考 libs/windows_arm/embedded.go")
    }
    return libWindowsARM, nil
```

### 步骤 5: 验证

```bash
# 重新构建
go build ./cmd/weisyn

# 测试运行
go run ./cmd/weisyn
```

## 📚 相关资源

- [ONNX Runtime 构建文档](https://onnxruntime.ai/docs/build/)
- [ONNX Runtime GitHub](https://github.com/microsoft/onnxruntime)
- [Visual Studio 下载](https://visualstudio.microsoft.com/)
- [平台支持说明](../../README.md)

## ⚠️ 注意事项

1. **版本一致性**：编译的库文件版本应与预编译库版本一致（当前为 v1.23.2）
2. **文件命名**：确保库文件名为 `onnxruntime.dll`
3. **架构设置**：确保使用 `ARM` 架构（不是 `ARM64`）
4. **交叉编译**：如果使用交叉编译，确保 ARM 工具链配置正确
5. **测试验证**：添加支持后，务必在 Windows ARM32v7 平台上进行完整测试

