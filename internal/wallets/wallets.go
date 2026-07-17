package wallets

import "time"

type Wallet struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChargesType struct {
	WalletID    string `json:"wallet_id"`
	AmountCents int    `json:"amount_cents"`
	Currency    string `json:"currency"`
}

type Transfer struct {
	ID          string    `json:"id"`
	WalletID    string    `json:"wallet_id"`
	AmountCents int       `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type IdempotentKey struct {
	Key            string          `json:"key"`
	RequestHash    string          `json:"request_hash"`
	ResponseStatus *int            `json:"response_status"`
	ResponseBody   *map[string]any `json:"response_body"`
	CreatedAt      time.Time       `json:"created_at"`
}
