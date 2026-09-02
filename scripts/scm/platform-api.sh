#!/bin/sh
# SCM 编译脚本：在仓库根执行，产物写到 $SCM_OUT/$SCM_NAME
set -e
: "${SCM_OUT:?}"
: "${SCM_NAME:?}"
go build -o "$SCM_OUT/$SCM_NAME" ./platform/server
