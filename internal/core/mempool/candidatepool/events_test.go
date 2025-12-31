package candidatepool

import (
	"testing"
	"time"

	"github.com/weisyn/v1/pkg/types"
	"github.com/stretchr/testify/assert"
)

type mockCandiSink struct {
	added   []*types.CandidateBlock
	removed []struct {
		c      *types.CandidateBlock
		reason string
	}
	expired   []*types.CandidateBlock
	cleared   []int
	cleanedUp int
}

func (m *mockCandiSink) OnCandidateAdded(c *types.CandidateBlock) { m.added = append(m.added, c) }
func (m *mockCandiSink) OnCandidateRemoved(c *types.CandidateBlock, reason string) {
	m.removed = append(m.removed, struct {
		c      *types.CandidateBlock
		reason string
	}{c, reason})
}
func (m *mockCandiSink) OnCandidateExpired(c *types.CandidateBlock) { m.expired = append(m.expired, c) }
func (m *mockCandiSink) OnPoolCleared(count int)                    { m.cleared = append(m.cleared, count) }
func (m *mockCandiSink) OnCleanupCompleted()                        { m.cleanedUp++ }

func TestCandidateEventSink_IntegratesWithCandidatePool(t *testing.T) {
	// 构造最小候选区块
	cb := &types.CandidateBlock{BlockHash: []byte("b1"), Height: 1, ReceivedAt: time.Now(), EstimatedSize: 100}

	// 桩事件下沉
	sink := &mockCandiSink{}

	// 构造最小 CandidatePool（只注入事件下沉）
	p := &CandidatePool{eventSink: sink}

	// 触发 Added
	p.eventSink.OnCandidateAdded(cb)
	assert.Len(t, sink.added, 1)
	assert.Equal(t, cb, sink.added[0])

	// 触发 Removed
	p.eventSink.OnCandidateRemoved(cb, "test")
	assert.Len(t, sink.removed, 1)
	assert.Equal(t, cb, sink.removed[0].c)
	assert.Equal(t, "test", sink.removed[0].reason)

	// 触发 Expired
	p.eventSink.OnCandidateExpired(cb)
	assert.Len(t, sink.expired, 1)
	assert.Equal(t, cb, sink.expired[0])

	// 触发 PoolCleared
	p.eventSink.OnPoolCleared(3)
	assert.Len(t, sink.cleared, 1)
	assert.Equal(t, 3, sink.cleared[0])

	// 触发 CleanupCompleted
	p.eventSink.OnCleanupCompleted()
	assert.Equal(t, 1, sink.cleanedUp)
}
