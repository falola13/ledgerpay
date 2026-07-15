package account

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

func (s *Store) Create(ctx context.Context, email string) (Account, Wallet, error) {
	var a Account
	var w Wallet

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})

	if err != nil {
		return Account{}, Wallet{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`
		INSERT INTO accounts (id,email)
		VALUES ($1,$2)
		RETURNING id,email,created_at,updated_at
	`, uuid.NewString(), email).Scan(&a.ID, &a.Email, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Account{}, Wallet{}, err
	}

	err = tx.QueryRow(ctx, `
	INSERT INTO wallets (id,account_id)
	VALUES ($1, $2)
	RETURNING id, account_id,currency, created_at,updated_at
	`, uuid.NewString(), a.ID).Scan(&w.ID, &w.AccountID, &w.Currency, &w.CreatedAt, &w.UpdatedAt)

	if err != nil {
		return Account{}, Wallet{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Account{}, Wallet{}, err
	}

	return a, w, nil
}
