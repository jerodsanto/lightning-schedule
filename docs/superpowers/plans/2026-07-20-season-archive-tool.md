# Season Archive Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `go run ./cmd/archive <source-dir> <archive-name>` snapshots a generated season site into a self-contained, correctly-linked subdirectory archive (e.g. `dist/lightning/2025/`).

**Architecture:** A new `cmd/archive` package with pure transform functions (`transform.go`) doing strict string surgery on the known generated markup, and a walk/copy orchestrator (`main.go`) that dereferences symlinks, excludes `.ics` files and the destination, and transforms `.html`/`manifest.json` in-flight.

**Tech Stack:** Go 1.24 stdlib only (`strings`, `regexp`, `os`, `io/fs`, `path/filepath`).

**Spec:** `docs/superpowers/specs/2026-07-20-season-archive-tool-design.md`

## Global Constraints

- Transforms are STRICT: if an expected marker (calendar div, button, JS registration, `<title>`, `</h1>`, manifest `start_url`) is missing, return an error — never silently produce a partial archive.
- Archive name must match `^[A-Za-z0-9-]+$`.
- Refuse to run when `<source-dir>/index.html` is missing or `<source-dir>/<archive-name>/` already exists.
- Exclude every file named `schedule.ics`; dereference symlinks when copying.
- Test output goes to `t.TempDir()` only. No extra binaries (`go run`/`go test` only). Manual verification may create `dist/lightning/2025/` but must remove it afterward.

---

### Task 1: Transform functions

**Files:**
- Create: `cmd/archive/transform.go`
- Test: `cmd/archive/transform_test.go`

**Interfaces:**
- Produces: `transformHTML(html, name string) (string, error)` and `transformManifest(manifest, name string) (string, error)` in `package main` under `cmd/archive/`. Also the test fixture consts `fixtureHTML` and `fixtureManifest` in `transform_test.go`, reused by Task 2's end-to-end test (same package).

- [ ] **Step 1: Write failing tests** — create `cmd/archive/transform_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

// Mirrors the structure of the generator's output (templates/schedule.html).
const fixtureHTML = `<!doctype html>
<html>
  <head>
    <title>Lightning Game Schedule</title>
    <link rel="icon" href="/favicon.ico" />
    <link rel="manifest" href="/manifest.json" />
  </head>
  <body>
    <h1>⚡️ Lightning Game Schedule [12-3]</h1>
    <div class="filter-buttons">
      <a href="/" class="filter-btn active">All Teams</a>
      <a href="/10ublack" class="filter-btn black">10U Black</a>
      <button id="onlyUpcoming" class="filter-btn">Only Upcoming</button>
    </div>
    <div class="schedule-body">
      <table><tbody><tr><td><a href="https://maps.google.com/?q=Some+Gym">Gym</a></td></tr></tbody></table>
    </div>
    <div class="calendar">
      <p class="instructions">Subscribe to this schedule 👇</p>
      <p><a href="webcal://example.com/schedule.ics">Add to Apple</a></p>
    </div>
    <script>
      document.addEventListener("DOMContentLoaded", applyFilters);
      document.addEventListener("DOMContentLoaded", handleTimestamps);
    </script>
  </body>
</html>
`

const fixtureManifest = `{
  "name": "Lightning Schedule",
  "icons": [
    {
      "src": "/android-chrome-192x192.png",
      "sizes": "192x192"
    }
  ],
  "start_url": "/"
}
`

func TestTransformHTML(t *testing.T) {
	out, err := transformHTML(fixtureHTML, "2025")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, gone := range []string{
		`<div class="calendar">`,
		`webcal://`,
		`id="onlyUpcoming"`,
		`document.addEventListener("DOMContentLoaded", applyFilters);`,
	} {
		if strings.Contains(out, gone) {
			t.Errorf("expected %q to be removed", gone)
		}
	}

	for _, want := range []string{
		`localStorage.removeItem("onlyUpcoming");`,
		`document.addEventListener("DOMContentLoaded", handleTimestamps);`,
		`href="/2025/"`,
		`href="/2025/10ublack"`,
		`href="/2025/favicon.ico"`,
		`href="/2025/manifest.json"`,
		`href="https://maps.google.com/?q=Some+Gym"`,
		`<title>2025 Lightning Game Schedule</title>`,
		`<h1>⚡️ Lightning Game Schedule [12-3] (2025)</h1>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q", want)
		}
	}
}

func TestTransformHTMLMissingMarker(t *testing.T) {
	_, err := transformHTML("<html><body>no markers</body></html>", "2025")
	if err == nil {
		t.Fatal("expected error for HTML without generated markers")
	}
}

func TestTransformManifest(t *testing.T) {
	out, err := transformManifest(fixtureManifest, "2025")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		`"src": "/2025/android-chrome-192x192.png"`,
		`"start_url": "/2025/"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected manifest to contain %q", want)
		}
	}
}

func TestTransformManifestMissingStartURL(t *testing.T) {
	_, err := transformManifest(`{"name": "x"}`, "2025")
	if err == nil {
		t.Fatal("expected error for manifest without start_url")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/archive/`
Expected: compile error — `undefined: transformHTML` / `transformManifest`.

- [ ] **Step 3: Implement** — create `cmd/archive/transform.go`:

```go
package main

import (
	"fmt"
	"strings"
)

const (
	calendarStart   = `<div class="calendar">`
	divClose        = `</div>`
	onlyUpcomingBtn = `<button id="onlyUpcoming" class="filter-btn">Only Upcoming</button>`
	applyFiltersReg = `document.addEventListener("DOMContentLoaded", applyFilters);`
	clearFilterPref = `localStorage.removeItem("onlyUpcoming");`
)

// transformHTML converts one generated page into its archived form. It is
// strict: a missing marker means the input isn't a page this tool
// understands, and erroring beats silently producing a broken archive.
func transformHTML(html, name string) (string, error) {
	// 1. Remove the calendar block (the generated block has no nested divs)
	start := strings.Index(html, calendarStart)
	if start == -1 {
		return "", fmt.Errorf("calendar div not found")
	}
	rest := html[start:]
	end := strings.Index(rest, divClose)
	if end == -1 {
		return "", fmt.Errorf("calendar div not closed")
	}
	html = html[:start] + html[start+end+len(divClose):]

	// 2. Remove the Only Upcoming button
	if !strings.Contains(html, onlyUpcomingBtn) {
		return "", fmt.Errorf("Only Upcoming button not found")
	}
	html = strings.Replace(html, onlyUpcomingBtn, "", 1)

	// 3. Neutralize the filter JS and unset the shared localStorage pref
	if !strings.Contains(html, applyFiltersReg) {
		return "", fmt.Errorf("applyFilters registration not found")
	}
	html = strings.Replace(html, applyFiltersReg, clearFilterPref, 1)

	// 4. Rewrite root-absolute links (external links don't start with href="/)
	html = strings.ReplaceAll(html, `href="/`, `href="/`+name+`/`)
	// The rewrite turns href="/" into href="/name/" and href="/slug" into
	// href="/name/slug" in one pass; nothing else in the generated markup
	// begins with href="/.

	// 5. Prepend the archive name to the page title
	if !strings.Contains(html, "<title>") {
		return "", fmt.Errorf("title tag not found")
	}
	html = strings.Replace(html, "<title>", "<title>"+name+" ", 1)

	// 6. Append the archive name to the h1
	if !strings.Contains(html, "</h1>") {
		return "", fmt.Errorf("h1 tag not found")
	}
	html = strings.Replace(html, "</h1>", " ("+name+")</h1>", 1)

	return html, nil
}

// transformManifest points the PWA manifest's icon paths and start_url at
// the archive subdirectory.
func transformManifest(manifest, name string) (string, error) {
	if !strings.Contains(manifest, `"start_url": "/"`) {
		return "", fmt.Errorf(`manifest "start_url": "/" not found`)
	}
	manifest = strings.ReplaceAll(manifest, `"src": "/`, `"src": "/`+name+`/`)
	manifest = strings.Replace(manifest, `"start_url": "/"`, `"start_url": "/`+name+`/"`, 1)
	return manifest, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/archive/`
Expected: PASS (4 tests). Also `gofmt -l .` and `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/archive/transform.go cmd/archive/transform_test.go
git commit -m "Add archive HTML/manifest transforms"
```

---

### Task 2: CLI, copy walk, and docs

**Files:**
- Create: `cmd/archive/main.go`
- Test: `cmd/archive/main_test.go`
- Modify: `CLAUDE.md` (add archiving section)

**Interfaces:**
- Consumes: `transformHTML(html, name string) (string, error)`, `transformManifest(manifest, name string) (string, error)`, and test fixtures `fixtureHTML`/`fixtureManifest` from Task 1 (same package).
- Produces: `run(srcDir, name string) error` (called by `main`); CLI `go run ./cmd/archive <source-dir> <archive-name>`.

- [ ] **Step 1: Write failing tests** — create `cmd/archive/main_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildFakeSite creates a miniature generated-site layout:
// index.html, one team page, a symlinked asset, manifest.json, schedule.ics.
func buildFakeSite(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	site := filepath.Join(root, "site")

	if err := os.MkdirAll(filepath.Join(site, "10ublack"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(site, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("index.html", fixtureHTML)
	writeFile(filepath.Join("10ublack", "index.html"), fixtureHTML)
	writeFile("manifest.json", fixtureManifest)
	writeFile("schedule.ics", "BEGIN:VCALENDAR")
	writeFile(filepath.Join("10ublack", "schedule.ics"), "BEGIN:VCALENDAR")

	// Symlinked asset, like build.sh's dist symlinks
	assetSrc := filepath.Join(root, "real-favicon.ico")
	if err := os.WriteFile(assetSrc, []byte("ICODATA"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(assetSrc, filepath.Join(site, "favicon.ico")); err != nil {
		t.Fatal(err)
	}
	return site
}

func TestRunArchivesSite(t *testing.T) {
	site := buildFakeSite(t)
	if err := run(site, "2025"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	dest := filepath.Join(site, "2025")

	// HTML transformed in both root and team dir
	for _, rel := range []string{"index.html", filepath.Join("10ublack", "index.html")} {
		data, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Fatalf("missing archived %s: %v", rel, err)
		}
		if !strings.Contains(string(data), `<title>2025 `) {
			t.Errorf("%s: title not transformed", rel)
		}
	}

	// Manifest transformed
	m, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m), `"start_url": "/2025/"`) {
		t.Error("manifest start_url not rewritten")
	}

	// Symlink dereferenced into a regular file with the real content
	fi, err := os.Lstat(filepath.Join(dest, "favicon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("archived favicon.ico is still a symlink")
	}
	data, _ := os.ReadFile(filepath.Join(dest, "favicon.ico"))
	if string(data) != "ICODATA" {
		t.Error("archived favicon.ico content wrong")
	}

	// .ics excluded everywhere
	for _, rel := range []string{"schedule.ics", filepath.Join("10ublack", "schedule.ics")} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err == nil {
			t.Errorf("%s should be excluded from archive", rel)
		}
	}
}

func TestRunRefusesExistingDest(t *testing.T) {
	site := buildFakeSite(t)
	if err := run(site, "2025"); err != nil {
		t.Fatal(err)
	}
	err := run(site, "2025")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got: %v", err)
	}
}

func TestRunRejectsBadName(t *testing.T) {
	site := buildFakeSite(t)
	err := run(site, "20/25")
	if err == nil || !strings.Contains(err.Error(), "letters, digits") {
		t.Fatalf("expected bad-name error, got: %v", err)
	}
}

func TestRunRejectsNonSiteDir(t *testing.T) {
	err := run(t.TempDir(), "2025")
	if err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("expected no-index.html error, got: %v", err)
	}
}

func TestRunSecondArchiveCoexists(t *testing.T) {
	site := buildFakeSite(t)
	if err := run(site, "2025"); err != nil {
		t.Fatal(err)
	}
	if err := run(site, "2026"); err != nil {
		t.Fatalf("second archive failed: %v", err)
	}
	// 2026 must not contain the nested 2025 archive
	if _, err := os.Stat(filepath.Join(site, "2026", "2025")); err == nil {
		t.Error("2026 archive should not contain the 2025 archive")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/archive/`
Expected: compile error — `undefined: run`.

- [ ] **Step 3: Implement** — create `cmd/archive/main.go`:

```go
// Command archive snapshots a generated season site (e.g. dist/lightning)
// into a self-contained subdirectory archive (e.g. dist/lightning/2025)
// with links rewritten and season-only UI removed. See
// docs/superpowers/specs/2026-07-20-season-archive-tool-design.md.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: go run ./cmd/archive <source-dir> <archive-name>")
		fmt.Println("  e.g. go run ./cmd/archive dist/lightning 2025")
		os.Exit(1)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(srcDir, name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("archive name %q must contain only letters, digits, or dashes (it becomes a URL path segment)", name)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "index.html")); err != nil {
		return fmt.Errorf("%s does not look like a generated site (no index.html)", srcDir)
	}
	dest := filepath.Join(srcDir, name)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite an archive", dest)
	}

	count := 0
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == dest {
				return fs.SkipDir
			}
			if isExistingArchive(srcDir, path) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == "schedule.ics" {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		data, err := os.ReadFile(path) // follows symlinks: archive is self-contained
		if err != nil {
			return err
		}
		switch {
		case strings.HasSuffix(path, ".html"):
			out, err := transformHTML(string(data), name)
			if err != nil {
				return fmt.Errorf("%s: %v", path, err)
			}
			data = []byte(out)
		case d.Name() == "manifest.json":
			out, err := transformManifest(string(data), name)
			if err != nil {
				return fmt.Errorf("%s: %v", path, err)
			}
			data = []byte(out)
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("🗄  Archived %d files to %s\n", count, dest)
	fmt.Printf("Publish with: scp -r %s <host>:<web-dir>/\n", dest)
	return nil
}

// isExistingArchive reports whether path is a direct subdirectory of srcDir
// that is a previous archive: name-shaped (letters/digits/dashes) and
// lacking a schedule.ics, which every generated team directory has.
func isExistingArchive(srcDir, path string) bool {
	if filepath.Dir(path) != filepath.Clean(srcDir) {
		return false
	}
	if !nameRe.MatchString(filepath.Base(path)) {
		return false
	}
	_, err := os.Stat(filepath.Join(path, "schedule.ics"))
	return err != nil // no schedule.ics feed → not a team page → an archive
}

```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./...`
Expected: all pass — 9 tests in root package, 9 in `cmd/archive` (4 transform + 5 CLI). Also `gofmt -l .` and `go vet ./...` clean.

- [ ] **Step 5: Manual verification**

```bash
go run generate.go               # fresh dist/lightning
./build.sh                       # ensure asset symlinks exist
go run ./cmd/archive dist/lightning 2025
ls dist/lightning/2025/          # team dirs + assets, no schedule.ics
grep -c "2025" dist/lightning/2025/index.html
open dist/lightning/2025/index.html   # visual check: banner "(2025)", no calendar section, no Only Upcoming button, team nav links work relative to /2025/
rm -rf dist/lightning/2025       # leave dist pristine per CLAUDE.md
```

- [ ] **Step 6: Add archiving section to `CLAUDE.md`** — append:

```markdown
## Archiving a Season

`go run ./cmd/archive dist/lightning 2025` snapshots the generated site into
`dist/lightning/2025/` (links rewritten, calendar UI removed). Publish by
copying that directory to the server web dir once; the cron never touches it.
```

- [ ] **Step 7: Commit**

```bash
git add cmd/archive/main.go cmd/archive/main_test.go CLAUDE.md
git commit -m "Add season archive command"
```
