package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/btcsuite/btcutil/base58"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("WES密钥生成工具")
		fmt.Println("用法:")
		fmt.Println("  wes-keygen generate <count>  - 生成指定数量的密钥对")
		fmt.Println("  wes-keygen genesis          - 生成创世块密钥文件")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  wes-keygen generate 5")
		fmt.Println("  wes-keygen genesis")
		return
	}

	switch os.Args[1] {
	case "generate":
		count := 1
		if len(os.Args) >= 3 {
			fmt.Sscanf(os.Args[2], "%d", &count)
		}
		generateKeys(count)
	case "genesis":
		generateGenesisKeys()
	default:
		fmt.Printf("未知命令: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func generateKeys(count int) {
	fmt.Printf("🔑 生成 %d 个密钥对\n", count)
	fmt.Println("====================")

	for i := 0; i < count; i++ {
		privateKey := make([]byte, 32)
		if _, err := rand.Read(privateKey); err != nil {
			log.Fatalf("生成私钥失败: %v", err)
		}

		// 简化的公钥生成（实际项目中应使用正确的椭圆曲线算法）
		publicKey := make([]byte, 33)
		if _, err := rand.Read(publicKey); err != nil {
			log.Fatalf("生成公钥失败: %v", err)
		}
		publicKey[0] = 0x02 // 压缩公钥前缀

		// 生成地址（Base58编码）
		address := base58.Encode(publicKey)

		fmt.Printf("密钥对 %d:\n", i+1)
		fmt.Printf("  私钥: %s\n", hex.EncodeToString(privateKey))
		fmt.Printf("  公钥: %s\n", hex.EncodeToString(publicKey))
		fmt.Printf("  地址: %s\n", address)
		fmt.Println()
	}
}

func generateGenesisKeys() {
	fmt.Println("🌱 生成创世块密钥文件")
	fmt.Println("======================")

	// 生成创世块所需的密钥对
	keys := make(map[string]interface{})

	// 创世块验证者密钥
	validatorPrivateKey := make([]byte, 32)
	if _, err := rand.Read(validatorPrivateKey); err != nil {
		log.Fatalf("生成验证者私钥失败: %v", err)
	}

	validatorPublicKey := make([]byte, 33)
	if _, err := rand.Read(validatorPublicKey); err != nil {
		log.Fatalf("生成验证者公钥失败: %v", err)
	}
	validatorPublicKey[0] = 0x02

	validatorAddress := base58.Encode(validatorPublicKey)

	// 创世块账户密钥
	accountPrivateKey := make([]byte, 32)
	if _, err := rand.Read(accountPrivateKey); err != nil {
		log.Fatalf("生成账户私钥失败: %v", err)
	}

	accountPublicKey := make([]byte, 33)
	if _, err := rand.Read(accountPublicKey); err != nil {
		log.Fatalf("生成账户公钥失败: %v", err)
	}
	accountPublicKey[0] = 0x02

	accountAddress := base58.Encode(accountPublicKey)

	keys["validator"] = map[string]string{
		"private_key": hex.EncodeToString(validatorPrivateKey),
		"public_key":  hex.EncodeToString(validatorPublicKey),
		"address":     validatorAddress,
	}

	keys["genesis_account"] = map[string]string{
		"private_key": hex.EncodeToString(accountPrivateKey),
		"public_key":  hex.EncodeToString(accountPublicKey),
		"address":     accountAddress,
	}

	// 保存到文件
	jsonData, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		log.Fatalf("JSON编码失败: %v", err)
	}

	filename := "genesis_keys.json"
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		log.Fatalf("写入文件失败: %v", err)
	}

	fmt.Printf("✅ 创世块密钥已保存到: %s\n", filename)
	fmt.Println("\n创世块密钥信息:")
	fmt.Printf("验证者地址: %s\n", validatorAddress)
	fmt.Printf("创世账户地址: %s\n", accountAddress)
}

