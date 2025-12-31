// Package types provides WebSocket event type definitions.
package types

// SubscriptionEvent WebSocket订阅事件
type SubscriptionEvent struct {
	Subscription string      `json:"subscription"` // 订阅ID
	Result       interface{} `json:"result"`       // 事件数据
}

// NewHeadEvent 新区块头事件
type NewHeadEvent struct {
	Type        string `json:"type"`        // "newHead"
	Height      uint64 `json:"height"`      // 区块高度
	Hash        string `json:"hash"`        // 区块哈希
	ParentHash  string `json:"parentHash"`  // 父区块哈希
	Timestamp   int64  `json:"timestamp"`   // 时间戳
	StateRoot   string `json:"stateRoot"`   // 状态根
	TxCount     int    `json:"txCount"`     // 交易数量
	Removed     bool   `json:"removed"`     // ⭐ 是否被重组移除
	ReorgID     string `json:"reorgId"`     // ⭐ 重组标识符
	ResumeToken string `json:"resumeToken"` // ⭐ 可恢复订阅的游标
}

// NewPendingTxEvent 新待处理交易事件
type NewPendingTxEvent struct {
	Type   string `json:"type"`   // "newPendingTx"
	TxHash string `json:"txHash"` // 交易哈希
}

// ReorgEvent 链重组事件
type ReorgEvent struct {
	Type            string   `json:"type"`            // "reorg"
	ReorgID         string   `json:"reorgId"`         // 重组标识符
	OldChainTip     string   `json:"oldChainTip"`     // 旧链顶端
	NewChainTip     string   `json:"newChainTip"`     // 新链顶端
	CommonAncestor  string   `json:"commonAncestor"`  // 共同祖先
	RemovedBlocks   []string `json:"removedBlocks"`   // 被移除的区块
	AddedBlocks     []string `json:"addedBlocks"`     // 新增的区块
	AffectedTxCount int      `json:"affectedTxCount"` // 受影响的交易数量
}

// LogsEvent 合约日志事件
type LogsEvent struct {
	Type             string   `json:"type"`             // "logs"
	Address          string   `json:"address"`          // 合约地址
	Topics           []string `json:"topics"`           // 日志主题
	Data             string   `json:"data"`             // 日志数据
	BlockHeight      uint64   `json:"blockHeight"`      // 区块高度
	BlockHash        string   `json:"blockHash"`        // 区块哈希
	TransactionHash  string   `json:"transactionHash"`  // 交易哈希
	TransactionIndex int      `json:"transactionIndex"` // 交易索引
	LogIndex         int      `json:"logIndex"`         // 日志索引
	Removed          bool     `json:"removed"`          // ⭐ 是否被重组移除
}
