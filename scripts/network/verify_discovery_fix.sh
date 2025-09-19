#!/bin/bash

# 验证网络发现修复脚本
# 用途：测试修复后的网络配置是否能够正常发现局域网节点

set -e

echo "🔍 网络发现修复验证脚本"
echo "=================================="

# 验证配置文件
echo "📁 验证配置文件..."

configs=(
    "configs/development/cluster/node1.json"
    "configs/development/cluster/node2.json"
    "configs/development/single/config.json"
    "configs/production/config.json"
    "configs/testing/config.json"
)

for config in "${configs[@]}"; do
    if [ -f "$config" ]; then
        echo "✅ $config 存在"
        
        # 检查mDNS配置
        mdns_enabled=$(jq -r '.node.enable_mdns // false' "$config")
        if [ "$mdns_enabled" = "true" ]; then
            echo "   ✅ mDNS已启用"
        else
            echo "   ❌ mDNS未启用"
        fi
        
        # 检查监听地址
        listen_addrs=$(jq -r '.node.listen_addresses[]' "$config" | head -1)
        if [[ $listen_addrs == *"0.0.0.0"* ]]; then
            echo "   ✅ 监听地址配置正确 (0.0.0.0)"
        else
            echo "   ❌ 监听地址仍为本地绑定"
        fi
        
        # 检查引导节点
        bootstrap_count=$(jq -r '.node.bootstrap_peers | length' "$config")
        if [ "$bootstrap_count" -eq "9" ]; then
            echo "   ✅ 引导节点已配置 ($bootstrap_count 个，包含DNS、美国、亚洲节点)"
        elif [ "$bootstrap_count" -ge "5" ]; then
            echo "   ✅ 引导节点已配置 ($bootstrap_count 个，包含多地区节点)"
        elif [ "$bootstrap_count" -gt "0" ]; then
            echo "   ⚠️ 引导节点已配置 ($bootstrap_count 个，建议配置更多节点)"
        else
            echo "   ❌ 引导节点为空"
        fi
        
        # 检查AutoRelay配置
        auto_relay=$(jq -r '.node.enable_auto_relay // false' "$config")
        if [ "$auto_relay" = "true" ]; then
            echo "   ✅ 自动中继已启用 (改善连接性)"
        else
            echo "   ⚠️ 自动中继未启用"
        fi
        
        echo ""
    else
        echo "❌ $config 不存在"
    fi
done

echo "🧪 网络发现测试建议："
echo "1. 启动node1: make run CONFIG=configs/development/cluster/node1.json"
echo "2. 在另一终端启动node2: make run CONFIG=configs/development/cluster/node2.json"
echo "3. 观察日志中是否出现:"
echo "   - 'p2p.discovery.mdns started'"
echo "   - 'Connected to bootstrap peer'"  
echo "   - '🎉 gossipsub initialized successfully'"
echo "   - 发现其他节点的日志"

echo ""
echo "🔧 修复摘要："
echo "- ✅ 启用mDNS局域网发现"
echo "- ✅ 修复监听地址为0.0.0.0"
echo "- ✅ 添加完整引导节点配置 (9个节点)"
echo "- ✅ 启用自动中继 (AutoRelay)"
echo "- ✅ 保持DHT发现启用"

echo ""
echo "📡 引导节点详情 (9个节点)："
echo "官方DNS引导节点 (4个):"
echo "  - bootstrap.libp2p.io (QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN)"
echo "  - bootstrap.libp2p.io (QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa)"  
echo "  - bootstrap.libp2p.io (QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb)"
echo "  - bootstrap.libp2p.io (QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt)"
echo "美国节点 (1个):"
echo "  - 104.131.131.82:4001"
echo "亚洲节点 (4个):"
echo "  - 8.130.32.119:4001 (阿里云)"
echo "  - 47.245.56.181:4001 (阿里云)"
echo "  - 129.226.196.23:4001 (腾讯云)"
echo "  - 47.103.15.12:4001 (阿里云)"

echo ""
echo "📊 如果仍然无法发现节点，请检查："
echo "1. 防火墙是否阻止端口4001-4002"
echo "2. 网络是否在同一子网"
echo "3. 查看详细日志定位具体问题"
