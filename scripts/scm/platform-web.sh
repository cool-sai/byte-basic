#!/bin/sh
# SCM 编译脚本：在仓库根执行，产物写到 $SCM_OUT/$SCM_NAME
set -e
: "${SCM_OUT:?}"
: "${SCM_NAME:?}"
cd platform/web
npm ci
npm run build
stage=$(mktemp -d)
mkdir -p "$stage/minikitex"
cp -R dist/. "$stage/minikitex/"
tar -C "$stage" -cf "$SCM_OUT/$SCM_NAME" .
rm -rf "$stage"
