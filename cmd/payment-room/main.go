package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"example.com/fintech-payment-rooms/internal/paymentchat"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	threshold := int64(100000)
	if raw := os.Getenv("REVIEW_THRESHOLD_MINOR"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			log.Fatal("REVIEW_THRESHOLD_MINOR must be a positive integer")
		}
		threshold = value
	}

	service := paymentchat.NewService(paymentchat.NewClient(apiKey), threshold)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments/events", func(w http.ResponseWriter, r *http.Request) {
		var event paymentchat.PaymentEvent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
			return
		}
		notification, err := service.RecordPayment(r.Context(), event)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, notification)
	})
	mux.HandleFunc("POST /accounts/{account}/realtime-token", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil || strings.TrimSpace(input.ClientID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_id is required"})
			return
		}
		data, err := service.Token(r.Context(), r.PathValue("account"), input.ClientID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeRawJSON(w, http.StatusOK, data)
	})
	mux.HandleFunc("GET /accounts/{account}/presence", func(w http.ResponseWriter, r *http.Request) {
		data, err := service.Presence(r.Context(), r.PathValue("account"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeRawJSON(w, http.StatusOK, data)
	})

	addr := ":8080"
	if value := os.Getenv("ADDR"); value != "" {
		addr = value
	}
	log.Printf("payment room service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *paymentchat.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "realtime request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeRawJSON(w http.ResponseWriter, status int, value json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(value)
}
