---
name: telegram
description: "Bidirectional Telegram messaging via Bot API — send messages, poll for updates, reply via headless agent."
version: 147
tags: [telegram, bot, messaging]
scope: user
auto_active: true
---

## Purpose

Bidirectional Telegram messaging via Bot API — send messages, poll for updates, reply via headless agent.

## Scripts

### telegram_send
kind: tool
schema: {"type":"object","properties":{"text":{"type":"string","description":"Message text"},"chat_id":{"type":"string","description":"Optional chat id override"},"reply_to":{"type":"integer","description":"Optional reply-to message id"}},"required":["text"]}

```bash
TOKEN=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["bot_token"],"limit":1}' | yesmem json -r '.rows[0].value')
CHAT_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["chat_id"],"limit":1}' | yesmem json -r '.rows[0].value')
curl -4 -s -m 10 "https://api.telegram.org/bot${TOKEN}/sendMessage" -d "chat_id=${CHAT_ID}" -d "text=${TEXT}" -d "parse_mode=Markdown"
```

### telegram_poll
kind: handler

```bash
exec 2>>/tmp/tcombined.log
log_p() { printf "[%s] %s\n" "$(date -Is)" "$*" >> /tmp/tcombined.log; }

TOKEN=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["bot_token"],"limit":1}' | yesmem json -r '.rows[0].value')
CHAT_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["chat_id"],"limit":1}' | yesmem json -r '.rows[0].value')
MODEL=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["reply_model"],"limit":1}' | yesmem json -r '.rows[0].value // "claude-opus-4-7"')
SYSPROMPT=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["system_prompt"],"limit":1}' | yesmem json -r '.rows[0].value // "Du bist ein hilfreicher Assistent."')

store '{"capability":"telegram","action":"create_table","table":"reply_errors","columns":"[{\"name\":\"row_id\",\"type\":\"INTEGER\"},{\"name\":\"sender\",\"type\":\"TEXT\"},{\"name\":\"message_text\",\"type\":\"TEXT\"},{\"name\":\"stage\",\"type\":\"TEXT\"},{\"name\":\"exit_code\",\"type\":\"INTEGER\"},{\"name\":\"stderr\",\"type\":\"TEXT\"},{\"name\":\"stdout\",\"type\":\"TEXT\"},{\"name\":\"model\",\"type\":\"TEXT\"}]"}' > /dev/null 2>&1 || true

DEADLINE=$(($(date +%s) + 55))
log_p "=== combined fire start, deadline=${DEADLINE} ==="
POLL_CALLS=0; STORED=0; REPLIES=0; FAILS=0

log_reply_error() {
  STAGE="$1"; EXIT_CODE="$2"; STDERR_T="$3"; STDOUT_T="$4"
  EP=$(yesmem json -n --argjson row_id "${ROW_ID:-0}" --arg sender "${SENDER:-}" --arg message_text "${TEXT:-}" --arg stage "$STAGE" --argjson exit_code "${EXIT_CODE:-0}" --arg stderr "${STDERR_T:0:4000}" --arg stdout "${STDOUT_T:0:4000}" --arg model "${MODEL:-}" '{capability:"telegram",action:"upsert",table:"reply_errors",data:{row_id:$row_id,sender:$sender,message_text:$message_text,stage:$stage,exit_code:$exit_code,stderr:$stderr,stdout:$stdout,model:$model}}')
  echo "$EP" | while IFS= read -r p; do store "$p" > /dev/null; done
  log_p "ERR stage=$STAGE exit=$EXIT_CODE row=${ROW_ID:-?}"
  FAILS=$((FAILS + 1))
}

while [ $(date +%s) -lt $DEADLINE ]; do
  REMAIN=$((DEADLINE - $(date +%s)))
  if [ $REMAIN -le 4 ]; then break; fi
  OFFSET=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["offset"],"limit":1}' | yesmem json -r '.rows[0].value // "0"')
  TIMEOUT=12
  if [ $REMAIN -lt 14 ]; then TIMEOUT=$((REMAIN - 2)); fi
  RESPONSE=$(curl -4 -sS -m $((TIMEOUT + 3)) "https://api.telegram.org/bot${TOKEN}/getUpdates?offset=${OFFSET}&timeout=${TIMEOUT}")
  POLL_CALLS=$((POLL_CALLS + 1))
  COUNT=$(echo "$RESPONSE" | yesmem json '.result | length')
  if [ -n "$COUNT" ] && [ "$COUNT" != "0" ] && [ "$COUNT" != "null" ]; then
    MAX_ID=$OFFSET
    for i in $(seq 0 $((COUNT - 1))); do
      UPDATE=$(echo "$RESPONSE" | yesmem json ".result[$i]")
      UPD_ID=$(echo "$UPDATE" | yesmem json '.update_id')
      PAYLOAD=$(echo "$UPDATE" | yesmem json '{capability:"telegram",action:"upsert",table:"updates",data:{telegram_id:.update_id,chat_id:.message.chat.id,sender:(.message.from.first_name // "unknown"),text:(.message.text // ""),direction:"in",processed:0,date:.message.date}}')
      echo "$PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
      STORED=$((STORED + 1))
      if [ "$UPD_ID" -ge "$MAX_ID" ]; then MAX_ID=$((UPD_ID + 1)); fi
    done
    OFFSET_PAYLOAD=$(yesmem json -n --arg key "offset" --arg value "$MAX_ID" '{capability:"telegram",action:"upsert",table:"config",data:{id:12,key:$key,value:$value}}')
    echo "$OFFSET_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
  fi
  REMAIN=$((DEADLINE - $(date +%s)))
  if [ $REMAIN -le 30 ]; then continue; fi
  MSG=$(store '{"capability":"telegram","action":"query","table":"updates","where":"processed=0","limit":1}')
  RCOUNT=$(echo "$MSG" | yesmem json '.count')
  if [ -z "$RCOUNT" ] || [ "$RCOUNT" = "0" ] || [ "$RCOUNT" = "null" ]; then continue; fi
  ROW_ID=$(echo "$MSG" | yesmem json '.rows[0].id')
  TEXT=$(echo "$MSG" | yesmem json -r '.rows[0].text')
  SENDER=$(echo "$MSG" | yesmem json -r '.rows[0].sender')
  log_p "reply attempt row=$ROW_ID sender=$SENDER text=${TEXT:0:60}"
  MARK_PAYLOAD=$(yesmem json -n --argjson id "$ROW_ID" '{capability:"telegram",action:"upsert",table:"updates",data:{id:$id,processed:1}}')
  echo "$MARK_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
  SESSION_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["reply_session"],"limit":1}' | yesmem json -r '.rows[0].value // empty')
  STDERR_FILE=$(mktemp)
  if [ -n "$SESSION_ID" ]; then
    RESULT=$(claude -p --tools "" --model "$MODEL" --output-format json --resume "$SESSION_ID" "$SYSPROMPT Nachricht von $SENDER: $TEXT" 2>"$STDERR_FILE")
  else
    RESULT=$(claude -p --tools "" --model "$MODEL" --output-format json "$SYSPROMPT Nachricht von $SENDER: $TEXT" 2>"$STDERR_FILE")
  fi
  CLAUDE_EXIT=$?
  STDERR_TEXT=$(cat "$STDERR_FILE")
  rm -f "$STDERR_FILE"
  if [ "$CLAUDE_EXIT" -ne 0 ]; then log_reply_error "claude_exit" "$CLAUDE_EXIT" "$STDERR_TEXT" "$RESULT"; continue; fi
  if ! echo "$RESULT" | yesmem json -e '.' > /dev/null 2>&1; then log_reply_error "invalid_json" 0 "$STDERR_TEXT" "$RESULT"; continue; fi
  REPLY=$(echo "$RESULT" | yesmem json -r '.result // empty')
  NEW_SESSION=$(echo "$RESULT" | yesmem json -r '.session_id // empty')
  if [ -n "$NEW_SESSION" ]; then
    SESSION_PAYLOAD=$(yesmem json -n --arg key "reply_session" --arg value "$NEW_SESSION" '{capability:"telegram",action:"upsert",table:"config",data:{key:$key,value:$value}}')
    echo "$SESSION_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
  fi
  if [ -z "$REPLY" ]; then log_reply_error "empty_reply" 0 "$STDERR_TEXT" "$RESULT"; continue; fi
  SEND_RESPONSE=$(curl -4 -s -m 10 "https://api.telegram.org/bot${TOKEN}/sendMessage" -d "chat_id=${CHAT_ID}" --data-urlencode "text=${REPLY}")
  CURL_EXIT=$?
  if [ "$CURL_EXIT" -ne 0 ]; then log_reply_error "send_message_curl" "$CURL_EXIT" "curl sendMessage failed" "$SEND_RESPONSE"; continue; fi
  if ! echo "$SEND_RESPONSE" | yesmem json -e '.' > /dev/null 2>&1; then log_reply_error "send_message_invalid_json" 0 "telegram returned invalid JSON" "$SEND_RESPONSE"; continue; fi
  if [ "$(echo "$SEND_RESPONSE" | yesmem json -r '.ok // false')" != "true" ]; then
    TG_DESC=$(echo "$SEND_RESPONSE" | yesmem json -r '.description // "telegram sendMessage failed"')
    log_reply_error "send_message_api" 1 "$TG_DESC" "$SEND_RESPONSE"; continue
  fi
  REPLIES=$((REPLIES + 1))
  log_p "replied row=$ROW_ID -> ${REPLY:0:60}"
done

log_p "=== fire end polls=$POLL_CALLS stored=$STORED replies=$REPLIES fails=$FAILS ==="
echo "Combined fire: polls=$POLL_CALLS stored=$STORED replies=$REPLIES fails=$FAILS"
```

### telegram_reply
kind: handler

```bash
MSG=$(store '{"capability":"telegram","action":"query","table":"updates","where":"processed=0","limit":1}')
COUNT=$(echo "$MSG" | yesmem json '.count')
if [ "$COUNT" = "0" ] || [ "$COUNT" = "null" ]; then exit 0; fi
store '{"capability":"telegram","action":"create_table","table":"reply_errors","columns":"[{\"name\":\"row_id\",\"type\":\"INTEGER\"},{\"name\":\"sender\",\"type\":\"TEXT\"},{\"name\":\"message_text\",\"type\":\"TEXT\"},{\"name\":\"stage\",\"type\":\"TEXT\"},{\"name\":\"exit_code\",\"type\":\"INTEGER\"},{\"name\":\"stderr\",\"type\":\"TEXT\"},{\"name\":\"stdout\",\"type\":\"TEXT\"},{\"name\":\"model\",\"type\":\"TEXT\"}]"}' > /dev/null 2>&1 || true
ROW_ID=$(echo "$MSG" | yesmem json '.rows[0].id')
TEXT=$(echo "$MSG" | yesmem json -r '.rows[0].text')
SENDER=$(echo "$MSG" | yesmem json -r '.rows[0].sender')
log_to_file() { printf '[%s] %s\n' "$(date -Is)" "$*" >> /tmp/treply.log; }
log_to_file "=== fire row=$ROW_ID sender=$SENDER text=${TEXT:0:80} ==="
log_reply_error() {
  STAGE="$1"; EXIT_CODE="$2"; STDERR_TEXT="$3"; STDOUT_TEXT="$4"
  ERROR_PAYLOAD=$(yesmem json -n --argjson row_id "${ROW_ID:-0}" --arg sender "$SENDER" --arg message_text "$TEXT" --arg stage "$STAGE" --argjson exit_code "${EXIT_CODE:-0}" --arg stderr "${STDERR_TEXT:0:4000}" --arg stdout "${STDOUT_TEXT:0:4000}" --arg model "${MODEL:-}" '{capability:"telegram",action:"upsert",table:"reply_errors",data:{row_id:$row_id,sender:$sender,message_text:$message_text,stage:$stage,exit_code:$exit_code,stderr:$stderr,stdout:$stdout,model:$model}}')
  echo "$ERROR_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
  log_to_file "ERR stage=$STAGE exit=$EXIT_CODE stderr=${STDERR_TEXT:0:300} stdout=${STDOUT_TEXT:0:600}"
}
MARK_PAYLOAD=$(yesmem json -n --argjson id "$ROW_ID" '{capability:"telegram",action:"upsert",table:"updates",data:{id:$id,processed:1}}')
echo "$MARK_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
TOKEN=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["bot_token"],"limit":1}' | yesmem json -r '.rows[0].value')
CHAT_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["chat_id"],"limit":1}' | yesmem json -r '.rows[0].value')
MODEL=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["reply_model"],"limit":1}' | yesmem json -r '.rows[0].value // "claude-opus-4-7"')
SYSPROMPT=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["system_prompt"],"limit":1}' | yesmem json -r '.rows[0].value // "Du bist ein hilfreicher Assistent."')
SESSION_ID=$(store '{"capability":"telegram","action":"query","table":"config","where":"key=?","args":["reply_session"],"limit":1}' | yesmem json -r '.rows[0].value // empty')
STDERR_FILE=$(mktemp)
if [ -n "$SESSION_ID" ]; then
  RESULT=$(claude -p --tools "" --model "$MODEL" --output-format json --resume "$SESSION_ID" "$SYSPROMPT Nachricht von $SENDER: $TEXT" 2>"$STDERR_FILE")
else
  RESULT=$(claude -p --tools "" --model "$MODEL" --output-format json "$SYSPROMPT Nachricht von $SENDER: $TEXT" 2>"$STDERR_FILE")
fi
CLAUDE_EXIT=$?
STDERR_TEXT=$(cat "$STDERR_FILE")
rm -f "$STDERR_FILE"
if [ "$CLAUDE_EXIT" -ne 0 ]; then
  log_reply_error "claude_exit" "$CLAUDE_EXIT" "$STDERR_TEXT" "$RESULT"
  echo "telegram_reply: claude failed for row $ROW_ID (exit $CLAUDE_EXIT)" | tee -a /tmp/treply.log >&2
  exit 0
fi
if ! echo "$RESULT" | yesmem json -e '.' > /dev/null 2>&1; then
  log_reply_error "invalid_json" 0 "$STDERR_TEXT" "$RESULT"
  echo "telegram_reply: invalid claude JSON for row $ROW_ID" | tee -a /tmp/treply.log >&2
  exit 0
fi
REPLY=$(echo "$RESULT" | yesmem json -r '.result // empty')
NEW_SESSION=$(echo "$RESULT" | yesmem json -r '.session_id // empty')
if [ -n "$NEW_SESSION" ]; then
  SESSION_PAYLOAD=$(yesmem json -n --arg key "reply_session" --arg value "$NEW_SESSION" '{capability:"telegram",action:"upsert",table:"config",data:{key:$key,value:$value}}')
  echo "$SESSION_PAYLOAD" | while IFS= read -r p; do store "$p" > /dev/null; done
fi
if [ -z "$REPLY" ]; then
  log_reply_error "empty_reply" 0 "$STDERR_TEXT" "$RESULT"
  echo "telegram_reply: empty reply for row $ROW_ID" | tee -a /tmp/treply.log >&2
  exit 0
fi
SEND_RESPONSE=$(curl -4 -s -m 10 "https://api.telegram.org/bot${TOKEN}/sendMessage" -d "chat_id=${CHAT_ID}" --data-urlencode "text=${REPLY}")
CURL_EXIT=$?
if [ "$CURL_EXIT" -ne 0 ]; then
  log_reply_error "send_message_curl" "$CURL_EXIT" "curl sendMessage failed" "$SEND_RESPONSE"
  echo "telegram_reply: sendMessage curl failed for row $ROW_ID (exit $CURL_EXIT)" | tee -a /tmp/treply.log >&2
  exit 0
fi
if ! echo "$SEND_RESPONSE" | yesmem json -e '.' > /dev/null 2>&1; then
  log_reply_error "send_message_invalid_json" 0 "telegram returned invalid JSON" "$SEND_RESPONSE"
  echo "telegram_reply: sendMessage invalid JSON for row $ROW_ID" | tee -a /tmp/treply.log >&2
  exit 0
fi
if [ "$(echo "$SEND_RESPONSE" | yesmem json -r '.ok // false')" != "true" ]; then
  TG_DESCRIPTION=$(echo "$SEND_RESPONSE" | yesmem json -r '.description // "telegram sendMessage failed"')
  log_reply_error "send_message_api" 1 "$TG_DESCRIPTION" "$SEND_RESPONSE"
  echo "telegram_reply: sendMessage API failed for row $ROW_ID: $TG_DESCRIPTION" | tee -a /tmp/treply.log >&2
  exit 0
fi
echo "Replied to $SENDER: ${TEXT:0:40}"
```

## Database

```sql
CREATE TABLE IF NOT EXISTS cap_telegram__config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cap_telegram__updates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE,
    chat_id INTEGER,
    sender TEXT,
    text TEXT,
    direction TEXT,
    processed INTEGER NOT NULL DEFAULT 0,
    date INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cap_telegram__reply_errors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    row_id INTEGER,
    sender TEXT,
    message_text TEXT,
    stage TEXT,
    exit_code INTEGER,
    stderr TEXT,
    stdout TEXT,
    model TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```
