// Package guides 提供新用户引导功能
package guides

import (
	"context"

	"github.com/weisyn/v1/internal/cli/commands"
	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/internal/cli/ui"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// GuideStep 引导步骤
type GuideStep struct {
	ID          int
	Title       string
	Description string
	Action      func(ctx context.Context) error
	IsCompleted bool
}

// GuideProgress 引导进度
type GuideProgress struct {
	TotalSteps     int
	CompletedSteps int
	CurrentStep    int
	Steps          []*GuideStep
}

// FirstTimeGuide 首次使用引导接口
type FirstTimeGuide interface {
	// CheckAndRunFirstTimeSetup 检查并运行首次设置
	CheckAndRunFirstTimeSetup(ctx context.Context) (bool, error)

	// RunFullGuide 运行完整的4步引导流程
	RunFullGuide(ctx context.Context) error

	// GetProgress 获取引导进度
	GetProgress() *GuideProgress

	// IsCompleted 检查引导是否完成
	IsCompleted() bool

	// ResetGuide 重置引导状态
	ResetGuide(ctx context.Context) error
}

// firstTimeGuide 首次使用引导实现
type firstTimeGuide struct {
	logger            log.Logger
	accountCmd        *commands.AccountCommands
	transferCmd       *commands.TransferCommands
	miningCmd         *commands.MiningCommands
	blockchainCmd     *commands.BlockchainCommands
	accountService    blockchainintf.AccountService
	permissionManager *permissions.Manager
	ui                ui.Components
	progress          *GuideProgress
}

// NewFirstTimeGuide 创建首次使用引导
func NewFirstTimeGuide(
	logger log.Logger,
	accountCmd *commands.AccountCommands,
	transferCmd *commands.TransferCommands,
	miningCmd *commands.MiningCommands,
	blockchainCmd *commands.BlockchainCommands,
	accountService blockchainintf.AccountService,
	permissionManager *permissions.Manager,
	uiComponents ui.Components,
) FirstTimeGuide {
	guide := &firstTimeGuide{
		logger:            logger,
		accountCmd:        accountCmd,
		transferCmd:       transferCmd,
		miningCmd:         miningCmd,
		blockchainCmd:     blockchainCmd,
		accountService:    accountService,
		permissionManager: permissionManager,
		ui:                uiComponents,
	}

	guide.initializeSteps()
	return guide
}
