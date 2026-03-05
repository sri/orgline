default:
    @just --list

dev:
    @go run ./cmd/dev

dev-loadtest:
    @ORGLINE_DB_PATH=./Shakespeare.db go run ./cmd/dev

loadtest: dev-loadtest

go-dev-loadtest: dev-loadtest

prod:
    @mkdir -p ./bin
    @go build -trimpath -ldflags="-s -w" -o ./bin/orgline ./cmd/web

deploy: prod
