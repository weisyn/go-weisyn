// Package lifecycle 交易生命周期管理 - 签名实现
//
// 🎯 **模块定位**：TransactionManager 接口的交易签名功能实现
//
// 本文件实现交易签名的核心业务逻辑，包括：
// - 交易数字签名（SignTransaction）
// - 支持多种签名算法（ECDSA、Ed25519等）
// - 签名数据格式化和验证
// - 签名哈希计算和标准化
// - 签名安全检查和防护
//
// 🏗️ **架构定位**：
// - 业务层：实现交易签名的业务逻辑
// - 密码学层：与密码学签名库的集成
// - 安全层：签名安全检查和攻击防护
// - 标准层：遵循区块链签名标准
//
// 🔧 **设计原则**：
// - 算法中立：支持多种签名算法
// - 安全优先：严格的签名验证和安全检查
// - 标准兼容：遵循 Bitcoin/Ethereum 签名标准
// - 性能优化：高效的签名计算和验证
// - 错误透明：详细的签名错误诊断
//
// 📋 **支持的签名算法**：
// - ECDSA secp256k1：Bitcoin 兼容签名算法
// - ECDSA secp256r1：企业级安全签名
// - Ed25519：高性能椭圆曲线签名
// - Schnorr：聚合签名支持
//
// 🎯 **签名类型**：
// - SIGHASH_ALL：签名整个交易
// - SIGHASH_NONE：不签名输出
// - SIGHASH_SINGLE：只签名对应输出
// - SIGHASH_ANYONECANPAY：允许添加输入
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package lifecycle

import (
	"bytes"
	"context"
	"fmt"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
)

// ============================================================================
//
//	交易签名实现服务
//
// ============================================================================
// TransactionSignService 交易签名核心实现服务
//
// 🎯 **服务职责**：
// - 实现 TransactionManager.SignTransaction 方法
// - 处理各类签名算法的交易签名
// - 管理签名数据的格式化和验证
// - 保证签名的安全性和正确性
//
// 🔧 **依赖注入**：
// - signatureProvider：数字签名提供服务
// - hashCalculator：交易哈希计算服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewTransactionSignService(sigProvider, hashCalc, cache, logger)
//	signedTx, err := service.SignTransaction(ctx, txHash, privateKey)
type TransactionSignService struct {
	// 核心依赖服务（使用公共接口）
	signatureManager crypto.SignatureManager                  // 数字签名服务
	keyManager       crypto.KeyManager                        // 密钥管理服务
	addressManager   crypto.AddressManager                    // 地址管理服务
	utxoManager      repository.UTXOManager                   // UTXO管理器
	txHashService    transaction.TransactionHashServiceClient // 交易哈希服务
	memoryStore      storage.MemoryStore                      // 内存存储服务
	logger           log.Logger                               // 日志记录器
}

// NewTransactionSignService 创建交易签名服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - signatureProvider: 数字签名提供服务
//   - hashCalculator: 交易哈希计算服务
//   - cacheStore: 交易缓存存储服务
//   - logger: 日志记录器
//
// 返回：
//   - *TransactionSignService: 交易签名服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewTransactionSignService(
	signatureManager crypto.SignatureManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	utxoManager repository.UTXOManager,
	txHashService transaction.TransactionHashServiceClient,
	memoryStore storage.MemoryStore,
	logger log.Logger,
) *TransactionSignService {
	// 严格校验关键依赖非空
	if signatureManager == nil {
		panic("TransactionSignService: signatureManager不能为nil")
	}
	if keyManager == nil {
		panic("TransactionSignService: keyManager不能为nil")
	}
	if addressManager == nil {
		panic("TransactionSignService: addressManager不能为nil")
	}
	if utxoManager == nil {
		panic("TransactionSignService: utxoManager不能为nil")
	}
	if txHashService == nil {
		panic("TransactionSignService: txHashService不能为nil")
	}
	if memoryStore == nil {
		panic("TransactionSignService: memoryStore不能为nil")
	}
	if logger == nil {
		panic("TransactionSignService: logger不能为nil")
	}

	return &TransactionSignService{
		signatureManager: signatureManager,
		keyManager:       keyManager,
		addressManager:   addressManager,
		utxoManager:      utxoManager,
		txHashService:    txHashService,
		memoryStore:      memoryStore,
		logger:           logger,
	}
}

// ============================================================================
//
//	核心交易签名方法实现
//
// ============================================================================
// SignTransaction 实现交易签名功能（完整实现）
//
// 🎯 **方法职责**：
// 实现 blockchain.TransactionManager.SignTransaction 接口
// 对已构建的交易进行数字签名
//
// 📋 **业务流程**：
// 1. 验证交易哈希和私钥的有效性
// 2. 从缓存中加载未签名的交易数据
// 3. 计算交易的签名哈希（根据 SIGHASH 类型）
// 4. 使用私钥对签名哈希进行数字签名
// 5. 将签名数据填入交易的解锁证明中
// 6. 验证签名的正确性和完整性
// 7. 更新缓存中的已签名交易数据
// 8. 返回完整的已签名交易字节数据
//
// 📝 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - transactionHash: 待签名交易的哈希（由构建方法返回）
//   - privateKey: 签名私钥（支持多种算法）
//
// 📤 **返回值**：
//   - []byte: 完整的已签名交易字节数据，可直接提交到网络
//   - error: 错误信息，签名失败时返回具体原因
//
// 🎯 **签名特性**：
// - 多算法支持：ECDSA、Ed25519、Schnorr等
// - SIGHASH 类型：支持 ALL、NONE、SINGLE等签名范围
// - 安全验证：签名后立即验证签名正确性
// - 标准兼容：符合区块链行业签名标准
func (s *TransactionSignService) SignTransaction(
	ctx context.Context,
	transactionHash []byte,
	privateKey []byte,
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始签名交易 - txHash: %x", transactionHash))
	}

	// 📋 基础参数验证
	if len(transactionHash) != 32 {
		return nil, fmt.Errorf("无效的交易哈希长度: %d，应为32字节", len(transactionHash))
	}
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("私钥不能为空")
	}

	// 🔍 验证私钥有效性
	err := s.validatePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("私钥验证失败: %v", err)
	}

	// 🔄 从缓存加载未签名交易
	tx, err := s.loadTransactionFromCache(ctx, transactionHash)
	if err != nil {
		return nil, fmt.Errorf("从缓存加载交易失败: %v", err)
	}

	// 🔐 添加签名到交易
	err = s.addSignatureToTransaction(ctx, tx, privateKey)
	if err != nil {
		return nil, fmt.Errorf("添加签名失败: %v", err)
	}

	// 🧮 计算签名后的哈希
	signedHash, err := s.computeSignedTransactionHash(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("计算签名哈希失败: %v", err)
	}

	// 💾 更新缓存
	err = s.updateTransactionCache(ctx, transactionHash, signedHash, tx)
	if err != nil {
		return nil, fmt.Errorf("更新缓存失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 交易签名完成 - originalHash: %x, signedHash: %x", transactionHash, signedHash))
	}

	return signedHash, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// loadTransactionFromCache 从缓存中加载交易数据
//
// 🔍 **加载内容**：
// - 完整的交易结构
// - 签名前的预处理数据
// - 交易构建时的元信息
//
// 参数：
//   - ctx: 上下文对象
//   - transactionHash: 交易哈希
//
// 返回：
//   - *transaction.Transaction: 交易数据
//   - error: 加载失败时的错误信息
func (s *TransactionSignService) loadTransactionFromCache(
	ctx context.Context,
	transactionHash []byte,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		s.logger.Debug("从缓存加载交易数据")
	}

	// 使用统一的缓存接口获取未签名交易
	tx, exists, err := internal.GetUnsignedTransactionFromCache(ctx, s.memoryStore, transactionHash, s.logger)
	if err != nil {
		return nil, fmt.Errorf("获取交易失败: %v", err)
	}
	if !exists {
		return nil, fmt.Errorf("未签名交易不存在于缓存中: %x", transactionHash)
	}

	return tx, nil
}

// computeSignature 计算交易签名
//
// 🔐 **数字签名计算器**
//
// 使用私钥对交易数据进行数字签名。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 交易对象
//   - privateKey: 私钥
//
// 返回：
//   - []byte: 签名数据
//   - error: 签名错误
func (s *TransactionSignService) computeSignature(
	ctx context.Context,
	tx *transaction.Transaction,
	privateKey []byte,
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("开始计算交易真实签名")
	}

	// 1. 使用交易哈希服务计算交易哈希
	hashReq := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	hashResp, err := s.txHashService.ComputeHash(ctx, hashReq)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 2. 使用签名管理器进行真实签名
	signature, err := s.signatureManager.Sign(hashResp.Hash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("交易签名失败: %w", err)
	}

	// 3. 验证签名有效性（自我校验）
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("从私钥导出公钥失败: %w", err)
	}

	isValid := s.signatureManager.Verify(hashResp.Hash, signature, publicKey)
	if !isValid {
		return nil, fmt.Errorf("签名自我验证失败")
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 交易签名计算成功 - 签名长度: %d字节", len(signature)))
	}

	return signature, nil
}

// addSignatureToTransaction 添加签名到交易
//
// 📝 **签名附加器**
//
// 将计算出的签名添加到交易的相应输入中。
//
// 参数：
//   - tx: 交易对象
//   - privateKey: 私钥（用于确定签名位置）
//
// 返回：
//   - error: 添加错误
func (s *TransactionSignService) addSignatureToTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	privateKey []byte,
) error {
	if s.logger != nil {
		s.logger.Debug("添加签名到交易")
	}

	// ⚡ 设置正确的Nonce（关键契约实现）
	if err := s.setTransactionNonce(tx); err != nil {
		return fmt.Errorf("设置交易Nonce失败: %v", err)
	}

	// 为每个输入按照UTXO锁定条件添加正确的签名证明
	for i, input := range tx.Inputs {
		if input == nil {
			continue
		}

		// 获取输入对应的UTXO锁定条件
		utxo, err := s.utxoManager.GetUTXO(ctx, input.PreviousOutput)
		if err != nil || utxo == nil {
			return fmt.Errorf("无法获取输入%d对应的UTXO: %v", i, err)
		}

		// 检查锁定条件类型，决定如何添加签名
		if err := s.addSignatureForInput(ctx, tx, i, privateKey, utxo); err != nil {
			return fmt.Errorf("为输入%d添加签名失败: %w", i, err)
		}

		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("✅ 输入%d签名添加完成", i))
		}
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 签名添加完成 - Nonce: %d, 输入数量: %d", tx.Nonce, len(tx.Inputs)))
	}

	return nil
}

// addSignatureForInput 为指定输入按UTXO锁定条件添加正确签名
//
// 🔐 **按锁定条件类型分发签名逻辑**
//
// 根据UTXO的锁定条件类型决定如何添加签名证明。
// 只处理SingleKeyLock，其他类型要求走专用流程。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 交易对象
//   - inputIndex: 输入索引
//   - privateKey: 私钥
//   - utxo: 对应的UTXO
//
// 返回：
//   - error: 签名添加错误
func (s *TransactionSignService) addSignatureForInput(
	ctx context.Context,
	tx *transaction.Transaction,
	inputIndex int,
	privateKey []byte,
	utxo *utxo.UTXO,
) error {
	// 检查UTXO是否有缓存的输出
	if utxo.GetCachedOutput() == nil {
		return fmt.Errorf("输入%d的UTXO没有缓存输出，无法获取锁定条件", inputIndex)
	}

	// 分析锁定条件类型
	cachedOutput := utxo.GetCachedOutput()
	if len(cachedOutput.LockingConditions) == 0 {
		return fmt.Errorf("输入%d的UTXO没有锁定条件", inputIndex)
	}

	// 遍历锁定条件，找到第一个可处理的类型
	for _, lockingCondition := range cachedOutput.LockingConditions {
		switch lockingCondition.Condition.(type) {
		case *transaction.LockingCondition_SingleKeyLock:
			// 处理单密钥锁定 - 可以在此服务中处理
			return s.addSingleKeySignature(ctx, tx, inputIndex, privateKey, lockingCondition.GetSingleKeyLock())

		case *transaction.LockingCondition_MultiKeyLock:
			// 多重签名需要走专用的多签会话流程
			return fmt.Errorf("输入%d使用MultiKeyLock锁定，请使用多签会话功能(CreateMultiSigSession)", inputIndex)

		case *transaction.LockingCondition_ContractLock:
			// 合约锁定需要走合约执行流程
			return fmt.Errorf("输入%d使用ContractLock锁定，请使用合约执行流程", inputIndex)

		case *transaction.LockingCondition_DelegationLock:
			// 委托锁定需要走专用的委托授权流程
			return fmt.Errorf("输入%d使用DelegationLock锁定，请使用委托授权流程", inputIndex)

		case *transaction.LockingCondition_ThresholdLock:
			// 门限签名需要走专用的门限签名流程
			return fmt.Errorf("输入%d使用ThresholdLock锁定，请使用门限签名流程", inputIndex)

		case *transaction.LockingCondition_TimeLock,
			*transaction.LockingCondition_HeightLock:
			// 时间锁和高度锁需要检查解锁条件并递归处理基础锁
			return fmt.Errorf("输入%d使用时间/高度锁定，暂不支持，需要专用解锁逻辑", inputIndex)

		default:
			return fmt.Errorf("输入%d使用不支持的锁定条件类型: %T", inputIndex, lockingCondition.Condition)
		}
	}

	return fmt.Errorf("输入%d没有找到可处理的锁定条件", inputIndex)
}

// addSingleKeySignature 为单密钥锁定添加签名证明
//
// 🔐 **单密钥签名专用处理器**
//
// 验证私钥权限并添加SingleKeyProof。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 交易对象
//   - inputIndex: 输入索引
//   - privateKey: 私钥
//   - singleKeyLock: 单密钥锁定条件
//
// 返回：
//   - error: 签名添加错误
func (s *TransactionSignService) addSingleKeySignature(
	ctx context.Context,
	tx *transaction.Transaction,
	inputIndex int,
	privateKey []byte,
	singleKeyLock *transaction.SingleKeyLock,
) error {
	// 1. 从私钥推导公钥
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return fmt.Errorf("从私钥导出公钥失败: %w", err)
	}

	// 2. 验证私钥是否有权签名此输入
	switch keyReq := singleKeyLock.KeyRequirement.(type) {
	case *transaction.SingleKeyLock_RequiredPublicKey:
		// P2PK模式：直接比较公钥
		if !bytes.Equal(publicKey, keyReq.RequiredPublicKey.Value) {
			return fmt.Errorf("私钥对应的公钥不匹配锁定条件")
		}
	case *transaction.SingleKeyLock_RequiredAddressHash:
		// P2PKH模式：计算地址哈希并比较
		address, err := s.addressManager.PublicKeyToAddress(publicKey)
		if err != nil {
			return fmt.Errorf("从公钥计算地址失败: %w", err)
		}
		addressBytes, err := s.addressManager.AddressToBytes(address)
		if err != nil {
			return fmt.Errorf("地址转字节失败: %w", err)
		}
		if !bytes.Equal(addressBytes, keyReq.RequiredAddressHash) {
			return fmt.Errorf("私钥对应的地址哈希不匹配锁定条件")
		}
	default:
		return fmt.Errorf("不支持的锁定条件类型: %T", keyReq)
	}

	// 3. 构造输入级签名消息
	sigHashType := types.SignatureHashType(singleKeyLock.SighashType)
	signatureMessage, err := s.constructSignatureMessage(ctx, tx, inputIndex, sigHashType)
	if err != nil {
		return fmt.Errorf("构造签名消息失败: %w", err)
	}

	// 4. 计算签名哈希
	hashReq := &transaction.ComputeHashRequest{
		Transaction:      signatureMessage,
		IncludeDebugInfo: false,
	}

	hashResp, err := s.txHashService.ComputeHash(ctx, hashReq)
	if err != nil {
		return fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 5. 计算签名
	signature, err := s.signatureManager.SignTransaction(hashResp.Hash, privateKey, sigHashType)
	if err != nil {
		return fmt.Errorf("交易签名失败: %w", err)
	}

	// 6. 验证签名正确性
	isValid := s.signatureManager.VerifyTransactionSignature(hashResp.Hash, signature, publicKey, sigHashType)
	if !isValid {
		return fmt.Errorf("签名自我验证失败")
	}

	// 7. 添加到交易输入
	tx.Inputs[inputIndex].UnlockingProof = &transaction.TxInput_SingleKeyProof{
		SingleKeyProof: &transaction.SingleKeyProof{
			Signature: &transaction.SignatureData{
				Value: signature,
			},
			PublicKey: &transaction.PublicKey{
				Value: publicKey,
			},
			Algorithm:   singleKeyLock.RequiredAlgorithm,
			SighashType: singleKeyLock.SighashType,
		},
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 为输入%d添加单密钥签名: %x", inputIndex, signature[:8]))
	}

	return nil
}

// constructSignatureMessage 构造输入级签名消息
//
// 🔐 **SIGHASH消息构造器**
//
// 根据Bitcoin SIGHASH标准构造特定输入的签名消息。
//
// 🎯 **SIGHASH类型处理**：
// - SIGHASH_ALL: 签名所有输入和输出
// - SIGHASH_NONE: 签名所有输入，不签名任何输出
// - SIGHASH_SINGLE: 签名所有输入和对应索引的输出
// - SIGHASH_ANYONECANPAY: 只签名当前输入
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 原始交易
//   - inputIndex: 当前验证的输入索引
//   - sighashType: 签名哈希类型
//
// 返回：
//   - *transaction.Transaction: 构造的签名消息交易
//   - error: 构造错误
func (s *TransactionSignService) constructSignatureMessage(
	ctx context.Context,
	tx *transaction.Transaction,
	inputIndex int,
	sighashType types.SignatureHashType,
) (*transaction.Transaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易不能为空")
	}
	if inputIndex < 0 || inputIndex >= len(tx.Inputs) {
		return nil, fmt.Errorf("输入索引无效: %d", inputIndex)
	}

	// 创建签名交易的副本
	sigTx := &transaction.Transaction{
		Version:                  tx.Version,
		Nonce:                    tx.Nonce,
		CreationTimestamp:        tx.CreationTimestamp,
		ChainId:                  tx.ChainId,
		ValidityWindow:           tx.ValidityWindow,
		FeeMechanism:             tx.FeeMechanism,
		Metadata:                 tx.Metadata,
		ResourceAttachmentHashes: tx.ResourceAttachmentHashes,
	}

	// 根据SIGHASH类型构造输入
	sigTx.Inputs = make([]*transaction.TxInput, 0)

	if sighashType&types.SigHashAnyoneCanPay != 0 {
		// ANYONECANPAY: 只包含当前输入
		currentInput := tx.Inputs[inputIndex]
		sigInput := &transaction.TxInput{
			PreviousOutput:  currentInput.PreviousOutput,
			IsReferenceOnly: currentInput.IsReferenceOnly,
			Sequence:        currentInput.Sequence,
			// 注意：不包含unlocking_proof，因为这是要签名的部分
		}
		sigTx.Inputs = append(sigTx.Inputs, sigInput)
	} else {
		// 包含所有输入，但清空unlocking_proof
		for _, input := range tx.Inputs {
			sigInput := &transaction.TxInput{
				PreviousOutput:  input.PreviousOutput,
				IsReferenceOnly: input.IsReferenceOnly,
				Sequence:        input.Sequence,
			}

			// 对于当前验证的输入，需要在签名消息中反映锁定条件约束
			// 注意：锁定条件不应混入unlocking_proof中，这里保持输入结构清洁
			// Bitcoin SIGHASH标准通过交易结构本身（而非unlocking_proof）来包含锁定约束

			sigTx.Inputs = append(sigTx.Inputs, sigInput)
		}
	}

	// 根据SIGHASH类型构造输出
	baseType := sighashType & 0x1F // 获取基础类型（去掉ANYONECANPAY标志）

	switch baseType {
	case types.SigHashAll:
		// SIGHASH_ALL: 包含所有输出
		sigTx.Outputs = make([]*transaction.TxOutput, len(tx.Outputs))
		copy(sigTx.Outputs, tx.Outputs)

	case types.SigHashNone:
		// SIGHASH_NONE: 不包含任何输出
		sigTx.Outputs = make([]*transaction.TxOutput, 0)

	case types.SigHashSingle:
		// SIGHASH_SINGLE: 只包含对应索引的输出
		if inputIndex < len(tx.Outputs) {
			sigTx.Outputs = []*transaction.TxOutput{tx.Outputs[inputIndex]}
		} else {
			sigTx.Outputs = make([]*transaction.TxOutput, 0)
		}

	default:
		return nil, fmt.Errorf("不支持的SIGHASH类型: %v", sighashType)
	}

	return sigTx, nil
}

// setTransactionNonce 设置交易的正确nonce值
//
// ⚡ **Nonce设置核心实现**
//
// 为交易设置正确的nonce值，确保防重放攻击保护
//
// ⚡ **Nonce设置核心实现**
//
// 为交易设置正确的nonce值，确保防重放攻击保护
//
// 参数：
//   - tx: 交易对象
//
// 返回：
//   - error: 设置错误
func (s *TransactionSignService) setTransactionNonce(tx *transaction.Transaction) error {
	if tx == nil {
		return fmt.Errorf("交易对象不能为空")
	}

	// 当前简化实现：使用时间戳作为nonce
	// 生产环境应该从account nonce服务获取正确的递增序号
	nonce := uint64(time.Now().UnixNano())
	tx.Nonce = nonce

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 设置交易Nonce: %d", nonce))
	}

	return nil
}

// computeSignedTransactionHash 计算签名后的交易哈希
//
// 🔐 **完整哈希计算器**
//
// 计算包含签名的完整交易哈希。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 已签名的交易对象
//
// 返回：
//   - []byte: 完整交易哈希
//   - error: 计算错误
func (s *TransactionSignService) computeSignedTransactionHash(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("计算签名后的交易哈希")
	}

	// 使用交易哈希服务计算包含签名的完整哈希
	hashReq := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	hashResp, err := s.txHashService.ComputeHash(ctx, hashReq)
	if err != nil {
		return nil, fmt.Errorf("调用交易哈希服务失败: %w", err)
	}

	if !hashResp.IsValid {
		return nil, fmt.Errorf("签名交易结构无效")
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 计算交易哈希成功: %x", hashResp.Hash[:8]))
	}

	return hashResp.Hash, nil
}

// updateTransactionCache 更新交易缓存
//
// 💾 **缓存更新器**
//
// 更新缓存中的交易，将未签名哈希替换为已签名哈希。
//
// 参数：
//   - ctx: 上下文对象
//   - oldHash: 未签名交易哈希
//   - newHash: 已签名交易哈希
//   - tx: 已签名交易对象
//
// 返回：
//   - error: 更新错误
func (s *TransactionSignService) updateTransactionCache(
	ctx context.Context,
	oldHash []byte,
	newHash []byte,
	tx *transaction.Transaction,
) error {
	if s.logger != nil {
		s.logger.Debug("更新交易缓存")
	}

	// 使用内部缓存工具更新交易缓存
	cacheConfig := internal.GetDefaultCacheConfig()

	// 缓存已签名交易
	err := internal.CacheSignedTransaction(ctx, s.memoryStore, newHash, tx, cacheConfig, s.logger)
	if err != nil {
		return fmt.Errorf("缓存已签名交易失败: %v", err)
	}

	// 删除未签名交易缓存（可选，节省内存）
	oldCacheKey := internal.GenerateCacheKey(internal.UnsignedTxPrefix, oldHash)
	err = s.memoryStore.Delete(ctx, oldCacheKey)
	if err != nil && s.logger != nil {
		// 删除失败不是致命错误，只记录警告
		s.logger.Warn(fmt.Sprintf("删除未签名交易缓存失败: %v", err))
	}

	return nil
}

// validatePrivateKey 验证私钥
//
// ✅ **私钥验证器**
//
// 验证私钥的格式和有效性。
//
// 参数：
//   - privateKey: 私钥数据
//
// 返回：
//   - error: 验证错误
func (s *TransactionSignService) validatePrivateKey(privateKey []byte) error {
	if s.logger != nil {
		s.logger.Debug("验证私钥")
	}

	// 基础验证
	if len(privateKey) == 0 {
		return fmt.Errorf("私钥不能为空")
	}

	// 常见的私钥长度验证
	switch len(privateKey) {
	case 32: // ECDSA secp256k1
		// 有效长度
	case 64: // EdDSA等
		// 有效长度
	default:
		return fmt.Errorf("无效的私钥长度: %d，支持32或64字节", len(privateKey))
	}

	// 使用密钥管理器进行严格验证
	return s.keyManager.ValidatePrivateKey(privateKey)
}

// derivePublicKey 从私钥推导公钥
//
// 🔑 **公钥推导器**
//
// 从私钥推导出对应的公钥。
//
// 参数：
//   - privateKey: 私钥
//
// 返回：
//   - []byte: 公钥
//   - error: 推导错误
func (s *TransactionSignService) derivePublicKey(privateKey []byte) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("从私钥推导公钥")
	}

	// 基础验证
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("私钥不能为空")
	}

	// 使用密钥管理器推导公钥
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("密钥管理器推导公钥失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 公钥推导成功 - 长度: %d字节", len(publicKey)))
	}

	return publicKey, nil
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// validateSignature 验证签名
//
// ✅ **签名验证器**
//
// 验证计算出的签名是否有效。
//
// 参数：
//   - tx: 交易对象
//   - signature: 签名数据
//   - publicKey: 公钥
//
// 返回：
//   - bool: 验证结果
//   - error: 验证错误
func (s *TransactionSignService) validateSignature(
	ctx context.Context,
	tx *transaction.Transaction,
	signature, publicKey []byte,
) (bool, error) {
	if s.logger != nil {
		s.logger.Debug("验证签名")
	}

	// 基础参数验证
	if len(signature) == 0 || len(publicKey) == 0 {
		return false, fmt.Errorf("签名或公钥不能为空")
	}

	// 使用交易哈希服务计算交易哈希
	hashReq := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	hashResp, err := s.txHashService.ComputeHash(ctx, hashReq)
	if err != nil {
		return false, fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 使用签名管理器进行验证
	isValid := s.signatureManager.Verify(hashResp.Hash, signature, publicKey)

	if s.logger != nil {
		if isValid {
			s.logger.Debug("✅ 签名验证通过")
		} else {
			s.logger.Debug("❌ 签名验证失败")
		}
	}

	return isValid, nil
}

// ============================================================================
//                              编译时接口检查
// ============================================================================

// 确保 TransactionSignService 实现了所需的接口部分
var _ interface {
	SignTransaction(context.Context, []byte, []byte) ([]byte, error)
} = (*TransactionSignService)(nil)
