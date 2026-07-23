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
    <div class="seasons">
      <p><a href="/2024/">2024</a> | <a href="/2025/">2025</a></p>
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
		`<div class="seasons">`,
		`href="/2024/"`,
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
