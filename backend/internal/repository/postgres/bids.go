package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auction"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAuctionNotActive     = errors.New("auction is not active")
	ErrAuctionNotStarted    = errors.New("auction has not started")
	ErrBidNotImproved       = errors.New("bid is not an improvement")
	ErrIdempotencyKeyReused = errors.New("idempotency key reused with different payload")
)

type SubmitBidParams struct {
	RFQID            string
	SupplierID       string
	IdempotencyKey   string
	CarrierName      string
	FreightMinor     int64
	OriginMinor      int64
	DestinationMinor int64
	TransitTimeHours int
	QuoteValidUntil  time.Time
}

type SubmitBidResult struct {
	Bid              domain.Bid
	CurrentRank      int
	CurrentCloseAt   time.Time
	ServerTime       time.Time
	IdempotentReplay bool
	Extended         bool
	ExtensionReason  *domain.TriggerType
	PreviousCloseAt  *time.Time
}

type BidPage struct {
	Items      []domain.Bid
	TotalItems int64
	ServerTime time.Time
}

type Bids struct {
	pool *pgxpool.Pool
}

func NewBids(pool *pgxpool.Pool) *Bids { return &Bids{pool: pool} }

func (repository *Bids) Submit(ctx context.Context, params SubmitBidParams) (SubmitBidResult, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return SubmitBidResult{}, fmt.Errorf("begin bid transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var rfq domain.RFQ
	if err := tx.QueryRow(ctx, `
		SELECT id, service_date, bid_start_at, current_close_at, forced_close_at,
		       trigger_window_minutes, extension_minutes, trigger_type
		FROM rfqs WHERE id = $1 FOR UPDATE
	`, params.RFQID).Scan(&rfq.ID, &rfq.ServiceDate, &rfq.BidStartAt, &rfq.CurrentCloseAt,
		&rfq.ForcedCloseAt, &rfq.TriggerWindowMinutes, &rfq.ExtensionMinutes, &rfq.TriggerType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmitBidResult{}, ErrNotFound
		}
		return SubmitBidResult{}, fmt.Errorf("lock RFQ: %w", err)
	}

	stored, err := findBidByKey(ctx, tx, params.RFQID, params.SupplierID, params.IdempotencyKey)
	if err == nil {
		if !samePayload(stored, params) {
			return SubmitBidResult{}, ErrIdempotencyKeyReused
		}
		now, rank, err := bidReplayState(ctx, tx, stored)
		if err != nil {
			return SubmitBidResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return SubmitBidResult{}, fmt.Errorf("commit bid replay: %w", err)
		}
		return SubmitBidResult{Bid: stored, CurrentRank: rank, CurrentCloseAt: rfq.CurrentCloseAt, ServerTime: now, IdempotentReplay: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SubmitBidResult{}, fmt.Errorf("find idempotent bid: %w", err)
	}

	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return SubmitBidResult{}, fmt.Errorf("read authoritative bid time: %w", err)
	}
	if now.Before(rfq.BidStartAt) {
		return SubmitBidResult{}, ErrAuctionNotStarted
	}
	if !now.Before(rfq.CurrentCloseAt) || !now.Before(rfq.ForcedCloseAt) {
		return SubmitBidResult{}, ErrAuctionNotActive
	}
	if params.QuoteValidUntil.Before(rfq.ServiceDate) {
		return SubmitBidResult{}, ErrQuoteValidity
	}

	var latestTotal int64
	err = tx.QueryRow(ctx, `
		SELECT total_minor FROM bids
		WHERE rfq_id = $1 AND supplier_id = $2
		ORDER BY submitted_at DESC, id DESC LIMIT 1
	`, params.RFQID, params.SupplierID).Scan(&latestTotal)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SubmitBidResult{}, fmt.Errorf("read supplier latest bid: %w", err)
	}
	total, err := domain.SumCharges(params.FreightMinor, params.OriginMinor, params.DestinationMinor)
	if err != nil || total <= 0 {
		return SubmitBidResult{}, ErrInvalidBidTotal
	}
	if hasLatest && latestTotal <= total {
		return SubmitBidResult{}, ErrBidNotImproved
	}
	before, err := rankingInTransaction(ctx, tx, params.RFQID)
	if err != nil {
		return SubmitBidResult{}, err
	}

	var bid domain.Bid
	err = tx.QueryRow(ctx, `
		INSERT INTO bids (
			rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
			origin_minor, destination_minor, transit_time_hours, quote_valid_until, submitted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
		          origin_minor, destination_minor, total_minor, transit_time_hours,
		          quote_valid_until, submitted_at
	`, params.RFQID, params.SupplierID, params.IdempotencyKey, params.CarrierName,
		params.FreightMinor, params.OriginMinor, params.DestinationMinor,
		params.TransitTimeHours, params.QuoteValidUntil, now).Scan(bidDestinations(&bid)...)
	if err != nil {
		return SubmitBidResult{}, fmt.Errorf("insert bid: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auction_events (rfq_id, bid_id, actor_id, event_type, event_data, created_at)
		VALUES ($1, $2, $3, 'BID_ACCEPTED', jsonb_build_object('total_minor', $4::bigint), $5)
	`, params.RFQID, bid.ID, params.SupplierID, bid.TotalMinor, now); err != nil {
		return SubmitBidResult{}, fmt.Errorf("insert bid accepted event: %w", err)
	}
	after, err := rankingInTransaction(ctx, tx, params.RFQID)
	if err != nil {
		return SubmitBidResult{}, err
	}
	rank := rankForSupplier(after, params.SupplierID)
	closeUpdate := auction.ComputeExtension(now, rfq, before, after)
	var reason *domain.TriggerType
	var previousClose *time.Time
	if closeUpdate.Extended {
		if _, err := tx.Exec(ctx, `
			UPDATE rfqs SET current_close_at = $2, updated_at = $3 WHERE id = $1
		`, params.RFQID, closeUpdate.NewClose, now); err != nil {
			return SubmitBidResult{}, fmt.Errorf("update RFQ close: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO auction_events (rfq_id, bid_id, actor_id, event_type, event_data, created_at)
			VALUES ($1, $2, $3, 'AUCTION_EXTENDED', jsonb_build_object(
				'trigger_type', $4::text, 'old_close_at', $5::timestamptz,
				'new_close_at', $6::timestamptz, 'trigger_window_minutes', $7::integer,
				'extension_minutes', $8::integer, 'before_ranking', $9::text[],
				'after_ranking', $10::text[]
			), $11)
		`, params.RFQID, bid.ID, params.SupplierID, closeUpdate.Reason,
			closeUpdate.OldClose, closeUpdate.NewClose, rfq.TriggerWindowMinutes,
			rfq.ExtensionMinutes, supplierIDs(before), supplierIDs(after), now); err != nil {
			return SubmitBidResult{}, fmt.Errorf("insert auction extended event: %w", err)
		}
		rfq.CurrentCloseAt = closeUpdate.NewClose
		reasonValue := closeUpdate.Reason
		reason = &reasonValue
		previousValue := closeUpdate.OldClose
		previousClose = &previousValue
	}
	if err := tx.Commit(ctx); err != nil {
		return SubmitBidResult{}, fmt.Errorf("commit bid: %w", err)
	}
	return SubmitBidResult{
		Bid: bid, CurrentRank: rank, CurrentCloseAt: rfq.CurrentCloseAt, ServerTime: now,
		Extended: closeUpdate.Extended, ExtensionReason: reason, PreviousCloseAt: previousClose,
	}, nil
}

var (
	ErrQuoteValidity   = errors.New("quote validity precedes service date")
	ErrInvalidBidTotal = errors.New("invalid bid total")
)

func (repository *Bids) Ranking(ctx context.Context, rfqID string) ([]domain.RankedBid, time.Time, error) {
	if err := repository.ensureRFQ(ctx, rfqID); err != nil {
		return nil, time.Time{}, err
	}
	var now time.Time
	if err := repository.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, time.Time{}, fmt.Errorf("read ranking timestamp: %w", err)
	}
	rows, err := repository.pool.Query(ctx, latestRankingSQL, rfqID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("query ranking: %w", err)
	}
	defer rows.Close()
	items := make([]domain.RankedBid, 0)
	for rows.Next() {
		var item domain.RankedBid
		destinations := append([]any{&item.Rank}, bidDestinations(&item.Bid)...)
		if err := rows.Scan(destinations...); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan ranking: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, fmt.Errorf("iterate ranking: %w", err)
	}
	return items, now, nil
}

func (repository *Bids) List(ctx context.Context, rfqID, sortOrder string, page, pageSize int) (BidPage, error) {
	if err := repository.ensureRFQ(ctx, rfqID); err != nil {
		return BidPage{}, err
	}
	var now time.Time
	if err := repository.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return BidPage{}, fmt.Errorf("read bid list timestamp: %w", err)
	}
	orderBy := `total_minor ASC, submitted_at ASC, id ASC`
	if sortOrder == "newest" {
		orderBy = `submitted_at DESC, id DESC`
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id, rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
		       origin_minor, destination_minor, total_minor, transit_time_hours,
		       quote_valid_until, submitted_at
		FROM bids WHERE rfq_id = $1 ORDER BY `+orderBy+` LIMIT $2 OFFSET $3
	`, rfqID, pageSize, (page-1)*pageSize)
	if err != nil {
		return BidPage{}, fmt.Errorf("list bids: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Bid, 0, pageSize)
	for rows.Next() {
		var bid domain.Bid
		if err := rows.Scan(bidDestinations(&bid)...); err != nil {
			return BidPage{}, fmt.Errorf("scan bid: %w", err)
		}
		items = append(items, bid)
	}
	if err := rows.Err(); err != nil {
		return BidPage{}, fmt.Errorf("iterate bids: %w", err)
	}
	var total int64
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM bids WHERE rfq_id = $1`, rfqID).Scan(&total); err != nil {
		return BidPage{}, fmt.Errorf("count bids: %w", err)
	}
	return BidPage{Items: items, TotalItems: total, ServerTime: now}, nil
}

func (repository *Bids) ensureRFQ(ctx context.Context, rfqID string) error {
	var exists bool
	if err := repository.pool.QueryRow(ctx, `SELECT true FROM rfqs WHERE id = $1`, rfqID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("find RFQ: %w", err)
	}
	return nil
}

const latestRankingSQL = `
	WITH latest AS (
		SELECT DISTINCT ON (supplier_id)
		       id, rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
		       origin_minor, destination_minor, total_minor, transit_time_hours,
		       quote_valid_until, submitted_at
		FROM bids WHERE rfq_id = $1
		ORDER BY supplier_id, submitted_at DESC, id DESC
	)
	SELECT row_number() OVER (ORDER BY total_minor, submitted_at, supplier_id) AS rank,
	       id, rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
	       origin_minor, destination_minor, total_minor, transit_time_hours,
	       quote_valid_until, submitted_at
	FROM latest ORDER BY total_minor, submitted_at, supplier_id`

func findBidByKey(ctx context.Context, tx pgx.Tx, rfqID, supplierID, key string) (domain.Bid, error) {
	var bid domain.Bid
	err := tx.QueryRow(ctx, `
		SELECT id, rfq_id, supplier_id, idempotency_key, carrier_name, freight_minor,
		       origin_minor, destination_minor, total_minor, transit_time_hours,
		       quote_valid_until, submitted_at
		FROM bids WHERE rfq_id = $1 AND supplier_id = $2 AND idempotency_key = $3
	`, rfqID, supplierID, key).Scan(bidDestinations(&bid)...)
	return bid, err
}

func bidReplayState(ctx context.Context, tx pgx.Tx, bid domain.Bid) (time.Time, int, error) {
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return time.Time{}, 0, fmt.Errorf("read replay timestamp: %w", err)
	}
	rank, err := currentRank(ctx, tx, bid.RFQID, bid.SupplierID)
	return now, rank, err
}

func currentRank(ctx context.Context, tx pgx.Tx, rfqID, supplierID string) (int, error) {
	var rank int
	if err := tx.QueryRow(ctx, `SELECT rank FROM (`+latestRankingSQL+`) ranked WHERE supplier_id = $2`, rfqID, supplierID).Scan(&rank); err != nil {
		return 0, fmt.Errorf("read current rank: %w", err)
	}
	return rank, nil
}

func rankingInTransaction(ctx context.Context, tx pgx.Tx, rfqID string) ([]domain.RankedBid, error) {
	rows, err := tx.Query(ctx, latestRankingSQL, rfqID)
	if err != nil {
		return nil, fmt.Errorf("query transactional ranking: %w", err)
	}
	defer rows.Close()
	items := make([]domain.RankedBid, 0)
	for rows.Next() {
		var item domain.RankedBid
		destinations := append([]any{&item.Rank}, bidDestinations(&item.Bid)...)
		if err := rows.Scan(destinations...); err != nil {
			return nil, fmt.Errorf("scan transactional ranking: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactional ranking: %w", err)
	}
	return items, nil
}

func rankForSupplier(ranking []domain.RankedBid, supplierID string) int {
	for _, item := range ranking {
		if item.Bid.SupplierID == supplierID {
			return item.Rank
		}
	}
	return 0
}

func supplierIDs(ranking []domain.RankedBid) []string {
	ids := make([]string, len(ranking))
	for index, item := range ranking {
		ids[index] = item.Bid.SupplierID
	}
	return ids
}

func samePayload(bid domain.Bid, params SubmitBidParams) bool {
	return bid.CarrierName == params.CarrierName && bid.FreightMinor == params.FreightMinor &&
		bid.OriginMinor == params.OriginMinor && bid.DestinationMinor == params.DestinationMinor &&
		bid.TransitTimeHours == params.TransitTimeHours &&
		bid.QuoteValidUntil.Format(time.DateOnly) == params.QuoteValidUntil.Format(time.DateOnly)
}

func bidDestinations(bid *domain.Bid) []any {
	return []any{&bid.ID, &bid.RFQID, &bid.SupplierID, &bid.IdempotencyKey, &bid.CarrierName,
		&bid.FreightMinor, &bid.OriginMinor, &bid.DestinationMinor, &bid.TotalMinor,
		&bid.TransitTimeHours, &bid.QuoteValidUntil, &bid.SubmittedAt}
}
