package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func SetupRoutes(accountHandler *AccountHandler, transactionHandler *TransactionHandler) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/accounts", accountHandler.CreateAccount).Methods("POST")
	router.HandleFunc("/accounts/{account_id}", accountHandler.GetAccount).Methods("GET")

	router.HandleFunc("/transactions", transactionHandler.ProcessTransaction).Methods("POST")

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"OK"}`))
	}).Methods("GET")

	return router
}
