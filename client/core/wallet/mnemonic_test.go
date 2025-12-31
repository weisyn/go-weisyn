package wallet

import (
	"strings"
	"testing"
)

func TestMnemonicManager_GenerateMnemonic(t *testing.T) {
	mm := NewMnemonicManager()

	tests := []struct {
		name      string
		strength  MnemonicStrength
		wantWords int
		wantErr   bool
	}{
		{"12 words", Mnemonic12Words, 12, false},
		{"15 words", Mnemonic15Words, 15, false},
		{"18 words", Mnemonic18Words, 18, false},
		{"21 words", Mnemonic21Words, 21, false},
		{"24 words", Mnemonic24Words, 24, false},
		{"invalid strength", MnemonicStrength(100), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mnemonic, err := mm.GenerateMnemonic(tt.strength)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateMnemonic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				words := strings.Split(mnemonic, " ")
				if len(words) != tt.wantWords {
					t.Errorf("GenerateMnemonic() got %d words, want %d", len(words), tt.wantWords)
				}
				// 验证生成的助记词是有效的
				if !mm.ValidateMnemonic(mnemonic) {
					t.Error("GenerateMnemonic() generated invalid mnemonic")
				}
			}
		})
	}
}

func TestMnemonicManager_ValidateMnemonic(t *testing.T) {
	mm := NewMnemonicManager()

	// 生成一个有效的助记词用于测试
	validMnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)

	tests := []struct {
		name     string
		mnemonic string
		want     bool
	}{
		{"valid 12 words", validMnemonic, true},
		{"empty mnemonic", "", false},
		{"invalid word count", "abandon abandon abandon", false},
		{"invalid word", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon invalidword", false},
		{"wrong checksum", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mm.ValidateMnemonic(tt.mnemonic); got != tt.want {
				t.Errorf("ValidateMnemonic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMnemonicManager_ValidateMnemonicWithDetails(t *testing.T) {
	mm := NewMnemonicManager()

	validMnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)

	tests := []struct {
		name        string
		mnemonic    string
		wantValid   bool
		wantMsgSub  string
	}{
		{"valid mnemonic", validMnemonic, true, "有效"},
		{"empty mnemonic", "", false, "不能为空"},
		{"wrong word count", "abandon abandon abandon", false, "数量无效"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValid, gotMsg := mm.ValidateMnemonicWithDetails(tt.mnemonic)
			if gotValid != tt.wantValid {
				t.Errorf("ValidateMnemonicWithDetails() valid = %v, want %v", gotValid, tt.wantValid)
			}
			if !strings.Contains(gotMsg, tt.wantMsgSub) {
				t.Errorf("ValidateMnemonicWithDetails() msg = %v, want containing %v", gotMsg, tt.wantMsgSub)
			}
		})
	}
}

func TestMnemonicManager_MnemonicToSeed(t *testing.T) {
	mm := NewMnemonicManager()
	
	// 使用已知的测试向量
	// "abandon" x 11 + "about" 是 BIP39 测试向量
	testMnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	
	seed, err := mm.MnemonicToSeed(testMnemonic, "")
	if err != nil {
		t.Fatalf("MnemonicToSeed() error = %v", err)
	}
	
	// 种子应该是 64 字节
	if len(seed) != 64 {
		t.Errorf("MnemonicToSeed() seed length = %d, want 64", len(seed))
	}

	// 带密码的种子应该不同
	seedWithPass, err := mm.MnemonicToSeed(testMnemonic, "TREZOR")
	if err != nil {
		t.Fatalf("MnemonicToSeed() with passphrase error = %v", err)
	}

	if string(seed) == string(seedWithPass) {
		t.Error("MnemonicToSeed() seeds should be different with different passphrases")
	}
}

func TestMnemonicManager_GetWordCount(t *testing.T) {
	mm := NewMnemonicManager()

	tests := []struct {
		name     string
		mnemonic string
		want     int
	}{
		{"12 words", "a b c d e f g h i j k l", 12},
		{"24 words", "a b c d e f g h i j k l m n o p q r s t u v w x", 24},
		{"empty", "", 0},
		{"extra spaces", "  a   b   c  ", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mm.GetWordCount(tt.mnemonic); got != tt.want {
				t.Errorf("GetWordCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMnemonicManager_SuggestWords(t *testing.T) {
	mm := NewMnemonicManager()

	suggestions := mm.SuggestWords("aba")
	if len(suggestions) == 0 {
		t.Error("SuggestWords() returned no suggestions for 'aba'")
	}

	// 所有建议应该以 "aba" 开头
	for _, s := range suggestions {
		if !strings.HasPrefix(s, "aba") {
			t.Errorf("SuggestWords() suggestion %s does not start with 'aba'", s)
		}
	}
}

func TestMnemonicManager_WordToIndex(t *testing.T) {
	mm := NewMnemonicManager()

	// "abandon" 是 BIP39 词表的第一个词（索引 0）
	idx, err := mm.WordToIndex("abandon")
	if err != nil {
		t.Fatalf("WordToIndex('abandon') error = %v", err)
	}
	if idx != 0 {
		t.Errorf("WordToIndex('abandon') = %d, want 0", idx)
	}

	// 无效的词
	_, err = mm.WordToIndex("invalidword")
	if err == nil {
		t.Error("WordToIndex('invalidword') should return error")
	}
}

func TestMnemonicManager_IndexToWord(t *testing.T) {
	mm := NewMnemonicManager()

	word, err := mm.IndexToWord(0)
	if err != nil {
		t.Fatalf("IndexToWord(0) error = %v", err)
	}
	if word != "abandon" {
		t.Errorf("IndexToWord(0) = %s, want 'abandon'", word)
	}

	// 无效的索引
	_, err = mm.IndexToWord(3000)
	if err == nil {
		t.Error("IndexToWord(3000) should return error")
	}
}

func TestMnemonicManager_GetMnemonicInfo(t *testing.T) {
	mm := NewMnemonicManager()

	// 生成一个有效的助记词
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)

	info, err := mm.GetMnemonicInfo(mnemonic)
	if err != nil {
		t.Fatalf("GetMnemonicInfo() error = %v", err)
	}

	if info.WordCount != 12 {
		t.Errorf("GetMnemonicInfo() WordCount = %d, want 12", info.WordCount)
	}

	if info.Strength != Mnemonic12Words {
		t.Errorf("GetMnemonicInfo() Strength = %d, want %d", info.Strength, Mnemonic12Words)
	}

	if !info.IsValid {
		t.Error("GetMnemonicInfo() IsValid = false, want true")
	}

	if len(info.Entropy) == 0 {
		t.Error("GetMnemonicInfo() Entropy is empty")
	}
}

