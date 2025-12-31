#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# This script is a modified version of the example from
# https://pypi.org/project/skl2onnx/, which we use to produce
# sklearn_randomforest.onnx. sklearn makes heavy use of onnxruntime maps and
# sequences in its networks, so this is used for testing those data types.

"""
sklearn 随机森林 ONNX 模型生成脚本（中文注释补充）

本脚本用于生成 sklearn_randomforest.onnx 模型文件，用于测试 WES 平台对
sklearn 模型和复杂数据类型（Map、Sequence）的支持能力。

模型设计目的：
- 测试 WES 平台对 sklearn 模型的支持
- 验证 ONNX Runtime 对 Map 和 Sequence 数据类型的支持
- 测试随机森林分类器的推理能力
- 验证多输出模型（标签 + 概率）的处理

模型说明：
- 基于 scikit-learn 的随机森林分类器
- 使用 Iris（鸢尾花）数据集进行训练
- 输入：4 个特征（花萼长度、花萼宽度、花瓣长度、花瓣宽度）
- 输出：分类标签（0, 1, 2）和概率分布

WES 平台测试场景：
- ✅ sklearn 模型转换到 ONNX
- ✅ Map 数据类型支持（sklearn 模型大量使用）
- ✅ Sequence 数据类型支持（sklearn 模型大量使用）
- ✅ 多输出模型（output_label, output_probability）
- ✅ 分类任务推理

使用方法：
    python generate_sklearn_network.py

依赖要求：
    pip install numpy scikit-learn skl2onnx onnxruntime
"""
import numpy as np
from sklearn.datasets import load_iris
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier

# 加载 Iris 数据集
# Load Iris dataset
# Iris 数据集包含 150 个样本，每个样本有 4 个特征（花萼长度、花萼宽度、花瓣长度、花瓣宽度）
# 3 个类别（Setosa, Versicolor, Virginica）
# Iris dataset contains 150 samples, each with 4 features (sepal length, sepal width, petal length, petal width)
# 3 classes (Setosa, Versicolor, Virginica)
iris = load_iris()
inputs, outputs = iris.data, iris.target

# 将输入转换为 float32 类型（ONNX 常用类型）
# Convert inputs to float32 type (commonly used in ONNX)
inputs = inputs.astype(np.float32)

# 划分训练集和测试集
# Split into training and test sets
inputs_train, inputs_test, outputs_train, outputs_test = train_test_split(inputs, outputs)

# 创建并训练随机森林分类器
# Create and train RandomForest classifier
# 随机森林使用默认参数，适合快速测试
# RandomForest uses default parameters, suitable for quick testing
classifier = RandomForestClassifier()
classifier.fit(inputs_train, outputs_train)

# 转换为 ONNX 格式
# Convert to ONNX format
# skl2onnx 是 sklearn 模型转换为 ONNX 的工具
# skl2onnx is a tool for converting sklearn models to ONNX
from skl2onnx import to_onnx

output_filename = "sklearn_randomforest.onnx"
# 使用第一个样本作为示例输入，用于确定输入形状和类型
# Use first sample as example input to determine input shape and type
onnx_content = to_onnx(classifier, inputs[:1])

# 保存 ONNX 模型文件
# Save ONNX model file
with open(output_filename, "wb") as f:
    f.write(onnx_content.SerializeToString())

# 使用 ONNX Runtime 验证模型
# Verify model with ONNX Runtime
import onnxruntime as ort

def float_formatter(f):
    """
    浮点数格式化函数：保留 6 位小数
    
    Float formatter function: keep 6 decimal places
    """
    return f"{float(f):.06f}"

# 设置 numpy 打印格式：使用自定义格式化函数
# Set numpy print format: use custom formatter function
np.set_printoptions(formatter={'float_kind': float_formatter})

# 创建 ONNX Runtime 推理会话
# Create ONNX Runtime inference session
session = ort.InferenceSession(output_filename)

# 打印模型的输入输出信息
# Print model input/output information
print(f"Input names: {[n.name for n in session.get_inputs()]!s}")
print(f"Output names: {[o.name for o in session.get_outputs()]!s}")

# 准备测试输入：使用测试集的前 6 个样本
# Prepare test inputs: use first 6 samples from test set
example_inputs = inputs_test.astype(np.float32)[:6]
print(f"Inputs shape = {example_inputs.shape!s}")

# 运行推理：获取分类标签和概率
# Run inference: get classification labels and probabilities
onnx_predictions = session.run(
    ["output_label", "output_probability"],
    {"X": example_inputs}
)
labels = onnx_predictions[0]
probabilities = onnx_predictions[1]

# 打印推理结果
# Print inference results
print(f"Inputs to network: {example_inputs.astype(np.float32)}")
print(f"ONNX predicted labels: {labels!s}")
print(f"ONNX predicted probabilities: {probabilities!s}")

print("\n✅ 模型生成完成！")
print("📝 该模型可用于测试 WES 平台对 sklearn 模型和复杂数据类型的支持能力。")
print("📖 详细使用说明请参考 README.md")

