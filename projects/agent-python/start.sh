#!/bin/sh
exec gunicorn -b "${LISTEN:-0.0.0.0:80}" app:app
