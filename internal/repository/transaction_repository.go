package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"txn-service/internal/logger"
	"txn-service/models"

	"github.com/google/uuid"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *models.Transaction) error
	Transfer(ctx context.Context, sourceAccountID int64, destinationAccountID int64, amount string, transactionId uuid.UUID) error
}

type transactionRepository struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewTransactionRepository(db *sql.DB) TransactionRepository {
	return &transactionRepository{
		db:     db,
		logger: logger.NewFromEnv(),
	}
}

func (r *transactionRepository) Create(ctx context.Context, transaction *models.Transaction) error {
	query := `
		INSERT INTO transactions (transaction_id, source_account_id, destination_account_id, amount, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		transaction.TransactionID,
		transaction.SourceAccountID,
		transaction.DestinationAccountID,
		transaction.Amount,
		transaction.Status,
	).Scan(&transaction.ID, &transaction.CreatedAt, &transaction.UpdatedAt)
}

// getByAccountIDWithLock will get the account and lock it until next update
func (r *transactionRepository) getByAccountIDWithLock(ctx context.Context, tx *sql.Tx, accountID int64) (*models.Account, error) {
	query := `
		SELECT id, account_id, balance, created_at, updated_at
		FROM accounts
		WHERE account_id = $1
		FOR UPDATE`

	account := &models.Account{}
	err := tx.QueryRowContext(ctx, query, accountID).
		Scan(&account.ID, &account.AccountID, &account.Balance, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found: %d", accountID)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

func (r *transactionRepository) getByAccountID(ctx context.Context, tx *sql.Tx, accountID int64) (*models.Account, error) {
	query := `
		SELECT id, account_id, balance, created_at, updated_at
		FROM accounts
		WHERE account_id = $1`

	account := &models.Account{}
	err := tx.QueryRowContext(ctx, query, accountID).
		Scan(&account.ID, &account.AccountID, &account.Balance, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found: %d", accountID)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

// Transfer would perform the main logic to process the transaction
// Isolation mode READ COMMITED is used with ROW lock to prevent issues in concurrent transaction
// this level can be bumped up to REPEATABLE READ or SERIALIZABLE isolation level if complexity of the
// function increases but the throughput would decrease as the isolation level is increased
func (r *transactionRepository) Transfer(ctx context.Context, sourceAccountID int64, destinationAccountID int64, amount string, transactionId uuid.UUID) error {
	entry := r.logger.WithFields(map[string]interface{}{
		"transaction_id":         transactionId,
		"source_account_id":      sourceAccountID,
		"destination_account_id": destinationAccountID,
		"amount":                 amount,
	})

	entry.Debug("Starting transfer transaction")

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		entry.Error("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback()

	var sourceAccount, destinationAccount *models.Account

	// we need to make sure to lock the lower id first to avoid dead locks
	if sourceAccountID < destinationAccountID {
		entry.Debug("Getting source account with lock (first)")
		var err error
		sourceAccount, err = r.getByAccountIDWithLock(ctx, tx, sourceAccountID)
		if err != nil {
			entry.Error("Failed to get source account: %v", err)
			return fmt.Errorf("failed to get source account: %w", err)
		}

		entry.Debug("Getting destination account with lock (second)")
		destinationAccount, err = r.getByAccountIDWithLock(ctx, tx, destinationAccountID)
		if err != nil {
			entry.Error("Failed to get destination account: %v", err)
			return fmt.Errorf("failed to get destination account: %w", err)
		}
	} else {
		entry.Debug("Getting destination account with lock (first)")
		var err error
		destinationAccount, err = r.getByAccountIDWithLock(ctx, tx, destinationAccountID)
		if err != nil {
			entry.Error("Failed to get destination account: %v", err)
			return fmt.Errorf("failed to get destination account: %w", err)
		}

		entry.Debug("Getting source account with lock (second)")
		sourceAccount, err = r.getByAccountIDWithLock(ctx, tx, sourceAccountID)
		if err != nil {
			entry.Error("Failed to get source account: %v", err)
			return fmt.Errorf("failed to get source account: %w", err)
		}
	}

	txnAmount, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		entry.Error("Failed to parse transaction amount: %v", err)
		return fmt.Errorf("failed to parse transaction amount: %w", err)
	}

	sourceBalance, err := strconv.ParseFloat(sourceAccount.Balance, 64)
	if err != nil {
		entry.Error("Failed to parse source account balance: %v", err)
		return fmt.Errorf("failed to parse source account balance: %w", err)
	}

	if sourceBalance < txnAmount {
		entry.Warn("Insufficient balance: source_balance=%f, requested_amount=%f", sourceBalance, txnAmount)
		return fmt.Errorf("insufficient balance")
	}

	sourceBalance -= txnAmount

	destinationBalance, err := strconv.ParseFloat(destinationAccount.Balance, 64)
	if err != nil {
		entry.Error("Failed to parse destination account balance: %v", err)
		return fmt.Errorf("failed to parse destination account balance: %w", err)
	}
	destinationBalance += txnAmount

	entry.Debug("Updating balances: source_new_balance=%f, destination_new_balance=%f", sourceBalance, destinationBalance)

	_, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = $1 WHERE account_id = $2", sourceBalance, sourceAccountID)
	if err != nil {
		entry.Error("Failed to update source account: %v", err)
		return fmt.Errorf("failed to update source account: %w", err)
	}

	_, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = $1 WHERE account_id = $2", destinationBalance, destinationAccountID)
	if err != nil {
		entry.Error("Failed to update destination account: %v", err)
		return fmt.Errorf("failed to update destination account: %w", err)
	}

	_, err = tx.ExecContext(ctx, "UPDATE transactions SET status = $1 WHERE transaction_id = $2", models.TransactionStatusCompleted, transactionId)
	if err != nil {
		entry.Error("Failed to update transaction status: %v", err)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	entry.Info("Transfer completed successfully")
	return tx.Commit()
}
