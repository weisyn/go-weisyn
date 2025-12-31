#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)"
cd "$ROOT_DIR"

# 检测并处理架构不匹配问题（ARM64 protoc vs x86_64 shell）
PROTOC_CMD="protoc"
if command -v protoc >/dev/null 2>&1; then
  PROTOC_PATH="$(command -v protoc)"
  # 检查protoc的架构
  PROTOC_ARCH="$(file "$PROTOC_PATH" 2>/dev/null | grep -oE 'arm64|x86_64|i386' || echo 'unknown')"
  CURRENT_ARCH="$(uname -m)"
  
  if [ "$PROTOC_ARCH" = "arm64" ] && [ "$CURRENT_ARCH" = "x86_64" ]; then
    # protoc是ARM64但shell是x86_64，无法直接执行
    echo "[proto] ❌ 架构不匹配错误：protoc是ARM64架构，但当前shell是x86_64架构"
    echo ""
    echo "[proto] 📋 解决方案（选择其一）："
    echo ""
    echo "[proto] 方案1：安装x86_64版本的protoc（推荐）"
    echo "[proto]   在x86_64终端中运行："
    echo "[proto]   arch -x86_64 /bin/bash -c \"brew install protobuf\""
    echo ""
    echo "[proto] 方案2：切换到ARM64终端"
    echo "[proto]   1. 打开新的终端窗口"
    echo "[proto]   2. 运行: arch -arm64 zsh  （如果您的Mac支持ARM64）"
    echo "[proto]   3. 然后重新执行此脚本"
    echo ""
    echo "[proto] 方案3：使用Docker（如果已安装Docker）"
    echo "[proto]   使用包含protoc的Docker镜像来生成代码"
    echo ""
    exit 1
  fi
fi

# 确保 GOPATH/bin 在 PATH 中（protoc-gen-go 需要）
if command -v go >/dev/null 2>&1; then
  GOPATH_BIN="$(go env GOPATH)/bin"
  if [[ -d "$GOPATH_BIN" ]] && [[ ":$PATH:" != *":$GOPATH_BIN:"* ]]; then
    export PATH="$GOPATH_BIN:$PATH"
    echo "[proto] Added $GOPATH_BIN to PATH"
  fi
fi

# Generate all protobufs

# 自动发现 pb 目录下的所有 .proto 文件
echo "[proto] Discovering .proto files in pb/ directory..."
PROTO_FILES=($(find pb -name "*.proto" -type f | sort))

if [ ${#PROTO_FILES[@]} -eq 0 ]; then
  echo "[proto] No .proto files found in pb/ directory"
  exit 1
fi

echo "[proto] Found ${#PROTO_FILES[@]} .proto files:"
for f in "${PROTO_FILES[@]}"; do
  echo "  - $f"
done

echo "[proto] Generating Go code..."
for f in "${PROTO_FILES[@]}"; do
  if [[ -f "$f" ]]; then
    echo "[proto] Processing: $f"
    $PROTOC_CMD \
      --go_out=. \
      --go_opt=paths=source_relative \
      --go-grpc_out=. \
      --go-grpc_opt=paths=source_relative \
      "$f"
  else
    echo "[proto] Warning: File not found: $f"
  fi
done

echo "[proto] Generated successfully!"
echo "[proto] Total files processed: ${#PROTO_FILES[@]}"
