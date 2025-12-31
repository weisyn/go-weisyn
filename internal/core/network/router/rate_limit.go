// Package router provides rate limiting functionality for network routing.
package router

// rate_limit.go
// 能力：
// - 按协议/节点进行限速/配额判定，返回通过/拒绝与剩余配额
// 依赖/公共接口：
// - 若存在统一限流接口/库（如令牌桶），应作为策略实现引入，不自造轮子
// 目的：
// - 为 RouterService.CheckRateLimit 提供内部实现，不暴露指标
