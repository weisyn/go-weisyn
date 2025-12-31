// Package host provides certificate validation for consortium chain mTLS.
package host

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ============================================================================
//                           证书验证错误类型
// ============================================================================

var (
	// ErrCABundleNotFound CA Bundle 文件不存在
	ErrCABundleNotFound = errors.New("CA bundle file not found")

	// ErrCABundleInvalid CA Bundle 解析失败
	ErrCABundleInvalid = errors.New("CA bundle file is invalid or cannot be parsed")

	// ErrCertChainInvalid 证书链验证失败（无法由联盟 CA 验证）
	ErrCertChainInvalid = errors.New("certificate chain cannot be verified by consortium CA")

	// ErrCertExpired 证书已过期
	ErrCertExpired = errors.New("certificate has expired")

	// ErrCertNotYetValid 证书尚未生效
	ErrCertNotYetValid = errors.New("certificate is not yet valid")

	// ErrSubjectNotAllowed Subject 不在白名单中
	ErrSubjectNotAllowed = errors.New("certificate subject is not in allowed list")

	// ErrOrgNotAllowed 组织不在白名单中
	ErrOrgNotAllowed = errors.New("certificate organization is not in allowed list")

	// ErrNoPeerCertificates 对端未提供证书
	ErrNoPeerCertificates = errors.New("peer did not provide certificates")
)

// CertificateValidationPolicy 证书验证策略
// 定义联盟链的证书验证规则
type CertificateValidationPolicy struct {
	// CA Cert Pool：信任的 CA 证书集合
	CACertPool *x509.CertPool

	// 是否允许中间 CA
	IntermediateAllowed bool

	// 允许的 Subject 白名单（可选）
	AllowedSubjects []string

	// 允许的组织白名单（可选）
	AllowedOrgs []string

	// 当前时间（用于证书有效期检查，测试时可注入）
	Now func() time.Time
}

// NewCertificateValidationPolicy 创建证书验证策略
func NewCertificateValidationPolicy(caCertPool *x509.CertPool, intermediateAllowed bool, allowedSubjects, allowedOrgs []string) *CertificateValidationPolicy {
	return &CertificateValidationPolicy{
		CACertPool:          caCertPool,
		IntermediateAllowed: intermediateAllowed,
		AllowedSubjects:     allowedSubjects,
		AllowedOrgs:         allowedOrgs,
		Now:                 time.Now,
	}
}

// ============================================================================
//                           证书链提取
// ============================================================================

// extractPeerCertChain 从 libp2p 连接中提取对端证书链
//
// 参数：
//   - conn: libp2p 网络连接
//
// 返回：
//   - []*x509.Certificate: 对端证书链（leaf cert 在前）
//   - error: 提取失败的错误
func extractPeerCertChain(conn network.Conn) ([]*x509.Certificate, error) {
	// libp2p 的 TLS 连接可能有多层包装，需要通过反射来获取底层 TLS 连接状态
	// libp2p TLS 实现通常将 tls.ConnectionState 存储在连接的某个字段中

	// 使用反射查找 TLS ConnectionState
	connValue := reflect.ValueOf(conn)
	if connValue.Kind() == reflect.Ptr {
		connValue = connValue.Elem()
	}

	// 递归查找包含 tls.ConnectionState 的字段
	var tlsState *tls.ConnectionState
	found := findTLSConnectionState(connValue, &tlsState)
	if !found {
		return nil, fmt.Errorf("cannot extract TLS connection state from connection: %w", ErrNoPeerCertificates)
	}

	if tlsState == nil {
		return nil, ErrNoPeerCertificates
	}

	// 获取对端证书链
	if len(tlsState.PeerCertificates) == 0 {
		return nil, ErrNoPeerCertificates
	}

	return tlsState.PeerCertificates, nil
}

// findTLSConnectionState 递归查找 TLS ConnectionState
func findTLSConnectionState(v reflect.Value, result **tls.ConnectionState) bool {
	if !v.IsValid() {
		return false
	}

	// 检查当前值是否是指向 tls.ConnectionState 的指针
	if v.Kind() == reflect.Ptr {
		if v.Type().Elem() == reflect.TypeOf(tls.ConnectionState{}) {
			if !v.IsNil() {
				*result = v.Interface().(*tls.ConnectionState)
				return true
			}
		}
		// 递归查找指针指向的值
		if findTLSConnectionState(v.Elem(), result) {
			return true
		}
	}

	// 如果是结构体，递归查找所有字段
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			// 尝试访问字段（包括不可导出字段）
			if field.CanInterface() {
				if findTLSConnectionState(field, result) {
					return true
				}
			} else {
				// 尝试通过反射访问不可导出字段
				if field.CanAddr() {
					if findTLSConnectionState(field, result) {
						return true
					}
				}
			}
		}
	}

	// 如果是接口，查找接口的值
	if v.Kind() == reflect.Interface && !v.IsNil() {
		return findTLSConnectionState(v.Elem(), result)
	}

	return false
}

// ============================================================================
//                           证书链验证
// ============================================================================

// validateConsortiumCertChain 验证证书链是否由联盟 CA 签发
//
// 参数：
//   - certChain: 对端证书链（leaf cert 在前）
//   - policy: 证书验证策略
//
// 返回：
//   - error: 验证失败的错误
func validateConsortiumCertChain(certChain []*x509.Certificate, policy *CertificateValidationPolicy) error {
	if len(certChain) == 0 {
		return ErrNoPeerCertificates
	}

	leafCert := certChain[0]

	// 1. 检查证书有效期
	now := policy.Now()
	if now.Before(leafCert.NotBefore) {
		return fmt.Errorf("%w: certificate valid from %v, current time %v", ErrCertNotYetValid, leafCert.NotBefore, now)
	}
	if now.After(leafCert.NotAfter) {
		return fmt.Errorf("%w: certificate expired at %v, current time %v", ErrCertExpired, leafCert.NotAfter, now)
	}

	// 2. 验证证书链是否由联盟 CA 签发
	// 构建验证选项
	opts := x509.VerifyOptions{
		Roots:         policy.CACertPool,
		CurrentTime:   now,
		Intermediates: x509.NewCertPool(),
	}

	// 如果有中间证书，添加到 intermediates
	if len(certChain) > 1 {
		for _, cert := range certChain[1:] {
			opts.Intermediates.AddCert(cert)
		}
	}

	// 如果不允许中间 CA，且证书链长度 > 1，则验证失败
	if !policy.IntermediateAllowed && len(certChain) > 1 {
		// 检查 leaf cert 是否直接由根 CA 签发
		// 如果链中有中间证书，则不允许
		return fmt.Errorf("%w: intermediate certificates not allowed but found in chain", ErrCertChainInvalid)
	}

	// 执行证书链验证
	_, err := leafCert.Verify(opts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCertChainInvalid, err)
	}

	// 3. 验证 Subject/组织白名单（如果配置）
	if len(policy.AllowedSubjects) > 0 || len(policy.AllowedOrgs) > 0 {
		if err := validateCertSubject(leafCert, policy.AllowedSubjects, policy.AllowedOrgs); err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
//                           Subject/组织白名单检查
// ============================================================================

// validateCertSubject 验证证书 Subject 是否在允许列表中
//
// 参数：
//   - cert: 证书
//   - allowedSubjects: 允许的 Subject 列表（CN 格式，如 "CN=node1.example.com"）
//   - allowedOrgs: 允许的组织列表
//
// 返回：
//   - error: 验证失败的错误
func validateCertSubject(cert *x509.Certificate, allowedSubjects []string, allowedOrgs []string) error {
	// 检查 Subject CN
	if len(allowedSubjects) > 0 {
		subjectCN := cert.Subject.CommonName
		found := false
		for _, allowed := range allowedSubjects {
			// 支持完整 Subject 格式（"CN=node1.example.com"）或仅 CN 值
			if strings.HasPrefix(allowed, "CN=") {
				if allowed == "CN="+subjectCN {
					found = true
					break
				}
			} else {
				if allowed == subjectCN {
					found = true
					break
				}
			}
		}
		if !found && subjectCN != "" {
			return fmt.Errorf("%w: subject CN=%s not in allowed list", ErrSubjectNotAllowed, subjectCN)
		}
	}

	// 检查 Organization
	if len(allowedOrgs) > 0 {
		certOrgs := cert.Subject.Organization
		if len(certOrgs) == 0 {
			return fmt.Errorf("%w: certificate has no organization", ErrOrgNotAllowed)
		}

		found := false
		for _, certOrg := range certOrgs {
			for _, allowedOrg := range allowedOrgs {
				if certOrg == allowedOrg {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			return fmt.Errorf("%w: certificate organizations %v not in allowed list %v", ErrOrgNotAllowed, certOrgs, allowedOrgs)
		}
	}

	return nil
}

// ============================================================================
//                           完整验证函数
// ============================================================================

// ValidatePeerCertificate 验证对端证书（完整流程）
//
// 这是对外暴露的主要验证函数，整合了证书链提取和验证逻辑
//
// 参数：
//   - conn: libp2p 网络连接
//   - policy: 证书验证策略
//   - peerID: 对端 peer ID（用于日志）
//
// 返回：
//   - error: 验证失败的错误
func ValidatePeerCertificate(conn network.Conn, policy *CertificateValidationPolicy, peerID peer.ID) error {
	// 1. 提取证书链
	certChain, err := extractPeerCertChain(conn)
	if err != nil {
		log.Printf("[mTLS] ❌ 证书链提取失败 peer=%s error=%v", peerID, err)
		return fmt.Errorf("failed to extract peer certificate chain for peer %s: %w", peerID, err)
	}

	// 记录证书信息（用于调试）
	if len(certChain) > 0 {
		leafCert := certChain[0]
		log.Printf("[mTLS] 🔍 验证对端证书 peer=%s subject=%s issuer=%s not_before=%v not_after=%v",
			peerID, leafCert.Subject.String(), leafCert.Issuer.String(), leafCert.NotBefore, leafCert.NotAfter)
	}

	// 2. 验证证书链
	if err := validateConsortiumCertChain(certChain, policy); err != nil {
		// 根据错误类型记录不同的日志
		var reason string
		switch {
		case errors.Is(err, ErrCertChainInvalid):
			reason = "证书链无法由联盟 CA 验证"
		case errors.Is(err, ErrCertExpired):
			reason = "证书已过期"
		case errors.Is(err, ErrCertNotYetValid):
			reason = "证书尚未生效"
		case errors.Is(err, ErrSubjectNotAllowed):
			reason = "证书 Subject 不在白名单中"
		case errors.Is(err, ErrOrgNotAllowed):
			reason = "证书组织不在白名单中"
		default:
			reason = "未知错误"
		}
		log.Printf("[mTLS] ❌ 证书验证失败 peer=%s reason=%s error=%v", peerID, reason, err)
		return fmt.Errorf("certificate chain validation failed for peer %s: %w", peerID, err)
	}

	log.Printf("[mTLS] ✅ 证书验证成功 peer=%s", peerID)
	return nil
}
