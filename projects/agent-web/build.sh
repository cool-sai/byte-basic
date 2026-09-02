#!/bin/sh
# SCM 编译脚本：在仓库根执行，产物写到 $SCM_OUT/$SCM_NAME
set -e
: "${SCM_OUT:?}"
: "${SCM_NAME:?}"
cd "$(dirname "$0")"
npm ci
npm run build
tar -C dist -cf "$SCM_OUT/$SCM_NAME" .
