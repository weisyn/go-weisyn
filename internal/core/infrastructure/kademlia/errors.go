// Package kbucket provides error definitions for Kademlia operations.
package kbucket

import "errors"

// 配置相关错误
var (
	ErrInvalidConfig      = errors.New("invalid kbucket config")
	ErrInvalidBucketSize  = errors.New("invalid bucket size")
	ErrInvalidMaxLatency  = errors.New("invalid max latency")
	ErrInvalidGracePeriod = errors.New("invalid grace period")
)

// 操作相关错误
var (
	ErrPeerNotFound      = errors.New("peer not found")
	ErrBucketFull        = errors.New("bucket is full")
	ErrManagerNotRunning = errors.New("manager is not running")
	ErrDuplicatePeer     = errors.New("duplicate peer")
)

// 网络相关错误
var (
	ErrNetworkTimeout   = errors.New("network timeout")
	ErrConnectionFailed = errors.New("connection failed")
	ErrInvalidAddress   = errors.New("invalid address")
)

// 多样性过滤相关错误
var (
	ErrDiversityFilterFailed = errors.New("diversity filter rejected peer")
	ErrIPLimitExceeded       = errors.New("IP address limit exceeded")
	ErrASNLimitExceeded      = errors.New("ASN limit exceeded")
)
