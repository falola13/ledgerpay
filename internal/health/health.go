package health

import (
	"context"
	"net/http"
	"time"

	"github.com/falola13/ledgerpay/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Checker struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Checker {
	return &Checker{
		db: db,
	}
}

func (c *Checker) Live(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (c *Checker) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)

	if err := c.db.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
	}

	if len(checks) > 0 {
		httpx.WriteJSON(w, 503, map[string]any{
			"status": "unavailable",
			"checks": checks,
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"checks": map[string]string{
			"postgres": "ok",
		},
	})
}
