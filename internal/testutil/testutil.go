package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"txn-service/internal/database"
	"txn-service/internal/handlers"
	"txn-service/internal/repository"
	"txn-service/internal/service"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestServer struct {
	Server  *httptest.Server
	DB      *sql.DB
	Cleanup func()
	client  *http.Client
}

func SetupTestServer(t *testing.T) *TestServer {
	t.Helper()

	os.Setenv("LOG_LEVEL", "ERROR")
	os.Setenv("LOG_FILE", "./logs/txn-service.log")

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "txn_service_test",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "password",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}

	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := postgres.Host(ctx)
	require.NoError(t, err)
	port, err := postgres.MappedPort(ctx, "5432")
	require.NoError(t, err)

	databaseURL := fmt.Sprintf("postgres://postgres:password@%s:%s/txn_service_test?sslmode=disable", host, port.Port())

	time.Sleep(2 * time.Second)

	db, err := database.NewConnection(databaseURL)
	require.NoError(t, err)

	err = runMigrations(db)
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	accountRepo := repository.NewAccountRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)

	accountService := service.NewAccountService(accountRepo)
	transactionService := service.NewTransactionService(transactionRepo, accountRepo)

	accountHandler := handlers.NewAccountHandler(accountService)
	transactionHandler := handlers.NewTransactionHandler(transactionService)

	router := handlers.SetupRoutes(accountHandler, transactionHandler)

	server := httptest.NewServer(router)

	cleanup := func() {
		server.Close()
		db.Close()
		postgres.Terminate(ctx)
	}

	ts := &TestServer{
		Server:  server,
		DB:      db,
		Cleanup: cleanup,
	}

	ts.client = &http.Client{
		Timeout: 10 * time.Second,
	}

	return ts
}

func runMigrations(db *sql.DB) error {

	accountsTable := `
		CREATE TABLE IF NOT EXISTS accounts (
			id SERIAL PRIMARY KEY,
			account_id BIGINT UNIQUE NOT NULL,
			balance DECIMAL(15,2) NOT NULL DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`

	transactionsTable := `
		CREATE TABLE IF NOT EXISTS transactions (
			id SERIAL PRIMARY KEY,
			transaction_id UUID UNIQUE NOT NULL,
			source_account_id BIGINT NOT NULL,
			destination_account_id BIGINT NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_account_id) REFERENCES accounts(account_id),
			FOREIGN KEY (destination_account_id) REFERENCES accounts(account_id)
		);
	`

	indexes := `
		CREATE INDEX IF NOT EXISTS idx_accounts_account_id ON accounts(account_id);
		CREATE INDEX IF NOT EXISTS idx_transactions_transaction_id ON transactions(transaction_id);
		CREATE INDEX IF NOT EXISTS idx_transactions_source_account_id ON transactions(source_account_id);
		CREATE INDEX IF NOT EXISTS idx_transactions_destination_account_id ON transactions(destination_account_id);
	`

	_, err := db.Exec(accountsTable)
	if err != nil {
		return fmt.Errorf("failed to create accounts table: %w", err)
	}

	_, err = db.Exec(transactionsTable)
	if err != nil {
		return fmt.Errorf("failed to create transactions table: %w", err)
	}

	_, err = db.Exec(indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func (ts *TestServer) CreateTestAccount(t *testing.T, accountID int64, balance string) {
	t.Helper()

	url := fmt.Sprintf("%s/accounts", ts.Server.URL)
	payload := fmt.Sprintf(`{"account_id": %d, "balance": "%s"}`, accountID, balance)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func (ts *TestServer) GetAccountBalance(t *testing.T, accountID int64) string {
	t.Helper()

	url := fmt.Sprintf("%s/accounts/%d", ts.Server.URL, accountID)

	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	resp, err := ts.client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	var account struct {
		AccountID int64  `json:"account_id"`
		Balance   string `json:"balance"`
	}

	err = json.NewDecoder(resp.Body).Decode(&account)
	require.NoError(t, err)

	return account.Balance
}

func (ts *TestServer) CreateTransaction(t *testing.T, sourceAccountID, destinationAccountID int64, amount string) string {
	t.Helper()

	url := fmt.Sprintf("%s/transactions", ts.Server.URL)
	payload := fmt.Sprintf(`{
		"source_account_id": %d,
		"destination_account_id": %d,
		"amount": "%s"
	}`, sourceAccountID, destinationAccountID, amount)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		t.Errorf("Failed to create request: %v", err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		t.Errorf("HTTP request failed: %v", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Transaction failed with status %d: %s", resp.StatusCode, string(body))
		resp.Body.Close()
		return ""
	}
	defer resp.Body.Close()

	var response struct {
		TransactionID string `json:"transaction_id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
		return ""
	}

	return response.TransactionID
}
