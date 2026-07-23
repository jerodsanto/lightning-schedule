# Previous-Season Footer Links

**Date:** 2026-07-23
**Status:** Approved

## Goal

Each generated page (index and team pages) links to archived seasons in a
small footer below the calendar div, e.g. `2025 | 2026 | 2027`. Programs
with no archives (Warriors) render no footer at all.

The archive list is **explicit config**, not auto-discovered: adding a
season is a deliberate one-line config change made as part of the annual
archiving ritual.

## Config

New optional field in `programs/<name>/config.json`:

```json
"archives": ["2025"]
```

- `Archives []string` on `ProgramConfig` (json tag `archives`).
- Lightning lists `"2025"`; Warriors omits the field.
- Validation at load time: every entry must be a 4-digit year (`^\d{4}$`).
  A bad entry fails generation with a clear error, consistent with the
  existing strict config validation.
- Years render in listed order; keep the list ascending when appending.

Note: `config.json` is embedded in the binary (`go:embed`), so adding a
year requires rebuild + redeploy. Acceptable — archiving is an annual,
already-manual ritual.

## Template

In `templates/schedule.html`, immediately after the `.calendar` div:

```html
{{if .Archives}}
<div class="seasons">
  <p><a href="/2025/">2025</a> | <a href="/2026/">2026</a></p>
</div>
{{end}}
```

- Bare year links separated by `|`, no label.
- Root-relative hrefs (`/2025/`) so links work from team pages
  (`/varsity/`, `/10ublack/`, ...) as well as the index.
- `Archives []string` added to `PageData`, populated from the config.
- **No nested divs inside `.seasons`** — the archive transform locates the
  block by finding the opening tag and the first `</div>` after it, same
  as the calendar removal.

## Style

In `templates/schedule.css`: `.seasons` is small, gray text with gray
links (subtle underline or hover-underline), centered like the calendar
block above it.

## Archive transform

`cmd/archive/transform.go` gains a step that removes the `.seasons` div,
using the same find-open/find-first-close approach as the calendar
removal, with two differences:

1. **Tolerant of absence** — no error when the div is missing, because a
   program's first-ever archive is generated from pages that have no
   footer yet.
2. **Runs before the root-link rewrite** (step 4), so `/2025/`-style
   hrefs are removed before that pass would corrupt them into
   `/<name>/2025/`.

Archived snapshots therefore contain no season navigation, matching the
existing "frozen snapshot, calendar UI removed" philosophy.

The existing `dist/lightning/2025` snapshot predates this feature and
stays untouched.

## Tests

- `generate_test.go`: footer renders with correct links when archives are
  configured; footer absent when the field is empty/omitted; config
  validation rejects non-year entries.
- `cmd/archive/transform_test.go`: seasons div stripped when present;
  pages without one still transform cleanly.
- Manual check: `go run generate.go` (lightning shows `2025` link) and
  `go run generate.go -program warriors` (no footer).
