package ordersms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSMSClientRetriesRateLimitWithSameRequestIdentity(t *testing.T) {
	var calls int
	var keys []string
	client := SMSClient{
		APIKey:     "test-key",
		BaseURL:    "https://example.test",
		MaxRetries: 1,
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			keys = append(keys, request.Header.Get("Idempotency-Key"))
			if request.Method != http.MethodPost || request.URL.Path != "/v1/sms/send" {
				t.Fatalf("request = %s %s", request.Method, request.URL.Path)
			}
			if calls == 1 {
				return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":"rate limited"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{"message_id":"msg_123"},"metadata":{}}`))}, nil
		})},
		Sleep: func(_ context.Context, delay time.Duration) error {
			if delay != 2*time.Second {
				t.Fatalf("retry delay = %s, want 2s", delay)
			}
			return nil
		},
	}

	_, err := client.Send(context.Background(), SendRequest{To: "+15550102030", Body: "Order A-42 shipped.", IdempotencyKey: "order/A-42/fulfillment_shipped"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || keys[0] != keys[1] || keys[0] == "" {
		t.Fatalf("calls = %d, idempotency keys = %#v", calls, keys)
	}
}
