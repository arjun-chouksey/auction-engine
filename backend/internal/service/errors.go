package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrReferenceExists    = errors.New("RFQ reference exists")
	ErrAuctionClosed      = errors.New("auction is not accepting bids")
	ErrAuctionNotStarted  = errors.New("auction has not started")
	ErrBidNotImproved     = errors.New("bid is not an improvement")
	ErrIdempotencyReused  = errors.New("idempotency key reused")
)

type ValidationError struct {
	Message string
}

func (err ValidationError) Error() string {
	return err.Message
}

type QuoteValidationError struct {
	Message string
}

func (err QuoteValidationError) Error() string {
	return err.Message
}
