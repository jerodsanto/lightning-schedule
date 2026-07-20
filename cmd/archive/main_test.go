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
