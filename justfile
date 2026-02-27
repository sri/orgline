default:
    @just --list

dev:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ ! -d web/node_modules ]; then
        (cd web && npm install --no-audit --no-fund)
    fi
    (cd web && npm run dev -- --host 127.0.0.1 --port 5173) &
    frontend_pid=$!
    trap 'kill "${frontend_pid}"' EXIT INT TERM
    ORGLINE_DEV_FRONTEND_URL=http://127.0.0.1:5173 go run ./cmd/web

prod:
    @if [ ! -d web/node_modules ]; then cd web && npm install --no-audit --no-fund; fi
    @cd web && npm run build
    @mkdir -p ./bin
    @go build -trimpath -ldflags="-s -w" -o ./bin/orgline ./cmd/web

deploy: prod
