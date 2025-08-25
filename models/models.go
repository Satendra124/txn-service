package models

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID        int64     `json:"-" db:"id"`
	AccountID int64     `json:"account_id" db:"account_id"`
	Balance   string    `json:"balance" db:"balance"`
	CreatedAt time.Time `json:"-" db:"created_at"`
	UpdatedAt time.Time `json:"-" db:"updated_at"`
}

type Transaction struct {
	ID                   int64     `json:"-" db:"id"`
	TransactionID        uuid.UUID `json:"transaction_id" db:"transaction_id"`
	SourceAccountID      int64     `json:"source_account_id" db:"source_account_id"`
	DestinationAccountID int64     `json:"destination_account_id" db:"destination_account_id"`
	Amount               string    `json:"amount" db:"amount"`
	Status               string    `json:"status" db:"status"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

type CreateTransactionSuccessResponse struct {
	TransactionID uuid.UUID `json:"transaction_id"`
}

type CreateAccountRequest struct {
	AccountID      int64  `json:"account_id" validate:"required,gt=0"`
	InitialBalance string `json:"initial_balance" validate:"required"`
}

type CreateTransactionRequest struct {
	SourceAccountID      int64  `json:"source_account_id" validate:"required,gt=0"`
	DestinationAccountID int64  `json:"destination_account_id" validate:"required,gt=0"`
	Amount               string `json:"amount" validate:"required"`
}

const (
	TransactionStatusPending   = "pending"
	TransactionStatusCompleted = "completed"
	TransactionStatusFailed    = "failed"
)
