package main

import (
	"context"
	"testing"
)

type recordingEmail struct {
	calls   int
	key     string
	payload emailPayload
}

func (r *recordingEmail) Send(_ context.Context, key string, payload emailPayload) (string, error) {
	r.calls++
	r.key = key
	r.payload = payload
	return "msg_receipt_42", nil
}

func TestReceiptModerationDecision(t *testing.T) {
	tests := []struct {
		name       string
		event      string
		states     []string
		wantStatus string
		wantCalls  int
	}{
		{name: "completed and approved", event: "purchase.completed", states: []string{"approved", "approved"}, wantStatus: "sent", wantCalls: 1},
		{name: "live event incomplete", event: "purchase.started", states: []string{"approved"}, wantStatus: "event_not_completed", wantCalls: 0},
		{name: "asset remains queued", event: "purchase.completed", states: []string{"approved", "pending"}, wantStatus: "moderation_pending", wantCalls: 0},
		{name: "asset rejected", event: "purchase.completed", states: []string{"rejected"}, wantStatus: "moderation_rejected", wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := &recordingEmail{}
			assets := make([]Asset, len(tt.states))
			for i, state := range tt.states {
				assets[i] = Asset{Name: "Player Banner", Moderation: state}
			}
			order := ReceiptRequest{
				OrderID: "ord-42", PlayerEmail: "player@example.com", AmountCents: 1299,
				Currency: "usd", Event: LiveEvent{ID: "evt-7", Title: "Summer Arena"},
				EventState: tt.event, Assets: assets,
			}

			got, err := (ReceiptSender{Email: email}).Send(context.Background(), order)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if email.calls != tt.wantCalls {
				t.Fatalf("email calls = %d, want %d", email.calls, tt.wantCalls)
			}
			if tt.wantCalls == 1 && (email.key == "" || email.payload.To != order.PlayerEmail) {
				t.Fatalf("send boundary missing key or recipient: %#v", email)
			}
		})
	}
}
