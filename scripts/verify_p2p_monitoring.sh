#!/bin/bash
# P2P 监控系统验证脚本
#
# 用途：验证 P2P 模块的 Prometheus 指标和 HTTP 端点可用性
# 包括：
# 1. 编译检查
# 2. 单元测试
# 3. HTTP 端点可用性测试
# 4. Prometheus 指标格式验证

set -e

echo "=========================================="
echo "P2P 监控系统验证脚本"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 检查端口是否可用
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
        echo -e "${YELLOW}警告: 端口 $port 已被占用${NC}"
        return 1
    fi
    return 0
}

echo -e "${BLUE}[1/5] 检查代码编译...${NC}"
if ! go build ./internal/core/p2p/... 2>&1; then
    echo -e "${RED}❌ 编译失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 编译通过${NC}"
echo ""

echo -e "${BLUE}[2/5] 运行 P2P Diagnostics 单元测试...${NC}"
if ! go test ./internal/core/p2p/diagnostics -v -run TestService 2>&1; then
    echo -e "${RED}❌ 单元测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 单元测试通过${NC}"
echo ""

echo -e "${BLUE}[3/5] 验证 Prometheus 指标注册...${NC}"
# 运行指标注册测试
if ! go test ./internal/core/p2p/diagnostics -v -run TestService_RegisterMetrics 2>&1; then
    echo -e "${RED}❌ 指标注册测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Prometheus 指标注册验证通过${NC}"
echo ""

echo -e "${BLUE}[4/5] 验证 HTTP 端点可用性...${NC}"
# 运行 HTTP 端点测试
if ! go test ./internal/core/p2p/diagnostics -v -run TestService_HTTPEndpoints 2>&1; then
    echo -e "${RED}❌ HTTP 端点测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ HTTP 端点验证通过${NC}"
echo ""

echo -e "${BLUE}[5/5] 验证 Prometheus 指标内容...${NC}"
# 运行指标内容测试
if ! go test ./internal/core/p2p/diagnostics -v -run TestService_MetricsEndpoint_Content 2>&1; then
    echo -e "${RED}❌ 指标内容测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Prometheus 指标内容验证通过${NC}"
echo ""

echo "=========================================="
echo -e "${GREEN}✅ 所有验证通过！${NC}"
echo "=========================================="
echo ""
echo "可用的 HTTP 端点："
echo "  - http://<diagnostics_addr>/metrics              (Prometheus 指标)"
echo "  - http://<diagnostics_addr>/debug/p2p/peers     (Peer 列表)"
echo "  - http://<diagnostics_addr>/debug/p2p/connections (连接信息)"
echo "  - http://<diagnostics_addr>/debug/p2p/stats     (统计信息)"
echo "  - http://<diagnostics_addr>/debug/p2p/health    (健康检查)"
echo "  - http://<diagnostics_addr>/debug/p2p/routing   (路由信息)"
echo ""
echo "Prometheus 指标列表："
echo "  - p2p_connections_total                          (连接数)"
echo "  - p2p_peers_total                                (Peer 数)"
echo "  - p2p_bandwidth_in_rate_bytes_per_sec            (入站带宽速率)"
echo "  - p2p_bandwidth_out_rate_bytes_per_sec           (出站带宽速率)"
echo "  - p2p_bandwidth_in_total_bytes                  (入站总字节数)"
echo "  - p2p_bandwidth_out_total_bytes                 (出站总字节数)"
echo "  - p2p_discovery_bootstrap_attempt_total          (Bootstrap 尝试次数)"
echo "  - p2p_discovery_bootstrap_success_total          (Bootstrap 成功次数)"
echo "  - p2p_discovery_mdns_peer_found_total            (mDNS 发现的 Peer 数)"
echo "  - p2p_discovery_mdns_connect_success_total       (mDNS 连接成功数)"
echo "  - p2p_discovery_mdns_connect_fail_total          (mDNS 连接失败数)"
echo "  - p2p_discovery_last_bootstrap_unixtime          (最后 Bootstrap 时间戳)"
echo "  - p2p_discovery_last_mdns_found_unixtime         (最后 mDNS 发现时间戳)"
echo ""

