#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../../" && pwd)
DATA_DIR="$ROOT_DIR/data"

read -r -p "This will remove local data directory ($DATA_DIR). Continue? [y/N] " ans
if [[ "${ans:-N}" != "y" && "${ans:-N}" != "Y" ]]; then
  echo "Aborted."
  exit 0
fi

echo "[P2P] Removing $DATA_DIR ..."
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"
echo "[P2P] Cleaned."


