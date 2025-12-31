#!/bin/bash
# WES 节点一键诊断脚本
#
# 用途：对单个节点快速生成一份自包含的 HTML 诊断报告
# 功能：
# 1. 调用 /api/v1/health/* 检查节点健康状态
# 2. 调用 /api/v1/system/diagnostics/summary 获取诊断汇总
# 3. 可选：下载 pprof heap/profile（如果启用）
# 4. 生成 HTML 报告（包含 L1/L2/L3 对应的信息）
#
# 使用方法（推荐与 dev-* 本地开发配置搭配使用）：
#   # 公链开发环境（dev-public-local，单机挖矿，本地诊断 + pprof）
#   # 节点启动命令：
#   #   go run ./cmd/node --chain public --config ./configs/chains/dev-public-local.json
#   # 诊断脚本：
#   ./scripts/diagnose_node.sh http://localhost:28680 http://127.0.0.1:28686 [输出路径]
#   例如：
#   ./scripts/diagnose_node.sh http://localhost:28680 http://127.0.0.1:28686 ./data/dev/dev-public-local/diagnostics/report.html
#   或使用默认路径（自动推断）：
#   ./scripts/diagnose_node.sh http://localhost:28680 http://127.0.0.1:28686
#   # 私链开发环境（dev-private-local）时，只需将输出目录切换为 dev-private-local：
#   #   ./scripts/diagnose_node.sh http://localhost:28680 http://127.0.0.1:28686 ./data/dev/dev-private-local/diagnostics/report.html
#
# 如果未指定输出路径，脚本会尝试推断：
#   1. 检查环境变量 DATA_DIR
#   2. 检查当前目录下是否存在 ./data/dev/dev-public-local 或 ./data/dev/dev-private-local
#   3. 如果都找不到，输出到 ./data/diagnostics/report.html

set -e

# 颜色定义（用于终端输出）
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
NODE_URL="${1:-http://localhost:28680}"
DIAGNOSTICS_URL="${2:-http://127.0.0.1:28686}"  # 诊断端口（与默认配置保持一致，默认为 28686）
OUTPUT_PATH="${3:-}"  # 输出路径（可选）
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# 自动推断输出路径（如果未指定）
if [ -z "$OUTPUT_PATH" ]; then
    # 优先使用环境变量
    if [ -n "$DATA_DIR" ]; then
        OUTPUT_PATH="$DATA_DIR/diagnostics/report.html"
    # 检查常见的开发环境数据目录
    elif [ -d "./data/dev/dev-public-local" ]; then
        OUTPUT_PATH="./data/dev/dev-public-local/diagnostics/report.html"
    elif [ -d "./data/dev/dev-private-local" ]; then
        OUTPUT_PATH="./data/dev/dev-private-local/diagnostics/report.html"
    else
        # 默认输出到 data/diagnostics
        OUTPUT_PATH="./data/diagnostics/report.html"
    fi
fi

# 确保输出目录存在
OUTPUT_DIR=$(dirname "$OUTPUT_PATH")
mkdir -p "$OUTPUT_DIR"

# 检查依赖
command -v curl >/dev/null 2>&1 || { echo -e "${RED}错误: 需要 curl 命令${NC}" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo -e "${YELLOW}警告: jq 未安装，JSON 格式化可能不完整${NC}" >&2; }

# 临时文件
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# 函数：获取 JSON 数据
fetch_json() {
    local url=$1
    local output=$2
    if curl -s -f "$url" > "$output" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# 函数：格式化字节数
format_bytes() {
    local bytes=$1
    if [ "$bytes" -gt 1073741824 ]; then
        echo "$(echo "scale=2; $bytes/1073741824" | bc) GB"
    elif [ "$bytes" -gt 1048576 ]; then
        echo "$(echo "scale=2; $bytes/1048576" | bc) MB"
    elif [ "$bytes" -gt 1024 ]; then
        echo "$(echo "scale=2; $bytes/1024" | bc) KB"
    else
        echo "${bytes} B"
    fi
}

# 函数：获取状态颜色
get_status_color() {
    local status=$1
    case "$status" in
        "ok"|"ready"|"healthy"|"true")
            echo "green"
            ;;
        "not_ready"|"unhealthy"|"false")
            echo "red"
            ;;
        *)
            echo "orange"
            ;;
    esac
}

echo -e "${BLUE}[1/4] 检查节点健康状态 (L1)...${NC}" >&2

# 1. 获取健康检查数据
HEALTH_LIVE_FILE="$TMP_DIR/health_live.json"
HEALTH_READY_FILE="$TMP_DIR/health_ready.json"
SUMMARY_FILE="$TMP_DIR/summary.json"

if fetch_json "$NODE_URL/api/v1/health/live" "$HEALTH_LIVE_FILE"; then
    echo -e "${GREEN}✅ Liveness check passed${NC}" >&2
else
    echo -e "${RED}❌ Liveness check failed${NC}" >&2
fi

if fetch_json "$NODE_URL/api/v1/health/ready" "$HEALTH_READY_FILE"; then
    echo -e "${GREEN}✅ Readiness check passed${NC}" >&2
else
    echo -e "${YELLOW}⚠️  Readiness check failed${NC}" >&2
fi

echo -e "${BLUE}[2/4] 获取诊断汇总 (L2+L3)...${NC}" >&2

# 2. 获取诊断汇总
if fetch_json "$NODE_URL/api/v1/system/diagnostics/summary" "$SUMMARY_FILE"; then
    echo -e "${GREEN}✅ Diagnostics summary retrieved${NC}" >&2
else
    echo -e "${RED}❌ Failed to get diagnostics summary${NC}" >&2
    SUMMARY_FILE=""
fi

echo -e "${BLUE}[3/4] 检查 pprof 可用性 (L4)...${NC}" >&2

# 3. 检查 pprof 是否可用
PPROF_AVAILABLE=false
if curl -s -f "$DIAGNOSTICS_URL/debug/pprof/" >/dev/null 2>&1; then
    PPROF_AVAILABLE=true
    echo -e "${GREEN}✅ pprof endpoints available${NC}" >&2
else
    echo -e "${YELLOW}⚠️  pprof endpoints not available (diagnostics_enabled=false?)${NC}" >&2
fi

echo -e "${BLUE}[4/4] 生成 HTML 报告...${NC}" >&2
echo -e "${BLUE}输出路径: $OUTPUT_PATH${NC}" >&2

# 4. 生成 HTML 报告（输出到文件）
{
cat <<EOF
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>WES 节点诊断报告 - $TIMESTAMP</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #f5f5f5;
            padding: 20px;
            line-height: 1.6;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 30px;
        }
        h1 {
            color: #333;
            border-bottom: 3px solid #4CAF50;
            padding-bottom: 10px;
            margin-bottom: 30px;
        }
        h2 {
            color: #555;
            margin-top: 30px;
            margin-bottom: 15px;
            padding-left: 10px;
            border-left: 4px solid #2196F3;
        }
        .section {
            margin-bottom: 30px;
        }
        .status-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: bold;
            margin-left: 10px;
        }
        .status-ok { background: #4CAF50; color: white; }
        .status-error { background: #f44336; color: white; }
        .status-warning { background: #ff9800; color: white; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 10px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background: #f8f9fa;
            font-weight: 600;
            color: #555;
        }
        tr:hover { background: #f5f5f5; }
        .metric-card {
            display: inline-block;
            background: #f8f9fa;
            padding: 15px 20px;
            margin: 10px 10px 10px 0;
            border-radius: 6px;
            border-left: 4px solid #2196F3;
            min-width: 150px;
        }
        .metric-label {
            font-size: 12px;
            color: #666;
            margin-bottom: 5px;
        }
        .metric-value {
            font-size: 24px;
            font-weight: bold;
            color: #333;
        }
        .code-block {
            background: #f4f4f4;
            padding: 15px;
            border-radius: 4px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            overflow-x: auto;
            margin-top: 10px;
        }
        .layer-badge {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: bold;
            margin-left: 5px;
        }
        .layer-l1 { background: #e3f2fd; color: #1976d2; }
        .layer-l2 { background: #fff3e0; color: #f57c00; }
        .layer-l3 { background: #f3e5f5; color: #7b1fa2; }
        .layer-l4 { background: #e8f5e9; color: #388e3c; }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            text-align: center;
            color: #666;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔍 WES 节点诊断报告</h1>
        <p style="color: #666; margin-bottom: 20px;">
            生成时间: <strong>$TIMESTAMP</strong><br>
            节点地址: <strong>$NODE_URL</strong>
        </p>

        <!-- L1: 健康检查 -->
        <div class="section">
            <h2><span class="layer-badge layer-l1">L1</span> 节点健康状态</h2>
EOF

# 解析健康检查数据
if [ -f "$HEALTH_LIVE_FILE" ]; then
    LIVE_STATUS=$(jq -r '.status // "unknown"' "$HEALTH_LIVE_FILE" 2>/dev/null || echo "unknown")
    LIVE_COLOR=$(get_status_color "$LIVE_STATUS")
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Liveness</div>"
    echo "                <div class=\"metric-value\"><span class=\"status-badge status-$LIVE_COLOR\">$LIVE_STATUS</span></div>"
    echo "            </div>"
fi

if [ -f "$HEALTH_READY_FILE" ]; then
    READY_STATUS=$(jq -r '.status // "unknown"' "$HEALTH_READY_FILE" 2>/dev/null || echo "unknown")
    READY_COLOR=$(get_status_color "$READY_STATUS")
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Readiness</div>"
    echo "                <div class=\"metric-value\"><span class=\"status-badge status-$READY_COLOR\">$READY_STATUS</span></div>"
    echo "            </div>"
fi

if [ -f "$SUMMARY_FILE" ]; then
    HEALTH_LIVE=$(jq -r '.health.live // false' "$SUMMARY_FILE" 2>/dev/null || echo "false")
    HEALTH_READY=$(jq -r '.health.ready // false' "$SUMMARY_FILE" 2>/dev/null || echo "false")
    
    if [ "$HEALTH_LIVE" = "true" ]; then
        echo "            <div class=\"metric-card\">"
        echo "                <div class=\"metric-label\">Live (Summary)</div>"
        echo "                <div class=\"metric-value\"><span class=\"status-badge status-ok\">✓</span></div>"
        echo "            </div>"
    fi
    
    if [ "$HEALTH_READY" = "true" ]; then
        echo "            <div class=\"metric-card\">"
        echo "                <div class=\"metric-label\">Ready (Summary)</div>"
        echo "                <div class=\"metric-value\"><span class=\"status-badge status-ok\">✓</span></div>"
        echo "            </div>"
    fi
fi

cat <<EOF
        </div>

        <!-- L2: 运行时资源 -->
        <div class="section">
            <h2><span class="layer-badge layer-l2">L2</span> 运行时资源统计</h2>
EOF

if [ -f "$SUMMARY_FILE" ]; then
    RSS_MB=$(jq -r '.runtime.rss_mb // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    HEAP_ALLOC=$(jq -r '.runtime.heap_alloc // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    GOROUTINES=$(jq -r '.runtime.num_goroutine // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    OPEN_FDS=$(jq -r '.runtime.open_fds // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    FD_LIMIT=$(jq -r '.runtime.fd_limit // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    
    FD_USAGE="0"
    if [ "$FD_LIMIT" -gt 0 ]; then
        FD_USAGE=$(echo "scale=1; $OPEN_FDS * 100 / $FD_LIMIT" | bc 2>/dev/null || echo "0")
    fi
    
    HEAP_MB=$(echo "scale=2; $HEAP_ALLOC / 1048576" | bc 2>/dev/null || echo "0")
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">RSS (物理内存)</div>"
    echo "                <div class=\"metric-value\">${RSS_MB} MB</div>"
    echo "            </div>"
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Heap Alloc</div>"
    echo "                <div class=\"metric-value\">${HEAP_MB} MB</div>"
    echo "            </div>"
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Goroutines</div>"
    echo "                <div class=\"metric-value\">$GOROUTINES</div>"
    echo "            </div>"
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">FD 使用率</div>"
    echo "                <div class=\"metric-value\">${OPEN_FDS}/${FD_LIMIT} (${FD_USAGE}%)</div>"
    echo "            </div>"
else
    echo "            <p style=\"color: #f44336;\">⚠️ 无法获取运行时资源数据</p>"
fi

cat <<EOF
        </div>

        <!-- L3: 模块内存占用 -->
        <div class="section">
            <h2><span class="layer-badge layer-l3">L3</span> Top 模块内存占用</h2>
EOF

if [ -f "$SUMMARY_FILE" ]; then
    MODULES_COUNT=$(jq '.modules_top | length' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    
    if [ "$MODULES_COUNT" -gt 0 ]; then
        echo "            <table>"
        echo "                <thead>"
        echo "                    <tr>"
        echo "                        <th>模块名称</th>"
        echo "                        <th>内存占用 (bytes)</th>"
        echo "                        <th>对象数量</th>"
        echo "                    </tr>"
        echo "                </thead>"
        echo "                <tbody>"
        
        jq -r '.modules_top[] | "<tr><td>\(.module)</td><td>\(.approx_bytes)</td><td>\(.objects)</td></tr>"' "$SUMMARY_FILE" 2>/dev/null || true
        
        echo "                </tbody>"
        echo "            </table>"
    else
        echo "            <p style=\"color: #666;\">暂无模块统计数据</p>"
    fi
else
    echo "            <p style=\"color: #f44336;\">⚠️ 无法获取模块统计数据</p>"
fi

cat <<EOF
        </div>

        <!-- P2P 简要信息 -->
        <div class="section">
            <h2>P2P 网络状态</h2>
EOF

if [ -f "$SUMMARY_FILE" ]; then
    P2P_PEERS=$(jq -r '.p2p_brief.peers // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    P2P_CONNECTIONS=$(jq -r '.p2p_brief.connections // 0' "$SUMMARY_FILE" 2>/dev/null || echo "0")
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Peers</div>"
    echo "                <div class=\"metric-value\">$P2P_PEERS</div>"
    echo "            </div>"
    
    echo "            <div class=\"metric-card\">"
    echo "                <div class=\"metric-label\">Connections</div>"
    echo "                <div class=\"metric-value\">$P2P_CONNECTIONS</div>"
    echo "            </div>"
else
    echo "            <p style=\"color: #f44336;\">⚠️ 无法获取 P2P 数据</p>"
fi

cat <<EOF
        </div>

        <!-- L4: pprof 指引 -->
        <div class="section">
            <h2><span class="layer-badge layer-l4">L4</span> 代码级分析 (pprof)</h2>
EOF

if [ "$PPROF_AVAILABLE" = "true" ]; then
    cat <<EOF
            <p>✅ pprof 端点已启用，可以使用以下命令进行深度分析：</p>
            <div class="code-block">
# 查看 heap 占用（火焰图）
go tool pprof -http=:28681 $DIAGNOSTICS_URL/debug/pprof/heap

# 查看 goroutine 分布
go tool pprof -http=:28681 $DIAGNOSTICS_URL/debug/pprof/goroutine

# CPU profile（30秒采样）
go tool pprof -http=:28681 $DIAGNOSTICS_URL/debug/pprof/profile?seconds=30

# 下载 profile 文件离线分析
curl -s $DIAGNOSTICS_URL/debug/pprof/heap > heap.out
go tool pprof heap.out
EOF
else
    cat <<EOF
            <p style="color: #ff9800;">⚠️ pprof 端点未启用</p>
            <p>要启用 pprof，请在配置文件中设置：</p>
            <div class="code-block">
{
  "node": {
    "host": {
      "diagnostics_enabled": true,
      "diagnostics_port": 28686
    }
  }
}
EOF
fi

cat <<EOF
        </div>

        <!-- 原始数据 -->
        <div class="section">
            <h2>原始数据 (JSON)</h2>
            <details>
                <summary style="cursor: pointer; color: #2196F3; margin-bottom: 10px;">点击展开原始 JSON 数据</summary>
                <div class="code-block">
EOF

if [ -f "$SUMMARY_FILE" ]; then
    jq '.' "$SUMMARY_FILE" 2>/dev/null || cat "$SUMMARY_FILE"
else
    echo "无法获取诊断汇总数据"
fi

cat <<EOF
                </div>
            </details>
        </div>

        <div class="footer">
            <p>WES 节点诊断报告 | 生成时间: $TIMESTAMP</p>
            <p style="margin-top: 10px; font-size: 11px; color: #999;">
                此报告基于 L1→L4 分层诊断模型生成<br>
                更多信息请参考: cmd/README.md 中的"标准排查流程"章节
            </p>
        </div>
    </div>
</body>
</html>
EOF
} > "$OUTPUT_PATH"

echo -e "${GREEN}✅ HTML 报告已生成: $OUTPUT_PATH${NC}" >&2
echo -e "${BLUE}💡 使用以下命令打开报告:${NC}" >&2
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo -e "   ${GREEN}open $OUTPUT_PATH${NC}" >&2
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo -e "   ${GREEN}xdg-open $OUTPUT_PATH${NC}" >&2
else
    echo -e "   ${GREEN}在浏览器中打开: file://$(realpath "$OUTPUT_PATH" 2>/dev/null || echo "$OUTPUT_PATH")${NC}" >&2
fi
