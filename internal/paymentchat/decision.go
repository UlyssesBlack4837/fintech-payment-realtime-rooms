package paymentchat

import (
	"errors"
	"fmt"
	"strings"
)

type PaymentEvent struct {
	PaymentID   string `json:"payment_id"`
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Kind        string `json:"kind"`
}

type Notification struct {
	PaymentID   string `json:"payment_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	Action      string `json:"action"`
}

func Decide(event PaymentEvent, reviewThreshold int64) (Notification, error) {
	if strings.TrimSpace(event.PaymentID) == "" || strings.TrimSpace(event.AccountID) == "" {
		return Notification{}, errors.New("payment_id and account_id are required")
	}
	if event.AmountMinor <= 0 || strings.TrimSpace(event.Currency) == "" || strings.TrimSpace(event.Kind) == "" {
		return Notification{}, errors.New("amount_minor must be positive; currency and kind are required")
	}

	n := Notification{
		PaymentID:   event.PaymentID,
		AmountMinor: event.AmountMinor,
		Currency:    strings.ToUpper(event.Currency),
		Status:      event.Kind,
		Action:      "inform",
	}
	if event.AmountMinor >= reviewThreshold {
		n.Action = "review_required"
	}
	return n, nil
}

func ChannelFor(accountID string) string {
	return fmt.Sprintf("account-%s-payments", accountID)
}
