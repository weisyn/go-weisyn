package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SecurityUtils 安全工具类
type SecurityUtils struct{}

// NewSecurityUtils 创建安全工具实例
func NewSecurityUtils() *SecurityUtils {
	return &SecurityUtils{}
}

// GenerateSecureToken 生成安全令牌
func (su *SecurityUtils) GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成安全令牌失败: %v", err)
	}

	return hex.EncodeToString(bytes), nil
}

// ValidateAddress 验证区块链地址格式
func (su *SecurityUtils) ValidateAddress(address string) (bool, error) {
	// 简化的地址验证规则
	if len(address) < 20 || len(address) > 50 {
		return false, fmt.Errorf("地址长度不正确")
	}

	// 检查地址格式（以大写字母开头）
	if !regexp.MustCompile(`^[A-Z][a-zA-Z0-9]+$`).MatchString(address) {
		return false, fmt.Errorf("地址格式不正确")
	}

	return true, nil
}

// ValidatePrivateKey 验证私钥格式
func (su *SecurityUtils) ValidatePrivateKey(privateKey string) (bool, error) {
	// 检查私钥长度（64位十六进制）
	if len(privateKey) != 64 {
		return false, fmt.Errorf("私钥长度必须为64位")
	}

	// 检查是否为十六进制
	if !regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(privateKey) {
		return false, fmt.Errorf("私钥必须为十六进制格式")
	}

	return true, nil
}

// ValidateTransferAmount 验证转账金额
func (su *SecurityUtils) ValidateTransferAmount(amount string) (float64, error) {
	// 解析金额
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式不正确: %v", err)
	}

	// 检查金额范围
	if value <= 0 {
		return 0, fmt.Errorf("转账金额必须大于0")
	}

	if value > 1000000 { // 单次转账限额
		return 0, fmt.Errorf("单次转账金额不能超过1,000,000 WES")
	}

	// 检查小数位数（最多8位）
	parts := strings.Split(amount, ".")
	if len(parts) == 2 && len(parts[1]) > 8 {
		return 0, fmt.Errorf("金额小数位数不能超过8位")
	}

	return value, nil
}

// CheckPasswordStrength 检查密码强度
func (su *SecurityUtils) CheckPasswordStrength(password string) PasswordStrength {
	score := 0
	feedback := make([]string, 0)

	// 长度检查
	if len(password) >= 8 {
		score += 1
	} else {
		feedback = append(feedback, "密码长度至少需要8位")
	}

	if len(password) >= 12 {
		score += 1
	}

	// 字符类型检查
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)

	if hasLower {
		score += 1
	} else {
		feedback = append(feedback, "包含小写字母")
	}

	if hasUpper {
		score += 1
	} else {
		feedback = append(feedback, "包含大写字母")
	}

	if hasDigit {
		score += 1
	} else {
		feedback = append(feedback, "包含数字")
	}

	if hasSpecial {
		score += 1
	} else {
		feedback = append(feedback, "包含特殊字符")
	}

	// 常见密码检查
	if su.isCommonPassword(password) {
		score -= 2
		feedback = append(feedback, "避免使用常见密码")
	}

	// 重复字符检查
	if su.hasRepeatingChars(password) {
		score -= 1
		feedback = append(feedback, "避免重复字符")
	}

	// 确定强度级别
	var level PasswordStrengthLevel
	var description string

	switch {
	case score >= 5:
		level = VeryStrongPassword
		description = "非常强"
	case score >= 4:
		level = StrongPassword
		description = "强"
	case score >= 3:
		level = MediumPassword
		description = "中等"
	case score >= 2:
		level = WeakPassword
		description = "弱"
	default:
		level = VeryWeakPassword
		description = "非常弱"
	}

	return PasswordStrength{
		Level:       level,
		Score:       score,
		Description: description,
		Feedback:    feedback,
	}
}

// isCommonPassword 检查是否为常见密码
func (su *SecurityUtils) isCommonPassword(password string) bool {
	commonPasswords := []string{
		"password", "123456", "123456789", "qwerty", "abc123",
		"password123", "admin", "root", "user", "guest",
		"12345678", "1234567890", "qwertyuiop", "asdfghjkl",
	}

	lowerPassword := strings.ToLower(password)
	for _, common := range commonPasswords {
		if lowerPassword == common {
			return true
		}
	}

	return false
}

// hasRepeatingChars 检查是否有重复字符
func (su *SecurityUtils) hasRepeatingChars(password string) bool {
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i] == password[i+2] {
			return true
		}
	}
	return false
}

// GetClientIP 获取客户端IP地址
func (su *SecurityUtils) GetClientIP() string {
	// 简化实现，在CLI环境中返回本地IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}

// FormatSecurityLevel 格式化安全级别显示
func (su *SecurityUtils) FormatSecurityLevel(level SecurityLevel) string {
	icon := ""
	color := ""

	switch level {
	case LowSecurity:
		icon = "🟢"
		color = "低风险"
	case MediumSecurity:
		icon = "🟡"
		color = "中风险"
	case HighSecurity:
		icon = "🟠"
		color = "高风险"
	case CriticalSecurity:
		icon = "🔴"
		color = "极高风险"
	default:
		icon = "⚪"
		color = "未知"
	}

	return fmt.Sprintf("%s %s", icon, color)
}

// TimeBasedOneTimePassword 基于时间的一次性密码（简化版本）
func (su *SecurityUtils) TimeBasedOneTimePassword(secret string, timeStep int64) (string, error) {
	if timeStep <= 0 {
		timeStep = time.Now().Unix() / 30 // 30秒窗口
	}

	// 简化实现：基于时间戳和密钥生成6位数字码
	hash := fmt.Sprintf("%s%d", secret, timeStep)
	code := 0

	for _, char := range hash {
		code += int(char)
	}

	return fmt.Sprintf("%06d", code%1000000), nil
}

// ValidateOperationTiming 验证操作时间
func (su *SecurityUtils) ValidateOperationTiming(lastOperationTime time.Time, minInterval time.Duration) error {
	if time.Since(lastOperationTime) < minInterval {
		remaining := minInterval - time.Since(lastOperationTime)
		return fmt.Errorf("操作过于频繁，请等待 %v 后重试", remaining.Round(time.Second))
	}

	return nil
}

// SanitizeInput 清理用户输入
func (su *SecurityUtils) SanitizeInput(input string) string {
	// 移除潜在的危险字符
	dangerousChars := []string{
		"<", ">", "\"", "'", "&", "script", "javascript:",
		"data:", "vbscript:", "onload=", "onerror=",
	}

	sanitized := input
	for _, char := range dangerousChars {
		sanitized = strings.ReplaceAll(sanitized, char, "")
	}

	// 限制长度
	if len(sanitized) > 1000 {
		sanitized = sanitized[:1000]
	}

	return strings.TrimSpace(sanitized)
}

// PasswordStrengthLevel 密码强度级别
type PasswordStrengthLevel int

const (
	VeryWeakPassword PasswordStrengthLevel = iota
	WeakPassword
	MediumPassword
	StrongPassword
	VeryStrongPassword
)

// PasswordStrength 密码强度信息
type PasswordStrength struct {
	Level       PasswordStrengthLevel
	Score       int
	Description string
	Feedback    []string
}

// String 返回密码强度的字符串表示
func (ps PasswordStrength) String() string {
	return fmt.Sprintf("%s (分数: %d/6)", ps.Description, ps.Score)
}

// SecurityAudit 安全审计结果
type SecurityAudit struct {
	Timestamp       time.Time
	ChecksPassed    int
	ChecksFailed    int
	TotalChecks     int
	Issues          []SecurityIssue
	Recommendations []string
}

// SecurityIssue 安全问题
type SecurityIssue struct {
	Level       SecurityLevel
	Category    string
	Title       string
	Description string
	Solution    string
}

// PerformSecurityAudit 执行安全审计
func (su *SecurityUtils) PerformSecurityAudit(config map[string]interface{}) SecurityAudit {
	audit := SecurityAudit{
		Timestamp:       time.Now(),
		Issues:          make([]SecurityIssue, 0),
		Recommendations: make([]string, 0),
	}

	checks := []func(map[string]interface{}) *SecurityIssue{
		su.checkPasswordPolicy,
		su.checkNetworkSecurity,
		su.checkFilePermissions,
		su.checkEncryptionSettings,
		su.checkAuditLogging,
	}

	audit.TotalChecks = len(checks)

	for _, check := range checks {
		if issue := check(config); issue != nil {
			audit.Issues = append(audit.Issues, *issue)
			audit.ChecksFailed++
		} else {
			audit.ChecksPassed++
		}
	}

	// 生成建议
	audit.Recommendations = su.generateSecurityRecommendations(audit.Issues)

	return audit
}

// checkPasswordPolicy 检查密码策略
func (su *SecurityUtils) checkPasswordPolicy(config map[string]interface{}) *SecurityIssue {
	minLength, ok := config["min_password_length"].(int)
	if !ok || minLength < 8 {
		return &SecurityIssue{
			Level:       MediumSecurity,
			Category:    "密码策略",
			Title:       "密码长度要求不足",
			Description: "最小密码长度应至少为8位",
			Solution:    "将最小密码长度设置为8位或更多",
		}
	}

	return nil
}

// checkNetworkSecurity 检查网络安全
func (su *SecurityUtils) checkNetworkSecurity(config map[string]interface{}) *SecurityIssue {
	httpsOnly, ok := config["https_only"].(bool)
	if !ok || !httpsOnly {
		return &SecurityIssue{
			Level:       HighSecurity,
			Category:    "网络安全",
			Title:       "未启用HTTPS加密",
			Description: "网络通信未使用HTTPS加密，存在数据泄露风险",
			Solution:    "启用HTTPS加密传输",
		}
	}

	return nil
}

// checkFilePermissions 检查文件权限
func (su *SecurityUtils) checkFilePermissions(config map[string]interface{}) *SecurityIssue {
	// 简化实现
	return nil
}

// checkEncryptionSettings 检查加密设置
func (su *SecurityUtils) checkEncryptionSettings(config map[string]interface{}) *SecurityIssue {
	encryptionEnabled, ok := config["wallet_encryption"].(bool)
	if !ok || !encryptionEnabled {
		return &SecurityIssue{
			Level:       CriticalSecurity,
			Category:    "数据加密",
			Title:       "钱包加密未启用",
			Description: "钱包文件未加密存储，存在极高安全风险",
			Solution:    "启用钱包加密功能",
		}
	}

	return nil
}

// checkAuditLogging 检查审计日志
func (su *SecurityUtils) checkAuditLogging(config map[string]interface{}) *SecurityIssue {
	loggingEnabled, ok := config["audit_logging"].(bool)
	if !ok || !loggingEnabled {
		return &SecurityIssue{
			Level:       MediumSecurity,
			Category:    "审计日志",
			Title:       "审计日志未启用",
			Description: "未启用安全审计日志，无法追踪安全事件",
			Solution:    "启用审计日志记录功能",
		}
	}

	return nil
}

// generateSecurityRecommendations 生成安全建议
func (su *SecurityUtils) generateSecurityRecommendations(issues []SecurityIssue) []string {
	recommendations := make([]string, 0)

	hasHighRisk := false
	hasCriticalRisk := false

	for _, issue := range issues {
		if issue.Level >= HighSecurity {
			hasHighRisk = true
		}
		if issue.Level >= CriticalSecurity {
			hasCriticalRisk = true
		}
	}

	if hasCriticalRisk {
		recommendations = append(recommendations, "立即处理所有关键安全风险")
		recommendations = append(recommendations, "暂停执行高风险操作直到问题解决")
	}

	if hasHighRisk {
		recommendations = append(recommendations, "优先处理高风险安全问题")
	}

	if len(issues) > 3 {
		recommendations = append(recommendations, "建议进行全面的安全评估")
	}

	recommendations = append(recommendations, "定期执行安全审计检查")
	recommendations = append(recommendations, "保持系统和依赖项的更新")

	return recommendations
}
