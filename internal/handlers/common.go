package handlers

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func sendJSONError(w http.ResponseWriter, errorCode, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   errorCode,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}
