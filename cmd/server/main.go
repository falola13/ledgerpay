package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/falola13/ledgerpay/internal/account"
	"github.com/falola13/ledgerpay/internal/config"
	"github.com/falola13/ledgerpay/internal/database"
	"github.com/falola13/ledgerpay/internal/health"
	"github.com/falola13/ledgerpay/internal/middleware"
	"github.com/falola13/ledgerpay/internal/wallets"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	mux := http.NewServeMux()

	ctx := context.Background()

	handler := middleware.Logger(mux)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("❌ Database not connected", "error", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Database connected")

	defer pool.Close()

	//Stores
	accountStore := account.NewStore(pool)
	walletStore := wallets.NewStore(pool)

	// Handlers
	accountHandler := account.NewHandler(accountStore)
	walletHandler := wallets.NewHandler(walletStore)

	checker := health.NewHandler(pool)

	mux.HandleFunc("GET /v1/health", http.HandlerFunc(checker.Live))
	mux.HandleFunc("GET /v1/ready", http.HandlerFunc(checker.Ready))

	//Accounts
	mux.HandleFunc("POST /v1/accounts", http.HandlerFunc(accountHandler.Create))

	// Wallets
	mux.HandleFunc("GET /v1/wallets/{id}", http.HandlerFunc(walletHandler.GetWalletById))
	mux.HandleFunc("POST /v1/wallets/{id}/fund", http.HandlerFunc(walletHandler.FundWallet))

	// Charges
	mux.HandleFunc("POST /v1/charges", http.HandlerFunc(walletHandler.Charges))

	//Server start
	go func() {
		slog.Info("Server started", "Port", cfg.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server could not be started", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

}
