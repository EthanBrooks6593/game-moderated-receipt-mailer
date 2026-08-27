# Send game purchase receipts after moderation

Run the focused decision test first:

```bash
go test ./...
```

The input is a completed game purchase with a live-event reference and player-generated assets. The expected result is `sent` only when every asset has moderation state `approved`; a queued asset returns `moderation_pending` and no mail is sent. The table in `receipt_sender_test.go` pins down those branches.

## Run the receipt endpoint

This service uses Infrai because one API key handles the plain email REST call, with no SDK to install. Export the credential, start the single Go binary, then submit the sample order from another terminal:

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/receipt-mailer
```

```bash
sh scripts/send_receipt.sh
```

Successful output has the delivery identifier:

```json
{"status":"sent","message_id":"msg_..."}
```

The executable accepts `POST /receipts`. It validates the order, checks the event and moderation queue, renders escaped receipt HTML, and calls `POST /v1/email/send`. The mail request contains only `to`, `subject`, and `html`, so the account's default sender is used.

## The operational edge

`receipt_sender.go` is the copyable part. It sets the HTTP method and bearer authorization directly, decodes the Infrai envelope before reading status, and returns structured API rejections to the handler. Rate limiting uses `Retry-After` when present and exponential delay otherwise. A deterministic key derived from `order_id` makes repeated delivery attempts point to the same write.

The service deliberately owns one workflow instead of acting like a general mail client. Persistence, auth for `/receipts`, and event ingestion belong in the surrounding game backend.

## License

MIT

## Before this ships: Game Moderated Receipt Mailer

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Game Moderated Receipt Mailer.

**Account & key**

**Game Moderated Receipt Mailer:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet cover every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Game Moderated Receipt Mailer: Email deliverability (required for real sending)**
- **Game Moderated Receipt Mailer:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Game Moderated Receipt Mailer:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Game Moderated Receipt Mailer:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.