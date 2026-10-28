//go:build integration

package integration_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventAPIAndSSEPostCommitNotification(t *testing.T) {
	resetAuctionData(t)
	rfqID, _, _ := insertActiveAuction(t, "EVENT-SSE-001", "ANY_BID", 4*time.Minute, time.Hour)
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO auction_events (rfq_id, actor_id, event_type, event_data, created_at)
		SELECT $1, buyer_id, 'RFQ_CREATED', jsonb_build_object('reference_code','EVENT-SSE-001'), clock_timestamp()
		FROM rfqs WHERE id=$1
	`, rfqID); err != nil {
		t.Fatal(err)
	}

	handler, hub := newTestApplication(t)
	supplierToken := login(t, handler, "supplier.a@example.test")
	buyerToken := login(t, handler, "buyer@example.test")
	server := httptest.NewServer(handler)
	defer server.Close()

	streamContext, cancelStream := context.WithCancel(context.Background())
	streamRequest, err := http.NewRequestWithContext(streamContext, http.MethodGet,
		server.URL+"/api/v1/rfqs/"+rfqID+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamRequest.Header.Set("Authorization", "Bearer "+buyerToken)
	streamResponse, err := server.Client().Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	if streamResponse.StatusCode != http.StatusOK || streamResponse.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream response = %d %s", streamResponse.StatusCode, streamResponse.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(streamResponse.Body)
	if line := readSSELine(t, reader); line != ": connected" {
		t.Fatalf("first SSE line = %q", line)
	}
	_ = readSSELine(t, reader)

	healthResponse, err := server.Client().Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, healthResponse.Body)
	healthResponse.Body.Close()
	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("health while streaming = %d", healthResponse.StatusCode)
	}

	quote := validBidRequest("700000", "150000", "100000")
	response := performBidRequest(t, handler, "/api/v1/rfqs/"+rfqID+"/bids", supplierToken,
		"00000000-0000-4000-8000-000000000601", quote)
	assertStatus(t, response, http.StatusCreated)
	if line := readSSELine(t, reader); line != "event: rfq_changed" {
		t.Fatalf("event line = %q", line)
	}
	dataLine := readSSELine(t, reader)
	if !strings.Contains(dataLine, `"rfq_id":"`+rfqID+`"`) || !strings.Contains(dataLine, `"reason":"BID_ACCEPTED"`) {
		t.Fatalf("data line = %q", dataLine)
	}
	_ = readSSELine(t, reader)

	eventsResponse := performRequest(t, handler, http.MethodGet,
		"/api/v1/rfqs/"+rfqID+"/events?page=1&page_size=10", buyerToken, nil)
	assertStatus(t, eventsResponse, http.StatusOK)
	var events struct {
		Items []struct {
			ID    string         `json:"id"`
			BidID *string        `json:"bid_id"`
			Type  string         `json:"event_type"`
			Data  map[string]any `json:"event_data"`
		} `json:"items"`
		Total int64 `json:"total_items"`
	}
	decodeResponse(t, eventsResponse, &events)
	if events.Total != 3 || len(events.Items) != 3 {
		t.Fatalf("events = %+v", events)
	}
	if events.Items[0].Type != "AUCTION_EXTENDED" || events.Items[1].Type != "BID_ACCEPTED" || events.Items[2].Type != "RFQ_CREATED" {
		t.Fatalf("event order = %+v", events.Items)
	}
	if events.Items[0].ID == "" || events.Items[0].BidID == nil ||
		events.Items[0].Data["old_close_at"] == nil || events.Items[0].Data["new_close_at"] == nil ||
		events.Items[0].Data["trigger_type"] != "ANY_BID" {
		t.Fatalf("extension metadata = %+v", events.Items[0])
	}

	probe, cancelProbe := hub.Subscribe(rfqID)
	response = performBidRequest(t, handler, "/api/v1/rfqs/"+rfqID+"/bids", supplierToken,
		"00000000-0000-4000-8000-000000000602", quote)
	assertErrorCode(t, response, http.StatusConflict, "BID_NOT_IMPROVED")
	select {
	case message := <-probe:
		t.Fatalf("rejected bid emitted signal: %+v", message)
	default:
	}
	cancelProbe()

	cancelStream()
	streamResponse.Body.Close()
	deadline := time.Now().Add(time.Second)
	for hub.SubscriberCount(rfqID) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if count := hub.SubscriberCount(rfqID); count != 0 {
		t.Fatalf("subscriber leak: %d", count)
	}
}

func TestStreamRequiresAuthentication(t *testing.T) {
	resetAuctionData(t)
	rfqID, _, _ := insertActiveAuction(t, "EVENT-SSE-AUTH", "ANY_BID", time.Hour, 2*time.Hour)
	handler, _ := newTestApplication(t)
	response := performRequest(t, handler, http.MethodGet, "/api/v1/rfqs/"+rfqID+"/stream", "", nil)
	assertErrorCode(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func readSSELine(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	resultChannel := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		resultChannel <- result{line: strings.TrimSuffix(line, "\n"), err: err}
	}()
	select {
	case value := <-resultChannel:
		if value.err != nil {
			t.Fatalf("read SSE: %v", value.err)
		}
		return strings.TrimSuffix(value.line, "\r")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE line")
		return ""
	}
}
