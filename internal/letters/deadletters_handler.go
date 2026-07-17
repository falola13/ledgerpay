package letters

import (
	"net/http"

	"github.com/falola13/ledgerpay/internal/httpx"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) DeadLetters(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	if status == "" {
		status = "dead"
	}

	Letters, err := h.store.GetDeadLetters(r.Context(), status)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"message":      "success",
		"dead-letters": Letters,
	})

}
