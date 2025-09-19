package env

// LinearModel 轻量级线性回归模型
//
// # 核心功能：
// - 在线随机梯度下降（SGD）学习算法
// - L2正则化防止过拟合
// - 增量学习，支持流式数据更新
// - 零外部依赖，纯Go实现
//
// # 设计目标：
// - 轻量级：最小内存占用，无外部库依赖
// - 实时性：在线学习，无需批量训练
// - 稳定性：L2正则化和学习率控制
// - 实用性：针对执行时间预测优化
//
// # 技术特点：
// - 特征预归一化：要求输入特征在[0,1]范围内
// - 自适应学习：通过样本数量判断模型成熟度
// - 数值稳定：防止预测值为负数
// - 简单有效：专注线性关系建模
//
// # 使用场景：
// - 执行时间预测和资源建议优化
// - 在线性能模型的增量更新
// - 无历史数据存储的流式学习
// - 轻量级AI增强的决策系统
//
// # 数学模型：
// - 预测函数：y = w₁x₁ + w₂x₂ + ... + wₙxₙ + b
// - 损失函数：MSE + L2正则化
// - 优化算法：随机梯度下降（SGD）
type LinearModel struct {
	// weights 特征权重向量
	// 每个元素对应一个输入特征的权重系数
	// 通过SGD算法进行在线更新
	weights []float64

	// intercept 截距项（偏置）
	// 线性模型的常数项，独立于输入特征
	// 提供基线预测值
	intercept float64

	// learningRate 学习率
	// 控制参数更新的步长大小
	// 默认0.05，平衡收敛速度和稳定性
	learningRate float64

	// l2 L2正则化系数
	// 防止过拟合，控制权重的复杂度
	// 默认0.001，适度正则化
	l2 float64

	// samples 已处理样本数量
	// 用于判断模型是否具备基本可用性
	// 样本数≥10时认为模型Ready
	samples int
}

// NewLinearModel 创建线性回归模型
//
// 构造函数，创建初始化的线性模型实例
//
// 参数：
//   - numFeatures: 特征维度数量，必须大于0
//
// 返回值：
//   - *LinearModel: 初始化完成的模型实例
//
// 初始状态：
//   - 权重向量初始化为零向量
//   - 截距初始化为0
//   - 学习率设置为0.05（经验最优值）
//   - L2正则化系数设置为0.001（轻度正则化）
//   - 样本计数器初始化为0
//
// 使用示例：
//
//	// 创建3特征的线性模型（资源Norm, avgDur, failRate）
//	model := NewLinearModel(3)
//
// 设计考虑：
//   - 参数校验：特征数≤0时自动修正为1
//   - 保守初始化：零权重避免初始偏置
//   - 经验参数：学习率和正则化系数基于实验优化
func NewLinearModel(numFeatures int) *LinearModel {
	// 参数校验和修正
	if numFeatures <= 0 {
		numFeatures = 1
	}
	return &LinearModel{
		weights:      make([]float64, numFeatures),
		intercept:    0,
		learningRate: 0.05,
		l2:           0.001,
	}
}

// Update 以一条样本进行在线更新
// features 需预归一化；target 建议同量纲归一化
func (m *LinearModel) Update(features []float64, target float64) {
	if m == nil || len(features) == 0 {
		return
	}
	// 预测
	pred := m.predictRaw(features)
	err := pred - target
	// 更新偏置
	m.intercept -= m.learningRate * err
	// 更新权重（含L2）
	for j := range m.weights {
		grad := err*features[j] + m.l2*m.weights[j]
		m.weights[j] -= m.learningRate * grad
	}
	m.samples++
}

// Predict 预测（features需与训练时一致的归一化）
func (m *LinearModel) Predict(features []float64) float64 {
	if m == nil {
		return 0
	}
	return m.predictRaw(features)
}

// Ready 是否具备基本可用性
func (m *LinearModel) Ready() bool { return m != nil && m.samples >= 10 }

// 内部：线性组合
func (m *LinearModel) predictRaw(features []float64) float64 {
	y := m.intercept
	n := len(features)
	if len(m.weights) < n {
		n = len(m.weights)
	}
	for j := 0; j < n; j++ {
		y += m.weights[j] * features[j]
	}
	if y < 0 {
		return 0
	}
	return y
}
