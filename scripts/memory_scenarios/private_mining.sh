#!/bin/bash
# 场景 2：本地私链 + 单节点挖矿测试
# 目的：在可控环境中跑"挖矿 + 全工作流"，避免主网因素干扰
#
# 运行时长建议：
#   - 快速验证：10-30 分钟
#   - 稳定性测试：2-24 小时
#
# 使用方法：
#   chmod +x scripts/memory_scenarios/private_mining.sh
#   ./scripts/memory_scenarios/private_mining.sh [--auto-mining]
#
# 参数：
#   --auto-mining: 自动启动挖矿（需要配置文件中有 genesis 账户私钥）
#   --clean: 清理旧数据

set -e

AUTO_MINING=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --auto-mining)
            AUTO_MINING=true
            shift
            ;;
        --clean)
            echo "清理旧数据..."
            rm -rf "${PROJECT_ROOT}/data/memory-test/private-mining"
            rm -f "${PROJECT_ROOT}/data/memory-test/private-mining-config.json"
            echo "✅ 清理完成"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DATA_DIR="${PROJECT_ROOT}/data/memory-test/private-mining"
CONFIG_FILE="${PROJECT_ROOT}/data/memory-test/private-mining-config.json"
LOG_DIR="${DATA_DIR}/logs"
PROFILE_NAME="memory-test-mining"
TEST_PASSWORD="test123456"

echo "=========================================="
echo "场景 2：私链挖矿内存测试"
echo "=========================================="
echo "数据目录: ${DATA_DIR}"
echo "配置文件: ${CONFIG_FILE}"
echo "日志目录: ${LOG_DIR}"
echo "自动挖矿: ${AUTO_MINING}"
echo ""

# 创建目录
mkdir -p "${LOG_DIR}"

# 生成私链配置（如果不存在）
if [ ! -f "${CONFIG_FILE}" ]; then
    echo "生成私链配置模板..."
    cd "${PROJECT_ROOT}"
    
    if [ -f "./bin/weisyn-node" ]; then
        ./bin/weisyn-node chain init --mode private --out "${CONFIG_FILE}" --force
    else
        go run ./cmd/node chain init --mode private --out "${CONFIG_FILE}" --force
    fi
    
    echo "✅ 配置已生成: ${CONFIG_FILE}"
    echo "⚠️  请检查配置文件，确保 genesis.accounts 有至少一个账户"
    if [ "${AUTO_MINING}" = "false" ]; then
        echo "   按 Enter 继续，或 Ctrl+C 退出后编辑配置..."
        read
    fi
fi

# 设置环境变量：日志只写入文件，不刷屏
export WES_CLI_MODE=true

cd "${PROJECT_ROOT}"

# 如果启用自动挖矿，需要后台启动节点
if [ "${AUTO_MINING}" = "true" ]; then
    echo "启动节点（后台模式，自动挖矿）..."
    
    # 启动节点（后台）
    if [ -f "./bin/weisyn-node" ]; then
        ./bin/weisyn-node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" > "${LOG_DIR}/node-console.log" 2>&1 &
    else
        go run ./cmd/node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" > "${LOG_DIR}/node-console.log" 2>&1 &
    fi
    
    NODE_PID=$!
    echo "✅ 节点已启动 (PID: ${NODE_PID})"
    
    # 等待节点就绪
    echo "等待节点就绪..."
    MAX_WAIT=60
    WAITED=0
    NODE_READY=false
    
    while [ ${WAITED} -lt ${MAX_WAIT} ]; do
        if curl -s http://localhost:28680 > /dev/null 2>&1; then
            NODE_READY=true
            break
        fi
        sleep 1
        WAITED=$((WAITED + 1))
        echo -n "."
    done
    echo ""
    
    if [ "${NODE_READY}" != "true" ]; then
        echo "❌ 节点启动超时"
        kill ${NODE_PID} 2>/dev/null || true
        exit 1
    fi
    
    echo "✅ 节点已就绪"
    
    # 提取 genesis 账户并导入
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
    acc = accounts[0]
    print(json.dumps({
        'private_key': acc.get('private_key', ''),
        'address': acc.get('address', '')
    }))
except Exception as e:
    print(f'ERROR: {e}', file=sys.stderr)
    sys.exit(1)
" 2>&1)
    
    if [ $? -eq 0 ]; then
        GENESIS_PRIVATE_KEY=$(echo "${GENESIS_ACCOUNTS}" | python3 -c "import json, sys; print(json.load(sys.stdin)['private_key'])" 2>/dev/null)
        GENESIS_ADDRESS=$(echo "${GENESIS_ACCOUNTS}" | python3 -c "import json, sys; print(json.load(sys.stdin)['address'])" 2>/dev/null)
        
        if [ -n "${GENESIS_PRIVATE_KEY}" ] && [ -n "${GENESIS_ADDRESS}" ]; then
            # 设置 CLI Profile
            if [ -f "./bin/weisyn-cli" ]; then
                CLI_CMD="./bin/weisyn-cli"
            else
                CLI_CMD="go run ./cmd/cli"
            fi
            
            CHAIN_ID=$(python3 -c "
import json
with open('${CONFIG_FILE}', 'r') as f:
    config = json.load(f)
print(config.get('network', {}).get('chain_id', 'wes-local-1'))
" 2>/dev/null || echo "wes-local-1")
            
            # 创建或切换 profile
            if ! ${CLI_CMD} profile show "${PROFILE_NAME}" > /dev/null 2>&1; then
                ${CLI_CMD} profile new "${PROFILE_NAME}" \
                    --jsonrpc http://localhost:28680 \
                    --chain-id "${CHAIN_ID}" > /dev/null 2>&1 || true
            fi
            ${CLI_CMD} profile switch "${PROFILE_NAME}" > /dev/null 2>&1 || true
            
            # 导入账户（如果不存在）
            if ! ${CLI_CMD} account show "${GENESIS_ADDRESS}" > /dev/null 2>&1; then
                echo "导入 genesis 账户..."
                echo "${TEST_PASSWORD}" | ${CLI_CMD} account import "${GENESIS_PRIVATE_KEY}" \
                    --password "${TEST_PASSWORD}" \
                    --label "Genesis-Account" > /dev/null 2>&1 || true
            fi
            
            # 启动挖矿
            echo "启动挖矿..."
            ${CLI_CMD} mining start --address "${GENESIS_ADDRESS}" > /dev/null 2>&1 || {
                sleep 3
                ${CLI_CMD} mining start --address "${GENESIS_ADDRESS}" > /dev/null 2>&1 || true
            }
            echo "✅ 挖矿已启动"
        fi
    fi
    
    echo ""
    echo "=========================================="
    echo "✅ 节点和挖矿已启动"
    echo "=========================================="
    echo "节点 PID: ${NODE_PID}"
    echo "日志文件: ${LOG_DIR}/node-system.log"
    echo ""
    echo "停止测试: kill ${NODE_PID}"
    echo ""
    
    # 等待用户中断
    trap "echo ''; echo '正在停止...'; ${CLI_CMD} mining stop > /dev/null 2>&1 || true; kill ${NODE_PID} 2>/dev/null || true; exit 0" INT TERM
    wait ${NODE_PID}
else
    # 前台模式（手动挖矿）
    echo "启动节点（私链模式）..."
    echo "  日志文件: ${LOG_DIR}/node-system.log"
    echo "  按 Ctrl+C 停止节点"
    echo ""
    echo "⚠️  提示：节点启动后，在另一个终端运行以下命令启动挖矿："
    echo "   go run ./cmd/cli mining start"
    echo "   或"
    echo "   ./bin/weisyn-cli mining start"
    echo ""
    
    # 使用 go run 或编译后的二进制
    if [ -f "./bin/weisyn-node" ]; then
        ./bin/weisyn-node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" 2>&1 | tee "${LOG_DIR}/node-console.log"
    else
        go run ./cmd/node --chain private --config "${CONFIG_FILE}" --data-dir "${DATA_DIR}" 2>&1 | tee "${LOG_DIR}/node-console.log"
    fi
fi

