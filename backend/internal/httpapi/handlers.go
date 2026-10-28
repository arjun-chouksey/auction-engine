package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

const maxRequestBodyBytes = 1 << 20

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type handlers struct {
	auth *service.Auth
	rfqs *service.RFQs
	bids *service.Bids
}

func newHandlers(authService *service.Auth, rfqService *service.RFQs, bidService *service.Bids) *handlers {
	return &handlers{auth: authService, rfqs: rfqService, bids: bidService}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID       string      `json:"id"`
	FullName string      `json:"full_name"`
	Email    string      `json:"email"`
	Role     domain.Role `json:"role"`
}

func (handlers *handlers) login(response http.ResponseWriter, request *http.Request) {
	var input loginRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := handlers.auth.Login(request.Context(), input.Email, input.Password)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"access_token": result.AccessToken, "token_type": "Bearer",
		"expires_in_seconds": result.ExpiresInSecond, "user": toUserResponse(result.User),
	})
}

func (handlers *handlers) me(response http.ResponseWriter, request *http.Request) {
	identity, _ := identityFromContext(request.Context())
	user, err := handlers.auth.Me(request.Context(), identity.UserID)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, toUserResponse(user))
}

type createRFQRequest struct {
	Name                 string             `json:"name"`
	ReferenceCode        string             `json:"reference_code"`
	CurrencyCode         string             `json:"currency_code"`
	ServiceDate          string             `json:"service_date"`
	BidStartAt           string             `json:"bid_start_at"`
	InitialCloseAt       string             `json:"initial_close_at"`
	ForcedCloseAt        string             `json:"forced_close_at"`
	TriggerWindowMinutes int                `json:"trigger_window_minutes"`
	ExtensionMinutes     int                `json:"extension_minutes"`
	TriggerType          domain.TriggerType `json:"trigger_type"`
}

func (handlers *handlers) createRFQ(response http.ResponseWriter, request *http.Request) {
	var input createRFQRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	parsed, err := parseCreateRFQ(input)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	identity, _ := identityFromContext(request.Context())
	rfq, err := handlers.rfqs.Create(request.Context(), identity, parsed)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	response.Header().Set("Location", "/api/v1/rfqs/"+rfq.ID)
	writeJSON(response, http.StatusCreated, toRFQResponse(rfq))
}

func (handlers *handlers) getRFQ(response http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "rfq_id")
	if !uuidPattern.MatchString(id) {
		writeError(response, http.StatusNotFound, "RFQ_NOT_FOUND", "RFQ not found")
		return
	}
	rfq, err := handlers.rfqs.Get(request.Context(), id)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, toRFQResponse(rfq))
}

func (handlers *handlers) listRFQs(response http.ResponseWriter, request *http.Request) {
	page, err := parsePositiveInt(request.URL.Query().Get("page"), 1)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "page must be a positive integer")
		return
	}
	pageSize, err := parsePositiveInt(request.URL.Query().Get("page_size"), 20)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "page_size must be a positive integer")
		return
	}
	var status *domain.RFQStatus
	if raw := strings.TrimSpace(request.URL.Query().Get("status")); raw != "" {
		value := domain.RFQStatus(strings.ToUpper(raw))
		status = &value
	}
	result, err := handlers.rfqs.List(request.Context(), page, pageSize, status)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	items := make([]rfqResponse, 0, len(result.Items))
	for _, rfq := range result.Items {
		items = append(items, toRFQResponse(rfq))
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"items": items, "page": page, "page_size": pageSize,
		"total_items": result.TotalItems, "server_time": result.ServerTime.UTC(),
	})
}

type rfqResponse struct {
	ID                   string             `json:"id"`
	ReferenceCode        string             `json:"reference_code"`
	Name                 string             `json:"name"`
	BuyerID              string             `json:"buyer_id"`
	CurrencyCode         string             `json:"currency_code"`
	ServiceDate          string             `json:"service_date"`
	BidStartAt           time.Time          `json:"bid_start_at"`
	InitialCloseAt       time.Time          `json:"initial_close_at"`
	CurrentCloseAt       time.Time          `json:"current_close_at"`
	ForcedCloseAt        time.Time          `json:"forced_close_at"`
	TriggerWindowMinutes int                `json:"trigger_window_minutes"`
	ExtensionMinutes     int                `json:"extension_minutes"`
	TriggerType          domain.TriggerType `json:"trigger_type"`
	Status               domain.RFQStatus   `json:"status"`
	LowestBidMinor       *string            `json:"lowest_bid_minor"`
	TotalBidCount        int64              `json:"total_bid_count"`
	UniqueSupplierCount  int64              `json:"unique_supplier_count"`
	ServerTime           time.Time          `json:"server_time"`
}

func toRFQResponse(rfq domain.RFQ) rfqResponse {
	var lowest *string
	if rfq.LowestBidMinor != nil {
		value := strconv.FormatInt(*rfq.LowestBidMinor, 10)
		lowest = &value
	}
	return rfqResponse{
		ID: rfq.ID, ReferenceCode: rfq.ReferenceCode, Name: rfq.Name, BuyerID: rfq.BuyerID,
		CurrencyCode: rfq.CurrencyCode, ServiceDate: rfq.ServiceDate.Format(time.DateOnly),
		BidStartAt: rfq.BidStartAt.UTC(), InitialCloseAt: rfq.InitialCloseAt.UTC(),
		CurrentCloseAt: rfq.CurrentCloseAt.UTC(), ForcedCloseAt: rfq.ForcedCloseAt.UTC(),
		TriggerWindowMinutes: rfq.TriggerWindowMinutes, ExtensionMinutes: rfq.ExtensionMinutes,
		TriggerType: rfq.TriggerType, Status: domain.EffectiveStatus(rfq.ServerTime, rfq),
		LowestBidMinor: lowest, TotalBidCount: rfq.TotalBidCount,
		UniqueSupplierCount: rfq.UniqueSupplierCount, ServerTime: rfq.ServerTime.UTC(),
	}
}

func toUserResponse(user domain.User) userResponse {
	return userResponse{ID: user.ID, FullName: user.FullName, Email: user.Email, Role: user.Role}
}

func parseCreateRFQ(input createRFQRequest) (service.CreateRFQInput, error) {
	serviceDate, err := time.Parse(time.DateOnly, input.ServiceDate)
	if err != nil {
		return service.CreateRFQInput{}, errors.New("service_date must use YYYY-MM-DD")
	}
	start, err := time.Parse(time.RFC3339, input.BidStartAt)
	if err != nil {
		return service.CreateRFQInput{}, errors.New("bid_start_at must be an RFC3339 timestamp with timezone")
	}
	initialClose, err := time.Parse(time.RFC3339, input.InitialCloseAt)
	if err != nil {
		return service.CreateRFQInput{}, errors.New("initial_close_at must be an RFC3339 timestamp with timezone")
	}
	forcedClose, err := time.Parse(time.RFC3339, input.ForcedCloseAt)
	if err != nil {
		return service.CreateRFQInput{}, errors.New("forced_close_at must be an RFC3339 timestamp with timezone")
	}
	return service.CreateRFQInput{
		Name: input.Name, ReferenceCode: input.ReferenceCode, CurrencyCode: input.CurrencyCode,
		ServiceDate: serviceDate, BidStartAt: start, InitialCloseAt: initialClose, ForcedCloseAt: forcedClose,
		TriggerWindowMinutes: input.TriggerWindowMinutes, ExtensionMinutes: input.ExtensionMinutes,
		TriggerType: input.TriggerType,
	}, nil
}

type submitBidRequest struct {
	CarrierName      string `json:"carrier_name"`
	FreightMinor     string `json:"freight_minor"`
	OriginMinor      string `json:"origin_minor"`
	DestinationMinor string `json:"destination_minor"`
	TransitTimeHours int    `json:"transit_time_hours"`
	QuoteValidUntil  string `json:"quote_valid_until"`
}

type bidResponse struct {
	ID               string    `json:"id"`
	RFQID            string    `json:"rfq_id"`
	SupplierID       string    `json:"supplier_id"`
	CarrierName      string    `json:"carrier_name"`
	FreightMinor     string    `json:"freight_minor"`
	OriginMinor      string    `json:"origin_minor"`
	DestinationMinor string    `json:"destination_minor"`
	TotalMinor       string    `json:"total_minor"`
	TransitTimeHours int       `json:"transit_time_hours"`
	QuoteValidUntil  string    `json:"quote_valid_until"`
	SubmittedAt      time.Time `json:"submitted_at"`
}

func (handlers *handlers) submitBid(response http.ResponseWriter, request *http.Request) {
	rfqID := chi.URLParam(request, "rfq_id")
	if !uuidPattern.MatchString(rfqID) {
		writeError(response, http.StatusNotFound, "RFQ_NOT_FOUND", "RFQ not found")
		return
	}
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if !uuidPattern.MatchString(key) {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key must be a UUID")
		return
	}
	var input submitBidRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	validUntil, err := time.Parse(time.DateOnly, input.QuoteValidUntil)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "quote_valid_until must use YYYY-MM-DD")
		return
	}
	identity, _ := identityFromContext(request.Context())
	result, err := handlers.bids.Submit(request.Context(), identity, service.SubmitBidInput{
		RFQID: rfqID, IdempotencyKey: key, CarrierName: input.CarrierName,
		FreightMinor: input.FreightMinor, OriginMinor: input.OriginMinor,
		DestinationMinor: input.DestinationMinor, TransitTimeHours: input.TransitTimeHours,
		QuoteValidUntil: validUntil,
	})
	if err != nil {
		handleServiceError(response, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	writeJSON(response, status, map[string]any{
		"bid": toBidResponse(result.Bid), "current_rank": result.CurrentRank,
		"extended": false, "extension_reason": nil, "previous_close_at": nil,
		"current_close_at": result.CurrentCloseAt.UTC(), "server_time": result.ServerTime.UTC(),
		"idempotent_replay": result.IdempotentReplay,
	})
}

func (handlers *handlers) getRanking(response http.ResponseWriter, request *http.Request) {
	rfqID := chi.URLParam(request, "rfq_id")
	if !uuidPattern.MatchString(rfqID) {
		writeError(response, http.StatusNotFound, "RFQ_NOT_FOUND", "RFQ not found")
		return
	}
	items, serverTime, err := handlers.bids.Ranking(request.Context(), rfqID)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"rank": item.Rank, "supplier_id": item.Bid.SupplierID, "bid": toBidResponse(item.Bid)})
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": result, "server_time": serverTime.UTC()})
}

func (handlers *handlers) listBids(response http.ResponseWriter, request *http.Request) {
	rfqID := chi.URLParam(request, "rfq_id")
	if !uuidPattern.MatchString(rfqID) {
		writeError(response, http.StatusNotFound, "RFQ_NOT_FOUND", "RFQ not found")
		return
	}
	page, err := parsePositiveInt(request.URL.Query().Get("page"), 1)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "page must be a positive integer")
		return
	}
	pageSize, err := parsePositiveInt(request.URL.Query().Get("page_size"), 20)
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "page_size must be a positive integer")
		return
	}
	result, err := handlers.bids.List(request.Context(), rfqID, request.URL.Query().Get("sort"), page, pageSize)
	if err != nil {
		handleServiceError(response, err)
		return
	}
	items := make([]bidResponse, 0, len(result.Items))
	for _, bid := range result.Items {
		items = append(items, toBidResponse(bid))
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"items": items, "page": page, "page_size": pageSize,
		"total_items": result.TotalItems, "server_time": result.ServerTime.UTC(),
	})
}

func toBidResponse(bid domain.Bid) bidResponse {
	return bidResponse{
		ID: strconv.FormatInt(bid.ID, 10), RFQID: bid.RFQID, SupplierID: bid.SupplierID,
		CarrierName: bid.CarrierName, FreightMinor: strconv.FormatInt(bid.FreightMinor, 10),
		OriginMinor: strconv.FormatInt(bid.OriginMinor, 10), DestinationMinor: strconv.FormatInt(bid.DestinationMinor, 10),
		TotalMinor: strconv.FormatInt(bid.TotalMinor, 10), TransitTimeHours: bid.TransitTimeHours,
		QuoteValidUntil: bid.QuoteValidUntil.Format(time.DateOnly), SubmittedAt: bid.SubmittedAt.UTC(),
	}
}

func parsePositiveInt(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errors.New("must be positive")
	}
	return value, nil
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return errors.New("request body must be valid JSON with documented fields")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
