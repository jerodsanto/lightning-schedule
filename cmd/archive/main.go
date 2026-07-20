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
