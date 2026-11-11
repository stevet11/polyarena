#!/usr/bin/env bash
# Build the client bundle and start the Go server.
set -euo pipefail
cd "$(dirname "$0")"
(cd client && [ -d node_modules ] || npm install)
(cd client && npm run build)
cd server && exec go run . "$@"
