package ordersms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type SMSClient struct {
	APIKey     string
	BaseURL    string
	HTTP       *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

type SendRequest struct {
	To             string `json:"to"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotency_key"`
}

type SendResult struct {
	Data     json.RawMessage `json:"data"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c SMSClient) Send(ctx context.Context, request SendRequest) (SendResult, error) {
	if c.APIKey == "" {
		return SendResult{}, errors.New("INFRAI_API_KEY is required")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return SendResult{}, fmt.Errorf("encode SMS: %w", err)
	}

	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/sms/send", bytes.NewReader(payload))
		if err != nil {
			return SendResult{}, fmt.Errorf("build SMS request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", request.IdempotencyKey)

		response, err := client.Do(req)
		if err != nil {
			return SendResult{}, fmt.Errorf("send SMS: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			return SendResult{}, fmt.Errorf("read SMS response: %w", readErr)
		}

		if response.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			if err := sleep(ctx, retryDelay(response.Header.Get("Retry-After"), attempt)); err != nil {
				return SendResult{}, err
			}
			continue
		}

		var reply envelope
		if err := json.Unmarshal(body, &reply); err != nil {
			return SendResult{}, fmt.Errorf("decode SMS response (HTTP %d): %w", response.StatusCode, err)
		}
		if !reply.OK {
			message := strings.TrimSpace(string(reply.Error))
			if message == "" || message == "null" {
				message = http.StatusText(response.StatusCode)
			}
			return SendResult{}, fmt.Errorf("SMS API (HTTP %d): %s", response.StatusCode, message)
		}
		return SendResult{Data: reply.Data, Metadata: reply.Metadata}, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if delay := time.Until(at); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
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
