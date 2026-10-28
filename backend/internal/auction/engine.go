package auction

import (
	"slices"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
)

type CloseUpdate struct {
	Extended bool
	OldClose time.Time
	NewClose time.Time
	Reason   domain.TriggerType
}

func CanAcceptBid(now time.Time, rfq domain.RFQ) bool {
	return !now.Before(rfq.BidStartAt) && now.Before(rfq.CurrentCloseAt) && now.Before(rfq.ForcedCloseAt)
}

func InsideTriggerWindow(now time.Time, rfq domain.RFQ) bool {
	windowStart := rfq.CurrentCloseAt.Add(-time.Duration(rfq.TriggerWindowMinutes) * time.Minute)
	if windowStart.Before(rfq.BidStartAt) {
		windowStart = rfq.BidStartAt
	}
	return !now.Before(windowStart) && now.Before(rfq.CurrentCloseAt) && now.Before(rfq.ForcedCloseAt)
}

func HasRankChanged(before, after []domain.RankedBid) bool {
	return !slices.Equal(supplierOrder(before), supplierOrder(after))
}

func HasL1Changed(before, after []domain.RankedBid) bool {
	return len(before) > 0 && len(after) > 0 && before[0].Bid.SupplierID != after[0].Bid.SupplierID
}

func ComputeExtension(now time.Time, rfq domain.RFQ, before, after []domain.RankedBid) CloseUpdate {
	result := CloseUpdate{OldClose: rfq.CurrentCloseAt, NewClose: rfq.CurrentCloseAt}
	if !CanAcceptBid(now, rfq) || !InsideTriggerWindow(now, rfq) {
		return result
	}
	qualifies := false
	switch rfq.TriggerType {
	case domain.TriggerAnyBid:
		qualifies = true
	case domain.TriggerAnyRankChange:
		qualifies = HasRankChanged(before, after)
	case domain.TriggerL1Change:
		qualifies = HasL1Changed(before, after)
	}
	if !qualifies {
		return result
	}
	newClose := rfq.CurrentCloseAt.Add(time.Duration(rfq.ExtensionMinutes) * time.Minute)
	if newClose.After(rfq.ForcedCloseAt) {
		newClose = rfq.ForcedCloseAt
	}
	if !newClose.After(rfq.CurrentCloseAt) {
		return result
	}
	result.Extended = true
	result.NewClose = newClose
	result.Reason = rfq.TriggerType
	return result
}

func supplierOrder(ranking []domain.RankedBid) []string {
	order := make([]string, len(ranking))
	for index, item := range ranking {
		order[index] = item.Bid.SupplierID
	}
	return order
}
