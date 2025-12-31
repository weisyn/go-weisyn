package screens

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/weisyn/v1/client/pkg/transport/api"
	"github.com/weisyn/v1/client/pkg/ux/ui"
)

// BlockchainScreen 区块信息屏幕
type BlockchainScreen struct {
	ui         ui.Components
	blockchain *api.BlockchainAdapter
}

// NewBlockchainScreen 创建区块信息屏幕
func NewBlockchainScreen(components ui.Components, blockchain *api.BlockchainAdapter) *BlockchainScreen {
	return &BlockchainScreen{
		ui:         components,
		blockchain: blockchain,
	}
}

// Show 显示区块信息菜单
func (s *BlockchainScreen) Show(ctx context.Context) error {
	for {
		s.ui.Clear()
		s.ui.ShowHeader("📊 区块链查询")

		// 菜单选项
		options := []string{
			"查看最新区块",
			"按高度查询区块",
			"查询交易",
			"查看链状态",
			"返回上一级",
		}

		choice, err := s.ui.ShowMenu("请选择操作", options)
		if err != nil {
			return err
		}

		switch choice {
		case 0: // 查看最新区块
			if err := s.showLatestBlock(ctx); err != nil {
				s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
			}
			s.ui.ShowContinuePrompt("", "")
		case 1: // 按高度查询区块
			if err := s.showBlockByHeight(ctx); err != nil {
				s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
			}
			s.ui.ShowContinuePrompt("", "")
		case 2: // 查询交易
			if err := s.showTransaction(ctx); err != nil {
				s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
			}
			s.ui.ShowContinuePrompt("", "")
		case 3: // 查看链状态
			if err := s.showChainStatus(ctx); err != nil {
				s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
			}
			s.ui.ShowContinuePrompt("", "")
		case 4: // 返回
			return nil
		}
	}
}

// showLatestBlock 显示最新区块
func (s *BlockchainScreen) showLatestBlock(ctx context.Context) error {
	s.ui.ShowInfo("正在查询最新区块...")

	// 获取当前高度
	height, err := s.blockchain.GetBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("获取区块高度失败: %w", err)
	}

	// 获取区块详情
	block, err := s.blockchain.GetBlockByHeight(ctx, height, false)
	if err != nil {
		return fmt.Errorf("获取区块失败: %w", err)
	}

	// 显示区块信息
	s.ui.ShowSuccess("✅ 查询成功")
	s.ui.ShowInfo("")
	s.ui.ShowInfo(fmt.Sprintf("📦 区块高度: %d", block.Height))
	s.ui.ShowInfo(fmt.Sprintf("🔗 区块哈希: %s", block.Hash))
	s.ui.ShowInfo(fmt.Sprintf("⬆️  父区块: %s", block.ParentHash))
	s.ui.ShowInfo(fmt.Sprintf("⏰ 时间戳: %s", formatTimestamp(block.Timestamp)))
	s.ui.ShowInfo(fmt.Sprintf("🌳 Merkle根: %s", block.MerkleRoot))
	s.ui.ShowInfo(fmt.Sprintf("📝 交易数量: %d", block.TxCount))

	// 显示交易列表（最多显示10个）
	if len(block.Transactions) > 0 {
		s.ui.ShowInfo("")
		s.ui.ShowInfo("📋 区块交易列表:")
		maxShow := 10
		if len(block.Transactions) < maxShow {
			maxShow = len(block.Transactions)
		}
		for i := 0; i < maxShow; i++ {
			s.ui.ShowInfo(fmt.Sprintf("  %d. %s", i+1, block.Transactions[i]))
		}
		if len(block.Transactions) > maxShow {
			s.ui.ShowInfo(fmt.Sprintf("  ... 还有 %d 笔交易", len(block.Transactions)-maxShow))
		}
	}

	return nil
}

// showBlockByHeight 按高度查询区块
func (s *BlockchainScreen) showBlockByHeight(ctx context.Context) error {
	// 提示输入高度
	heightStr, err := s.ui.ShowInputDialog("查询区块", "请输入区块高度", false)
	if err != nil {
		return err
	}

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return fmt.Errorf("区块高度格式错误: %w", err)
	}

	s.ui.ShowInfo(fmt.Sprintf("正在查询区块 #%d...", height))

	// 获取区块详情
	block, err := s.blockchain.GetBlockByHeight(ctx, height, false)
	if err != nil {
		return fmt.Errorf("获取区块失败: %w", err)
	}

	// 显示区块信息
	s.ui.ShowSuccess("✅ 查询成功")
	s.ui.ShowInfo("")
	s.ui.ShowInfo(fmt.Sprintf("📦 区块高度: %d", block.Height))
	s.ui.ShowInfo(fmt.Sprintf("🔗 区块哈希: %s", block.Hash))
	s.ui.ShowInfo(fmt.Sprintf("⬆️  父区块: %s", block.ParentHash))
	s.ui.ShowInfo(fmt.Sprintf("⏰ 时间戳: %s", formatTimestamp(block.Timestamp)))
	s.ui.ShowInfo(fmt.Sprintf("🌳 Merkle根: %s", block.MerkleRoot))
	s.ui.ShowInfo(fmt.Sprintf("📝 交易数量: %d", block.TxCount))

	// 显示交易列表
	if len(block.Transactions) > 0 {
		s.ui.ShowInfo("")
		s.ui.ShowInfo("📋 区块交易列表:")
		maxShow := 10
		if len(block.Transactions) < maxShow {
			maxShow = len(block.Transactions)
		}
		for i := 0; i < maxShow; i++ {
			s.ui.ShowInfo(fmt.Sprintf("  %d. %s", i+1, block.Transactions[i]))
		}
		if len(block.Transactions) > maxShow {
			s.ui.ShowInfo(fmt.Sprintf("  ... 还有 %d 笔交易", len(block.Transactions)-maxShow))
		}
	}

	return nil
}

// showTransaction 查询交易详情
func (s *BlockchainScreen) showTransaction(ctx context.Context) error {
	// 提示输入交易哈希
	txHash, err := s.ui.ShowInputDialog("查询交易", "请输入交易哈希", false)
	if err != nil {
		return err
	}

	s.ui.ShowInfo("正在查询交易...")

	// 获取交易详情
	tx, err := s.blockchain.GetTransactionByHash(ctx, txHash)
	if err != nil {
		return fmt.Errorf("获取交易失败: %w", err)
	}

	// 显示交易信息
	s.ui.ShowSuccess("✅ 查询成功")
	s.ui.ShowInfo("")
	s.ui.ShowInfo(fmt.Sprintf("📝 交易哈希: %s", tx.Hash))
	s.ui.ShowInfo(fmt.Sprintf("📦 所在区块: #%d", tx.BlockHeight))
	s.ui.ShowInfo(fmt.Sprintf("🔗 区块哈希: %s", tx.BlockHash))
	s.ui.ShowInfo(fmt.Sprintf("📍 交易索引: %d", tx.Index))

	if tx.From != "" {
		s.ui.ShowInfo(fmt.Sprintf("👤 发送方: %s", tx.From))
	}
	if tx.To != "" {
		s.ui.ShowInfo(fmt.Sprintf("👥 接收方: %s", tx.To))
	}
	if tx.Value != "" {
		s.ui.ShowInfo(fmt.Sprintf("💰 金额: %s", tx.Value))
	}
	if tx.Fee != "" {
		s.ui.ShowInfo(fmt.Sprintf("⛽ 手续费: %s", tx.Fee))
	}
	s.ui.ShowInfo(fmt.Sprintf("✅ 状态: %s", tx.Status))

	// 查询交易收据
	s.ui.ShowInfo("")
	s.ui.ShowInfo("正在查询交易收据...")
	receipt, err := s.blockchain.GetTransactionReceipt(ctx, txHash)
	if err == nil && receipt != nil {
		s.ui.ShowInfo("📄 交易收据:")
		for k, v := range receipt {
			s.ui.ShowInfo(fmt.Sprintf("  %s: %v", k, v))
		}
	}

	return nil
}

// showChainStatus 显示链状态
func (s *BlockchainScreen) showChainStatus(ctx context.Context) error {
	s.ui.ShowInfo("正在查询链状态...")

	// 获取链信息
	chainInfo, err := s.blockchain.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	// 显示链信息
	s.ui.ShowSuccess("✅ 查询成功")
	s.ui.ShowInfo("")
	s.ui.ShowInfo(fmt.Sprintf("🔗 链ID: %d", chainInfo.ChainID))
	s.ui.ShowInfo(fmt.Sprintf("📊 当前高度: %d", chainInfo.Height))
	s.ui.ShowInfo(fmt.Sprintf("🔗 最新区块: %s", chainInfo.BlockHash))
	s.ui.ShowInfo(fmt.Sprintf("🌐 网络ID: %s", chainInfo.NetworkID))

	// 同步状态
	if chainInfo.IsSyncing {
		s.ui.ShowWarning("🔄 正在同步中...")
	} else {
		s.ui.ShowSuccess("✅ 同步完成")
	}

	// 获取交易池状态
	s.ui.ShowInfo("")
	s.ui.ShowInfo("正在查询交易池...")
	txPoolStatus, err := s.blockchain.GetTxPoolStatus(ctx)
	if err == nil && txPoolStatus != nil {
		s.ui.ShowInfo("💼 交易池状态:")
		s.ui.ShowInfo(fmt.Sprintf("  待处理: %d 笔", txPoolStatus.Pending))
		s.ui.ShowInfo(fmt.Sprintf("  排队中: %d 笔", txPoolStatus.Queued))
	}

	return nil
}

// formatTimestamp 格式化时间戳
func formatTimestamp(timestamp uint64) string {
	t := time.Unix(int64(timestamp), 0)
	return t.Format("2006-01-02 15:04:05")
}
