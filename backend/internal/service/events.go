package service

import (
	"context"
	"errors"
	"fmt"

	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

type EventRepository interface {
	List(context.Context, string, int, int) (postgresrepository.EventPage, error)
}

type Events struct{ repository EventRepository }

func NewEvents(repository EventRepository) *Events { return &Events{repository: repository} }

func (service *Events) List(ctx context.Context, rfqID string, page, pageSize int) (postgresrepository.EventPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return postgresrepository.EventPage{}, ValidationError{Message: "page must be at least 1 and page_size must be between 1 and 100"}
	}
	result, err := service.repository.List(ctx, rfqID, page, pageSize)
	if errors.Is(err, postgresrepository.ErrNotFound) {
		return postgresrepository.EventPage{}, ErrNotFound
	}
	if err != nil {
		return postgresrepository.EventPage{}, fmt.Errorf("list events: %w", err)
	}
	return result, nil
}
