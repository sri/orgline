# orgline

An experiment with the best of argmode and web using Go and some JavaScript.

## Runtime Configuration

The binary supports flags (with matching environment variable defaults):

- `-port` (`ORGLINE_PORT`) HTTP port when `-addr` is not set. Default: `8080`
- `-addr` (`ORGLINE_ADDR`) Full HTTP listen address, for example `:8080` or `127.0.0.1:8080`
- `-db-path` (`ORGLINE_DB_PATH`) SQLite database path. Default: `orgline.db`
- `-read-header-timeout` (`ORGLINE_READ_HEADER_TIMEOUT`) Default: `5s`
- `-read-timeout` (`ORGLINE_READ_TIMEOUT`) Default: `15s`
- `-write-timeout` (`ORGLINE_WRITE_TIMEOUT`) Default: `15s`
- `-idle-timeout` (`ORGLINE_IDLE_TIMEOUT`) Default: `60s`

Examples:

```bash
./bin/orgline -port 9090 -db-path /var/lib/orgline/orgline.db
./bin/orgline -addr 127.0.0.1:8081 -db-path ./orgline.db
ORGLINE_PORT=8082 ORGLINE_DB_PATH=/data/orgline.db ./bin/orgline
```

## Development

- `just dev` runs a local dev runner that watches source files, restarts `cmd/web` on changes, and triggers browser auto-reload.
- `just prod` builds a single deployable binary at `./bin/orgline`.
