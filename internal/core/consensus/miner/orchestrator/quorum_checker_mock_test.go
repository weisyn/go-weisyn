package orchestrator_test

import (
	"context"

	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
)

type allowAllQuorumChecker struct{}

func (c *allowAllQuorumChecker) Check(ctx context.Context) (*quorum.Result, error) {
	return &quorum.Result{
		AllowMining: true,
		Reason:      "allow_all_for_test",
	}, nil
}


