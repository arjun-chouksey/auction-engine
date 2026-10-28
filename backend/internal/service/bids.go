package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

var moneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

type BidRepository interface {
	Submit(context.Context, postgresrepository.SubmitBidParams) (postgresrepository.SubmitBidResult, error)
	Ranking(context.Context, string) ([]domain.RankedBid, time.Time, error)
	List(context.Context, string, string, int, int) (postgresrepository.BidPage, error)
}

type ChangePublisher interface {
	Publish(rfqID, reason string)
}

type Bids struct {
	repository BidRepository
	publisher  ChangePublisher
}

type SubmitBidInput struct {
	RFQID            string
	IdempotencyKey   string
	CarrierName      string
	FreightMinor     string
	OriginMinor      string
	DestinationMinor string
	TransitTimeHours int
	QuoteValidUntil  time.Time
}

func NewBids(repository BidRepository, publishers ...ChangePublisher) *Bids {
	service := &Bids{repository: repository}
	if len(publishers) > 0 {
		service.publisher = publishers[0]
	}
	return service
}

func (service *Bids) Submit(ctx context.Context, identity auth.Identity, input SubmitBidInput) (postgresrepository.SubmitBidResult, error) {
	if identity.Role != domain.RoleSupplier {
		return postgresrepository.SubmitBidResult{}, ErrForbidden
	}
	input.CarrierName = strings.TrimSpace(input.CarrierName)
	if input.CarrierName == "" || len(input.CarrierName) > 200 {
		return postgresrepository.SubmitBidResult{}, QuoteValidationError{Message: "carrier_name is required and must not exceed 200 characters"}
	}
	if input.TransitTimeHours <= 0 {
		return postgresrepository.SubmitBidResult{}, QuoteValidationError{Message: "transit_time_hours must be positive"}
	}
	if input.QuoteValidUntil.IsZero() {
		return postgresrepository.SubmitBidResult{}, ValidationError{Message: "quote_valid_until is required"}
	}
	freight, err := parseMoney(input.FreightMinor)
	if err != nil {
		return postgresrepository.SubmitBidResult{}, err
	}
	origin, err := parseMoney(input.OriginMinor)
	if err != nil {
		return postgresrepository.SubmitBidResult{}, err
	}
	destination, err := parseMoney(input.DestinationMinor)
	if err != nil {
		return postgresrepository.SubmitBidResult{}, err
	}
	total, err := domain.SumCharges(freight, origin, destination)
	if err != nil || total <= 0 {
		return postgresrepository.SubmitBidResult{}, QuoteValidationError{Message: "charge total must be positive and fit in a signed 64-bit integer"}
	}
	result, err := service.repository.Submit(ctx, postgresrepository.SubmitBidParams{
		RFQID: input.RFQID, SupplierID: identity.UserID, IdempotencyKey: input.IdempotencyKey,
		CarrierName: input.CarrierName, FreightMinor: freight, OriginMinor: origin,
		DestinationMinor: destination, TransitTimeHours: input.TransitTimeHours,
		QuoteValidUntil: input.QuoteValidUntil,
	})
	if err != nil {
		switch {
		case errors.Is(err, postgresrepository.ErrNotFound):
			return postgresrepository.SubmitBidResult{}, ErrNotFound
		case errors.Is(err, postgresrepository.ErrAuctionNotActive):
			return postgresrepository.SubmitBidResult{}, ErrAuctionClosed
		case errors.Is(err, postgresrepository.ErrAuctionNotStarted):
			return postgresrepository.SubmitBidResult{}, ErrAuctionNotStarted
		case errors.Is(err, postgresrepository.ErrBidNotImproved):
			return postgresrepository.SubmitBidResult{}, ErrBidNotImproved
		case errors.Is(err, postgresrepository.ErrIdempotencyKeyReused):
			return postgresrepository.SubmitBidResult{}, ErrIdempotencyReused
		case errors.Is(err, postgresrepository.ErrQuoteValidity):
			return postgresrepository.SubmitBidResult{}, QuoteValidationError{Message: "quote_valid_until must not precede the RFQ service_date"}
		case errors.Is(err, postgresrepository.ErrInvalidBidTotal):
			return postgresrepository.SubmitBidResult{}, QuoteValidationError{Message: "charge total must be positive and fit in a signed 64-bit integer"}
		default:
			return postgresrepository.SubmitBidResult{}, fmt.Errorf("submit bid: %w", err)
		}
	}
	if !result.IdempotentReplay && service.publisher != nil {
		service.publisher.Publish(input.RFQID, string(domain.EventBidAccepted))
	}
	return result, nil
}

func (service *Bids) Ranking(ctx context.Context, rfqID string) ([]domain.RankedBid, time.Time, error) {
	items, serverTime, err := service.repository.Ranking(ctx, rfqID)
	if errors.Is(err, postgresrepository.ErrNotFound) {
		return nil, time.Time{}, ErrNotFound
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("get ranking: %w", err)
	}
	return items, serverTime, nil
}

func (service *Bids) List(ctx context.Context, rfqID, sortOrder string, page, pageSize int) (postgresrepository.BidPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return postgresrepository.BidPage{}, ValidationError{Message: "page must be at least 1 and page_size must be between 1 and 100"}
	}
	if sortOrder == "" {
		sortOrder = "price_asc"
	}
	if sortOrder != "price_asc" && sortOrder != "newest" {
		return postgresrepository.BidPage{}, ValidationError{Message: "sort must be price_asc or newest"}
	}
	result, err := service.repository.List(ctx, rfqID, sortOrder, page, pageSize)
	if errors.Is(err, postgresrepository.ErrNotFound) {
		return postgresrepository.BidPage{}, ErrNotFound
	}
	if err != nil {
		return postgresrepository.BidPage{}, fmt.Errorf("list bids: %w", err)
	}
	return result, nil
}

func parseMoney(value string) (int64, error) {
	if strings.HasPrefix(value, "-") {
		return 0, QuoteValidationError{Message: "money fields must be nonnegative"}
	}
	if !moneyPattern.MatchString(value) {
		return 0, ValidationError{Message: "money fields must be nonnegative decimal strings"}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, ValidationError{Message: "money field exceeds the signed 64-bit range"}
	}
	return parsed, nil
}
