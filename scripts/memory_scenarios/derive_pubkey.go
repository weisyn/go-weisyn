package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "用法: %s <private_key_hex>\n", os.Args[0])
		os.Exit(1)
	}

	privateKeyHex := os.Args[1]
	
	// 移除可能的 0x 前缀
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "0x" {
		privateKeyHex = privateKeyHex[2:]
	}
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "Cf" {
		privateKeyHex = privateKeyHex[2:]
	}

	// 解码私钥
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: 私钥解码失败: %v\n", err)
		os.Exit(1)
	}

	// 验证私钥长度
	if len(privateKeyBytes) != 32 {
		fmt.Fprintf(os.Stderr, "ERROR: 私钥长度无效: 期望32字节, 实际%d字节\n", len(privateKeyBytes))
		os.Exit(1)
	}

	// 转换为 ECDSA 私钥
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: 私钥转换失败: %v\n", err)
		os.Exit(1)
	}

	// 获取压缩公钥（33字节）
	compressedPubKey := crypto.CompressPubkey(&privateKey.PublicKey)

	// 编码为十六进制字符串
	publicKeyHex := hex.EncodeToString(compressedPubKey)
	fmt.Println(publicKeyHex)
}

