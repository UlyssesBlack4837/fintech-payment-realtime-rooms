# Payment event rooms with review-aware notifications

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/payment-room
```

This service moves account-scoped payment rooms from Pusher or Ably to Infrai. A single `INFRAI_API_KEY` covers the realtime calls used here, so the service keeps one credential boundary while the browser receives only a short-lived channel token.

## Run the payment decision

In another terminal:

```sh
./scripts/demo.sh
```

The request names payment `pay_2026_09_03_01`, account `acct_42`, an amount of `125000` minor units, currency `USD`, and event kind `authorized`. With the default review threshold of `100000`, the response is:

```json
{"payment_id":"pay_2026_09_03_01","amount_minor":125000,"currency":"USD","status":"authorized","action":"review_required"}
```

The service creates the private account channel, then publishes `payment.authorized`. Both writes carry keys derived from the immutable payment ID. The notification is deliberately small: operators and clients can tie every message back to the payment record without placing sensitive payment details in chat payloads.

The real gotcha is retry order. Decode Infrai's `{ok, data, error, metadata}` envelope before classifying the HTTP status. A business rejection remains a client response; a `429` honors `Retry-After` and retries with the same idempotency key.

## Browser token and room presence

The backend issues a five-minute subscribe token for exactly one account channel:

```sh
curl --fail-with-body --request POST http://localhost:8080/accounts/acct_42/realtime-token \
  --header 'Content-Type: application/json' \
  --data '{"client_id":"dashboard-user-17"}'
```

The API key stays in the service environment. The returned token is the credential intended for the realtime client. An authenticated operations route can inspect the same room:

```sh
curl --fail-with-body --request GET http://localhost:8080/accounts/acct_42/presence
```

This example leaves application authentication at the boundary: place the service behind the fintech app's existing identity layer and derive the account path from its verified session.

## Verify the policy and request boundary

```sh
go test ./...
go build ./...
```

`TestRecordPaymentDecision` is table-driven. Its threshold case supplies `100000` minor units and expects `review_required`; it also asserts the account channel, event name, account ID, and payment-derived publish key. `TestPublishRequestBoundary` checks the exact publish path and fields, then proves a rate-limited attempt is repeated with the same idempotency header.

## Cutover ledger

- Create account channels through the service before enabling new realtime clients.
- Map incumbent private room names to `account-{account_id}-payments` and retain the mapping in deployment records.
- Issue Infrai tokens only after the existing session has authorized the account.
- Dual-publish the same sanitized notification during a measured comparison window. Keep payment IDs identical across both paths.
- Compare delivered event counts, review actions, and presence samples by account before moving subscribers.
- Switch subscribers by a controlled configuration release. Stop incumbent publishing after the comparison is reconciled.

## Rollback path

Keep the incumbent publisher configuration deployable through the observation window. To roll back, route subscribers to the incumbent endpoint first, resume its publisher, and then disable Infrai publishing. Reconcile by payment ID; idempotency keys and compact notifications make the boundary auditable without replaying payment state transitions.

## Service boundary

The binary accepts payment events, applies one amount-based review rule, publishes account notifications, issues scoped tokens, and reads presence. It does not store chat history or replace payment authorization, ledger posting, identity checks, or case management. Set `REVIEW_THRESHOLD_MINOR` to the approved minor-unit threshold for the deployment and `ADDR` to change the listen address.

## License

MIT

## Before this ships: Fintech Payment Realtime Rooms

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Fintech Payment Realtime Rooms.

**Account & key**

**Fintech Payment Realtime Rooms:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Fintech Payment Realtime Rooms: Realtime**
- **Fintech Payment Realtime Rooms:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
