package service

import (
	"context"
	"fmt"
	"strconv"

	"txn-service/internal/logger"
	"txn-service/internal/repository"
	"txn-service/models"
)

type AccountService interface {
	CreateAccount(ctx context.Context, req *models.CreateAccountRequest) error
	GetAccount(ctx context.Context, accountID int64) (*models.Account, error)
}

type accountService struct {
	accountRepo repository.AccountRepository
	logger      *logger.Logger
}

func NewAccountService(accountRepo repository.AccountRepository) AccountService {
	return &accountService{
		accountRepo: accountRepo,
		logger:      logger.NewFromEnv(),
	}
}

func (s *accountService) CreateAccount(ctx context.Context, req *models.CreateAccountRequest) error {

	if req.AccountID <= 0 {
		return fmt.Errorf("invalid account ID: %d", req.AccountID)
	}

	if err := s.validateBalance(req.InitialBalance); err != nil {
		return fmt.Errorf("invalid initial balance: %w", err)
	}

	account := &models.Account{
		AccountID: req.AccountID,
		Balance:   req.InitialBalance,
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

func (s *accountService) GetAccount(ctx context.Context, accountID int64) (*models.Account, error) {
	account, err := s.accountRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return account, nil
}

func (s *accountService) validateBalance(balance string) error {
	if balance == "" {
		return fmt.Errorf("balance cannot be empty")
	}

	_, err := strconv.ParseFloat(balance, 64)
	if err != nil {
		return fmt.Errorf("invalid balance format: %s", balance)
	}

	return nil
}
