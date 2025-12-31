#!/bin/bash

# 简单的批量替换脚本
set -e

echo "更新导入路径..."

# 查找所有使用旧导入的文件并替换为通用导入
find . -name "*.go" -not -path "./pb/*" -not -path "./vendor/*" -exec grep -l "github.com/weisyn/v1/pb/blockchain[^/]" {} \; | while read file; do
    echo "处理文件: $file"
    
    # 先替换导入
    sed -i '' 's|pb "github.com/weisyn/v1/pb/blockchain"|core "github.com/weisyn/v1/pb/blockchain/core"\
	resource "github.com/weisyn/v1/pb/blockchain/resource"\
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"\
	execution "github.com/weisyn/v1/pb/blockchain/execution"|g' "$file"
    
    # 替换常用类型引用
    sed -i '' \
        -e 's/pb\.Transaction/core.Transaction/g' \
        -e 's/pb\.Block/core.Block/g' \
        -e 's/pb\.Signature/core.Signature/g' \
        -e 's/pb\.TxInput/core.TxInput/g' \
        -e 's/pb\.TxOutput/core.TxOutput/g' \
        -e 's/pb\.UserIntent/core.UserIntent/g' \
        -e 's/pb\.ResourceTarget/core.ResourceTarget/g' \
        -e 's/pb\.ResourceExecutionData/core.ResourceExecutionData/g' \
        -e 's/pb\.ResourceType/resource.ResourceType/g' \
        -e 's/pb\.ResourceReference/resource.ResourceReference/g' \
        -e 's/pb\.ResourceStateUTXO/resource.ResourceStateUTXO/g' \
        -e 's/pb\.FileStore/resource.FileStore/g' \
        -e 's/pb\.OutPoint/utxo.OutPoint/g' \
        -e 's/pb\.UTXO/utxo.UTXO/g' \
        -e 's/pb\.TokenAmount/utxo.TokenAmount/g' \
        -e 's/pb\.ContractIntent/execution.ContractIntent/g' \
        -e 's/pb\.ModelIntent/execution.ModelIntent/g' \
        -e 's/pb\.InputRequirement/execution.InputRequirement/g' \
        "$file"
done

echo "导入更新完成！" 