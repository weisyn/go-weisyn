#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_several_inputs_and_outputs.onnx to use in
# testing. The "network" is entirely deterministic, and is intended just to
# illustrate a wide variety of inputs and outputs with varying names,
# dimensions, and types.
#
# Inputs:
#  - "input 1": a 2x5x2x5 int32 tensor
#  - "input 2": a 2x3x20 float tensor
#  - "input 3": a 9-element bfloat16 tensor
#
# Outputs:
#  - "output 1": A 10x10 element int64 tensor
#  - "output 2": A 1x2x3x4x5 element double tensor
#
# The contents of the inputs and outputs are arbitrary.

"""
多输入多输出 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_several_inputs_and_outputs.onnx 模型文件，用于测试 WES 平台
处理多个输入和输出的能力。

模型设计目的：
- 测试 WES 平台对多输入多输出的支持（3个输入，2个输出）
- 验证不同数据类型的处理（int32, float32, bfloat16, int64, double）
- 测试不同维度的张量处理（1维到5维）
- 验证输入输出名称映射功能

模型输入：
- "input 1": [2, 5, 2, 5] 形状的 int32 张量（4维整数张量，共100个元素）
- "input 2": [2, 3, 20] 形状的 float32 张量（3维浮点张量，共120个元素）
- "input 3": [9] 形状的 bfloat16 张量（1维半精度浮点张量，9个元素）

模型输出：
- "output 1": [10, 10] 形状的 int64 张量（由 input 1 重塑并转换类型）
- "output 2": [1, 2, 3, 4, 5] 形状的 double 张量（由 input 2 重塑并转换类型）

WES 平台测试场景：
- ✅ 多输入处理（3个输入）
- ✅ 多输出处理（2个输出）
- ✅ 输入输出名称映射
- ✅ 不同数据类型支持（int32, float32, bfloat16, int64, double）
- ✅ 不同维度处理（1维到5维）
- ✅ 张量重塑（reshape）操作
- ✅ 类型转换操作

使用方法：
    python generate_several_inputs_and_outputs.py

依赖要求：
    pip install torch onnx
"""
import torch


class ManyInputOutputModel(torch.nn.Module):
    """多输入多输出测试模型。
    
    该模型设计用于测试 ONNX Runtime 和 WES 平台对多输入多输出的支持能力。
    模型本身不执行复杂的计算，主要目的是验证输入输出映射和类型转换。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, a, b, c):
        """前向传播：处理多个输入并生成多个输出。
        
        Args:
            a: int32 类型的输入张量，形状 [2, 5, 2, 5]
            b: float32 类型的输入张量，形状 [2, 3, 20]
            c: bfloat16 类型的输入张量，形状 [9]
        
        Returns:
            output_a: int64 类型的输出张量，形状 [10, 10]
            output_b: double 类型的输出张量，形状 [1, 2, 3, 4, 5]
        
        计算流程：
        1. OutputA 计算：
           - 将 InputA 重塑为 [10, 10] 形状（2*5*2*5 = 100 = 10*10）
           - 转换为 int64 类型
           - 使用 InputC 的第一个元素更新 output_a[0][0]
        
        2. OutputB 计算：
           - 将 InputB 重塑为 [1, 2, 3, 4, 5] 形状（2*3*20 = 120 = 1*2*3*4*5）
           - 转换为 double 类型
        """
        # OutputA 计算：重塑 InputA 并转换类型
        # OutputA calculation: reshape InputA and convert type
        # 将 [2, 5, 2, 5] 重塑为 [10, 10]（总元素数不变：2*5*2*5 = 100 = 10*10）
        # Reshape [2, 5, 2, 5] to [10, 10] (total elements unchanged: 2*5*2*5 = 100 = 10*10)
        output_a = a.reshape((10, 10))
        # 转换为 int64 类型：测试 int32 → int64 的类型转换
        # Convert to int64: test int32 → int64 type conversion
        output_a = output_a.type(torch.int64)
        
        # OutputB 计算：重塑 InputB 并转换类型
        # OutputB calculation: reshape InputB and convert type
        # 将 [2, 3, 20] 重塑为 [1, 2, 3, 4, 5]（总元素数不变：2*3*20 = 120 = 1*2*3*4*5）
        # Reshape [2, 3, 20] to [1, 2, 3, 4, 5] (total elements unchanged: 2*3*20 = 120 = 1*2*3*4*5)
        output_b = b.reshape((1, 2, 3, 4, 5))
        # 转换为 double 类型：测试 float32 → double 的类型转换
        # Convert to double: test float32 → double type conversion
        output_b = output_b.type(torch.double)
        
        # 确保使用 InputC：将 InputC 的第一个元素添加到 output_a[0][0]
        # Just to make sure we use input C: add InputC's first element to output_a[0][0]
        # 这是为了确保所有输入都被使用，测试 WES 平台对多输入的处理
        # This ensures all inputs are used, testing WES platform's multi-input handling
        output_a[0][0] += c[0].type(torch.int64)
        
        return output_a, output_b


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据（3个不同形状和类型的输入）
    3. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = ManyInputOutputModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 导出 ONNX 模型
    # Export ONNX model
    out_name = "example_several_inputs_and_outputs.onnx"
    
    # 生成测试输入数据：3个不同形状和类型的输入
    # Generate test input data: 3 inputs with different shapes and types
    # Input 1: int32 类型，4维张量 [2, 5, 2, 5]
    # Input 1: int32 type, 4D tensor [2, 5, 2, 5]
    input_a = torch.zeros((2, 5, 2, 5), dtype=torch.int32)
    # Input 2: float32 类型，3维张量 [2, 3, 20]
    # Input 2: float32 type, 3D tensor [2, 3, 20]
    input_b = torch.zeros((2, 3, 20), dtype=torch.float)
    # Input 3: bfloat16 类型，1维张量 [9]
    # Input 3: bfloat16 type, 1D tensor [9]
    input_c = torch.zeros((9), dtype=torch.bfloat16)
    
    # 导出为 ONNX 格式
    # Export to ONNX format
    # torch.onnx.export 参数说明：
    # - model: 要导出的 PyTorch 模型
    # - (input_a, input_b, input_c): 示例输入（用于确定模型输入形状和类型）
    # - out_name: 输出文件名
    # - input_names: 输入张量的名称（在 ONNX 模型中，注意名称中包含空格）
    # - output_names: 输出张量的名称（在 ONNX 模型中，注意名称中包含空格）
    # torch.onnx.export parameters:
    # - model: PyTorch model to export
    # - (input_a, input_b, input_c): Example inputs (used to determine model input shapes and types)
    # - out_name: Output filename
    # - input_names: Input tensor names (in ONNX model, note names contain spaces)
    # - output_names: Output tensor names (in ONNX model, note names contain spaces)
    torch.onnx.export(
        model, 
        (input_a, input_b, input_c), 
        out_name,
        input_names=["input 1", "input 2", "input 3"],
        output_names=["output 1", "output 2"]
    )
    print(f"{out_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对多输入多输出的支持能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

