package account

import (
	"encoding/json"
	"net/http"
	"net/mail"

	"github.com/falola13/ledgerpay/internal/httpx"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)

	return err == nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {

	var in CreateAccountDto

	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if ok := IsValidEmail(in.Email); !ok {
		httpx.WriteError(w, http.StatusBadRequest, "Must be a valid email")
		return
	}

	account, wallet, err := h.store.Create(r.Context(), in.Email)
	if err != nil {

		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"wallet":  wallet,
			"account": account,
		},
	})
}
