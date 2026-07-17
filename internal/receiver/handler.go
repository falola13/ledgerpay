package receiver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"log/slog"
	"net/http"

	"github.com/falola13/ledgerpay/internal/httpx"
)

type Handler struct {
	secret string
}

func NewHandler(secret string) *Handler {
	return &Handler{secret: secret}
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(bodyBytes)
	expected := hex.EncodeToString(mac.Sum(nil))

	got := r.Header.Get("X-LedgerPay-Signature")

	if got == "" {
		slog.Warn("Webhook signature missing ")
		httpx.WriteError(w, http.StatusUnauthorized, "Webhook signature missing - Required")
		return
	}

	if !hmac.Equal([]byte(expected), []byte(got)) {
		slog.Warn("Webhook signature INVALID")
		httpx.WriteError(w, http.StatusUnauthorized, "Invalid Signature")
		return
	}

	slog.Info("Webhook Signature OK")
	httpx.WriteJSON(w, http.StatusOK, "Signature ok")
}
