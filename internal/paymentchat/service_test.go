package paymentchat

import (
	"context"
	"encoding/json"
	"testing"
)

type realtimeSpy struct {
	channel        string
	vendor         string
	eventName      string
	accountID      string
	idempotencyKey string
	published      Notification
}

func (s *realtimeSpy) CreateChannel(_ context.Context, _ string, _ string, vendor string, _ string) error {
	s.vendor = vendor
	return nil
}
func (s *realtimeSpy) IssueToken(context.Context, string, []string, []string, int) (json.RawMessage, error) {
	return json.RawMessage(`{"token":"issued"}`), nil
}
func (s *realtimeSpy) Presence(context.Context, string) (json.RawMessage, error) { return nil, nil }
func (s *realtimeSpy) Publish(_ context.Context, channel, eventName string, data any, accountID, idempotencyKey string) error {
	s.channel = channel
	s.eventName = eventName
	s.accountID = accountID
	s.idempotencyKey = idempotencyKey
	s.published = data.(Notification)
	return nil
}

func TestRecordPaymentDecision(t *testing.T) {
	tests := []struct {
		name       string
		amount     int64
		wantAction string
	}{
		{name: "routine payment informs room", amount: 4999, wantAction: "inform"},
		{name: "threshold payment requires review", amount: 100000, wantAction: "review_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &realtimeSpy{}
			service := NewService(spy, 100000)
			got, err := service.RecordPayment(context.Background(), PaymentEvent{
				PaymentID: "pay_01", AccountID: "acct_42", AmountMinor: tt.amount, Currency: "usd", Kind: "authorized",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != tt.wantAction || spy.published.Action != tt.wantAction {
				t.Fatalf("action = %q, published = %q; want %q", got.Action, spy.published.Action, tt.wantAction)
			}
			if spy.channel != "account-acct_42-payments" || spy.eventName != "payment.authorized" {
				t.Fatalf("unexpected publish target: channel=%q event=%q", spy.channel, spy.eventName)
			}
			if spy.accountID != "acct_42" || spy.idempotencyKey != "payment-pay_01-publish" {
				t.Fatalf("audit boundary mismatch: account=%q key=%q", spy.accountID, spy.idempotencyKey)
			}
			if spy.vendor != "" {
				t.Fatalf("channel vendor = %q; want omitted", spy.vendor)
			}
		})
	}
}
