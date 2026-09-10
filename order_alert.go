package ordersms

import (
	"context"
	"errors"
	"fmt"
)

type OrderEvent struct {
	OrderID       string `json:"order_id"`
	CustomerPhone string `json:"customer_phone"`
	Kind          string `json:"kind"`
	Amount        string `json:"amount,omitempty"`
	TrackingCode  string `json:"tracking_code,omitempty"`
	Update        string `json:"update,omitempty"`
}

type Alert struct {
	To             string
	Body           string
	IdempotencyKey string
}

type Sender interface {
	Send(context.Context, SendRequest) (SendResult, error)
}

type ReceiptSender struct {
	SMS Sender
}

func AlertFor(event OrderEvent) (Alert, bool, error) {
	if event.OrderID == "" || event.CustomerPhone == "" {
		return Alert{}, false, errors.New("order_id and customer_phone are required")
	}

	var body string
	switch event.Kind {
	case "checkout_confirmed":
		body = fmt.Sprintf("Order %s is confirmed. We will text you when fulfillment starts.", event.OrderID)
	case "fulfillment_shipped":
		if event.TrackingCode == "" {
			return Alert{}, false, errors.New("tracking_code is required for fulfillment_shipped")
		}
		body = fmt.Sprintf("Order %s shipped. Tracking: %s", event.OrderID, event.TrackingCode)
	case "receipt_ready":
		if event.Amount == "" {
			return Alert{}, false, errors.New("amount is required for receipt_ready")
		}
		body = fmt.Sprintf("Receipt for order %s: %s paid.", event.OrderID, event.Amount)
	case "customer_update":
		if event.Update == "" {
			return Alert{}, false, errors.New("update is required for customer_update")
		}
		body = fmt.Sprintf("Order %s update: %s", event.OrderID, event.Update)
	case "order_cancelled":
		return Alert{}, false, nil
	default:
		return Alert{}, false, fmt.Errorf("unknown order event kind %q", event.Kind)
	}

	return Alert{
		To:             event.CustomerPhone,
		Body:           body,
		IdempotencyKey: "order/" + event.OrderID + "/" + event.Kind,
	}, true, nil
}

func (s ReceiptSender) Handle(ctx context.Context, event OrderEvent) (SendResult, bool, error) {
	alert, shouldSend, err := AlertFor(event)
	if err != nil || !shouldSend {
		return SendResult{}, false, err
	}
	result, err := s.SMS.Send(ctx, SendRequest{
		To:             alert.To,
		Body:           alert.Body,
		IdempotencyKey: alert.IdempotencyKey,
	})
	return result, true, err
}
