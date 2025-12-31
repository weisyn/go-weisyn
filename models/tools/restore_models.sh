#!/bin/bash
# 按照之前的迁移规划恢复 models/examples 目录
# 从备份目录恢复模型文件

set -e

SOURCE_MODELS_MAIN="/Users/qinglong/go/src/chaincodes/WES/AI/models-main"
SOURCE_ONNXRUNTIME="/Users/qinglong/go/src/chaincodes/WES/AI/onnxruntime_go-master"
TARGET_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../examples" && pwd)"

echo "=== 恢复 models/examples 目录 ==="
echo ""
echo "源目录："
echo "  - ONNX Model Zoo: $SOURCE_MODELS_MAIN"
echo "  - onnxruntime_go: $SOURCE_ONNXRUNTIME"
echo "目标目录：$TARGET_DIR"
echo ""

# 检查源目录是否存在
if [ ! -d "$SOURCE_MODELS_MAIN" ]; then
    echo "❌ 错误: 源目录不存在: $SOURCE_MODELS_MAIN"
    exit 1
fi

if [ ! -d "$SOURCE_ONNXRUNTIME" ]; then
    echo "❌ 错误: 源目录不存在: $SOURCE_ONNXRUNTIME"
    exit 1
fi

# 创建目标目录结构
echo "📁 创建目录结构..."
mkdir -p "$TARGET_DIR/test/basic"
mkdir -p "$TARGET_DIR/test/edge_cases"
mkdir -p "$TARGET_DIR/computer_vision"
mkdir -p "$TARGET_DIR/natural_language_processing"
mkdir -p "$TARGET_DIR/generative_ai"
mkdir -p "$TARGET_DIR/graph_machine_learning"

# 1. 从 onnxruntime_go-master/test_data 迁移测试模型
echo ""
echo "1️⃣ 迁移测试模型 (onnxruntime_go)..."
if [ -d "$SOURCE_ONNXRUNTIME/test_data" ]; then
    # test_data 目录下的文件直接复制，按文件名分类
    for onnx_file in "$SOURCE_ONNXRUNTIME/test_data"/*.onnx; do
        if [ -f "$onnx_file" ]; then
            filename=$(basename "$onnx_file")
            # 根据文件名判断是 basic 还是 edge_cases
            if [[ "$filename" == "sklearn_randomforest.onnx" ]] || [[ "$filename" == "example_several_inputs_and_outputs.onnx" ]] || [[ "$filename" == "example_multitype.onnx" ]]; then
                cp "$onnx_file" "$TARGET_DIR/test/basic/" 2>/dev/null || true
            else
                cp "$onnx_file" "$TARGET_DIR/test/edge_cases/" 2>/dev/null || true
            fi
        fi
    done
    echo "  ✅ 已复制测试模型文件"
else
    echo "  ⚠️  未找到 test_data 目录"
fi

# 2. 从 models-main 的各个分类目录迁移
echo ""
echo "2️⃣ 迁移分类模型 (ONNX Model Zoo)..."

# Computer_Vision -> computer_vision
if [ -d "$SOURCE_MODELS_MAIN/Computer_Vision" ]; then
    echo "  📸 迁移 Computer_Vision..."
    cp -r "$SOURCE_MODELS_MAIN/Computer_Vision"/* "$TARGET_DIR/computer_vision/" 2>/dev/null || true
    echo "    ✅ 已完成"
fi

# Natural_Language_Processing -> natural_language_processing
if [ -d "$SOURCE_MODELS_MAIN/Natural_Language_Processing" ]; then
    echo "  📝 迁移 Natural_Language_Processing..."
    cp -r "$SOURCE_MODELS_MAIN/Natural_Language_Processing"/* "$TARGET_DIR/natural_language_processing/" 2>/dev/null || true
    echo "    ✅ 已完成"
fi

# Generative_AI -> generative_ai
if [ -d "$SOURCE_MODELS_MAIN/Generative_AI" ]; then
    echo "  🎨 迁移 Generative_AI..."
    cp -r "$SOURCE_MODELS_MAIN/Generative_AI"/* "$TARGET_DIR/generative_ai/" 2>/dev/null || true
    echo "    ✅ 已完成"
fi

# Graph_Machine_Learning -> graph_machine_learning
if [ -d "$SOURCE_MODELS_MAIN/Graph_Machine_Learning" ]; then
    echo "  🕸️  迁移 Graph_Machine_Learning..."
    cp -r "$SOURCE_MODELS_MAIN/Graph_Machine_Learning"/* "$TARGET_DIR/graph_machine_learning/" 2>/dev/null || true
    echo "    ✅ 已完成"
fi

# 3. 从 models-main/validated 迁移已验证模型
echo ""
echo "3️⃣ 迁移已验证模型 (validated)..."
if [ -d "$SOURCE_MODELS_MAIN/validated" ]; then
    # 遍历 validated 目录下的各个分类
    for category_dir in "$SOURCE_MODELS_MAIN/validated"/*; do
        if [ ! -d "$category_dir" ]; then
            continue
        fi
        
        category_name=$(basename "$category_dir")
        
        # 映射分类名称
        case "$category_name" in
            "vision")
                target_category="computer_vision"
                ;;
            "text")
                target_category="natural_language_processing"
                ;;
            "generative")
                target_category="generative_ai"
                ;;
            "graph")
                target_category="graph_machine_learning"
                ;;
            *)
                echo "    ⚠️  跳过未知分类: $category_name"
                continue
                ;;
        esac
        
        echo "  📦 处理 $category_name -> $target_category..."
        
        # 遍历该分类下的模型目录
        for model_dir in "$category_dir"/*; do
            if [ ! -d "$model_dir" ]; then
                continue
            fi
            
            model_name=$(basename "$model_dir")
            target_model_dir="$TARGET_DIR/$target_category/$model_name"
            
            # 创建目标目录
            mkdir -p "$target_model_dir"
            
            # 复制模型文件
            if [ -f "$model_dir/model"/*.onnx ]; then
                cp "$model_dir/model"/*.onnx "$target_model_dir/" 2>/dev/null || true
            fi
            
            # 复制 README.md
            if [ -f "$model_dir/README.md" ]; then
                cp "$model_dir/README.md" "$target_model_dir/" 2>/dev/null || true
            fi
            
            # 复制预处理目录（如果存在）
            if [ -d "$model_dir/preproc" ]; then
                cp -r "$model_dir/preproc" "$target_model_dir/" 2>/dev/null || true
            fi
        done
        
        echo "    ✅ $category_name 已完成"
    done
else
    echo "  ⚠️  未找到 validated 目录"
fi

# 清理不需要的文件
echo ""
echo "4️⃣ 清理不需要的文件..."
find "$TARGET_DIR" -name ".git" -type d -exec rm -rf {} + 2>/dev/null || true
find "$TARGET_DIR" -name ".gitattributes" -type f -delete 2>/dev/null || true
find "$TARGET_DIR" -name ".DS_Store" -type f -delete 2>/dev/null || true
find "$TARGET_DIR" -name "*.tar.gz" -type f -delete 2>/dev/null || true
echo "  ✅ 清理完成"

# 验证恢复结果
echo ""
echo "5️⃣ 验证恢复结果..."
ONNX_COUNT=$(find "$TARGET_DIR" -name "*.onnx" -type f 2>/dev/null | wc -l | tr -d ' ')
echo "  📊 找到 $ONNX_COUNT 个 .onnx 文件"

if [ "$ONNX_COUNT" -eq 0 ]; then
    echo "  ⚠️  警告: 未找到任何 .onnx 文件"
else
    # 检查文件大小（确认不是 LFS 指针）
    SAMPLE_FILE=$(find "$TARGET_DIR" -name "*.onnx" -type f | head -1)
    if [ -n "$SAMPLE_FILE" ]; then
        FILE_SIZE=$(stat -f%z "$SAMPLE_FILE" 2>/dev/null || stat -c%s "$SAMPLE_FILE" 2>/dev/null)
        if [ "$FILE_SIZE" -lt 200 ]; then
            echo "  ⚠️  警告: 文件可能是 LFS 指针（$FILE_SIZE 字节）"
        else
            echo "  ✅ 文件大小正常（示例文件: $FILE_SIZE 字节）"
        fi
    fi
fi

echo ""
echo "✅ 恢复完成！"
echo ""
echo "📋 下一步："
echo "1. 检查文件: find models/examples -name '*.onnx' -type f | wc -l"
echo "2. 提交文件: git add models/examples/**/*.onnx"
echo "3. 提交更改: git commit -m 'chore: restore ONNX models as regular files'"
echo "4. 推送代码: git push origin main"
