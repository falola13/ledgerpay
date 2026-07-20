package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/falola13/ledgerpay/internal/config"
	"github.com/falola13/ledgerpay/internal/middleware"
	"github.com/falola13/ledgerpay/internal/receiver"
	"github.com/joho/godotenv"
)

const port = "9090"

func main() {
	_ = godotenv.Load()
	config := config.Load()
	ctx := context.Background()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	handler := middleware.RequestId(middleware.Logger(mux))

	srv := &http.Server{Addr: ":" + port, Handler: handler}

	webhookHandler := receiver.NewHandler(config.WEBHOOK_SECRET)

	mux.HandleFunc("POST /webhook", http.HandlerFunc(webhookHandler.Webhook))

	go func() {
		slog.Info("Reciever started", "Port", "9090")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info(fmt.Sprintf("Receiver waiting on port %s", port))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("Receiver Shuting down...")
}
