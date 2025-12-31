# Android ARM32v7 (arm) 平台支持

本目录用于存放 Android ARM32v7 平台的 ONNX Runtime 库文件和嵌入代码。

## 📋 平台信息

- **操作系统**: Android
- **架构**: ARM32v7 (armeabi-v7a)
- **库文件名**: `libonnxruntime.so`
- **状态**: ⚠️ 需从源码编译（交叉编译，无预编译库）

## 🔧 如何启用支持

### 步骤 1: 从源码编译 ONNX Runtime（交叉编译）

ONNX Runtime 官方不提供此平台的预编译库，需要从源码交叉编译。

**编译环境要求**：
- Linux 或 macOS 主机系统
- Android NDK（推荐 r21+）
- CMake 3.18+
- Python 3.6+（用于构建脚本）

**编译命令**：
```bash
# 克隆 ONNX Runtime 仓库
git clone --recursive https://github.com/microsoft/onnxruntime.git
cd onnxruntime

# 配置 Android NDK 路径
export ANDROID_NDK_HOME=/path/to/android-ndk

# 配置构建（Android ARM32v7）
./build.sh --config Release --build_shared_lib \
  --android --android_abi armeabi-v7a --android_api 29

# 编译后的库文件位置
# build/Android/Release/libonnxruntime.so
```

**详细参数说明**：
- `--android`: 启用 Android 构建
- `--android_abi armeabi-v7a`: 指定 ARM32v7 架构
- `--android_api 29`: 指定 Android API 级别（建议 29+）

### 步骤 2: 复制库文件

将编译好的库文件复制到此目录：

```bash
cp build/Android/Release/libonnxruntime.so pkg/build/deps/onnx/libs/android_arm/libonnxruntime.so
```

### 步骤 3: 启用嵌入代码

编辑 `embedded.go` 文件，取消注释：

**修改前**：
```go
// 需要从源码编译 ONNX Runtime（交叉编译），编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibAndroidARM []byte

// func init() {
// 	libAndroidARM = embeddedLibAndroidARM
// }
```

**修改后**：
```go
//go:embed libonnxruntime.so
var embeddedLibAndroidARM []byte

func init() {
	libAndroidARM = embeddedLibAndroidARM
}
```

### 步骤 4: 更新主文件

编辑 `pkg/build/deps/onnx/embedded.go`，在 `getEmbeddedLibrary()` 函数中添加对应的 `case` 分支：

```go
case "android_arm":
    if len(libAndroidARM) == 0 {
        return nil, fmt.Errorf("嵌入的库文件为空 (android_arm)。请参考 libs/android_arm/embedded.go")
    }
    return libAndroidARM, nil
```

### 步骤 5: 验证

```bash
# 重新构建
go build ./cmd/weisyn

# 测试运行（需要在 Android 设备或模拟器上）
go run ./cmd/weisyn
```

## 📚 相关资源

- [ONNX Runtime 构建文档](https://onnxruntime.ai/docs/build/)
- [ONNX Runtime GitHub](https://github.com/microsoft/onnxruntime)
- [Android NDK 下载](https://developer.android.com/ndk/downloads)
- [平台支持说明](../../README.md)

## ⚠️ 注意事项

1. **版本一致性**：编译的库文件版本应与预编译库版本一致（当前为 v1.23.2）
2. **文件命名**：确保库文件名为 `libonnxruntime.so`
3. **NDK 版本**：建议使用 Android NDK r21 或更高版本
4. **API 级别**：建议使用 Android API 29 或更高版本
5. **交叉编译**：确保 Android NDK 路径配置正确
6. **测试验证**：添加支持后，务必在 Android ARM32v7 设备上进行完整测试

