# Empty-Schedule Placeholder

**Date:** 2026-07-20
**Status:** Approved

## Goal

A program whose Google Sheet has no games yet (e.g., a freshly scaffolded
Warriors program) should still build and deploy a branded page showing
"Game schedule coming soon!" instead of the generator exiting with an error.

## Background

`generate.go`'s `main` currently exits 1 when zero games are found. That check
doubles as a production safeguard: the binary runs on the server writing
directly into the live web dir, so when Google fetches fail transiently, the
abort leaves the existing site untouched until the next cron run. The fix must
preserve that safeguard.

## Design

### Fetch-error tracking (generate.go `main`)

- Add a `fetchErrors` counter in `main`. Increment it when a
  `scrapeTeamSchedule` call fails and when `fetchGoogleSheetGames` fails
  (both already print their errors; that behavior stays).
- Replace the unconditional zero-games exit with: exit 1 only when
  `len(allGames) == 0 && fetchErrors > 0`, with a message noting fetch
  errors occurred so the existing site is being left alone.
- Zero games with all fetches clean proceeds to generation.
- Unchanged: teams-fetch failure stays fatal; locations/notes fetch failures
  keep degrading to empty slices.

### Template placeholder (schedule.html + generateHTML)

- `TemplateData` gains `HasGames bool`; `generateHTML` sets it from
  `len(gamesToDisplay) > 0` (per-page, so team pages behave identically).
- In `templates/schedule.html`: when `HasGames` is false, render
  `<p class="coming-soon">Game schedule coming soon!</p>` between the filter
  buttons and the schedule tables.
- The two schedule table divs (`.schedule-header`, `.schedule-body`) render
  only when `ScheduleItems` is non-empty — so a notes-only sheet shows the
  placeholder message followed by its notes, and a fully empty sheet shows
  just the message.
- Everything else (title, filter buttons, calendar subscribe links) renders
  as usual.

### Styling (templates/schedule.css)

- One program-agnostic `.coming-soon` rule in the shared stylesheet (not
  theme.css): centered, italic, comfortable vertical margin. Include a
  dark-mode variant in the existing `@media (prefers-color-scheme: dark)`
  block.

### iCal

- No change. An event-less `.ics` file is valid output.

## Error handling

| Condition | Behavior |
|---|---|
| Zero games, no fetch errors | Build placeholder page, exit 0 |
| Zero games, ≥1 games-fetch error | Print message, exit 1 (live site untouched) |
| Some games despite fetch errors | Build normally (today's behavior) |

## Testing

- First unit tests for `generateHTML` (which reads package-level `cfg` and
  `themeCSS`; tests set them directly): with zero games the output file
  contains "Game schedule coming soon!" and no `.schedule-body` table; with
  one game it contains the game and no placeholder. Output written to
  `t.TempDir()`.
- Manual: `go run generate.go` still produces an unchanged Lightning site
  (placeholder absent, tables present).

## Out of scope

- Distinguishing partial fetch failures (some teams scraped, some not) — any
  games at all means a normal build, as today.
- Placeholder styling per program.
