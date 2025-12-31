# iOS ARM64 平台支持

本目录用于存放 iOS ARM64 平台的 ONNX Runtime 库文件和嵌入代码。

## 📋 平台信息

- **操作系统**: iOS
- **架构**: ARM64
- **库文件名**: `libonnxruntime.dylib`
- **状态**: ⚠️ 需从源码编译（交叉编译，无预编译库）

## 🔧 如何启用支持

### 步骤 1: 从源码编译 ONNX Runtime（交叉编译）

ONNX Runtime 官方不提供此平台的预编译库，需要从源码交叉编译。

**编译环境要求**：
- macOS 主机系统（iOS 开发必须在 macOS 上）
- Xcode 12.0+
- CMake 3.18+
- Python 3.6+（用于构建脚本）
- iOS SDK

**编译命令**：
```bash
# 克隆 ONNX Runtime 仓库
git clone --recursive https://github.com/microsoft/onnxruntime.git
cd onnxruntime

# 查找 iOS SDK 路径
export IOS_SYSROOT=$(xcrun --sdk iphoneos --show-sdk-path)

# 配置构建（iOS ARM64）
./build.sh --config Release --build_shared_lib \
  --ios --ios_sysroot $IOS_SYSROOT --ios_arch arm64

# 编译后的库文件位置
# build/iOS/Release/libonnxruntime.dylib
```

**详细参数说明**：
- `--ios`: 启用 iOS 构建
- `--ios_sysroot <path>`: 指定 iOS SDK 路径
- `--ios_arch arm64`: 指定 ARM64 架构

**使用 CMake 直接构建**：
```bash
cmake -G Xcode \
  -DCMAKE_SYSTEM_NAME=iOS \
  -DCMAKE_OSX_ARCHITECTURES=arm64 \
  -DCMAKE_OSX_SYSROOT=$IOS_SYSROOT \
  -DCMAKE_BUILD_TYPE=Release \
  -Donnxruntime_BUILD_SHARED_LIB=ON \
  .

cmake --build . --config Release --parallel
```

### 步骤 2: 复制库文件

将编译好的库文件复制到此目录：

```bash
cp build/iOS/Release/libonnxruntime.dylib pkg/build/deps/onnx/libs/ios_arm64/libonnxruntime.dylib
```

### 步骤 3: 启用嵌入代码

编辑 `embedded.go` 文件，取消注释：

**修改前**：
```go
// 需要从源码编译 ONNX Runtime（交叉编译），编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.dylib
// var embeddedLibIOSARM64 []byte

// func init() {
// 	libIOSARM64 = embeddedLibIOSARM64
// }
```

**修改后**：
```go
//go:embed libonnxruntime.dylib
var embeddedLibIOSARM64 []byte

func init() {
	libIOSARM64 = embeddedLibIOSARM64
}
```

### 步骤 4: 更新主文件

编辑 `pkg/build/deps/onnx/embedded.go`，在 `getEmbeddedLibrary()` 函数中添加对应的 `case` 分支：

```go
case "ios_arm64":
    if len(libIOSARM64) == 0 {
        return nil, fmt.Errorf("嵌入的库文件为空 (ios_arm64)。请参考 libs/ios_arm64/embedded.go")
    }
    return libIOSARM64, nil
```

### 步骤 5: 验证

```bash
# 重新构建
go build ./cmd/weisyn

# 测试运行（需要在 iOS 设备或模拟器上）
go run ./cmd/weisyn
```

## 📚 相关资源

- [ONNX Runtime 构建文档](https://onnxruntime.ai/docs/build/)
- [ONNX Runtime GitHub](https://github.com/microsoft/onnxruntime)
- [Xcode 下载](https://developer.apple.com/xcode/)
- [iOS 开发文档](https://developer.apple.com/ios/)
- [平台支持说明](../../README.md)

## ⚠️ 注意事项

1. **版本一致性**：编译的库文件版本应与预编译库版本一致（当前为 v1.23.2）
2. **文件命名**：确保库文件名为 `libonnxruntime.dylib`
3. **macOS 要求**：iOS 开发必须在 macOS 系统上进行
4. **Xcode 版本**：建议使用 Xcode 12.0 或更高版本
5. **SDK 路径**：确保 iOS SDK 路径配置正确
6. **代码签名**：iOS 应用可能需要代码签名才能运行
7. **测试验证**：添加支持后，务必在 iOS ARM64 设备上进行完整测试

