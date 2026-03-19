package metrics

import (
	"testing"

	"github.com/shutter-network/observer/internal/data"
)

func TestClassifyInclusion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		expectedIndex int
		receiptIndex  uint
		wantStatus    data.TxStatusVal
		wantPos       string
	}{
		{"exact", 2, 2, data.TxStatusValShieldedinclusion, InclPosExact},
		{"later", 1, 3, data.TxStatusValUnshieldedinclusion, InclPosLater},
		{"earlier", 4, 1, data.TxStatusValTentativeshieldedinclusion, InclPosEarlier},
	}
	for _, c := range cases {
		gotStatus, gotPos := classifyInclusion(c.expectedIndex, c.receiptIndex)
		if gotStatus != c.wantStatus || gotPos != c.wantPos {
			t.Fatalf("%s: got (%v,%s), want (%v,%s)", c.name, gotStatus, gotPos, c.wantStatus,
				c.wantPos)
		}
	}
}

func TestClassifyWithPredecessors(t *testing.T) {
	t.Parallel()

	makeEntries := func(statuses ...data.TxStatusVal) []batchEntry {
		out := make([]batchEntry, len(statuses))
		for i, st := range statuses {
			out[i] = batchEntry{status: st}
		}
		return out
	}

	cases := []struct {
		name        string
		expectedPos int
		blockPos    int
		entries     []batchEntry
		wantStatus  data.TxStatusVal
		wantPos     string
	}{
		{
			name:        "pos0-exact-shielded",
			expectedPos: 0, blockPos: 0,
			entries:    makeEntries(data.TxStatusValPending),
			wantStatus: data.TxStatusValShieldedinclusion, wantPos: InclPosExact,
		},
		{
			name:        "pos0-later-unshielded",
			expectedPos: 0, blockPos: 1,
			entries:    makeEntries(data.TxStatusValPending),
			wantStatus: data.TxStatusValUnshieldedinclusion, wantPos: InclPosLater,
		},
		{
			name:        "exact-shielded",
			expectedPos: 1, blockPos: 1,
			entries:    makeEntries(data.TxStatusValShieldedinclusion, data.TxStatusValPending),
			wantStatus: data.TxStatusValShieldedinclusion, wantPos: InclPosExact,
		},
		{
			name:        "later-unshielded",
			expectedPos: 0, blockPos: 1,
			entries:    makeEntries(data.TxStatusValPending),
			wantStatus: data.TxStatusValUnshieldedinclusion, wantPos: InclPosLater,
		},
		{
			name:        "exact-tentative-predecessor",
			expectedPos: 1, blockPos: 1,
			entries:    makeEntries(data.TxStatusValTentativeshieldedinclusion, data.TxStatusValPending),
			wantStatus: data.TxStatusValTentativeshieldedinclusion, wantPos: InclPosExact,
		},
		{
			name:        "earlier-tentative",
			expectedPos: 2, blockPos: 1,
			entries:    makeEntries(data.TxStatusValShieldedinclusion, data.TxStatusValPending, data.TxStatusValPending),
			wantStatus: data.TxStatusValTentativeshieldedinclusion, wantPos: InclPosEarlier,
		},
		{
			name:        "bad-predecessor-unshielded",
			expectedPos: 1, blockPos: 1,
			entries:    makeEntries(data.TxStatusValUnshieldedinclusion, data.TxStatusValPending),
			wantStatus: data.TxStatusValUnshieldedinclusion, wantPos: InclPosExact,
		},
		{
			name:        "earlier due to invalid predecessor -> tentative earlier",
			expectedPos: 2, blockPos: 1,
			entries:    makeEntries(data.TxStatusValInvalid, data.TxStatusValPending, data.TxStatusValPending),
			wantStatus: data.TxStatusValTentativeshieldedinclusion, wantPos: InclPosEarlier,
		},
		{
			name:        "exact with not included predecessor -> tentative",
			expectedPos: 1, blockPos: 1,
			entries:    makeEntries(data.TxStatusValNotincluded, data.TxStatusValPending),
			wantStatus: data.TxStatusValTentativeshieldedinclusion, wantPos: InclPosExact,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStatus, gotPos := classifyWithPredecessors(c.expectedPos, c.blockPos, c.entries)
			if gotStatus != c.wantStatus || gotPos != c.wantPos {
				t.Errorf("got (%v, %s), want (%v, %s)", gotStatus, gotPos, c.wantStatus, c.wantPos)
			}
		})
	}
}