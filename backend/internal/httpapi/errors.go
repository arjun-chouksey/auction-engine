package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/service"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func writeError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, errorEnvelope{Error: errorBody{Code: code, Message: message, Details: map[string]any{}}})
}

func handleServiceError(response http.ResponseWriter, err error) {
	var validation service.ValidationError
	var quoteValidation service.QuoteValidationError
	switch {
	case errors.As(err, &quoteValidation):
		writeError(response, http.StatusUnprocessableEntity, "INVALID_QUOTE", quoteValidation.Message)
	case errors.As(err, &validation):
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", validation.Message)
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Invalid email or password")
	case errors.Is(err, service.ErrForbidden):
		writeError(response, http.StatusForbidden, "FORBIDDEN", "You are not allowed to perform this action")
	case errors.Is(err, service.ErrNotFound):
		writeError(response, http.StatusNotFound, "RFQ_NOT_FOUND", "RFQ not found")
	case errors.Is(err, service.ErrReferenceExists):
		writeError(response, http.StatusConflict, "RFQ_REFERENCE_EXISTS", "An RFQ with this reference already exists")
	case errors.Is(err, service.ErrAuctionClosed):
		writeError(response, http.StatusConflict, "AUCTION_CLOSED", "Auction is no longer accepting bids")
	case errors.Is(err, service.ErrAuctionNotStarted):
		writeError(response, http.StatusConflict, "AUCTION_NOT_STARTED", "Auction is not yet accepting bids")
	case errors.Is(err, service.ErrBidNotImproved):
		writeError(response, http.StatusConflict, "BID_NOT_IMPROVED", "Bid total must be lower than your latest accepted bid")
	case errors.Is(err, service.ErrIdempotencyReused):
		writeError(response, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency key was already used with a different quote")
	default:
		slog.Error("request failed", "error", err)
		writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
	}
}
