package handlers

import (
	"encoding/json"
	"net/http"

	"txn-service/internal/service"
	"txn-service/models"
)

type TransactionHandler struct {
	transactionService service.TransactionService
}

func NewTransactionHandler(transactionService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

func (h *TransactionHandler) ProcessTransaction(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SourceAccountID <= 0 {
		sendJSONError(w, "INVALID_SOURCE_ACCOUNT", "source_account_id must be a positive integer", http.StatusBadRequest)
		return
	}

	if req.DestinationAccountID <= 0 {
		sendJSONError(w, "INVALID_DESTINATION_ACCOUNT", "destination_account_id must be a positive integer", http.StatusBadRequest)
		return
	}

	if req.Amount == "" {
		sendJSONError(w, "MISSING_AMOUNT", "amount is required", http.StatusBadRequest)
		return
	}

	transaction, err := h.transactionService.ProcessTransaction(r.Context(), &req)
	if err != nil {
		sendJSONError(w, "TRANSACTION_FAILED", err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transaction)
}
