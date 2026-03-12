# intervals

`intervals` is an agent-first CLI for the public [Intervals.icu API](https://intervals.icu/api-docs.html).

V1 goals:
- generated transport/types from the published OpenAPI spec
- hand-written command UX and output shaping
- JSON-first automation contract with human-readable TTY output
- non-interactive auth via `INTERVALS_ACCESS_TOKEN` or `INTERVALS_API_KEY`

## Status

This repo targets a narrow v1 surface:
- `auth status`
- `whoami`
- athlete info and training plan
- activities list/search/upload and activity detail helpers
- events list/get/create/upsert/delete
- workouts list/get/create/download
- wellness list/get/put/bulk-put

## Build

Requirements:
- Go 1.26+

Generate the client and build:

```bash
go generate ./...
go build ./cmd/intervals
```

## Auth

Bearer token takes precedence:

```bash
export INTERVALS_ACCESS_TOKEN=...
```

API key fallback:

```bash
export INTERVALS_API_KEY=...
```

## Output

- TTY stdout defaults to `table`
- non-TTY stdout defaults to `json`
- override with `--format json|table|plain`

Examples:

```bash
intervals auth status
intervals whoami --format json
intervals activities list --oldest 2026-03-01 --newest 2026-03-12
intervals activity streams 123456 --types watts,hr
intervals events create --file event.json
intervals workout download 123 --format zwo --output workout.zwo
intervals wellness put --date 2026-03-12 --file wellness.json
```

## Development

The pinned OpenAPI source lives at [api/openapi-spec.json](/Users/jonaswide/dybo/intervals-cli/api/openapi-spec.json).
Generated code is emitted into `internal/api/gen`.

Tests:

```bash
go test ./...
```

Optional smoke tests:

```bash
go test -tags smoke ./...
```

