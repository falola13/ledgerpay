package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/falola13/ledgerpay/internal/config"
	"github.com/falola13/ledgerpay/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Events struct {
	ID          string         `json:"id"`
	EventType   string         `json:"event_type"`
	Payload     map[string]any `json:"payload"`
	Status      string         `json:"status"`
	Attempts    int            `json:"attempts"`
	NextRetryAt time.Time      `json:"next_retry_at"`
	CreatedAt   time.Time      `json:"created_at"`
}

const EventType = "transfer.succeeded"

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	config := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, config.DatabaseURL)
	if err != nil {
		slog.Error("Database could not be connected")
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("✅ Worker started, waiting for jobs")

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("worker shutting down")
			return
		default:
		}

		n, err := processBatch(ctx, pool)
		if err != nil {
			slog.Error("Batch failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if n == 0 {
			time.Sleep(4 * time.Second)
		}

	}
}

func deliverEvent(v Events) error {

	if rand.Float64() < 0 {
		return fmt.Errorf("Failed to send event")
	}

	slog.Info("delivering event", "event-type", v.EventType, "status", "delivered", "payload", v.Payload)
	return nil
}

func processBatch(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("BeginTx : %w", err)

	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
				SELECT id,event_type,payload,attempts FROM outbox_events
				WHERE status = 'pending' AND next_retry_at <= now()
				ORDER BY created_at
				LIMIT 10
				FOR UPDATE SKIP LOCKED
			`)
	if err != nil {
		return 0, fmt.Errorf("error : %w", err)
	}

	events := make([]Events, 0)
	for rows.Next() {
		var e Events

		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Attempts); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan : %w", err)
		}

		events = append(events, e)

	}

	if err := rows.Err(); err != nil {
		return 0, err
	}
	defer rows.Close()

	for _, event := range events {

		switch event.EventType {
		case EventType:
			if err := deliverEvent(event); err != nil {
				nextRetry := time.Now().Add(10 * time.Second)
				_, err = tx.Exec(ctx, `
				UPDATE outbox_events 
				SET attempts = COALESCE($1,attempts),
					next_retry_at = COALESCE($2,next_retry_at)
				WHERE id = $3
				`, event.Attempts+1, nextRetry, event.ID)

				if err != nil {
					return 0, fmt.Errorf("update-query: %w", err)
				}
				continue

			}

			//set to deliver
			_, err = tx.Exec(ctx, `
				UPDATE outbox_events 
				SET status = COALESCE($1,status)
				WHERE id = $2
				`, "delivered", event.ID)
			if err != nil {

				return 0, fmt.Errorf("update delivered: %w", err)
			}

		default:
			slog.Warn("unknown job type")
		}

	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(events), nil

}
