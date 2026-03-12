# intervals

`intervals` is an agent-first CLI for the public [Intervals.icu API](https://intervals.icu/api-docs.html).

V1 goals:
- generated transport/types from the published OpenAPI spec
- hand-written command UX and output shaping
- JSON-first automation contract with human-readable TTY output
- non-interactive auth via `INTERVALS_ACCESS_TOKEN` or `INTERVALS_API_KEY`

## Install

Recommended: install from a GitHub Release with the installer script:

```bash
curl -fsSL https://raw.githubusercontent.com/jonaswide/intervals-cli/master/scripts/install.sh | sh
intervals --version
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/jonaswide/intervals-cli/master/scripts/install.sh | VERSION=v0.1.0 sh
```

Install into a specific directory:

```bash
curl -fsSL https://raw.githubusercontent.com/jonaswide/intervals-cli/master/scripts/install.sh | INSTALL_DIR="$HOME/.local/bin" sh
```

Manual fallback: download a prebuilt archive from [GitHub Releases](https://github.com/jonaswide/intervals-cli/releases), extract `intervals`, put it on `PATH`, and verify with:

```bash
intervals --version
```

If you already have Go installed, you can install from source:

```bash
go install github.com/jonaswide/intervals-cli/cmd/intervals@latest
intervals --version
```

If you are working from a local checkout:

```bash
go generate ./...
go build ./cmd/intervals
./intervals --version
```

## Status

This repo targets a narrow v1 surface:
- `auth status`
- `whoami`
- athlete info and training plan
- activities list/search/upload and activity detail helpers
- events list/get/create/upsert/delete
- workouts list/get/create/download
- wellness list/get/put/bulk-put

## Auth

For personal use, set your Intervals.icu API key from Settings -> Developer Settings:

```bash
export INTERVALS_API_KEY=...
```

Bearer token also works and takes precedence if both are set:

```bash
export INTERVALS_ACCESS_TOKEN=...
```

Verify auth:

```bash
intervals auth status
intervals whoami --format json
```

## Output

- TTY stdout defaults to `table`
- non-TTY stdout defaults to `json`
- override with `--format json|table|plain`

## For Agents

Recommended calling pattern:
- prefer `--format json`
- use absolute dates like `2026-03-16`, not relative values like `tomorrow`
- use `--file -` or a JSON file for complex writes
- treat stdout as result data and stderr as diagnostics

Examples:

```bash
intervals auth status
intervals whoami --format json
intervals activities list --oldest 2026-03-01 --newest 2026-03-12
intervals activities search --query "#threshold" --oldest 2026-03-01 --newest 2026-03-12 --format json
intervals activity streams 123456 --types watts,hr
intervals events create --file event.json
intervals workout download 123 --format zwo --output workout.zwo
intervals wellness put --date 2026-03-12 --file wellness.json
```

## Development

Requirements:
- Go 1.26+

The pinned OpenAPI source lives at [`api/openapi-spec.json`](api/openapi-spec.json).
Generated code is emitted into `internal/api/gen`.

Tests:

```bash
go test ./...
```

Optional smoke tests:

```bash
go test -tags smoke ./...
```

## Releases

Tagging `v*` publishes release archives and `checksums.txt` automatically via GitHub Actions.
