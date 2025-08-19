package service

import (
	"context"
	"fmt"
	"strconv"

	"txn-service/internal/logger"
	"txn-service/internal/repository"
	"txn-service/models"

	"github.com/google/uuid"
)

type TransactionService interface {
	ProcessTransaction(ctx context.Context, req *models.CreateTransactionRequest) (*models.CreateTransactionSuccessResponse, error)
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	accountRepo     repository.AccountRepository
	logger          *logger.Logger
}

func NewTransactionService(transactionRepo repository.TransactionRepository, accountRepo repository.AccountRepository) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		accountRepo:     accountRepo,
		logger:          logger.NewFromEnv(),
	}
}

func (s *transactionService) ProcessTransaction(ctx context.Context, req *models.CreateTransactionRequest) (*models.CreateTransactionSuccessResponse, error) {
	if err := s.validateTransactionRequest(req); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}

	transactionID := uuid.New()
	if err := s.transactionRepo.Create(ctx, &models.Transaction{
		TransactionID:        transactionID,
		SourceAccountID:      req.SourceAccountID,
		DestinationAccountID: req.DestinationAccountID,
		Amount:               req.Amount,
		Status:               models.TransactionStatusPending,
	}); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := s.transactionRepo.Transfer(ctx, req.SourceAccountID, req.DestinationAccountID, req.Amount, transactionID); err != nil {
		return nil, fmt.Errorf("failed to transfer funds: %w", err)
	}

	return &models.CreateTransactionSuccessResponse{
		TransactionID: transactionID,
	}, nil
}

func (s *transactionService) validateTransactionRequest(req *models.CreateTransactionRequest) error {
	if req.SourceAccountID <= 0 {
		return fmt.Errorf("invalid source account ID: %d", req.SourceAccountID)
	}

	if req.DestinationAccountID <= 0 {
		return fmt.Errorf("invalid destination account ID: %d", req.DestinationAccountID)
	}

	if req.SourceAccountID == req.DestinationAccountID {
		return fmt.Errorf("source and destination accounts cannot be the same")
	}

	if req.Amount == "" {
		return fmt.Errorf("amount cannot be empty")
	}

	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return fmt.Errorf("invalid amount format: %s", req.Amount)
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	return nil
}
