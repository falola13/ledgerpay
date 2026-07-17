package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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

type Schedule map[int]time.Duration

var RetryBackup = Schedule{
	0: time.Second,
	1: 10 * time.Second,
	2: 30 * time.Second,
	3: 100 * time.Second,
}

type Para struct {
	secret string
	url    string
}

func NewPara(secret, url string) *Para {
	return &Para{secret: secret, url: url}
}

func NextRetry(attempts int) (time.Duration, error) {
	if d, ok := RetryBackup[attempts]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("Out of schedule")
}

const EventType = "transfer.succeeded"

func main() {
	_ = godotenv.Load()
	con := config.Load()
	secret := con.WEBHOOK_SECRET
	Url := con.WEBHOOK_URL

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

	handler := NewPara(secret, Url)

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

		n, err := handler.processBatch(ctx, pool)
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

func (p *Para) deliverEvent(ctx context.Context, v Events) error {

	bodyBytes, err := json.Marshal(v.Payload)
	if err != nil {
		slog.Warn("Marshal error", "error", err)
		return err
	}

	mac := hmac.New(sha256.New, []byte(p.secret))
	mac.Write(bodyBytes)
	stamp := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-LedgerPay-Signature", stamp)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		return fmt.Errorf("Network error")
	}

}

func (p *Para) processBatch(ctx context.Context, pool *pgxpool.Pool) (int, error) {
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
			if err := p.deliverEvent(ctx, event); err != nil {

				retry, retryerr := NextRetry(event.Attempts)
				nextRetry := time.Now().Add(retry)

				//Set status to dead after schedule timeout
				if retryerr != nil {
					status := "dead"
					_, err = tx.Exec(ctx, `
				UPDATE outbox_events 
				SET attempts = COALESCE($1,attempts),
					next_retry_at = COALESCE($2,next_retry_at),
					status = COALESCE($4,status),
					last_error = COALESCE($5,last_error)
				WHERE id = $3
				`, event.Attempts+1, nextRetry, event.ID, status, err.Error())

					if err != nil {
						return 0, fmt.Errorf("update-query: %w", err)
					}
					continue
				}

				// Update attempt and last error

				_, err = tx.Exec(ctx, `
				UPDATE outbox_events 
				SET attempts = COALESCE($1,attempts),
					next_retry_at = COALESCE($2,next_retry_at),
					last_error = COALESCE($4,last_error)
				WHERE id = $3
				`, event.Attempts+1, nextRetry, event.ID, err.Error())

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
