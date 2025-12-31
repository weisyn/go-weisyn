// Package utils provides locking condition utility functions.
package utils

import (
	"encoding/json"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/encoding/protojson"
)

// EncodeLockingConditions 将锁定条件数组编码为 bytes
//
// 🎯 **用途**：Host ABI 锁定条件入参的编码协议
//
// 📋 **实现策略**：
//   - 使用 pkg/types.LockingConditionListDTO（Host ABI 专用 DTO）
//   - 采用 JSON 编码（protojson 序列化每个条件）
//   - 与共识层 proto 隔离，避免污染协议定义
//
// ⚠️ **未来优化**：
//   - 可在独立的 pb/hostabi/ 中定义 LockingConditionList proto
//   - 切换为 protobuf 序列化（性能更优）
//
// 参数:
//   - conditions: 锁定条件数组
//
// 返回:
//   - []byte: 编码后的字节数组
//   - error: 编码错误
func EncodeLockingConditions(conditions []*pb.LockingCondition) ([]byte, error) {
	if conditions == nil {
		return nil, nil
	}

	// 使用 Host ABI DTO 承载数据
	dto := &types.LockingConditionListDTO{
		Conditions: conditions,
	}

	// 采用 protojson 编码每个条件，再用 JSON 数组承载
	var jsonConditions []json.RawMessage
	for _, cond := range dto.Conditions {
		data, err := protojson.Marshal(cond)
		if err != nil {
			return nil, fmt.Errorf("编码锁定条件失败: %w", err)
		}
		jsonConditions = append(jsonConditions, data)
	}

	// 将整个数组编码为 JSON
	result, err := json.Marshal(jsonConditions)
	if err != nil {
		return nil, fmt.Errorf("编码锁定条件数组失败: %w", err)
	}

	return result, nil
}

// DecodeLockingConditions 将 bytes 解码为锁定条件数组
//
// 🎯 **用途**：Host ABI 锁定条件入参的解码协议
//
// 📋 **实现策略**：
//   - 使用 pkg/types.LockingConditionListDTO（Host ABI 专用 DTO）
//   - 采用 JSON 解码（protojson 反序列化每个条件）
//   - 与共识层 proto 隔离
//
// ⚠️ **未来优化**：
//   - 可在独立的 pb/hostabi/ 中定义 LockingConditionList proto
//   - 切换为 protobuf 反序列化
//
// 参数:
//   - data: 编码后的字节数组
//
// 返回:
//   - []*pb.LockingCondition: 解码后的锁定条件数组
//   - error: 解码错误
func DecodeLockingConditions(data []byte) ([]*pb.LockingCondition, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// 采用 protojson 解码每个条件
	var jsonConditions []json.RawMessage
	if err := json.Unmarshal(data, &jsonConditions); err != nil {
		return nil, fmt.Errorf("解码锁定条件数组失败: %w", err)
	}

	dto := &types.LockingConditionListDTO{
		Conditions: make([]*pb.LockingCondition, 0, len(jsonConditions)),
	}

	for _, jsonData := range jsonConditions {
		cond := &pb.LockingCondition{}
		if err := protojson.Unmarshal(jsonData, cond); err != nil {
			return nil, fmt.Errorf("解码锁定条件失败: %w", err)
		}
		dto.Conditions = append(dto.Conditions, cond)
	}

	return dto.Conditions, nil
}
