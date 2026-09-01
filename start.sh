#!/bin/sh
# 一键起控制台：MySQL（已有则跳过）+ API :8081 + 前端 :5173
set -e
cd "$(dirname "$0")"

docker compose up -d mysql

go run ./platform/server &
api=$!
cleanup() {
  trap - INT TERM EXIT
  kill "$api" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

cd platform/web
[ -d node_modules ] || npm install
npm run dev
