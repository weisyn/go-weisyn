package fork

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weisyn/v1/pkg/types"
)

func TestShouldSwitchChain_TieBreakByTipHash(t *testing.T) {
	s := &Service{}

	main := &types.ChainWeight{
		CumulativeDifficulty: big.NewInt(100),
		BlockCount:           10,
		TipHash:              []byte{0x02},
		LastBlockTime:        1, // 即使更早，也不应影响 tie-break
	}
	fork := &types.ChainWeight{
		CumulativeDifficulty: big.NewInt(100),
		BlockCount:           10,
		TipHash:              []byte{0x01}, // 更小 => 应胜出
		LastBlockTime:        999,
	}

	assert.True(t, s.shouldSwitchChain(main, fork))
}

func TestShouldSwitchChain_FallbackToTimeWhenTipHashMissing(t *testing.T) {
	s := &Service{}

	main := &types.ChainWeight{
		CumulativeDifficulty: big.NewInt(100),
		BlockCount:           10,
		TipHash:              nil,
		LastBlockTime:        100,
	}
	fork := &types.ChainWeight{
		CumulativeDifficulty: big.NewInt(100),
		BlockCount:           10,
		TipHash:              nil,
		LastBlockTime:        99, // 更早 => 旧规则下胜出
	}

	assert.True(t, s.shouldSwitchChain(main, fork))
}
