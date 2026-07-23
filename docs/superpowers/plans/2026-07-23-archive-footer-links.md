# Previous-Season Footer Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render small gray links to archived seasons (e.g. `2025 | 2026`) below the calendar div on every generated page, driven by an explicit `archives` list in each program's config.

**Architecture:** A new optional `archives` array in `programs/<name>/config.json` flows through `ProgramConfig` → `TemplateData` → a guarded block in `templates/schedule.html`. The archive tool (`cmd/archive`) strips that block when snapshotting, tolerating its absence, before its root-link rewrite pass.

**Tech Stack:** Go (html/template, embed, testing/fstest), plain CSS.

**Spec:** `docs/superpowers/specs/2026-07-23-archive-footer-links-design.md`

## Global Constraints

- Test with `go run generate.go` (defaults to `-program lightning` → `dist/lightning`); use `-program warriors` → `dist/warriors`. Do not generate additional binaries or output directories. (CLAUDE.md)
- Archive entries must be 4-digit years; validation fails generation with a clear error.
- Footer hrefs are root-relative (`/2025/`) so they work from team subdirectory pages.
- The `.seasons` div must contain no nested divs (the archive transform finds the first `</div>` after the opening tag).
- `dist/` is gitignored — never commit generated output.
- Existing `dist/lightning/2025` snapshot stays untouched.

---

### Task 1: `archives` config field with validation

**Files:**
- Modify: `generate.go:25-35` (ProgramConfig struct), `generate.go:57-81` (loadProgramConfig)
- Modify: `programs/lightning/config.json`
- Test: `generate_test.go`

**Interfaces:**
- Produces: `ProgramConfig.Archives []string` (json tag `archives`), validated so every entry matches `^\d{4}$`. Omitted field → nil slice. Task 2 reads `cfg.Archives`.

- [ ] **Step 1: Write the failing tests**

Add to `generate_test.go` (after `TestLoadProgramConfigInvalidSport`):

```go
func TestLoadProgramConfigArchives(t *testing.T) {
	c, err := loadProgramConfig(testFS(`{"name": "Test", "sport": "basketball", "domain": "example.com", "sheetID": "abc", "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"}, "archives": ["2025", "2026"]}`), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Archives) != 2 || c.Archives[0] != "2025" || c.Archives[1] != "2026" {
		t.Errorf("Archives = %v, want [2025 2026]", c.Archives)
	}
}

func TestLoadProgramConfigArchivesOmitted(t *testing.T) {
	c, err := loadProgramConfig(testFS(validConfigJSON), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Archives) != 0 {
		t.Errorf("Archives = %v, want empty", c.Archives)
	}
}

func TestLoadProgramConfigInvalidArchiveYear(t *testing.T) {
	_, err := loadProgramConfig(testFS(`{"name": "Test", "sport": "basketball", "domain": "example.com", "sheetID": "abc", "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"}, "archives": ["25"]}`), "test")
	if err == nil {
		t.Fatal("expected error for non-4-digit archive year")
	}
	if !strings.Contains(err.Error(), "archives") {
		t.Errorf("error should mention archives, got: %v", err)
	}
}
```

Also extend `TestLoadEmbeddedLightningConfig` (generate_test.go:108) with:

```go
	if len(c.Archives) != 1 || c.Archives[0] != "2025" {
		t.Errorf("Archives = %v, want [2025]", c.Archives)
	}
```

And `TestLoadEmbeddedWarriorsConfig` (generate_test.go:121) with:

```go
	if len(c.Archives) != 0 {
		t.Errorf("Archives = %v, want empty", c.Archives)
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestLoad' ./...`
Expected: compile error `c.Archives undefined` (the struct has no such field yet).

- [ ] **Step 3: Implement**

In `generate.go`, add to `ProgramConfig` (after `ICalCalName`, line 34):

```go
	Archives    []string          `json:"archives"`
```

Add a package-level regex near the struct (above `loadProgramConfig`):

```go
var archiveYearRegex = regexp.MustCompile(`^\d{4}$`)
```

In `loadProgramConfig`, after the sport check (line 79), add:

```go
	for _, y := range c.Archives {
		if !archiveYearRegex.MatchString(y) {
			return nil, fmt.Errorf("programs/%s/config.json: archives entries must be 4-digit years, got %q", name, y)
		}
	}
```

In `programs/lightning/config.json`, add after `"icalCalName": "Lightning Schedule"` (keep valid JSON — add a comma to the preceding line):

```json
  "archives": ["2025"]
```

Do NOT touch `programs/warriors/config.json`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run 'TestLoad' ./...`
Expected: PASS (all TestLoad* tests, including the embedded-config ones).

- [ ] **Step 5: Commit**

```bash
git add generate.go generate_test.go programs/lightning/config.json
git commit -m "Add archives config field listing archived season years"
```

---

### Task 2: Render the seasons footer

**Files:**
- Modify: `generate.go:172-189` (TemplateData struct), `generate.go:1142-1159` (data population in generateHTML)
- Modify: `templates/schedule.html:151` (after the `.calendar` div closes)
- Modify: `templates/schedule.css:148` (after the `.calendar` rules)
- Test: `generate_test.go`

**Interfaces:**
- Consumes: `cfg.Archives []string` from Task 1.
- Produces: rendered `<div class="seasons">` block containing one `<a href="/<year>/"><year></a>` per entry, ` | ` separated; absent entirely when `Archives` is empty. Task 3's transform strips this exact markup.

- [ ] **Step 1: Write the failing tests**

Add to `generate_test.go` (after `TestGenerateHTMLWithGames`). Note `setupGenerateHTMLTest()` (generate_test.go:131) sets a `cfg` without archives — the first test overrides that field:

```go
func TestGenerateHTMLArchiveLinks(t *testing.T) {
	setupGenerateHTMLTest()
	cfg.Archives = []string{"2025", "2026"}
	out := filepath.Join(t.TempDir(), "index.html")
	if err := generateHTML(nil, nil, out, nil); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	for _, want := range []string{
		`<div class="seasons">`,
		`<a href="/2025/">2025</a>`,
		`<a href="/2026/">2026</a>`,
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("expected output to contain %q", want)
		}
	}
}

func TestGenerateHTMLNoArchiveLinks(t *testing.T) {
	setupGenerateHTMLTest()
	out := filepath.Join(t.TempDir(), "index.html")
	if err := generateHTML(nil, nil, out, nil); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if strings.Contains(string(html), `class="seasons"`) {
		t.Error("seasons footer should be absent when no archives are configured")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestGenerateHTML.*Archive' .`
Expected: `TestGenerateHTMLArchiveLinks` FAILS (missing `<div class="seasons">`); `TestGenerateHTMLNoArchiveLinks` passes (nothing renders yet — that's fine, it guards against regressions).

- [ ] **Step 3: Implement**

In `generate.go`, add to `TemplateData` (after `ScheduleJS`, line 188):

```go
	Archives       []string
```

In the `data := TemplateData{...}` literal in `generateHTML` (generate.go:1143), add:

```go
		Archives:       cfg.Archives,
```

In `templates/schedule.html`, insert after the closing `</div>` of the calendar block (line 151) and before the `<script>` tag:

```html
    {{if .Archives}}
    <div class="seasons">
      <p>
        {{range $i, $year := .Archives}}{{if $i}} | {{end}}<a href="/{{$year}}/">{{$year}}</a>{{end}}
      </p>
    </div>
    {{end}}
```

(Keep the whole `range` on one line so no whitespace lands between the links and the ` | ` separators.)

In `templates/schedule.css`, insert after the `.calendar a:hover` rule (line 148):

```css
.seasons {
  margin: 0 auto 40px auto;
  text-align: center;
  font-size: 0.85em;
}
.seasons,
.seasons a {
  color: #888;
}
.seasons a:hover {
  text-decoration: none;
}
```

(`#888` reads as gray on both the light background and the dark-mode background, so no `@media (prefers-color-scheme: dark)` override is needed.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run 'TestGenerateHTML' .`
Expected: PASS (all TestGenerateHTML* tests).

- [ ] **Step 5: Commit**

```bash
git add generate.go generate_test.go templates/schedule.html templates/schedule.css
git commit -m "Render previous-season links below the calendar"
```

---

### Task 3: Strip the seasons footer when archiving

**Files:**
- Modify: `cmd/archive/transform.go`
- Test: `cmd/archive/transform_test.go`

**Interfaces:**
- Consumes: the `<div class="seasons">...</div>` markup produced in Task 2 (no nested divs inside).
- Produces: `transformHTML` output never contains the seasons div; pages without one still transform. Removal happens before the `href="/` rewrite (transform.go:45) so year links can't be corrupted into `/<name>/<year>/`.

- [ ] **Step 1: Write the failing tests**

In `cmd/archive/transform_test.go`, add a seasons block to `fixtureHTML` after the calendar div (between transform_test.go lines 29 and 30) so the fixture keeps mirroring generator output:

```go
    <div class="seasons">
      <p><a href="/2024/">2024</a> | <a href="/2025/">2025</a></p>
    </div>
```

In `TestTransformHTML`, extend the `gone` list (transform_test.go:56-60) with:

```go
		`<div class="seasons">`,
		`href="/2024/"`,
```

Add a new test after `TestTransformHTML`:

```go
func TestTransformHTMLNoSeasons(t *testing.T) {
	noSeasons := strings.Replace(fixtureHTML, `<div class="seasons">
      <p><a href="/2024/">2024</a> | <a href="/2025/">2025</a></p>
    </div>`, "", 1)
	if noSeasons == fixtureHTML {
		t.Fatal("fixture surgery failed: seasons block not found")
	}
	out, err := transformHTML(noSeasons, "2025")
	if err != nil {
		t.Fatalf("expected page without seasons footer to transform, got: %v", err)
	}
	if strings.Contains(out, `class="seasons"`) {
		t.Error("no seasons div expected in output")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/archive/`
Expected: `TestTransformHTML` FAILS with `expected "<div class=\"seasons\">" to be removed` (the div survives; note the root-link pass rewrites its hrefs to `href="/2025/2024/"`, so the `href="/2024/"` gone-check alone would not catch survival — the div check is the reliable one). `TestTransformHTMLNoSeasons` passes.

- [ ] **Step 3: Implement**

In `cmd/archive/transform.go`, add to the `const` block (transform.go:8-14):

```go
	seasonsStart    = `<div class="seasons">`
```

In `transformHTML`, insert after the calendar removal (line 30) and renumber the following comments (2→3, 3→4, 4→5, 5→6, 6→7):

```go
	// 2. Remove the seasons footer if present. Unlike the other markers it
	// may legitimately be absent (a program's first archive has no prior
	// seasons). Must run before the root-link rewrite below so its
	// year hrefs are never rewritten.
	if start := strings.Index(html, seasonsStart); start != -1 {
		rest := html[start:]
		end := strings.Index(rest, divClose)
		if end == -1 {
			return "", fmt.Errorf("seasons div not closed")
		}
		html = html[:start] + html[start+end+len(divClose):]
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/archive/`
Expected: PASS (all four transform tests plus main_test.go tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/archive/transform.go cmd/archive/transform_test.go
git commit -m "Strip seasons footer when archiving a season"
```

---

### Task 4: End-to-end verification

**Files:**
- No source changes — verification only. Output inspected in `dist/lightning/` and `dist/warriors/` (gitignored, never committed).

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS across `.` and `./cmd/archive/`.

- [ ] **Step 2: Generate the Lightning site**

Run: `go run generate.go`
Expected: completes without error (needs network access to Google Sheets).

- [ ] **Step 3: Inspect Lightning output**

Run: `grep -c 'class="seasons"' dist/lightning/index.html dist/lightning/varsity/index.html && grep -o '<a href="/2025/">2025</a>' dist/lightning/index.html`
Expected: count `1` for both pages, and the `/2025/` anchor printed — footer present on index AND team pages.

Run: `grep -c 'class="seasons"' dist/lightning/2025/index.html; true`
Expected: `0` — the existing archived snapshot is untouched.

- [ ] **Step 4: Generate and inspect the Warriors site**

Run: `go run generate.go -program warriors && grep -c 'class="seasons"' dist/warriors/index.html; true`
Expected: generation succeeds; grep count is `0` (Warriors has no archives, no footer).

- [ ] **Step 5: Visual check**

Open `dist/lightning/index.html` in a browser (or serve `dist/lightning/`): the `2025` link renders small, gray, centered below the calendar block, in both light and dark mode.
