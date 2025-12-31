package repair

import (
	"context"
	"fmt"

	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// CheckBlocksAndBadgerTip 是“blocks/ + Badger 索引”模式下的启动一致性门闸：
// - 区块原始数据应落盘到 blocks/（通过 FileStore 访问，路径形如 blocks/<segment>/<height>.bin）
// - BadgerDB 存储链尖与索引（state:chain:tip / indices:*）
//
// 本门闸的目标不是“强原子提交”（跨存储做不到），而是确保节点不会在
// “Badger 声称 tip>0 但 blocks/ 缺失”这类致命状态下继续运行。
//
// 约束（对齐 _dev）：
// - blocks/ 是区块原始数据的落点，不允许把区块数据改存入 Badger 来追求事务原子性。
func CheckBlocksAndBadgerTip(ctx context.Context, store storeiface.BadgerStore, fileStore storeiface.FileStore, logger logiface.Logger) error {
	if store == nil {
		return fmt.Errorf("store is nil")
	}
	if fileStore == nil {
		return fmt.Errorf("fileStore is nil")
	}

	// 1) 读取 tipHeight（tip 不存在视为新链/空链）
	tipData, err := store.Get(ctx, []byte("state:chain:tip"))
	var tipHeight uint64
	if err == nil && len(tipData) >= 8 {
		tipHeight = bytesToUint64(tipData[:8])
	}

	if tipHeight == 0 {
		// 空链：允许 blocks/ 为空（历史残留数据目录的检测依赖递归目录遍历，这里不强依赖 FileStore 的 ListFiles 行为）
		return nil
	}

	// tip>0：必须能读取 tip 对应区块文件（这是跨存储不一致中最致命的一类）
	if err := checkTipBlockFileExists(ctx, fileStore, tipHeight); err != nil {
		if logger != nil {
			logger.Errorf("❌ 启动门闸：Badger tip=%d 但 blocks/ 缺失对应区块文件: %v", tipHeight, err)
		}
		return err
	}

	if logger != nil {
		logger.Infof("✅ 启动门闸：tip 对应 blocks/ 区块文件存在: badger_tip=%d", tipHeight)
	}
	return nil
}

func checkTipBlockFileExists(ctx context.Context, fs storeiface.FileStore, tipHeight uint64) error {
	if tipHeight == 0 {
		return nil
	}
	seg := (tipHeight / 1000) * 1000
	p := fmt.Sprintf("blocks/%010d/%010d.bin", seg, tipHeight)
	b, err := fs.Load(ctx, p)
	if err != nil || len(b) == 0 {
		if err == nil {
			err = fmt.Errorf("empty block file")
		}
		return fmt.Errorf("tip block file missing: height=%d path=%s err=%v", tipHeight, p, err)
	}
	return nil
}
