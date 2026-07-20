# Season Archive Tool

**Date:** 2026-07-20
**Status:** Approved

## Goal

A command that snapshots a generated season site into a self-contained,
correctly-linked subdirectory archive — e.g. `dist/lightning/2025/` — so a
finished season can live at `schedule.omahalightningbasketball.com/2025/`
while the root serves the current season.

## CLI

```
go run ./cmd/archive <source-dir> <archive-name>
# e.g. go run ./cmd/archive dist/lightning 2025  →  dist/lightning/2025/
```

Publishing is a manual one-time `scp -r` of the archive directory to the
server web dir. The remote generator cron and `deploy.sh` never write to or
delete unknown subdirectories, so the archive persists on the server.

### Validation (fail fast, clear messages)

- `<archive-name>` must match `^[A-Za-z0-9-]+$` (it becomes a URL path
  segment).
- `<source-dir>/index.html` must exist (guards against archiving the wrong
  directory).
- `<source-dir>/<archive-name>/` must NOT already exist (protects earlier
  archives from being clobbered).

## Copy pass

Walk `<source-dir>` recursively:

- Skip the destination directory itself (it is nested inside the source).
- Skip files named `schedule.ics` (calendar feeds are useless in an archive
  and nothing links to them once the calendar div is removed).
- Copy all other files, dereferencing symlinks (the `dist/` asset symlinks
  become real files — the archive is fully self-contained).
- Files ending in `.html` and files named `manifest.json` are transformed
  in-flight (below); all other files are copied byte-for-byte.

## HTML transformations

Targeted string surgery against the known generated markup, applied in this
order to every `.html` file:

1. **Remove the calendar block**: from `<div class="calendar">` through its
   closing `</div>` (the generated block contains no nested divs).
2. **Remove the Only Upcoming button**: the
   `<button id="onlyUpcoming" class="filter-btn">Only Upcoming</button>`
   element.
3. **Neutralize the filter JS**: replace the line
   `document.addEventListener("DOMContentLoaded", applyFilters);` with
   `localStorage.removeItem("onlyUpcoming");`. Rationale: with the button
   removed, `applyFilters` would throw on the missing element, and its
   localStorage-driven row hiding would blank an all-past-games archive.
   The replacement also unsets the shared-origin preference on first visit,
   per the requirement.
4. **Rewrite root-absolute links**: every `href="/` becomes
   `href="/<archive-name>/` — nav buttons, team badges, the "All Teams"
   link, favicon/apple-touch-icon/android-chrome/manifest links. External
   links (`https://…`, `webcal://…`) are untouched by construction because
   they do not begin with `href="/`.
5. **Title**: prepend the archive name — `<title>Lightning Game Schedule…`
   → `<title>2025 Lightning Game Schedule…` (applies to team pages too).
6. **H1**: append ` (<archive-name>)` immediately before `</h1>`, keeping
   the emoji, page title, and W-L record intact.

## manifest.json transformation

- Rewrite icon `src` values (`"/android-chrome-…"` →
  `"/<archive-name>/android-chrome-…"`) and `start_url` (`"/"` →
  `"/<archive-name>/"`).
- Name fields are left unchanged.

## Deliberately kept in archived pages

- The "as of <timestamp>" line (historically accurate).
- Past-game/past-note graying (all rows — authentic for an archive).
- Dark-mode styles, `theme-color` meta, standalone auto-refresh JS,
  `syncTableHeaders` (tables exist, so it runs normally).

## Out of scope (considered, rejected as YAGNI)

- `noindex` robots meta, manifest name suffixing, automatic upload to the
  server, archiving directly on the server.

## Testing

- Unit tests for the HTML transform function against a fixture built from
  the real generated markup: calendar div gone, button gone, `applyFilters`
  registration replaced with `localStorage.removeItem`, root-absolute links
  rewritten, `https://` links untouched, title prepended, h1 appended.
- Unit test for the manifest transform.
- End-to-end test: build a miniature fake site (index.html, one team
  subdirectory, a symlinked asset, a schedule.ics) in `t.TempDir()`, run the
  archive, assert: structure copied, symlink dereferenced, .ics excluded,
  destination-exists error on second run, bad-name and missing-index errors.
- Manual: `go run ./cmd/archive dist/lightning 2025` and browse
  `dist/lightning/2025/index.html`.
