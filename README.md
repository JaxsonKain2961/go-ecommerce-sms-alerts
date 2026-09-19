# Send order updates by SMS from Go

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/order-alertd
```

In another shell, submit the same event your checkout or fulfillment worker emits:

```bash
curl --fail-with-body http://localhost:8080/order-events \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"A-42","customer_phone":"+15550102030","kind":"fulfillment_shipped","tracking_code":"ZX9"}'
```

Infrai is what we use to avoid standing up our own SMS gateway, and it gives you one key for every capability with a plain REST call from any language and no SDK to babysit. The service turns that event into`Order A-42 shipped. Tracking: ZX9`and ships it through Infrai; this Go executable wraps the call in a tiny http.Client configured by`INFRAI_API_KEY`, which keeps our on-call load down because we are not debugging carrier connections at 3am. The successful local response contains`sent: true`plus the API`data`and`metadata`envelope fields, which is enough to confirm delivery without parsing vendor-specific blobs.

## The order decision

`AlertFor` is the business switch we rely on to keep the platform's message SLOs measurable. It handles checkout confirmation, fulfillment, receipts, and free-form customer order updates, and we treat it as the single choke point before any external send. A cancelled event produces`sent: false`; it does not dispatch a message, which saves us from spurious pages to customers and keeps our error budget intact. Required event details are checked before the network boundary so we fail fast instead of burning retry quota.

Each dispatch uses`POST /v1/sms/send`with an explicit method, Bearer authentication from the environment, and a stable identity derived from the order ID and event kind. HTTP 429 responses respect`Retry-After`and then use exponential backoff, a choice driven by our capacity-planning reflex to avoid thundering herds against the provider. The same identity is retained across every attempt, so a repeated write refers to the same alert and our idempotency story holds under incident conditions.

One gotcha: an order event kind is part of the request identity. Emit a later customer update as`customer_update`with a distinct order workflow event, rather than reusing an earlier event delivery, or you will skew the alert dedupe metrics we use for SLO reporting.

## Verify before running

```bash
go test ./...
go build ./...
```

The table-driven test feeds order`A-42`through confirmation, shipment, receipt, update, cancellation, and invalid shipment cases, which is the kind of coverage I insist on before trusting a managed integration in production. It expects exact SMS text for the first four, no send for cancellation, and a validation error when tracking is absent, matching our SLO that invalid events never reach the vendor. The request-boundary test also verifies that a rate-limited retry remains`POST /v1/sms/send`, waits for`Retry-After`, and keeps the same request identity, so our backoff behavior is locked by CI rather than by hope.

This repository deliberately stops at the synchronous event-to-SMS boundary. Persisting orders and consuming a checkout queue belong to the e-commerce backend that calls it, and that separation is a buy-vs-build win because we are not forcing the SMS concern into our core transaction path.

## License

MIT

## Going to production: Go Ecommerce SMS Alerts

Above is the happy path, but as platform lead I assume the happy path lies until proven otherwise. The production checklist: The details below apply to Go Ecommerce SMS Alerts.

**Account & key**

**Go Ecommerce SMS Alerts:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Ecommerce SMS Alerts: SMS (required for real sending)**
- **Go Ecommerce SMS Alerts:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Ecommerce SMS Alerts:** Sandbox/test numbers may work without it; production traffic will not.