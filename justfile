default:
    @just --list

dev:
    @go run ./cmd/web

prod:
    @mkdir -p ./bin
    @go build -trimpath -ldflags="-s -w" -o ./bin/orgline ./cmd/web

deploy: prod
