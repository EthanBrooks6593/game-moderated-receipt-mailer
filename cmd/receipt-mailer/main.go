package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	sender := ReceiptSender{Email: &InfraiClient{APIKey: key}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /receipts", receiptHandler(sender))
	log.Println("receipt-mailer listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func receiptHandler(sender ReceiptSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var order ReceiptRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&order); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid receipt request"})
			return
		}
		if order.OrderID == "" || order.PlayerEmail == "" || order.AmountCents < 0 || order.Currency == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order_id, player_email, currency, and a non-negative amount_cents are required"})
			return
		}

		result, err := sender.Send(r.Context(), order)
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email delivery failed"})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
