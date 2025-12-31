//go:build js || wasm
// +build js wasm

package signature

import (
	"crypto/ecdsa"
	"errors"
	"math/big"

	"github.com/weisyn/v1/internal/core/infrastructure/crypto/secp256k1"
)

// verifySignatureSecp256k1 使用Go标准库验证签名 (WebAssembly版本)
func verifySignatureSecp256k1(pubKey, msgHash, signature []byte) bool {
	// 在WASM环境中，使用Go标准库的ECDSA验证
	if len(pubKey) != 65 || pubKey[0] != 4 {
		return false
	}

	if len(signature) != 64 {
		return false
	}

	// 解析公钥
	x := new(big.Int).SetBytes(pubKey[1:33])
	y := new(big.Int).SetBytes(pubKey[33:65])

	curve := secp256k1.NewCurve()
	publicKey := &ecdsa.PublicKey{
		Curve: curve.S256(),
		X:     x,
		Y:     y,
	}

	// 解析签名
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])

	// 验证签名
	return ecdsa.Verify(publicKey, msgHash, r, s)
}

// recoverPubkeySecp256k1 从签名恢复公钥 (WebAssembly版本)
func recoverPubkeySecp256k1(hash, signature []byte) ([]byte, error) {
	// 在WASM环境中，公钥恢复功能受限
	// 这里返回一个错误，表示该功能在WASM中不可用
	return nil, errors.New("公钥恢复功能在WebAssembly环境中不可用")
}
