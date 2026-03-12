# Agent Usage Guide

This CLI is designed so an agent can turn user intent into explicit commands and JSON payloads without relying on interactive prompts.

## Core Rules

- Prefer `--format json`.
- Use absolute dates like `2026-03-16`, not relative values like `tomorrow`.
- Use stdout for result parsing and stderr for diagnostics only.
- For complex writes, prefer `--file -` or a temp file.
- Payloads for `events`, `workouts`, and `wellness` are raw Intervals-compatible JSON. This CLI does not define a custom schema on top.
- Field names follow the upstream API, which often uses camelCase like `restingHR` and `sleepSecs`.

## Choose The Right Write Command

- Use `intervals events create` to schedule something on a specific date in the calendar.
- Use `intervals events upsert` when you want idempotent calendar writes based on a stable `uid`.
- Use `intervals workouts create` to create a workout/library object.
- Use `intervals wellness put` to write a single day of wellness data.
- Use `intervals wellness bulk-put` to write multiple wellness rows at once.

Practical rule:
- "Create a workout for next Monday" usually means `events create`.
- "Add this workout to my workout library" means `workouts create`.

## Preferred Write Patterns

### Pattern 1: Temp file

Use this when the agent is generating JSON in multiple steps or wants to inspect it before sending.

```bash
tmp="$(mktemp)"
cat > "$tmp" <<'JSON'
{
  "category": "WORKOUT",
  "start_date_local": "2026-03-16T00:00:00",
  "type": "Run",
  "name": "10km @ 5:00/km",
  "moving_time": 3000,
  "description": "- 10km 5:00/km Pace"
}
JSON

intervals events create --file "$tmp" --format json
rm -f "$tmp"
```

### Pattern 2: Stdin

Use this when the payload is generated in one step and does not need to persist.

```bash
cat payload.json | intervals events create --file - --format json
```

Or:

```bash
printf '%s\n' '{"category":"WORKOUT","start_date_local":"2026-03-16T00:00:00","type":"Run","name":"10km @ 5:00/km","moving_time":3000,"description":"- 10km 5:00/km Pace"}' | intervals events create --file - --format json
```

## Event Creation

`events create` writes a calendar event. For planned workouts, this is usually the correct command.

Example payloads:
- [`examples/events/create-10km-run.json`](../examples/events/create-10km-run.json)
- [`examples/events/create-interval-run.json`](../examples/events/create-interval-run.json)

```bash
tmp="$(mktemp)"
cp examples/events/create-10km-run.json "$tmp"
intervals events create --file "$tmp" --format json
rm -f "$tmp"
```

Common fields:
- `category`: often `WORKOUT`
- `start_date_local`: local start datetime
- `type`: sport type like `Run`
- `name`: short event title
- `moving_time`: planned duration in seconds
- `description`: free text

To confirm the event exists:

```bash
intervals events list --oldest 2026-03-16 --newest 2026-03-16 --format json
```

To delete it:

```bash
intervals event delete EVENT_ID --format json
```

## Event Upsert

`events upsert` is safer for agents that may retry or rerun the same instruction. It requires a `uid`.

Example payloads:
- [`examples/events/upsert-10km-run.json`](../examples/events/upsert-10km-run.json)
- [`examples/events/upsert-interval-run.json`](../examples/events/upsert-interval-run.json)

```bash
tmp="$(mktemp)"
cp examples/events/upsert-10km-run.json "$tmp"
intervals events upsert --file "$tmp" --format json
rm -f "$tmp"
```

Use `upsert` when:
- the agent may rerun the same task
- the event should be updated instead of duplicated
- the calling system already has a stable identifier

## Workout Library Creation

`workouts create` writes a workout object, not a scheduled calendar event.

Example payloads:
- [`examples/workouts/create-simple-run.json`](../examples/workouts/create-simple-run.json)
- [`examples/workouts/create-interval-run.json`](../examples/workouts/create-interval-run.json)

```bash
tmp="$(mktemp)"
cp examples/workouts/create-simple-run.json "$tmp"
intervals workouts create --file "$tmp" --format json
rm -f "$tmp"
```

Use this when:
- you want a reusable workout/library item
- the user asked to create a workout template, not schedule it on a day
- the payload is already Intervals-compatible

Important:
- v1 does not define a normalized workout-step schema.
- For richer structured workouts, prefer upstream-native content such as `workout_doc` or other Intervals-compatible workout JSON you already have.
- The example files in this repo show two safe v1 patterns:
  - a minimal library object
  - a more complex interval session represented in `description`

## Wellness Writes

Single day:

Example payload: [`examples/wellness/put-day.json`](../examples/wellness/put-day.json)

```bash
printf '%s\n' '{"restingHR":48,"weight":78.2,"sleepSecs":27000}' | intervals wellness put --date 2026-03-12 --file - --format json
```

Bulk:

```bash
tmp="$(mktemp)"
cat > "$tmp" <<'JSON'
[
  {
    "id": "2026-03-11",
    "restingHR": 49,
    "weight": 78.4
  },
  {
    "id": "2026-03-12",
    "restingHR": 48,
    "weight": 78.2,
    "sleepSecs": 27000
  }
]
JSON
intervals wellness bulk-put --file "$tmp" --format json
rm -f "$tmp"
```

Important:
- `wellness put` takes the date from `--date`.
- The payload must be a JSON object.
- For `bulk-put`, the payload must be a JSON array.

## Reading Back Results

After a write, prefer a read to verify what happened.

Examples:

```bash
intervals whoami --format json
intervals workouts list --format json
intervals wellness get --date 2026-03-12 --format json
intervals events list --oldest 2026-03-16 --newest 2026-03-16 --format json
```

## How Agents Should Think About Search vs List

- Use `activities search` for text and tag queries such as `tempo` or `#threshold`.
- Use `activities list` when the user asks for semantic filters that are not guaranteed to appear in names, such as "long runs on weekends" or "hard sessions from the current block".
- For semantic queries, fetch a bounded window with `activities list` and filter locally on structured fields like `type`, `start_date_local`, `distance`, `moving_time`, and `tags`.

## Good Defaults For Agents

- Resolve natural-language dates before calling the CLI.
- Prefer exact JSON payloads over ad hoc flag parsing for writes.
- Ask follow-up questions only when the intent is genuinely ambiguous.
- Prefer `events upsert` over `events create` when duplicate creation would be harmful.
- Store temporary JSON in a temp file when the payload needs inspection or reuse.
