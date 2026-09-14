#!/bin/sh
# 一键起控制台（Docker）：MySQL + platform + TLB（对外 :80）
# 首页走控制台 SCM / Deploy，不在这份 compose 里。
set -e
cd "$(dirname "$0")"
docker compose up --build -d mysql platform platform-web tlb
echo "console http://127.0.0.1/minikitex/"
