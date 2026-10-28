package auction

import (
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
)

func TestBidTimeBoundaries(t *testing.T) {
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	rfq := testRFQ(start, start.Add(time.Hour), start.Add(2*time.Hour), domain.TriggerAnyBid)
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before start", start.Add(-time.Nanosecond), false},
		{"exactly start", start, true},
		{"before current close", rfq.CurrentCloseAt.Add(-time.Nanosecond), true},
		{"exactly current close", rfq.CurrentCloseAt, false},
		{"exactly forced close", rfq.ForcedCloseAt, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanAcceptBid(test.now, rfq); got != test.want {
				t.Fatalf("CanAcceptBid() = %v, want %v", got, test.want)
			}
		})
	}
	windowStart := rfq.CurrentCloseAt.Add(-10 * time.Minute)
	if !InsideTriggerWindow(windowStart, rfq) {
		t.Fatal("trigger lower bound must be inclusive")
	}
}

func TestComputeExtensionPolicies(t *testing.T) {
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	closeAt := start.Add(time.Hour)
	now := closeAt.Add(-time.Minute)
	a := ranking("a")
	ab := ranking("a", "b")
	ba := ranking("b", "a")
	tests := []struct {
		name    string
		trigger domain.TriggerType
		before  []domain.RankedBid
		after   []domain.RankedBid
		want    bool
	}{
		{"any bid unchanged", domain.TriggerAnyBid, a, a, true},
		{"any rank unchanged", domain.TriggerAnyRankChange, ab, ab, false},
		{"any rank new supplier", domain.TriggerAnyRankChange, a, ab, true},
		{"any rank swap", domain.TriggerAnyRankChange, ab, ba, true},
		{"l1 initial assignment", domain.TriggerL1Change, nil, a, false},
		{"l1 unchanged", domain.TriggerL1Change, ab, ab, false},
		{"l1 changed", domain.TriggerL1Change, ab, ba, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rfq := testRFQ(start, closeAt, closeAt.Add(time.Hour), test.trigger)
			update := ComputeExtension(now, rfq, test.before, test.after)
			if update.Extended != test.want {
				t.Fatalf("Extended = %v, want %v", update.Extended, test.want)
			}
		})
	}
}

func TestComputeExtensionWindowRepeatAndCap(t *testing.T) {
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	closeAt := start.Add(time.Hour)
	rfq := testRFQ(start, closeAt, closeAt.Add(12*time.Minute), domain.TriggerAnyBid)
	if update := ComputeExtension(closeAt.Add(-10*time.Minute-time.Nanosecond), rfq, nil, ranking("a")); update.Extended {
		t.Fatal("bid before trigger window extended")
	}
	first := ComputeExtension(closeAt.Add(-10*time.Minute), rfq, nil, ranking("a"))
	if !first.Extended || !first.NewClose.Equal(closeAt.Add(5*time.Minute)) {
		t.Fatalf("first extension = %+v", first)
	}
	rfq.CurrentCloseAt = first.NewClose
	second := ComputeExtension(closeAt.Add(-4*time.Minute), rfq, ranking("a"), ranking("a"))
	if !second.NewClose.Equal(closeAt.Add(10 * time.Minute)) {
		t.Fatalf("second extension = %+v", second)
	}
	rfq.CurrentCloseAt = second.NewClose
	capped := ComputeExtension(closeAt.Add(time.Minute), rfq, ranking("a"), ranking("a"))
	if !capped.Extended || !capped.NewClose.Equal(rfq.ForcedCloseAt) {
		t.Fatalf("capped extension = %+v", capped)
	}
	rfq.CurrentCloseAt = rfq.ForcedCloseAt
	if update := ComputeExtension(rfq.ForcedCloseAt, rfq, ranking("a"), ranking("a")); update.Extended {
		t.Fatal("extension occurred at forced close")
	}
}

func TestStatusAfterNaturalAndForcedClose(t *testing.T) {
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	rfq := testRFQ(start, start.Add(time.Hour), start.Add(2*time.Hour), domain.TriggerAnyBid)
	if status := domain.EffectiveStatus(rfq.ForcedCloseAt.Add(time.Hour), rfq); status != domain.StatusClosed {
		t.Fatalf("unused forced cap status = %s", status)
	}
	rfq.CurrentCloseAt = rfq.ForcedCloseAt
	if status := domain.EffectiveStatus(rfq.ForcedCloseAt, rfq); status != domain.StatusForceClosed {
		t.Fatalf("reached forced cap status = %s", status)
	}
}

func testRFQ(start, currentClose, forcedClose time.Time, trigger domain.TriggerType) domain.RFQ {
	return domain.RFQ{
		BidStartAt: start, CurrentCloseAt: currentClose, ForcedCloseAt: forcedClose,
		TriggerWindowMinutes: 10, ExtensionMinutes: 5, TriggerType: trigger,
	}
}

func ranking(suppliers ...string) []domain.RankedBid {
	result := make([]domain.RankedBid, len(suppliers))
	for index, supplier := range suppliers {
		result[index] = domain.RankedBid{Rank: index + 1, Bid: domain.Bid{SupplierID: supplier}}
	}
	return result
}
