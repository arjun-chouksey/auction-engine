//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

func TestAnyBidRepeatedExtensionAndAudit(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, initialClose, _ := insertActiveAuction(t, "EXT-ANY-BID", domain.TriggerAnyBid, 4*time.Minute, time.Hour)
	supplier := userID(t, "supplier.a@example.test")

	first, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 1, 900000))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Extended || first.ExtensionReason == nil || *first.ExtensionReason != domain.TriggerAnyBid ||
		first.PreviousCloseAt == nil || !first.CurrentCloseAt.Equal(initialClose.Add(5*time.Minute)) {
		t.Fatalf("first extension = %+v", first)
	}
	second, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 2, 800000))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Extended || !second.CurrentCloseAt.Equal(initialClose.Add(10*time.Minute)) {
		t.Fatalf("repeated extension = %+v", second)
	}
	assertExtensionState(t, rfqID, initialClose.Add(10*time.Minute), 2)

	replay, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 1, 900000))
	if err != nil || !replay.IdempotentReplay || replay.Extended {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	assertExtensionState(t, rfqID, initialClose.Add(10*time.Minute), 2)
}

func TestExtensionClipsAtForcedCloseWithoutFalseEvent(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, _, forcedClose := insertActiveAuction(t, "EXT-CAP", domain.TriggerAnyBid, 4*time.Minute, 7*time.Minute)
	supplier := userID(t, "supplier.a@example.test")

	first, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 10, 900000))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Extended || !first.CurrentCloseAt.Equal(forcedClose) {
		t.Fatalf("capped extension = %+v", first)
	}
	second, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 11, 800000))
	if err != nil {
		t.Fatal(err)
	}
	if second.Extended || !second.CurrentCloseAt.Equal(forcedClose) {
		t.Fatalf("bid at capped close before deadline = %+v", second)
	}
	assertExtensionState(t, rfqID, forcedClose, 1)
}

func TestL1ChangePolicy(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, initialClose, _ := insertActiveAuction(t, "EXT-L1", domain.TriggerL1Change, 4*time.Minute, time.Hour)
	supplierA := userID(t, "supplier.a@example.test")
	supplierB := userID(t, "supplier.b@example.test")

	first, err := repository.Submit(context.Background(), bidParams(rfqID, supplierA, 20, 900000))
	if err != nil || first.Extended {
		t.Fatalf("initial L1 assignment = %+v, %v", first, err)
	}
	second, err := repository.Submit(context.Background(), bidParams(rfqID, supplierB, 21, 950000))
	if err != nil || second.Extended {
		t.Fatalf("unchanged L1 = %+v, %v", second, err)
	}
	third, err := repository.Submit(context.Background(), bidParams(rfqID, supplierB, 22, 850000))
	if err != nil || !third.Extended || !third.CurrentCloseAt.Equal(initialClose.Add(5*time.Minute)) {
		t.Fatalf("changed L1 = %+v, %v", third, err)
	}
	assertExtensionState(t, rfqID, initialClose.Add(5*time.Minute), 1)
}

func TestAnyRankChangePolicyUsesTransactionalPreAndPostRanks(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, initialClose, _ := insertActiveAuction(t, "EXT-ANY-RANK", domain.TriggerAnyRankChange, 4*time.Minute, time.Hour)
	supplierA := userID(t, "supplier.a@example.test")
	supplierB := userID(t, "supplier.b@example.test")

	first, err := repository.Submit(context.Background(), bidParams(rfqID, supplierA, 23, 900000))
	if err != nil || !first.Extended {
		t.Fatalf("new supplier did not change rank set: %+v, %v", first, err)
	}
	second, err := repository.Submit(context.Background(), bidParams(rfqID, supplierB, 24, 800000))
	if err != nil || !second.Extended {
		t.Fatalf("new leader did not change rank order: %+v, %v", second, err)
	}
	third, err := repository.Submit(context.Background(), bidParams(rfqID, supplierA, 25, 850000))
	if err != nil || third.Extended {
		t.Fatalf("same rank order should not extend: %+v, %v", third, err)
	}
	assertExtensionState(t, rfqID, initialClose.Add(10*time.Minute), 2)
}

func TestConcurrentExtensionBidsSerialize(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, initialClose, _ := insertActiveAuction(t, "RACE-EXT", domain.TriggerAnyBid, 4*time.Minute, time.Hour)
	suppliers := []string{userID(t, "supplier.a@example.test"), userID(t, "supplier.b@example.test")}

	results := make(chan postgresrepository.SubmitBidResult, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for index, supplier := range suppliers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 30+index, int64(900000+index*10000)))
			results <- result
			errs <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for result := range results {
		if !result.Extended {
			t.Fatalf("concurrent result did not extend: %+v", result)
		}
	}
	assertExtensionState(t, rfqID, initialClose.Add(10*time.Minute), 2)
}

func TestConcurrentSameSupplierPreservesStrictImprovement(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, _, _ := insertActiveAuction(t, "RACE-SUPPLIER", domain.TriggerL1Change, 30*time.Minute, time.Hour)
	supplier := userID(t, "supplier.a@example.test")

	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for index, total := range []int64{900000, 800000} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := repository.Submit(context.Background(), bidParams(rfqID, supplier, 40+index, total))
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	accepted := 0
	for err := range errs {
		if err == nil {
			accepted++
			continue
		}
		if !errors.Is(err, postgresrepository.ErrBidNotImproved) {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if accepted < 1 || accepted > 2 {
		t.Fatalf("accepted count = %d", accepted)
	}
	var latest int64
	if err := testPool.QueryRow(context.Background(), `
		SELECT total_minor FROM bids WHERE rfq_id=$1 AND supplier_id=$2
		ORDER BY submitted_at DESC, id DESC LIMIT 1
	`, rfqID, supplier).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if latest != 800000 {
		t.Fatalf("latest total = %d", latest)
	}
}

func TestPostLockDatabaseTimeRejectsClosedAuction(t *testing.T) {
	resetAuctionData(t)
	repository := postgresrepository.NewBids(testPool)
	rfqID, _, _ := insertActiveAuction(t, "RACE-POST-LOCK", domain.TriggerAnyBid, 30*time.Minute, time.Hour)
	supplier := userID(t, "supplier.a@example.test")
	ctx := context.Background()

	blocker, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `SELECT id FROM rfqs WHERE id=$1 FOR UPDATE`, rfqID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := repository.Submit(ctx, bidParams(rfqID, supplier, 50, 900000))
		done <- err
	}()
	waitForRFQLockWaiter(t)
	if _, err := blocker.Exec(ctx, `UPDATE rfqs SET current_close_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, rfqID); err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, postgresrepository.ErrAuctionNotActive) {
		t.Fatalf("post-lock result = %v", err)
	}
	var count int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM bids WHERE rfq_id=$1`, rfqID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("accepted %d bids after close", count)
	}
}

func resetAuctionData(t *testing.T) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `TRUNCATE auction_events, bids, rfqs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
}

func insertActiveAuction(t *testing.T, reference string, trigger domain.TriggerType, closeAfter, forcedAfter time.Duration) (string, time.Time, time.Time) {
	t.Helper()
	var id string
	var closeAt, forcedAt time.Time
	err := testPool.QueryRow(context.Background(), `
		WITH t AS (SELECT clock_timestamp() AS now)
		INSERT INTO rfqs (
			reference_code, name, buyer_id, currency_code, service_date, bid_start_at,
			initial_close_at, current_close_at, forced_close_at,
			trigger_window_minutes, extension_minutes, trigger_type
		) SELECT $1, 'Auction engine test', u.id, 'INR', current_date + 10,
		         t.now - interval '2 hours', t.now - interval '1 hour', t.now + $2::interval,
		         t.now + $3::interval, 10, 5, $4
		FROM users u CROSS JOIN t WHERE u.email='buyer@example.test'
		RETURNING id, current_close_at, forced_close_at
	`, reference, durationInterval(closeAfter), durationInterval(forcedAfter), trigger).Scan(&id, &closeAt, &forcedAt)
	if err != nil {
		t.Fatal(err)
	}
	return id, closeAt, forcedAt
}

func durationInterval(value time.Duration) string {
	return fmt.Sprintf("%f seconds", value.Seconds())
}

func userID(t *testing.T, email string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM users WHERE email=$1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func bidParams(rfqID, supplierID string, keyNumber int, total int64) postgresrepository.SubmitBidParams {
	return postgresrepository.SubmitBidParams{
		RFQID: rfqID, SupplierID: supplierID,
		IdempotencyKey: fmt.Sprintf("00000000-0000-4000-8000-%012d", keyNumber),
		CarrierName:    "Auction Test Carrier", FreightMinor: total,
		OriginMinor: 0, DestinationMinor: 0, TransitTimeHours: 24,
		QuoteValidUntil: time.Now().UTC().AddDate(0, 1, 0),
	}
}

func assertExtensionState(t *testing.T, rfqID string, wantClose time.Time, wantEvents int) {
	t.Helper()
	var closeAt time.Time
	var events int
	if err := testPool.QueryRow(context.Background(), `SELECT current_close_at FROM rfqs WHERE id=$1`, rfqID).Scan(&closeAt); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM auction_events WHERE rfq_id=$1 AND event_type='AUCTION_EXTENDED'`, rfqID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if !closeAt.Equal(wantClose) || events != wantEvents {
		t.Fatalf("extension state = close:%s events:%d, want close:%s events:%d", closeAt, events, wantClose, wantEvents)
	}
}

func waitForRFQLockWaiter(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		err := testPool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname=current_database() AND wait_event_type='Lock'
				  AND query LIKE '%FROM rfqs WHERE id = $1 FOR UPDATE%'
			)
		`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("bid submission did not wait for RFQ row lock")
}
