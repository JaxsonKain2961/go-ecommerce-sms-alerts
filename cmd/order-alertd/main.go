package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	ordersms "github.com/infrai-examples/ecommerce-sms-alerts"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	sender := ordersms.ReceiptSender{SMS: ordersms.SMSClient{
		APIKey:     apiKey,
		MaxRetries: 3,
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /order-events", func(response http.ResponseWriter, request *http.Request) {
		var event ordersms.OrderEvent
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			http.Error(response, "invalid order event", http.StatusBadRequest)
			return
		}

		result, sent, err := sender.Handle(request.Context(), event)
		if err != nil {
			log.Printf("order %q: %v", event.OrderID, err)
			http.Error(response, "order event was not accepted", http.StatusBadGateway)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		if !sent {
			json.NewEncoder(response).Encode(map[string]any{"sent": false})
			return
		}
		json.NewEncoder(response).Encode(map[string]any{"sent": true, "data": result.Data, "metadata": result.Metadata})
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("order-alertd listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
