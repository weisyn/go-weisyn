// Package wallet provides wallet functionality for WES blockchain.
package wallet

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

// MnemonicStrength 助记词强度
type MnemonicStrength int

const (
	// Mnemonic12Words 12个助记词 (128 bits 熵)
	Mnemonic12Words MnemonicStrength = 128
	// Mnemonic15Words 15个助记词 (160 bits 熵)
	Mnemonic15Words MnemonicStrength = 160
	// Mnemonic18Words 18个助记词 (192 bits 熵)
	Mnemonic18Words MnemonicStrength = 192
	// Mnemonic21Words 21个助记词 (224 bits 熵)
	Mnemonic21Words MnemonicStrength = 224
	// Mnemonic24Words 24个助记词 (256 bits 熵)
	Mnemonic24Words MnemonicStrength = 256
)

// MnemonicManager 助记词管理器
type MnemonicManager struct {
	// wordList 用于生成助记词的字典
	wordList []string
}

// NewMnemonicManager 创建新的助记词管理器
func NewMnemonicManager() *MnemonicManager {
	return &MnemonicManager{
		wordList: bip39.GetWordList(),
	}
}

// GenerateMnemonic 生成助记词
// strength: 熵的位数，支持 128(12词), 160(15词), 192(18词), 224(21词), 256(24词)
func (m *MnemonicManager) GenerateMnemonic(strength MnemonicStrength) (string, error) {
	// 验证强度
	switch strength {
	case Mnemonic12Words, Mnemonic15Words, Mnemonic18Words, Mnemonic21Words, Mnemonic24Words:
		// 有效强度
	default:
		return "", fmt.Errorf("invalid mnemonic strength: %d, must be 128, 160, 192, 224, or 256", strength)
	}

	// 生成熵
	entropyBytes := int(strength) / 8
	entropy := make([]byte, entropyBytes)
	if _, err := rand.Read(entropy); err != nil {
		return "", fmt.Errorf("failed to generate entropy: %w", err)
	}

	// 从熵生成助记词
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate mnemonic: %w", err)
	}

	return mnemonic, nil
}

// GenerateMnemonicFromEntropy 从指定熵生成助记词
func (m *MnemonicManager) GenerateMnemonicFromEntropy(entropy []byte) (string, error) {
	if len(entropy) < 16 || len(entropy) > 32 || len(entropy)%4 != 0 {
		return "", errors.New("entropy must be 16, 20, 24, 28, or 32 bytes")
	}

	return bip39.NewMnemonic(entropy)
}

// ValidateMnemonic 验证助记词是否有效
func (m *MnemonicManager) ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// ValidateMnemonicWithDetails 验证助记词并返回详细信息
func (m *MnemonicManager) ValidateMnemonicWithDetails(mnemonic string) (bool, string) {
	// 清理输入
	mnemonic = strings.TrimSpace(mnemonic)
	mnemonic = normalizeSpaces(mnemonic)

	// 检查是否为空
	if mnemonic == "" {
		return false, "助记词不能为空"
	}

	// 分割单词
	words := strings.Split(mnemonic, " ")

	// 检查单词数量
	wordCount := len(words)
	validCounts := []int{12, 15, 18, 21, 24}
	isValidCount := false
	for _, count := range validCounts {
		if wordCount == count {
			isValidCount = true
			break
		}
	}
	if !isValidCount {
		return false, fmt.Sprintf("助记词数量无效: %d，应为 12, 15, 18, 21 或 24", wordCount)
	}

	// 检查每个单词是否在词表中
	wordSet := make(map[string]bool)
	for _, word := range m.wordList {
		wordSet[word] = true
	}

	for i, word := range words {
		if !wordSet[word] {
			return false, fmt.Sprintf("第 %d 个单词 '%s' 不在 BIP39 词表中", i+1, word)
		}
	}

	// 使用 BIP39 库进行校验和验证
	if !bip39.IsMnemonicValid(mnemonic) {
		return false, "校验和验证失败，请检查助记词是否正确"
	}

	return true, "助记词有效"
}

// MnemonicToSeed 将助记词转换为种子
// passphrase 是可选的密码，用于增加安全性
func (m *MnemonicManager) MnemonicToSeed(mnemonic, passphrase string) ([]byte, error) {
	// 首先验证助记词
	if !m.ValidateMnemonic(mnemonic) {
		return nil, errors.New("invalid mnemonic")
	}

	// 生成种子 (使用 PBKDF2 with HMAC-SHA512)
	seed := bip39.NewSeed(mnemonic, passphrase)
	return seed, nil
}

// MnemonicToEntropy 将助记词转换回熵
func (m *MnemonicManager) MnemonicToEntropy(mnemonic string) ([]byte, error) {
	return bip39.MnemonicToByteArray(mnemonic, true)
}

// GetWordCount 获取助记词单词数量
func (m *MnemonicManager) GetWordCount(mnemonic string) int {
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		return 0
	}
	return len(strings.Split(normalizeSpaces(mnemonic), " "))
}

// GetWordList 获取 BIP39 词表
func (m *MnemonicManager) GetWordList() []string {
	return m.wordList
}

// WordToIndex 获取单词在词表中的索引
func (m *MnemonicManager) WordToIndex(word string) (int, error) {
	for i, w := range m.wordList {
		if w == word {
			return i, nil
		}
	}
	return -1, fmt.Errorf("word '%s' not found in BIP39 wordlist", word)
}

// IndexToWord 根据索引获取单词
func (m *MnemonicManager) IndexToWord(index int) (string, error) {
	if index < 0 || index >= len(m.wordList) {
		return "", fmt.Errorf("index %d out of range (0-%d)", index, len(m.wordList)-1)
	}
	return m.wordList[index], nil
}

// SuggestWords 根据前缀建议单词
func (m *MnemonicManager) SuggestWords(prefix string) []string {
	prefix = strings.ToLower(prefix)
	var suggestions []string
	for _, word := range m.wordList {
		if strings.HasPrefix(word, prefix) {
			suggestions = append(suggestions, word)
		}
	}
	return suggestions
}

// normalizeSpaces 规范化空格（将多个连续空格替换为单个空格）
func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// MnemonicInfo 助记词信息
type MnemonicInfo struct {
	Mnemonic   string           `json:"mnemonic"`    // 助记词
	WordCount  int              `json:"word_count"`  // 单词数量
	Strength   MnemonicStrength `json:"strength"`    // 熵强度（bits）
	IsValid    bool             `json:"is_valid"`    // 是否有效
	Entropy    []byte           `json:"entropy"`     // 原始熵（可选）
	Passphrase string           `json:"-"`           // 密码（不序列化）
}

// GetMnemonicInfo 获取助记词详细信息
func (m *MnemonicManager) GetMnemonicInfo(mnemonic string) (*MnemonicInfo, error) {
	wordCount := m.GetWordCount(mnemonic)
	info := &MnemonicInfo{
		Mnemonic:  mnemonic,
		WordCount: wordCount,
		IsValid:   m.ValidateMnemonic(mnemonic),
	}

	// 根据单词数量确定强度
	switch wordCount {
	case 12:
		info.Strength = Mnemonic12Words
	case 15:
		info.Strength = Mnemonic15Words
	case 18:
		info.Strength = Mnemonic18Words
	case 21:
		info.Strength = Mnemonic21Words
	case 24:
		info.Strength = Mnemonic24Words
	default:
		info.Strength = 0
	}

	// 如果有效，尝试恢复熵
	if info.IsValid {
		entropy, err := m.MnemonicToEntropy(mnemonic)
		if err == nil {
			info.Entropy = entropy
		}
	}

	return info, nil
}

