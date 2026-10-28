package domain

import (
	"errors"
	"math"
	"sort"
	"time"
)

var ErrMoneyOverflow = errors.New("money total exceeds int64")

type Bid struct {
	ID               int64
	RFQID            string
	SupplierID       string
	IdempotencyKey   string
	CarrierName      string
	FreightMinor     int64
	OriginMinor      int64
	DestinationMinor int64
	TotalMinor       int64
	TransitTimeHours int
	QuoteValidUntil  time.Time
	SubmittedAt      time.Time
}

type RankedBid struct {
	Rank int
	Bid  Bid
}

func SumCharges(charges ...int64) (int64, error) {
	var total int64
	for _, charge := range charges {
		if charge < 0 || charge > math.MaxInt64-total {
			return 0, ErrMoneyOverflow
		}
		total += charge
	}
	return total, nil
}

func RankLatest(bids []Bid) []RankedBid {
	latest := make(map[string]Bid, len(bids))
	for _, bid := range bids {
		previous, exists := latest[bid.SupplierID]
		if !exists || bid.SubmittedAt.After(previous.SubmittedAt) ||
			(bid.SubmittedAt.Equal(previous.SubmittedAt) && bid.ID > previous.ID) {
			latest[bid.SupplierID] = bid
		}
	}
	current := make([]Bid, 0, len(latest))
	for _, bid := range latest {
		current = append(current, bid)
	}
	sort.Slice(current, func(i, j int) bool {
		if current[i].TotalMinor != current[j].TotalMinor {
			return current[i].TotalMinor < current[j].TotalMinor
		}
		if !current[i].SubmittedAt.Equal(current[j].SubmittedAt) {
			return current[i].SubmittedAt.Before(current[j].SubmittedAt)
		}
		return current[i].SupplierID < current[j].SupplierID
	})
	ranked := make([]RankedBid, len(current))
	for index, bid := range current {
		ranked[index] = RankedBid{Rank: index + 1, Bid: bid}
	}
	return ranked
}
