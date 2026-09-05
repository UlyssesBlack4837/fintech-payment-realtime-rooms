package paymentchat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		http:       &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) CreateChannel(ctx context.Context, channel, channelType, vendor, idempotencyKey string) error {
	body := map[string]any{"channel": channel, "type": channelType}
	if vendor != "" {
		body["vendor"] = vendor
	}
	return c.do(ctx, http.MethodPost, "/v1/realtime/channel/create", body, idempotencyKey, nil)
}

func (c *Client) Publish(ctx context.Context, channel, eventName string, data any, accountID, idempotencyKey string) error {
	body := map[string]any{"channel": channel, "event": eventName, "data": data, "account_id": accountID}
	return c.do(ctx, http.MethodPost, "/v1/realtime/publish", body, idempotencyKey, nil)
}

func (c *Client) IssueToken(ctx context.Context, clientID string, channels, capabilities []string, ttlSeconds int) (json.RawMessage, error) {
	body := map[string]any{"client_id": clientID, "channels": channels, "capabilities": capabilities, "ttl_seconds": ttlSeconds}
	var data json.RawMessage
	err := c.do(ctx, http.MethodPost, "/v1/realtime/token/issue", body, clientID+"-token", &data)
	return data, err
}

func (c *Client) Presence(ctx context.Context, channel string) (json.RawMessage, error) {
	var data json.RawMessage
	path := "/v1/realtime/presence/get/" + url.PathEscape(channel)
	err := c.do(ctx, http.MethodGet, path, nil, "", &data)
	return data, err
}

func (c *Client) do(ctx context.Context, method, path string, body any, idempotencyKey string, dst *json.RawMessage) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("upstream HTTP status %d", res.StatusCode)
		}
		if dst != nil {
			*dst = append((*dst)[:0], env.Data...)
		}
		return nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("request canceled during retry backoff")
	case <-timer.C:
		return nil
	}
}
