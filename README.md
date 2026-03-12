# intervals.icu CLI - agent-first training data without breaking stride.

`intervals` is a lightweight CLI for the public [Intervals.icu API](https://intervals.icu/api-docs.html), built for AI agents.

The main use case is letting tools like openclaw inspect training data, create workouts and events, and automate common Intervals workflows through command line and JSON-first output.

Install it (or tell your agent to), set `INTERVALS_API_KEY`, and talk to your agent. 🏃‍♂️🚴‍♂️

## Install

Recommended: install from a GitHub Release with the installer script:

```bash
curl -fsSL https://raw.githubusercontent.com/jonaswide/intervals-cli/master/scripts/install.sh | sh
```

Manual fallback: download a prebuilt archive from [GitHub Releases](https://github.com/jonaswide/intervals-cli/releases), extract `intervals`, put it on `PATH`.

Verify install:

```bash
intervals --version
```

## Status

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
