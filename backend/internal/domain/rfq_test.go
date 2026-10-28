package domain

import (
	"testing"
	"time"
)

func TestEffectiveStatus(t *testing.T) {
	base := time.Date(2030, time.January, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		now          time.Time
		start        time.Time
		currentClose time.Time
		forcedClose  time.Time
		want         RFQStatus
	}{
		{name: "upcoming", now: base, start: base.Add(time.Hour), currentClose: base.Add(2 * time.Hour), forcedClose: base.Add(3 * time.Hour), want: StatusUpcoming},
		{name: "active at start", now: base, start: base, currentClose: base.Add(time.Hour), forcedClose: base.Add(2 * time.Hour), want: StatusActive},
		{name: "closed at current close", now: base.Add(time.Hour), start: base, currentClose: base.Add(time.Hour), forcedClose: base.Add(2 * time.Hour), want: StatusClosed},
		{name: "natural close remains closed past unused cap", now: base.Add(3 * time.Hour), start: base, currentClose: base.Add(time.Hour), forcedClose: base.Add(2 * time.Hour), want: StatusClosed},
		{name: "forced close", now: base.Add(2 * time.Hour), start: base, currentClose: base.Add(2 * time.Hour), forcedClose: base.Add(2 * time.Hour), want: StatusForceClosed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rfq := RFQ{BidStartAt: test.start, CurrentCloseAt: test.currentClose, ForcedCloseAt: test.forcedClose}
			if got := EffectiveStatus(test.now, rfq); got != test.want {
				t.Fatalf("EffectiveStatus() = %s, want %s", got, test.want)
			}
		})
	}
}
