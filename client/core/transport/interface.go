// Package transport provides transport interface definitions for client operations.
package transport

import (
	"context"
	"time"
)

// Client 统一传输客户端接口 - CLI与节点通信的唯一通道
// 所有网络调用必须经由此接口，严禁CLI直接依赖internal/core
type Client interface {
	// ===== 链信息 =====

	// ChainID 获取链ID
	ChainID(ctx context.Context) (string, error)

	// Syncing 获取同步状态
	Syncing(ctx context.Context) (*SyncStatus, error)

	// BlockNumber 获取最新区块高度
	BlockNumber(ctx context.Context) (uint64, error)

	// ===== 区块查询(支持状态锚定) =====

	// GetBlockByHeight 根据高度获取区块
	GetBlockByHeight(ctx context.Context, height uint64, fullTx bool, anchor *StateAnchor) (*Block, error)

	// GetBlockByHash 根据哈希获取区块
	GetBlockByHash(ctx context.Context, hash string, fullTx bool) (*Block, error)

	// ===== 交易提交与查询 =====

	// SendTransaction 执行转账（节点内部完成构建→签名→提交）
	// 这是一个完整的转账接口，适用于CLI等信任环境
	SendTransaction(ctx context.Context, fromAddress string, toAddress string, amount uint64, privateKey []byte) (*SendTxResult, error)

	// SendRawTransaction 发送已签名交易（适用于外部签名场景）
	SendRawTransaction(ctx context.Context, signedTxHex string) (*SendTxResult, error)

	// GetTransaction 获取交易详情
	GetTransaction(ctx context.Context, txHash string) (*Transaction, error)

	// GetTransactionReceipt 获取交易回执
	GetTransactionReceipt(ctx context.Context, txHash string) (*Receipt, error)

	// GetTransactionHistory 查询交易历史
	GetTransactionHistory(ctx context.Context, txID string, resourceID string, limit int, offset int) ([]*Transaction, error)

	// EstimateFee 估算交易费用
	EstimateFee(ctx context.Context, tx *UnsignedTx) (*FeeEstimate, error)

	// ===== 状态查询(支持状态锚定) =====

	// GetBalance 获取账户余额
	GetBalance(ctx context.Context, address string, anchor *StateAnchor) (*Balance, error)

	// GetContractTokenBalance 获取账户在指定合约代币下的余额
	GetContractTokenBalance(ctx context.Context, req *ContractTokenBalanceRequest) (*ContractTokenBalanceResult, error)

	// GetUTXOs 获取账户UTXO列表
	GetUTXOs(ctx context.Context, address string, anchor *StateAnchor) ([]*UTXO, error)

	// Call 模拟合约调用(不上链)
	Call(ctx context.Context, call *CallRequest, anchor *StateAnchor) (*CallResult, error)

	// ===== 交易池(Mempool) =====

	// TxPoolStatus 获取交易池状态
	TxPoolStatus(ctx context.Context) (*TxPoolStatus, error)

	// TxPoolContent 获取交易池内容
	TxPoolContent(ctx context.Context) (*TxPoolContent, error)

	// ===== 订阅(WebSocket专用) =====

	// Subscribe 订阅事件(newHeads/logs/newPendingTxs)
	// resumeToken: 用于断线重连恢复,首次订阅传空字符串
	Subscribe(ctx context.Context, eventType SubscriptionType, filters map[string]interface{}, resumeToken string) (Subscription, error)

	// ===== SPV轻客户端 =====

	// GetBlockHeader 获取区块头(用于SPV验证)
	GetBlockHeader(ctx context.Context, height uint64) (*BlockHeader, error)

	// GetTxProof 获取交易的Merkle证明
	GetTxProof(ctx context.Context, txHash string) (*MerkleProof, error)

	// ===== 健康检查 =====

	// Ping 检查节点是否可达
	Ping(ctx context.Context) error

	// ===== 智能合约 =====

	// DeployContract 部署智能合约
	DeployContract(ctx context.Context, req *DeployContractRequest) (*DeployContractResult, error)

	// CallContract 调用智能合约
	CallContract(ctx context.Context, req *CallContractRequest) (*CallContractResult, error)

	// GetContract 查询合约元数据
	GetContract(ctx context.Context, contentHash string) (*ContractMetadata, error)

	// CallAIModel 调用AI模型
	CallAIModel(ctx context.Context, req *CallAIModelRequest) (*CallAIModelResult, error)

	// DeployAIModel 部署AI模型
	DeployAIModel(ctx context.Context, req *DeployAIModelRequest) (*DeployAIModelResult, error)

	// CallRaw 调用任意 JSON-RPC 方法（高级接口）
	// method: JSON-RPC 方法名
	// params: 参数数组
	// 返回：result 部分的原始反序列化结果（通常是 map[string]interface{} 或基础类型）
	CallRaw(ctx context.Context, method string, params []interface{}) (interface{}, error)

	// Close 关闭客户端连接
	Close() error
}

// StateAnchor 状态锚定参数 - 用于查询历史状态
type StateAnchor struct {
	Height *uint64 // 指定区块高度
	Hash   *string // 指定区块哈希
}

// SyncStatus 同步状态
type SyncStatus struct {
	Syncing       bool   `json:"syncing"`
	StartingBlock uint64 `json:"starting_block"`
	CurrentBlock  uint64 `json:"current_block"`
	HighestBlock  uint64 `json:"highest_block"`
}

// Block 区块数据
type Block struct {
	Height       uint64        `json:"height"`
	Hash         string        `json:"hash"`
	ParentHash   string        `json:"parent_hash"`
	Timestamp    time.Time     `json:"timestamp"`
	TxCount      int           `json:"tx_count"`
	Transactions []Transaction `json:"transactions,omitempty"` // fullTx=true时包含
	TxHashes     []string      `json:"tx_hashes,omitempty"`    // fullTx=false时包含
	StateRoot    string        `json:"state_root"`
	Miner        string        `json:"miner"`
	Difficulty   string        `json:"difficulty"`
}

// BlockHeader 区块头(SPV用)
type BlockHeader struct {
	Height     uint64    `json:"height"`
	Hash       string    `json:"hash"`
	ParentHash string    `json:"parent_hash"`
	Timestamp  time.Time `json:"timestamp"`
	StateRoot  string    `json:"state_root"`
	TxRoot     string    `json:"tx_root"`
	Difficulty string    `json:"difficulty"`
	Nonce      string    `json:"nonce"`
}

// Transaction 交易数据（完整结构，对应 transaction.proto）
type Transaction struct {
	// 基础信息
	Hash        string    `json:"tx_hash,omitempty"`        // 交易哈希（由 API 计算）
	Version     uint32    `json:"version,omitempty"`        // 交易版本号
	Nonce       uint64    `json:"nonce"`                    // 账户 nonce
	Timestamp   time.Time `json:"timestamp"`                // 时间戳（解析后）
	ChainID     string    `json:"chain_id,omitempty"`       // 链 ID
	Status      string    `json:"status"`                   // pending/confirmed/failed

	// 区块信息
	BlockHash   string `json:"block_hash,omitempty"`
	BlockHeight uint64 `json:"block_height,omitempty"`
	TxIndex     uint32 `json:"tx_index,omitempty"`

	// EUTXO 核心结构
	Inputs  []TxInput  `json:"inputs,omitempty"`  // 交易输入列表
	Outputs []TxOutput `json:"outputs,omitempty"` // 交易输出列表

	// 兼容字段（简化显示用）
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Value string `json:"value,omitempty"`
	Fee   string `json:"fee,omitempty"`

	// 原始数据（用于调试）
	RawData map[string]interface{} `json:"-"` // 不序列化，仅内部使用
}

// TxInput 交易输入（引用已有 UTXO）
type TxInput struct {
	PreviousOutput  *OutPoint `json:"previous_output,omitempty"`  // 引用的 UTXO 位置
	IsReferenceOnly bool      `json:"is_reference_only,omitempty"` // 是否只读引用
	Sequence        uint32    `json:"sequence,omitempty"`          // 序列号

	// 解锁证明（简化表示）
	UnlockingProofType string `json:"unlocking_proof_type,omitempty"` // 解锁类型
}

// OutPoint UTXO 位置引用
type OutPoint struct {
	TxID        string `json:"tx_id,omitempty"`        // 交易 ID（十六进制）
	OutputIndex uint32 `json:"output_index,omitempty"` // 输出索引
}

// TxOutput 交易输出（创建新 UTXO）
type TxOutput struct {
	Owner             string `json:"owner,omitempty"`              // 所有者地址
	OutputType        string `json:"output_type,omitempty"`        // 输出类型: asset/resource/state
	LockingConditions []any  `json:"locking_conditions,omitempty"` // 锁定条件

	// 资产输出（asset）
	Asset *AssetOutput `json:"asset,omitempty"`

	// 资源输出（resource）
	Resource *ResourceOutput `json:"resource,omitempty"`

	// 状态输出（state）
	State *StateOutput `json:"state,omitempty"`
}

// AssetOutput 资产输出
type AssetOutput struct {
	// 原生币
	NativeCoin *NativeCoinAsset `json:"native_coin,omitempty"`
	// 合约代币
	ContractToken *ContractTokenAsset `json:"contract_token,omitempty"`
}

// NativeCoinAsset 原生币资产
type NativeCoinAsset struct {
	Amount string `json:"amount,omitempty"`
}

// ContractTokenAsset 合约代币资产
type ContractTokenAsset struct {
	ContractAddress string `json:"contract_address,omitempty"`
	Amount          string `json:"amount,omitempty"`
}

// ResourceOutput 资源输出
type ResourceOutput struct {
	ContentHash       string `json:"content_hash,omitempty"`
	Category          string `json:"category,omitempty"`           // EXECUTABLE/STATIC
	ExecutableType    string `json:"executable_type,omitempty"`    // CONTRACT/AI_MODEL
	MimeType          string `json:"mime_type,omitempty"`
	Size              int64  `json:"size,omitempty"`
	CreationTimestamp uint64 `json:"creation_timestamp,omitempty"`
	IsImmutable       bool   `json:"is_immutable,omitempty"`
}

// StateOutput 状态输出
type StateOutput struct {
	StateID             string `json:"state_id,omitempty"`
	StateVersion        uint64 `json:"state_version,omitempty"`
	ExecutionResultHash string `json:"execution_result_hash,omitempty"`
	ParentStateHash     string `json:"parent_state_hash,omitempty"`
}

// Receipt 交易回执
type Receipt struct {
	TxHash      string `json:"tx_hash"`
	BlockHash   string `json:"block_hash"`
	BlockHeight uint64 `json:"block_height"`
	Status      string `json:"status"` // success/failed
	GasUsed     string `json:"gas_used"`
	Logs        []Log  `json:"logs"`
}

// Log 事件日志
type Log struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// SendTxResult 交易提交结果
type SendTxResult struct {
	TxHash   string `json:"tx_hash"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"` // 拒绝原因
}

// UnsignedTx 未签名交易(用于费用估算)
type UnsignedTx struct {
	From   string            `json:"from"`
	To     string            `json:"to"`
	Value  string            `json:"value"`
	Data   string            `json:"data,omitempty"`
	Nonce  uint64            `json:"nonce"`
	Params map[string]string `json:"params,omitempty"`
}

// FeeEstimate 费用估算结果
type FeeEstimate struct {
	BaseFee      string `json:"base_fee"`
	PriorityFee  string `json:"priority_fee"`
	TotalFee     string `json:"total_fee"`
	GasLimit     uint64 `json:"gas_limit"`
	SuggestedTip string `json:"suggested_tip"`
}

// Balance 账户余额
type Balance struct {
	Address   string    `json:"address"`
	Balance   string    `json:"balance"`
	Height    uint64    `json:"height"`
	Hash      string    `json:"hash"`
	StateRoot string    `json:"state_root"`
	Timestamp time.Time `json:"timestamp"`
}

// ContractTokenBalanceRequest 合约代币余额查询请求
type ContractTokenBalanceRequest struct {
	Address     string `json:"address"`
	ContentHash string `json:"content_hash"`
	TokenID     string `json:"token_id,omitempty"`
}

// ContractTokenBalanceResult 合约代币余额查询结果
type ContractTokenBalanceResult struct {
	Address         string `json:"address"`
	ContentHash     string `json:"content_hash"`
	ContractAddress string `json:"contract_address"`
	TokenID         string `json:"token_id"`
	Balance         string `json:"balance"`
	BalanceHex      string `json:"balance_hex"`
	BalanceUint64   uint64 `json:"balance_uint64"`
	Height          string `json:"height"`
	Hash            string `json:"hash,omitempty"`
	StateRoot       string `json:"stateRoot,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	UTXOCount       int    `json:"utxo_count"`
}

// UTXO 未花费输出
type UTXO struct {
	TxHash        string `json:"tx_hash"`
	OutputIndex   uint32 `json:"output_index"`
	Amount        string `json:"amount"`
	Address       string `json:"address"`
	LockScript    string `json:"lock_script"`
	Confirmations uint64 `json:"confirmations"`
}

// CallRequest 合约调用请求
type CallRequest struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
	Data  string `json:"data"`
	Value string `json:"value,omitempty"`
}

// CallResult 合约调用结果
type CallResult struct {
	Output  string `json:"output"`
	GasUsed string `json:"gas_used"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// TxPoolStatus 交易池状态
type TxPoolStatus struct {
	Pending int `json:"pending"`
	Queued  int `json:"queued"`
	Total   int `json:"total"`
}

// TxPoolContent 交易池内容
type TxPoolContent struct {
	Pending map[string][]*Transaction `json:"pending"` // address -> txs
	Queued  map[string][]*Transaction `json:"queued"`  // address -> txs
}

// SubscriptionType 订阅类型
type SubscriptionType string

const (
	SubscribeNewHeads      SubscriptionType = "newHeads"      // 新区块
	SubscribeLogs          SubscriptionType = "logs"          // 事件日志
	SubscribeNewPendingTxs SubscriptionType = "newPendingTxs" // 新待处理交易
)

// Subscription 订阅接口
type Subscription interface {
	// Events 获取事件通道
	Events() <-chan *Event

	// Err 获取错误通道
	Err() <-chan error

	// Unsubscribe 取消订阅
	Unsubscribe()
}

// Event 订阅事件
type Event struct {
	Type SubscriptionType       `json:"type"`
	Data map[string]interface{} `json:"data"`

	// 重组安全字段
	Removed     bool   `json:"removed"`      // 是否被重组移除
	ReorgID     string `json:"reorg_id"`     // 重组标识符
	ResumeToken string `json:"resume_token"` // 可恢复游标

	// 状态锚定
	Height    uint64    `json:"height"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

// MerkleProof Merkle证明(SPV用)
type MerkleProof struct {
	TxHash      string   `json:"tx_hash"`
	BlockHash   string   `json:"block_hash"`
	BlockHeight uint64   `json:"block_height"`
	TxIndex     uint32   `json:"tx_index"`
	Siblings    []string `json:"siblings"` // Merkle路径
	Root        string   `json:"root"`     // Merkle根
}

// ============================================================================
// 智能合约相关类型
// ============================================================================

// DeployContractRequest 部署合约请求
type DeployContractRequest struct {
	PrivateKey        string // 十六进制私钥
	WasmContentBase64 string // Base64编码的WASM文件内容
	AbiVersion        string // ABI版本（如: v1）
	Name              string // 合约名称
	Description       string // 合约描述（可选）
}

// DeployContractResult 部署合约结果
type DeployContractResult struct {
	ContentHash string // 合约ID（64位十六进制ContentHash）
	TxHash      string // 交易哈希
	Success     bool   // 是否成功
	Message     string // 结果消息
}

// CallContractRequest 调用合约请求
type CallContractRequest struct {
	PrivateKey    string   // 十六进制私钥
	ContentHash   string   // 合约ID（64位十六进制）
	Method        string   // 方法名
	Params        []uint64 // 方法参数（u64数组）
	PayloadBase64 string   // Base64编码的额外数据（可选）
}

// CallContractResult 调用合约结果
type CallContractResult struct {
	TxHash     string                   // 交易哈希
	Results    []uint64                 // 返回值（u64数组）
	ReturnData string                   // Base64编码的返回数据
	Events     []map[string]interface{} // 事件列表
	Success    bool                     // 是否成功
	Message    string                   // 结果消息
}

// ContractMetadata 合约元数据
type ContractMetadata struct {
	ContentHash       string   `json:"content_hash"`       // 合约ID
	Name              string   `json:"name"`               // 合约名称
	Version           string   `json:"version"`            // 合约版本
	AbiVersion        string   `json:"abi_version"`        // ABI版本
	ExportedFunctions []string `json:"exported_functions"` // 导出函数列表
	Description       string   `json:"description"`        // 合约描述
	Size              int64    `json:"size"`               // WASM文件大小
	MimeType          string   `json:"mime_type"`          // MIME类型
	CreationTime      int64    `json:"creation_time"`      // 创建时间戳
	Owner             string   `json:"owner"`              // 部署者地址
	Success           bool     `json:"success"`            // 查询是否成功
	Message           string   `json:"message,omitempty"`  // 错误消息（如果有）
}

// CallAIModelRequest 调用AI模型请求
type CallAIModelRequest struct {
	PrivateKey string                   // 十六进制私钥
	ModelHash  string                   // 模型ID（64位十六进制）
	Inputs     []map[string]interface{} // 张量输入列表
}

// TensorOutput 张量输出（与 JSON-RPC tensor_outputs 对应）
type TensorOutput struct {
	Name         string    `json:"name"`                   // 张量名称
	DType        string    `json:"dtype"`                  // 数据类型，例如 "float64"
	Shape        []int64   `json:"shape"`                  // 张量形状
	Layout       string    `json:"layout,omitempty"`       // 可选：布局信息，例如 "NCHW"
	Encoding     string    `json:"encoding"`               // 原始数据编码方式，例如 "base64"
	RawData      string    `json:"raw_data,omitempty"`     // 原始字节（按 encoding 编码）
	Values       []float64 `json:"values"`                 // 展平的数值视图（便于直接消费）
	Quantization any       `json:"quantization,omitempty"` // 量化信息（暂未使用）
}

// ElementCount 返回当前张量中元素的总数（按 Values 视图计算）
func (t TensorOutput) ElementCount() int {
	return len(t.Values)
}

// IsScalar 返回当前张量是否为标量（shape 为空或所有维度乘积为 1）
func (t TensorOutput) IsScalar() bool {
	if len(t.Shape) == 0 {
		return len(t.Values) == 1
	}
	total := int64(1)
	for _, d := range t.Shape {
		if d <= 0 {
			return false
		}
		total *= d
	}
	return total == 1
}

// ShapeProduct 返回 Shape 中各维度的乘积（若包含非正维度则返回 -1）
func (t TensorOutput) ShapeProduct() int64 {
	if len(t.Shape) == 0 {
		return int64(len(t.Values))
	}
	total := int64(1)
	for _, d := range t.Shape {
		if d <= 0 {
			return -1
		}
		total *= d
	}
	return total
}

// ToFloat32Slice 将 Values 转为 float32 切片（常用于 dtype=float32 场景）
func (t TensorOutput) ToFloat32Slice() []float32 {
	out := make([]float32, len(t.Values))
	for i, v := range t.Values {
		out[i] = float32(v)
	}
	return out
}

// CallAIModelResult 调用AI模型结果
type CallAIModelResult struct {
	TxHash        string         `json:"tx_hash"`         // 交易哈希
	TensorOutputs []TensorOutput `json:"tensor_outputs"`  // 统一张量输出
	Success       bool           `json:"success"`         // 是否成功
	Message       string         `json:"message"`         // 结果消息
}

// DeployAIModelRequest 部署AI模型请求
type DeployAIModelRequest struct {
	PrivateKey  string // 十六进制私钥
	OnnxContent string // Base64编码的ONNX文件内容
	Name        string // 模型名称
	Description string // 模型描述（可选）
}

// DeployAIModelResult 部署AI模型结果
type DeployAIModelResult struct {
	ContentHash string // 模型ID（64位十六进制ContentHash）
	TxHash      string // 交易哈希
	Success     bool   // 是否成功
	Message     string // 结果消息
}
