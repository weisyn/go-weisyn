#!/bin/bash
# ONNX Runtime 库文件下载脚本
# 下载 ONNX Runtime 官方提供的预编译库文件

set -e

VERSION="1.23.2"
BASE_DIR="pkg/build/deps/onnx/libs"

echo "📥 开始下载 ONNX Runtime v${VERSION} 预编译库文件..."
echo ""
echo "ℹ️  注意：ONNX Runtime 官方仅提供部分平台的预编译库"
echo "   其他平台需要从源码编译，详见文档说明"
echo ""

# 创建目录（仅创建实际有预编译库的平台）
mkdir -p ${BASE_DIR}/{darwin_amd64,darwin_arm64,linux_amd64,linux_arm64,windows_amd64,windows_arm64}

# macOS 平台
echo "📥 下载 macOS Intel (x86_64)..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-x86_64-${VERSION}.tgz | tar -xz
find onnxruntime-osx-x86_64-${VERSION}/lib -name "libonnxruntime*.dylib" -type f | head -1 | xargs -I {} cp {} ${BASE_DIR}/darwin_amd64/libonnxruntime.dylib
rm -rf onnxruntime-osx-x86_64-${VERSION}

echo "📥 下载 macOS Apple Silicon (arm64)..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-arm64-${VERSION}.tgz | tar -xz
find onnxruntime-osx-arm64-${VERSION}/lib -name "libonnxruntime*.dylib" -type f | head -1 | xargs -I {} cp {} ${BASE_DIR}/darwin_arm64/libonnxruntime.dylib
rm -rf onnxruntime-osx-arm64-${VERSION}

# Linux 平台
echo "📥 下载 Linux x64 (amd64)..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-x64-${VERSION}.tgz | tar -xz
cp onnxruntime-linux-x64-${VERSION}/lib/libonnxruntime.so.${VERSION} ${BASE_DIR}/linux_amd64/libonnxruntime.so
rm -rf onnxruntime-linux-x64-${VERSION}

echo "📥 下载 Linux ARM64 (aarch64)..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-aarch64-${VERSION}.tgz | tar -xz
cp onnxruntime-linux-aarch64-${VERSION}/lib/libonnxruntime.so.${VERSION} ${BASE_DIR}/linux_arm64/libonnxruntime.so
rm -rf onnxruntime-linux-aarch64-${VERSION}

# Windows 平台
echo "📥 下载 Windows x64 (amd64)..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-win-x64-${VERSION}.zip -o /tmp/onnx-win-x64.zip
unzip -q -j /tmp/onnx-win-x64.zip "onnxruntime-win-x64-${VERSION}/lib/onnxruntime.dll" -d ${BASE_DIR}/windows_amd64/ 2>/dev/null || echo "⚠️  Windows x64 下载失败"
rm -f /tmp/onnx-win-x64.zip

echo "📥 下载 Windows ARM64..."
curl -L https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-win-arm64-${VERSION}.zip -o /tmp/onnx-win-arm64.zip
unzip -q -j /tmp/onnx-win-arm64.zip "onnxruntime-win-arm64-${VERSION}/lib/onnxruntime.dll" -d ${BASE_DIR}/windows_arm64/ 2>/dev/null || echo "⚠️  Windows ARM64 下载失败"
rm -f /tmp/onnx-win-arm64.zip

echo ""
echo "✅ 下载完成！"
echo ""
echo "📊 已下载的文件:"
find ${BASE_DIR} -type f \( -name "libonnxruntime.*" -o -name "onnxruntime.dll" \) 2>/dev/null | while read file; do
    size=$(ls -lh "$file" 2>/dev/null | awk '{print $5}')
    echo "  $file ($size)"
done

echo ""
echo "ℹ️  说明："
echo "   - ONNX Runtime 官方仅提供 7 个平台的预编译库（v1.23.2）"
echo "   - 已下载所有可用的预编译库：darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64"
echo "   - 其他平台（linux-386, linux-arm, windows-386, android, ios 等）无预编译库，需要从源码编译"
