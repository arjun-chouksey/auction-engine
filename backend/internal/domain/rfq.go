package domain

import "time"

type TriggerType string

const (
	TriggerAnyBid        TriggerType = "ANY_BID"
	TriggerAnyRankChange TriggerType = "ANY_RANK_CHANGE"
	TriggerL1Change      TriggerType = "L1_CHANGE"
)

type RFQStatus string

const (
	StatusUpcoming    RFQStatus = "UPCOMING"
	StatusActive      RFQStatus = "ACTIVE"
	StatusClosed      RFQStatus = "CLOSED"
	StatusForceClosed RFQStatus = "FORCE_CLOSED"
)

type RFQ struct {
	ID                   string
	ReferenceCode        string
	Name                 string
	BuyerID              string
	CurrencyCode         string
	ServiceDate          time.Time
	BidStartAt           time.Time
	InitialCloseAt       time.Time
	CurrentCloseAt       time.Time
	ForcedCloseAt        time.Time
	TriggerWindowMinutes int
	ExtensionMinutes     int
	TriggerType          TriggerType
	LowestBidMinor       *int64
	TotalBidCount        int64
	UniqueSupplierCount  int64
	ServerTime           time.Time
}

func EffectiveStatus(now time.Time, rfq RFQ) RFQStatus {
	if rfq.CurrentCloseAt.Equal(rfq.ForcedCloseAt) && !now.Before(rfq.ForcedCloseAt) {
		return StatusForceClosed
	}
	if !now.Before(rfq.CurrentCloseAt) {
		return StatusClosed
	}
	if now.Before(rfq.BidStartAt) {
		return StatusUpcoming
	}
	return StatusActive
}

func ValidStatus(status RFQStatus) bool {
	switch status {
	case StatusUpcoming, StatusActive, StatusClosed, StatusForceClosed:
		return true
	default:
		return false
	}
}
