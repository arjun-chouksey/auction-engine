//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

func TestBidSubmissionRankingAndHistory(t *testing.T) {
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `TRUNCATE auction_events, bids, rfqs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var rfqID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO rfqs (
			reference_code, name, buyer_id, currency_code, service_date, bid_start_at,
			initial_close_at, current_close_at, forced_close_at,
			trigger_window_minutes, extension_minutes, trigger_type
		) SELECT 'RFQ-BIDS-001', 'Bid integration', id, 'INR', current_date + 10,
		         clock_timestamp() - interval '2 hours', clock_timestamp() - interval '1 hour',
		         clock_timestamp() + interval '1 hour', clock_timestamp() + interval '2 hours',
		         10, 5, 'ANY_BID' FROM users WHERE email = 'buyer@example.test'
		RETURNING id
	`).Scan(&rfqID); err != nil {
		t.Fatal(err)
	}

	handler := newTestHandler(t)
	buyer := login(t, handler, "buyer@example.test")
	supplierA := login(t, handler, "supplier.a@example.test")
	supplierB := login(t, handler, "supplier.b@example.test")
	supplierC := login(t, handler, "supplier.c@example.test")
	path := "/api/v1/rfqs/" + rfqID + "/bids"

	quote := validBidRequest("700000", "150000", "100000")
	response := performBidRequest(t, handler, path, buyer, "00000000-0000-4000-8000-000000000001", quote)
	assertErrorCode(t, response, http.StatusForbidden, "FORBIDDEN")
	response = performBidRequest(t, handler, path, supplierA, "", quote)
	assertErrorCode(t, response, http.StatusBadRequest, "INVALID_REQUEST")

	firstKey := "00000000-0000-4000-8000-000000000011"
	response = performBidRequest(t, handler, path, supplierA, firstKey, quote)
	assertStatus(t, response, http.StatusCreated)
	first := decodeBidResult(t, response)
	if first.Bid.ID == "" || first.Bid.TotalMinor != "950000" || first.CurrentRank != 1 || first.Replay {
		t.Fatalf("first bid response = %+v", first)
	}

	response = performBidRequest(t, handler, path, supplierA, firstKey, quote)
	assertStatus(t, response, http.StatusOK)
	replay := decodeBidResult(t, response)
	if replay.Bid.ID != first.Bid.ID || !replay.Replay {
		t.Fatalf("replay = %+v, first = %+v", replay, first)
	}
	changed := validBidRequest("699999", "150000", "100000")
	response = performBidRequest(t, handler, path, supplierA, firstKey, changed)
	assertErrorCode(t, response, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")

	response = performBidRequest(t, handler, path, supplierC, "00000000-0000-4000-8000-000000000031", quote)
	assertStatus(t, response, http.StatusCreated)
	response = performRequest(t, handler, http.MethodGet, "/api/v1/rfqs/"+rfqID+"/ranking", buyer, nil)
	assertStatus(t, response, http.StatusOK)
	var tiedRanking struct {
		Items []struct {
			SupplierID string `json:"supplier_id"`
		} `json:"items"`
	}
	decodeResponse(t, response, &tiedRanking)
	if len(tiedRanking.Items) != 2 || tiedRanking.Items[0].SupplierID != first.Bid.SupplierID {
		t.Fatalf("tie ranking = %+v", tiedRanking.Items)
	}
	lowerB := validBidRequest("550000", "150000", "100000")
	response = performBidRequest(t, handler, path, supplierB, "00000000-0000-4000-8000-000000000021", lowerB)
	assertStatus(t, response, http.StatusCreated)

	secondA := validBidRequest("650000", "150000", "100000")
	response = performBidRequest(t, handler, path, supplierA, "00000000-0000-4000-8000-000000000012", secondA)
	assertStatus(t, response, http.StatusCreated)
	response = performBidRequest(t, handler, path, supplierA, "00000000-0000-4000-8000-000000000013", secondA)
	assertErrorCode(t, response, http.StatusConflict, "BID_NOT_IMPROVED")

	thirdA := validBidRequest("600000", "150000", "100000")
	response = performBidRequest(t, handler, path, supplierA, "00000000-0000-4000-8000-000000000014", thirdA)
	assertStatus(t, response, http.StatusCreated)

	invalidCases := []map[string]any{
		validBidRequest("-1", "1", "1"),
		validBidRequest("0", "0", "0"),
		validBidRequest("9223372036854775807", "1", "0"),
		{"carrier_name": "Carrier", "freight_minor": "1", "origin_minor": "0", "destination_minor": "0", "transit_time_hours": 0, "quote_valid_until": "2035-01-01"},
		{"carrier_name": "", "freight_minor": "1", "origin_minor": "0", "destination_minor": "0", "transit_time_hours": 1, "quote_valid_until": "2035-01-01"},
		{"carrier_name": "Carrier", "freight_minor": "1", "origin_minor": "0", "destination_minor": "0", "transit_time_hours": 1, "quote_valid_until": "2020-01-01"},
	}
	for index, invalid := range invalidCases {
		response = performBidRequest(t, handler, path, supplierB, fmt.Sprintf("00000000-0000-4000-8000-%012d", 100+index), invalid)
		assertErrorCode(t, response, http.StatusUnprocessableEntity, "INVALID_QUOTE")
	}
	malformed := validBidRequest("not-a-number", "0", "0")
	response = performBidRequest(t, handler, path, supplierB, "00000000-0000-4000-8000-000000000199", malformed)
	assertErrorCode(t, response, http.StatusBadRequest, "INVALID_REQUEST")

	response = performRequest(t, handler, http.MethodGet, "/api/v1/rfqs/"+rfqID+"/ranking", buyer, nil)
	assertStatus(t, response, http.StatusOK)
	var ranking struct {
		Items []struct {
			Rank       int    `json:"rank"`
			SupplierID string `json:"supplier_id"`
			Bid        struct {
				TotalMinor string `json:"total_minor"`
			} `json:"bid"`
		} `json:"items"`
	}
	decodeResponse(t, response, &ranking)
	if len(ranking.Items) != 3 || ranking.Items[0].Rank != 1 || ranking.Items[0].Bid.TotalMinor != "800000" ||
		ranking.Items[1].Rank != 2 || ranking.Items[1].Bid.TotalMinor != "850000" || ranking.Items[2].Rank != 3 {
		t.Fatalf("ranking = %+v", ranking.Items)
	}

	response = performRequest(t, handler, http.MethodGet, path, buyer, nil)
	assertStatus(t, response, http.StatusOK)
	var history struct {
		Items []struct {
			ID         string `json:"id"`
			TotalMinor string `json:"total_minor"`
		} `json:"items"`
		Total int64 `json:"total_items"`
	}
	decodeResponse(t, response, &history)
	if history.Total != 5 || len(history.Items) != 5 {
		t.Fatalf("history count = %+v", history)
	}
	response = performRequest(t, handler, http.MethodGet, path+"?sort=newest", buyer, nil)
	assertStatus(t, response, http.StatusOK)
	for index := 1; index < len(history.Items); index++ {
		if history.Items[index-1].TotalMinor > history.Items[index].TotalMinor {
			t.Fatalf("history is not price sorted: %+v", history.Items)
		}
	}

	var bidCount, eventCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM bids WHERE rfq_id=$1`, rfqID).Scan(&bidCount); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM auction_events WHERE rfq_id=$1 AND event_type='BID_ACCEPTED'`, rfqID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if bidCount != 5 || eventCount != 5 {
		t.Fatalf("side effects = bids:%d events:%d", bidCount, eventCount)
	}

	if _, err := testPool.Exec(ctx, `UPDATE rfqs SET current_close_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, rfqID); err != nil {
		t.Fatal(err)
	}
	response = performBidRequest(t, handler, path, supplierB, "00000000-0000-4000-8000-000000000022", validBidRequest("500000", "100000", "100000"))
	assertErrorCode(t, response, http.StatusConflict, "AUCTION_CLOSED")
	response = performBidRequest(t, handler, path, supplierA, firstKey, quote)
	assertStatus(t, response, http.StatusOK)
	if !decodeBidResult(t, response).Replay {
		t.Fatal("replay after close was not identified as replay")
	}

	var upcomingID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO rfqs (
			reference_code, name, buyer_id, currency_code, service_date, bid_start_at,
			initial_close_at, current_close_at, forced_close_at,
			trigger_window_minutes, extension_minutes, trigger_type
		) SELECT 'RFQ-UPCOMING-001', 'Upcoming', id, 'INR', current_date + 10,
		         clock_timestamp() + interval '1 hour', clock_timestamp() + interval '2 hours',
		         clock_timestamp() + interval '2 hours', clock_timestamp() + interval '3 hours',
		         10, 5, 'ANY_BID' FROM users WHERE email='buyer@example.test'
		RETURNING id
	`).Scan(&upcomingID); err != nil {
		t.Fatal(err)
	}
	response = performBidRequest(t, handler, "/api/v1/rfqs/"+upcomingID+"/bids", supplierB,
		"00000000-0000-4000-8000-000000000299", quote)
	assertErrorCode(t, response, http.StatusConflict, "AUCTION_NOT_STARTED")
}

func TestConcurrentSameIdempotencyKeyCreatesOneBid(t *testing.T) {
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `TRUNCATE auction_events, bids, rfqs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var rfqID, supplierID string
	if err := testPool.QueryRow(ctx, `SELECT id FROM users WHERE email='supplier.a@example.test'`).Scan(&supplierID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO rfqs (
			reference_code, name, buyer_id, currency_code, service_date, bid_start_at,
			initial_close_at, current_close_at, forced_close_at,
			trigger_window_minutes, extension_minutes, trigger_type
		) SELECT 'RFQ-CONCURRENT-KEY', 'Concurrent key', id, 'INR', current_date + 10,
		         clock_timestamp() - interval '2 hours', clock_timestamp() - interval '1 hour',
		         clock_timestamp() + interval '1 hour', clock_timestamp() + interval '2 hours',
		         10, 5, 'ANY_BID' FROM users WHERE email='buyer@example.test'
		RETURNING id
	`).Scan(&rfqID); err != nil {
		t.Fatal(err)
	}
	repository := postgresrepository.NewBids(testPool)
	params := postgresrepository.SubmitBidParams{
		RFQID: rfqID, SupplierID: supplierID, IdempotencyKey: "00000000-0000-4000-8000-000000000099",
		CarrierName: "Concurrent Carrier", FreightMinor: 700000, OriginMinor: 150000,
		DestinationMinor: 100000, TransitTimeHours: 24,
		QuoteValidUntil: time.Now().UTC().AddDate(0, 1, 0),
	}
	results := make(chan postgresrepository.SubmitBidResult, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := repository.Submit(ctx, params)
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent submit: %v", err)
		}
	}
	replays := 0
	var bidID int64
	for result := range results {
		if result.IdempotentReplay {
			replays++
		}
		if bidID != 0 && result.Bid.ID != bidID {
			t.Fatalf("concurrent results returned bid IDs %d and %d", bidID, result.Bid.ID)
		}
		bidID = result.Bid.ID
	}
	if replays != 1 {
		t.Fatalf("replay count = %d, want 1", replays)
	}
	var bidCount, eventCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM bids WHERE rfq_id=$1`, rfqID).Scan(&bidCount); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM auction_events WHERE rfq_id=$1 AND event_type='BID_ACCEPTED'`, rfqID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if bidCount != 1 || eventCount != 1 {
		t.Fatalf("concurrent side effects = bids:%d events:%d", bidCount, eventCount)
	}
}

func validBidRequest(freight, origin, destination string) map[string]any {
	return map[string]any{
		"carrier_name": "Supplier Logistics", "freight_minor": freight,
		"origin_minor": origin, "destination_minor": destination,
		"transit_time_hours": 24, "quote_valid_until": "2035-01-01",
	}
}

func performBidRequest(t *testing.T, handler http.Handler, path, token, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type bidResultBody struct {
	Bid struct {
		ID         string `json:"id"`
		SupplierID string `json:"supplier_id"`
		TotalMinor string `json:"total_minor"`
	} `json:"bid"`
	CurrentRank int  `json:"current_rank"`
	Replay      bool `json:"idempotent_replay"`
}

func decodeBidResult(t *testing.T, response *httptest.ResponseRecorder) bidResultBody {
	t.Helper()
	var body bidResultBody
	decodeResponse(t, response, &body)
	return body
}
