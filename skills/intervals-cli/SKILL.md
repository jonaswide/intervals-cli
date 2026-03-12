---
name: intervals-cli
description: Use this skill when working with the intervals CLI or Intervals.icu through shell commands, especially for listing activities, creating scheduled workout events, creating workout library items, and writing wellness data. It teaches the command selection rules, the JSON-first write patterns, and how to choose between events and workouts for agent-driven workflows such as OpenClaw.
---

# intervals-cli

Use this skill when the task involves the `intervals` CLI.

## Core Rules

- Prefer `intervals ... --format json`.
- Resolve relative dates before calling the CLI.
- Use `events create` or `events upsert` for scheduled calendar items.
- Use `workouts create` for reusable workout library items.
- Use `wellness put` or `wellness bulk-put` for wellness writes.
- For complex writes, prefer `--file -` or a temp file.
- Treat stdout as result data and stderr as diagnostics.

## Events vs Workouts

- `event`: a scheduled item on a date in the athlete calendar
- `workout`: a reusable library object, not tied to a date

Rule of thumb:
- "Create a workout for next Monday" usually means `events create`.
- "Save this workout for later reuse" means `workouts create`.

## Examples And Detailed Guidance

Read these only as needed:

- Agent guide: [`../../docs/agents.md`](../../docs/agents.md)
- Event examples: [`../../examples/events/create-10km-run.json`](../../examples/events/create-10km-run.json), [`../../examples/events/create-interval-run.json`](../../examples/events/create-interval-run.json)
- Workout examples: [`../../examples/workouts/create-simple-run.json`](../../examples/workouts/create-simple-run.json), [`../../examples/workouts/create-interval-run.json`](../../examples/workouts/create-interval-run.json)
- Wellness example: [`../../examples/wellness/put-day.json`](../../examples/wellness/put-day.json)

## Command Selection

- Use `activities search` for text/tag queries.
- Use `activities list` for semantic filtering that depends on fields like date, type, distance, or moving time.
- Prefer `events upsert` over `events create` when duplicate creation would be harmful.
