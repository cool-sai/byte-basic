#!/bin/sh
# 一键起控制台（Docker）：MySQL + platform + TLB（对外 :80）
set -e
cd "$(dirname "$0")"
docker compose up --build -d mysql platform tlb
echo "console http://127.0.0.1/scm"
echo "direct  http://127.0.0.1:5173"
