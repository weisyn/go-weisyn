// ==================== 我的第一个WES智能合约 ====================
//
// 🎯 学习目标：通过这个最简单的例子，你将学会：
// ✅ 什么是智能合约，它是如何工作的
// ✅ 如何获取调用者的身份信息
// ✅ 如何发出事件来记录重要操作
// ✅ 如何处理错误和返回结果
//
// 📚 背景知识：
// 智能合约就像一个放在区块链上的小程序，任何人都可以调用它。
// 这个程序会永久保存在区块链上，不会丢失，也不会被篡改。
//
// 🔍 生活化理解：
// 把智能合约想象成一台自动售货机：
// - 你投入硬币（发起交易）
// - 机器执行程序（运行合约代码）
// - 你得到商品和收据（获得结果和事件记录）

package main

// 📦 导入必要的包
// framework包提供了与WES区块链交互的所有基础功能
import (
	"github.com/weisyn/v1/contracts/sdk/go/framework"
)

// ==================== SayHello函数 ====================
//
// 🎯 函数作用：向调用者发送一个问候消息
//
// 💡 工作原理：
// 1. 获取调用者的区块链地址（身份标识）
// 2. 创建个性化的问候消息
// 3. 发出事件到区块链网络（相当于发布公告）
// 4. 返回执行结果
//
// 🔧 返回值说明：
// - framework.SUCCESS (0): 表示执行成功
// - framework.ERROR_EXECUTION_FAILED (6): 表示执行失败
func SayHello() uint32 {
	// 📍 步骤1：获取调用者地址
	//
	// 💭 什么是地址？
	// 地址就像你的银行账号，是区块链上唯一标识你身份的字符串
	// 例如：0x1234567890abcdef1234567890abcdef12345678
	//
	// 🔍 GetCaller()函数：
	// 这个函数告诉我们"谁在调用这个合约"
	// 就像接电话时询问"您是哪位？"
	caller := framework.GetCaller()

	// 📍 步骤2：构造个性化问候消息
	//
	// 💡 ToString()方法：
	// 将地址转换为人类可读的字符串格式
	// 原始地址是二进制数据，toString()让它变成可以阅读的文字
	message := "Hello, WES World! 🎉 您的区块链地址是: " + caller.ToString()

	// 📍 步骤3：创建事件来记录这次互动
	//
	// 🌟 什么是事件？
	// 事件就像区块链的"朋友圈动态"或"微博"，记录发生了什么
	// 其他人可以看到这些事件，了解合约的活动情况
	//
	// 🎯 为什么要发出事件？
	// - 记录重要操作的历史
	// - 让其他程序能够监听合约活动
	// - 提供可审计的操作日志
	//
	// 📝 创建名为"HelloWorld"的事件
	event := framework.NewEvent("HelloWorld")

	// 📝 添加事件数据字段
	// 每个字段都是一个键值对，记录相关信息

	// 添加问候消息内容
	event.AddStringField("message", message)

	// 添加调用者地址（方便后续查询是谁调用了合约）
	event.AddAddressField("caller", caller)

	// 添加时间戳（记录事件发生的具体时间）
	// GetTimestamp()返回当前区块的时间戳
	event.AddUint64Field("timestamp", framework.GetTimestamp())

	// 📍 步骤4：将事件发送到区块链网络
	//
	// 💫 EmitEvent()函数：
	// 这个函数将我们创建的事件广播到整个区块链网络
	// 所有节点都会收到并记录这个事件
	//
	// ⚠️ 重要：事件一旦发出就不能撤回或修改
	err := framework.EmitEvent(event)

	// 🚨 错误处理：检查事件是否发送成功
	if err != nil {
		// 如果发送事件失败，返回错误状态码
		// 这会让调用者知道操作没有成功完成
		return framework.ERROR_EXECUTION_FAILED
	}

	// 🎉 执行成功！
	// 返回成功状态码，告诉调用者一切都正常完成了
	return framework.SUCCESS
}

// ==================== GetGreeting函数 ====================
//
// 🎯 函数作用：返回一个标准的欢迎消息
//
// 💡 学习重点：
// ✅ 理解"查询函数"的概念（只读取，不修改数据）
// ✅ 学习如何向调用者返回字符串数据
// ✅ 区分查询操作和修改操作的差异
//
// 🔍 函数特点：
// - 这是一个纯查询函数，不会改变区块链上的任何数据
// - 不需要发出事件（因为没有状态变化）
// - 主要用于获取信息，就像询问"现在几点了？"
func GetGreeting() uint32 {
	// 📍 步骤1：准备要返回的问候消息
	//
	// 💭 为什么要有固定消息？
	// 这展示了合约可以提供"静态信息"，就像商店门口的营业时间牌
	// 任何人随时都可以查询这个信息
	greeting := "🌟 欢迎来到WES区块链世界！这里是去中心化应用的新起点！"

	// 📍 步骤2：将消息返回给调用者
	//
	// 🔧 SetReturnString()函数：
	// 这个函数将字符串数据发送回调用者
	// 就像服务员把菜单递给顾客一样
	//
	// 💡 与事件的区别：
	// - 返回值：直接发给调用者，只有调用者能看到
	// - 事件：广播到全网络，所有人都能看到
	err := framework.SetReturnString(greeting)
	if err != nil {
		// 如果返回数据失败，返回错误状态
		return framework.ERROR_EXECUTION_FAILED
	}

	// ✅ 查询成功完成
	return framework.SUCCESS
}

// ==================== SetMessage函数 ====================
//
// 🎯 函数作用：允许用户设置自定义消息
//
// 💡 学习重点：
// ✅ 理解如何接收和处理用户输入
// ✅ 学习参数验证的重要性
// ✅ 了解WES的UTXO状态管理模式
//
// 📝 使用方法：
// 调用时传入参数：{"message": "你的自定义消息"}
func SetMessage() uint32 {
	// 📍 步骤1：获取用户传入的参数
	//
	// 💭 什么是合约参数？
	// 当用户调用合约时，可以传入一些数据作为"输入"
	// 就像给自动售货机投币并选择商品一样
	params := framework.GetContractParams()

	// 📍 步骤2：解析JSON格式的参数
	//
	// 🔧 ParseJSON("message")：
	// 从用户传入的JSON数据中提取"message"字段的值
	// 例如：{"message": "Hello"} → 返回 "Hello"
	customMessage := params.ParseJSON("message")

	// 📍 步骤3：验证输入的有效性
	//
	// 🛡️ 为什么要验证？
	// 智能合约一旦部署就不能修改，必须严格检查所有输入
	// 防止恶意或错误的数据破坏合约功能
	if customMessage == "" {
		// 如果消息为空，返回"参数无效"错误
		return framework.ERROR_INVALID_PARAMS
	}

	// 📍 步骤4：记录消息设置操作
	//
	// 🌟 WES特色：UTXO状态管理
	// 在WES中，数据存储通过UTXO（未花费交易输出）机制管理
	// 这里我们通过事件来演示和记录操作
	//
	// 🔍 什么是UTXO？
	// UTXO就像现金，每次交易都会消费旧的"钞票"，产生新的"钞票"
	// 这比传统的"账户余额"模式更安全和灵活

	// 创建"MessageSet"事件
	event := framework.NewEvent("MessageSet")
	event.AddStringField("message", customMessage)              // 记录消息内容
	event.AddAddressField("setter", framework.GetCaller())      // 记录设置者
	event.AddUint64Field("timestamp", framework.GetTimestamp()) // 记录时间

	// 发送事件到区块链网络
	err := framework.EmitEvent(event)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	// ✅ 消息设置成功
	return framework.SUCCESS
}

// ==================== GetMessage函数 ====================
//
// 🎯 函数作用：获取之前设置的自定义消息
//
// 💡 学习重点：
// ✅ 理解状态查询的概念
// ✅ 了解WES的UTXO状态存储机制
// ✅ 区分演示代码和实际生产代码
//
// 📚 UTXO状态管理详解：
// 在WES中，合约状态不是存储在"变量"中，而是通过UTXO来管理
// 这意味着每次状态变化都会创建新的交易输出
func GetMessage() uint32 {
	// 📍 状态查询的实现
	//
	// 💭 为什么返回固定消息？
	// 在真实的WES应用中，这里会：
	// 1. 查询与当前合约相关的UTXO
	// 2. 从UTXO中提取存储的消息数据
	// 3. 返回最新的消息内容
	//
	// 🎯 学习目的：
	// 这里使用固定消息来演示查询功能的基本结构
	// 让初学者先理解"如何返回数据"，再学习复杂的状态管理
	message := "📝 这是通过UTXO机制存储的演示消息 - 在实际应用中，这里会返回用户设置的真实消息"

	// 📍 返回查询结果
	//
	// 🔧 SetReturnString()：
	// 将查询到的消息发送回调用者
	err := framework.SetReturnString(message)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	// ✅ 查询成功
	return framework.SUCCESS
}

// ==================== GetContractInfo函数 ====================
//
// 🎯 函数作用：返回合约的基本信息和元数据
//
// 💡 学习重点：
// ✅ 学习如何返回JSON格式的复杂数据
// ✅ 理解合约元数据的重要性
// ✅ 了解Go语言中的map数据结构
//
// 🔍 使用场景：
// - 让用户了解合约的基本信息
// - 便于其他程序识别和使用合约
// - 提供版本信息用于兼容性检查
func GetContractInfo() uint32 {
	// 📍 步骤1：构建合约信息数据
	//
	// 💭 什么是map？
	// map就像一个字典，通过"键"来查找"值"
	// 例如："name"键对应"Hello World Contract"值
	//
	// 🔧 interface{}类型：
	// 这表示可以存储任何类型的数据（字符串、数字、布尔值等）
	// 就像一个万能盒子，什么都可以装
	info := map[string]interface{}{
		"name":        "🌟 Hello World 智能合约",
		"version":     "1.0.0",
		"description": "WES区块链的第一个入门示例合约，展示基础功能",
		"author":      "WES开发团队",
		"functions":   []string{"SayHello", "GetGreeting", "SetMessage", "GetMessage", "GetContractInfo"},
		"created_at":  "2024年",
		"language":    "Go (TinyGo)",
		"blockchain":  "WES",
	}

	// 📍 步骤2：将信息以JSON格式返回
	//
	// 🔧 SetReturnJSON()函数：
	// 将Go的map数据结构转换为JSON格式并返回给调用者
	// JSON是一种通用的数据交换格式，大多数编程语言都支持
	//
	// 💡 JSON格式示例：
	// {"name": "Hello World", "version": "1.0.0", ...}
	err := framework.SetReturnJSON(info)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	// ✅ 信息返回成功
	return framework.SUCCESS
}

// ==================== 合约入口点 ====================
//
// 💡 重要说明：
// 在TinyGo编译为WASM时，需要有main函数作为程序入口点
// 但实际的合约功能是通过上面定义的各个函数实现的
// ==================== invoke函数 ====================
//
// 🎯 函数作用：合约的默认入口点，在部署时被调用
//
// 💡 工作原理：
// WES在部署合约时会自动调用invoke函数来初始化合约
// 这相当于合约的"开机启动"程序
//
//export invoke
func invoke() uint32 {
	// 📍 合约部署初始化
	// 最简版本：直接返回0，不调用任何framework函数

	// 返回成功状态（直接返回0，不使用framework.SUCCESS）
	return 0
}

func main() {
	// 🎯 这个函数在WASM编译时是必需的
	// 但在WES环境中，实际调用的是上面定义的具体函数
	// 把这里想象成一个"目录"，告诉系统有哪些功能可用
}
