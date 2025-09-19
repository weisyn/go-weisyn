package host

import (
	"fmt"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
)

// UTXOProvider UTXO能力提供者
//
// # 核心功能：
// - UTXO资产管理：创建输出、转移资产、余额查询
// - 地址校验：确保UTXO操作使用有效的地址格式
// - 幂等性保证：防止重复操作导致的资产异常
// - 批量操作：支持高效的批量UTXO操作处理
//
// # 安全特性：
// - 余额检查：转移前验证发送方余额充足
// - 地址验证：确保所有地址符合区块链地址规范
// - 幂等控制：通过操作ID防止重复执行
// - 参数校验：严格检查所有操作参数的有效性
//
// # 设计目标：
// - 安全第一：多层安全检查，防止资产损失
// - 性能优化：批量操作和高效的余额查询
// - 易于集成：标准的函数注入模式
// - 可扩展：支持未来的UTXO操作类型扩展
//
// # 使用场景：
// - 智能合约中的资产转移操作
// - AI模型触发的自动化支付
// - 多签钱包的资产管理
// - 去中心化金融(DeFi)应用
type UTXOProvider struct {
	// ==================== 核心UTXO操作 ====================

	// createOutput UTXO输出创建函数
	// 为指定接收者创建新的UTXO输出
	createOutput func(recipient []byte, amount uint64) error

	// transfer 资产转移函数
	// 执行从发送方到接收方的资产转移
	transfer func(from, to []byte, amount uint64) error

	// queryBalance 余额查询函数
	// 查询指定地址的可用余额（聚合所有UTXO）
	queryBalance func(address []byte) (uint64, error)

	// ==================== 安全和幂等控制 ====================

	// validateAddress 地址校验函数
	// 验证地址格式是否符合区块链规范
	validateAddress func(address []byte) (bool, error)

	// checkIdempotency 幂等性检查函数
	// 检查操作ID是否已执行过，防止重复操作
	checkIdempotency func(opID []byte) (bool, error)

	// markIdempotent 幂等标记函数
	// 将操作ID标记为已执行，用于后续幂等检查
	markIdempotent func(opID []byte) error
}

func NewUTXOProvider() *UTXOProvider { return &UTXOProvider{} }

func (p *UTXOProvider) CapabilityDomain() string { return "utxo" }

func (p *UTXOProvider) Register(r execiface.HostCapabilityRegistry) error {
	if err := r.RegisterProvider(p); err != nil {
		return fmt.Errorf("register utxo provider failed: %w", err)
	}
	return nil
}

// WithCreateOutput 注入创建UTXO输出函数
func (p *UTXOProvider) WithCreateOutput(fn func(recipient []byte, amount uint64) error) *UTXOProvider {
	p.createOutput = fn
	return p
}

// WithTransfer 注入转移函数
func (p *UTXOProvider) WithTransfer(fn func(from, to []byte, amount uint64) error) *UTXOProvider {
	p.transfer = fn
	return p
}

// WithQueryBalance 注入余额查询函数
func (p *UTXOProvider) WithQueryBalance(fn func(address []byte) (uint64, error)) *UTXOProvider {
	p.queryBalance = fn
	return p
}

// WithValidateAddress 注入地址校验函数
func (p *UTXOProvider) WithValidateAddress(fn func(address []byte) (bool, error)) *UTXOProvider {
	p.validateAddress = fn
	return p
}

// WithIdempotency 注入幂等处理函数
func (p *UTXOProvider) WithIdempotency(check func(opID []byte) (bool, error), mark func(opID []byte) error) *UTXOProvider {
	p.checkIdempotency = check
	p.markIdempotent = mark
	return p
}

// CreateUTXO 创建输出（简单场景：指定接收者与金额）
func (p *UTXOProvider) CreateUTXO(recipient []byte, amount uint64, opID []byte) error {
	if len(recipient) == 0 || amount == 0 {
		return fmt.Errorf("invalid utxo create params")
	}
	if p.createOutput == nil {
		return fmt.Errorf("utxo capability unavailable: CreateUTXO")
	}
	// 地址校验
	if p.validateAddress != nil {
		ok, err := p.validateAddress(recipient)
		if err != nil || !ok {
			if err != nil {
				return fmt.Errorf("invalid recipient address: %w", err)
			}
			return fmt.Errorf("invalid recipient address")
		}
	}
	// 幂等校验
	if len(opID) > 0 && p.checkIdempotency != nil {
		ok, err := p.checkIdempotency(opID)
		if err != nil {
			return fmt.Errorf("idempotency check failed: %w", err)
		}
		if ok {
			return nil
		}
	}
	if err := p.createOutput(recipient, amount); err != nil {
		return err
	}
	if len(opID) > 0 && p.markIdempotent != nil {
		_ = p.markIdempotent(opID)
	}
	return nil
}

// SpendUTXO 转移（from -> to）简单抽象
func (p *UTXOProvider) SpendUTXO(from, to []byte, amount uint64, opID []byte) error {
	if len(from) == 0 || len(to) == 0 || amount == 0 {
		return fmt.Errorf("invalid utxo spend params")
	}
	if p.transfer == nil {
		return fmt.Errorf("utxo capability unavailable: SpendUTXO")
	}
	// 地址校验
	if p.validateAddress != nil {
		ok, err := p.validateAddress(from)
		if err != nil || !ok {
			if err != nil {
				return fmt.Errorf("invalid from address: %w", err)
			}
			return fmt.Errorf("invalid from address")
		}
		ok, err = p.validateAddress(to)
		if err != nil || !ok {
			if err != nil {
				return fmt.Errorf("invalid to address: %w", err)
			}
			return fmt.Errorf("invalid to address")
		}
	}
	// 余额校验
	if p.queryBalance != nil {
		bal, err := p.queryBalance(from)
		if err != nil {
			return fmt.Errorf("query balance failed: %w", err)
		}
		if bal < amount {
			return fmt.Errorf("insufficient balance")
		}
	}
	// 幂等
	if len(opID) > 0 && p.checkIdempotency != nil {
		ok, err := p.checkIdempotency(opID)
		if err != nil {
			return fmt.Errorf("idempotency check failed: %w", err)
		}
		if ok {
			return nil
		}
	}
	if err := p.transfer(from, to, amount); err != nil {
		return err
	}
	if len(opID) > 0 && p.markIdempotent != nil {
		_ = p.markIdempotent(opID)
	}
	return nil
}

// UTXOOperation 批量操作定义
// opType: "create" | "transfer"

type UTXOOperation struct {
	OpType string
	From   []byte
	To     []byte
	Amount uint64
	OpID   []byte
}

// BatchUTXOOperations 批量执行（遇错收集第一个错误并继续）
func (p *UTXOProvider) BatchUTXOOperations(ops []UTXOOperation) error {
	var firstErr error
	for _, op := range ops {
		switch op.OpType {
		case "create":
			if err := p.CreateUTXO(op.To, op.Amount, op.OpID); err != nil && firstErr == nil {
				firstErr = err
			}
		case "transfer":
			if err := p.SpendUTXO(op.From, op.To, op.Amount, op.OpID); err != nil && firstErr == nil {
				firstErr = err
			}
		default:
			if firstErr == nil {
				firstErr = fmt.Errorf("unknown utxo op type: %s", op.OpType)
			}
		}
	}
	return firstErr
}
