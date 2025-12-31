#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script generates the .onnx file with a bunch of different special chars
# in the filename. It takes a 1x2 uint32 tensor and produces a 1x1-element
# uint32 output containing the sum of the 2 inputs.

"""
特殊字符文件名 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成包含 Unicode 特殊字符的文件名模型，用于测试 WES 平台对 Unicode 文件名的支持和文件系统兼容性。

模型设计目的：
- 测试 WES 平台对 Unicode 文件名的支持
- 验证文件系统兼容性
- 测试特殊字符处理能力
- 验证文件名编码处理

模型说明：
- 输入：1x2 形状的 int32 张量
- 输出：1x1 形状的 int32 张量（两个输入的和）
- 文件名：包含 Unicode 特殊字符（ż, 大, 김）

WES 平台测试场景：
- ✅ Unicode 文件名支持
- ✅ 文件系统兼容性
- ✅ 特殊字符处理
- ✅ 文件名编码处理

使用方法：
    python generate_odd_name_onnx.py

依赖要求：
    pip install torch onnx
"""
import torch


class AddModel(torch.nn.Module):
    """加法模型。
    
    该模型执行简单的加法运算，用于测试 Unicode 文件名的处理。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, inputs):
        """前向传播：对输入求和。
        
        Args:
            inputs: int32 类型的输入张量，形状 [1, 2]
        
        Returns:
            output: int32 类型的输出张量，形状 [1, 1]
                    包含两个输入元素的和
        """
        # 对第 1 维求和：将 [1, 2] 转换为 [1]
        # Sum along dimension 1: convert [1, 2] to [1]
        # 转换为 int 类型：确保输出类型正确
        # Convert to int type: ensure correct output type
        return inputs.sum(1).int()


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型（使用 Unicode 文件名）。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据
    3. 导出为 ONNX 格式（使用包含 Unicode 字符的文件名）
    """
    # 创建模型实例
    # Create model instance
    model = AddModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据：全为 1
    # Generate test input data: all ones
    x = torch.ones((1, 2), dtype=torch.int32)
    
    # 使用包含 Unicode 特殊字符的文件名
    # Use filename with Unicode special characters
    # 文件名包含：ż（波兰语字符）、大（中文字符）、김（韩文字符）
    # Filename contains: ż (Polish character), 大 (Chinese character), 김 (Korean character)
    # 这用于测试 WES 平台对 Unicode 文件名的支持
    # This tests WES platform's support for Unicode filenames
    file_name = "example ż 大 김.onnx"
    
    # 导出为 ONNX 格式
    # Export to ONNX format
    torch.onnx.export(
        model, 
        (x,), 
        file_name, 
        input_names=["in"],
        output_names=["out"]
    )
    print(f"{file_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对 Unicode 文件名的支持能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

