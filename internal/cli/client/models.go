package client

import (
	"encoding/json"
	"fmt"
	"time"

	// 导入protobuf结构
	blockpb "github.com/weisyn/v1/pb/blockchain/block"
)

// APIResponse API响应的通用结构
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   APIError        `json:"error,omitempty"`
}

// APIError API错误信息
type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"details,omitempty"`
}

// BalanceInfo 账户余额信息
type BalanceInfo struct {
	Address struct {
		RawHash string `json:"raw_hash"` // base64编码的地址哈希
	} `json:"address"`
	TokenID      interface{} `json:"token_id"` // 可能为null
	Available    uint64      `json:"available"`
	Locked       uint64      `json:"locked"`
	Pending      uint64      `json:"pending"`
	Total        uint64      `json:"total"`
	UTXOCount    int         `json:"utxo_count"`
	LastUpdated  string      `json:"last_updated"` // 时间字符串格式
	UpdateHeight uint64      `json:"update_height"`
}

// GetAddressString 获取地址的字符串表示（简化处理）
func (b *BalanceInfo) GetAddressString() string {
	if b.Address.RawHash == "" {
		return "unknown"
	}
	// 这里应该实现Base64到Base58的转换，但为了简化，我们直接显示前几个字符
	if len(b.Address.RawHash) > 8 {
		return b.Address.RawHash[:8] + "..."
	}
	return b.Address.RawHash
}

// ToFloat64 将余额转换为浮点数显示（使用系统的8位小数精度）
func (b *BalanceInfo) ToFloat64() float64 {
	// 使用系统正确的精度：1 WES = 100,000,000 wei (1e8)
	return float64(b.Available) / 1e8
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID             string   `json:"node_id"`
	Success            bool     `json:"success"`
	Addresses          []string `json:"addresses"`
	ActualListenAddrs  []string `json:"actual_listen_addrs"`
	ActualListenCount  int      `json:"actual_listen_count"`
	AddressCount       int      `json:"address_count"`
	ProtocolCount      int      `json:"protocol_count"`
	SupportedProtocols []string `json:"supported_protocols"`
	Note               string   `json:"note"`

	// 兼容性字段 - 从其他字段计算得出
	Version     string    `json:"-"`
	Uptime      int64     `json:"-"`
	BlockHeight uint64    `json:"-"`
	PeerCount   int       `json:"-"` // 使用AddressCount作为PeerCount的近似值
	IsMining    bool      `json:"-"`
	LastSyncAt  time.Time `json:"-"`
}

// GetPeerCount 获取连接的节点数量（使用AddressCount作为近似值）
func (n *NodeInfo) GetPeerCount() int {
	return n.AddressCount
}

// BlockInfo 区块信息 - 基于protobuf结构
type BlockInfo struct {
	// 使用protobuf Block结构
	*blockpb.Block
}

// NewBlockInfoFromProto 从protobuf Block创建BlockInfo
func NewBlockInfoFromProto(block *blockpb.Block) *BlockInfo {
	return &BlockInfo{Block: block}
}

// GetHeight 获取区块高度
func (b *BlockInfo) GetHeight() uint64 {
	if b.Header != nil {
		return b.Header.Height
	}
	return 0
}

// GetChainID 获取链ID
func (b *BlockInfo) GetChainID() uint64 {
	if b.Header != nil {
		return b.Header.ChainId
	}
	return 0
}

// GetVersion 获取版本
func (b *BlockInfo) GetVersion() uint64 {
	if b.Header != nil {
		return b.Header.Version
	}
	return 0
}

// GetPreviousHash 获取前一区块哈希（十六进制字符串）
func (b *BlockInfo) GetPreviousHashHex() string {
	if b.Header != nil && len(b.Header.PreviousHash) > 0 {
		return fmt.Sprintf("%x", b.Header.PreviousHash)
	}
	return ""
}

// GetMerkleRoot 获取Merkle根（十六进制字符串）
func (b *BlockInfo) GetMerkleRootHex() string {
	if b.Header != nil && len(b.Header.MerkleRoot) > 0 {
		return fmt.Sprintf("%x", b.Header.MerkleRoot)
	}
	return ""
}

// GetNonce 获取随机数（十六进制字符串）
func (b *BlockInfo) GetNonceHex() string {
	if b.Header != nil && len(b.Header.Nonce) > 0 {
		return fmt.Sprintf("%x", b.Header.Nonce)
	}
	return ""
}

// GetDifficulty 获取难度
func (b *BlockInfo) GetDifficulty() uint64 {
	if b.Header != nil {
		return b.Header.Difficulty
	}
	return 0
}

// GetTimestamp 获取时间戳
func (b *BlockInfo) GetTimestamp() uint64 {
	if b.Header != nil {
		return b.Header.Timestamp
	}
	return 0
}

// GetTxCount 获取交易数量
func (b *BlockInfo) GetTxCount() int {
	if b.Body != nil && b.Body.Transactions != nil {
		return len(b.Body.Transactions)
	}
	return 0
}

// GetFormattedTime 获取格式化的时间
func (b *BlockInfo) GetFormattedTime() string {
	if b.Header != nil {
		return time.Unix(int64(b.Header.Timestamp), 0).Format("2006-01-02 15:04:05")
	}
	return ""
}

// MiningStatus 挖矿状态
type MiningStatus struct {
	CurrentHeight *uint64    `json:"current_height"`
	IsMining      bool       `json:"is_mining"`
	MinerAddress  string     `json:"miner_address"`
	StartTime     *time.Time `json:"start_time"`

	// 兼容性字段 - 从其他字段计算得出
	IsActive    bool    `json:"-"` // 使用IsMining
	HashRate    float64 `json:"-"`
	BlocksMined int64   `json:"-"`
	Difficulty  string  `json:"-"`
	TargetTime  int     `json:"-"`
	LastBlock   string  `json:"-"`
	Uptime      int64   `json:"-"`
}

// GetIsActive 获取挖矿活跃状态
func (m *MiningStatus) GetIsActive() bool {
	return m.IsMining
}

// GetHashRateFormatted 获取格式化的哈希率
func (m *MiningStatus) GetHashRateFormatted() string {
	if m.HashRate >= 1e9 {
		return fmt.Sprintf("%.2f GH/s", m.HashRate/1e9)
	} else if m.HashRate >= 1e6 {
		return fmt.Sprintf("%.2f MH/s", m.HashRate/1e6)
	} else if m.HashRate >= 1e3 {
		return fmt.Sprintf("%.2f KH/s", m.HashRate/1e3)
	}
	return fmt.Sprintf("%.2f H/s", m.HashRate)
}

// PeerInfo 对等节点信息
type PeerInfo struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	Direction string    `json:"direction"` // inbound/outbound
	Protocol  string    `json:"protocol"`
	Latency   int64     `json:"latency"` // 毫秒
	LastSeen  time.Time `json:"last_seen"`
}

// GetLatencyFormatted 获取格式化的延迟
func (p *PeerInfo) GetLatencyFormatted() string {
	return fmt.Sprintf("%d ms", p.Latency)
}

// TransferRequest 转账请求
type TransferRequest struct {
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	Amount      string `json:"amount"`
	TokenID     string `json:"token_id,omitempty"`
	FeeAmount   string `json:"fee_amount"`
	Memo        string `json:"memo,omitempty"`
	// 🔐 关键修复：添加私钥字段用于区块链交易签名
	SenderPrivateKey string `json:"sender_private_key"` // 发送方私钥（用于签名）
}

// TransferResponse 转账响应
type TransferResponse struct {
	TransactionHash string `json:"transaction_hash"`
	Message         string `json:"message"`
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	Hash        string    `json:"hash"`
	BlockHash   string    `json:"block_hash"`
	BlockHeight uint64    `json:"block_height"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Amount      uint64    `json:"amount"`
	Fee         uint64    `json:"fee"`
	Status      string    `json:"status"`
	Timestamp   int64     `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetAmountFormatted 获取格式化的金额
func (t *TransactionInfo) GetAmountFormatted() string {
	// 使用系统正确的精度：1 WES = 100,000,000 wei (1e8)
	return fmt.Sprintf("%.8f WES", float64(t.Amount)/1e8)
}

// GetFeeFormatted 获取格式化的手续费
func (t *TransactionInfo) GetFeeFormatted() string {
	// 使用系统正确的精度：1 WES = 100,000,000 wei (1e8)
	return fmt.Sprintf("%.8f WES", float64(t.Fee)/1e8)
}

// GetFormattedTime 获取格式化的时间
func (t *TransactionInfo) GetFormattedTime() string {
	return time.Unix(t.Timestamp, 0).Format("2006-01-02 15:04:05")
}
