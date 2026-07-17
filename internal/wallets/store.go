package wallets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInsufficientFunds means the wallet does not have enough balance for the charge.
// Returning a named error lets tests (and the handler) check with errors.Is.
var ErrInsufficientFunds = errors.New("insufficient_funds")

// ErrIdempotencyConflict means another request already claimed this Idempotency-Key
// (unique constraint on key). Caller should look up and replay the winner's response.
var ErrIdempotencyConflict = errors.New("idempotency_conflict")

type Store struct {
	db *pgxpool.Pool
}

const WalletID = "wal_system"
const EventType = "transfer.succeeded"

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) GetWallet(ctx context.Context, id string) (Wallet, int64, error) {

	var w Wallet
	var balance int64

	err := s.db.QueryRow(ctx, `
		SELECT id,account_id,currency,created_at,updated_at
		FROM wallets
		WHERE id = $1
	`, id).Scan(&w.ID, &w.AccountID, &w.Currency, &w.CreatedAt, &w.UpdatedAt)

	if err != nil {
		return Wallet{}, 0, err
	}

	err2 := s.db.QueryRow(ctx, `
	SELECT 
		COALESCE (
			SUM (
					CASE 
						WHEN direction = 'credit' THEN amount_cents
						WHEN direction = 'debit' THEN -amount_cents
					END
				),
					0
			) AS balance  FROM ledger_entries WHERE wallet_id = $1 
	`, id).Scan(&balance)

	if err2 != nil {
		return Wallet{}, 0, err2
	}

	return w, balance, nil
}

func (s *Store) FundWallet(ctx context.Context, id string, amount int) error {

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	transfer_id := uuid.NewString()

	_, err = tx.Exec(ctx, `
	 
		INSERT INTO ledger_entries (id,transfer_id, wallet_id,amount_cents,direction)
		VALUES ($1,$2,$3,$4,'credit')
		
	
	`, uuid.NewString(), transfer_id, id, amount)

	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
	 
		
		INSERT INTO ledger_entries (id,transfer_id, wallet_id,amount_cents,direction)
		VALUES ($1,$2,$3,$4,'debit')
	
	`, uuid.NewString(), transfer_id, WalletID, amount)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// 4d493ae0-4baa-48a4-b286-fdb54b36df18
	return nil
}

// Charges runs the charge and inserts the idempotency row in the same transaction.
// On success it returns the exact response body that was stored (so first reply == replay).
func (s *Store) Charges(ctx context.Context, transfer_id string, c ChargesType, key string, request_hash string) (map[string]any, error) {

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var balance int
	var t Transfer

	//Lock wallet row
	_, err = tx.Exec(ctx,
		`
			SELECT id FROM wallets WHERE id = $1 FOR UPDATE
		`, c.WalletID)
	if err != nil {
		return nil, err
	}

	// Aggregate balance
	err = tx.QueryRow(ctx,
		`
		SELECT
		COALESCE (
			SUM (
				CASE
					WHEN direction = 'credit' THEN amount_cents
					WHEN direction = 'debit' THEN -amount_cents
				END
			),
			0
			) AS balance FROM ledger_entries WHERE wallet_id = $1
	`, c.WalletID,
	).Scan(&balance)

	if err != nil {
		return nil, err
	}

	//Check if balance is less than the charge amount
	if balance < c.AmountCents {
		return nil, ErrInsufficientFunds
	}

	//Transfers
	err = tx.QueryRow(ctx, `
			INSERT INTO transfers (id,wallet_id,amount_cents,currency,status)
			VALUES ($1,$2,$3,$4,'succeeded')
			RETURNING id,wallet_id,amount_cents,currency,status,created_at
		`, transfer_id, c.WalletID, c.AmountCents, c.Currency).Scan(&t.ID, &t.WalletID, &t.AmountCents, &t.Currency, &t.Status, &t.CreatedAt)

	if err != nil {
		return nil, err
	}

	//Insert leg
	_, err = tx.Exec(ctx, `
	INSERT INTO ledger_entries (id,transfer_id,wallet_id,amount_cents,direction) 
	VALUES ($1,$2,$3,$4,'credit')
`, uuid.NewString(), transfer_id, WalletID, c.AmountCents)

	if err != nil {
		return nil, err
	}

	//Insert leg
	_, err = tx.Exec(ctx, `
	INSERT INTO ledger_entries (id,transfer_id,wallet_id,amount_cents,direction) 
	VALUES ($1,$2,$3,$4,'debit')
`, uuid.NewString(), transfer_id, c.WalletID, c.AmountCents)

	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"transfer_id":   t.ID,
		"status":        t.Status,
		"balance_cents": balance - c.AmountCents,
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (key,request_hash,response_status,response_body)
		VALUES ($1,$2,$3,$4)
	`, key, request_hash, httpStatusCreated, body)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrIdempotencyConflict
		}
		return nil, err
	}

	payload := map[string]any{
		"transfer_id": t.ID,
		"wallet_id":   t.WalletID,
		"amount":      t.AmountCents,
		"currency":    t.Currency,
		"timestamp":   t.CreatedAt,
	}
	//Inform outbox
	_, err = tx.Exec(ctx,
		`
			INSERT INTO outbox_events (id,event_type,payload)
			VALUES ($1,$2,$3)
		`, uuid.NewString(), EventType, payload)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return body, nil
}

const httpStatusCreated = 201

func (s *Store) FindIdempotent(ctx context.Context, key string) (IdempotentKey, bool, error) {
	var R IdempotentKey

	err := s.db.QueryRow(ctx, ` SELECT key,request_hash,response_status,response_body,created_at FROM idempotency_keys WHERE key = $1 `, key).Scan(&R.Key, &R.RequestHash, &R.ResponseStatus, &R.ResponseBody, &R.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return IdempotentKey{}, false, nil
	}
	if err != nil {
		return IdempotentKey{}, false, err
	}

	return R, true, nil
}
