// Package interfaces 定义区块链内部接口
package interfaces

import (
	blockchain "github.com/weisyn/v1/pkg/interfaces/blockchain"
)

// InternalAccountService 内部账户服务接口
//
// 🎯 设计理念: 继承公共AccountService接口，确保实现完整性
// 📋 当前功能: 仅作为类型约束，不添加额外方法
// 🔮 未来扩展: 为将来可能的内部方法扩展预留接口
type InternalAccountService interface {
	blockchain.AccountService // 继承所有公共账户服务方法
}
