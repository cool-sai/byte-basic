#!/bin/sh
set -e
cd "$(dirname "$0")/.."
mkdir -p bin
export CGO_ENABLED=0 GOOS=linux
case "$(docker info --format '{{.Architecture}}')" in
aarch64|arm64) export GOARCH=arm64 ;;
*) export GOARCH=amd64 ;;
esac
go build -o bin/user ./example/server
go build -o bin/order ./example/order
go build -o bin/order-client ./example/order_client
go build -o bin/registry ./example/registry
exec docker compose up --build "$@"
