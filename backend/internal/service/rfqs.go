package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type RFQRepository interface {
	Create(ctx context.Context, params postgresrepository.CreateRFQParams) (domain.RFQ, error)
	GetByID(ctx context.Context, id string) (domain.RFQ, error)
	List(ctx context.Context, filter postgresrepository.RFQFilter) (postgresrepository.RFQPage, error)
}

type RFQs struct {
	repository RFQRepository
	now        func() time.Time
}

type CreateRFQInput struct {
	Name                 string
	ReferenceCode        string
	CurrencyCode         string
	ServiceDate          time.Time
	BidStartAt           time.Time
	InitialCloseAt       time.Time
	ForcedCloseAt        time.Time
	TriggerWindowMinutes int
	ExtensionMinutes     int
	TriggerType          domain.TriggerType
}

func NewRFQs(repository RFQRepository) *RFQs {
	return &RFQs{repository: repository, now: time.Now}
}

func (service *RFQs) Create(ctx context.Context, identity auth.Identity, input CreateRFQInput) (domain.RFQ, error) {
	if identity.Role != domain.RoleBuyer {
		return domain.RFQ{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ReferenceCode = strings.TrimSpace(input.ReferenceCode)
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if input.CurrencyCode == "" {
		input.CurrencyCode = "INR"
	}
	if err := service.validateCreate(input); err != nil {
		return domain.RFQ{}, err
	}

	rfq, err := service.repository.Create(ctx, postgresrepository.CreateRFQParams{
		ReferenceCode: input.ReferenceCode, Name: input.Name, BuyerID: identity.UserID,
		CurrencyCode: input.CurrencyCode, ServiceDate: input.ServiceDate, BidStartAt: input.BidStartAt,
		InitialCloseAt: input.InitialCloseAt, ForcedCloseAt: input.ForcedCloseAt,
		TriggerWindowMinutes: input.TriggerWindowMinutes, ExtensionMinutes: input.ExtensionMinutes,
		TriggerType: input.TriggerType,
	})
	if err != nil {
		if errors.Is(err, postgresrepository.ErrReferenceExists) {
			return domain.RFQ{}, ErrReferenceExists
		}
		return domain.RFQ{}, fmt.Errorf("create RFQ: %w", err)
	}
	return rfq, nil
}

func (service *RFQs) Get(ctx context.Context, id string) (domain.RFQ, error) {
	rfq, err := service.repository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, postgresrepository.ErrNotFound) {
			return domain.RFQ{}, ErrNotFound
		}
		return domain.RFQ{}, fmt.Errorf("get RFQ: %w", err)
	}
	return rfq, nil
}

func (service *RFQs) List(ctx context.Context, page, pageSize int, status *domain.RFQStatus) (postgresrepository.RFQPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return postgresrepository.RFQPage{}, ValidationError{Message: "page must be at least 1 and page_size must be between 1 and 100"}
	}
	if status != nil && !domain.ValidStatus(*status) {
		return postgresrepository.RFQPage{}, ValidationError{Message: "invalid status filter"}
	}
	result, err := service.repository.List(ctx, postgresrepository.RFQFilter{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		return postgresrepository.RFQPage{}, fmt.Errorf("list RFQs: %w", err)
	}
	return result, nil
}

func (service *RFQs) validateCreate(input CreateRFQInput) error {
	if input.Name == "" || input.ReferenceCode == "" {
		return ValidationError{Message: "name and reference_code are required"}
	}
	if len(input.Name) > 200 || len(input.ReferenceCode) > 100 {
		return ValidationError{Message: "name or reference_code is too long"}
	}
	if !currencyPattern.MatchString(input.CurrencyCode) {
		return ValidationError{Message: "currency_code must contain three uppercase letters"}
	}
	if input.ServiceDate.IsZero() {
		return ValidationError{Message: "service_date is required"}
	}
	if input.BidStartAt.Before(service.now()) {
		return ValidationError{Message: "bid_start_at must not be in the past"}
	}
	if !input.BidStartAt.Before(input.InitialCloseAt) {
		return ValidationError{Message: "bid_start_at must be before initial_close_at"}
	}
	if !input.InitialCloseAt.Before(input.ForcedCloseAt) {
		return ValidationError{Message: "forced_close_at must be after initial_close_at"}
	}
	if input.TriggerWindowMinutes <= 0 || input.ExtensionMinutes <= 0 {
		return ValidationError{Message: "trigger_window_minutes and extension_minutes must be positive"}
	}
	switch input.TriggerType {
	case domain.TriggerAnyBid, domain.TriggerAnyRankChange, domain.TriggerL1Change:
	default:
		return ValidationError{Message: "invalid trigger_type"}
	}
	return nil
}
