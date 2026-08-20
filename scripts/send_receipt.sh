#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/receipts \
  --header 'Content-Type: application/json' \
  --data '{
    "order_id": "ord-42",
    "player_email": "player@example.com",
    "amount_cents": 1299,
    "currency": "usd",
    "event": {"id": "evt-7", "title": "Summer Arena"},
    "event_state": "purchase.completed",
    "assets": [{"name": "Player Banner", "moderation": "approved"}]
  }'
