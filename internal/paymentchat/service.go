package paymentchat

import (
	"context"
	"encoding/json"
	"fmt"
)

type Realtime interface {
	CreateChannel(context.Context, string, string, string, string) error
	Publish(context.Context, string, string, any, string, string) error
	IssueToken(context.Context, string, []string, []string, int) (json.RawMessage, error)
	Presence(context.Context, string) (json.RawMessage, error)
}

type Service struct {
	realtime        Realtime
	reviewThreshold int64
}

func NewService(realtime Realtime, reviewThreshold int64) *Service {
	return &Service{realtime: realtime, reviewThreshold: reviewThreshold}
}

func (s *Service) RecordPayment(ctx context.Context, event PaymentEvent) (Notification, error) {
	notification, err := Decide(event, s.reviewThreshold)
	if err != nil {
		return Notification{}, err
	}
	channel := ChannelFor(event.AccountID)
	key := "payment-" + event.PaymentID
	if err := s.realtime.CreateChannel(ctx, channel, "private", "", key+"-channel"); err != nil {
		return Notification{}, fmt.Errorf("create payment channel: %w", err)
	}
	if err := s.realtime.Publish(ctx, channel, "payment."+event.Kind, notification, event.AccountID, key+"-publish"); err != nil {
		return Notification{}, fmt.Errorf("publish payment notification: %w", err)
	}
	return notification, nil
}

func (s *Service) Token(ctx context.Context, accountID, clientID string) (json.RawMessage, error) {
	return s.realtime.IssueToken(ctx, clientID, []string{ChannelFor(accountID)}, []string{"subscribe"}, 300)
}

func (s *Service) Presence(ctx context.Context, accountID string) (json.RawMessage, error) {
	return s.realtime.Presence(ctx, ChannelFor(accountID))
}
