#!/bin/sh
# Generate runtime env config from environment variables.
# This file is written to the nginx document root before nginx starts,
# so the browser loads it before the app bundle.
#
# Values are JSON-encoded via python3 to handle special characters
# (quotes, backslashes, newlines) that would break naive shell interpolation.

set -eu

json_encode() {
  printf '%s' "$1" | python3 -c "import json,sys; print(json.dumps(sys.stdin.read()), end='')"
}

cat > /usr/share/nginx/html/env.js <<EOF
window.__env__ = {
  "API_BASE_URL": $(json_encode "${API_BASE_URL:-}")
};
EOF

# Patch CSP placeholders in nginx.conf to include the API origin.
# Both img-src and connect-src need the API base URL when configured.
api_extra=""
if [ -n "${API_BASE_URL:-}" ]; then
  api_extra="${API_BASE_URL}"
fi
sed -i "s|IMG_SRC_PLACEHOLDER|${api_extra}|" /etc/nginx/conf.d/default.conf
# 'self' and Google Fonts origins are already in nginx.conf; append API URL only if set.
sed -i "s|CONNECT_SRC_PLACEHOLDER|${api_extra}|" /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
