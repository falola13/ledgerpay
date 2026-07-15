package wallets

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

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
	
	`, uuid.NewString(), transfer_id, "wal_system", amount)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// 4d493ae0-4baa-48a4-b286-fdb54b36df18
	return nil
}
