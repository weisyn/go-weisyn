#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_big_compute.onnx to use in testing.
# The "network" is entirely deterministic; it simply does a large amount of
# hopefully expensive arithmetic operations.
#
# It takes one input: "Input", a one-dimensional vector of 1024*1024*50 32-bit
# floats, and produces one output, named "Output" of the same dimensions.

"""
大计算量网络 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_big_compute.onnx 模型文件，用于测试 WES 平台对复杂计算的处理能力。

模型设计目的：
- 测试 WES 平台对复杂计算的处理能力
- 验证大量算术运算的执行效率
- 测试大张量的处理能力
- 验证计算密集型模型的性能

模型说明：
- 输入：1 维向量，包含 1024*1024*50 = 52,428,800 个 float32 元素
- 输出：相同维度的 float32 向量
- 操作：对大型张量执行 40 次除法和乘法操作

WES 平台测试场景：
- ✅ 大张量处理（52M 元素）
- ✅ 复杂计算处理（40 次运算）
- ✅ 计算密集型模型性能
- ✅ 内存管理能力

使用方法：
    python generate_network_big_compute.py

依赖要求：
    pip install torch onnx
"""
import torch


class BigComputeModel(torch.nn.Module):
    """大计算量网络模型。
    
    该模型设计用于测试 ONNX Runtime 和 WES 平台对复杂计算的处理能力。
    模型对大型张量执行大量算术运算。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, x):
        """前向传播：执行大量算术运算。
        
        Args:
            x: float32 类型的输入张量，形状 [1, 52428800]
               包含 52,428,800 个元素（1024 * 1024 * 50）
        
        Returns:
            output: float32 类型的输出张量，形状 [1, 52428800]
                   理论上应该等于输入（除以 10.0 再乘以 10.0）
        
        计算流程：
        - 执行 40 次循环，每次循环：
          1. 除以 10.0
          2. 乘以 10.0
        - 这用于测试大量算术运算的处理能力
        """
        # 执行 40 次除法和乘法操作
        # Execute 40 division and multiplication operations
        # 这用于测试 WES 平台对复杂计算的处理能力
        # This tests WES platform's capability to handle complex computations
        for i in range(40):
            # 除以 10.0：测试除法运算
            # Divide by 10.0: test division operation
            x = x / 10.0
            # 乘以 10.0：测试乘法运算
            # Multiply by 10.0: test multiplication operation
            x = x * 10.0
        return x


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据（大型张量，52M 元素）
    3. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = BigComputeModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据：大型张量（52,428,800 个元素）
    # Generate test input data: large tensor (52,428,800 elements)
    # 1024 * 1024 * 50 = 52,428,800 个 float32 元素
    # 1024 * 1024 * 50 = 52,428,800 float32 elements
    # 这用于测试 WES 平台对大张量的处理能力
    # This tests WES platform's capability to handle large tensors
    x = torch.zeros((1, 1024 * 1024 * 50), dtype=torch.float32)

    # 导出 ONNX 模型
    # Export ONNX model
    out_name = "example_big_compute.onnx"
    torch.onnx.export(
        model, 
        x, 
        out_name,
        input_names=["Input"], 
        output_names=["Output"]
    )
    print(f"{out_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对复杂计算的处理能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

