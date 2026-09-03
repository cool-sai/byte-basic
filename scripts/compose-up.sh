#!/bin/sh
# 本机交叉编译 Linux 二进制，再 docker compose 发布。
#   ./scripts/compose-up.sh -d           全量
#   ./scripts/compose-up.sh -d gateway   只发网关
set -e
cd "$(dirname "$0")/.."
mkdir -p bin

# scratch 镜像没有 glibc，必须静态编成 linux
export CGO_ENABLED=0 GOOS=linux
case "$(docker info --format '{{.Architecture}}')" in
aarch64|arm64) export GOARCH=arm64 ;;
*) export GOARCH=amd64 ;;
esac

# 每个函数只编一次（user 多副本共用 bin/user）
build_user() { [ -n "$built_user" ] && return; built_user=1; go build -o bin/user ./example/server; }
build_order() { [ -n "$built_order" ] && return; built_order=1; go build -o bin/order ./example/order; }
build_etcdui() { [ -n "$built_etcdui" ] && return; built_etcdui=1; go build -o bin/etcdui ./example/etcdui; }
build_gateway() { [ -n "$built_gateway" ] && return; built_gateway=1; go build -o bin/gateway ./example/gateway; }

# "$@" 是所有参数：-d / --build 是旗标，剩下的才是服务名
svcs=
for a in "$@"; do
  case "$a" in
    -*) ;;
    *) svcs="$svcs $a" ;;
  esac
done

if [ -z "$svcs" ]; then
  build_user
  build_order
  build_etcdui
  build_gateway
else
  for s in $svcs; do
    case "$s" in
      user) build_user ;;
      order) build_order ;;
      etcdui) build_etcdui ;;
      gateway) build_gateway ;;
    esac
  done
fi

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

if [ -n "$svcs" ]; then
  # 只发点名的服务，--no-deps 不重建它依赖的 user/order/consul
  docker compose build $svcs
  exec docker compose up --no-deps --remove-orphans "$@"
fi
exec docker compose up --build --remove-orphans "$@"
