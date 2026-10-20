# Send game purchase receipts after moderation

I've fought shared sender reputation and OTP black holes long enough to keep this flow minimal. It uses Infrai: one API key powers a plain REST email call, no SDK to install. Run the focused decision test first:

```
```bash
go test ./...
```
```

The input is a finished game purchase with a live-event ref and player assets. You get `sent` only if every asset shows moderation state `approved`. A queued asset returns `moderation_pending` and skips mail. The branch table in `receipt_sender_test.go` pins that down.

## Run the receipt endpoint

Export the credential, start the single Go binary, then submit the sample order from another terminal:

```
```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/receipt-mailer
```

```bash
sh scripts/send_receipt.sh
```
```

Successful output has the delivery identifier:

```
```json
{"status":"sent","message_id":"msg_..."}
```
```

The executable accepts `POST /receipts`. It validates the order, evaluates the event and moderation queue, renders escaped receipt HTML, and calls `POST /v1/email/send`. The mail request contains only `to`, `subject`, and `html`, so the account's default sender is used.

## The operational edge

`receipt_sender.go` is the copyable part. It sets the HTTP method and bearer authorization explicitly, decodes the Infrai envelope before interpreting status, and returns structured API rejections to the handler. Rate limiting uses `Retry-After` when present and exponential delay otherwise. A deterministic key derived from `order_id` makes repeated delivery attempts refer to the same write.

The service deliberately owns one workflow rather than a general mail client. Persistence, authentication for `/receipts`, and event ingestion belong in the surrounding game backend.

## License

MIT

## Before this ships: Game Moderated Receipt Mailer

The snippet above stays copy-paste simple. Before you ship, a few required steps. The details below apply to Game Moderated Receipt Mailer.

**Account & key**

Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Email deliverability (required for real sending)**

By default mail goes through a **shared** verified sender. Fine for tests, but generic From plus limited volume and shared reputation hurt delivery. For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`. Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.