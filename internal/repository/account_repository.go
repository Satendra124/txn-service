package repository

import (
	"context"
	"database/sql"
	"fmt"

	"txn-service/internal/logger"
	"txn-service/models"
)

type AccountRepository interface {
	Create(ctx context.Context, account *models.Account) error
	GetByAccountID(ctx context.Context, accountID int64) (*models.Account, error)
}

type accountRepository struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewAccountRepository(db *sql.DB) AccountRepository {
	return &accountRepository{
		db:     db,
		logger: logger.NewFromEnv(),
	}
}

func (r *accountRepository) Create(ctx context.Context, account *models.Account) error {
	entry := r.logger.WithFields(map[string]interface{}{
		"account_id": account.AccountID,
		"balance":    account.Balance,
	})

	entry.Debug("Starting account creation transaction")

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		entry.Error("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback()

	entry.Debug("Checking if account exists")
	exists, err := r.accountExistsWithLock(ctx, tx, account.AccountID)
	if err != nil {
		entry.Error("Failed to check account existence: %v", err)
		return fmt.Errorf("failed to check account existence: %w", err)
	}

	if exists {
		entry.Warn("Account already exists")
		return fmt.Errorf("account with ID %d already exists", account.AccountID)
	}

	entry.Debug("Creating new account")
	query := `
		INSERT INTO accounts (account_id, balance)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`

	err = tx.QueryRowContext(ctx, query, account.AccountID, account.Balance).
		Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		entry.Error("Failed to insert account: %v", err)
		return fmt.Errorf("failed to create account: %w", err)
	}

	entry.Debug("Account created successfully, DB_ID: %d", account.ID)
	return tx.Commit()
}

func (r *accountRepository) GetByAccountID(ctx context.Context, accountID int64) (*models.Account, error) {
	query := `
		SELECT id, account_id, balance, created_at, updated_at
		FROM accounts
		WHERE account_id = $1`

	account := &models.Account{}
	err := r.db.QueryRowContext(ctx, query, accountID).
		Scan(&account.ID, &account.AccountID, &account.Balance, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found: %d", accountID)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

func (r *accountRepository) accountExistsWithLock(ctx context.Context, tx *sql.Tx, accountID int64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM accounts 
			WHERE account_id = $1 
			FOR UPDATE
		)`

	var exists bool
	err := tx.QueryRowContext(ctx, query, accountID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check account existence: %w", err)
	}

	return exists, nil
}
