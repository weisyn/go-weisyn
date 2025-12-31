#!/usr/bin/env bash
set -euo pipefail

echo "🔍 开始静态代码检查..."

# 1. 自定义规则检查（项目特定的代码模式检查）
echo "[1/2] 运行自定义规则检查..."

# 禁止在交易/区块哈希中使用本地 proto.Marshal + SHA256 实现（白名单放行 infra/crypto）
if rg -n "proto\.Marshal\(.*\)" --glob '!internal/core/infrastructure/crypto/**' | rg -n "SHA256\(" >/dev/null; then
  echo "[STATIC CHECK] ❌ Detected proto.Marshal + SHA256 combination outside whitelist. Use txHashService/BlockHashService instead." >&2
  exit 1
fi

# 禁止手写 OutPoint 键（白名单放行 pkg/utils/transaction.go）
if rg -n "fmt\.Sprintf\(\"%x:%d\"" --glob '!pkg/utils/transaction.go' >/dev/null; then
  echo "[STATIC CHECK] ❌ Detected manual OutPoint key formatting. Use utils.OutPointKey/UTXOKey instead." >&2
  exit 1
fi

# 禁止直接 string(TxId) 用于比较/键
if rg -n "string\(.*TxId\)" --glob '!**/*_test.go' >/dev/null; then
  echo "[STATIC CHECK] ❌ Detected string(TxId) usage. Use byte-wise compare or common keys instead." >&2
  exit 1
fi

echo "[STATIC CHECK] ✅ 自定义规则检查通过"

# 2. golangci-lint 检查（如果已安装）
echo "[2/2] 运行 golangci-lint 检查..."

GOLANGCI_LINT=""
if [ -f "./bin/golangci-lint" ]; then
  GOLANGCI_LINT="./bin/golangci-lint"
elif command -v golangci-lint >/dev/null 2>&1; then
  GOLANGCI_LINT="golangci-lint"
fi

if [ -n "$GOLANGCI_LINT" ]; then
  echo "✅ 使用 golangci-lint 进行代码质量检查..."
  $GOLANGCI_LINT run
  echo "[STATIC CHECK] ✅ golangci-lint 检查通过"
else
  echo "⚠️  golangci-lint 未安装，跳过 golangci-lint 检查"
  echo "💡 提示: 运行 'make install-lint-tools' 安装 golangci-lint 以获得更全面的检查"
  echo "[STATIC CHECK] ⚠️  仅运行了自定义规则检查"
fi

echo ""
echo "✅ [STATIC CHECK] 所有检查通过"


