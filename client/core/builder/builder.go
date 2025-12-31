package builder

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/weisyn/v1/client/core/transport"
)

// DefaultTxBuilder 默认交易构建器实现
type DefaultTxBuilder struct {
	client transport.Client
}

// NewTxBuilder 创建交易构建器
func NewTxBuilder(client transport.Client) TxBuilder {
	return &DefaultTxBuilder{
		client: client,
	}
}

// CreateDraft 创建交易草稿
func (b *DefaultTxBuilder) CreateDraft() *DraftTx {
	return &DraftTx{
		inputs:  make([]Input, 0),
		outputs: make([]Output, 0),
		params:  TxParams{},
		builder: b,
	}
}

// LoadDraft 从文件加载草稿
func (b *DefaultTxBuilder) LoadDraft(filePath string) (*DraftTx, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read draft file: %w", err)
	}

	var draft DraftTx
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("unmarshal draft: %w", err)
	}

	draft.builder = b
	return &draft, nil
}

// computeTxID 计算交易ID
func (b *DefaultTxBuilder) computeTxID(draft *DraftTx) (string, error) {
	// 序列化交易核心数据
	data, err := b.serializeTxCore(draft)
	if err != nil {
		return "", err
	}

	// 计算SHA-256哈希
	hash := sha256.Sum256(data)
	return "0x" + bytesToHex(hash[:]), nil
}

// serializeTxCore 序列化交易核心数据(用于计算TxID)
func (b *DefaultTxBuilder) serializeTxCore(draft *DraftTx) ([]byte, error) {
	// 简化版序列化:仅序列化inputs和outputs
	// 实际实现应该按照WES的交易格式规范

	type coreTx struct {
		Inputs  []Input  `json:"inputs"`
		Outputs []Output `json:"outputs"`
		Params  TxParams `json:"params"`
	}

	core := coreTx{
		Inputs:  draft.inputs,
		Outputs: draft.outputs,
		Params:  draft.params,
	}

	return json.Marshal(core)
}

// serializeTx 序列化完整交易(含证明和签名)
// ⚠️ 修复：改用 protobuf 序列化，节点不接受 JSON
func (b *DefaultTxBuilder) serializeTx(proven *ProvenTx) ([]byte, error) {
	// TODO: 将 builder 内部类型转换为 protobuf Transaction
	// 当前 builder 使用的是简化的 Input/Output 结构，需要转换为 pb.Transaction

	// 临时方案：返回错误，提示需要使用 protobuf
	return nil, fmt.Errorf("serializeTx: builder 使用 JSON 序列化，节点需要 protobuf。请使用 internal/core/tx 构建交易")
}

// saveDraft 保存草稿到文件
func (b *DefaultTxBuilder) saveDraft(draft *DraftTx, filePath string) error {
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draft: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write draft file: %w", err)
	}

	return nil
}

// saveComposed 保存组合交易到文件
func (b *DefaultTxBuilder) saveComposed(composed *ComposedTx, filePath string) error {
	data, err := json.MarshalIndent(composed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal composed: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// saveSigned 保存签名交易到文件
func (b *DefaultTxBuilder) saveSigned(signed *SignedTx, filePath string) error {
	type signedTxFile struct {
		TxID       string      `json:"tx_id"`
		RawHex     string      `json:"raw_hex"`
		Signatures []Signature `json:"signatures"`
	}

	file := signedTxFile{
		TxID:       signed.Hash(),
		RawHex:     signed.RawHex(),
		Signatures: signed.Signatures(),
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signed tx: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// sendTx 发送交易到节点
func (b *DefaultTxBuilder) sendTx(client transport.Client, signed *SignedTx) (*transport.SendTxResult, error) {
	ctx := context.Background()
	return client.SendRawTransaction(ctx, signed.RawHex())
}

// ===== 确保实现了接口 =====

var _ TxBuilder = (*DefaultTxBuilder)(nil)
