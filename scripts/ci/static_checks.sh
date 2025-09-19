#!/usr/bin/env bash
set -euo pipefail

# 禁止在交易/区块哈希中使用本地 proto.Marshal + SHA256 实现（白名单放行 infra/crypto）
if rg -n "proto\.Marshal\(.*\)" --glob '!internal/core/infrastructure/crypto/**' | rg -n "SHA256\(" >/dev/null; then
  echo "[STATIC CHECK] Detected proto.Marshal + SHA256 combination outside whitelist. Use txHashService/BlockHashService instead." >&2
  exit 1
fi

# 禁止手写 OutPoint 键（白名单放行 pkg/utils/transaction.go）
if rg -n "fmt\.Sprintf\(\"%x:%d\"" --glob '!pkg/utils/transaction.go' >/dev/null; then
  echo "[STATIC CHECK] Detected manual OutPoint key formatting. Use utils.OutPointKey/UTXOKey instead." >&2
  exit 1
fi

# 禁止直接 string(TxId) 用于比较/键
if rg -n "string\(.*TxId\)" --glob '!**/*_test.go' >/dev/null; then
  echo "[STATIC CHECK] Detected string(TxId) usage. Use byte-wise compare or common keys instead." >&2
  exit 1
fi

echo "[STATIC CHECK] Passed"


