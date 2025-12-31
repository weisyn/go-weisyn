#!/bin/bash
# ensure_onnx_libs.sh - 确保 ONNX Runtime 库文件存在
#
# 此脚本用于在构建前确保 ONNX Runtime 库文件已下载。
# 支持多个镜像源，自动重试，适用于 GitHub 被封禁的环境。
#
# 使用方法:
#   bash scripts/build/ensure_onnx_libs.sh
#   或
#   go generate ./pkg/build/deps/onnx

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ONNX Runtime 版本
ONNX_VERSION="1.18.0"

# 获取平台信息
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}❌ 不支持的架构: $ARCH${NC}"
        exit 1
        ;;
esac

PLATFORM="${OS}_${ARCH}"

# 平台配置
case "$PLATFORM" in
    darwin_amd64)
        ARCHIVE_NAME="onnxruntime-osx-x64-${ONNX_VERSION}"
        LIB_NAME="libonnxruntime.dylib"
        EXTRACT_PATH="${ARCHIVE_NAME}/lib/libonnxruntime.dylib"
        ;;
    darwin_arm64)
        ARCHIVE_NAME="onnxruntime-osx-arm64-${ONNX_VERSION}"
        LIB_NAME="libonnxruntime.dylib"
        EXTRACT_PATH="${ARCHIVE_NAME}/lib/libonnxruntime.dylib"
        ;;
    linux_amd64)
        ARCHIVE_NAME="onnxruntime-linux-x64-${ONNX_VERSION}"
        LIB_NAME="libonnxruntime.so"
        EXTRACT_PATH="${ARCHIVE_NAME}/lib/libonnxruntime.so"
        ;;
    linux_arm64)
        ARCHIVE_NAME="onnxruntime-linux-aarch64-${ONNX_VERSION}"
        LIB_NAME="libonnxruntime.so"
        EXTRACT_PATH="${ARCHIVE_NAME}/lib/libonnxruntime.so"
        ;;
    *)
        echo -e "${RED}❌ 不支持的平台: $PLATFORM${NC}"
        exit 1
        ;;
esac

# 目录设置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LIBS_DIR="${PROJECT_ROOT}/pkg/build/deps/onnx/libs/${PLATFORM}"
LIB_PATH="${LIBS_DIR}/${LIB_NAME}"

echo -e "${BLUE}📥 确保 ONNX Runtime 库文件存在${NC}"
echo -e "   平台: ${PLATFORM}"
echo -e "   版本: ${ONNX_VERSION}"
echo -e "   目标: ${LIB_PATH}"

# 检查库文件是否已存在
if [ -f "$LIB_PATH" ]; then
    FILE_SIZE=$(stat -f%z "$LIB_PATH" 2>/dev/null || stat -c%s "$LIB_PATH" 2>/dev/null || echo "0")
    if [ "$FILE_SIZE" -gt 1048576 ]; then  # > 1MB
        echo -e "${GREEN}✅ 库文件已存在: ${LIB_PATH} (大小: $(($FILE_SIZE / 1024 / 1024)) MB)${NC}"
        exit 0
    else
        echo -e "${YELLOW}⚠️  库文件存在但大小异常，将重新下载...${NC}"
        rm -f "$LIB_PATH"
    fi
fi

# 创建目录
mkdir -p "$LIBS_DIR"

# 镜像源列表（按优先级排序）
# 注意：ARCHIVE_NAME 已经包含版本号，不要重复追加
MIRRORS=(
    "https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${ARCHIVE_NAME}.tgz"
    "https://ghproxy.com/https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${ARCHIVE_NAME}.tgz"
    "https://ghps.cc/https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${ARCHIVE_NAME}.tgz"
    "https://mirror.ghproxy.com/https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${ARCHIVE_NAME}.tgz"
)

# 临时文件
TEMP_ARCHIVE="${LIBS_DIR}/temp_archive.tgz"

# 尝试从各个镜像源下载
for i in "${!MIRRORS[@]}"; do
    MIRROR="${MIRRORS[$i]}"
    MIRROR_NAME="镜像源 $((i + 1))"
    
    if [ $i -eq 0 ]; then
        MIRROR_NAME="GitHub (主源)"
    elif [[ "$MIRROR" == *"ghproxy.com"* ]]; then
        MIRROR_NAME="GitHub Proxy (ghproxy.com)"
    elif [[ "$MIRROR" == *"ghps.cc"* ]]; then
        MIRROR_NAME="GitHub Proxy (ghps.cc)"
    fi
    
    echo -e "\n${BLUE}[$((i + 1))/${#MIRRORS[@]}] 尝试从 ${MIRROR_NAME} 下载...${NC}"
    echo -e "   URL: ${MIRROR}"
    
    # 下载归档文件
    if curl -L --fail --progress-bar --connect-timeout 10 --max-time 300 \
        -o "$TEMP_ARCHIVE" "$MIRROR" 2>&1; then
        
        echo -e "   ${GREEN}✅ 下载成功${NC}"
        
        # 解压库文件
        echo -e "   ${BLUE}📦 解压归档文件...${NC}"
        if tar -xzf "$TEMP_ARCHIVE" -C "$LIBS_DIR" "$EXTRACT_PATH"; then
            # 移动文件到正确位置
            EXTRACTED_FILE="${LIBS_DIR}/${EXTRACT_PATH}"
            if [ -f "$EXTRACTED_FILE" ]; then
                mv "$EXTRACTED_FILE" "$LIB_PATH"
                chmod 755 "$LIB_PATH"
                
                # 清理临时文件和目录
                rm -f "$TEMP_ARCHIVE"
                rm -rf "${LIBS_DIR}/${ARCHIVE_NAME}"
                
                FILE_SIZE=$(stat -f%z "$LIB_PATH" 2>/dev/null || stat -c%s "$LIB_PATH" 2>/dev/null || echo "0")
                echo -e "   ${GREEN}✅ 提取成功 (大小: $(($FILE_SIZE / 1024 / 1024)) MB)${NC}"
                echo -e "\n${GREEN}✅ ONNX Runtime 库文件已下载: ${LIB_PATH}${NC}"
                exit 0
            else
                echo -e "   ${RED}❌ 解压后未找到库文件: ${EXTRACTED_FILE}${NC}"
            fi
        else
            echo -e "   ${RED}❌ 解压失败${NC}"
        fi
        
        # 清理临时文件
        rm -f "$TEMP_ARCHIVE"
    else
        echo -e "   ${RED}❌ 下载失败${NC}"
        if [ $i -lt $((${#MIRRORS[@]} - 1)) ]; then
            echo -e "   ${YELLOW}⏭️  尝试下一个镜像源...${NC}"
            sleep 1
        fi
    fi
done

# 所有镜像源都失败
echo -e "\n${RED}❌ 所有镜像源下载失败${NC}"
echo -e "\n${YELLOW}💡 手动下载方法:${NC}"
echo -e "   1. 访问: https://github.com/microsoft/onnxruntime/releases/tag/v${ONNX_VERSION}"
echo -e "   2. 下载对应平台的归档文件: ${ARCHIVE_NAME}-${ONNX_VERSION}.tgz"
echo -e "   3. 解压并将库文件放到: ${LIB_PATH}"
echo -e "   4. 或使用 Go generate: go generate ./pkg/build/deps/onnx"
echo -e "\n${YELLOW}💡 离线安装:${NC}"
echo -e "   如果网络受限，可以:"
echo -e "   1. 在其他有网络的机器上下载库文件"
echo -e "   2. 将库文件复制到: ${LIB_PATH}"
echo -e "   3. 确保文件权限: chmod 755 ${LIB_PATH}"

exit 1

