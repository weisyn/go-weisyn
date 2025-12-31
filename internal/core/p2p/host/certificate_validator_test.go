// Package host provides unit tests for certificate validation.
package host

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
//                           测试辅助函数：证书生成
// ============================================================================

// createTestCA 创建测试用的 CA 证书
func createTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Consortium CA"},
			Country:       []string{"US"},
			Province:      []string{"CA"},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{},
			PostalCode:    []string{},
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	return caCert, caKey
}

// createTestCert 创建由指定 CA 签发的测试证书
func createTestCert(t *testing.T, caCert *x509.Certificate, caKey *rsa.PrivateKey, cn string, orgs []string, notBefore, notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey) {
	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: orgs,
			Country:      []string{"US"},
		},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{cn},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &certKey.PublicKey, caKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return cert, certKey
}

// ============================================================================
//                           测试辅助函数：证书生成（已实现）
// ============================================================================
// 注意：extractPeerCertChain 和 ValidatePeerCertificate 的完整测试
// 需要在真实的 libp2p 环境中进行，因为需要真实的 TLS 连接。
// 这里我们主要测试证书验证逻辑本身（validateConsortiumCertChain）。

// ============================================================================
//                           单元测试：证书链验证
// ============================================================================

// TestValidateConsortiumCertChain_Valid 测试正常情况：同一联盟 CA 签发的证书验证通过
func TestValidateConsortiumCertChain_Valid(t *testing.T) {
	// 1. 创建 CA
	caCert, caKey := createTestCA(t)

	// 2. 创建 CA Cert Pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// 3. 创建由该 CA 签发的证书
	validCert, _ := createTestCert(t, caCert, caKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建验证策略
	policy := NewCertificateValidationPolicy(caPool, true, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{validCert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	assert.NoError(t, err, "有效证书应该验证通过")
}

// TestValidateConsortiumCertChain_InvalidCA 测试错误 CA：其他 CA 签发的证书验证失败
func TestValidateConsortiumCertChain_InvalidCA(t *testing.T) {
	// 1. 创建两个不同的 CA
	caCert1, _ := createTestCA(t)
	caCert2, caKey2 := createTestCA(t)

	// 2. 创建 CA Cert Pool（只信任 CA1）
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert1)

	// 3. 创建由 CA2 签发的证书（不在信任列表中）
	invalidCert, _ := createTestCert(t, caCert2, caKey2, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建验证策略
	policy := NewCertificateValidationPolicy(caPool, true, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{invalidCert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	assert.Error(t, err, "其他 CA 签发的证书应该验证失败")
	assert.True(t, errors.Is(err, ErrCertChainInvalid), "应该返回 ErrCertChainInvalid 错误")
}

// TestValidateConsortiumCertChain_Expired 测试过期证书：NotAfter 已过期的证书被拒绝
func TestValidateConsortiumCertChain_Expired(t *testing.T) {
	// 1. 创建 CA
	caCert, caKey := createTestCA(t)

	// 2. 创建 CA Cert Pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// 3. 创建已过期的证书
	expiredCert, _ := createTestCert(t, caCert, caKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-48*time.Hour), time.Now().Add(-1*time.Hour))

	// 4. 创建验证策略
	policy := NewCertificateValidationPolicy(caPool, true, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{expiredCert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	assert.Error(t, err, "过期证书应该验证失败")
	assert.True(t, errors.Is(err, ErrCertExpired), "应该返回 ErrCertExpired 错误")
}

// TestValidateConsortiumCertChain_NotYetValid 测试未生效证书：NotBefore 未到的证书被拒绝
func TestValidateConsortiumCertChain_NotYetValid(t *testing.T) {
	// 1. 创建 CA
	caCert, caKey := createTestCA(t)

	// 2. 创建 CA Cert Pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// 3. 创建尚未生效的证书
	futureCert, _ := createTestCert(t, caCert, caKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour))

	// 4. 创建验证策略
	policy := NewCertificateValidationPolicy(caPool, true, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{futureCert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	assert.Error(t, err, "未生效证书应该验证失败")
	assert.True(t, errors.Is(err, ErrCertNotYetValid), "应该返回 ErrCertNotYetValid 错误")
}

// TestValidateConsortiumCertChain_SubjectWhitelist 测试 Subject 白名单：CA 正确但 Subject 不在列表 → 拒绝
func TestValidateConsortiumCertChain_SubjectWhitelist(t *testing.T) {
	// 1. 创建 CA
	caCert, caKey := createTestCA(t)

	// 2. 创建 CA Cert Pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// 3. 创建证书（CN = "node1.example.com"）
	cert, _ := createTestCert(t, caCert, caKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建验证策略（只允许 "node2.example.com"）
	policy := NewCertificateValidationPolicy(caPool, true, []string{"node2.example.com"}, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{cert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	assert.Error(t, err, "Subject 不在白名单的证书应该验证失败")
	assert.True(t, errors.Is(err, ErrSubjectNotAllowed), "应该返回 ErrSubjectNotAllowed 错误")
}

// TestValidateConsortiumCertChain_OrgWhitelist 测试组织白名单：CA 正确但 Organization 不在列表 → 拒绝
func TestValidateConsortiumCertChain_OrgWhitelist(t *testing.T) {
	// 1. 创建 CA
	caCert, caKey := createTestCA(t)

	// 2. 创建 CA Cert Pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// 3. 创建证书（Org = "Test Org"）
	cert, _ := createTestCert(t, caCert, caKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建验证策略（只允许 "Allowed Org"，不允许 "Test Org"）
	policy := NewCertificateValidationPolicy(caPool, true, nil, []string{"Allowed Org"})
	policy.Now = func() time.Time { return time.Now() }

	// 5. 验证证书链
	certChain := []*x509.Certificate{cert}
	err := validateConsortiumCertChain(certChain, policy)

	// 6. 验证结果
	// 根据 validateConsortiumCertChain 的逻辑，如果配置了 AllowedSubjects 或 AllowedOrgs，
	// 会调用 validateCertSubject 进行检查
	// 如果 AllowedSubjects 为空但 AllowedOrgs 不为空，应该检查组织
	assert.Error(t, err, "组织不在白名单的证书应该验证失败")
	assert.True(t, errors.Is(err, ErrOrgNotAllowed), "应该返回 ErrOrgNotAllowed 错误，实际错误: %v", err)
}

// TestValidateConsortiumCertChain_IntermediateNotAllowed 测试中间 CA：不允许中间 CA 时验证失败
func TestValidateConsortiumCertChain_IntermediateNotAllowed(t *testing.T) {
	// 1. 创建根 CA
	rootCA, rootKey := createTestCA(t)

	// 2. 创建中间 CA（由根 CA 签发）
	intermediateCA, intermediateKey := createTestCert(t, rootCA, rootKey, "Intermediate CA", []string{"Test Consortium"}, time.Now().Add(-1*time.Hour), time.Now().Add(365*24*time.Hour))
	intermediateCA.IsCA = true
	intermediateCA.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign

	// 3. 创建由中间 CA 签发的证书
	leafCert, _ := createTestCert(t, intermediateCA, intermediateKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建 CA Cert Pool（只信任根 CA）
	caPool := x509.NewCertPool()
	caPool.AddCert(rootCA)

	// 5. 创建验证策略（不允许中间 CA）
	policy := NewCertificateValidationPolicy(caPool, false, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 6. 验证证书链（包含中间 CA）
	certChain := []*x509.Certificate{leafCert, intermediateCA}
	err := validateConsortiumCertChain(certChain, policy)

	// 7. 验证结果
	assert.Error(t, err, "不允许中间 CA 时，包含中间 CA 的证书链应该验证失败")
	assert.True(t, errors.Is(err, ErrCertChainInvalid), "应该返回 ErrCertChainInvalid 错误")
}

// TestValidateConsortiumCertChain_IntermediateAllowed 测试中间 CA：允许中间 CA 时验证通过
func TestValidateConsortiumCertChain_IntermediateAllowed(t *testing.T) {
	// 1. 创建根 CA
	rootCA, rootKey := createTestCA(t)

	// 2. 创建中间 CA 模板（由根 CA 签发）
	intermediateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "Intermediate CA",
			Organization: []string{"Test Consortium"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	intermediateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, rootCA, &intermediateKey.PublicKey, rootKey)
	require.NoError(t, err)

	intermediateCA, err := x509.ParseCertificate(intermediateDER)
	require.NoError(t, err)

	// 3. 创建由中间 CA 签发的证书
	leafCert, _ := createTestCert(t, intermediateCA, intermediateKey, "node1.example.com", []string{"Test Org"}, time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

	// 4. 创建 CA Cert Pool（只信任根 CA）
	caPool := x509.NewCertPool()
	caPool.AddCert(rootCA)

	// 5. 创建验证策略（允许中间 CA）
	policy := NewCertificateValidationPolicy(caPool, true, nil, nil)
	policy.Now = func() time.Time { return time.Now() }

	// 6. 验证证书链（包含中间 CA）
	certChain := []*x509.Certificate{leafCert, intermediateCA}
	err = validateConsortiumCertChain(certChain, policy)

	// 7. 验证结果
	assert.NoError(t, err, "允许中间 CA 时，包含中间 CA 的证书链应该验证通过")
}

// ============================================================================
//                           单元测试：证书链提取
// ============================================================================
// 注意：extractPeerCertChain 使用反射从 libp2p 连接中提取 TLS ConnectionState
// 由于 libp2p 连接结构的复杂性，完整的集成测试需要在真实的 libp2p 环境中进行
// 这里我们主要测试证书验证逻辑本身

// ============================================================================
//                           单元测试：完整验证流程
// ============================================================================
// 注意：ValidatePeerCertificate 需要真实的 libp2p 连接，完整的集成测试
// 需要在真实的 libp2p 环境中进行。这里我们主要测试证书验证逻辑本身。

