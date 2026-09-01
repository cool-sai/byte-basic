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
go build -o bin/etcdui ./example/etcdui

ver=1.20.5
if [ ! -x bin/consul ]; then
  zip=/tmp/consul_${ver}.zip
  url="https://releases.hashicorp.com/consul/${ver}/consul_${ver}_linux_${GOARCH}.zip"
  echo "downloading $url"
  curl -fsSL -o "$zip" "$url"
  unzip -o -d bin "$zip" consul
  chmod +x bin/consul
fi

etcd_ver=3.5.16
if [ ! -x bin/etcd ]; then
  tgz=/tmp/etcd_${etcd_ver}.tar.gz
  url="https://github.com/etcd-io/etcd/releases/download/v${etcd_ver}/etcd-v${etcd_ver}-linux-${GOARCH}.tar.gz"
  echo "downloading $url"
  curl -fsSL -o "$tgz" "$url"
  tar -xzf "$tgz" -C /tmp
  cp "/tmp/etcd-v${etcd_ver}-linux-${GOARCH}/etcd" bin/etcd
  chmod +x bin/etcd
fi

exec docker compose up --build --remove-orphans "$@"
