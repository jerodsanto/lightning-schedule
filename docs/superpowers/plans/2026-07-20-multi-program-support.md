# Multi-Program Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve multiple basketball programs (Lightning now, Warriors next) from one codebase, each with its own Google Sheet, domain, branding, and output directory.

**Architecture:** Program-specific values move from hardcoded constants into `programs/<name>/` directories (config.json + theme.css + icon sources), embedded into the single Go binary and selected with a `-program` flag defaulting to `lightning`. CSS theming uses custom properties (`--accent`, `--accent-hover`) so the shared stylesheet stays program-agnostic. `build.sh` and `deploy.sh` loop over programs.

**Tech Stack:** Go 1.24 (stdlib `embed`, `flag`, `encoding/json`, `testing/fstest`), bash, jq (`/usr/bin/jq`), ImageMagick (`magick`).

**Spec:** `docs/superpowers/specs/2026-07-20-multi-program-support-design.md`

## Global Constraints

- Backward compatibility: the deployed server cron runs `~/scripts/lightning-schedule <web-dir>` with no flag — the `-program` flag MUST default to `lightning` and the output dir MUST remain an optional positional arg, so existing invocations keep working.
- Verify output per CLAUDE.md: run `go run generate.go` and inspect the `dist` output. Do NOT create extra binaries or output directories inside the repo — baselines go in the session scratchpad: `/private/tmp/claude-501/-Users-jerod-src-lightning-schedule/c1849b90-1cbb-4668-b88c-70b3fbf8800b/scratchpad`.
- Lightning output must be functionally identical to pre-refactor output (same games, notes, links, colors); only the output directory (`dist/lightning/`) and timestamps may differ. Task 2 additionally moves CSS rules to the end of the inlined stylesheet (same rules, new order).
- Bash scripts must work on macOS bash 3.2 — no `declare -A` associative arrays.
- Lightning brand values (copy verbatim): accent `#fbcb44`, accent hover `#ffd65f`, domain `schedule.omahalightningbasketball.com`, sheet ID `1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ`, gids: notes `436458989`, locations `1311642203`, teams `440511811`, schedule `0`.

---

### Task 1: Program config loading in generate.go

Replace the hardcoded constants with a `ProgramConfig` loaded from an embedded `programs/lightning/config.json`, selected by a `-program` flag.

**Files:**
- Create: `programs/lightning/config.json`
- Create: `generate_test.go`
- Modify: `generate.go` (constants block at lines 22-27; `fetchLocations`, `fetchTeams`, `fetchGoogleSheetGames`, `fetchGoogleSheetNotes` URL references; `pageTitle` in `generateHTML`; `PRODID`/`X-WR-CALNAME`/UIDs in `generateICalendar`; `main` arg parsing)

**Interfaces:**
- Produces: `type ProgramConfig struct { Name, Domain, SheetID string; Gids map[string]string; ThemeColor, ICalProdID, ICalCalName string }`; `loadProgramConfig(fsys fs.FS, name string) (*ProgramConfig, error)`; `(c *ProgramConfig) csvURL(tab string) string`; package-level `var cfg *ProgramConfig`; embedded `var programsFS embed.FS` covering `programs/*/config.json` (Task 2 extends the embed to `theme.css`). Later tasks rely on the CLI: `go run generate.go -program <name> [outputDir]`, default output `dist/<name>`.

- [ ] **Step 1: Capture a pre-refactor baseline (outside the repo)**

```bash
cd /Users/jerod/src/lightning/schedule
SCRATCH=/private/tmp/claude-501/-Users-jerod-src-lightning-schedule/c1849b90-1cbb-4668-b88c-70b3fbf8800b/scratchpad
go run generate.go "$SCRATCH/baseline"
```

Expected: `💪 Generated schedule with N games and M notes` (N, M > 0). Note N and M — later steps must reproduce them.

- [ ] **Step 2: Create `programs/lightning/config.json`**

```json
{
  "name": "Lightning",
  "domain": "schedule.omahalightningbasketball.com",
  "sheetID": "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ",
  "gids": {
    "schedule": "0",
    "notes": "436458989",
    "locations": "1311642203",
    "teams": "440511811"
  },
  "themeColor": "#fbcb44",
  "icalProdID": "-//Omaha Lightning//Basketball Schedule//EN",
  "icalCalName": "Lightning Schedule"
}
```

- [ ] **Step 3: Write failing tests in `generate_test.go`**

```go
package main

import (
	"strings"
	"testing"
	"testing/fstest"
)

const validConfigJSON = `{
  "name": "Test",
  "domain": "example.com",
  "sheetID": "abc123",
  "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"},
  "themeColor": "#ffffff",
  "icalProdID": "-//Test//Schedule//EN",
  "icalCalName": "Test Schedule"
}`

func testFS(config string) fstest.MapFS {
	return fstest.MapFS{
		"programs/test/config.json": &fstest.MapFile{Data: []byte(config)},
	}
}

func TestLoadProgramConfig(t *testing.T) {
	c, err := loadProgramConfig(testFS(validConfigJSON), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "Test" {
		t.Errorf("Name = %q, want %q", c.Name, "Test")
	}
	if c.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", c.Domain, "example.com")
	}
	want := "https://docs.google.com/spreadsheets/d/abc123/export?format=csv&gid=1"
	if got := c.csvURL("notes"); got != want {
		t.Errorf("csvURL(notes) = %q, want %q", got, want)
	}
}

func TestLoadProgramConfigUnknownProgram(t *testing.T) {
	_, err := loadProgramConfig(testFS(validConfigJSON), "nope")
	if err == nil {
		t.Fatal("expected error for unknown program")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error should list available programs, got: %v", err)
	}
}

func TestLoadProgramConfigMissingSheetID(t *testing.T) {
	_, err := loadProgramConfig(testFS(`{"name": "Test", "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"}}`), "test")
	if err == nil {
		t.Fatal("expected error for missing sheetID")
	}
	if !strings.Contains(err.Error(), "sheetID") {
		t.Errorf("error should mention sheetID, got: %v", err)
	}
}

func TestLoadProgramConfigMissingGid(t *testing.T) {
	_, err := loadProgramConfig(testFS(`{"name": "Test", "sheetID": "abc", "gids": {"schedule": "0"}}`), "test")
	if err == nil {
		t.Fatal("expected error for missing gids")
	}
	if !strings.Contains(err.Error(), "gids.notes") {
		t.Errorf("error should mention gids.notes, got: %v", err)
	}
}

func TestLoadEmbeddedLightningConfig(t *testing.T) {
	c, err := loadProgramConfig(programsFS, "lightning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.SheetID != "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ" {
		t.Errorf("wrong sheetID: %q", c.SheetID)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./...`
Expected: compile error — `undefined: loadProgramConfig` (and `programsFS`).

- [ ] **Step 5: Implement config loading in `generate.go`**

Replace the constants block (lines 22-27):

```go
// Constants
const domain = "schedule.omahalightningbasketball.com"
const googleSheetID = "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ"
const googleSheetCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv"
const googleSheetNotesCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv&gid=436458989"
const googleSheetLocationsCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv&gid=1311642203"
const googleSheetTeamsCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv&gid=440511811"
```

with:

```go
// ProgramConfig holds everything specific to one program (Lightning, Warriors, ...)
type ProgramConfig struct {
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	SheetID     string            `json:"sheetID"`
	Gids        map[string]string `json:"gids"`
	ThemeColor  string            `json:"themeColor"`
	ICalProdID  string            `json:"icalProdID"`
	ICalCalName string            `json:"icalCalName"`
}

//go:embed programs/*/config.json
var programsFS embed.FS

var cfg *ProgramConfig

func listPrograms(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, "programs")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func loadProgramConfig(fsys fs.FS, name string) (*ProgramConfig, error) {
	data, err := fs.ReadFile(fsys, "programs/"+name+"/config.json")
	if err != nil {
		return nil, fmt.Errorf("unknown program %q (available: %s)", name, strings.Join(listPrograms(fsys), ", "))
	}
	var c ProgramConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid programs/%s/config.json: %v", name, err)
	}
	if c.Name == "" || c.SheetID == "" {
		return nil, fmt.Errorf("programs/%s/config.json: name and sheetID are required (copy the sheet ID from the Google Sheet URL)", name)
	}
	for _, tab := range []string{"schedule", "notes", "locations", "teams"} {
		if c.Gids[tab] == "" {
			return nil, fmt.Errorf("programs/%s/config.json: gids.%s is required (open that tab in the Google Sheet and copy the gid= value from the URL)", name, tab)
		}
	}
	return &c, nil
}

func (c *ProgramConfig) csvURL(tab string) string {
	return "https://docs.google.com/spreadsheets/d/" + c.SheetID + "/export?format=csv&gid=" + c.Gids[tab]
}
```

Import changes: replace `_ "embed"` with `"embed"`; add `"encoding/json"`, `"flag"`, `"io/fs"` (keep everything else).

- [ ] **Step 6: Replace all constant references**

Each is a one-line change:
- `fetchLocations` (~line 132): `client.Get(googleSheetLocationsCSVURL)` → `client.Get(cfg.csvURL("locations"))`
- `fetchTeams` (~line 178): `client.Get(googleSheetTeamsCSVURL)` → `client.Get(cfg.csvURL("teams"))`
- `fetchGoogleSheetGames` (~line 291): `client.Get(googleSheetCSVURL)` → `client.Get(cfg.csvURL("schedule"))`
- `fetchGoogleSheetNotes` (~line 432): `client.Get(googleSheetNotesCSVURL)` → `client.Get(cfg.csvURL("notes"))`
- `generateHTML` (~line 900): `pageTitle := "Lightning"` → `pageTitle := cfg.Name`
- `generateHTML` template data (~line 1048): `ProdDomain: domain,` → `ProdDomain: cfg.Domain,`
- `generateICalendar` (~line 1112): `ical.WriteString("PRODID:-//Omaha Lightning//Basketball Schedule//EN\r\n")` → `ical.WriteString("PRODID:" + cfg.ICalProdID + "\r\n")`
- `generateICalendar` (~line 1115): `ical.WriteString("X-WR-CALNAME:Lightning Schedule")` → `ical.WriteString("X-WR-CALNAME:" + cfg.ICalCalName)`
- Game UID (~line 1187): `uid := fmt.Sprintf("game-%s-%s-%s@lightningschedule.local",` → `uid := fmt.Sprintf("game-%s-%s-%s@"+cfg.Domain,`
- Note UID (~line 1258): `uid := fmt.Sprintf("note-%s-%s@lightningschedule.local",` → `uid := fmt.Sprintf("note-%s-%s@"+cfg.Domain,`

- [ ] **Step 7: Add flag parsing to `main`**

At the top of `main()` (before `AllTeams, err = fetchTeams()`), add:

```go
program := flag.String("program", "lightning", "program to generate (directory name under programs/)")
flag.Parse()

var err error
cfg, err = loadProgramConfig(programsFS, *program)
if err != nil {
	fmt.Println(err)
	os.Exit(1)
}
```

Then change `var err error` / `AllTeams, err = fetchTeams()` to not redeclare `err` (drop the existing `var err error` line since it's declared above).

Replace the output-dir parsing (lines 1361-1365):

```go
outputDir := "dist"
if len(os.Args) > 1 {
	outputDir = os.Args[1]
}
```

with:

```go
outputDir := filepath.Join("dist", *program)
if flag.NArg() > 0 {
	outputDir = flag.Arg(0)
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./...`
Expected: `ok` — all 6 tests pass.

- [ ] **Step 9: Generate and diff against baseline**

```bash
cd /Users/jerod/src/lightning/schedule
go run generate.go
SCRATCH=/private/tmp/claude-501/-Users-jerod-src-lightning-schedule/c1849b90-1cbb-4668-b88c-70b3fbf8800b/scratchpad
normalize() { sed -E -e 's/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z//g' -e 's/DTSTAMP:[0-9TZ]+//g' -e 's|[0-9]+/[0-9]+/[0-9]+ at [0-9]+:[0-9]+[AP]M UTC||g' "$1"; }
cd "$SCRATCH/baseline" && find . -type f | while read -r f; do
  diff <(normalize "$f") <(normalize "/Users/jerod/src/lightning/schedule/dist/lightning/$f") > /dev/null || echo "DIFF: $f"
done; cd /Users/jerod/src/lightning/schedule
```

Expected: game/note counts match Step 1, output lands in `dist/lightning/`, and no `DIFF:` lines — except `.ics` files, where UIDs intentionally changed from `@lightningschedule.local` to `@schedule.omahalightningbasketball.com`. Verify .ics diffs show ONLY UID lines: `diff <(normalize "$SCRATCH/baseline/schedule.ics") <(normalize dist/lightning/schedule.ics)`. (Live sheet edits between runs can also cause diffs — if a diff looks like data, re-capture the baseline and re-run.)

Also verify the error path: `go run generate.go -program nope` → prints `unknown program "nope" (available: lightning)` and exits 1.

- [ ] **Step 10: Commit**

```bash
git add generate.go generate_test.go programs/lightning/config.json
git commit -m "Extract program config from constants; add -program flag"
```

---

### Task 2: Per-program CSS theming

Move the accent color and team badge color classes out of the shared stylesheet into `programs/lightning/theme.css`, appended at generate time.

**Files:**
- Create: `programs/lightning/theme.css`
- Modify: `templates/schedule.css` (replace `#fbcb44`/`#ffd65f` with CSS vars; delete team badge color classes from both light and dark sections)
- Modify: `templates/schedule.html:9` (theme-color meta tag → template variable)
- Modify: `generate.go` (embed directive, load theme, concatenate into `StylesCSS`, add `ThemeColor` to `TemplateData`)

**Interfaces:**
- Consumes: `programsFS`, `cfg`, `-program` flag from Task 1.
- Produces: package-level `var themeCSS string` loaded in `main`; embed directive becomes `//go:embed programs/*/config.json programs/*/theme.css`; every `programs/<name>/` directory MUST contain `theme.css` (missing file is a fatal startup error). Inlined page CSS = `schedule.css` + `"\n"` + `theme.css`. `TemplateData` gains `ThemeColor string`.

- [ ] **Step 1: Create `programs/lightning/theme.css`**

Content — the accent variables plus the team badge classes moved verbatim from `templates/schedule.css` (light-mode classes from lines 123-152, dark-mode from lines 307-343):

```css
:root {
  --accent: #fbcb44;
  --accent-hover: #ffd65f;
}

/* Team-specific styles */
.team-badge.varsity {
  background-color: #f59c44;
  color: black;
}
.team-badge.jv {
  background-color: #44a15b;
  color: white;
}
.team-badge.gold {
  background-color: #ffd700;
  color: black;
}
.team-badge.white {
  background-color: #ffffff;
  color: black;
  border: 1px solid black;
}
.team-badge.blue {
  background-color: #5b9de9;
  color: white;
}
.team-badge.red {
  background-color: #d53a44;
  color: white;
}
.team-badge.black {
  background-color: #000000;
  color: white;
}

/* Team badge adjustments for dark mode */
@media (prefers-color-scheme: dark) {
  .team-badge.varsity {
    background-color: #f59c44;
    color: black;
  }
  .team-badge.jv {
    background-color: #44a15b;
    color: white;
  }
  .team-badge.gold {
    background-color: #ffd700;
    color: black;
  }
  .team-badge.white {
    background-color: #e8e8e8;
    color: black;
    border: 1px solid #666;
  }
  .team-badge.blue {
    background-color: #5b9de9;
    color: white;
  }
  .team-badge.red {
    background-color: #d53a44;
    color: white;
  }
  .team-badge.black {
    background-color: #333;
    color: white;
    border: 1px solid #555;
  }
}
```

- [ ] **Step 2: De-brand `templates/schedule.css`**

- Replace every `#fbcb44` with `var(--accent)` (8 occurrences: `.filter-btn.active` light+dark, `th` light+dark, `tr.week-start td` light+dark, `td a:hover` dark, `tr.note-row a` dark, `.calendar a` dark).
- Replace every `#ffd65f` with `var(--accent-hover)` (2 occurrences: `tr.note-row a:hover` dark, `.calendar a:hover` dark).
- Delete the light-mode team badge block (lines 123-152, the `/* Team-specific styles */` comment through `.team-badge.black { ... }`) — keep the base `.team-badge` rule at lines 113-121.
- Delete the dark-mode team badge block (lines 307-343, the `/* Team badge adjustments for dark mode */` comment through the final `.team-badge.black { ... }`).

- [ ] **Step 3: Load and concatenate the theme in `generate.go`**

Change the embed directive from `//go:embed programs/*/config.json` to:

```go
//go:embed programs/*/config.json programs/*/theme.css
var programsFS embed.FS
```

Add below `var cfg *ProgramConfig`:

```go
var themeCSS string
```

In `main`, right after the `loadProgramConfig` error check, add:

```go
theme, err := programsFS.ReadFile("programs/" + *program + "/theme.css")
if err != nil {
	fmt.Printf("programs/%s/theme.css is required: %v\n", *program, err)
	os.Exit(1)
}
themeCSS = string(theme)
```

In `generateHTML`, change `StylesCSS: template.CSS(stylesCSS),` to:

```go
StylesCSS: template.CSS(stylesCSS + "\n" + themeCSS),
```

- [ ] **Step 4: Template the theme-color meta tag**

`templates/schedule.html:9` hardcodes the accent:

```html
    <meta name="theme-color" content="#fbcb44" />
```

Change it to:

```html
    <meta name="theme-color" content="{{.ThemeColor}}" />
```

Add `ThemeColor string` to the `TemplateData` struct (after `PagePath`), and in `generateHTML` add `ThemeColor: cfg.ThemeColor,` to the `data := TemplateData{...}` literal.

- [ ] **Step 5: Verify**

Run: `go test ./...` — expected: all tests still pass (embed pattern change compiles).

```bash
go run generate.go
grep -c -- "--accent: #fbcb44" dist/lightning/index.html       # expected: 1
grep -c "team-badge.varsity" dist/lightning/index.html         # expected: 2 (light + dark)
grep -c "#fbcb44" dist/lightning/index.html                    # expected: 2 (theme-color meta + :root var)
grep -c "var(--accent)" dist/lightning/index.html              # expected: 8
```

Open `dist/lightning/index.html` in a browser and confirm: gold header bar, gold active filter button, colored team badges (black/red/blue/gold/white/jv/varsity), gold week-start separators. Check dark mode too (browser dev tools → emulate `prefers-color-scheme: dark`).

- [ ] **Step 6: Commit**

```bash
git add templates/schedule.css templates/schedule.html programs/lightning/theme.css generate.go
git commit -m "Move accent color and team badge styles to per-program theme.css"
```

---

### Task 3: Per-program icons, manifest, and build.sh

Move icon sources into `programs/<name>/`, generate sized assets and manifest.json into `static/<name>/`, and make `build.sh` loop over programs.

**Files:**
- Move (git mv): `templates/icon.png` → `programs/lightning/icon.png`; `templates/favicon.png` → `programs/lightning/favicon.png`; all 8 files `static/*.png`, `static/favicon.ico` → `static/lightning/`; delete `static/manifest.json` (regenerated below)
- Modify: `build.sh` (full rewrite)

**Interfaces:**
- Consumes: `go run generate.go -program <name>` from Task 1; `programs/<name>/config.json` fields `name`, `themeColor`.
- Produces: `static/<name>/` containing `apple-touch-icon*.png`, `android-chrome-*.png`, `favicon-*.png`, `favicon.ico`, `manifest.json` — committed to git. `dist/<name>/` gets symlinks `../../static/<name>/<file>`. Task 4's deploy.sh uploads `static/<name>/*`.

- [ ] **Step 1: Move brand assets**

```bash
cd /Users/jerod/src/lightning/schedule
git mv templates/icon.png programs/lightning/icon.png
git mv templates/favicon.png programs/lightning/favicon.png
mkdir -p static/lightning
git mv static/*.png static/favicon.ico static/lightning/
git rm static/manifest.json
rm -rf dist
```

(`dist` is gitignored and regenerable; removing it clears stale root-level symlinks.)

- [ ] **Step 2: Rewrite `build.sh`**

```bash
#!/bin/bash

# Build per-program assets (icons, manifest) into static/<program>/,
# symlink them into dist/<program>/, and generate schedules.

set -e

for progdir in programs/*/; do
  prog=$(basename "$progdir")
  SRC="programs/${prog}"
  DST="static/${prog}"
  mkdir -p "$DST"

  # Generate icon sizes from source images (skip if sources are missing or unchanged)
  if [ -f "${SRC}/icon.png" ]; then
    if [ "${SRC}/icon.png" -nt "${DST}/apple-touch-icon.png" ]; then
      magick "${SRC}/icon.png" -resize 120x120 "${DST}/apple-touch-icon-120x120.png"
      magick "${SRC}/icon.png" -resize 152x152 "${DST}/apple-touch-icon-152x152.png"
      magick "${SRC}/icon.png" -resize 180x180 "${DST}/apple-touch-icon.png"
      magick "${SRC}/icon.png" -resize 192x192 "${DST}/android-chrome-192x192.png"
      magick "${SRC}/icon.png" -resize 512x512 "${DST}/android-chrome-512x512.png"
    fi
  else
    echo "⚠️  ${SRC}/icon.png missing — skipping icon generation for ${prog}"
  fi

  if [ -f "${SRC}/favicon.png" ]; then
    if [ "${SRC}/favicon.png" -nt "${DST}/favicon-16x16.png" ]; then
      magick "${SRC}/favicon.png" -resize 16x16 "${DST}/favicon-16x16.png"
      magick "${SRC}/favicon.png" -resize 32x32 "${DST}/favicon-32x32.png"
      magick "${SRC}/favicon.png" -define icon:auto-resize=64,48,32,16 "${DST}/favicon.ico"
    fi
  else
    echo "⚠️  ${SRC}/favicon.png missing — skipping favicon generation for ${prog}"
  fi

  # Generate manifest.json from the program config
  jq '{
    name: (.name + " Schedule"),
    short_name: .name,
    icons: [
      {src: "/android-chrome-192x192.png", sizes: "192x192", type: "image/png", purpose: "any maskable"},
      {src: "/android-chrome-512x512.png", sizes: "512x512", type: "image/png", purpose: "any maskable"}
    ],
    theme_color: .themeColor,
    background_color: "#f5f5f5",
    display: "standalone",
    start_url: "/"
  }' "${SRC}/config.json" > "${DST}/manifest.json"

  # Symlink static files into dist/<program> (only if they don't exist)
  mkdir -p "dist/${prog}"
  for file in "${DST}"/*; do
    filename=$(basename "$file")
    if [ ! -e "dist/${prog}/${filename}" ]; then
      ln -s "../../static/${prog}/${filename}" "dist/${prog}/${filename}"
    fi
  done

  # Generate the schedule (a program with an unfilled config fails without stopping the others)
  go run generate.go -program "$prog" || echo "⚠️  generate failed for ${prog}"
done
```

- [ ] **Step 3: Run and verify**

```bash
./build.sh
ls static/lightning/           # 8 pngs + favicon.ico + manifest.json
jq -r '.name, .theme_color' static/lightning/manifest.json
ls -la dist/lightning/ | grep -c '\->'
```

Expected: manifest prints `Lightning Schedule` and `#fbcb44`; dist/lightning contains ~10 symlinks pointing at `../../static/lightning/`, plus `index.html`, `schedule.ics`, and team subdirectories. Confirm `git diff --stat` shows the icon pngs as pure renames (no content change — the `-nt` check found them up to date). Open `dist/lightning/index.html` and confirm the favicon loads.

- [ ] **Step 4: Commit**

```bash
git add -A build.sh programs static templates
git commit -m "Generate per-program icons and manifest under static/<program>"
```

---

### Task 4: Warriors scaffold, deploy.sh, and docs

Scaffold `programs/warriors/` (fails fast until the real sheet ID is pasted in), make deploy per-program, and update CLAUDE.md.

**Files:**
- Create: `programs/warriors/config.json`, `programs/warriors/theme.css`
- Modify: `deploy.sh` (full rewrite), `CLAUDE.md`

**Interfaces:**
- Consumes: `-program` CLI, `static/<name>/` layout, per-program validation errors from Task 1.
- Produces: `deploy.sh [program]` — no arg deploys every program listed in its `PROGRAMS` array; `web_dir()` case statement maps program → remote web dir.

- [ ] **Step 1: Create `programs/warriors/config.json`**

Empty sheetID/gids are intentional — generation fails with the instructive error from Task 1 until Jerod pastes real values:

```json
{
  "name": "Warriors",
  "domain": "",
  "sheetID": "",
  "gids": {
    "schedule": "",
    "notes": "",
    "locations": "",
    "teams": ""
  },
  "themeColor": "#c8102e",
  "icalProdID": "-//Warriors//Basketball Schedule//EN",
  "icalCalName": "Warriors Schedule"
}
```

- [ ] **Step 2: Create `programs/warriors/theme.css`**

Starter palette (red accent) — adjust to actual Warriors colors when known. Team badge classes get added as the Warriors Teams tab is filled in:

```css
:root {
  --accent: #c8102e;
  --accent-hover: #e8354f;
}
```

- [ ] **Step 3: Verify the fail-fast path**

Run: `go run generate.go -program warriors`
Expected: exit 1 with `programs/warriors/config.json: name and sheetID are required (copy the sheet ID from the Google Sheet URL)`.

Run: `./build.sh`
Expected: lightning builds fully; warriors prints the sheetID error then `⚠️  generate failed for warriors`; script exits 0.

- [ ] **Step 4: Rewrite `deploy.sh`**

```bash
#!/bin/bash

# Deploy script: uploads the binary once, then per-program static files,
# and runs the generator remotely for each program.
# Usage: ./deploy.sh [program]   (no arg = all programs in PROGRAMS)

set -e

HOST="dh"
BINARY="lightning-schedule"
SCRIPT_DIR="~/scripts"

# Programs to deploy. Add "warriors" once its domain and web dir exist.
PROGRAMS=("lightning")

# macOS ships bash 3.2 (no associative arrays), hence the case statement
web_dir() {
  case "$1" in
    lightning) echo "~/schedule.omahalightningbasketball.com" ;;
    # warriors) echo "~/schedule.WARRIORS-DOMAIN-HERE" ;;
    *) echo "" ;;
  esac
}

if [ -n "$1" ]; then
  PROGRAMS=("$1")
fi

# Compile Linux binary
echo "🔨 Compiling Linux binary..."
GOOS=linux GOARCH=amd64 go build -o ${BINARY}

# Upload binary to remote scripts directory
echo "📤 Uploading binary to ${HOST}:${SCRIPT_DIR}..."
scp -q ${BINARY} ${HOST}:${SCRIPT_DIR}

for prog in "${PROGRAMS[@]}"; do
  WEB_DIR=$(web_dir "$prog")
  if [ -z "$WEB_DIR" ]; then
    echo "❌ No web dir configured for ${prog} (edit web_dir() in deploy.sh)"
    exit 1
  fi

  echo "📁 Uploading static files to ${HOST}:${WEB_DIR}..."
  scp -r -q static/${prog}/* ${HOST}:${WEB_DIR}/

  echo "🚀 Generating ${prog} on ${HOST}..."
  ssh ${HOST} "${SCRIPT_DIR}/${BINARY} -program ${prog} ${WEB_DIR}"
done

# Delete local binary
echo "🗑️  Removing local binary..."
rm ${BINARY}

echo "✅ Deploy complete!"
```

Note: the server cron invoking `~/scripts/lightning-schedule <web-dir>` without `-program` continues to work because the flag defaults to `lightning`.

- [ ] **Step 5: Verify deploy.sh syntax (no actual deploy)**

Run: `bash -n deploy.sh`
Expected: no output (parses clean). Do NOT run an actual deploy in this task — that's Jerod's call.

- [ ] **Step 6: Update `CLAUDE.md`**

Replace the whole file with:

```markdown
## Testing Output

Whenever you want to test results, run:

`go run generate.go`

and test the output in the `dist/lightning` folder (the `-program` flag defaults
to `lightning`; use `-program warriors` → `dist/warriors` for the Warriors
program). Do not generate additional binaries or output directories.

## Adding a Program

Create `programs/<name>/` with `config.json`, `theme.css`, `icon.png`, and
`favicon.png` (copy `programs/lightning/` as a template), run `./build.sh`,
then add the program and its web dir to `deploy.sh`.
```

- [ ] **Step 7: Commit**

```bash
git add programs/warriors deploy.sh CLAUDE.md
git commit -m "Scaffold Warriors program; make deploy.sh per-program"
```

---

## Post-plan follow-ups (Jerod, not the implementer)

- Copy the Lightning Google Sheet as the Warriors sheet; paste its sheet ID and the four tab gids into `programs/warriors/config.json`.
- Fill in the Warriors domain in `config.json` and `web_dir()` in `deploy.sh`, and uncomment `"warriors"` in `PROGRAMS`; create the web dir + cron on host `dh`.
- Drop real `icon.png`/`favicon.png` into `programs/warriors/` and set actual team colors in its `theme.css`.
