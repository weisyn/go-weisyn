#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_float16.onnx to use in testing.
# It takes one input:
#  - "InputA": A 2x2x2 16-bit float16 tensor
# It produces one output:
#  - "OutputA": A 2x2x2 16-bit bfloat16 tensor
#
# The "network" just multiplies each element in the input by 3.0

"""
Float16 精度 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_float16.onnx 模型文件，用于测试 WES 平台对半精度浮点数的支持能力。

模型设计目的：
- 测试 WES 平台对 Float16 数据类型的支持
- 验证 BFloat16 数据类型的支持
- 测试半精度浮点数的精度转换
- 验证半精度浮点数的计算能力

模型说明：
- 输入：Float16 类型的张量，形状 [1, 2, 2, 2]
- 输出：BFloat16 类型的张量，形状 [1, 2, 2, 2]
- 操作：每个元素乘以 3.0，然后转换为 BFloat16

WES 平台测试场景：
- ✅ Float16 数据类型支持
- ✅ BFloat16 数据类型支持
- ✅ 半精度浮点数计算
- ✅ 精度转换处理

使用方法：
    python generate_float16_network.py

依赖要求：
    pip install torch onnx
"""
import torch


class Float16Model(torch.nn.Module):
    """Float16 精度测试模型。
    
    该模型设计用于测试 ONNX Runtime 和 WES 平台对半精度浮点数的支持。
    模型执行简单的乘法运算并转换精度类型。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, input_a):
        """前向传播：执行乘法运算并转换精度类型。
        
        Args:
            input_a: float16 类型的输入张量，形状 [1, 2, 2, 2]
        
        Returns:
            output_a: bfloat16 类型的输出张量，形状 [1, 2, 2, 2]
        
        计算流程：
        1. 将输入乘以 3.0（测试半精度浮点数计算）
        2. 转换为 bfloat16 类型（测试精度转换）
        """
        # 将输入乘以 3.0：测试半精度浮点数计算
        # Multiply input by 3.0: test half-precision floating point computation
        output_a = input_a * 3.0
        # 转换为 bfloat16 类型：测试 float16 → bfloat16 的精度转换
        # Convert to bfloat16: test float16 → bfloat16 precision conversion
        # WES 平台需要正确处理半精度浮点数的转换
        # WES platform needs to correctly handle half-precision floating point conversion
        output_a = output_a.type(torch.bfloat16)
        return output_a


def fake_inputs():
    """生成测试用的假输入数据。
    
    Returns:
        input_a: float16 类型的随机张量，形状 [1, 2, 2, 2]
    """
    return torch.rand((1, 2, 2, 2), dtype=torch.float16)


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据（float16 类型）
    3. 运行模型推理（用于验证）
    4. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = Float16Model()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据：float16 类型
    # Generate test input data: float16 type
    input_a = torch.rand((1, 2, 2, 2), dtype=torch.float16)
    
    # 运行模型推理，验证模型工作正常
    # Run model inference to verify the model works correctly
    output_a = model(input_a)

    # 导出 ONNX 模型
    # Export ONNX model
    out_name = "example_float16.onnx"
    torch.onnx.export(
        model, 
        (input_a), 
        out_name, 
        input_names=["InputA"],
        output_names=["OutputA"]
    )
    print(f"{out_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对半精度浮点数的支持能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

