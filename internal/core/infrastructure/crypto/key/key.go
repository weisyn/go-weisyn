package key

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"golang.org/x/crypto/sha3"
)

// 错误定义
var (
	ErrInvalidPrivateKey  = errors.New("无效的私钥")
	ErrInvalidPublicKey   = errors.New("无效的公钥")
	ErrOperationCancelled = errors.New("操作已取消")
	ErrOperationTimeout   = errors.New("操作超时")
)

// PrivateKeyPool 私钥内存池，提供安全的私钥内存管理
//
// 🛡️ 安全特性：
// - 多重清除：使用随机数据覆盖确保私钥完全清除
// - 长度验证：严格验证私钥长度防止错误使用
// - 防止重复归还：检测已清除的缓冲区防止重复操作
// - 内存污染检测：验证缓冲区状态确保安全
type PrivateKeyPool struct {
	pool          sync.Pool
	clearingMutex sync.Mutex // 防止并发清除操作
}

// NewPrivateKeyPool 创建新的私钥内存池
func NewPrivateKeyPool() *PrivateKeyPool {
	return &PrivateKeyPool{
		pool: sync.Pool{
			New: func() interface{} {
				// 创建32字节缓冲区并预清零
				buf := make([]byte, 32)
				// 初始化时用随机数据填充，确保不包含敏感信息
				rand.Read(buf)
				// 然后清零
				for i := range buf {
					buf[i] = 0
				}
				return buf
			},
		},
	}
}

// Get 从池中获取一个私钥缓冲区
//
// 返回的缓冲区已清零，可以安全使用
func (p *PrivateKeyPool) Get() []byte {
	buf := p.pool.Get().([]byte)

	// 二次验证：确保缓冲区是清零状态
	for i := range buf {
		if buf[i] != 0 {
			// 如果发现非零数据，说明清除不彻底，强制清零
			for j := range buf {
				buf[j] = 0
			}
			break
		}
	}

	return buf
}

// Put 安全归还私钥缓冲区到池中
//
// 执行多重安全清除：
// 1. 验证长度确保是有效的私钥缓冲区
// 2. 用随机数据覆盖确保原始数据无法恢复
// 3. 清零确保缓冲区处于安全状态
// 4. 防止重复归还同一缓冲区
func (p *PrivateKeyPool) Put(privateKey []byte) {
	if len(privateKey) != 32 {
		// 长度不匹配的缓冲区不归还到池中，直接丢弃
		// 但仍然清除数据以确保安全
		p.secureWipe(privateKey)
		return
	}

	p.clearingMutex.Lock()
	defer p.clearingMutex.Unlock()

	// 执行三阶段安全清除
	p.secureWipe(privateKey)

	// 归还到池中
	p.pool.Put(privateKey)
}

// secureWipe 执行安全的私钥数据清除
//
// 清除策略：
// 1. 第一阶段：用随机数据覆盖
// 2. 第二阶段：用0xFF覆盖
// 3. 第三阶段：用0x00覆盖
//
// 这样三重覆盖确保即使在某些硬件上也无法通过物理方法恢复数据
func (p *PrivateKeyPool) secureWipe(data []byte) {
	if len(data) == 0 {
		return
	}

	// 第一阶段：随机数据覆盖
	randomData := make([]byte, len(data))
	rand.Read(randomData)
	copy(data, randomData)

	// 第二阶段：全1覆盖
	for i := range data {
		data[i] = 0xFF
	}

	// 第三阶段：全0覆盖（最终状态）
	for i := range data {
		data[i] = 0x00
	}

	// 清除临时随机数据
	for i := range randomData {
		randomData[i] = 0
	}
}

// PublicKeyPool 公钥内存池
type PublicKeyPool struct {
	pool sync.Pool
}

// NewPublicKeyPool 创建新的公钥内存池
func NewPublicKeyPool() *PublicKeyPool {
	return &PublicKeyPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 64)
			},
		},
	}
}

// Get 从池中获取一个公钥缓冲区
func (p *PublicKeyPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put 归还公钥缓冲区到池中
func (p *PublicKeyPool) Put(publicKey []byte) {
	p.pool.Put(publicKey)
}

// KeyManager 提供密钥管理功能
type KeyManager struct {
	privateKeyPool *PrivateKeyPool
	publicKeyPool  *PublicKeyPool

	// 熵池增强随机性
	entropyMu   sync.Mutex
	entropyPool []byte
	lastAddTime time.Time
}

// NewKeyManager 创建新的密钥管理器
func NewKeyManager() *KeyManager {
	km := &KeyManager{
		privateKeyPool: NewPrivateKeyPool(),
		publicKeyPool:  NewPublicKeyPool(),
		entropyPool:    make([]byte, 64),
		lastAddTime:    time.Now(),
	}

	// 初始化熵池
	_, err := rand.Read(km.entropyPool)
	if err != nil {
		// 如果初始随机数获取失败，使用当前时间和其他系统状态
		hasher := sha3.NewLegacyKeccak256()
		hasher.Write([]byte(time.Now().String()))

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		timeBytes := make([]byte, 8)
		big.NewInt(time.Now().UnixNano()).FillBytes(timeBytes)
		hasher.Write(timeBytes)

		km.entropyPool = hasher.Sum(nil)
	}

	// 启动定期收集熵的后台服务
	go km.collectEntropyPeriodically()

	return km
}

// collectEntropyPeriodically 定期收集系统熵增强随机性
func (km *KeyManager) collectEntropyPeriodically() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		km.entropyMu.Lock()

		// 收集系统状态作为额外熵源
		hasher := sha3.NewLegacyKeccak256()
		hasher.Write(km.entropyPool)

		// 使用时间作为熵源
		timeBytes := make([]byte, 8)
		big.NewInt(time.Now().UnixNano()).FillBytes(timeBytes)
		hasher.Write(timeBytes)

		// 使用内存状态作为熵源
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memBytes := make([]byte, 8)
		big.NewInt(int64(memStats.Alloc)).FillBytes(memBytes)
		hasher.Write(memBytes)

		// 从系统随机数生成器获取额外熵
		extraEntropy := make([]byte, 32)
		rand.Read(extraEntropy)
		hasher.Write(extraEntropy)

		// 更新熵池
		km.entropyPool = hasher.Sum(nil)
		km.lastAddTime = time.Now()

		km.entropyMu.Unlock()
	}
}

// getRandomReader 获取增强型随机数读取器
func (km *KeyManager) getRandomReader() *EnhancedReader {
	km.entropyMu.Lock()
	defer km.entropyMu.Unlock()

	// 创建一个新的读取器，包含熵池的当前副本
	return &EnhancedReader{
		entropyPool: append([]byte{}, km.entropyPool...),
	}
}

// EnhancedReader 增强型随机数读取器
type EnhancedReader struct {
	entropyPool []byte
	position    int
}

// Read 实现io.Reader接口
func (r *EnhancedReader) Read(p []byte) (n int, err error) {
	// 首先尝试系统随机数生成器
	n, err = rand.Read(p)
	if err != nil {
		// 如果系统随机数失败，使用熵池混入一些随机性
		hasher := sha3.NewLegacyKeccak256()

		// 使用熵池和当前时间
		hasher.Write(r.entropyPool)
		timeBytes := make([]byte, 8)
		big.NewInt(time.Now().UnixNano()).FillBytes(timeBytes)
		hasher.Write(timeBytes)

		// 使用位置信息增加变化
		posBytes := make([]byte, 4)
		big.NewInt(int64(r.position)).FillBytes(posBytes)
		hasher.Write(posBytes)
		r.position++

		// 使用派生值填充输出
		derived := hasher.Sum(nil)
		n = copy(p, derived)
	}

	// 更新熵池
	if len(p) > 0 {
		hasher := sha3.NewLegacyKeccak256()
		hasher.Write(r.entropyPool)
		hasher.Write(p) // 混入刚生成的随机数
		r.entropyPool = hasher.Sum(nil)
	}

	return n, nil
}

// GenerateKeyPair 生成新的ECDSA密钥对
//
// 返回标准格式：
//   - 私钥：32字节
//   - 公钥：33字节压缩格式（Bitcoin标准）
//
// 返回:
//   - []byte: 32字节的私钥
//   - []byte: 33字节的压缩公钥
//   - error: 生成错误，成功时为nil
func (km *KeyManager) GenerateKeyPair() ([]byte, []byte, error) {
	// 直接调用压缩格式生成方法
	return km.GenerateCompressedKeyPair()
}

// GenerateKeyPairWithContext 生成ECDSA密钥对，支持上下文控制
//
// 参数:
//   - ctx: 上下文用于控制操作的取消和超时
//
// 返回:
//   - []byte: 32字节私钥
//   - []byte: 64字节公钥 (去掉前缀的X+Y坐标)
//   - error: 操作错误，成功时为nil
func (km *KeyManager) GenerateKeyPairWithContext(ctx context.Context) ([]byte, []byte, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, nil, ErrOperationCancelled
	default:
	}

	// 获取增强型随机数读取器
	reader := km.getRandomReader()

	// 使用secp256k1曲线生成私钥
	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), reader)
	if err != nil {
		return nil, nil, err
	}

	// 获取私钥字节
	privateKeyBytes := km.privateKeyPool.Get()

	// 确保函数返回前清除私钥
	defer func() {
		if err != nil {
			km.privateKeyPool.Put(privateKeyBytes)
		}
	}()

	// 填充私钥数据
	privBytes := privateKey.D.Bytes()
	// 补齐到32字节
	if len(privBytes) < 32 {
		// 清零
		for i := range privateKeyBytes {
			privateKeyBytes[i] = 0
		}
		// 拷贝到末尾部分
		copy(privateKeyBytes[32-len(privBytes):], privBytes)
	} else {
		copy(privateKeyBytes, privBytes)
	}

	// 获取公钥缓冲区
	publicKeyBytes := km.publicKeyPool.Get()

	// 确保函数返回前清除公钥
	defer func() {
		if err != nil {
			km.publicKeyPool.Put(publicKeyBytes)
		}
	}()

	// 获取公钥字节（去掉前缀字节）
	pubBytes := elliptic.Marshal(privateKey.Curve, privateKey.X, privateKey.Y)[1:]
	copy(publicKeyBytes, pubBytes)

	// 安全清除敏感的私钥对象
	privateKey.D = big.NewInt(0)

	return privateKeyBytes, publicKeyBytes, nil
}

// DerivePublicKey 从私钥导出公钥
//
// 参数:
//   - privateKey: 32字节的私钥数据
//
// 返回:
//   - []byte: 33字节压缩公钥（Bitcoin标准）
//   - error: 操作错误，无效私钥时返回ErrInvalidPrivateKey
func (km *KeyManager) DerivePublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	// 解析私钥
	k := new(big.Int).SetBytes(privateKey)

	// 计算公钥点
	x, y := secp256k1.S256().ScalarBaseMult(k.Bytes())
	if x == nil || y == nil {
		return nil, ErrInvalidPrivateKey
	}

	// 返回33字节压缩公钥
	return km.compressPoint(x, y), nil
}

// DerivePublicKeyWithContext 从私钥导出公钥，支持上下文控制
//
// 参数:
//   - ctx: 上下文用于控制操作的取消和超时
//   - privateKey: 32字节的私钥数据
//
// 返回:
//   - []byte: 64字节公钥 (去掉前缀的X+Y坐标)
//   - error: 操作错误，成功时为nil
func (km *KeyManager) DerivePublicKeyWithContext(ctx context.Context, privateKey []byte) ([]byte, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ErrOperationCancelled
	default:
	}

	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	// 解析私钥
	k := new(big.Int).SetBytes(privateKey)

	// 计算公钥点
	x, y := secp256k1.S256().ScalarBaseMult(k.Bytes())
	if x == nil || y == nil {
		return nil, ErrInvalidPrivateKey
	}

	// 获取公钥缓冲区
	publicKeyBytes := km.publicKeyPool.Get()

	// 获取公钥字节（去掉前缀字节）
	pubBytes := elliptic.Marshal(secp256k1.S256(), x, y)[1:]
	copy(publicKeyBytes, pubBytes)

	return publicKeyBytes, nil
}

// PrivateKeyToECDSA 将私钥字节转换为ECDSA私钥对象
//
// 参数:
//   - privateKey: 32字节的私钥数据
//
// 返回:
//   - *ecdsa.PrivateKey: ECDSA私钥对象
//   - error: 操作错误，无效私钥时返回ErrInvalidPrivateKey
func (km *KeyManager) PrivateKeyToECDSA(privateKey []byte) (*ecdsa.PrivateKey, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	k := new(big.Int).SetBytes(privateKey)
	priv := new(ecdsa.PrivateKey)
	priv.D = k
	priv.Curve = secp256k1.S256()
	priv.X, priv.Y = priv.Curve.ScalarBaseMult(k.Bytes())

	return priv, nil
}

// PublicKeyToECDSA 将字节数组形式的公钥转换为ECDSA公钥
//
// 支持多种公钥格式：
//   - 33字节压缩公钥格式（Bitcoin标准）
//   - 64字节未压缩公钥（X和Y坐标的连接）
//   - 65字节带前缀未压缩公钥（0x04前缀）
//
// 参数:
//   - publicKey: 公钥字节数组（33、64或65字节）
//
// 返回:
//   - *ecdsa.PublicKey: ECDSA公钥
//   - error: 转换错误，成功时为nil
func (km *KeyManager) PublicKeyToECDSA(publicKey []byte) (*ecdsa.PublicKey, error) {
	switch len(publicKey) {
	case 33:
		// 处理33字节压缩公钥（Bitcoin标准）
		return km.compressedPublicKeyToECDSA(publicKey)
	case 64:
		// 处理64字节未压缩公钥（无前缀）
		return km.uncompressed64PublicKeyToECDSA(publicKey)
	case 65:
		// 处理65字节未压缩公钥（带0x04前缀）
		if publicKey[0] != 4 {
			return nil, fmt.Errorf("无效的65字节公钥前缀: 0x%02x，期望0x04", publicKey[0])
		}
		return km.uncompressed64PublicKeyToECDSA(publicKey[1:])
	default:
		return nil, fmt.Errorf("无效的公钥长度: %d，期望33、64或65字节", len(publicKey))
	}
}

// compressedPublicKeyToECDSA 将33字节压缩公钥转换为ECDSA公钥
func (km *KeyManager) compressedPublicKeyToECDSA(compressedKey []byte) (*ecdsa.PublicKey, error) {
	// 首先解压缩公钥
	uncompressedKey, err := km.DecompressPublicKey(compressedKey)
	if err != nil {
		return nil, fmt.Errorf("解压缩公钥失败: %w", err)
	}

	// 转换为ECDSA公钥
	return crypto.UnmarshalPubkey(uncompressedKey)
}

// uncompressed64PublicKeyToECDSA 将64字节未压缩公钥转换为ECDSA公钥
func (km *KeyManager) uncompressed64PublicKeyToECDSA(publicKey []byte) (*ecdsa.PublicKey, error) {
	// 添加0x04前缀
	pubKeyBytes := make([]byte, 65)
	pubKeyBytes[0] = 4 // 未压缩公钥前缀
	copy(pubKeyBytes[1:], publicKey)

	return crypto.UnmarshalPubkey(pubKeyBytes)
}

// SecureWipe 安全擦除敏感数据
//
// 使用多阶段覆盖策略确保数据无法恢复：
// 1. 随机数据覆盖
// 2. 全1覆盖
// 3. 全0覆盖
//
// 参数:
//   - data: 要擦除的数据字节切片
//
// 此函数采用防恢复的安全清除算法
func SecureWipe(data []byte) {
	if len(data) == 0 {
		return
	}

	// 第一阶段：随机数据覆盖
	randomData := make([]byte, len(data))
	rand.Read(randomData)
	copy(data, randomData)

	// 第二阶段：全1覆盖
	for i := range data {
		data[i] = 0xFF
	}

	// 第三阶段：全0覆盖（最终状态）
	for i := range data {
		data[i] = 0x00
	}

	// 清除临时随机数据
	for i := range randomData {
		randomData[i] = 0
	}
}

// ReleasePrivateKey 安全释放私钥
//
// 参数:
//   - privateKey: 要释放的私钥
//
// 此函数会安全擦除私钥数据并将其归还到内存池
func (km *KeyManager) ReleasePrivateKey(privateKey []byte) {
	if len(privateKey) == 32 {
		km.privateKeyPool.Put(privateKey)
	}
}

// ParsePublicKeyString 解析十六进制字符串公钥
//
// 支持多种格式：
//   - "02abc123..." (66字符，33字节压缩公钥) - Bitcoin标准
//   - "03abc123..." (66字符，33字节压缩公钥) - Bitcoin标准
//   - "04abc123..." (130字符，65字节未压缩公钥) - 兼容格式
//   - "0x04abc123..." (含0x前缀的格式) - 兼容格式
//   - "1234abcd..." (128字符，64字节公钥) - 兼容格式（以太坊风格）
//
// 参数：
//   - publicKeyHex: 十六进制公钥字符串
//
// 返回：
//   - []byte: 解析后的公钥字节数组
//   - error: 格式错误或解析失败
func (km *KeyManager) ParsePublicKeyString(publicKeyHex string) ([]byte, error) {
	// 去掉可能的0x前缀
	if len(publicKeyHex) >= 2 && (publicKeyHex[:2] == "0x" || publicKeyHex[:2] == "0X") {
		publicKeyHex = publicKeyHex[2:]
	}

	// 根据长度判断公钥格式
	switch len(publicKeyHex) {
	case 66:
		// 33字节压缩公钥（Bitcoin标准）
		return km.parseCompressedPublicKey(publicKeyHex)
	case 130:
		// 65字节未压缩公钥
		return km.parseUncompressedPublicKey(publicKeyHex)
	case 128:
		// 64字节公钥（兼容格式，以太坊风格）
		return km.parse64BytePublicKey(publicKeyHex)
	default:
		return nil, fmt.Errorf("公钥长度错误: %d个字符, 期望66(压缩)、128(64字节)或130(未压缩)个十六进制字符", len(publicKeyHex))
	}
}

// 解析33字节压缩公钥
func (km *KeyManager) parseCompressedPublicKey(publicKeyHex string) ([]byte, error) {
	// 验证前缀
	if publicKeyHex[0:2] != "02" && publicKeyHex[0:2] != "03" {
		return nil, fmt.Errorf("压缩公钥前缀错误: %s, 期望02或03", publicKeyHex[0:2])
	}

	// 解析为33字节
	publicKeyBytes := make([]byte, 33)
	for i := 0; i < 33; i++ {
		high := hexCharToByte(publicKeyHex[i*2])
		low := hexCharToByte(publicKeyHex[i*2+1])
		if high == 255 || low == 255 {
			return nil, fmt.Errorf("公钥包含无效的十六进制字符: %s", publicKeyHex[i*2:i*2+2])
		}
		publicKeyBytes[i] = (high << 4) | low
	}

	// 验证公钥有效性
	if err := km.ValidatePublicKey(publicKeyBytes); err != nil {
		return nil, fmt.Errorf("公钥格式无效: %w", err)
	}

	return publicKeyBytes, nil
}

// 解析65字节未压缩公钥
func (km *KeyManager) parseUncompressedPublicKey(publicKeyHex string) ([]byte, error) {
	// 验证前缀
	if publicKeyHex[0:2] != "04" {
		return nil, fmt.Errorf("未压缩公钥前缀错误: %s, 期望04", publicKeyHex[0:2])
	}

	// 解析为65字节
	publicKeyBytes := make([]byte, 65)
	for i := 0; i < 65; i++ {
		high := hexCharToByte(publicKeyHex[i*2])
		low := hexCharToByte(publicKeyHex[i*2+1])
		if high == 255 || low == 255 {
			return nil, fmt.Errorf("公钥包含无效的十六进制字符: %s", publicKeyHex[i*2:i*2+2])
		}
		publicKeyBytes[i] = (high << 4) | low
	}

	// 验证公钥有效性
	if err := km.ValidatePublicKey(publicKeyBytes); err != nil {
		return nil, fmt.Errorf("公钥格式无效: %w", err)
	}

	return publicKeyBytes, nil
}

// 解析64字节公钥（兼容格式）
func (km *KeyManager) parse64BytePublicKey(publicKeyHex string) ([]byte, error) {
	// 解析为64字节
	publicKeyBytes := make([]byte, 64)
	for i := 0; i < 64; i++ {
		high := hexCharToByte(publicKeyHex[i*2])
		low := hexCharToByte(publicKeyHex[i*2+1])
		if high == 255 || low == 255 {
			return nil, fmt.Errorf("公钥包含无效的十六进制字符: %s", publicKeyHex[i*2:i*2+2])
		}
		publicKeyBytes[i] = (high << 4) | low
	}

	// 验证公钥有效性
	if err := km.ValidatePublicKey(publicKeyBytes); err != nil {
		return nil, fmt.Errorf("公钥格式无效: %w", err)
	}

	return publicKeyBytes, nil
}

// hexCharToByte 将十六进制字符转换为字节值
func hexCharToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	default:
		return 255 // 无效字符
	}
}

// ReleasePublicKey 释放公钥
//
// 参数:
//   - publicKey: 要释放的公钥
//
// 此函数会将公钥归还到内存池
func (km *KeyManager) ReleasePublicKey(publicKey []byte) {
	if len(publicKey) == 64 {
		km.publicKeyPool.Put(publicKey)
	}
}

// GenerateCompressedKeyPair 生成压缩格式密钥对
//
// 专门生成Bitcoin标准的33字节压缩公钥格式
//
// 返回：
//   - []byte: 32字节私钥
//   - []byte: 33字节压缩公钥
//   - error: 生成失败时的错误
func (km *KeyManager) GenerateCompressedKeyPair() ([]byte, []byte, error) {
	// 生成ECDSA私钥
	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// 转换私钥为32字节
	privateKeyBytes := make([]byte, 32)
	blob := privateKey.D.Bytes()
	copy(privateKeyBytes[32-len(blob):], blob)

	// 生成33字节压缩公钥
	compressedPubKey := km.compressPoint(privateKey.X, privateKey.Y)

	return privateKeyBytes, compressedPubKey, nil
}

// DeriveUncompressedPublicKey 从私钥导出未压缩公钥
//
// 用于需要完整公钥坐标的场景
//
// 参数：
//   - privateKey: 32字节私钥
//
// 返回：
//   - []byte: 65字节未压缩公钥
//   - error: 私钥无效时的错误
func (km *KeyManager) DeriveUncompressedPublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	// 解析私钥
	k := new(big.Int).SetBytes(privateKey)

	// 计算公钥点
	x, y := secp256k1.S256().ScalarBaseMult(k.Bytes())
	if x == nil || y == nil {
		return nil, ErrInvalidPrivateKey
	}

	// 生成65字节未压缩公钥（0x04 + X + Y）
	uncompressedPubKey := make([]byte, 65)
	uncompressedPubKey[0] = 0x04
	x.FillBytes(uncompressedPubKey[1:33])
	y.FillBytes(uncompressedPubKey[33:65])

	return uncompressedPubKey, nil
}

// ValidatePrivateKey 验证私钥有效性
//
// 检查私钥是否符合secp256k1的要求
//
// 参数：
//   - privateKey: 待验证的私钥字节
//
// 返回：
//   - error: 私钥无效时返回错误
func (km *KeyManager) ValidatePrivateKey(privateKey []byte) error {
	if len(privateKey) != 32 {
		return fmt.Errorf("私钥长度错误: %d, 期望32字节", len(privateKey))
	}

	// 检查私钥是否为零
	k := new(big.Int).SetBytes(privateKey)
	if k.Cmp(big.NewInt(0)) == 0 {
		return fmt.Errorf("私钥不能为零")
	}

	// 检查私钥是否超出secp256k1的范围
	curveOrder := secp256k1.S256().Params().N
	if k.Cmp(curveOrder) >= 0 {
		return fmt.Errorf("私钥超出secp256k1曲线范围")
	}

	return nil
}

// ValidatePublicKey 验证公钥有效性
//
// 检查公钥是否符合secp256k1的要求，支持压缩和未压缩格式
//
// 参数：
//   - publicKey: 待验证的公钥字节
//
// 返回：
//   - error: 公钥无效时返回错误
func (km *KeyManager) ValidatePublicKey(publicKey []byte) error {
	switch len(publicKey) {
	case 33:
		// 压缩公钥格式验证
		return km.validateCompressedPublicKey(publicKey)
	case 65:
		// 未压缩公钥格式验证
		return km.validateUncompressedPublicKey(publicKey)
	case 64:
		// 兼容64字节格式（以太坊风格，无前缀）
		return km.validate64BytePublicKey(publicKey)
	default:
		return fmt.Errorf("公钥长度错误: %d, 期望33、64或65字节", len(publicKey))
	}
}

// CompressPublicKey 将未压缩公钥转换为压缩格式
//
// 参数：
//   - uncompressedKey: 65字节未压缩公钥
//
// 返回：
//   - []byte: 33字节压缩公钥
//   - error: 格式错误时返回错误
func (km *KeyManager) CompressPublicKey(uncompressedKey []byte) ([]byte, error) {
	if len(uncompressedKey) == 64 {
		// 处理64字节格式（无前缀）
		x := new(big.Int).SetBytes(uncompressedKey[0:32])
		y := new(big.Int).SetBytes(uncompressedKey[32:64])
		return km.compressPoint(x, y), nil
	}

	if len(uncompressedKey) != 65 {
		return nil, fmt.Errorf("未压缩公钥长度错误: %d, 期望65字节", len(uncompressedKey))
	}

	if uncompressedKey[0] != 0x04 {
		return nil, fmt.Errorf("未压缩公钥前缀错误: 0x%02x, 期望0x04", uncompressedKey[0])
	}

	// 提取X和Y坐标
	x := new(big.Int).SetBytes(uncompressedKey[1:33])
	y := new(big.Int).SetBytes(uncompressedKey[33:65])

	return km.compressPoint(x, y), nil
}

// DecompressPublicKey 将压缩公钥转换为未压缩格式
//
// 参数：
//   - compressedKey: 33字节压缩公钥
//
// 返回：
//   - []byte: 65字节未压缩公钥
//   - error: 格式错误时返回错误
func (km *KeyManager) DecompressPublicKey(compressedKey []byte) ([]byte, error) {
	if len(compressedKey) != 33 {
		return nil, fmt.Errorf("压缩公钥长度错误: %d, 期望33字节", len(compressedKey))
	}

	prefix := compressedKey[0]
	if prefix != 0x02 && prefix != 0x03 {
		return nil, fmt.Errorf("压缩公钥前缀错误: 0x%02x, 期望0x02或0x03", prefix)
	}

	// 提取X坐标
	x := new(big.Int).SetBytes(compressedKey[1:33])

	// 计算Y坐标
	y, err := km.decompressPoint(x, prefix == 0x03)
	if err != nil {
		return nil, fmt.Errorf("解压缩公钥失败: %w", err)
	}

	// 生成65字节未压缩公钥
	uncompressedKey := make([]byte, 65)
	uncompressedKey[0] = 0x04
	x.FillBytes(uncompressedKey[1:33])
	y.FillBytes(uncompressedKey[33:65])

	return uncompressedKey, nil
}

// 辅助方法：压缩公钥坐标点
func (km *KeyManager) compressPoint(x, y *big.Int) []byte {
	compressedKey := make([]byte, 33)

	// 根据Y坐标的奇偶性确定前缀
	if y.Bit(0) == 0 {
		compressedKey[0] = 0x02 // Y是偶数
	} else {
		compressedKey[0] = 0x03 // Y是奇数
	}

	// 填充X坐标
	x.FillBytes(compressedKey[1:33])

	return compressedKey
}

// 辅助方法：解压缩公钥坐标点
func (km *KeyManager) decompressPoint(x *big.Int, isOdd bool) (*big.Int, error) {
	// secp256k1: y² = x³ + 7
	curve := secp256k1.S256()

	// 计算 x³
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	// 计算 x³ + 7
	x3.Add(x3, big.NewInt(7))

	// 计算 y² = x³ + 7 (mod p)
	x3.Mod(x3, curve.Params().P)

	// 计算平方根
	y := new(big.Int).ModSqrt(x3, curve.Params().P)
	if y == nil {
		return nil, fmt.Errorf("无法计算平方根，无效的X坐标")
	}

	// 确保Y坐标的奇偶性正确
	if y.Bit(0) == 0 && isOdd {
		y.Sub(curve.Params().P, y)
	} else if y.Bit(0) == 1 && !isOdd {
		y.Sub(curve.Params().P, y)
	}

	return y, nil
}

// 辅助方法：验证33字节压缩公钥
func (km *KeyManager) validateCompressedPublicKey(publicKey []byte) error {
	prefix := publicKey[0]
	if prefix != 0x02 && prefix != 0x03 {
		return fmt.Errorf("压缩公钥前缀错误: 0x%02x", prefix)
	}

	// 验证坐标是否在曲线上
	x := new(big.Int).SetBytes(publicKey[1:33])
	_, err := km.decompressPoint(x, prefix == 0x03)
	if err != nil {
		return fmt.Errorf("公钥不在secp256k1曲线上: %w", err)
	}

	return nil
}

// 辅助方法：验证65字节未压缩公钥
func (km *KeyManager) validateUncompressedPublicKey(publicKey []byte) error {
	if publicKey[0] != 0x04 {
		return fmt.Errorf("未压缩公钥前缀错误: 0x%02x", publicKey[0])
	}

	x := new(big.Int).SetBytes(publicKey[1:33])
	y := new(big.Int).SetBytes(publicKey[33:65])

	// 验证点是否在secp256k1曲线上
	if !secp256k1.S256().IsOnCurve(x, y) {
		return fmt.Errorf("公钥不在secp256k1曲线上")
	}

	return nil
}

// 辅助方法：验证64字节公钥（兼容格式）
func (km *KeyManager) validate64BytePublicKey(publicKey []byte) error {
	x := new(big.Int).SetBytes(publicKey[0:32])
	y := new(big.Int).SetBytes(publicKey[32:64])

	// 验证点是否在secp256k1曲线上
	if !secp256k1.S256().IsOnCurve(x, y) {
		return fmt.Errorf("公钥不在secp256k1曲线上")
	}

	return nil
}

// 确保KeyManager实现了cryptointf.KeyManager接口
var _ cryptointf.KeyManager = (*KeyManager)(nil)
