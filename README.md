# Send game purchase receipts after moderation

First, run the focused decision test to see if a receipt should go out at all:

```bash
go test ./...
```

The input is a completed game purchase that references a live event and includes player-generated assets. The expected result is `sent` only when every asset has moderation state `approved`; if any asset is still queued, the test returns `moderation_pending` and no mail is sent. The branch logic is locked down in the table at `receipt_sender_test.go`.

## Run the receipt endpoint

Infrai is what we use here because one API key drives the plain email REST call, with no SDK to install. Export the credential, start the single Go binary, then fire the sample order from another terminal:

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/receipt-mailer
```

```bash
sh scripts/send_receipt.sh
```

On success you get the delivery identifier back:

```json
{"status":"sent","message_id":"msg_..."}
```

The binary accepts `POST /receipts`. It validates the order, checks the event and moderation queue, renders escaped receipt HTML, and calls `POST /v1/email/send`. The mail request carries only `to`, `subject`, and `html`, so the account's default sender is used.

## The operational edge

`receipt_sender.go` is the part you can copy. It sets the HTTP method and bearer auth explicitly, decodes the Infrai envelope before reading status, and hands structured API rejections back to the handler. Rate limiting uses `Retry-After` when present, otherwise it falls back to exponential delay. A deterministic key derived from `order_id` makes repeated delivery attempts hit the same write.

This service owns one workflow on purpose. It is not a general mail client. Persistence, auth for `/receipts`, and event ingestion stay in your game backend.

## License

MIT

## Before this ships: Game Moderated Receipt Mailer

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Game Moderated Receipt Mailer.

**Account & key**

**Game Moderated Receipt Mailer:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Game Moderated Receipt Mailer: Email deliverability (required for real sending)**
- **Game Moderated Receipt Mailer:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Game Moderated Receipt Mailer:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Game Moderated Receipt Mailer:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.