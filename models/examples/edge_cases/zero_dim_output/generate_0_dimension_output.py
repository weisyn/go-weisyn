#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_0_dim_output.onnx to use in testing. The idea is
# that the network produces an output with one a dimension of size 0.

"""
零维输出 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_0_dim_output.onnx 模型文件，用于测试 WES 平台对零维张量和边界条件的处理能力。

模型设计目的：
- 测试 WES 平台对零维张量的支持
- 验证边界条件的处理能力
- 测试动态维度大小的处理
- 验证标量输出的处理

模型说明：
- 输入：2x8 形状的 float32 张量
- 输出：2xNx8 形状的 float32 张量，其中 N 是第一个输入列的和（转换为整数）
- 特殊场景：当输入全为 0 时，输出为 2x0x8（其中一个维度为 0）

WES 平台测试场景：
- ✅ 零维张量支持
- ✅ 边界条件处理（维度为 0）
- ✅ 动态维度大小
- ✅ 标量输出处理

使用方法：
    python generate_0_dimension_output.py

依赖要求：
    pip install torch onnx
"""
import torch


class ZeroDimOutputModel(torch.nn.Module):
    """零维输出测试模型。
    
    接受 2x8 输入，产生 2xNx8 输出，其中 N 是第一个输入列的和（转换为整数）。
    在测试中，输入全为 0，因此会产生 2x0x8 的输出。
    
    Takes a 2x8 input, and produces a 2xNx8 output, where N is the sum of
    the first input column, cast to an int. In tests, the input will be all 0s,
    so this should result in a 2x0x8 output.
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, x):
        """前向传播：生成可能包含零维的输出。
        
        Args:
            x: float32 类型的输入张量，形状 [2, 8]
        
        Returns:
            output: float32 类型的输出张量，形状 [2, N, 8]
                    其中 N = sum(x[:, 0])（第一个输入列的和）
        
        计算流程：
        1. 对第 0 维求和：将 [2, 8] 转换为 [8]
        2. 在第 0 维添加维度：将 [8] 转换为 [1, 8]
        3. 扩展为 [2, N, 8]，其中 N 是第一个输入列的和
        """
        # 对第 0 维求和：将 [2, 8] 转换为 [8]
        # Sum along dimension 0: convert [2, 8] to [8]
        tmp = x.sum(0)
        # 在第 0 维添加维度并扩展：将 [8] 转换为 [2, N, 8]
        # Add dimension at 0 and expand: convert [8] to [2, N, 8]
        # 其中 N = tmp.int()[0]（第一个输入列的和，转换为整数）
        # where N = tmp.int()[0] (sum of first input column, cast to int)
        # 当输入全为 0 时，N = 0，输出为 [2, 0, 8]（零维）
        # When input is all zeros, N = 0, output is [2, 0, 8] (zero dimension)
        return tmp.unsqueeze(0).expand(2, tmp.int()[0], -1)


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据（全为 1，用于正常测试）
    3. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = ZeroDimOutputModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据：全为 1（用于正常测试）
    # Generate test input data: all ones (for normal testing)
    # 注意：在实际测试中，可以使用全为 0 的输入来测试零维输出
    # Note: in actual testing, can use all-zero input to test zero-dimension output
    x = torch.ones((2, 8), dtype=torch.float32)
    
    # 导出 ONNX 模型
    # Export ONNX model
    out_name = "example_0_dim_output.onnx"
    torch.onnx.export(
        model, 
        (x,), 
        out_name, 
        input_names=["x"],
        output_names=["y"]
    )
    print(f"{out_name} saved OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对零维张量和边界条件的处理能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

