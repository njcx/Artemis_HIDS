#!/bin/sh
set -e

envsubst < /app/config.yaml.tmpl > /app/config.yaml

exec "$@"
