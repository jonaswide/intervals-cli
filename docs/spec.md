# intervals v1 spec

This repository implements a self-only Intervals.icu CLI with:
- env-based auth only
- no interactive login or config file
- `athlete_id=0` for athlete-scoped commands
- raw upstream JSON payloads for event/workout/wellness writes
- generated client transport plus hand-written UX

Public commands in v1:
- `auth status`
- `whoami`
- `athlete get|profile|training-plan`
- `activities list|search|upload`
- `activity get|streams|intervals|best-efforts|download`
- `events list|create|upsert`
- `event get|delete`
- `workouts list|create`
- `workout get|download`
- `wellness list|get|put|bulk-put`

Behavioral constraints:
- default output is `table` on TTY and `json` otherwise
- explicit `--format json|table|plain`
- stdout contains results only on success
- stderr contains diagnostics and errors
- retries are read-only and bounded

