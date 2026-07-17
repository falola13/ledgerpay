package wallets

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/falola13/ledgerpay/internal/account"
	"github.com/falola13/ledgerpay/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCannotOverdraw answers one business question:
// "If the wallet only has 100 cents, charging 500 must fail AND write nothing."
//
// Needs your Docker Postgres running (docker compose up).
// Connection URL: env DATABASE_URL, or the local compose default on port 5436.
func TestCannotOverdraw(t *testing.T) {
	ctx := context.Background()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		// Host machine → published compose port (see docker-compose.yml: "5436:5432")
		url = "postgres://postgres:dev@localhost:5436/ledgerpay"
	}

	pool, err := database.NewPool(ctx, url)
	if err != nil {
		t.Skipf("postgres not available (start docker compose): %v", err)
	}
	defer pool.Close()

	accounts := account.NewStore(pool)
	store := NewStore(pool)

	// 1. Fresh wallet (unique email so re-runs don't collide)
	_, wallet, err := accounts.Create(ctx, "overdraw-"+uuid.NewString()+"@test.local")
	if err != nil {
		t.Fatalf("create account/wallet: %v", err)
	}

	// 2. Fund exactly 100 cents
	if err := store.FundWallet(ctx, wallet.ID, 100); err != nil {
		t.Fatalf("fund 100: %v", err)
	}

	// 3. Snapshot row counts BEFORE the bad charge
	transfersBefore := countRows(t, ctx, pool, `SELECT count(*) FROM transfers WHERE wallet_id = $1`, wallet.ID)
	ledgerBefore := countRows(t, ctx, pool, `SELECT count(*) FROM ledger_entries WHERE wallet_id = $1`, wallet.ID)

	// 4. Try to charge 500 — must fail
	_, err = store.Charges(ctx, uuid.NewString(), ChargesType{
		WalletID:    wallet.ID,
		AmountCents: 500,
		Currency:    "USD",
	}, "test-overdraw-"+uuid.NewString(), "dummy-hash")
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("want ErrInsufficientFunds, got %v", err)
	}

	// 5. Prove nothing was written for that attempt
	transfersAfter := countRows(t, ctx, pool, `SELECT count(*) FROM transfers WHERE wallet_id = $1`, wallet.ID)
	ledgerAfter := countRows(t, ctx, pool, `SELECT count(*) FROM ledger_entries WHERE wallet_id = $1`, wallet.ID)

	if transfersAfter != transfersBefore {
		t.Fatalf("transfers changed on failed charge: before=%d after=%d", transfersBefore, transfersAfter)
	}
	if ledgerAfter != ledgerBefore {
		t.Fatalf("ledger_entries changed on failed charge: before=%d after=%d", ledgerBefore, ledgerAfter)
	}
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count query: %v", err)
	}
	return n
}
