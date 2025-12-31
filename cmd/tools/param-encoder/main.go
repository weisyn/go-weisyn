package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/btcsuite/btcutil/base58"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("WES合约参数编码工具")
		fmt.Println("用法:")
		fmt.Println("  wes-param-encoder transfer <to_address> <amount>")
		fmt.Println("  wes-param-encoder balance <address>")
		fmt.Println("  wes-param-encoder approve <spender> <amount>")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  wes-param-encoder transfer CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG 1000")
		fmt.Println("  wes-param-encoder balance CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR")
		return
	}

	switch os.Args[1] {
	case "transfer":
		if len(os.Args) != 4 {
			log.Fatal("transfer需要2个参数: <to_address> <amount>")
		}
		encodeTransfer(os.Args[2], os.Args[3])
	case "balance":
		if len(os.Args) != 3 {
			log.Fatal("balance需要1个参数: <address>")
		}
		encodeBalance(os.Args[2])
	case "approve":
		if len(os.Args) != 4 {
			log.Fatal("approve需要2个参数: <spender> <amount>")
		}
		encodeApprove(os.Args[2], os.Args[3])
	case "transfer_from":
		if len(os.Args) != 5 {
			log.Fatal("transfer_from需要3个参数: <from_address> <to_address> <amount>")
		}
		encodeTransferFrom(os.Args[2], os.Args[3], os.Args[4])
	default:
		log.Fatal("未知操作:", os.Args[1])
	}
}

func encodeTransfer(toAddress, amountStr string) {
	fmt.Printf("🔄 编码转账参数...\n")

	// 解码地址
	toAddrBytes := decodeAddress(toAddress)

	// 解析金额
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		log.Fatal("金额解析失败:", err)
	}

	// 编码金额 (8字节，小端序，考虑8位精度)
	amountWithDecimals := amount * 100000000 // 8位精度
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amountWithDecimals)

	// 合并参数: 接收方地址(20字节) + 转账金额(8字节)
	params := append(toAddrBytes, amountBytes...)

	fmt.Printf("✅ 转账参数编码完成\n")
	fmt.Printf("操作: 转账 %d WES 到 %s\n", amount, toAddress)
	fmt.Printf("十六进制参数: %s\n", hex.EncodeToString(params))
	fmt.Printf("参数长度: %d 字节 (地址20字节 + 金额8字节)\n", len(params))
	fmt.Printf("\n📋 可用于API调用的参数:\n")
	fmt.Printf(`"parameters": "%s"\n`, hex.EncodeToString(params))
}

func encodeBalance(address string) {
	fmt.Printf("📊 编码余额查询参数...\n")

	addrBytes := decodeAddress(address)

	fmt.Printf("✅ 余额查询参数编码完成\n")
	fmt.Printf("操作: 查询 %s 的余额\n", address)
	fmt.Printf("十六进制参数: %s\n", hex.EncodeToString(addrBytes))
	fmt.Printf("参数长度: %d 字节 (地址20字节)\n", len(addrBytes))
	fmt.Printf("\n📋 可用于API调用的参数:\n")
	fmt.Printf(`"parameters": "%s"\n`, hex.EncodeToString(addrBytes))
}

func encodeApprove(spender, amountStr string) {
	fmt.Printf("✅ 编码授权参数...\n")

	spenderBytes := decodeAddress(spender)

	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		log.Fatal("金额解析失败:", err)
	}

	amountWithDecimals := amount * 100000000
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amountWithDecimals)

	// 合并参数: 被授权者地址(20字节) + 授权金额(8字节)
	params := append(spenderBytes, amountBytes...)

	fmt.Printf("✅ 授权参数编码完成\n")
	fmt.Printf("操作: 授权 %s 使用 %d WES\n", spender, amount)
	fmt.Printf("十六进制参数: %s\n", hex.EncodeToString(params))
	fmt.Printf("参数长度: %d 字节 (授权者地址20字节 + 金额8字节)\n", len(params))
	fmt.Printf("\n📋 可用于API调用的参数:\n")
	fmt.Printf(`"parameters": "%s"\n`, hex.EncodeToString(params))
}

func encodeTransferFrom(fromAddress, toAddress, amountStr string) {
	fmt.Printf("🔄 编码代理转账参数...\n")

	fromAddrBytes := decodeAddress(fromAddress)
	toAddrBytes := decodeAddress(toAddress)

	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		log.Fatal("金额解析失败:", err)
	}

	amountWithDecimals := amount * 100000000
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amountWithDecimals)

	// 合并参数: 原始拥有者地址(20字节) + 接收方地址(20字节) + 转账金额(8字节)
	params := append(fromAddrBytes, toAddrBytes...)
	params = append(params, amountBytes...)

	fmt.Printf("✅ 代理转账参数编码完成\n")
	fmt.Printf("操作: 代理转账 %d WES 从 %s 到 %s\n", amount, fromAddress, toAddress)
	fmt.Printf("十六进制参数: %s\n", hex.EncodeToString(params))
	fmt.Printf("参数长度: %d 字节 (from地址20字节 + to地址20字节 + 金额8字节)\n", len(params))
	fmt.Printf("\n📋 可用于API调用的参数:\n")
	fmt.Printf(`"parameters": "%s"\n`, hex.EncodeToString(params))
}

func decodeAddress(address string) []byte {
	fmt.Printf("🔍 解码地址: %s\n", address)

	// 解码Base58地址
	decoded := base58.Decode(address)
	if len(decoded) < 25 { // 至少需要21字节数据 + 4字节校验
		log.Fatal("无效的地址格式:", address)
	}

	// 返回20字节地址 (去掉版本字节和校验和)
	// WES地址格式: [版本1字节][地址20字节][校验和4字节]
	addrBytes := decoded[1:21]

	fmt.Printf("地址字节: %s (20字节)\n", hex.EncodeToString(addrBytes))

	return addrBytes
}

