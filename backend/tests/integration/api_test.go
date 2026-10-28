//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/httpapi"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/realtime"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/service"
)

const testJWTSecret = "integration-test-jwt-secret-with-at-least-thirty-two-characters"

func TestAuthenticationAndRFQAPI(t *testing.T) {
	if _, err := testPool.Exec(context.Background(), `TRUNCATE auction_events, bids, rfqs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset RFQ data: %v", err)
	}
	handler := newTestHandler(t)

	t.Run("missing and invalid tokens are rejected", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodGet, "/api/v1/auth/me", "", nil)
		assertStatus(t, response, http.StatusUnauthorized)
		response = performRequest(t, handler, http.MethodGet, "/api/v1/auth/me", "not-a-token", nil)
		assertStatus(t, response, http.StatusUnauthorized)
	})

	t.Run("bad login is rejected", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
			"email": "buyer@example.test", "password": "wrong-password",
		})
		assertStatus(t, response, http.StatusUnauthorized)
	})

	buyerToken := login(t, handler, "buyer@example.test")
	supplierToken := login(t, handler, "supplier.a@example.test")

	t.Run("authenticated identity is returned", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodGet, "/api/v1/auth/me", buyerToken, nil)
		assertStatus(t, response, http.StatusOK)
		var user struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		decodeResponse(t, response, &user)
		if user.Email != "buyer@example.test" || user.Role != "BUYER" {
			t.Fatalf("me response = %+v", user)
		}
	})

	validRequest := validRFQRequest("RFQ-INTEGRATION-001")
	t.Run("supplier cannot create RFQ", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodPost, "/api/v1/rfqs", supplierToken, validRequest)
		assertStatus(t, response, http.StatusForbidden)
	})

	var created struct {
		ID             string  `json:"id"`
		InitialCloseAt string  `json:"initial_close_at"`
		CurrentCloseAt string  `json:"current_close_at"`
		Lowest         *string `json:"lowest_bid_minor"`
		Status         string  `json:"status"`
	}
	t.Run("buyer creates valid RFQ", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodPost, "/api/v1/rfqs", buyerToken, validRequest)
		assertStatus(t, response, http.StatusCreated)
		decodeResponse(t, response, &created)
		if created.ID == "" || created.InitialCloseAt != created.CurrentCloseAt {
			t.Fatalf("created RFQ = %+v", created)
		}
		if created.Lowest != nil || created.Status != "UPCOMING" {
			t.Fatalf("created aggregates/status = lowest:%v status:%s", created.Lowest, created.Status)
		}
	})

	t.Run("duplicate reference is rejected", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodPost, "/api/v1/rfqs", buyerToken, validRequest)
		assertErrorCode(t, response, http.StatusConflict, "RFQ_REFERENCE_EXISTS")
	})

	invalidCases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "forced close equals initial close", mutate: func(request map[string]any) { request["forced_close_at"] = request["initial_close_at"] }},
		{name: "start equals initial close", mutate: func(request map[string]any) { request["bid_start_at"] = request["initial_close_at"] }},
		{name: "trigger window is zero", mutate: func(request map[string]any) { request["trigger_window_minutes"] = 0 }},
		{name: "extension duration is zero", mutate: func(request map[string]any) { request["extension_minutes"] = 0 }},
	}
	for index, test := range invalidCases {
		t.Run(test.name, func(t *testing.T) {
			request := validRFQRequest(fmt.Sprintf("RFQ-INVALID-%d", index))
			test.mutate(request)
			response := performRequest(t, handler, http.MethodPost, "/api/v1/rfqs", buyerToken, request)
			assertErrorCode(t, response, http.StatusBadRequest, "INVALID_REQUEST")
		})
	}

	t.Run("authenticated users list and view RFQ", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodGet, "/api/v1/rfqs?status=UPCOMING&page=1&page_size=10", supplierToken, nil)
		assertStatus(t, response, http.StatusOK)
		var list struct {
			Items      []map[string]any `json:"items"`
			TotalItems int              `json:"total_items"`
		}
		decodeResponse(t, response, &list)
		if list.TotalItems != 1 || len(list.Items) != 1 {
			t.Fatalf("list response = %+v", list)
		}

		response = performRequest(t, handler, http.MethodGet, "/api/v1/rfqs/"+created.ID, supplierToken, nil)
		assertStatus(t, response, http.StatusOK)
	})

	t.Run("oversized page is rejected", func(t *testing.T) {
		response := performRequest(t, handler, http.MethodGet, "/api/v1/rfqs?page_size=101", buyerToken, nil)
		assertErrorCode(t, response, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("creation writes one audit event", func(t *testing.T) {
		var eventCount int
		var eventType string
		err := testPool.QueryRow(context.Background(), `
			SELECT count(*), max(event_type) FROM auction_events WHERE rfq_id = $1
		`, created.ID).Scan(&eventCount, &eventType)
		if err != nil {
			t.Fatal(err)
		}
		if eventCount != 1 || eventType != "RFQ_CREATED" {
			t.Fatalf("events = count:%d type:%s", eventCount, eventType)
		}
	})
}

func newTestHandler(t *testing.T) http.Handler {
	handler, _ := newTestApplication(t)
	return handler
}

func newTestApplication(t *testing.T) (http.Handler, *realtime.Hub) {
	t.Helper()
	tokens, err := auth.NewTokenManager(testJWTSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	users := postgresrepository.NewUsers(testPool)
	rfqs := postgresrepository.NewRFQs(testPool)
	bids := postgresrepository.NewBids(testPool)
	events := postgresrepository.NewEvents(testPool)
	hub := realtime.NewHub(4)
	return httpapi.NewRouter(httpapi.Dependencies{
		AuthService:  service.NewAuth(users, tokens),
		RFQService:   service.NewRFQs(rfqs),
		BidService:   service.NewBids(bids, hub),
		EventService: service.NewEvents(events),
		Hub:          hub,
		Tokens:       tokens,
	}), hub
}

func login(t *testing.T, handler http.Handler, email string) string {
	t.Helper()
	response := performRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": demoPassword,
	})
	assertStatus(t, response, http.StatusOK)
	var body struct {
		AccessToken string `json:"access_token"`
	}
	decodeResponse(t, response, &body)
	if body.AccessToken == "" {
		t.Fatal("login returned an empty token")
	}
	return body.AccessToken
}

func validRFQRequest(reference string) map[string]any {
	return map[string]any{
		"name": "Integration Freight", "reference_code": reference, "currency_code": "INR",
		"service_date": "2030-01-05", "bid_start_at": "2030-01-01T10:00:00Z",
		"initial_close_at": "2030-01-01T11:00:00Z", "forced_close_at": "2030-01-01T12:00:00Z",
		"trigger_window_minutes": 10, "extension_minutes": 5, "trigger_type": "ANY_BID",
	}
}

func performRequest(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	assertStatus(t, response, status)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeResponse(t, response, &body)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}
