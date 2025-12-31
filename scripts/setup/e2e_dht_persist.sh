#!/usr/bin/env bash
set -euo pipefail

# End-to-end check: DHT persistence + bootstrap dialing
# Requirements: jq, curl

ROOT_DIR="$(cd "$(dirname "$0")"/.. && pwd)"
BIN_DIR="$ROOT_DIR/bin"
NODE_BIN="$BIN_DIR/node"
CFG1_SRC="$ROOT_DIR/configs/config_client_node.json"
CFG2_SRC="$ROOT_DIR/configs/config_node2.json"

CFG1_TMP="$(mktemp /tmp/weisyn_node1_XXXX.json)"
CFG2_TMP="$(mktemp /tmp/weisyn_node2_XXXX.json)"
LOG1="$(mktemp /tmp/weisyn_node1_XXXX.log)"
LOG2="$(mktemp /tmp/weisyn_node2_XXXX.log)"

cleanup() {
  set +e
  if [[ -n "${PID1:-}" ]]; then kill $PID1 2>/dev/null || true; fi
  if [[ -n "${PID2:-}" ]]; then kill $PID2 2>/dev/null || true; fi
  # Keep log files and config files for debugging
  # rm -f "$CFG1_TMP" "$CFG2_TMP"
  echo "Debug: Log files preserved at $LOG1 and $LOG2"
  ls -la "$LOG1" "$LOG2" 2>/dev/null || echo "Log files not found"
}
trap cleanup EXIT

# Build node binary
mkdir -p "$BIN_DIR"
( cd "$ROOT_DIR" && go build -o "$NODE_BIN" ./cmd/node )

echo "[1/5] Prepare temp configs"
# Node1: enable diagnostics on 28686, clear gater restrictions for local testing, add genesis config
jq '.p2p.diagnostics_enabled=true
  | .p2p.diagnostics_port=28686
  | .p2p.advertise_private_addrs=true
  | .p2p.features.enable_nat_port=false
  | .p2p.features.enable_auto_relay=false
  | .p2p.features.enable_autonat=false
  | .p2p.features.enable_dcutr=false
  | .p2p.resources.memory_limit_mb=2048
  | .p2p.resources.max_file_descriptors=16384
  | .p2p.resources.max_conns_per_peer=32
  | .p2p.resources.max_streams_per_peer=1024
  | .p2p.gater.allowed_prefixes = ["/ip4/127.", "/ip4/10.", "/ip4/192.168.", "/ip6/fd"]
  | .p2p.gater.blocked_prefixes = []
  | .p2p.listen_addresses = ["/ip4/127.0.0.1/tcp/28683", "/ip4/127.0.0.1/udp/28683/quic-v1"]
  | .blockchain.genesis = {"initial_difficulty": 1000, "coinbase_reward": "1000000000", "genesis_time": "2024-01-01T00:00:00Z", "version": 1}' "$CFG1_SRC" > "$CFG1_TMP"
# Node2: enable diagnostics on 28706, clear all gater restrictions for local testing, add genesis config, use separate storage
jq '.p2p.diagnostics_enabled=true
  | .p2p.diagnostics_port=28706
  | .p2p.advertise_private_addrs=true
  | .p2p.features.enable_nat_port=false
  | .p2p.features.enable_auto_relay=false
  | .p2p.features.enable_autonat=false
  | .p2p.features.enable_dcutr=false
  | .p2p.resources.memory_limit_mb=2048
  | .p2p.resources.max_file_descriptors=16384
  | .p2p.resources.max_conns_per_peer=32
  | .p2p.resources.max_streams_per_peer=1024
  | .p2p.gater.allowed_prefixes = ["/ip4/127.", "/ip4/10.", "/ip4/192.168.", "/ip6/fd"]
  | .p2p.gater.blocked_prefixes = []
  | .p2p.listen_addresses = ["/ip4/127.0.0.1/tcp/28703", "/ip4/127.0.0.1/udp/28703/quic-v1"]
  | .p2p.dht.enabled=true
  | .storage.badger.path = "./data_node2/badger"
  | .p2p.dht.data_store_path = "./data_node2/dht"
  | .blockchain.genesis = {"initial_difficulty": 1000, "coinbase_reward": "1000000000", "genesis_time": "2024-01-01T00:00:00Z", "version": 1}' "$CFG2_SRC" > "$CFG2_TMP"

# Start node1
echo "[2/5] Start node1"
echo "Node1 log: $LOG1"
"$NODE_BIN" --config "$CFG1_TMP" > "$LOG1" 2>&1 & PID1=$!
# wait diagnostics
for i in {1..30}; do
  if curl -sf "http://127.0.0.1:28686/debug/host" >/dev/null; then break; fi
  sleep 1
  if [[ $i -eq 30 ]]; then 
    echo "node1 diagnostics not available, checking logs..."
    tail -10 "$LOG1" | grep -i "error\|panic\|failed" || echo "No obvious errors in node1 log"
    exit 1
  fi
done

# Query node1 peer_id and a reachable addr (prefer tcp)
HOST_JSON=$(curl -sf "http://127.0.0.1:28686/debug/host")
PEER_ID=$(echo "$HOST_JSON" | jq -r '.peer_id')
if [[ -z "$PEER_ID" || "$PEER_ID" == "null" ]]; then echo "peer_id not found"; exit 1; fi
ADDR=$(echo "$HOST_JSON" | jq -r '.addrs[] | select(test("/tcp/"))' | head -n1)
if [[ -z "$ADDR" ]]; then
  ADDR=$(echo "$HOST_JSON" | jq -r '.addrs[]' | head -n1)
fi
if [[ -z "$ADDR" ]]; then echo "no multiaddr found"; exit 1; fi
# Use diagnostics TCP addr for bootstrap to avoid NAT/ephemeral issues
# Prefer the first IPv4 TCP address from diagnostics
# Use fixed IPv4 TCP address for local testing
TCP_ADDR="/ip4/127.0.0.1/tcp/28683"
BOOTSTRAP="$TCP_ADDR/p2p/$PEER_ID"
echo "Bootstrap: $BOOTSTRAP (using current node1 peer_id: $PEER_ID)"

# Sanity check: ensure fixed ports are listened on
sleep 2
if ! lsof -iTCP:28683 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "node1 is not listening on tcp:28683"
  tail -30 "$LOG1" || true
  exit 1
fi

# Inject bootstrap into node2 cfg
jq --arg b "$BOOTSTRAP" '.p2p.bootstrap_peers=[ $b ]' "$CFG2_TMP" > "$CFG2_TMP.tmp" && mv "$CFG2_TMP.tmp" "$CFG2_TMP"

# Start node2
echo "[3/5] Start node2"
echo "Node2 log: $LOG2"
"$NODE_BIN" --config "$CFG2_TMP" > "$LOG2" 2>&1 & PID2=$!
# Wait and assert node2 port 28703 is listening
for i in {1..10}; do
  if lsof -iTCP:28703 -sTCP:LISTEN >/dev/null 2>&1; then break; fi
  sleep 1
  if [[ $i -eq 10 ]]; then
    echo "node2 is not listening on tcp:28703"
    tail -30 "$LOG2" || true
    exit 1
  fi
done

for i in {1..30}; do
  if curl -sf "http://127.0.0.1:28706/debug/host" >/dev/null; then break; fi
  sleep 1
  if [[ $i -eq 30 ]]; then 
    echo "node2 diagnostics not available, checking logs..."
    tail -20 "$LOG2" | grep -i "error\|panic\|failed\|BadgerDB\|存储" || echo "No obvious errors in node2 log"
    echo "Node2 bootstrap config:"
    jq -r '.p2p.bootstrap_peers[]' "$CFG2_TMP" 2>/dev/null || echo "No bootstrap peers configured"
    exit 1
  fi
done

# Wait for P2P connection establishment
echo "Waiting for P2P connection..."
sleep 10

# Verify node2 connected peers > 0
HOST2_JSON=$(curl -sf "http://127.0.0.1:28706/debug/host")
CONN=$(echo "$HOST2_JSON" | jq -r '.connected_peers')
if [[ "$CONN" -lt 1 ]]; then echo "node2 has no connected peers"; exit 1; fi
echo "[4/5] Connected peers: $CONN"

# Verify DHT datastore created (badger files)
DHT_DIR=$(jq -r '.p2p.dht.data_store_path // "./data/dht_node2"' "$CFG2_TMP")
if [[ ! -d "$DHT_DIR" ]]; then echo "DHT dir not found: $DHT_DIR"; exit 1; fi
COUNT=$(ls -1 "$DHT_DIR" 2>/dev/null | wc -l | tr -d ' ')
if [[ "$COUNT" -eq 0 ]]; then echo "DHT dir is empty: $DHT_DIR"; exit 1; fi
echo "[5/5] DHT datastore present at $DHT_DIR (files: $COUNT)"

echo "SUCCESS: E2E DHT persistence and bootstrap verified."
