package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventPage struct {
	Items      []domain.AuctionEvent
	TotalItems int64
	ServerTime time.Time
}

type Events struct{ pool *pgxpool.Pool }

func NewEvents(pool *pgxpool.Pool) *Events { return &Events{pool: pool} }

func (repository *Events) List(ctx context.Context, rfqID string, page, pageSize int) (EventPage, error) {
	var serverTime time.Time
	if err := repository.pool.QueryRow(ctx, `
		SELECT clock_timestamp() FROM rfqs WHERE id=$1
	`, rfqID).Scan(&serverTime); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EventPage{}, ErrNotFound
		}
		return EventPage{}, fmt.Errorf("find RFQ for events: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id, rfq_id, bid_id, actor_id, event_type, event_data, created_at
		FROM auction_events WHERE rfq_id=$1
		ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3
	`, rfqID, pageSize, (page-1)*pageSize)
	if err != nil {
		return EventPage{}, fmt.Errorf("list auction events: %w", err)
	}
	defer rows.Close()
	items := make([]domain.AuctionEvent, 0, pageSize)
	for rows.Next() {
		var event domain.AuctionEvent
		var data []byte
		if err := rows.Scan(&event.ID, &event.RFQID, &event.BidID, &event.ActorID,
			&event.Type, &data, &event.CreatedAt); err != nil {
			return EventPage{}, fmt.Errorf("scan auction event: %w", err)
		}
		event.Data = json.RawMessage(data)
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterate auction events: %w", err)
	}
	var total int64
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM auction_events WHERE rfq_id=$1`, rfqID).Scan(&total); err != nil {
		return EventPage{}, fmt.Errorf("count auction events: %w", err)
	}
	return EventPage{Items: items, TotalItems: total, ServerTime: serverTime}, nil
}
