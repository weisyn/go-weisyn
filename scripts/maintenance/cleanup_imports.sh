#!/bin/bash

# 清理未使用的导入
set -e

echo "清理未使用的导入..."

# 查找所有Go文件并移除未使用的导入
find . -name "*.go" -not -path "./pb/*" -not -path "./vendor/*" | while read file; do
    echo "处理文件: $file"
    
    # 移除未使用的resource导入
    if grep -q 'resource "github.com/weisyn/v1/pb/blockchain/resource"' "$file" && ! grep -q '\bresource\.' "$file"; then
        sed -i '' '/resource "github.com\/vidchain\/WES\/pb\/blockchain\/resource"/d' "$file"
    fi
    
    # 移除未使用的utxo导入
    if grep -q 'utxo "github.com/weisyn/v1/pb/blockchain/utxo"' "$file" && ! grep -q '\butxo\.' "$file"; then
        sed -i '' '/utxo "github.com\/vidchain\/WES\/pb\/blockchain\/utxo"/d' "$file"
    fi
    
    # 移除未使用的execution导入
    if grep -q 'execution "github.com/weisyn/v1/pb/blockchain/execution"' "$file" && ! grep -q '\bexecution\.' "$file"; then
        sed -i '' '/execution "github.com\/vidchain\/WES\/pb\/blockchain\/execution"/d' "$file"
    fi
    
    # 移除未使用的core导入
    if grep -q 'core "github.com/weisyn/v1/pb/blockchain/core"' "$file" && ! grep -q '\bcore\.' "$file"; then
        sed -i '' '/core "github.com\/vidchain\/WES\/pb\/blockchain\/core"/d' "$file"
    fi
done

echo "清理完成！" 