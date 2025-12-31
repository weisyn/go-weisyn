package candidate_collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	consensustestutil "github.com/weisyn/v1/internal/core/consensus/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

func TestValidateMinBlockInterval_RejectsTooEarlyCandidateAtTipPlusOne(t *testing.T) {
	qs := consensustestutil.NewMockQueryService()
	now := time.Now().Unix()
	// tip = 10, parentTS 足够接近当前时间，避免被“过于陈旧/超前”规则拒绝
	parentTS := now - 120
	qs.SetBlock([]byte{1}, &core.Block{
		Header: &core.BlockHeader{
			Height:    10,
			Timestamp: uint64(parentTS),
		},
	})

	v := &candidateValidator{
		query:                   qs,
		minBlockIntervalSeconds: 30,
	}

	candidate := &types.CandidateBlock{
		Height: 11, // tip+1
		Block: &core.Block{
			Header: &core.BlockHeader{
				Height:    11,
				Timestamp: uint64(parentTS + 29), // < parentTS+30
			},
			Body: &core.BlockBody{},
		},
	}

	err := v.validateMinBlockInterval(candidate)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "候选区块过早（min_block_interval）")
}

func TestValidateMinBlockInterval_AllowsCandidateAtOrAfterMinAllowed(t *testing.T) {
	qs := consensustestutil.NewMockQueryService()
	now := time.Now().Unix()
	parentTS := now - 120
	qs.SetBlock([]byte{1}, &core.Block{
		Header: &core.BlockHeader{
			Height:    10,
			Timestamp: uint64(parentTS),
		},
	})

	v := &candidateValidator{
		query:                   qs,
		minBlockIntervalSeconds: 30,
	}

	candidate := &types.CandidateBlock{
		Height: 11, // tip+1
		Block: &core.Block{
			Header: &core.BlockHeader{
				Height:    11,
				Timestamp: uint64(parentTS + 30), // == parentTS+30
			},
			Body: &core.BlockBody{},
		},
	}

	require.NoError(t, v.validateMinBlockInterval(candidate))
}
