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

ver=1.20.5
if [ ! -x bin/consul ]; then
  zip=/tmp/consul_${ver}.zip
  url="https://releases.hashicorp.com/consul/${ver}/consul_${ver}_linux_${GOARCH}.zip"
  echo "downloading $url"
  curl -fsSL -o "$zip" "$url"
  unzip -o -d bin "$zip" consul
  chmod +x bin/consul
fi

exec docker compose up --build --remove-orphans "$@"
