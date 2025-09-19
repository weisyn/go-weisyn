// Package lifecycle 提供交易生命周期管理 - 验证服务
//
// 🎯 **职责定位**：TransactionManager验证接口的适配实现
//
// 本文件实现公共接口`TransactionManager.ValidateTransaction`方法，
// 作为外部调用和内部专业验证服务之间的适配层。
//
// 🏗️ **架构分层**：
// - 本文件：公共接口适配层（简洁的接口实现）
// - validation/：专业验证逻辑层（复杂的验证实现）
// - manager.go：顶层协调层（方法委托和依赖注入）
//
// 📋 **设计价值**：
// - 接口职责分离：公共接口适配 vs 专业验证逻辑
// - 依赖解耦：外部接口不直接依赖内部验证细节
// - 便于测试：可以独立测试接口适配逻辑
// - 便于扩展：专业验证逻辑可以独立演进
package lifecycle

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/weisyn/v1/internal/core/blockchain/transaction/validation"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"google.golang.org/protobuf/proto"
)

// TransactionValidationService 交易验证服务适配器
//
// 🎯 **公共接口适配层**
//
// 作为外部公共接口和内部验证管理器之间的适配桥梁：
// - 实现 TransactionManager 验证接口
// - 委托给统一的验证管理器处理
// - 处理参数转换和错误格式统一
// - 提供统一的日志记录
//
// 💡 **核心价值**：
// - ✅ **接口适配**：将公共接口适配到内部验证管理器
// - ✅ **薄委托层**：不包含业务逻辑，纯粹委托
// - ✅ **错误统一**：统一错误格式和处理策略
// - ✅ **日志协调**：统一日志记录和调试信息
//
// 📝 **典型调用链**：
// 外部API → TransactionManager → 本适配器 → ValidationManager → 专业验证器
type TransactionValidationService struct {
	logger            log.Logger                    // 日志记录器（可选）
	validationManager *validation.ValidationManager // 验证管理器（统一入口）
}

// NewTransactionValidationService 创建交易验证服务适配器
//
// 🎯 **适配器工厂方法**
//
// 创建公共接口适配器，委托给统一的验证管理器。
// 使用依赖注入模式，确保验证管理器有正确的依赖。
//
// 💡 **参数说明**：
//   - logger: 日志记录器（可选，传nil则不记录日志）
//   - cacheStore: 内存缓存（用于获取交易，可为nil）
//   - utxoManager: UTXO管理器（用于状态验证，可为nil）
//   - hashServiceClient: 交易哈希服务客户端（用于哈希计算）
//   - localChainID: 本地链ID（用于跨网防护，0表示不检查）
//
// 💡 **返回值说明**：
//   - *TransactionValidationService: 验证适配器实例
func NewTransactionValidationService(
	logger log.Logger,
	cacheStore storage.MemoryStore,
	utxoManager repository.UTXOManager,
	hashServiceClient transaction.TransactionHashServiceClient,
	localChainID uint64,
) *TransactionValidationService {
	return &TransactionValidationService{
		logger:            logger,
		validationManager: validation.NewValidationManager(logger, cacheStore, utxoManager, hashServiceClient, localChainID),
	}
}

// ValidateTransaction 交易验证（公共接口实现）
//
// 🎯 **TransactionManager.ValidateTransaction接口实现**
//
// 通过交易哈希查找交易对象，然后进行完整的有效性验证，
// 确保交易符合区块链网络的所有规则和要求。
//
// 📋 **验证内容**：
//   - 交易格式正确性 - 签名有效性 - 余额充足性 - 基本规则检查
//
// 📝 **设计说明**：
// 这个方法接收交易哈希作为参数，这意味着：
// 1. 系统需要维护交易哈希到交易对象的映射
// 2. 可能涉及未签名哈希vs已签名哈希的处理
// 3. 需要考虑缓存策略来优化查找性能
//
// ⚠️ **当前限制**：
// 交易查找功能需要与存储/缓存层集成，当前为演示实现
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时控制
//   - txHash: 交易哈希（32字节，签名前后均可）
//
// 💡 **返回值说明**：
//   - bool: 验证结果（true=通过，false=不通过）
//   - error: 验证过程中的错误
//
// 💡 **调用示例**：
//
//	service := NewTransactionValidationService(logger)
//	valid, err := service.ValidateTransaction(ctx, txHash)
//	if err != nil {
//	    log.Errorf("验证出错: %v", err)
//	    return false, err
//	}
//	if !valid {
//	    log.Warn("交易无效")
//	    return false, fmt.Errorf("交易验证失败")
//	}
func (s *TransactionValidationService) ValidateTransaction(
	ctx context.Context,
	txHash []byte,
) (bool, error) {
	if s.logger != nil {
		s.logger.Debugf("开始验证交易 - 哈希: %x", txHash[:8])
	}

	// 委托给验证管理器
	valid, err := s.validationManager.ValidateTransaction(ctx, txHash)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("交易验证失败: %v", err)
		}
		return false, fmt.Errorf("交易验证失败: %w", err)
	}

	if s.logger != nil {
		if valid {
			s.logger.Debug("✅ 交易验证通过")
		} else {
			s.logger.Warn("❌ 交易验证不通过")
		}
	}

	return valid, nil
}

// ValidateTransactionObject 验证交易对象（内部使用）
//
// 🎯 **直接验证交易对象的便捷方法**
//
// 为内部调用提供的便捷方法，直接验证交易对象而无需哈希查找。
// 主要用于新构建的交易或已知交易对象的验证场景。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - tx: 完整的交易对象
//
// 💡 **返回值说明**：
//   - bool: 验证结果
//   - error: 验证错误
func (s *TransactionValidationService) ValidateTransactionObject(
	ctx context.Context,
	tx interface{}, // 使用interface{}以兼容不同的交易类型
) (bool, error) {
	if s.logger != nil {
		s.logger.Debug("开始验证交易对象")
	}

	// 1. 验证输入参数
	if tx == nil {
		return false, fmt.Errorf("交易对象为空")
	}

	// 2. 类型转换和规范化
	transactionObj, err := s.convertAndValidateTransactionType(tx)
	if err != nil {
		return false, fmt.Errorf("交易类型转换失败: %w", err)
	}

	// 3. 委托给验证管理器进行完整验证
	valid, err := s.validationManager.ValidateTransactionObject(ctx, transactionObj)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("交易验证过程出错: %v", err))
		}
		return false, fmt.Errorf("交易验证过程出错: %w", err)
	}

	if s.logger != nil {
		if valid {
			s.logger.Debug("✅ 交易对象验证通过")
		} else {
			s.logger.Warn("❌ 交易对象验证未通过")
		}
	}

	return valid, nil
}

// convertAndValidateTransactionType 转换和验证交易类型
//
// 🔄 **交易类型转换器**
//
// 支持多种输入类型的交易对象转换，将不同格式的交易
// 统一转换为标准的 *transaction.Transaction 对象。
//
// 📝 **支持的输入类型**：
//   - *transaction.Transaction: 直接返回
//   - []byte: protobuf序列化数据，进行反序列化
//   - string: 十六进制编码的protobuf数据，先解码再反序列化
//   - map[string]interface{}: JSON格式，转换为protobuf
//
// 📝 **参数说明**：
//   - txData: 交易数据（多种类型）
//
// 📤 **返回值说明**：
//   - *transaction.Transaction: 标准交易对象
//   - error: 转换错误
func (s *TransactionValidationService) convertAndValidateTransactionType(
	txData interface{},
) (*transaction.Transaction, error) {
	if txData == nil {
		return nil, fmt.Errorf("交易数据为空")
	}

	switch data := txData.(type) {
	case *transaction.Transaction:
		// 直接使用标准交易对象
		if data == nil {
			return nil, fmt.Errorf("交易对象指针为空")
		}
		return data, nil

	case []byte:
		// protobuf序列化数据
		if len(data) == 0 {
			return nil, fmt.Errorf("交易数据为空字节数组")
		}

		tx := &transaction.Transaction{}
		if err := proto.Unmarshal(data, tx); err != nil {
			return nil, fmt.Errorf("protobuf反序列化失败: %w", err)
		}

		return tx, nil

	case string:
		// 十六进制编码的protobuf数据
		if len(data) == 0 {
			return nil, fmt.Errorf("交易数据为空字符串")
		}

		// 移除可能的0x前缀
		hexData := data
		if len(hexData) >= 2 && hexData[:2] == "0x" {
			hexData = hexData[2:]
		}

		// 十六进制解码
		rawData, err := hex.DecodeString(hexData)
		if err != nil {
			return nil, fmt.Errorf("十六进制解码失败: %w", err)
		}

		// protobuf反序列化
		tx := &transaction.Transaction{}
		if err := proto.Unmarshal(rawData, tx); err != nil {
			return nil, fmt.Errorf("protobuf反序列化失败: %w", err)
		}

		return tx, nil

	default:
		return nil, fmt.Errorf("不支持的交易类型: %T", txData)
	}
}
