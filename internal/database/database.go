package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func NewConnection(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database connection established successfully")
	return db, nil
}

func runMigrations(db *sql.DB) error {

	accountsTable := `
	CREATE TABLE IF NOT EXISTS accounts (
		id SERIAL PRIMARY KEY,
		account_id BIGINT UNIQUE NOT NULL,
		balance DECIMAL(20,8) NOT NULL DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`

	transactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		transaction_id UUID UNIQUE NOT NULL,
		source_account_id BIGINT NOT NULL,
		destination_account_id BIGINT NOT NULL,
		amount DECIMAL(20,8) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_accounts_account_id ON accounts(account_id);",
		"CREATE INDEX IF NOT EXISTS idx_transactions_source_account_id ON transactions(source_account_id);",
		"CREATE INDEX IF NOT EXISTS idx_transactions_destination_account_id ON transactions(destination_account_id);",
		"CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);",
	}

	migrations := []string{accountsTable, transactionsTable}
	migrations = append(migrations, indexes...)

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	log.Println("Database migrations completed successfully")
	return nil
}
