#!/bin/sh
# Usage: ./scripts/config-put.sh user/name_suffix '!!!'
set -e
key=$1
val=$2
if [ -z "$key" ]; then
  echo "usage: $0 <key> <value>" >&2
  exit 1
fi
enc() { printf '%s' "$1" | base64 | tr -d '\n'; }
curl -sf http://127.0.0.1:2379/v3/kv/put \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"$(enc "$key")\",\"value\":\"$(enc "$val")\"}"
echo
