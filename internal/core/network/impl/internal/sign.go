package internal

// sign.go
// 能力：提供对签名/验签的适配层（如需在 Network 层做 Envelope 级签名校验）。
// 依赖/公共接口：
// - pkg/interfaces/infrastructure/crypto.SignatureManager：统一签名服务
// - pkg/interfaces/infrastructure/crypto.HashManager：统一哈希服务（如需前置摘要）
// 目的：
// - 只做接口编排与封装，不重新实现加密算法，不生成密钥
// 非目标：
// - 不自定义签名格式与算法；遵循项目统一的签名/哈希实现

// Signer 签名适配器（方法框架）
// 说明：封装对 Envelope 或载荷的签名操作
type Signer struct{}

// NewSigner 创建签名适配器
func NewSigner() *Signer { return &Signer{} }

// SignPayload 对载荷进行签名（方法框架）
// 返回：签名字节与错误
func (s *Signer) SignPayload(payload []byte) ([]byte, error) { return nil, nil }

// Verifier 验签适配器（方法框架）
// 说明：封装对 Envelope 或载荷的验签操作
type Verifier struct{}

// NewVerifier 创建验签适配器
func NewVerifier() *Verifier { return &Verifier{} }

// VerifyPayload 验证载荷与签名是否匹配（方法框架）
// 返回：是否通过与错误
func (v *Verifier) VerifyPayload(payload, signature []byte) (bool, error) { return true, nil }
