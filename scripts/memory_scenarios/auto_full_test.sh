#!/bin/bash
# 全自动内存测试脚本
# 功能：自动启动节点 + 初始化钱包 + 启动挖矿 + 发送交易
#
# 使用方法：
#   chmod +x scripts/memory_scenarios/auto_full_test.sh
#   ./scripts/memory_scenarios/auto_full_test.sh [--with-tx] [--duration <minutes>]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DATA_DIR="${PROJECT_ROOT}/data/memory-test/auto-full"
CONFIG_FILE="${PROJECT_ROOT}/data/memory-test/auto-full-config.json"
LOG_DIR="${DATA_DIR}/logs"
PROFILE_NAME="memory-test-auto"
TEST_PASSWORD="test123456"  # 测试用密码（仅用于测试环境）

WITH_TX=false
DURATION_MIN=30

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-tx)
            WITH_TX=true
            shift
            ;;
        --duration)
            DURATION_MIN="$2"
            shift 2
            ;;
        --clean)
            echo "清理旧数据..."
            rm -rf "${DATA_DIR}"
            rm -f "${CONFIG_FILE}"
            # 清理 profile（可选）
            # rm -rf ~/.wes/profiles/${PROFILE_NAME}.json
            echo "✅ 清理完成"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

echo "=========================================="
echo "全自动内存测试（节点 + 挖矿 + 交易）"
echo "=========================================="
echo "数据目录: ${DATA_DIR}"
echo "配置文件: ${CONFIG_FILE}"
echo "日志目录: ${LOG_DIR}"
echo "Profile: ${PROFILE_NAME}"
echo "测试时长: ${DURATION_MIN} 分钟"
echo "包含交易: ${WITH_TX}"
echo ""

# 创建目录
mkdir -p "${LOG_DIR}"

# 1. 生成私链配置（如果不存在）
if [ ! -f "${CONFIG_FILE}" ]; then
    echo "[1/6] 生成私链配置..."
    cd "${PROJECT_ROOT}"
    
    if [ -f "./bin/weisyn-node" ]; then
        ./bin/weisyn-node chain init --mode private --out "${CONFIG_FILE}" --force
    else
        go run ./cmd/node chain init --mode private --out "${CONFIG_FILE}" --force
    fi
    
    echo "✅ 配置已生成: ${CONFIG_FILE}"
    
    # 自动为所有 genesis 账户添加 public_key（从 private_key 推导）
    echo "  为 genesis 账户添加 public_key..."
    
    # 编译公钥推导工具（如果不存在）
    DERIVE_TOOL="${SCRIPT_DIR}/derive_pubkey"
    if [ ! -f "${DERIVE_TOOL}" ]; then
        cd "${SCRIPT_DIR}"
        go build -o derive_pubkey derive_pubkey.go 2>/dev/null || {
            echo "⚠️  编译公钥推导工具失败，跳过 public_key 添加"
            echo "   节点可能仍能启动（如果配置允许 PublicKey 为空）"
        }
    fi
    
    # 使用 Python 更新配置文件
    if [ -f "${DERIVE_TOOL}" ]; then
        CONFIG_FILE_VAR="${CONFIG_FILE}"
        DERIVE_TOOL_VAR="${DERIVE_TOOL}"
        python3 << PYTHON_SCRIPT
import json
import subprocess
import sys
import os

def derive_public_key_from_private(private_key_hex, derive_tool):
    """从私钥推导公钥（使用 Go 工具）"""
    try:
        result = subprocess.run(
            [derive_tool, private_key_hex],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            return result.stdout.strip()
        else:
            print(f"WARNING: 推导公钥失败: {result.stderr}", file=sys.stderr)
            return None
    except Exception as e:
        print(f"WARNING: 推导公钥异常: {e}", file=sys.stderr)
        return None

try:
    config_file = "${CONFIG_FILE_VAR}"
    derive_tool = "${DERIVE_TOOL_VAR}"
    
    with open(config_file, 'r') as f:
        config = json.load(f)
    
    accounts = config.get('genesis', {}).get('accounts', [])
    modified = False
    
    for acc in accounts:
        if 'private_key' in acc and acc['private_key'] and 'public_key' not in acc:
            private_key = acc['private_key']
            public_key = derive_public_key_from_private(private_key, derive_tool)
            if public_key:
                acc['public_key'] = public_key
                modified = True
    
    if modified:
        with open(config_file, 'w') as f:
            json.dump(config, f, indent=2, ensure_ascii=False)
        print("✅ 已为所有账户添加 public_key")
    else:
        print("ℹ️  所有账户已有 public_key 或没有 private_key")
except Exception as e:
    print(f"WARNING: 添加 public_key 失败: {e}", file=sys.stderr)
    print("   节点可能仍能启动（如果配置允许 PublicKey 为空）", file=sys.stderr)
PYTHON_SCRIPT
    fi
    
else
    echo "[1/6] 使用现有配置: ${CONFIG_FILE}"
fi

# 2. 提取 genesis 账户信息（从配置文件）
echo "[2/6] 提取 genesis 账户信息..."
GENESIS_ACCOUNTS=$(python3 -c "
import json
import sys
try:
    with open('${CONFIG_FILE}', 'r') as f:
        config = json.load(f)
    accounts = config.get('genesis', {}).get('accounts', [])
    if not accounts:
        print('ERROR: 配置文件中没有 genesis.accounts', file=sys.stderr)
        sys.exit(1)
    # 输出第一个账户的私钥和地址（JSON 格式）
    acc = accounts[0]
    print(json.dumps({
        'private_key': acc.get('private_key', ''),
        'address': acc.get('address', '')
    }))
except Exception as e:
    print(f'ERROR: {e}', file=sys.stderr)
    sys.exit(1)
" 2>&1)

if [ $? -ne 0 ]; then
    echo "❌ 提取 genesis 账户失败"
    echo "${GENESIS_ACCOUNTS}"
    exit 1
fi

GENESIS_PRIVATE_KEY=$(echo "${GENESIS_ACCOUNTS}" | python3 -c "import json, sys; print(json.load(sys.stdin)['private_key'])" 2>/dev/null)
GENESIS_ADDRESS=$(echo "${GENESIS_ACCOUNTS}" | python3 -c "import json, sys; print(json.load(sys.stdin)['address'])" 2>/dev/null)

if [ -z "${GENESIS_PRIVATE_KEY}" ] || [ -z "${GENESIS_ADDRESS}" ]; then
    echo "❌ 无法从配置文件中提取私钥或地址"
    echo "请确保配置文件包含 genesis.accounts[0].private_key 和 genesis.accounts[0].address"
    exit 1
fi

echo "✅ 提取成功:"
echo "   地址: ${GENESIS_ADDRESS}"
echo "   私钥: ${GENESIS_PRIVATE_KEY:0:16}..."

# 3. 启动节点（后台）
echo "[3/6] 启动节点（后台）..."
export WES_CLI_MODE=true

cd "${PROJECT_ROOT}"

NODE_PID_FILE="${LOG_DIR}/node.pid"

if [ -f "./bin/weisyn-node" ]; then
    ./bin/weisyn-node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" > "${LOG_DIR}/node-console.log" 2>&1 &
else
    go run ./cmd/node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" > "${LOG_DIR}/node-console.log" 2>&1 &
fi

NODE_PID=$!
echo "${NODE_PID}" > "${NODE_PID_FILE}"
echo "✅ 节点已启动 (PID: ${NODE_PID})"

# 4. 等待节点就绪（检查 HTTP 端口）
echo "[4/6] 等待节点就绪..."

# 从配置文件读取 HTTP 端口
HTTP_PORT=$(python3 -c "
import json
try:
    with open('${CONFIG_FILE}', 'r') as f:
        config = json.load(f)
    print(config.get('api', {}).get('http_port', 28680))
except:
    print('28680')
" 2>/dev/null || echo "28680")

MAX_WAIT=60
WAITED=0
NODE_READY=false

while [ ${WAITED} -lt ${MAX_WAIT} ]; do
    if curl -s "http://localhost:${HTTP_PORT}" > /dev/null 2>&1; then
        NODE_READY=true
        break
    fi
    sleep 1
    WAITED=$((WAITED + 1))
    echo -n "."
done
echo ""

if [ "${NODE_READY}" != "true" ]; then
    echo "❌ 节点启动超时（等待 ${MAX_WAIT} 秒，检查端口 ${HTTP_PORT}）"
    kill ${NODE_PID} 2>/dev/null || true
    exit 1
fi

echo "✅ 节点已就绪 (HTTP 端口: ${HTTP_PORT})"

# 5. 设置 CLI Profile 和导入账户
echo "[5/6] 设置 CLI Profile 和导入账户..."

# 获取 chain_id（从配置文件）
CHAIN_ID=$(python3 -c "
import json
with open('${CONFIG_FILE}', 'r') as f:
    config = json.load(f)
print(config.get('network', {}).get('chain_id', 'wes-local-1'))
" 2>/dev/null || echo "wes-local-1")

cd "${PROJECT_ROOT}"

# 创建或更新 profile
if [ -f "./bin/weisyn-cli" ]; then
    CLI_CMD="./bin/weisyn-cli"
else
    CLI_CMD="go run ./cmd/cli"
fi

# 检查 profile 是否存在
if ! ${CLI_CMD} profile show "${PROFILE_NAME}" > /dev/null 2>&1; then
    echo "  创建 Profile: ${PROFILE_NAME}"
    ${CLI_CMD} profile new "${PROFILE_NAME}" \
        --jsonrpc "http://localhost:${HTTP_PORT}" \
        --chain-id "${CHAIN_ID}" > /dev/null 2>&1 || true
fi

# 切换到 profile
${CLI_CMD} profile switch "${PROFILE_NAME}" > /dev/null 2>&1 || true

# 检查账户是否已存在
if ! ${CLI_CMD} account show "${GENESIS_ADDRESS}" > /dev/null 2>&1; then
    echo "  导入 genesis 账户..."
    # 使用 expect 或直接通过环境变量传递密码（如果 CLI 支持）
    # 如果 CLI 需要交互式输入，使用 expect 或 printf
    printf "%s\n%s\n" "${TEST_PASSWORD}" "${TEST_PASSWORD}" | ${CLI_CMD} account import "${GENESIS_PRIVATE_KEY}" \
        --password "${TEST_PASSWORD}" \
        --label "Genesis-Account" 2>&1 | grep -v "请输入\|请确认" || {
        # 如果失败，尝试不使用 --password 参数（让 CLI 从 stdin 读取）
        printf "%s\n%s\n" "${TEST_PASSWORD}" "${TEST_PASSWORD}" | ${CLI_CMD} account import "${GENESIS_PRIVATE_KEY}" \
            --label "Genesis-Account" 2>&1 | grep -v "请输入\|请确认" || {
            echo "⚠️  账户导入失败，但继续运行（账户可能已存在）"
        }
    }
else
    echo "  账户已存在，跳过导入"
fi

echo "✅ Profile 和账户已设置"

# 6. 启动挖矿
echo "[6/6] 启动挖矿..."
# 等待节点完全就绪（创世块初始化等）
sleep 3

MINING_STARTED=false
for i in 1 2 3 4 5; do
    if ${CLI_CMD} mining start --address "${GENESIS_ADDRESS}" > /dev/null 2>&1; then
        MINING_STARTED=true
        break
    fi
    echo "  重试 ${i}/5..."
    sleep 2
done

if [ "${MINING_STARTED}" != "true" ]; then
    echo "⚠️  挖矿启动失败（可能节点尚未完全就绪或挖矿 API 未启用）"
    echo "   继续运行测试（仅监控节点内存，不挖矿）"
else
    echo "✅ 挖矿已启动"
fi

# 7. 可选：发送交易
if [ "${WITH_TX}" = "true" ]; then
    echo ""
    echo "[7/7] 启动交易压测..."
    
    # 检查是否有第二个账户（用于接收）
    SECOND_ACCOUNT=$(python3 -c "
import json
with open('${CONFIG_FILE}', 'r') as f:
    config = json.load(f)
accounts = config.get('genesis', {}).get('accounts', [])
if len(accounts) >= 2:
    print(accounts[1].get('address', ''))
" 2>/dev/null || echo "")
    
    if [ -z "${SECOND_ACCOUNT}" ]; then
        echo "⚠️  配置文件中只有一个账户，无法进行转账测试"
        echo "   建议在配置文件中添加第二个 genesis 账户"
    else
        echo "   发送方: ${GENESIS_ADDRESS}"
        echo "   接收方: ${SECOND_ACCOUNT}"
        echo "   TPS: 1"
        echo "   时长: ${DURATION_MIN} 分钟"
        echo ""
        
        # 在后台运行交易压测
        (
            cd "${PROJECT_ROOT}"
            for i in $(seq 1 $((DURATION_MIN * 60))); do
                echo "${TEST_PASSWORD}" | ${CLI_CMD} tx build transfer "${SECOND_ACCOUNT}" 1wes --from "${GENESIS_ADDRESS}" > /dev/null 2>&1 && \
                ${CLI_CMD} tx send > /dev/null 2>&1 || true
                sleep 1
                
                if [ $((i % 60)) -eq 0 ]; then
                    echo "[$(date +%H:%M:%S)] 已发送 ${i} 笔交易"
                fi
            done
        ) &
        TX_PID=$!
        echo "✅ 交易压测已启动 (PID: ${TX_PID})"
    fi
fi

# 8. 等待测试完成
echo ""
echo "=========================================="
echo "✅ 测试环境已就绪"
echo "=========================================="
echo "节点 PID: ${NODE_PID}"
echo "测试时长: ${DURATION_MIN} 分钟"
echo "日志目录: ${LOG_DIR}"
echo ""
echo "监控内存采样:"
echo "  grep 'memory_sample' ${LOG_DIR}/node-system.log | tail -n 5"
echo ""
echo "停止测试:"
echo "  kill ${NODE_PID}"
if [ "${WITH_TX}" = "true" ] && [ -n "${TX_PID:-}" ]; then
    echo "  kill ${TX_PID}"
fi
echo ""

# 等待指定时长
echo "等待 ${DURATION_MIN} 分钟..."
sleep $((DURATION_MIN * 60))

# 9. 清理
echo ""
echo "=========================================="
echo "测试完成，正在清理..."
echo "=========================================="

if [ "${WITH_TX}" = "true" ] && [ -n "${TX_PID:-}" ]; then
    kill ${TX_PID} 2>/dev/null || true
    echo "✅ 交易压测已停止"
fi

${CLI_CMD} mining stop > /dev/null 2>&1 || true
echo "✅ 挖矿已停止"

kill ${NODE_PID} 2>/dev/null || true
sleep 2
kill -9 ${NODE_PID} 2>/dev/null || true
echo "✅ 节点已停止"

echo ""
echo "=========================================="
echo "✅ 测试完成"
echo "=========================================="
echo "日志文件: ${LOG_DIR}/node-system.log"
echo ""
echo "分析内存趋势:"
echo "  python3 scripts/analyze_memory_from_logs.py \\"
echo "    --log ${LOG_DIR}/node-system.log \\"
echo "    --output ./memory-report.csv \\"
echo "    --summary ./memory-summary.txt"

