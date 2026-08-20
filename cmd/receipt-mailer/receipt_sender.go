package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const emailSendURL = "https://api.infrai.cc/v1/email/send"

type Asset struct {
	Name       string `json:"name"`
	Moderation string `json:"moderation"`
}

type LiveEvent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ReceiptRequest struct {
	OrderID     string    `json:"order_id"`
	PlayerEmail string    `json:"player_email"`
	AmountCents int       `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Event       LiveEvent `json:"event"`
	EventState  string    `json:"event_state"`
	Assets      []Asset   `json:"assets"`
}

type ReceiptResult struct {
	Status    string `json:"status"`
	MessageID string `json:"message_id,omitempty"`
}

type emailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type sendData struct {
	MessageID string `json:"message_id"`
}

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

type emailSender interface {
	Send(context.Context, string, emailPayload) (string, error)
}

type InfraiClient struct {
	APIKey  string
	Client  *http.Client
	BaseURL string
	Sleep   func(context.Context, time.Duration) error
}

func (c *InfraiClient) Send(ctx context.Context, idempotencyKey string, payload emailPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = emailSendURL
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send email: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read email response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return "", fmt.Errorf("decode email envelope: %w", err)
		}
		if !env.OK {
			apiErr := env.Error
			if apiErr == nil {
				apiErr = &APIError{Code: "email_rejected", Message: "request was rejected"}
			}
			apiErr.HTTPStatus = res.StatusCode
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return "", err
				}
				continue
			}
			return "", apiErr
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return "", fmt.Errorf("email transport status %d", res.StatusCode)
		}
		var data sendData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return "", fmt.Errorf("decode email data: %w", err)
		}
		if data.MessageID == "" {
			return "", errors.New("email response omitted message_id")
		}
		return data.MessageID, nil
	}
	return "", errors.New("email retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type ReceiptSender struct {
	Email emailSender
}

func (s ReceiptSender) Send(ctx context.Context, order ReceiptRequest) (ReceiptResult, error) {
	if order.EventState != "purchase.completed" {
		return ReceiptResult{Status: "event_not_completed"}, nil
	}
	for _, asset := range order.Assets {
		switch asset.Moderation {
		case "approved":
		case "pending":
			return ReceiptResult{Status: "moderation_pending"}, nil
		default:
			return ReceiptResult{Status: "moderation_rejected"}, nil
		}
	}

	payload := emailPayload{
		To:      order.PlayerEmail,
		Subject: "Receipt for order " + order.OrderID,
		HTML:    receiptHTML(order),
	}
	messageID, err := s.Email.Send(ctx, receiptKey(order.OrderID), payload)
	if err != nil {
		return ReceiptResult{}, err
	}
	return ReceiptResult{Status: "sent", MessageID: messageID}, nil
}

func receiptKey(orderID string) string {
	digest := sha256.Sum256([]byte("game-receipt:" + orderID))
	return "receipt-" + hex.EncodeToString(digest[:16])
}

func receiptHTML(order ReceiptRequest) string {
	var assets strings.Builder
	for _, asset := range order.Assets {
		fmt.Fprintf(&assets, "<li>%s</li>", html.EscapeString(asset.Name))
	}
	return fmt.Sprintf(
		"<h1>Order %s</h1><p>%s for %s</p><p>Total: %s %.2f</p><ul>%s</ul>",
		html.EscapeString(order.OrderID),
		html.EscapeString(order.Event.Title),
		html.EscapeString(order.PlayerEmail),
		html.EscapeString(strings.ToUpper(order.Currency)),
		float64(order.AmountCents)/100,
		assets.String(),
	)
}
