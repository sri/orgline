---
name: waterware
description: Engineering guardrails for building this project with Go, TypeScript, and SQLite 3.
---

# Waterware Skill

Use this skill when implementing or reviewing code in this repository.

## Tech Stack

- Go
- TypeScript
- SQLite 3

## Global Standards

- Optimize for clarity, maintainability, and predictable behavior.
- Keep dependencies minimal and justified.
- Write tests for critical behavior and bug fixes.
- Prefer secure defaults: input validation, least privilege, and parameterized queries.

## Go Standards

- Use the standard library by default. Add third-party packages only when the required capability is not available in the standard library.
- Keep project layout idiomatic (`cmd/` for entrypoints, `internal/` for private app code).
- Pass `context.Context` through request and DB boundaries.
- Handle errors explicitly; return wrapped errors with actionable context.
- Keep handlers thin; move business logic into internal packages.
- Use table-driven tests where useful and keep tests deterministic.

## TypeScript Standards

- Use `strict` TypeScript settings.
- Prefer explicit types at module boundaries (API I/O, storage I/O, public functions).
- Avoid `any`; use `unknown` plus narrowing when needed.
- Validate untrusted runtime data before use.
- Keep modules small and focused; avoid large, stateful utility files.
- Use clear async error handling and avoid unhandled promise rejections.

## SQLite 3 Standards

- Use parameterized queries only; never build SQL via string interpolation.
- Enable and rely on foreign keys where applicable (`PRAGMA foreign_keys = ON`).
- Use transactions for multi-step writes to preserve consistency.
- Add indexes to support real query patterns, then verify with `EXPLAIN QUERY PLAN`.
- Prefer WAL mode for concurrent read/write workloads when appropriate.
- Keep schema migrations versioned and reversible when possible.
- N+1 queries are not automatically a problem in SQLite (official guidance), because there is no client/server round-trip cost. Still profile real workloads and optimize hot paths.

## Documentation References

- Go: https://go.dev/doc/
- Effective Go: https://go.dev/doc/effective_go
- TypeScript: https://www.typescriptlang.org/docs/
- SQLite docs: https://www.sqlite.org/docs.html
- SQLite and N+1 queries: https://www.sqlite.org/np1queryprob.html
