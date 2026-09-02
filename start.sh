#!/bin/sh
# 一键起控制台（Docker）：MySQL + platform（API + 前端 :5173）
set -e
cd "$(dirname "$0")"
docker compose up --build -d mysql platform
echo "console http://127.0.0.1:5173"
