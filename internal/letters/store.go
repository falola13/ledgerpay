package letters

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) GetDeadLetters(ctx context.Context, status string) ([]DeadLetters, error) {

	rows, err := s.db.Query(ctx, `
		SELECT id, status,attempts,next_retry_at,last_error,created_at
		FROM outbox_events
		WHERE status = $1
	`, status)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	letters := make([]DeadLetters, 0)

	for rows.Next() {
		var d DeadLetters

		if err := rows.Scan(&d.ID, &d.Status, &d.Attempts, &d.NextRetryAt, &d.LastError, &d.CreatedAt); err != nil {
			return nil, err
		}

		letters = append(letters, d)

	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return letters, nil
}
