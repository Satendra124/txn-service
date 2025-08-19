package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"txn-service/internal/config"
	"txn-service/internal/database"
	"txn-service/internal/handlers"
	"txn-service/internal/logger"
	"txn-service/internal/repository"
	"txn-service/internal/service"
)

func main() {
	// Initialize the logger from ENV vars - supports Log level and file logging
	logger := logger.NewFromEnv()
	logger.Info("Starting transaction service")

	// Load configuration
	cfg := config.Load()
	logger.Info("Configuration loaded - server_address: %s", cfg.ServerAddress)

	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("Database connection established")

	accountRepo := repository.NewAccountRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)

	accountService := service.NewAccountService(accountRepo)
	transactionService := service.NewTransactionService(transactionRepo, accountRepo)

	accountHandler := handlers.NewAccountHandler(accountService)
	transactionHandler := handlers.NewTransactionHandler(transactionService)

	router := handlers.SetupRoutes(accountHandler, transactionHandler)

	server := &http.Server{
		Addr:         cfg.ServerAddress,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("Starting HTTP server - address: %s", cfg.ServerAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Received shutdown signal, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown: %v", err)
		os.Exit(1)
	}

	logger.Info("Server shutdown completed")
}
