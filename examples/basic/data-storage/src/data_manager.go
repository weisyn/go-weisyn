package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

/*
🎯 数据管理模块

这个模块展示如何在应用中管理数据：
1. 数据预处理和格式化
2. 数据加密和解密
3. 数据压缩和解压
4. 数据完整性验证

💡 实际应用考虑：
- 支持多种加密算法
- 实现密钥管理系统
- 提供数据格式转换
- 优化存储效率
*/

// DataManager 数据管理器
type DataManager struct {
	encryptionKey []byte // 加密密钥（实际应用中应安全管理）
}

// DataFormat 数据格式枚举
type DataFormat string

const (
	FormatText     DataFormat = "text"
	FormatJSON     DataFormat = "json"
	FormatBinary   DataFormat = "binary"
	FormatImage    DataFormat = "image"
	FormatDocument DataFormat = "document"
)

// ProcessingOptions 处理选项
type ProcessingOptions struct {
	Encrypt   bool   `json:"encrypt"`    // 是否加密
	Compress  bool   `json:"compress"`   // 是否压缩
	Format    string `json:"format"`     // 数据格式
	ChunkSize int    `json:"chunk_size"` // 分片大小
}

// NewDataManager 创建新的数据管理器
func NewDataManager() *DataManager {
	// 生成默认加密密钥（实际应用中应从安全存储中获取）
	key := make([]byte, 32) // AES-256
	if _, err := rand.Read(key); err != nil {
		// 使用固定密钥作为演示（生产环境禁止）
		copy(key, []byte("demo_key_32_bytes_for_aes_256!!"))
	}

	return &DataManager{
		encryptionKey: key,
	}
}

// ProcessContent 处理内容（加密/压缩）
// 🎯 功能：根据需求对数据进行预处理
func (dm *DataManager) ProcessContent(content string, encrypt bool) (string, error) {
	var processedContent string = content

	// 💡 生活化理解：
	// 数据处理就像准备邮寄包裹
	// - 压缩 = 把东西压紧节省空间
	// - 加密 = 给包裹上锁保护隐私
	// - 分片 = 把大包裹分成小包分别寄送

	// 📋 步骤1：数据验证
	if content == "" {
		return "", fmt.Errorf("内容不能为空")
	}

	// 📋 步骤2：数据清理和格式化
	processedContent = dm.sanitizeContent(content)

	// 📋 步骤3：数据压缩（如果内容较大）
	if len(processedContent) > 1024 { // 大于1KB时压缩
		compressed, err := dm.compressContent(processedContent)
		if err != nil {
			return "", fmt.Errorf("压缩失败: %v", err)
		}
		processedContent = compressed
	}

	// 📋 步骤4：数据加密（如果需要）
	if encrypt {
		encrypted, err := dm.encryptContent(processedContent)
		if err != nil {
			return "", fmt.Errorf("加密失败: %v", err)
		}
		processedContent = encrypted
	}

	return processedContent, nil
}

// DecryptContent 解密内容
// 🎯 功能：解密存储的加密内容
func (dm *DataManager) DecryptContent(encryptedContent string, requester string) (string, error) {
	// 📋 步骤1：权限检查（简化版）
	if requester == "" {
		return "", fmt.Errorf("请求者不能为空")
	}

	// 📋 步骤2：解密内容
	decrypted, err := dm.decryptContent(encryptedContent)
	if err != nil {
		return "", fmt.Errorf("解密失败: %v", err)
	}

	// 📋 步骤3：解压缩（如果需要）
	if dm.isCompressed(decrypted) {
		decompressed, err := dm.decompressContent(decrypted)
		if err != nil {
			return "", fmt.Errorf("解压缩失败: %v", err)
		}
		decrypted = decompressed
	}

	return decrypted, nil
}

// ValidateIntegrity 验证数据完整性
// 🎯 功能：通过哈希值验证数据是否被篡改
func (dm *DataManager) ValidateIntegrity(content string, expectedHash string) (bool, error) {
	// 计算当前内容的哈希
	currentHash := dm.calculateHash(content)

	// 比较哈希值
	if currentHash == expectedHash {
		return true, nil
	}

	return false, fmt.Errorf("数据完整性验证失败：哈希不匹配")
}

// ChunkData 数据分片
// 🎯 功能：将大数据分成小片，便于存储和传输
func (dm *DataManager) ChunkData(content string, chunkSize int) ([]string, error) {
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // 默认1MB
	}

	var chunks []string
	contentBytes := []byte(content)

	for i := 0; i < len(contentBytes); i += chunkSize {
		end := i + chunkSize
		if end > len(contentBytes) {
			end = len(contentBytes)
		}

		chunk := string(contentBytes[i:end])
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ReassembleChunks 重组数据片
// 🎯 功能：将分片数据重新组合成原始数据
func (dm *DataManager) ReassembleChunks(chunks []string) (string, error) {
	if len(chunks) == 0 {
		return "", fmt.Errorf("分片列表为空")
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		builder.WriteString(chunk)
	}

	return builder.String(), nil
}

// FormatData 格式化数据
// 🎯 功能：将数据转换为指定格式
func (dm *DataManager) FormatData(content string, format DataFormat) (string, error) {
	switch format {
	case FormatText:
		return dm.formatAsText(content), nil
	case FormatJSON:
		return dm.formatAsJSON(content)
	case FormatBinary:
		return dm.formatAsBinary(content), nil
	default:
		return content, nil // 保持原格式
	}
}

// AnalyzeContent 分析内容特征
// 🎯 功能：分析数据的特征和统计信息
func (dm *DataManager) AnalyzeContent(content string) map[string]interface{} {
	analysis := make(map[string]interface{})

	// 基本统计
	analysis["size_bytes"] = len(content)
	analysis["size_chars"] = len([]rune(content))
	analysis["lines"] = strings.Count(content, "\n") + 1

	// 内容类型推测
	analysis["detected_type"] = dm.detectContentType(content)

	// 复杂度分析
	analysis["entropy"] = dm.calculateEntropy(content)
	analysis["compressibility"] = dm.estimateCompression(content)

	// 哈希指纹
	analysis["hash"] = dm.calculateHash(content)

	return analysis
}

// 私有方法：内容清理
func (dm *DataManager) sanitizeContent(content string) string {
	// 移除潜在的恶意字符
	// 在实际应用中应该更加严格
	content = strings.ReplaceAll(content, "\x00", "") // 移除null字符
	content = strings.TrimSpace(content)
	return content
}

// 私有方法：压缩内容
func (dm *DataManager) compressContent(content string) (string, error) {
	// 简化的压缩实现
	// 实际应用中可以使用更高效的压缩算法

	// 这里使用base64编码模拟压缩
	compressed := base64.StdEncoding.EncodeToString([]byte(content))

	// 添加压缩标记
	return "COMPRESSED:" + compressed, nil
}

// 私有方法：解压缩内容
func (dm *DataManager) decompressContent(compressedContent string) (string, error) {
	if !strings.HasPrefix(compressedContent, "COMPRESSED:") {
		return compressedContent, nil
	}

	// 移除压缩标记
	compressed := strings.TrimPrefix(compressedContent, "COMPRESSED:")

	// 解码
	decoded, err := base64.StdEncoding.DecodeString(compressed)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

// 私有方法：加密内容
func (dm *DataManager) encryptContent(content string) (string, error) {
	// 使用AES-GCM加密
	block, err := aes.NewCipher(dm.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密内容
	ciphertext := gcm.Seal(nonce, nonce, []byte(content), nil)

	// 编码为base64
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	// 添加加密标记
	return "ENCRYPTED:" + encoded, nil
}

// 私有方法：解密内容
func (dm *DataManager) decryptContent(encryptedContent string) (string, error) {
	if !strings.HasPrefix(encryptedContent, "ENCRYPTED:") {
		return encryptedContent, nil
	}

	// 移除加密标记
	encrypted := strings.TrimPrefix(encryptedContent, "ENCRYPTED:")

	// 解码base64
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	// 创建加密器
	block, err := aes.NewCipher(dm.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 提取nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("密文太短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// 私有方法：检查是否压缩
func (dm *DataManager) isCompressed(content string) bool {
	return strings.HasPrefix(content, "COMPRESSED:")
}

// 私有方法：计算哈希
func (dm *DataManager) calculateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// 私有方法：格式化为文本
func (dm *DataManager) formatAsText(content string) string {
	// 确保是纯文本格式
	return strings.TrimSpace(content)
}

// 私有方法：格式化为JSON
func (dm *DataManager) formatAsJSON(content string) (string, error) {
	// 尝试验证JSON格式
	// 这里简化处理，实际应该使用json.Valid()
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") && !strings.HasPrefix(content, "[") {
		// 包装为JSON对象
		return fmt.Sprintf(`{"content": %q}`, content), nil
	}
	return content, nil
}

// 私有方法：格式化为二进制
func (dm *DataManager) formatAsBinary(content string) string {
	// 转换为base64编码的二进制
	return base64.StdEncoding.EncodeToString([]byte(content))
}

// 私有方法：检测内容类型
func (dm *DataManager) detectContentType(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return "json"
	}

	if strings.Contains(content, "<html") || strings.Contains(content, "<!DOCTYPE") {
		return "html"
	}

	if strings.HasPrefix(content, "data:image") {
		return "image"
	}

	return "text"
}

// 私有方法：计算熵值
func (dm *DataManager) calculateEntropy(content string) float64 {
	// 简化的熵值计算
	charCount := make(map[rune]int)
	total := 0

	for _, char := range content {
		charCount[char]++
		total++
	}

	if total == 0 {
		return 0
	}

	entropy := 0.0
	for _, count := range charCount {
		probability := float64(count) / float64(total)
		if probability > 0 {
			entropy -= probability * (float64(count) / float64(total))
		}
	}

	return entropy
}

// 私有方法：估算压缩率
func (dm *DataManager) estimateCompression(content string) float64 {
	// 简化的压缩率估算
	original := len(content)
	if original == 0 {
		return 0
	}

	// 模拟压缩效果（计算重复字符）
	unique := make(map[rune]bool)
	for _, char := range content {
		unique[char] = true
	}

	compressionRatio := float64(len(unique)) / float64(original)
	return 1.0 - compressionRatio // 返回压缩节省的比例
}

// 演示函数：展示数据管理功能
func DemoDataManagement() {
	fmt.Println("🎮 数据管理演示")
	fmt.Println("===============")

	// 创建数据管理器
	dm := NewDataManager()

	// 1. 数据处理演示
	fmt.Println("1. 数据处理演示...")
	originalContent := "这是一个测试文档，包含重要信息。这是一个测试文档，包含重要信息。"

	processedContent, err := dm.ProcessContent(originalContent, true)
	if err != nil {
		fmt.Printf("处理失败: %v\n", err)
		return
	}
	fmt.Printf("原始内容: %s\n", originalContent[:30]+"...")
	fmt.Printf("处理后内容: %s\n", processedContent[:30]+"...")

	// 2. 解密演示
	fmt.Println("\n2. 解密演示...")
	decryptedContent, err := dm.DecryptContent(processedContent, "test_user")
	if err != nil {
		fmt.Printf("解密失败: %v\n", err)
		return
	}
	fmt.Printf("解密后内容: %s\n", decryptedContent[:30]+"...")

	// 3. 数据分析演示
	fmt.Println("\n3. 数据分析演示...")
	analysis := dm.AnalyzeContent(originalContent)
	fmt.Printf("分析结果: %+v\n", analysis)

	// 4. 数据分片演示
	fmt.Println("\n4. 数据分片演示...")
	chunks, err := dm.ChunkData(originalContent, 20)
	if err != nil {
		fmt.Printf("分片失败: %v\n", err)
		return
	}
	fmt.Printf("分片数量: %d\n", len(chunks))
	for i, chunk := range chunks {
		fmt.Printf("分片%d: %s\n", i+1, chunk)
	}

	// 5. 重组演示
	fmt.Println("\n5. 重组演示...")
	reassembled, err := dm.ReassembleChunks(chunks)
	if err != nil {
		fmt.Printf("重组失败: %v\n", err)
		return
	}
	fmt.Printf("重组后: %s\n", reassembled)
	fmt.Printf("重组正确性: %t\n", reassembled == originalContent)

	fmt.Println("✅ 数据管理演示完成")
}
