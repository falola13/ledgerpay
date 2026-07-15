package wallets

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/falola13/ledgerpay/internal/httpx"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func validateAmount(amt int) error {
	if amt < 0 || amt == 0 {
		return fmt.Errorf("Amount needs to be greater than 0 ")
	}
	return nil
}

func (h *Handler) GetWalletById(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	wallet, balance, err := h.store.GetWallet(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Wallet not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"balance_cents": balance,
		"currency":      wallet.Currency,
		"wallet":        wallet,
	})
}

func (h *Handler) FundWallet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in struct {
		AmountCents int `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid json")
		return
	}

	if err := validateAmount(in.AmountCents); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.FundWallet(r.Context(), id, in.AmountCents); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"wallet": "Wallet funded",
	})
}
