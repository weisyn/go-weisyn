package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/wallet"
	addresspkg "github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	"golang.org/x/term"
)

var (
	accountPassword   string
	accountLabel      string
	accountExport     bool
	accountMnemonic   bool
	accountWIF        bool
	accountWords      int
	accountPassphrase string
	accountPath       string
	accountCompressed bool
)

// accountCmd 账户相关命令
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "账户管理",
	Long:  "创建、导入、导出和查询账户",
}

// accountNewCmd 创建新账户
var accountNewCmd = &cobra.Command{
	Use:   "new",
	Short: "创建新账户",
	Long: `创建新的账户并保存到keystore。

支持两种模式：
1. 随机私钥模式（默认）：生成随机私钥
2. 助记词模式（--mnemonic）：生成BIP39助记词钱包

示例：
  wes account new                      # 随机私钥
  wes account new --mnemonic           # 12词助记词
  wes account new --mnemonic --words 24  # 24词助记词`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 提示输入密码
		if accountPassword == "" {
			accountPassword, err = promptPassword("请输入密码")
			if err != nil {
				return err
			}

			// 确认密码
			confirmPassword, err := promptPassword("请确认密码")
			if err != nil {
				return err
			}

			if accountPassword != confirmPassword {
				return fmt.Errorf("密码不匹配")
			}
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		var account *wallet.AccountInfo
		var mnemonic string

		if accountMnemonic {
			// 助记词模式
			var strength wallet.MnemonicStrength
			switch accountWords {
			case 12:
				strength = wallet.Mnemonic12Words
			case 15:
				strength = wallet.Mnemonic15Words
			case 18:
				strength = wallet.Mnemonic18Words
			case 21:
				strength = wallet.Mnemonic21Words
			case 24:
				strength = wallet.Mnemonic24Words
			default:
				return fmt.Errorf("无效的助记词数量: %d，支持 12, 15, 18, 21, 24", accountWords)
			}

			// 生成助记词
			mnemonic, err = am.GenerateNewMnemonic(strength)
			if err != nil {
				return fmt.Errorf("生成助记词失败: %w", err)
			}

			// 从助记词创建账户
			account, err = am.CreateAccountFromMnemonic(mnemonic, accountPassphrase, accountPassword, accountLabel)
			if err != nil {
				return fmt.Errorf("从助记词创建账户失败: %w", err)
			}

			formatter.PrintSuccess(fmt.Sprintf("助记词账户创建成功: %s", account.Address))
			formatter.PrintWarning("⚠️  请务必安全备份以下助记词，丢失将无法恢复账户:")
			fmt.Println()
			fmt.Printf("  %s\n", mnemonic)
			fmt.Println()
			formatter.PrintInfo(fmt.Sprintf("派生路径: %s", wallet.WESDefaultPath()))
		} else {
			// 随机私钥模式
			account, err = am.CreateAccount(accountPassword, accountLabel)
			if err != nil {
				return fmt.Errorf("创建账户失败: %w", err)
			}

			formatter.PrintSuccess(fmt.Sprintf("账户创建成功: %s", account.Address))
		}

		return formatter.Print(map[string]interface{}{
			"address":       account.Address,
			"keystore_path": account.KeystorePath,
			"label":         account.Label,
			"created_at":    account.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	},
}

// accountListCmd 列出所有账户
var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有账户",
	Long:  "列出keystore中的所有账户",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 列出账户
		accounts, err := am.ListAccounts()
		if err != nil {
			return fmt.Errorf("列出账户失败: %w", err)
		}

		if len(accounts) == 0 {
			formatter.PrintInfo(fmt.Sprintf("Keystore: %s", profile.KeystorePath))
			formatter.PrintWarning("未找到账户，使用 'wes account new' 创建新账户")
			return nil
		}

		// 转换为输出格式
		accountList := make([]map[string]interface{}, 0, len(accounts))
		for _, account := range accounts {
			accountList = append(accountList, map[string]interface{}{
				"address":    account.Address,
				"label":      account.Label,
				"created_at": account.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		formatter.PrintInfo(fmt.Sprintf("Keystore: %s", profile.KeystorePath))
		formatter.PrintInfo(fmt.Sprintf("共 %d 个账户", len(accounts)))

		return formatter.Print(accountList)
	},
}

// accountShowCmd 显示账户详情
var accountShowCmd = &cobra.Command{
	Use:   "show <address>",
	Short: "显示账户详情",
	Long:  "显示指定账户的详细信息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := args[0]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 获取账户信息
		account, err := am.GetAccount(address)
		if err != nil {
			return fmt.Errorf("获取账户失败: %w", err)
		}

		return formatter.Print(map[string]interface{}{
			"address":       account.Address,
			"keystore_path": account.KeystorePath,
			"label":         account.Label,
			"created_at":    account.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	},
}

// accountImportCmd 导入私钥
var accountImportCmd = &cobra.Command{
	Use:   "import <private-key>",
	Short: "导入私钥",
	Long: `从私钥导入账户到keystore。

支持多种格式：
1. 十六进制私钥（默认）：64位十六进制字符串
2. WIF格式（--wif）：Base58Check编码的私钥
3. 助记词（--mnemonic）：BIP39助记词

示例：
  wes account import <hex-private-key>
  wes account import --wif <wif-string>
  wes account import --mnemonic "word1 word2 ... word12"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputKey := args[0]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 提示输入密码
		if accountPassword == "" {
			accountPassword, err = promptPassword("请输入加密密码")
			if err != nil {
				return err
			}

			// 确认密码
			confirmPassword, err := promptPassword("请确认密码")
			if err != nil {
				return err
			}

			if accountPassword != confirmPassword {
				return fmt.Errorf("密码不匹配")
			}
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		var account *wallet.AccountInfo

		switch {
		case accountMnemonic:
			// 助记词导入
			account, err = am.CreateAccountFromMnemonic(inputKey, accountPassphrase, accountPassword, accountLabel)
			if err != nil {
				return fmt.Errorf("从助记词导入失败: %w", err)
			}
			formatter.PrintSuccess(fmt.Sprintf("助记词账户导入成功: %s", account.Address))
			formatter.PrintInfo(fmt.Sprintf("派生路径: %s", wallet.WESDefaultPath()))

		case accountWIF:
			// WIF格式导入
			account, err = am.ImportWIF(inputKey, accountPassword, accountLabel)
			if err != nil {
				return fmt.Errorf("WIF导入失败: %w", err)
			}
			formatter.PrintSuccess(fmt.Sprintf("WIF私钥导入成功: %s", account.Address))

		default:
			// 十六进制私钥导入
			account, err = am.ImportPrivateKey(inputKey, accountPassword, accountLabel)
			if err != nil {
				return fmt.Errorf("导入私钥失败: %w", err)
			}
			formatter.PrintSuccess(fmt.Sprintf("私钥导入成功: %s", account.Address))
		}

		return formatter.Print(map[string]interface{}{
			"address":       account.Address,
			"keystore_path": account.KeystorePath,
			"label":         account.Label,
			"created_at":    account.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	},
}

// accountExportCmd 导出私钥
var accountExportCmd = &cobra.Command{
	Use:   "export <address>",
	Short: "导出私钥",
	Long: `导出账户的私钥（危险操作，请妥善保管）。

支持多种格式：
1. 十六进制（默认）：64位十六进制字符串
2. WIF格式（--wif）：Base58Check编码，可选压缩/非压缩

示例：
  wes account export <address>                    # 十六进制
  wes account export --wif <address>              # WIF（压缩）
  wes account export --wif --compressed=false <address>  # WIF（非压缩）`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := args[0]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 提示输入密码
		if accountPassword == "" {
			accountPassword, err = promptPassword("请输入keystore密码")
			if err != nil {
				return err
			}
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		formatter.PrintWarning("⚠️  危险：请妥善保管私钥，不要泄露给他人")

		if accountWIF {
			// 导出为 WIF 格式
			wifKey, err := am.ExportWIF(address, accountPassword, accountCompressed)
			if err != nil {
				return fmt.Errorf("导出WIF失败: %w", err)
			}

			compressionType := "压缩"
			if !accountCompressed {
				compressionType = "非压缩"
			}

			return formatter.Print(map[string]interface{}{
				"address":     address,
				"wif":         wifKey,
				"compression": compressionType,
				"format":      "WIF (WES)",
			})
		}

		// 导出十六进制私钥
		privateKey, err := am.ExportPrivateKey(address, accountPassword)
		if err != nil {
			return fmt.Errorf("导出私钥失败: %w", err)
		}

		return formatter.Print(map[string]interface{}{
			"address":     address,
			"private_key": privateKey,
			"format":      "hex",
		})
	},
}

// accountDeleteCmd 删除账户
var accountDeleteCmd = &cobra.Command{
	Use:   "delete <address>",
	Short: "删除账户",
	Long:  "从keystore中删除账户（危险操作，确保已备份）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := args[0]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 确认删除
		fmt.Printf("确认删除账户 %s? (yes/no): ", address)
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		if strings.ToLower(confirm) != "yes" {
			formatter.PrintInfo("取消删除")
			return nil
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 删除账户
		if err := am.DeleteAccount(address); err != nil {
			return fmt.Errorf("删除账户失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("账户已删除: %s", address))
		return nil
	},
}

// accountUpdateLabelCmd 更新账户标签
var accountUpdateLabelCmd = &cobra.Command{
	Use:   "label <address> <new-label>",
	Short: "更新账户标签",
	Long:  "更新账户的标签名称",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := args[0]
		newLabel := args[1]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 更新标签
		if err := am.UpdateLabel(address, newLabel); err != nil {
			return fmt.Errorf("更新标签失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("标签已更新: %s -> '%s'", address, newLabel))
		return nil
	},
}

// promptPassword 提示输入密码（不回显）
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt + ": ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("读取密码失败: %w", err)
	}
	fmt.Println()
	return string(bytePassword), nil
}

// accountDeriveCmd 派生新地址（HD钱包）
var accountDeriveCmd = &cobra.Command{
	Use:   "derive <mnemonic>",
	Short: "从助记词派生地址",
	Long: `从助记词派生指定路径的地址。

用于预览助记词对应的地址，不会创建账户。

示例：
  wes account derive "word1 word2 ... word12"
  wes account derive --path "m/44'/8888'/0'/0/1" "word1 word2 ..."`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mnemonic := args[0]

		// 创建账户管理器（使用临时目录，因为只是预览，不需要保存）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		tempDir, err := os.MkdirTemp("", "wes-derive-*")
		if err != nil {
			return fmt.Errorf("创建临时目录失败: %w", err)
		}
		defer os.RemoveAll(tempDir) // 清理临时目录
		
		am, err := wallet.NewAccountManager(tempDir, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 验证助记词
		valid, msg := am.ValidateMnemonic(mnemonic)
		if !valid {
			return fmt.Errorf("无效的助记词: %s", msg)
		}

		// 派生地址
		address, err := am.DeriveAddressFromMnemonic(mnemonic, accountPassphrase, accountPath)
		if err != nil {
			return fmt.Errorf("派生地址失败: %w", err)
		}

		path := accountPath
		if path == "" {
			path = wallet.WESDefaultPath()
		}

		return formatter.Print(map[string]interface{}{
			"address":         address,
			"derivation_path": path,
		})
	},
}

// accountValidateMnemonicCmd 验证助记词
var accountValidateMnemonicCmd = &cobra.Command{
	Use:   "validate-mnemonic <mnemonic>",
	Short: "验证助记词",
	Long:  "验证BIP39助记词是否有效",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mnemonic := args[0]

		mm := wallet.NewMnemonicManager()
		valid, msg := mm.ValidateMnemonicWithDetails(mnemonic)

		if valid {
			formatter.PrintSuccess("助记词有效")
			info, _ := mm.GetMnemonicInfo(mnemonic)
			return formatter.Print(map[string]interface{}{
				"valid":      true,
				"word_count": info.WordCount,
				"strength":   fmt.Sprintf("%d bits", info.Strength),
			})
		}

		formatter.PrintError(fmt.Errorf("助记词无效: %s", msg))
		return formatter.Print(map[string]interface{}{
			"valid":  false,
			"reason": msg,
		})
	},
}

func init() {
	accountCmd.AddCommand(accountNewCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountShowCmd)
	accountCmd.AddCommand(accountImportCmd)
	accountCmd.AddCommand(accountExportCmd)
	accountCmd.AddCommand(accountDeleteCmd)
	accountCmd.AddCommand(accountUpdateLabelCmd)
	accountCmd.AddCommand(accountDeriveCmd)
	accountCmd.AddCommand(accountValidateMnemonicCmd)

	// accountNewCmd 标志
	accountNewCmd.Flags().StringVarP(&accountPassword, "password", "p", "", "账户密码")
	accountNewCmd.Flags().StringVarP(&accountLabel, "label", "l", "", "账户标签")
	accountNewCmd.Flags().BoolVarP(&accountMnemonic, "mnemonic", "m", false, "使用助记词模式创建")
	accountNewCmd.Flags().IntVarP(&accountWords, "words", "w", 12, "助记词数量 (12, 15, 18, 21, 24)")
	accountNewCmd.Flags().StringVar(&accountPassphrase, "passphrase", "", "BIP39密码短语（可选，用于额外安全）")

	// accountImportCmd 标志
	accountImportCmd.Flags().StringVarP(&accountPassword, "password", "p", "", "加密密码")
	accountImportCmd.Flags().StringVarP(&accountLabel, "label", "l", "", "账户标签")
	accountImportCmd.Flags().BoolVarP(&accountMnemonic, "mnemonic", "m", false, "从助记词导入")
	accountImportCmd.Flags().BoolVar(&accountWIF, "wif", false, "从WIF格式导入")
	accountImportCmd.Flags().StringVar(&accountPassphrase, "passphrase", "", "BIP39密码短语")

	// accountExportCmd 标志
	accountExportCmd.Flags().StringVarP(&accountPassword, "password", "p", "", "keystore密码")
	accountExportCmd.Flags().BoolVar(&accountWIF, "wif", false, "导出为WIF格式")
	accountExportCmd.Flags().BoolVar(&accountCompressed, "compressed", true, "使用压缩公钥（WIF模式）")

	// accountDeriveCmd 标志
	accountDeriveCmd.Flags().StringVar(&accountPath, "path", "", "派生路径（默认：m/44'/8888'/0'/0/0）")
	accountDeriveCmd.Flags().StringVar(&accountPassphrase, "passphrase", "", "BIP39密码短语")
}
