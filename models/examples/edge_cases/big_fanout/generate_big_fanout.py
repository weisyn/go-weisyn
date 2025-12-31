#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_big_fanout.onnx to use in testing. The idea is
# to create a newtwork where parallelism makes a big difference.

"""
大扇出网络 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_big_fanout.onnx 模型文件，用于测试 WES 平台对并行计算的处理能力。

模型设计目的：
- 测试 WES 平台对并行化网络的处理能力
- 验证大量并行矩阵乘法操作的执行效率
- 测试扇出（fanout）网络结构的处理
- 验证合并和求和操作的性能

模型说明：
- 输入：1x4 向量
- 输出：1x4 向量
- 结构：100 个并行的矩阵乘法操作，然后合并并求和

WES 平台测试场景：
- ✅ 并行计算能力（100 个并行矩阵乘法）
- ✅ 大扇出网络处理
- ✅ 张量合并（concat）操作
- ✅ 求和（sum）操作
- ✅ 性能优化测试

使用方法：
    python generate_big_fanout.py

依赖要求：
    pip install torch onnx
"""
import torch


class BigFanoutModel(torch.nn.Module):
    """大扇出网络模型。
    
    将 1x4 向量映射到另一个 1x4 向量，但通过大量可并行的全连接操作。
    该模型设计用于测试并行计算能力。
    
    Maps a 1x4 vector to another 1x4 vector, but goes through a large
    number of parallelizable useless FC operations.
    """
    
    def __init__(self):
        """初始化模型。
        
        创建 100 个随机的 4x4 矩阵，用于并行矩阵乘法操作。
        """
        super().__init__()
        # 扇出数量：100 个并行操作
        # Fanout amount: 100 parallel operations
        self.fanout_amount = 100
        # 创建 100 个随机的 4x4 矩阵
        # Create 100 random 4x4 matrices
        self.matrices = [torch.rand((4, 4)) for i in range(self.fanout_amount)]

    def forward(self, x):
        """前向传播：执行大量并行矩阵乘法，然后合并并求和。
        
        Args:
            x: float32 类型的输入张量，形状 [1, 4]
        
        Returns:
            output: float32 类型的输出张量，形状 [1, 4]
        
        计算流程：
        1. 执行 100 个并行的矩阵乘法操作
        2. 将所有结果合并（concat）
        3. 对合并后的张量求和
        """
        # 执行 fanout_amount 个矩阵乘法，然后合并并求和结果
        # Do fanout_amount matrix multiplies, then merge and sum the result
        # 这 100 个操作可以并行执行，测试 WES 平台的并行计算能力
        # These 100 operations can be executed in parallel, testing WES platform's parallel computing capability
        tmp_results = [
            torch.matmul(x, self.matrices[i])
            for i in range(self.fanout_amount)
        ]
        # 合并所有结果：将 100 个 [1, 4] 张量合并为 [100, 4]
        # Concatenate all results: merge 100 [1, 4] tensors into [100, 4]
        combined_tensor = torch.cat(tmp_results)
        # 对第 0 维求和：将 [100, 4] 求和为 [4]，然后扩展为 [1, 4]
        # Sum along dimension 0: sum [100, 4] to [4], then expand to [1, 4]
        return combined_tensor.sum(0)


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型。
    
    流程：
    1. 创建模型实例并设置为 float32 类型
    2. 设置为评估模式
    3. 禁用梯度计算（推理模式）
    4. 导出为 ONNX 格式
    """
    # 创建模型实例
    # Create model instance
    model = BigFanoutModel()
    # 将模型转换为 float32 类型（确保所有参数都是 float32）
    # Convert model to float32 type (ensure all parameters are float32)
    model.float()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据
    # Generate test input data
    test_input = torch.rand((1, 4), dtype=torch.float32)
    output_name = "example_big_fanout.onnx"
    
    # 禁用梯度计算：推理时不需要梯度
    # Disable gradient computation: not needed for inference
    test_input.requires_grad = False
    for param in model.parameters():
        param.requires_grad = False
    
    # 使用 torch.no_grad() 上下文管理器导出模型
    # Export model using torch.no_grad() context manager
    # 这样可以节省内存并加快导出速度
    # This saves memory and speeds up export
    with torch.no_grad():
        torch.onnx.export(
            model, 
            (test_input), 
            output_name,
            input_names=["input"], 
            output_names=["output"]
        )
    print(f"Saved {output_name} OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对并行计算的处理能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

