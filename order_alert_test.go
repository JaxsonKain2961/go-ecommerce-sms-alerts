package ordersms

import "testing"

func TestAlertFor(t *testing.T) {
	tests := []struct {
		name     string
		event    OrderEvent
		wantBody string
		wantSend bool
		wantErr  bool
	}{
		{
			name:     "checkout confirmation",
			event:    OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "checkout_confirmed"},
			wantBody: "Order A-42 is confirmed. We will text you when fulfillment starts.",
			wantSend: true,
		},
		{
			name:     "shipment includes tracking",
			event:    OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "fulfillment_shipped", TrackingCode: "ZX9"},
			wantBody: "Order A-42 shipped. Tracking: ZX9",
			wantSend: true,
		},
		{
			name:     "receipt includes charged amount",
			event:    OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "receipt_ready", Amount: "USD 19.00"},
			wantBody: "Receipt for order A-42: USD 19.00 paid.",
			wantSend: true,
		},
		{
			name:     "customer update",
			event:    OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "customer_update", Update: "pickup is ready"},
			wantBody: "Order A-42 update: pickup is ready",
			wantSend: true,
		},
		{
			name:     "cancelled order is suppressed",
			event:    OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "order_cancelled"},
			wantSend: false,
		},
		{
			name:    "shipment requires tracking",
			event:   OrderEvent{OrderID: "A-42", CustomerPhone: "+15550102030", Kind: "fulfillment_shipped"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, send, err := AlertFor(test.event)
			if (err != nil) != test.wantErr {
				t.Fatalf("AlertFor() error = %v, wantErr %v", err, test.wantErr)
			}
			if send != test.wantSend {
				t.Fatalf("AlertFor() send = %v, want %v", send, test.wantSend)
			}
			if got.Body != test.wantBody {
				t.Fatalf("AlertFor() body = %q, want %q", got.Body, test.wantBody)
			}
			if send && got.IdempotencyKey != "order/A-42/"+test.event.Kind {
				t.Fatalf("AlertFor() idempotency key = %q", got.IdempotencyKey)
			}
		})
	}
}
