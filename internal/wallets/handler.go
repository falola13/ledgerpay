package wallets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/falola13/ledgerpay/internal/httpx"
	"github.com/google/uuid"
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

func validateCurrency(currency string) error {
	if currency == "" {
		return fmt.Errorf("Currency is required")
	}
	if currency != "USD" {
		return fmt.Errorf("Invalid currency '%s', Currency must be USD", currency)
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

func (h *Handler) Charges(w http.ResponseWriter, r *http.Request) {
	var charges ChargesType
	transfer_id := uuid.NewString()
	if err := json.NewDecoder(r.Body).Decode(&charges); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid Json")
		return
	}

	if err := validateAmount(charges.AmountCents); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCurrency(charges.Currency); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key := r.Header.Get("Idempotency-Key")

	if key == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Idempotent_key Header is required")
		return
	}

	data, err := json.Marshal(charges)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Error Marshalling JSON")
		return
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	const (
		pollInterval = 100 * time.Millisecond
		maxPoll      = 5
	)

	for attempt := 0; ; attempt++ {
		rec, ok, err := h.store.FindIdempotent(r.Context(), key)

		if err != nil {

			httpx.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error", "message": err})
			return
		}

		if ok && rec.RequestHash != hash {
			httpx.WriteError(w, http.StatusConflict, "Hash mismatch")
			return
		}

		// Completed already: replay the stored response verbatim.
		if ok && rec.ResponseBody != nil {
			w.Header().Set("Idempotent-Replay", "true")
			httpx.WriteJSON(w, *rec.ResponseStatus, rec.ResponseBody)
			return
		}

		if ok && rec.ResponseBody == nil {
			if attempt >= maxPoll {
				httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "request in flight"})
				return
			}
			time.Sleep(pollInterval)
			continue
		}
		break
	}

	body, err := h.store.Charges(r.Context(), transfer_id, charges, key, hash)
	if err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			httpx.WriteError(w, 402, err.Error())
			return
		}
		// Lost the unique-key race: winner owns this key — fetch and replay their response.
		if errors.Is(err, ErrIdempotencyConflict) {
			for attempt := 0; attempt < maxPoll; attempt++ {
				rec, ok, findErr := h.store.FindIdempotent(r.Context(), key)
				if findErr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, findErr.Error())
					return
				}
				if ok && rec.ResponseBody != nil {
					w.Header().Set("Idempotent-Replay", "true")
					httpx.WriteJSON(w, *rec.ResponseStatus, rec.ResponseBody)
					return
				}
				time.Sleep(pollInterval)
			}
			httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "request in flight"})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the exact body we stored — first reply matches every replay.
	httpx.WriteJSON(w, http.StatusCreated, body)
}
