# Empty-Schedule Placeholder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build succeeds with zero games, rendering "Game schedule coming soon!" — while a zero-games build caused by fetch errors still aborts to protect the live site.

**Architecture:** A `fetchErrors` counter in `main` gates the zero-games exit; a `HasGames` template flag renders the placeholder; the schedule tables render only when there are schedule items.

**Tech Stack:** Go 1.24, html/template.

**Spec:** `docs/superpowers/specs/2026-07-20-empty-schedule-placeholder-design.md`

## Global Constraints

- Placeholder copy, exact: `Game schedule coming soon!`
- Lightning's normal output must be unchanged (placeholder absent when games exist).
- Zero games + ≥1 games-fetch error → exit 1 without generating files (live site untouched). Zero games + clean fetches → exit 0 with placeholder pages.
- Verify per CLAUDE.md: `go run generate.go`, inspect `dist/lightning`. No extra binaries/output dirs in the repo; test output goes to `t.TempDir()`.

---

### Task 1: Placeholder rendering + fetch-error gate

**Files:**
- Modify: `generate.go` (`TemplateData` struct; `generateHTML` data literal; `main` zero-games exit)
- Modify: `templates/schedule.html` (placeholder + conditional tables)
- Modify: `templates/schedule.css` (`.coming-soon` rule, light + dark)
- Test: `generate_test.go` (append)

**Interfaces:**
- Consumes: package-level `cfg *ProgramConfig` and `themeCSS string` (set directly by tests), `generateHTML(allGames []Game, allNotes []Note, outputFile string, filterTeam *Team) error`.
- Produces: `TemplateData.HasGames bool`.

- [ ] **Step 1: Write failing tests** — append to `generate_test.go`:

```go
func setupGenerateHTMLTest() {
	cfg = &ProgramConfig{
		Name:       "Test",
		Domain:     "example.com",
		ThemeColor: "#ffffff",
	}
	themeCSS = ""
}

func TestGenerateHTMLNoGames(t *testing.T) {
	setupGenerateHTMLTest()
	out := filepath.Join(t.TempDir(), "index.html")
	if err := generateHTML(nil, nil, out, nil); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(html), "Game schedule coming soon!") {
		t.Error("expected placeholder message in output with no games")
	}
	if strings.Contains(string(html), `class="schedule-body"`) {
		t.Error("expected schedule tables to be omitted with no schedule items")
	}
}

func TestGenerateHTMLWithGames(t *testing.T) {
	setupGenerateHTMLTest()
	games := []Game{{
		Team:     &Team{Name: "10U Black", Slug: "10ublack", CssClass: "black", Order: 1},
		Date:     "Saturday, October 18, 2025",
		Time:     "1:00 PM",
		Opponent: "Rivals",
	}}
	out := filepath.Join(t.TempDir(), "index.html")
	if err := generateHTML(games, nil, out, nil); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if strings.Contains(string(html), "Game schedule coming soon!") {
		t.Error("placeholder message should not appear when games exist")
	}
	if !strings.Contains(string(html), "Rivals") {
		t.Error("expected game opponent in output")
	}
}
```

Add `"os"` and `"path/filepath"` to the test file's imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestGenerateHTML`
Expected: both tests FAIL (placeholder missing; schedule-body present even when empty).

- [ ] **Step 3: Implement**

`generate.go` — add to `TemplateData` (after `TeamRecord`):

```go
	HasGames       bool
```

In `generateHTML`'s `data := TemplateData{...}` literal add:

```go
		HasGames:       len(gamesToDisplay) > 0,
```

`templates/schedule.html` — insert between the closing `</div>` of `.filter-buttons` (line 69) and `<div class="schedule-header">`:

```html
    {{if not .HasGames}}
    <p class="coming-soon">Game schedule coming soon!</p>
    {{end}}
```

Wrap both table divs (`.schedule-header` through the `.schedule-body` closing `</div>`, lines 71-123) in:

```html
    {{if .ScheduleItems}}
    ...existing two divs unchanged...
    {{end}}
```

`templates/schedule.css` — after the `.filter-btn:hover` rule:

```css
.coming-soon {
  text-align: center;
  font-style: italic;
  margin: 60px 0;
}
```

and inside the `@media (prefers-color-scheme: dark)` block (near the `.info` rule):

```css
  .coming-soon {
    color: #f5f5f5;
  }
```

`generate.go` `main` — in the CBL links loop, count scrape failures:

```go
	fetchErrors := 0
	for _, team := range AllTeams {
		for _, link := range team.CBLLinks {
			games, err := scrapeTeamSchedule(team.Name, link, team.CBLName, team.CssClass)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				fetchErrors++
			} else {
				allGames = append(allGames, games...)
			}
		}
	}
```

Count the sheet-games failure the same way:

```go
	sheetGames, err := fetchGoogleSheetGames()
	if err != nil {
		fmt.Printf("Error fetching Google Sheet: %v\n", err)
		fetchErrors++
	} else {
		allGames = append(allGames, sheetGames...)
	}
```

Replace the zero-games exit:

```go
	if len(allGames) == 0 {
		fmt.Println("No games found. Please check the URLs and try again.")
		os.Exit(1)
	}
```

with:

```go
	if len(allGames) == 0 && fetchErrors > 0 {
		fmt.Println("No games found and fetch errors occurred; leaving existing output untouched.")
		os.Exit(1)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./...`
Expected: all tests pass (the 6 existing tests plus these 2).

- [ ] **Step 5: Verify Lightning output unchanged**

```bash
go run generate.go
grep -c "coming soon" dist/lightning/index.html         # expected: 0 (grep exits 1 — that's the pass condition)
grep -c 'class="schedule-body"' dist/lightning/index.html # expected: 1 (bare "schedule-body" also matches the inlined CSS — don't use it)
```

Spot-check `dist/lightning/index.html` renders the normal table. Also `gofmt -l .` and `go vet` clean.

- [ ] **Step 6: Commit**

```bash
git add generate.go generate_test.go templates/schedule.html templates/schedule.css
git commit -m "Build placeholder page when schedule has no games"
```
