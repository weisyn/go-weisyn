#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_multitype.onnx to use in testing.
# The "network" doesn't actually do much other than cast around some types and
# perform basic arithmetic.  It takes two inputs:
#  - "InputA": A 1x1 8-bit unsigned int tensor
#  - "InputB": A 2x2 64-bit float tensor
#
# It produces 2 outputs:
#  - "OutputA": A 2x2 16-bit signed int tensor, equal to (InputB * InputA) - 512
#  - "OutputB": A 1x1 64-bit int tensor, equal to InputA multiplied by 1234

"""
多数据类型 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_multitype.onnx 模型文件，用于测试 WES 平台对不同数据类型的支持能力。

模型设计目的：
- 测试 WES 平台对多种数据类型的支持（uint8, float64, int16, int64）
- 验证类型转换和兼容性处理
- 测试混合类型输入输出的处理能力

模型输入：
- "InputA": [1, 1, 1] 形状的 uint8 张量（8位无符号整数，范围 0-255）
- "InputB": [1, 2, 2] 形状的 float64 张量（64位双精度浮点数）

模型输出：
- "OutputA": [1, 2, 2] 形状的 int16 张量（16位有符号整数）
  计算公式：OutputA = (InputB * InputA[0][0][0]) - 512
- "OutputB": [1, 1, 1] 形状的 int64 张量（64位有符号整数）
  计算公式：OutputB = InputA * 1234

WES 平台测试场景：
- ✅ uint8 数据类型支持
- ✅ float64 数据类型支持
- ✅ int16 数据类型支持
- ✅ int64 数据类型支持
- ✅ 类型转换处理（uint8 → int64, float64 → int16）
- ✅ 混合类型输入输出

使用方法：
    python generate_network_different_types.py

依赖要求：
    pip install torch onnx
"""
import torch


class DifferentTypesModel(torch.nn.Module):
    """多数据类型测试模型。
    
    该模型设计用于测试 ONNX Runtime 和 WES 平台对不同数据类型的支持。
    模型本身不执行复杂的计算，主要目的是验证类型转换和兼容性。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, input_a, input_b):
        """前向传播：执行类型转换和基本算术运算。
        
        Args:
            input_a: uint8 类型的输入张量，形状 [1, 1, 1]
            input_b: float64 类型的输入张量，形状 [1, 2, 2]
        
        Returns:
            output_a: int16 类型的输出张量，形状 [1, 2, 2]
            output_b: int64 类型的输出张量，形状 [1, 1, 1]
        
        计算流程：
        1. OutputA 计算：
           - 将 InputA 的第一个元素（标量）与 InputB 的每个元素相乘
           - 减去常数 512（用于测试负数处理）
           - 转换为 int16 类型（测试 float64 → int16 的类型转换）
        
        2. OutputB 计算：
           - 将 InputA 转换为 int64 类型（测试 uint8 → int64 的类型转换）
           - 乘以常数 1234（用于测试整数运算）
        """
        # OutputA 计算：测试 float64 到 int16 的类型转换
        # OutputA calculation: test float64 to int16 type conversion
        # 将 InputA 的第一个元素（标量）广播到 InputB 的每个元素
        # Broadcast InputA[0][0][0] (scalar) to each element of InputB
        output_a = input_b * input_a[0][0][0]
        # 减去常数 512，用于测试负数处理能力
        # Subtract 512 to test negative number handling
        output_a -= 512
        # 转换为 int16 类型：这是关键的类型转换测试点
        # WES 平台需要正确处理 float64 → int16 的转换
        # Convert to int16: key type conversion test point
        # WES platform needs to correctly handle float64 → int16 conversion
        output_a = output_a.type(torch.int16)
        
        # OutputB 计算：测试 uint8 到 int64 的类型转换
        # OutputB calculation: test uint8 to int64 type conversion
        # 将 InputA 转换为 int64 类型：测试 uint8 → int64 的类型转换
        # Convert InputA to int64: test uint8 → int64 type conversion
        output_b = input_a.type(torch.int64)
        # 乘以常数 1234，用于测试整数运算和溢出处理
        # Multiply by 1234 to test integer operations and overflow handling
        output_b *= 1234
        
        return output_a, output_b


def fake_inputs():
    """生成测试用的假输入数据。
    
    Returns:
        input_a: uint8 类型的随机张量，形状 [1, 1, 1]，值范围 [0, 255]
        input_b: float64 类型的随机张量，形状 [1, 2, 2]，值范围 [0.0, 1.0]
    
    注意：
        - InputA 使用 uint8 类型，模拟图像像素值等场景
        - InputB 使用 float64 类型，模拟高精度计算场景
        - 这些数据类型的选择是为了测试 WES 平台的类型兼容性
    """
    # 生成 uint8 类型的输入：模拟图像像素值（0-255）
    # Generate uint8 input: simulate image pixel values (0-255)
    input_a = torch.rand((1, 1, 1)) * 255.0
    input_a = input_a.type(torch.uint8)
    
    # 生成 float64 类型的输入：模拟高精度计算场景
    # Generate float64 input: simulate high-precision computation scenarios
    input_b = torch.rand((1, 2, 2), dtype=torch.float64)
    
    return input_a, input_b


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据
    3. 运行模型推理（用于验证）
    4. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = DifferentTypesModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据
    # Generate test input data
    input_a, input_b = fake_inputs()
    
    # 运行模型推理，验证模型工作正常
    # 注意：这里使用 torch.no_grad() 可以节省内存，但为了演示清晰，我们直接运行
    # Run model inference to verify the model works correctly
    # Note: Using torch.no_grad() can save memory, but we run directly for clarity
    output_a, output_b = model(input_a, input_b)
    print(f"Example inputs: A = {input_a!s}, B = {input_b!s}")
    print(f"Produced outputs: A = {output_a!s}, B = {output_b!s}")

    # 导出 ONNX 模型
    # Export ONNX model
    out_name = "example_multitype.onnx"
    print(f"Saving model as {out_name}")
    
    # 定义输入输出名称：这些名称将在 WES 平台中用于识别输入输出
    # Define input/output names: these names will be used in WES platform to identify inputs/outputs
    input_names = ["InputA", "InputB"]
    output_names = ["OutputA", "OutputB"]
    
    # 导出为 ONNX 格式
    # torch.onnx.export 参数说明：
    # - model: 要导出的 PyTorch 模型
    # - (input_a, input_b): 示例输入（用于确定模型输入形状和类型）
    # - out_name: 输出文件名
    # - input_names: 输入张量的名称（在 ONNX 模型中）
    # - output_names: 输出张量的名称（在 ONNX 模型中）
    # Export to ONNX format
    # torch.onnx.export parameters:
    # - model: PyTorch model to export
    # - (input_a, input_b): Example inputs (used to determine model input shapes and types)
    # - out_name: Output filename
    # - input_names: Input tensor names (in ONNX model)
    # - output_names: Output tensor names (in ONNX model)
    torch.onnx.export(
        model, 
        (input_a, input_b), 
        out_name,
        input_names=input_names, 
        output_names=output_names
    )
    print(f"{out_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对不同数据类型的支持能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

