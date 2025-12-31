package main

import "fmt"

/*
🎯 代币转账应用主程序

这是token-transfer应用的主入口，整合了：
1. 代币转账客户端 (transfer_client.go)
2. 交易构建器 (transaction_builder.go)
3. 钱包管理器 (wallet_manager.go)

运行方式：
go run src/*.go
*/

func main() {
	fmt.Println("🚀 代币转账应用启动")
	fmt.Println("==================")
	fmt.Println()

	// 演示1：钱包管理
	fmt.Println("=== 演示1：钱包管理 ===")
	DemoWalletManager()
	fmt.Println()

	// 演示2：交易构建
	fmt.Println("=== 演示2：交易构建 ===")
	DemoTransactionBuilder()
	fmt.Println()

	// 演示3：代币转账
	fmt.Println("=== 演示3：代币转账 ===")
	DemoTransferFlow()
	fmt.Println()

	fmt.Println("🎉 所有演示完成！")
}
