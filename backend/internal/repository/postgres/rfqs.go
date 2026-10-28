package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrReferenceExists = errors.New("RFQ reference exists")

type CreateRFQParams struct {
	ReferenceCode        string
	Name                 string
	BuyerID              string
	CurrencyCode         string
	ServiceDate          time.Time
	BidStartAt           time.Time
	InitialCloseAt       time.Time
	ForcedCloseAt        time.Time
	TriggerWindowMinutes int
	ExtensionMinutes     int
	TriggerType          domain.TriggerType
}

type RFQFilter struct {
	Page     int
	PageSize int
	Status   *domain.RFQStatus
}

type RFQPage struct {
	Items      []domain.RFQ
	TotalItems int64
	ServerTime time.Time
}

type RFQs struct {
	pool *pgxpool.Pool
}

func NewRFQs(pool *pgxpool.Pool) *RFQs {
	return &RFQs{pool: pool}
}

func (repository *RFQs) Create(ctx context.Context, params CreateRFQParams) (domain.RFQ, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return domain.RFQ{}, fmt.Errorf("begin create RFQ transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	var rfq domain.RFQ
	err = transaction.QueryRow(ctx, `
		INSERT INTO rfqs (
			reference_code, name, buyer_id, currency_code, service_date,
			bid_start_at, initial_close_at, current_close_at, forced_close_at,
			trigger_window_minutes, extension_minutes, trigger_type
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, $9, $10, $11)
		RETURNING id, reference_code, name, buyer_id, currency_code, service_date,
		          bid_start_at, initial_close_at, current_close_at, forced_close_at,
		          trigger_window_minutes, extension_minutes, trigger_type
	`, params.ReferenceCode, params.Name, params.BuyerID, params.CurrencyCode, params.ServiceDate,
		params.BidStartAt, params.InitialCloseAt, params.ForcedCloseAt,
		params.TriggerWindowMinutes, params.ExtensionMinutes, params.TriggerType).Scan(
		&rfq.ID, &rfq.ReferenceCode, &rfq.Name, &rfq.BuyerID, &rfq.CurrencyCode, &rfq.ServiceDate,
		&rfq.BidStartAt, &rfq.InitialCloseAt, &rfq.CurrentCloseAt, &rfq.ForcedCloseAt,
		&rfq.TriggerWindowMinutes, &rfq.ExtensionMinutes, &rfq.TriggerType,
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return domain.RFQ{}, ErrReferenceExists
		}
		return domain.RFQ{}, fmt.Errorf("insert RFQ: %w", err)
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO auction_events (rfq_id, actor_id, event_type, event_data, created_at)
		VALUES ($1, $2, 'RFQ_CREATED', jsonb_build_object('reference_code', $3::text), clock_timestamp())
	`, rfq.ID, params.BuyerID, rfq.ReferenceCode); err != nil {
		return domain.RFQ{}, fmt.Errorf("insert RFQ created event: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.RFQ{}, fmt.Errorf("commit create RFQ transaction: %w", err)
	}
	return repository.GetByID(ctx, rfq.ID)
}

func (repository *RFQs) GetByID(ctx context.Context, id string) (domain.RFQ, error) {
	var serverTime time.Time
	if err := repository.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&serverTime); err != nil {
		return domain.RFQ{}, fmt.Errorf("read RFQ timestamp: %w", err)
	}

	var rfq domain.RFQ
	err := repository.pool.QueryRow(ctx, rfqSelect(`$2::timestamptz`)+`
		WHERE r.id = $1
	`, id, serverTime).Scan(rfqScanDestinations(&rfq)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RFQ{}, ErrNotFound
		}
		return domain.RFQ{}, fmt.Errorf("get RFQ: %w", err)
	}
	return rfq, nil
}

func (repository *RFQs) List(ctx context.Context, filter RFQFilter) (RFQPage, error) {
	var serverTime time.Time
	if err := repository.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&serverTime); err != nil {
		return RFQPage{}, fmt.Errorf("read list timestamp: %w", err)
	}

	status := ""
	if filter.Status != nil {
		status = string(*filter.Status)
	}
	offset := (filter.Page - 1) * filter.PageSize
	rows, err := repository.pool.Query(ctx, rfqSelect(`$4::timestamptz`)+`
		WHERE ($1 = '' OR (`+statusExpression(`$4::timestamptz`)+`) = $1)
		ORDER BY r.created_at DESC, r.id
		LIMIT $2 OFFSET $3
	`, status, filter.PageSize, offset, serverTime)
	if err != nil {
		return RFQPage{}, fmt.Errorf("list RFQs: %w", err)
	}
	defer rows.Close()

	items := make([]domain.RFQ, 0, filter.PageSize)
	for rows.Next() {
		var rfq domain.RFQ
		if err := rows.Scan(rfqScanDestinations(&rfq)...); err != nil {
			return RFQPage{}, fmt.Errorf("scan listed RFQ: %w", err)
		}
		items = append(items, rfq)
	}
	if err := rows.Err(); err != nil {
		return RFQPage{}, fmt.Errorf("iterate listed RFQs: %w", err)
	}

	var totalItems int64
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*) FROM rfqs r
		WHERE ($1 = '' OR (`+statusExpression(`$2::timestamptz`)+`) = $1)
	`, status, serverTime).Scan(&totalItems); err != nil {
		return RFQPage{}, fmt.Errorf("count RFQs: %w", err)
	}
	return RFQPage{Items: items, TotalItems: totalItems, ServerTime: serverTime}, nil
}

func statusExpression(at string) string {
	return `CASE
	WHEN r.current_close_at = r.forced_close_at AND ` + at + ` >= r.forced_close_at THEN 'FORCE_CLOSED'
	WHEN ` + at + ` >= r.current_close_at THEN 'CLOSED'
	WHEN ` + at + ` < r.bid_start_at THEN 'UPCOMING'
	ELSE 'ACTIVE'
END`
}

func rfqSelect(serverTime string) string {
	return `
	WITH latest AS (
		SELECT DISTINCT ON (rfq_id, supplier_id) rfq_id, supplier_id, total_minor
		FROM bids
		ORDER BY rfq_id, supplier_id, submitted_at DESC, id DESC
	), aggregates AS (
		SELECT rfq_id, min(total_minor) AS lowest_bid_minor,
		       count(*) AS unique_supplier_count
		FROM latest GROUP BY rfq_id
	), bid_counts AS (
		SELECT rfq_id, count(*) AS total_bid_count FROM bids GROUP BY rfq_id
	)
	SELECT r.id, r.reference_code, r.name, r.buyer_id, r.currency_code, r.service_date,
	       r.bid_start_at, r.initial_close_at, r.current_close_at, r.forced_close_at,
	       r.trigger_window_minutes, r.extension_minutes, r.trigger_type,
	       a.lowest_bid_minor, COALESCE(bc.total_bid_count, 0),
	       COALESCE(a.unique_supplier_count, 0), ` + serverTime + `
	FROM rfqs r
	LEFT JOIN aggregates a ON a.rfq_id = r.id
	LEFT JOIN bid_counts bc ON bc.rfq_id = r.id
`
}

func rfqScanDestinations(rfq *domain.RFQ) []any {
	return []any{
		&rfq.ID, &rfq.ReferenceCode, &rfq.Name, &rfq.BuyerID, &rfq.CurrencyCode, &rfq.ServiceDate,
		&rfq.BidStartAt, &rfq.InitialCloseAt, &rfq.CurrentCloseAt, &rfq.ForcedCloseAt,
		&rfq.TriggerWindowMinutes, &rfq.ExtensionMinutes, &rfq.TriggerType,
		&rfq.LowestBidMinor, &rfq.TotalBidCount, &rfq.UniqueSupplierCount, &rfq.ServerTime,
	}
}
