#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script creates example_dynamic_sizes.py to use in testing. It takes a
# batch of [-1, 10] input vectors and produces [-1] output scalars---the sum of
# each input vector (where -1 is a dynamic batch size).

"""
动态轴 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 example_dynamic_axes.onnx 模型文件，用于测试 WES 平台对动态输入大小的支持能力。

模型设计目的：
- 测试 WES 平台对动态批次大小的支持
- 验证运行时形状推断能力
- 测试动态轴（dynamic axes）的处理
- 验证可变输入大小的模型推理

模型说明：
- 输入：[-1, 10] 形状的向量（-1 表示动态批次大小）
- 输出：[-1] 形状的标量（每个输入向量的和）
- 操作：对每个输入向量求和（sum along dimension 1）

WES 平台测试场景：
- ✅ 动态批次大小支持（-1 表示可变）
- ✅ 运行时形状推断
- ✅ 动态轴处理
- ✅ 可变输入大小推理

使用方法：
    python generate_dynamic_axes_network.py

依赖要求：
    pip install torch onnx
"""
import torch


class DynamicSizeModel(torch.nn.Module):
    """动态大小模型。
    
    该模型接受可变批次大小的输入，并对每个输入向量求和。
    设计用于测试动态轴和运行时形状推断。
    """
    
    def __init__(self):
        """初始化模型。
        
        注意：这是一个简单的测试模型，不需要任何可训练参数。
        所有计算都在 forward 方法中完成。
        """
        super().__init__()

    def forward(self, input_batch):
        """前向传播：对每个输入向量求和。
        
        Args:
            input_batch: float32 类型的输入张量，形状 [batch_size, 10]
                        其中 batch_size 可以是任意值（动态批次）
        
        Returns:
            output: float32 类型的输出张量，形状 [batch_size]
                   每个元素是对应输入向量的和
        
        计算流程：
        - 对输入张量的第 1 维求和（sum along dimension 1）
        - 将 [batch_size, 10] 转换为 [batch_size]
        """
        # 对第 1 维求和：将 [batch_size, 10] 转换为 [batch_size]
        # Sum along dimension 1: convert [batch_size, 10] to [batch_size]
        # 每个输出元素是对应输入向量所有元素的和
        # Each output element is the sum of all elements in the corresponding input vector
        return input_batch.sum(1)


def main():
    """主函数：创建模型、生成测试数据、导出 ONNX 模型（带动态轴）。
    
    流程：
    1. 创建模型实例并设置为评估模式
    2. 生成测试输入数据（使用示例批次大小 123）
    3. 定义动态轴（指定批次维度为动态）
    4. 导出为 ONNX 格式（包含动态轴信息）
    """
    # 创建模型实例
    # Create model instance
    model = DynamicSizeModel()
    # 设置为评估模式：禁用 dropout、batch normalization 等训练时的行为
    # Set to evaluation mode: disable dropout, batch normalization, etc.
    model.eval()
    
    # 生成测试输入数据：使用示例批次大小 123
    # Generate test input data: use example batch size 123
    # 注意：导出时使用 123 作为示例，但模型支持任意批次大小
    # Note: use 123 as example during export, but model supports any batch size
    test_input = torch.rand((123, 10), dtype=torch.float32)
    
    # 定义动态轴：指定哪些维度是动态的
    # Define dynamic axes: specify which dimensions are dynamic
    # "input_vectors": [0] 表示输入的第 0 维（批次维度）是动态的
    # "output_scalars": [0] 表示输出的第 0 维（批次维度）是动态的
    # "input_vectors": [0] means the 0th dimension (batch dimension) of input is dynamic
    # "output_scalars": [0] means the 0th dimension (batch dimension) of output is dynamic
    dynamic_axes = {
        "input_vectors": [0],  # 批次维度是动态的
        "output_scalars": [0],  # 批次维度是动态的
    }
    
    output_name = "example_dynamic_axes.onnx"
    
    # 导出为 ONNX 格式（包含动态轴信息）
    # Export to ONNX format (with dynamic axes information)
    # torch.onnx.export 参数说明：
    # - model: 要导出的 PyTorch 模型
    # - (test_input): 示例输入（用于确定模型输入形状和类型）
    # - output_name: 输出文件名
    # - input_names: 输入张量的名称
    # - output_names: 输出张量的名称
    # - dynamic_axes: 动态轴定义（指定哪些维度是动态的）
    # torch.onnx.export parameters:
    # - model: PyTorch model to export
    # - (test_input): Example input (used to determine model input shape and type)
    # - output_name: Output filename
    # - input_names: Input tensor names
    # - output_names: Output tensor names
    # - dynamic_axes: Dynamic axes definition (specify which dimensions are dynamic)
    torch.onnx.export(
        model, 
        (test_input), 
        output_name,
        input_names=["input_vectors"], 
        output_names=["output_scalars"],
        dynamic_axes=dynamic_axes
    )
    print(f"Saved {output_name} OK.")
    
    print("\n✅ 模型生成完成！")
    print("📝 该模型可用于测试 WES 平台对动态输入大小的支持能力。")
    print("📖 详细使用说明请参考 README.md")

if __name__ == "__main__":
    main()

