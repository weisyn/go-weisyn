//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package hsm 提供 HSM（Hardware Security Module）签名器实现
//
// pkcs11_wrapper.go: PKCS#11 CGO 封装
//
// ⚠️ **构建标签**：需要 CGO 支持和 pkcs11 库，排除 Android 平台
// 此文件仅在安装了 github.com/miekg/pkcs11 依赖时编译，且不在 Android 平台编译
//
// 🎯 **核心职责**：封装 PKCS#11 C API，提供 Go 友好的接口
//
// 💡 **设计理念**：
// - 使用 github.com/miekg/pkcs11 库封装 PKCS#11 标准接口
// - 提供 Session 管理和错误处理
// - 支持多种 HSM 厂商（Thales、AWS CloudHSM、YubiHSM等）
//
// 📦 **依赖要求**：
//   go get github.com/miekg/pkcs11
package hsm

import (
	"fmt"

	"github.com/miekg/pkcs11"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// PKCS11Context PKCS#11 上下文封装
//
// 🎯 **核心功能**：封装 PKCS#11 库的基本操作
type PKCS11Context struct {
	ctx    *pkcs11.Ctx    // PKCS#11 上下文
	slotID uint           // Slot ID
	logger log.Logger     // 日志服务
}

// NewPKCS11Context 创建 PKCS#11 上下文
//
// 参数：
//   - libraryPath: PKCS#11 库路径（如 /usr/lib/softhsm/libsofthsm2.so）
//   - logger: 日志服务
//
// 返回：
//   - *PKCS11Context: PKCS#11 上下文实例
//   - error: 初始化失败的原因
func NewPKCS11Context(libraryPath string, logger log.Logger) (*PKCS11Context, error) {
	if libraryPath == "" {
		return nil, fmt.Errorf("PKCS#11库路径不能为空")
	}

	// 1. 加载 PKCS#11 库
	ctx := pkcs11.New(libraryPath)
	if ctx == nil {
		return nil, fmt.Errorf("无法加载PKCS#11库: %s", libraryPath)
	}

	// 2. 初始化库
	if err := ctx.Initialize(); err != nil {
		return nil, fmt.Errorf("PKCS#11初始化失败: %w", err)
	}

	// 3. 获取 Slot 列表（仅获取有 token 的 slot）
	slots, err := ctx.GetSlotList(true)
	if err != nil {
		ctx.Finalize()
		return nil, fmt.Errorf("获取Slot列表失败: %w", err)
	}

	if len(slots) == 0 {
		ctx.Finalize()
		return nil, fmt.Errorf("未找到可用的HSM Slot")
	}

	// 使用第一个可用的 Slot
	slotID := slots[0]

	if logger != nil {
		logger.Infof("✅ PKCS#11上下文初始化成功，库路径: %s, Slot ID: %d", libraryPath, slotID)
	}

	return &PKCS11Context{
		ctx:    ctx,
		slotID: slotID,
		logger: logger,
	}, nil
}

// FindKeyByLabel 根据标签查找密钥对象句柄
//
// 参数：
//   - session: PKCS#11 Session 句柄
//   - label: 密钥标签
//
// 返回：
//   - pkcs11.ObjectHandle: 密钥对象句柄（如果未找到则返回0）
func (c *PKCS11Context) FindKeyByLabel(session pkcs11.SessionHandle, label string) pkcs11.ObjectHandle {
	// 构建查找模板
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}

	// 查找对象
	if err := c.ctx.FindObjectsInit(session, template); err != nil {
		if c.logger != nil {
			c.logger.Errorf("FindObjectsInit失败: %v", err)
		}
		return 0
	}
	defer c.ctx.FindObjectsFinal(session)

	// 获取对象句柄
	handles, _, err := c.ctx.FindObjects(session, 1)
	if err != nil {
		// 如果出错，返回0句柄（调用方需要检查）
		if c.logger != nil {
			c.logger.Errorf("FindObjects失败: %v", err)
		}
		return 0
	}

	if len(handles) == 0 {
		if c.logger != nil {
			c.logger.Errorf("未找到标签为 %s 的密钥", label)
		}
		return 0
	}

	return handles[0]
}

// GetPublicKey 从HSM获取公钥
//
// 参数：
//   - session: PKCS#11 Session 句柄
//   - keyHandle: 密钥对象句柄
//
// 返回：
//   - *transaction.PublicKey: 公钥对象
//   - error: 获取失败的原因
func (c *PKCS11Context) GetPublicKey(session pkcs11.SessionHandle, keyHandle pkcs11.ObjectHandle) (*transaction.PublicKey, error) {
	// 获取公钥属性
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_PUBLIC_KEY_INFO, nil),
	}

	attrs, err := c.ctx.GetAttributeValue(session, keyHandle, template)
	if err != nil {
		return nil, fmt.Errorf("获取公钥属性失败: %w", err)
	}

	if len(attrs) == 0 || len(attrs[0].Value) == 0 {
		return nil, fmt.Errorf("公钥属性为空")
	}

	return &transaction.PublicKey{
		Value: attrs[0].Value,
	}, nil
}

// SignData 使用HSM签名数据
//
// 参数：
//   - session: PKCS#11 Session 句柄
//   - keyHandle: 密钥对象句柄
//   - data: 待签名的数据（已哈希）
//   - mechanism: 签名机制（如 CKM_ECDSA）
//
// 返回：
//   - []byte: 签名字节数组
//   - error: 签名失败的原因
func (c *PKCS11Context) SignData(
	session pkcs11.SessionHandle,
	keyHandle pkcs11.ObjectHandle,
	data []byte,
	mechanism uint,
) ([]byte, error) {
	// 1. 初始化签名操作
	// mechanism 是 uint 类型，NewMechanism 接受 uint 参数
	mech := []*pkcs11.Mechanism{
		pkcs11.NewMechanism(mechanism, nil),
	}

	if err := c.ctx.SignInit(session, mech, keyHandle); err != nil {
		return nil, fmt.Errorf("SignInit失败: %w", err)
	}

	// 2. 执行签名
	signature, err := c.ctx.Sign(session, data)
	if err != nil {
		return nil, fmt.Errorf("Sign失败: %w", err)
	}

	return signature, nil
}

// OpenSession 打开 PKCS#11 Session
//
// 参数：
//   - flags: Session 标志（如 CKF_SERIAL_SESSION | CKF_RW_SESSION）
//
// 返回：
//   - pkcs11.SessionHandle: Session 句柄
//   - error: 打开失败的原因
func (c *PKCS11Context) OpenSession(flags uint) (pkcs11.SessionHandle, error) {
	session, err := c.ctx.OpenSession(c.slotID, flags)
	if err != nil {
		return 0, fmt.Errorf("OpenSession失败: %w", err)
	}
	return session, nil
}

// Login 登录到 HSM
//
// 参数：
//   - session: Session 句柄
//   - pin: PIN 码
//
// 返回：
//   - error: 登录失败的原因
func (c *PKCS11Context) Login(session pkcs11.SessionHandle, pin string) error {
	if err := c.ctx.Login(session, pkcs11.CKU_USER, pin); err != nil {
		return fmt.Errorf("Login失败: %w", err)
	}
	return nil
}

// Logout 登出 HSM
//
// 参数：
//   - session: Session 句柄
func (c *PKCS11Context) Logout(session pkcs11.SessionHandle) error {
	return c.ctx.Logout(session)
}

// CloseSession 关闭 Session
//
// 参数：
//   - session: Session 句柄
func (c *PKCS11Context) CloseSession(session pkcs11.SessionHandle) error {
	return c.ctx.CloseSession(session)
}

// Finalize 清理 PKCS#11 上下文
func (c *PKCS11Context) Finalize() error {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Finalize()
}

// GetSlotID 获取 Slot ID
func (c *PKCS11Context) GetSlotID() uint {
	return c.slotID
}

// GetCtx 获取 PKCS#11 上下文（用于高级操作）
func (c *PKCS11Context) GetCtx() *pkcs11.Ctx {
	return c.ctx
}

// GetSessionInfo 获取 Session 信息（用于检查 Session 有效性）
//
// ✅ **真实实现**：调用 PKCS#11 C_GetSessionInfo API
//
// 参数：
//   - session: Session 句柄
//
// 返回：
//   - pkcs11.SessionInfo: Session 信息（值类型）
//   - error: 获取失败的原因
//
// 用途：
//   - 检查 Session 是否仍然有效
//   - 获取 Session 状态（如是否已登录）
func (c *PKCS11Context) GetSessionInfo(session pkcs11.SessionHandle) (pkcs11.SessionInfo, error) {
	info, err := c.ctx.GetSessionInfo(session)
	if err != nil {
		return pkcs11.SessionInfo{}, fmt.Errorf("GetSessionInfo失败: %w", err)
	}
	return info, nil
}

