package storage

import (
	"context"
	"database/sql"
	"valiant/internal/domain"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	// TODO: Implement actual SQL insert
	return nil
}

func (s *PostgresStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, error) {
	// TODO: Implement actual SQL query
	return []domain.ChangeEvent{}, nil
}

func (s *PostgresStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	// TODO: Implement actual SQL query
	return domain.ChangeEvent{}, nil
}
