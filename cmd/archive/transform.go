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
