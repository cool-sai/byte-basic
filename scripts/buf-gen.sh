#!/bin/sh
set -e
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
export PATH="$(go env GOPATH)/bin:$PATH"
buf generate
rm -rf gen/google
