#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)"
cd "$ROOT_DIR"

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
    protoc \
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
