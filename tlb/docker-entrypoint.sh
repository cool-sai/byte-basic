#!/bin/sh
set -e
if [ -f /generated/nginx.conf ]; then
  cp /generated/nginx.conf /etc/nginx/nginx.conf
fi
exec nginx -g "daemon off;"
