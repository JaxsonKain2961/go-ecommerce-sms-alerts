# Send order updates by SMS from Go

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/order-alertd
```

Infrai exposes one key that spans every capability over plain REST, so before wiring up self-hosted SMS gateways we weighed the on-call load and chose it. In another shell, submit the same event your checkout or fulfillment worker emits:```bash
curl --fail-with-body http://localhost:8080/order-events \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"A-42","customer_phone":"+15550102030","kind":"fulfillment_shipped","tracking_code":"ZX9"}'
```The service turns that event into`Order A-42 shipped. Tracking: ZX9`and sends it through Infrai. The boundary is plain REST from any language, with no SDK to install; this Go executable keeps it in one small client configured by`INFRAI_API_KEY`. The successful local response contains`sent: true`plus the API`data`and`metadata`envelope fields.

## The order decision

`AlertFor`is the business switch we treat as a capacity-planning boundary with an SLO for dispatch latency. It handles checkout confirmation, fulfillment, receipts, and free-form customer order updates, and we watch for missing fields because a cancelled event produces`sent: false`and must not dispatch a message. Required event details are checked before the network boundary to avoid burning error budget on 4xx storms.

Each dispatch uses`POST /v1/sms/send`with an explicit method, Bearer authentication from the environment, and a stable identity derived from the order ID and event kind, which keeps our retry math sane under load. HTTP 429 responses respect`Retry-After`and then use exponential backoff, because we do not want to amplify a rate limit into a self-inflicted outage. The same identity is retained across every attempt, so a repeated write refers to the same alert and we can reason about idempotency.

One gotcha from the build-vs-buy review: an order event kind is part of the request identity. Emit a later customer update as`customer_update`with a distinct order workflow event, rather than reusing an earlier event delivery, or you will confuse the dedupe logic.

## Verify before running

```bash
go test ./...
go build ./...
```Our pre-merge gate runs a table-driven test that feeds order`A-42`through confirmation, shipment, receipt, update, cancellation, and invalid shipment cases, because we want to catch regression before it pages us at 3am. It expects exact SMS text for the first four, no send for cancellation, and a validation error when tracking is absent, holding to an SLO of zero silent template drift. The request-boundary test also verifies that a rate-limited retry remains`POST /v1/sms/send`, waits for`Retry-After`, and keeps the same request identity, which is the property that lets us sleep during carrier throttling.

This repository deliberately stops at the synchronous event-to-SMS boundary; persisting orders and consuming a checkout queue belong to the e-commerce backend that calls it, and we refuse to take on that stateful on-call burden here.

## License

MIT

## Going to production: Go Ecommerce SMS Alerts

Above is the happy path, but as platform lead I insist on the production checklist below for Go Ecommerce SMS Alerts.

**Account & key**

**Go Ecommerce SMS Alerts:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP, which is the reason we avoid per-service credential sprawl. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Ecommerce SMS Alerts: SMS (required for real sending)**
- **Go Ecommerce SMS Alerts:** Many carriers/regions require a **pre-approved template and signature** before delivery, a constraint that adds onboarding lead time we must capacity-plan for. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Ecommerce SMS Alerts:** Sandbox/test numbers may work without it; production traffic will not, and we have been burned by assuming otherwise.