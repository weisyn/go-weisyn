# Linux ARM32v7 (arm) 平台支持

本目录用于存放 Linux ARM32v7 平台的 ONNX Runtime 库文件和嵌入代码。

## 📋 平台信息

- **操作系统**: Linux
- **架构**: ARM32v7 (arm)
- **库文件名**: `libonnxruntime.so`
- **状态**: ⚠️ 需从源码编译（无预编译库）

## 🔧 如何启用支持

### 步骤 1: 从源码编译 ONNX Runtime

ONNX Runtime 官方不提供此平台的预编译库，需要从源码编译。

**编译环境要求**：
- Linux ARM32v7 系统（或交叉编译环境）
- CMake 3.18+
- ARM 交叉编译工具链（如果交叉编译）
- Python 3.6+（用于构建脚本）

**编译命令**（在 ARM32v7 设备上）：
```bash
# 克隆 ONNX Runtime 仓库
git clone --recursive https://github.com/microsoft/onnxruntime.git
cd onnxruntime

# 配置构建（Linux ARM32v7）
./build.sh --config Release --build_shared_lib --parallel

# 编译后的库文件位置
# build/Linux/Release/libonnxruntime.so
```

**交叉编译**（在 x86_64 主机上）：
```bash
# 安装交叉编译工具链
sudo apt-get install gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf

# 配置交叉编译
./build.sh --config Release --build_shared_lib \
  --cmake_extra_defines CMAKE_C_COMPILER=arm-linux-gnueabihf-gcc \
  --cmake_extra_defines CMAKE_CXX_COMPILER=arm-linux-gnueabihf-g++
```

### 步骤 2: 复制库文件

将编译好的库文件复制到此目录：

```bash
cp build/Linux/Release/libonnxruntime.so pkg/build/deps/onnx/libs/linux_arm/libonnxruntime.so
```

### 步骤 3: 启用嵌入代码

编辑 `embedded.go` 文件，取消注释：

**修改前**：
```go
// 需要从源码编译 ONNX Runtime，编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibLinuxARM []byte

// func init() {
// 	libLinuxARM = embeddedLibLinuxARM
// }
```

**修改后**：
```go
//go:embed libonnxruntime.so
var embeddedLibLinuxARM []byte

func init() {
	libLinuxARM = embeddedLibLinuxARM
}
```

### 步骤 4: 更新主文件

编辑 `pkg/build/deps/onnx/embedded.go`，在 `getEmbeddedLibrary()` 函数中添加对应的 `case` 分支：

```go
case "linux_arm":
    if len(libLinuxARM) == 0 {
        return nil, fmt.Errorf("嵌入的库文件为空 (linux_arm)。请参考 libs/linux_arm/embedded.go")
    }
    return libLinuxARM, nil
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
- [平台支持说明](../../README.md)

## ⚠️ 注意事项

1. **版本一致性**：编译的库文件版本应与预编译库版本一致（当前为 v1.23.2）
2. **文件命名**：确保库文件名为 `libonnxruntime.so`
3. **交叉编译**：如果使用交叉编译，确保工具链配置正确
4. **测试验证**：添加支持后，务必在 Linux ARM32v7 平台上进行完整测试

