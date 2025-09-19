// multi_path_distribute.go
// PubSub广播分发实现
//
// 核心业务功能：
// 1. 使用Network.Publish进行全网广播
// 2. 发布到标准的TopicConsensusResult主题
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package result_distributor

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	consensuspb "github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// pubsubDistributor PubSub分发器
type pubsubDistributor struct {
	logger  log.Logger
	network network.Network
}

// newPubsubDistributor 创建PubSub分发器
func newPubsubDistributor(logger log.Logger, network network.Network) *pubsubDistributor {
	return &pubsubDistributor{
		logger:  logger,
		network: network,
	}
}

// publishConsensusResult 发布共识结果到全网
func (d *pubsubDistributor) publishConsensusResult(
	ctx context.Context,
	broadcast *consensuspb.ConsensusResultBroadcast,
) error {
	// 验证消息
	if broadcast == nil {
		return errors.New("consensus result broadcast is nil")
	}

	// 发布门控：检查是否需要等待最小区块间隔
	if err := d.checkPublishGate(broadcast); err != nil {
		return err
	}

	// 序列化protobuf消息
	data, err := proto.Marshal(broadcast)
	if err != nil {
		return err
	}

	// 使用Network.Publish发布到标准主题
	opts := &types.PublishOptions{
		Topic:   protocols.TopicConsensusResult,
		Timeout: 30 * time.Second, // 30秒超时
	}

	err = d.network.Publish(ctx, protocols.TopicConsensusResult, data, opts)
	if err != nil {
		return err
	}

	d.logger.Info("共识结果已成功发布到全网")
	return nil
}

// checkPublishGate 发布门控：基本验证（不基于时间戳等待）
//
// ⚠️ 重要设计原则：
// - 聚合器通过固定收集窗口控制分发频率
// - 不基于区块时间戳进行等待（时间戳必须保持真实性）
// - 只进行基本的消息完整性验证
func (d *pubsubDistributor) checkPublishGate(broadcast *consensuspb.ConsensusResultBroadcast) error {
	if broadcast.FinalBlock == nil || broadcast.FinalBlock.Header == nil {
		return errors.New("invalid broadcast: missing block or header")
	}

	// 基本的时间戳合理性检查（防止明显的时间戳攻击）
	blockTimestamp := time.Unix(int64(broadcast.FinalBlock.Header.Timestamp), 0)
	now := time.Now()

	// 检查区块时间戳不能过于超前（2分钟容错）
	if blockTimestamp.After(now.Add(2 * time.Minute)) {
		return errors.New("区块时间戳过于超前，拒绝发布")
	}

	// 检查区块时间戳不能过于陈旧（10分钟容错）
	if blockTimestamp.Before(now.Add(-10 * time.Minute)) {
		return errors.New("区块时间戳过于陈旧，拒绝发布")
	}

	d.logger.Debugf("发布门控检查通过，区块时间戳: %v", blockTimestamp.Format("15:04:05"))
	return nil
}
