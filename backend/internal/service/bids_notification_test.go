package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

type notificationBidRepository struct {
	result postgresrepository.SubmitBidResult
	err    error
}

func (repository notificationBidRepository) Submit(context.Context, postgresrepository.SubmitBidParams) (postgresrepository.SubmitBidResult, error) {
	return repository.result, repository.err
}

func (notificationBidRepository) Ranking(context.Context, string) ([]domain.RankedBid, time.Time, error) {
	return nil, time.Time{}, nil
}

func (notificationBidRepository) List(context.Context, string, string, int, int) (postgresrepository.BidPage, error) {
	return postgresrepository.BidPage{}, nil
}

type recordingPublisher struct{ messages []string }

func (publisher *recordingPublisher) Publish(rfqID, reason string) {
	publisher.messages = append(publisher.messages, rfqID+":"+reason)
}

func TestBidNotificationOnlyForNewCommittedResult(t *testing.T) {
	identity := auth.Identity{UserID: "supplier", Role: domain.RoleSupplier}
	input := SubmitBidInput{
		RFQID: "rfq", IdempotencyKey: "key", CarrierName: "Carrier",
		FreightMinor: "1", OriginMinor: "0", DestinationMinor: "0",
		TransitTimeHours: 1, QuoteValidUntil: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name      string
		result    postgresrepository.SubmitBidResult
		err       error
		wantCount int
	}{
		{name: "new accepted bid", wantCount: 1},
		{name: "idempotent replay", result: postgresrepository.SubmitBidResult{IdempotentReplay: true}, wantCount: 0},
		{name: "repository error", err: postgresrepository.ErrAuctionNotActive, wantCount: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publisher := &recordingPublisher{}
			service := NewBids(notificationBidRepository{result: test.result, err: test.err}, publisher)
			_, err := service.Submit(context.Background(), identity, input)
			if test.err == nil && err != nil {
				t.Fatal(err)
			}
			if test.err != nil && !errors.Is(err, ErrAuctionClosed) {
				t.Fatalf("error = %v", err)
			}
			if len(publisher.messages) != test.wantCount {
				t.Fatalf("published %d messages, want %d", len(publisher.messages), test.wantCount)
			}
			if test.wantCount == 1 && publisher.messages[0] != "rfq:BID_ACCEPTED" {
				t.Fatalf("message = %q", publisher.messages[0])
			}
		})
	}
}
