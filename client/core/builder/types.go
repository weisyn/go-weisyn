package builder

import (
	"time"

	"github.com/weisyn/v1/client/core/transport"
)

// TxBuilder 交易构建器接口 - Type-State模式的入口
type TxBuilder interface {
	// CreateDraft 创建交易草稿
	CreateDraft() *DraftTx

	// LoadDraft 从文件加载草稿
	LoadDraft(filePath string) (*DraftTx, error)
}

// ===== Type-State 交易状态 =====

// DraftTx 草稿交易(可变状态)
// 可以添加inputs/outputs,修改参数
type DraftTx struct {
	inputs  []Input
	outputs []Output
	params  TxParams
	builder *DefaultTxBuilder
}

// ComposedTx 组合交易(不可变状态)
// 已密封,不可再修改,可计算TxID
type ComposedTx struct {
	txID    string
	inputs  []Input
	outputs []Output
	params  TxParams
	builder *DefaultTxBuilder
}

// ProvenTx 证明交易(含授权证明)
// 已添加UnlockingProof
type ProvenTx struct {
	composed *ComposedTx
	proofs   []UnlockingProof
	builder  *DefaultTxBuilder
}

// SignedTx 签名交易(可广播)
// 含完整签名,可提交到节点
type SignedTx struct {
	proven     *ProvenTx
	signatures []Signature
	raw        []byte // 序列化后的原始交易
	builder    *DefaultTxBuilder
}

// ===== 输入输出类型 =====

// Input 交易输入
type Input struct {
	// UTXO引用
	TxHash      string `json:"tx_hash"`
	OutputIndex uint32 `json:"output_index"`

	// UTXO信息
	Amount     string `json:"amount"`
	Address    string `json:"address"`
	LockScript string `json:"lock_script"`

	// 解锁信息(后续填充)
	UnlockScript string `json:"unlock_script,omitempty"`
}

// Output 交易输出
type Output struct {
	// 接收方
	Address string `json:"address"`
	Amount  string `json:"amount"`

	// 锁定脚本
	LockScript string `json:"lock_script"`

	// 输出类型
	Type OutputType `json:"type"`

	// 扩展数据(合约/资源等)
	Data map[string]interface{} `json:"data,omitempty"`
}

// OutputType 输出类型
type OutputType string

const (
	OutputTypeTransfer OutputType = "transfer" // 普通转账
	OutputTypeContract OutputType = "contract" // 合约部署/调用
	OutputTypeResource OutputType = "resource" // 资源输出
	OutputTypeState    OutputType = "state"    // 状态输出
)

// TxParams 交易参数
type TxParams struct {
	// 费用
	FeeRate string `json:"fee_rate,omitempty"` // 费率
	MaxFee  string `json:"max_fee,omitempty"`  // 最大费用

	// 时间锁
	LockTime uint64 `json:"lock_time,omitempty"`

	// 备注
	Memo string `json:"memo,omitempty"`

	// 其他参数
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// UnlockingProof 解锁证明
type UnlockingProof struct {
	InputIndex int    `json:"input_index"` // 对应的输入索引
	Type       string `json:"type"`        // 证明类型(signature/script)
	Data       []byte `json:"data"`        // 证明数据
}

// Signature 签名
type Signature struct {
	Signer    string    `json:"signer"`     // 签名者地址
	PublicKey string    `json:"public_key"` // 公钥
	Signature []byte    `json:"signature"`  // 签名数据
	Timestamp time.Time `json:"timestamp"`  // 签名时间
}

// ===== DraftTx 方法 =====

// AddInput 添加输入
func (d *DraftTx) AddInput(input Input) *DraftTx {
	d.inputs = append(d.inputs, input)
	return d
}

// AddOutput 添加输出
func (d *DraftTx) AddOutput(output Output) *DraftTx {
	d.outputs = append(d.outputs, output)
	return d
}

// SetParams 设置参数
func (d *DraftTx) SetParams(params TxParams) *DraftTx {
	d.params = params
	return d
}

// SetFeeRate 设置费率
func (d *DraftTx) SetFeeRate(feeRate string) *DraftTx {
	d.params.FeeRate = feeRate
	return d
}

// SetMemo 设置备注
func (d *DraftTx) SetMemo(memo string) *DraftTx {
	d.params.Memo = memo
	return d
}

// GetParams 获取交易参数
func (d *DraftTx) GetParams() TxParams {
	return d.params
}

// Seal 密封交易,转换为ComposedTx
// 这是从可变状态到不可变状态的唯一出口
func (d *DraftTx) Seal() (*ComposedTx, error) {
	// 验证交易有效性
	if err := d.validate(); err != nil {
		return nil, err
	}

	// 计算TxID
	txID, err := d.builder.computeTxID(d)
	if err != nil {
		return nil, err
	}

	return &ComposedTx{
		txID:    txID,
		inputs:  d.inputs,
		outputs: d.outputs,
		params:  d.params,
		builder: d.builder,
	}, nil
}

// Save 保存草稿到文件
func (d *DraftTx) Save(filePath string) error {
	return d.builder.saveDraft(d, filePath)
}

// validate 验证草稿交易
func (d *DraftTx) validate() error {
	// 基本验证
	if len(d.inputs) == 0 {
		return ErrNoInputs
	}
	if len(d.outputs) == 0 {
		return ErrNoOutputs
	}

	// 余额验证
	if err := d.validateBalance(); err != nil {
		return err
	}

	return nil
}

// validateBalance 验证余额
func (d *DraftTx) validateBalance() error {
	// TODO: 实现余额验证逻辑
	// 输入总额 >= 输出总额 + 费用
	return nil
}

// ===== ComposedTx 方法 =====

// TxID 获取交易ID
func (c *ComposedTx) TxID() string {
	return c.txID
}

// Inputs 获取输入列表
func (c *ComposedTx) Inputs() []Input {
	return c.inputs
}

// Outputs 获取输出列表
func (c *ComposedTx) Outputs() []Output {
	return c.outputs
}

// WithProofs 添加解锁证明,转换为ProvenTx
func (c *ComposedTx) WithProofs(proofs []UnlockingProof) (*ProvenTx, error) {
	// 验证证明数量与输入数量匹配
	if len(proofs) != len(c.inputs) {
		return nil, ErrProofCountMismatch
	}

	return &ProvenTx{
		composed: c,
		proofs:   proofs,
		builder:  c.builder,
	}, nil
}

// Save 保存到文件
func (c *ComposedTx) Save(filePath string) error {
	return c.builder.saveComposed(c, filePath)
}

// ===== ProvenTx 方法 =====

// Sign 签名交易,转换为SignedTx
func (p *ProvenTx) Sign(signer transport.Client, signers map[string]string) (*SignedTx, error) {
	// TODO: 实际实现需要调用wallet.Signer
	// 这里简化处理
	signatures := make([]Signature, 0, len(signers))

	for address, _ := range signers {
		sig := Signature{
			Signer:    address,
			Timestamp: time.Now(),
		}
		signatures = append(signatures, sig)
	}

	// 序列化交易
	raw, err := p.builder.serializeTx(p)
	if err != nil {
		return nil, err
	}

	return &SignedTx{
		proven:     p,
		signatures: signatures,
		raw:        raw,
		builder:    p.builder,
	}, nil
}

// TxID 获取交易ID
func (p *ProvenTx) TxID() string {
	return p.composed.txID
}

// ===== SignedTx 方法 =====

// Raw 获取原始交易数据
func (s *SignedTx) Raw() []byte {
	return s.raw
}

// RawHex 获取十六进制格式的原始交易
func (s *SignedTx) RawHex() string {
	return "0x" + bytesToHex(s.raw)
}

// Hash 获取交易哈希
func (s *SignedTx) Hash() string {
	return s.proven.composed.txID
}

// Signatures 获取签名列表
func (s *SignedTx) Signatures() []Signature {
	return s.signatures
}

// Save 保存到文件
func (s *SignedTx) Save(filePath string) error {
	return s.builder.saveSigned(s, filePath)
}

// Send 发送交易到节点
func (s *SignedTx) Send(client transport.Client) (*transport.SendTxResult, error) {
	return s.builder.sendTx(client, s)
}

// ===== 错误定义 =====

var (
	ErrNoInputs           = newTxError("no inputs")
	ErrNoOutputs          = newTxError("no outputs")
	ErrProofCountMismatch = newTxError("proof count mismatch")
	ErrInsufficientFunds  = newTxError("insufficient funds")
	ErrInvalidAddress     = newTxError("invalid address")
)

// TxError 交易错误
type TxError struct {
	message string
}

func newTxError(message string) *TxError {
	return &TxError{message: message}
}

func (e *TxError) Error() string {
	return e.message
}

// ===== 辅助函数 =====

// bytesToHex 字节转十六进制
func bytesToHex(data []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}
