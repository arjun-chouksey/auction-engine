package domain

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventRFQCreated      EventType = "RFQ_CREATED"
	EventBidAccepted     EventType = "BID_ACCEPTED"
	EventAuctionExtended EventType = "AUCTION_EXTENDED"
)

type AuctionEvent struct {
	ID        int64
	RFQID     string
	BidID     *int64
	ActorID   *string
	Type      EventType
	Data      json.RawMessage
	CreatedAt time.Time
}
