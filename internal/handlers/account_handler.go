package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"txn-service/internal/service"
	"txn-service/models"

	"github.com/gorilla/mux"
)

type AccountHandler struct {
	accountService service.AccountService
}

func NewAccountHandler(accountService service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AccountID <= 0 {
		sendJSONError(w, "INVALID_ACCOUNT_ID", "account_id must be a positive integer", http.StatusBadRequest)
		return
	}

	if req.InitialBalance == "" {
		sendJSONError(w, "MISSING_BALANCE", "initial_balance is required", http.StatusBadRequest)
		return
	}

	if err := h.accountService.CreateAccount(r.Context(), &req); err != nil {

		if isDuplicateAccountError(err) {
			sendJSONError(w, "ACCOUNT_ALREADY_EXISTS", err.Error(), http.StatusConflict)
			return
		}
		sendJSONError(w, "CREATE_ACCOUNT_FAILED", err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func isDuplicateAccountError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return len(errMsg) > 0 && errMsg[:25] == "account with ID"
}

func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {

	accountIDStr := mux.Vars(r)["account_id"]
	if accountIDStr == "" {
		sendJSONError(w, "MISSING_ACCOUNT_ID", "account_id parameter is required", http.StatusBadRequest)
		return
	}

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil {
		sendJSONError(w, "INVALID_ACCOUNT_ID_FORMAT", "Invalid account_id format", http.StatusBadRequest)
		return
	}

	account, err := h.accountService.GetAccount(r.Context(), accountID)
	if err != nil {
		sendJSONError(w, "ACCOUNT_NOT_FOUND", err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}
