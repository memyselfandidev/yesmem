---
name: telegram
description: "Bidirectional Telegram messaging via Bot API"
version: 1
tags: [messaging, telegram, notification]
runtime: bash
scope: user
tested: false
auto_active: false
requires: [store, web]
---

## Purpose

Send and receive Telegram messages via Bot API. Config (bot_token, chat_id) lives in cap_store. Used by scheduled jobs for polling and by other caps for notifications.

## Script

```bash
TOKEN=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":"[\"bot_token\"]","limit":1}' | jq -r '.[0].value')
CHAT_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":"[\"chat_id\"]","limit":1}' | jq -r '.[0].value')
curl -s -m 10 "https://api.telegram.org/bot${TOKEN}/sendMessage" -d "chat_id=${CHAT_ID}" -d "text=${TEXT}" -d "parse_mode=Markdown"
```

## Database

```sql
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE messages (
    telegram_id INTEGER UNIQUE,
    chat_id INTEGER,
    direction TEXT NOT NULL,
    sender TEXT,
    text TEXT,
    processed INTEGER DEFAULT 0
);
```

## Actions

### Setup

1. Ask the user for their Telegram Bot Token (create one via @BotFather)
2. Ask for the Chat ID (use @userinfobot or @getidsbot to find it)
3. Store bot_token: `cap_store({capability: "telegram", action: "upsert", table: "config", data: '{"key": "bot_token", "value": "<TOKEN>"}'})`
4. Store chat_id: `cap_store({capability: "telegram", action: "upsert", table: "config", data: '{"key": "chat_id", "value": "<CHAT_ID>"}'})`
5. Verify with a test message: `cap_store({capability: "telegram", action: "query", table: "config", where: "key = ?", args: '["bot_token"]'})`

IMPORTANT: After successful setup, mark as complete: `cap_store({capability: "telegram", action: "upsert", table: "config", data: '{"key": "_setup_complete", "value": "true"}'})`

### Teardown

1. Remove scheduled jobs for this cap via `schedule({action: "delete", id: "<job-id>"})`
2. Delete config: `cap_store({capability: "telegram", action: "delete", table: "config"})`
3. Remove setup marker to re-enable setup instructions
