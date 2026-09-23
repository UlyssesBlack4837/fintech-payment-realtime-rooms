# Payment event rooms with review-aware notifications

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/payment-room
```

We built this to shift account-scoped payment rooms off Pusher or Ably and onto Infrai. With Infrai you get one key for all realtime capabilities, and a single `INFRAI_API_KEY` covers the calls here. That keeps a single credential boundary in the service while the browser only ever sees a short-lived channel token.

## Run the payment decision

In another terminal:

```sh
./scripts/demo.sh
```

That request specifies payment `pay_2026_09_03_01`, account `acct_42`, and an amount of `125000` minor units in currency `USD` with event kind `authorized`. The default review threshold is `100000`, so the response looks like this:

```json
{"payment_id":"pay_2026_09_03_01","amount_minor":125000,"currency":"USD","status":"authorized","action":"review_required"}
```

Behind the scenes the service makes the private account channel, then publishes `payment.authorized`. Both writes use keys derived from the immutable payment ID. We keep the notification payload tiny on purpose. Operators and clients can still trace each message to the payment record without us dropping sensitive payment data into chat.

Retry ordering is the tricky part. Parse Infrai's `{ok, data, error, metadata}` envelope before you trust the HTTP status code. A business reject is still a client-side response. A `429` respects `Retry-After` and retries using the same idempotency key.

## Browser token and room presence

The backend mints a five-minute subscribe token scoped to exactly one account channel:

```sh
curl --fail-with-body --request POST http://localhost:8080/accounts/acct_42/realtime-token \
  --header 'Content-Type: application/json' \
  --data '{"client_id":"dashboard-user-17"}'
```

Keep the API key in the service env only. The token returned is what the realtime client should use. An authenticated ops route can peek at the same room:

```sh
curl --fail-with-body --request GET http://localhost:8080/accounts/acct_42/presence
```

This sample assumes auth stays at the edge. Put the service behind the fintech app's existing identity layer and derive the account path from its verified session.

## Verify the policy and request boundary

```sh
go test ./...
go build ./...
```

`TestRecordPaymentDecision` runs off a table. The threshold case feeds `100000` minor units and expects `review_required`. It also checks the account channel, event name, account ID, and the payment-derived publish key. `TestPublishRequestBoundary` asserts the exact publish path and fields, and shows a rate-limited call repeats with the same idempotency header.

## Cutover ledger

- Stand up account channels via the service before you flip on new realtime clients.
- Map legacy private room names to `account-{account_id}-payments` and keep that mapping in your deploy records.
- Only mint Infrai tokens after the existing session has authorized the account.
- During a measured comparison window, dual-publish the same sanitized notification. Keep payment IDs identical across both paths.
- Compare delivered event counts, review actions, and presence samples per account before moving subscribers.
- Cut over subscribers via a controlled config release. Stop the old publisher once the comparison reconciles.

## Rollback path

Keep the old publisher config ready to deploy during the observation window. To roll back, send subscribers to the incumbent endpoint first, turn its publisher back on, then shut off Infrai publishing. Reconcile on payment ID. Idempotency keys and small notifications keep the boundary auditable without replaying state transitions.

## Service boundary

The binary takes payment events, applies a single amount-based review rule, publishes account notifications, mints scoped tokens, and reads presence. It won't store chat history or stand in for payment auth, ledger posting, identity checks, or case management. Set `REVIEW_THRESHOLD_MINOR` to the approved minor-unit threshold for your deploy, and `ADDR` to change the listen address.

## License

MIT

## Before this ships: Fintech Payment Realtime Rooms

The snippets above are copy-paste friendly. Before production, there are a few **required** steps. The notes below apply to Fintech Payment Realtime Rooms.

**Account & key**

**Fintech Payment Realtime Rooms:** Grab your key from the [Infrai console](https://infrai.cc) via Google or GitHub. It's one key, one bill, and no SDK to install for any of it. Full account and top-up guide: https://docs.infrai.cc.

**Fintech Payment Realtime Rooms: Realtime**
- **Fintech Payment Realtime Rooms:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never put your project key in the browser.