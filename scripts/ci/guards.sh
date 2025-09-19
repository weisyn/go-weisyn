#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")"/../.. && pwd)"
cd "$ROOT_DIR"

echo "[guard] scanning interfaces for struct/enum definitions..."
if command -v rg >/dev/null 2>&1; then
  if rg -n --no-heading --hidden --glob 'pkg/interfaces/**' -e '^type\s+\w+\s+struct\s*\{'; then
    echo "[guard] pkg/interfaces contains struct definitions. Please move to pkg/types." >&2
    exit 1
  fi
else
  if grep -RIn --include='*.go' '^type[[:space:]]\+[A-Za-z0-9_][A-Za-z0-9_]*[[:space:]]\+struct[[:space:]]*{' pkg/interfaces >/dev/null 2>&1; then
    echo "[guard] pkg/interfaces contains struct definitions. Please move to pkg/types." >&2
    exit 1
  fi
fi

echo "[guard] scanning pb for config-like messages..."
if command -v rg >/dev/null 2>&1; then
  if rg -n --no-heading --hidden --glob 'pb/**' -e 'message\s+.*Config\b' -e 'message\s+.*Configuration\b'; then
    echo "[guard] pb contains config-like messages. Use configs/ or types + adapters instead." >&2
    exit 1
  fi
else
  if grep -RIn --include='*.proto' -E 'message[[:space:]]+.*Config\b|message[[:space:]]+.*Configuration\b' pb >/dev/null 2>&1; then
    echo "[guard] pb contains config-like messages. Use configs/ or types + adapters instead." >&2
    exit 1
  fi
fi

echo "[guard] scanning types for pb usage..."
if command -v rg >/dev/null 2>&1; then
  if rg -n --no-heading --hidden --glob 'pkg/types/**' -e '"github.com/.*/pb/' -e '\.pb\.go'; then
    echo "[guard] pkg/types references pb packages or .pb.go. Use adapters instead." >&2
    exit 1
  fi
else
  if grep -RIn --include='*.go' -E '"github.com/.*/pb/' pkg/types >/dev/null 2>&1; then
    echo "[guard] pkg/types imports pb packages. Use adapters instead." >&2
    exit 1
  fi
  if grep -RIn --include='*.go' '\.pb\.go' pkg/types >/dev/null 2>&1; then
    echo "[guard] pkg/types references .pb.go. Use adapters instead." >&2
    exit 1
  fi
fi

echo "[guard] all checks passed"


