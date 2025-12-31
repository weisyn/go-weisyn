//go:build !js && !wasm
// +build !js,!wasm

package signature

import (
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/secp256k1"
)

// verifySignatureSecp256k1 使用secp256k1库验证签名 (Unix/Linux版本)
func verifySignatureSecp256k1(pubKey, msgHash, signature []byte) bool {
	curve := secp256k1.NewCurve()
	return curve.VerifySignature(pubKey, msgHash, signature)
}

// recoverPubkeySecp256k1 从签名恢复公钥 (Unix/Linux版本)
func recoverPubkeySecp256k1(hash, signature []byte) ([]byte, error) {
	curve := secp256k1.NewCurve()
	return curve.RecoverPubkey(hash, signature)
}
