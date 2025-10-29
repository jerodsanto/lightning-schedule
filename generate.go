package main

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

//go:embed templates/schedule.html
var scheduleTemplate string

//go:embed templates/schedule.css
var stylesCSS string

//go:embed templates/schedule.js
var scheduleJS string

const domain = "schedule.omahalightningbasketball.com"

const googleSheetID = "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ"
const googleSheetCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv"
const googleSheetNotesCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv&gid=436458989"
const googleSheetLocationsCSVURL = "https://docs.google.com/spreadsheets/d/" + googleSheetID + "/export?format=csv&gid=1311642203"

// Location represents a game location
type Location struct {
	Abbrev  string
	Name    string
	Address string
}

func fetchLocations() ([]Location, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(googleSheetLocationsCSVURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching locations sheet: %v", err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	var locations []Location

	// Read header row
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}

	// Parse data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Expected columns: Abbreviation, Full Location Name, Address
		if len(record) < 3 {
			continue
		}

		abbreviation := strings.TrimSpace(record[0])
		fullName := strings.TrimSpace(record[1])
		address := strings.TrimSpace(record[2])

		// Skip rows with missing data
		if fullName == "" {
			continue
		}

		locations = append(locations, Location{
			Abbrev:  abbreviation,
			Name:    fullName,
			Address: address,
		})
	}

	return locations, nil
}

var locations []Location

// findLocationByName searches for a location by its full name
// Returns the Location and any court/gym info after the hyphen
func findLocationByName(name string) (*Location, string) {
	name = strings.TrimSpace(name)
	if name == "" || name == "TBD" {
		return nil, ""
	}

	// Strip out court/gym info if present (e.g., "Venue Name - Court 1" -> "Venue Name")
	baseName := name
	courtGymInfo := ""
	if idx := strings.Index(name, " - "); idx != -1 {
		baseName = strings.TrimSpace(name[:idx])
		courtGymInfo = strings.TrimSpace(name[idx+3:])
	}

	for i := range locations {
		if locations[i].Name == baseName {
			return &locations[i], courtGymInfo
		}
	}
	return nil, courtGymInfo
}

// findLocationByAbbrev searches for a location by its abbreviation
// Returns the Location and any court/gym info after the hyphen
func findLocationByAbbrev(abbrev string) (*Location, string) {
	abbrev = strings.TrimSpace(abbrev)
	if abbrev == "" || abbrev == "TBD" {
		return nil, ""
	}

	// Strip out court/gym info if present
	baseAbbrev := abbrev
	courtGymInfo := ""
	if idx := strings.Index(abbrev, " - "); idx != -1 {
		baseAbbrev = strings.TrimSpace(abbrev[:idx])
		courtGymInfo = strings.TrimSpace(abbrev[idx+3:])
	}

	for i := range locations {
		if locations[i].Abbrev == baseAbbrev {
			return &locations[i], courtGymInfo
		}
	}
	return nil, courtGymInfo
}

type Team struct {
	URL      string
	HTMLName string
	CssClass string
	Order    int
}

var AllTeams = map[string]Team{
	"Varsity": {
		URL:      "", // No URL - only from Google Sheet
		HTMLName: "",
		CssClass: "varsity",
		Order:    1,
	},
	"JV": {
		URL:      "", // No URL - only from Google Sheet
		HTMLName: "",
		CssClass: "jv",
		Order:    2,
	},
	"14U Gold": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h2025080322162058474d91e7d042e47",
		HTMLName: "Omaha Lightning Gold 8th",
		CssClass: "gold",
		Order:    3,
	},
	"14U White": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h20250803221620558cb62c45d697d46",
		HTMLName: "Omaha Lightning White 8th",
		CssClass: "white",
		Order:    4,
	},
	"12U Blue": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263029c941335204c&IDTeam=h20250803221620486ddba884e17c748",
		HTMLName: "Omaha Lightning Blue 6th",
		CssClass: "blue",
		Order:    5,
	},
	"10U Red": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263e6b6d69f385c49&IDTeam=h202508032216206132b484a6720f345",
		HTMLName: "Omaha Lightning Red 4th",
		CssClass: "red",
		Order:    6,
	},
	"10U Black": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263934d14719c5d45&IDTeam=h202508032216205157e930ef2d5314d",
		HTMLName: "Omaha Lightning Black 3rd",
		CssClass: "black",
		Order:    7,
	},
}

// Game represents a single game
type Game struct {
	Team         string
	Date         string
	Time         string
	Location     *Location
	CourtGymInfo string // Court/Gym information (e.g., "court 1", "gym a")
	Opponent     string
	HomeAway     string
	Score        string
	Result       string // "W", "L", or "" for unplayed games
	CssClass     string
}

// Note represents a note to display on a specific date
type Note struct {
	Date     string
	Text     string
	HTMLText template.HTML // HTML-safe version of Text for template rendering
	Teams    string        // Comma-separated team names or "All Teams"
}

// ScheduleItem represents either a game or a note in the schedule
type ScheduleItem struct {
	IsNote bool
	Game   *Game
	Note   *Note
}

func getTeamSlug(teamName string) string {
	return strings.ToLower(strings.ReplaceAll(teamName, " ", ""))
}

func getTeamCssClass(teamName string) string {
	if Team, exists := AllTeams[teamName]; exists {
		return Team.CssClass
	} else {
		return "unknown"
	}
}

func fetchGoogleSheetGames() ([]Game, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(googleSheetCSVURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching Google Sheet: %v", err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	var games []Game

	// Read header row
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}

	// Parse data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Expected columns: Team, Date, Time, Location, Jersey, Opponent, Score
		if len(record) < 6 {
			continue
		}

		team := strings.TrimSpace(record[0])
		date := strings.TrimSpace(record[1])
		timeStr := strings.TrimSpace(record[2])
		location := strings.TrimSpace(record[3])
		jersey := strings.TrimSpace(record[4])
		opponent := strings.TrimSpace(record[5])
		score := ""
		if len(record) >= 7 {
			score = strings.TrimSpace(record[6])
		}

		// Skip rows with missing critical data
		if team == "" || date == "" || opponent == "" {
			continue
		}

		// Determine home/away from jersey field
		homeAway := ""
		jerseyLower := strings.ToLower(jersey)
		if strings.Contains(jerseyLower, "home") || strings.Contains(jerseyLower, "light") {
			homeAway = "Home"
		} else if strings.Contains(jerseyLower, "away") || strings.Contains(jerseyLower, "dark") {
			homeAway = "Away"
		}

		// Parse date to standard format
		formattedDate := date
		if dateObj, err := time.Parse("1/2/2006", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		} else if dateObj, err := time.Parse("01/02/2006", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		} else if dateObj, err := time.Parse("1/2/06", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		}

		if timeStr == "" {
			timeStr = "TBD"
		}
		if location == "" {
			location = "TBD"
		}

		// Parse score and add W/L if score is in format "ourScore-theirScore"
		result := ""
		if score != "" && score != "-" {
			scoreParts := strings.Split(score, "-")
			if len(scoreParts) == 2 {
				ourScore, err1 := strconv.Atoi(strings.TrimSpace(scoreParts[0]))
				theirScore, err2 := strconv.Atoi(strings.TrimSpace(scoreParts[1]))
				if err1 == nil && err2 == nil {
					if ourScore > theirScore {
						result = "W"
					} else {
						result = "L"
					}
				}
			}
		}

		// Find location by abbreviation (Google Sheets uses abbreviations)
		loc, courtGymInfo := findLocationByAbbrev(location)

		games = append(games, Game{
			Team:         team,
			Date:         formattedDate,
			Time:         timeStr,
			Location:     loc,
			CourtGymInfo: courtGymInfo,
			Opponent:     opponent,
			HomeAway:     homeAway,
			Score:        score,
			Result:       result,
			CssClass:     getTeamCssClass(team),
		})
	}

	return games, nil
}

// parseNoteTextWithLinks converts note text to HTML, handling embedded links
// Supports formats:
// - Plain URLs: http://example.com -> clickable link
// - Markdown style: [text](url) -> <a href="url">text</a>
// - Just pass through any existing HTML from copy-paste
func parseNoteTextWithLinks(text string) string {
	// First, convert markdown-style links [text](url)
	markdownLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\)]+)\)`)
	text = markdownLinkRegex.ReplaceAllString(text, `<a href="$2" target="_blank">$1</a>`)

	// Then convert bare URLs that aren't already in anchor tags
	urlRegex := regexp.MustCompile(`(?:^|[^"'>])(https?://[^\s<]+)`)
	text = urlRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Check if this URL is already part of an href attribute
		if strings.Contains(match, `href="`) {
			return match
		}
		// Extract just the URL part (might have leading space/character)
		parts := strings.SplitN(match, "http", 2)
		if len(parts) == 2 {
			url := "http" + parts[1]
			return parts[0] + fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, url, url)
		}
		return match
	})

	return text
}

func fetchGoogleSheetNotes() ([]Note, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(googleSheetNotesCSVURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching Google Sheet notes: %v", err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	var notes []Note

	// Read header row
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}

	// Parse data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Expected columns: Date, Text, Link URL (optional), Teams
		if len(record) < 2 {
			continue
		}

		date := strings.TrimSpace(record[0])
		text := strings.TrimSpace(record[1])

		// Skip rows with missing data
		if date == "" || text == "" {
			continue
		}

		// Parse the text for embedded links (markdown style or bare URLs)
		text = parseNoteTextWithLinks(text)

		// Get teams from third column (or default to empty string)
		teams := ""
		if len(record) >= 3 {
			teams = strings.TrimSpace(record[2])
		}

		// Parse date to standard format
		formattedDate := date
		if dateObj, err := time.Parse("1/2/2006", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		} else if dateObj, err := time.Parse("01/02/2006", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		} else if dateObj, err := time.Parse("1/2/06", date); err == nil {
			formattedDate = dateObj.Format("Monday, January 2, 2006")
		}

		notes = append(notes, Note{
			Date:     formattedDate,
			Text:     text,
			HTMLText: template.HTML(text),
			Teams:    teams,
		})
	}

	return notes, nil
}

func scrapeTeamSchedule(displayName, url, htmlName, CssClass string) ([]Game, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create request with browser-like headers to avoid Cloudflare blocking
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %v", displayName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received status code %d for %s", resp.StatusCode, displayName)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var games []Game
	currentDate := ""

	// Find all tables and look for schedule data
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			// Check if this is a header row with a date
			thCells := row.Find("th")
			if thCells.Length() > 0 {
				headerText := strings.TrimSpace(row.Text())
				// Look for date pattern like "Saturday, October 18, 2025"
				if matched, _ := regexp.MatchString(`\w+day,\s+\w+\s+\d+,\s+\d{4}`, headerText); matched {
					currentDate = headerText
				}
			}

			// Look for table data rows
			cells := row.Find("td")

			// The schedule table has 8 columns: Game, Time, Location, Visitor, Visitor Score, Home Score, Home, (blank)
			if cells.Length() == 8 && currentDate != "" {
				gameNum := strings.TrimSpace(cells.Eq(0).Text())
				timeStr := strings.TrimSpace(cells.Eq(1).Text())
				location := strings.TrimSpace(cells.Eq(2).Text())
				visitor := strings.TrimSpace(cells.Eq(3).Text())
				visitorScore := strings.TrimSpace(cells.Eq(4).Text())
				homeScore := strings.TrimSpace(cells.Eq(5).Text())
				home := strings.TrimSpace(cells.Eq(6).Text())

				// Remove date prefix from time if present (e.g., "Sat 10/18/25 6:00 PM" -> "6:00 PM")
				re := regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+\d+/\d+/\d+\s+`)
				timeStr = re.ReplaceAllString(timeStr, "")

				// Check if this row has valid time data
				if matched, _ := regexp.MatchString(`\d+:\d+`, timeStr); !matched || gameNum == "" {
					return
				}

				// Determine opponent based on whether our team is home or away
				opponent := ""
				homeAway := ""
				score := ""
				result := ""

				if visitor == htmlName {
					opponent = home
					homeAway = "Away"
					if visitorScore != "×" && homeScore != "×" && visitorScore != "" && homeScore != "" {
						// We are visitor, so our score is visitorScore
						ourScore, _ := strconv.Atoi(visitorScore)
						theirScore, _ := strconv.Atoi(homeScore)
						if ourScore > theirScore {
							result = "W"
						} else {
							result = "L"
						}
						score = fmt.Sprintf("%s %s-%s", result, visitorScore, homeScore)
					} else {
						score = ""
					}
				} else if home == htmlName {
					opponent = visitor
					homeAway = "Home"
					if visitorScore != "×" && homeScore != "×" && visitorScore != "" && homeScore != "" {
						// We are home, so our score is homeScore
						ourScore, _ := strconv.Atoi(homeScore)
						theirScore, _ := strconv.Atoi(visitorScore)
						if ourScore > theirScore {
							score = fmt.Sprintf("W %s-%s", homeScore, visitorScore)
							result = "W"
						} else {
							score = fmt.Sprintf("L %s-%s", homeScore, visitorScore)
							result = "L"
						}
					} else {
						score = ""
					}
				} else {
					// Skip this row if it doesn't contain our team
					return
				}

				// Find location by name (TourneyMachine uses full location names)
				loc, courtGymInfo := findLocationByName(location)

				games = append(games, Game{
					Team:         displayName,
					Date:         currentDate,
					Time:         timeStr,
					Location:     loc,
					CourtGymInfo: courtGymInfo,
					Opponent:     opponent,
					HomeAway:     homeAway,
					Score:        score,
					Result:       result,
					CssClass:     CssClass,
				})
			}
		})
	})

	return games, nil
}

// parseDateForSorting parses date string for sorting
func parseDateForSorting(dateStr string) time.Time {
	// Handle format like "Saturday, October 18, 2025"
	layouts := []string{
		"Monday, January 2, 2006",
		"Monday, January 02, 2006",
		"1/2/2006",
		"01/02/2006",
		"1/2/06",
	}

	for _, layout := range layouts {
		if date, err := time.Parse(layout, dateStr); err == nil {
			return date
		}
	}

	// If parsing fails, return far future date
	return time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
}

// parseTimeToMinutes parses time string to minutes for sorting
func parseTimeToMinutes(timeStr string) int {
	// Parse time like "6:00 PM" or "10:30 AM"
	re := regexp.MustCompile(`(\d+):(\d+)\s*(AM|PM)`)
	match := re.FindStringSubmatch(timeStr)
	if len(match) == 4 {
		hours, _ := strconv.Atoi(match[1])
		minutes, _ := strconv.Atoi(match[2])
		ampm := strings.ToUpper(match[3])

		if ampm == "PM" && hours != 12 {
			hours += 12
		} else if ampm == "AM" && hours == 12 {
			hours = 0
		}

		return hours*60 + minutes
	}
	return 9999 // Default to end of day if can't parse
}

// formatTime removes unnecessary :00
// Examples: "4:00 PM" -> "4PM", "9:30 AM" -> "9:30AM"
func formatTime(timeStr string) string {
	if timeStr == "" || timeStr == "TBD" {
		return timeStr
	}

	// Match time pattern like "4:00 PM" or "9:30 AM"
	re := regexp.MustCompile(`(\d+):(\d+)\s*(AM|PM)`)
	match := re.FindStringSubmatch(timeStr)
	if len(match) == 4 {
		hours := match[1]
		minutes := match[2]
		ampm := strings.ToUpper(match[3])

		// If minutes are 00, omit them
		if minutes == "00" {
			return fmt.Sprintf("%s%s", hours, ampm)
		}
		return fmt.Sprintf("%s:%s%s", hours, minutes, ampm)
	}

	return timeStr
}

// convertLinksToHTML converts URLs in text to HTML anchor tags
func convertLinksToHTML(text string) string {
	// Match URLs (http:// or https://)
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	return urlRegex.ReplaceAllStringFunc(text, func(url string) string {
		return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, url, url)
	})
}

// Template data structures
type TeamButton struct {
	Name     string
	Link     string
	CssClass string
	IsActive bool
}

type TemplateScheduleItem struct {
	IsNote          bool
	IsWeekStart     bool
	IsPastGame      bool
	Game            *Game
	Note            *Note
	DisplayDateTime string
	LocationHTML    template.HTML
	JerseyText      string
	OpponentDisplay string
	ScoreDisplay    string
}

type TemplateData struct {
	ProdDomain     string
	PageTitle      string
	PagePath       string
	UpdatedUTC     string
	UpdatedDisplay string
	AllTeamsLink   string
	IsAllTeams     bool
	TeamRecord     string
	Teams          []TeamButton
	ScheduleItems  []TemplateScheduleItem
	StylesCSS      template.CSS
	ScheduleJS     template.JS
}

// generateHTML generates HTML schedule page using templates
func generateHTML(allGames []Game, allNotes []Note, outputFile string, filterTeam string) error {
	// Parse the embedded template
	tmpl, err := template.New("schedule").Parse(scheduleTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	// Filter games if a specific team is requested
	var gamesToDisplay []Game
	if filterTeam != "" {
		for _, game := range allGames {
			if game.Team == filterTeam {
				gamesToDisplay = append(gamesToDisplay, game)
			}
		}
	} else {
		gamesToDisplay = allGames
	}

	// Filter notes based on the team filter
	var notesToDisplay []Note
	for _, note := range allNotes {
		// For combined schedule (no filter), show all notes
		if filterTeam == "" {
			notesToDisplay = append(notesToDisplay, note)
		} else {
			// For team pages, only show notes that:
			// 1. Have "All Teams" in the Teams column (case-insensitive), OR
			// 2. Have the team name in the Teams column
			teamsLower := strings.ToLower(note.Teams)
			filterTeamLower := strings.ToLower(filterTeam)

			if teamsLower == "all teams" || strings.Contains(teamsLower, filterTeamLower) {
				notesToDisplay = append(notesToDisplay, note)
			}
		}
	}

	// Create combined list of schedule items (games and notes)
	var scheduleItems []ScheduleItem

	// Add all games as schedule items
	for i := range gamesToDisplay {
		scheduleItems = append(scheduleItems, ScheduleItem{
			IsNote: false,
			Game:   &gamesToDisplay[i],
		})
	}

	// Add all notes as schedule items
	for i := range notesToDisplay {
		scheduleItems = append(scheduleItems, ScheduleItem{
			IsNote: true,
			Note:   &notesToDisplay[i],
		})
	}

	// Sort schedule items by date and time
	sort.Slice(scheduleItems, func(i, j int) bool {
		var dateA, dateB time.Time

		// Get dates for comparison
		if scheduleItems[i].IsNote {
			dateA = parseDateForSorting(scheduleItems[i].Note.Date)
		} else {
			dateA = parseDateForSorting(scheduleItems[i].Game.Date)
		}

		if scheduleItems[j].IsNote {
			dateB = parseDateForSorting(scheduleItems[j].Note.Date)
		} else {
			dateB = parseDateForSorting(scheduleItems[j].Game.Date)
		}

		// First sort by date
		if !dateA.Equal(dateB) {
			return dateA.Before(dateB)
		}

		// Notes come before games on the same date
		if scheduleItems[i].IsNote && !scheduleItems[j].IsNote {
			return true
		}
		if !scheduleItems[i].IsNote && scheduleItems[j].IsNote {
			return false
		}

		// Both are notes - maintain order
		if scheduleItems[i].IsNote && scheduleItems[j].IsNote {
			return false
		}

		// Both are games - sort by time then team
		gameA := scheduleItems[i].Game
		gameB := scheduleItems[j].Game

		timeA := gameA.Time
		timeB := gameB.Time
		isTBDA := timeA == "TBD" || timeA == ""
		isTBDB := timeB == "TBD" || timeB == ""

		// If both have times or both are TBD, sort by time then team
		if !isTBDA && !isTBDB {
			// Both have times - sort by time
			timeMinA := parseTimeToMinutes(timeA)
			timeMinB := parseTimeToMinutes(timeB)
			if timeMinA != timeMinB {
				return timeMinA < timeMinB
			}
			// Same time - sort by team order
			orderA := AllTeams[gameA.Team].Order
			orderB := AllTeams[gameB.Team].Order
			if orderA != orderB {
				return orderA < orderB
			}
			return gameA.Team < gameB.Team
		}

		if isTBDA && isTBDB {
			// Both are TBD - group by team
			orderA := AllTeams[gameA.Team].Order
			orderB := AllTeams[gameB.Team].Order
			if orderA != orderB {
				return orderA < orderB
			}
			return gameA.Team < gameB.Team
		}

		// One has time, one is TBD - games with times come first
		return !isTBDA
	})

	// Get unique teams and sort by their Order field in AllTeams
	teamSet := make(map[string]bool)
	for _, game := range allGames {
		teamSet[game.Team] = true
	}

	var teams []string
	for team := range teamSet {
		teams = append(teams, team)
	}

	// Sort teams by their Order field, with teams not in AllTeams map at the end
	sort.Slice(teams, func(i, j int) bool {
		infoI, hasI := AllTeams[teams[i]]
		infoJ, hasJ := AllTeams[teams[j]]

		// Both have order info - sort by order
		if hasI && hasJ {
			return infoI.Order < infoJ.Order
		}
		// Only i has order info - it comes first
		if hasI {
			return true
		}
		// Only j has order info - it comes first
		if hasJ {
			return false
		}
		// Neither has order info - sort alphabetically
		return teams[i] < teams[j]
	})

	now := time.Now().UTC()

	// Determine page title and path based on filter
	pageTitle := "Lightning"
	pagePath := "/"
	teamRecord := ""

	if filterTeam != "" {
		pageTitle = filterTeam
		pagePath = "/" + getTeamSlug(filterTeam) + "/"

		// Calculate W-L record for team pages
		wins := 0
		losses := 0
		for _, game := range gamesToDisplay {
			if game.Result == "W" {
				wins++
			} else if game.Result == "L" {
				losses++
			}
		}
		if wins > 0 || losses > 0 {
			teamRecord = fmt.Sprintf(" [%d-%d]", wins, losses)
		}
	}

	// Prepare team buttons
	var teamButtons []TeamButton

	for _, team := range teams {
		teamSlug := getTeamSlug(team)
		teamLink := "/" + teamSlug + "/"

		teamButtons = append(teamButtons, TeamButton{
			Name:     team,
			Link:     teamLink,
			CssClass: getTeamCssClass(team),
			IsActive: filterTeam == team,
		})
	}

	// Prepare template schedule items
	var templateItems []TemplateScheduleItem
	for i, item := range scheduleItems {
		if item.IsNote {
			templateItems = append(templateItems, TemplateScheduleItem{
				IsNote: true,
				Note:   item.Note,
			})
			continue
		}

		game := item.Game

		// Determine if this is the first game of a new calendar week
		isWeekStart := false
		currentDate := parseDateForSorting(game.Date)
		if currentDate.Year() != 2099 {
			if i == 0 {
				isWeekStart = true
			} else {
				// Look backwards to find the previous game (skip notes)
				for j := i - 1; j >= 0; j-- {
					if !scheduleItems[j].IsNote {
						prevDate := parseDateForSorting(scheduleItems[j].Game.Date)
						currentYear, currentWeek := currentDate.ISOWeek()
						prevYear, prevWeek := prevDate.ISOWeek()
						if currentYear != prevYear || currentWeek != prevWeek {
							isWeekStart = true
						}
						break
					}
				}
			}
		}

		// Combine date and time in format: "Sat Oct 18 11AM"
		displayDateTime := "TBD"
		dateObj := parseDateForSorting(game.Date)
		if dateObj.Year() != 2099 {
			weekday := dateObj.Format("Mon")
			month := dateObj.Format("Jan")
			day := dateObj.Day()
			timeFormatted := formatTime(game.Time)

			if timeFormatted == "TBD" {
				displayDateTime = fmt.Sprintf("%s %s %d TBD", weekday, month, day)
			} else {
				displayDateTime = fmt.Sprintf("%s %s %d %s", weekday, month, day, timeFormatted)
			}
		}

		// Format jersey text
		jerseyText := "TBD"
		if game.HomeAway == "Home" {
			jerseyText = "⬜️"
		} else if game.HomeAway == "Away" {
			jerseyText = "⬛️"
		}

		// Generate location HTML with Google Maps link if address is available
		var locationHTML template.HTML
		if game.Location != nil {
			var locDisplay string
			if game.Location.Address != "" {
				// Location has an address - make it a Google Maps link
				mapsURL := "https://maps.google.com/?q=" +
					strings.ReplaceAll(game.Location.Address, " ", "+")
				locDisplay = fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`,
					mapsURL, game.Location.Abbrev)
			} else {
				// Location has no address - just show the abbreviation
				locDisplay = game.Location.Abbrev
			}

			// Add court/gym info if present
			if game.CourtGymInfo != "" {
				locationHTML = template.HTML(fmt.Sprintf("%s (%s)", locDisplay, strings.ToLower(game.CourtGymInfo)))
			} else {
				locationHTML = template.HTML(locDisplay)
			}
		} else {
			locationHTML = template.HTML("TBD")
		}

		opponent := game.Opponent
		if opponent == "" {
			opponent = "TBD"
		}
		score := game.Score
		if score == "-" {
			score = ""
		}

		isPastGame := game.Result == "W" || game.Result == "L" || (dateObj.Year() != 2099 && dateObj.Before(now))

		templateItems = append(templateItems, TemplateScheduleItem{
			IsNote:          false,
			IsWeekStart:     isWeekStart,
			IsPastGame:      isPastGame,
			Game:            game,
			DisplayDateTime: displayDateTime,
			LocationHTML:    locationHTML,
			JerseyText:      jerseyText,
			OpponentDisplay: opponent,
			ScoreDisplay:    score,
		})
	}

	// Prepare template data
	data := TemplateData{
		PageTitle:      pageTitle,
		PagePath:       pagePath,
		ProdDomain:     domain,
		UpdatedUTC:     now.Format(time.RFC3339),
		UpdatedDisplay: now.Format("1/2/06") + " at " + now.Format("3:04PM") + " UTC",
		IsAllTeams:     filterTeam == "",
		TeamRecord:     teamRecord,
		Teams:          teamButtons,
		ScheduleItems:  templateItems,
		StylesCSS:      template.CSS(stylesCSS),
		ScheduleJS:     template.JS(scheduleJS),
	}

	// Create output file
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer f.Close()

	// Execute template
	err = tmpl.Execute(f, data)
	if err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	return nil
}

// generateICalendar generates an iCal file for games and notes
func generateICalendar(allGames []Game, allNotes []Note, outputFile string, filterTeam string) error {
	// Filter games if a specific team is requested
	var gamesToExport []Game
	if filterTeam != "" {
		for _, game := range allGames {
			if game.Team == filterTeam {
				gamesToExport = append(gamesToExport, game)
			}
		}
	} else {
		gamesToExport = allGames
	}

	// Filter notes based on the team filter
	var notesToExport []Note
	for _, note := range allNotes {
		// For combined schedule (no filter), show all notes
		if filterTeam == "" {
			notesToExport = append(notesToExport, note)
		} else {
			// For team calendars, only show notes that:
			// 1. Have "All Teams" in the Teams column (case-insensitive), OR
			// 2. Have the team name in the Teams column
			teamsLower := strings.ToLower(note.Teams)
			filterTeamLower := strings.ToLower(filterTeam)

			if teamsLower == "all teams" || strings.Contains(teamsLower, filterTeamLower) {
				notesToExport = append(notesToExport, note)
			}
		}
	}

	var ical strings.Builder

	// iCal header
	ical.WriteString("BEGIN:VCALENDAR\r\n")
	ical.WriteString("VERSION:2.0\r\n")
	ical.WriteString("PRODID:-//Omaha Lightning//Basketball Schedule//EN\r\n")
	ical.WriteString("CALSCALE:GREGORIAN\r\n")
	ical.WriteString("METHOD:PUBLISH\r\n")
	ical.WriteString("X-WR-CALNAME:Lightning Schedule")
	if filterTeam != "" {
		ical.WriteString(" - " + filterTeam)
	}
	ical.WriteString("\r\n")
	ical.WriteString("X-WR-TIMEZONE:America/Chicago\r\n")

	// Central timezone definition
	ical.WriteString("BEGIN:VTIMEZONE\r\n")
	ical.WriteString("TZID:America/Chicago\r\n")
	ical.WriteString("BEGIN:DAYLIGHT\r\n")
	ical.WriteString("TZOFFSETFROM:-0600\r\n")
	ical.WriteString("TZOFFSETTO:-0500\r\n")
	ical.WriteString("DTSTART:19700308T020000\r\n")
	ical.WriteString("RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU\r\n")
	ical.WriteString("TZNAME:CDT\r\n")
	ical.WriteString("END:DAYLIGHT\r\n")
	ical.WriteString("BEGIN:STANDARD\r\n")
	ical.WriteString("TZOFFSETFROM:-0500\r\n")
	ical.WriteString("TZOFFSETTO:-0600\r\n")
	ical.WriteString("DTSTART:19701101T020000\r\n")
	ical.WriteString("RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU\r\n")
	ical.WriteString("TZNAME:CST\r\n")
	ical.WriteString("END:STANDARD\r\n")
	ical.WriteString("END:VTIMEZONE\r\n")

	// Add game events
	for _, game := range gamesToExport {
		// Parse date
		dateObj := parseDateForSorting(game.Date)
		if dateObj.Year() == 2099 {
			continue // Skip games with invalid dates
		}

		// Parse time - determine if TBD
		isTBD := game.Time == "TBD" || game.Time == ""

		var startTime, endTime time.Time

		if isTBD {
			// All-day event for TBD games
			startTime = time.Date(dateObj.Year(), dateObj.Month(), dateObj.Day(), 0, 0, 0, 0, time.UTC)
			endTime = startTime.Add(24 * time.Hour)
		} else {
			// Parse time like "6:00 PM" or "10:30 AM"
			re := regexp.MustCompile(`(\d+):(\d+)\s*(AM|PM)`)
			match := re.FindStringSubmatch(game.Time)
			if len(match) == 4 {
				hours, _ := strconv.Atoi(match[1])
				minutes, _ := strconv.Atoi(match[2])
				ampm := strings.ToUpper(match[3])

				if ampm == "PM" && hours != 12 {
					hours += 12
				} else if ampm == "AM" && hours == 12 {
					hours = 0
				}

				// Create time in Central timezone
				centralLoc, _ := time.LoadLocation("America/Chicago")
				startTime = time.Date(dateObj.Year(), dateObj.Month(), dateObj.Day(), hours, minutes, 0, 0, centralLoc)
				// Assume games are 1 hour long
				endTime = startTime.Add(1 * time.Hour)
			} else {
				// Fallback to all-day if time parsing fails
				isTBD = true
				startTime = time.Date(dateObj.Year(), dateObj.Month(), dateObj.Day(), 0, 0, 0, 0, time.UTC)
				endTime = startTime.Add(24 * time.Hour)
			}
		}

		// Create event UID
		uid := fmt.Sprintf("game-%s-%s-%s@lightningschedule.local",
			strings.ReplaceAll(game.Team, " ", ""),
			dateObj.Format("20060102"),
			strings.ReplaceAll(game.Time, " ", ""))

		ical.WriteString("BEGIN:VEVENT\r\n")
		ical.WriteString("UID:" + uid + "\r\n")
		ical.WriteString("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z") + "\r\n")

		if isTBD {
			// All-day event format
			ical.WriteString("DTSTART;VALUE=DATE:" + startTime.Format("20060102") + "\r\n")
			ical.WriteString("DTEND;VALUE=DATE:" + endTime.Format("20060102") + "\r\n")
		} else {
			// Timed event format
			ical.WriteString("DTSTART;TZID=America/Chicago:" + startTime.Format("20060102T150405") + "\r\n")
			ical.WriteString("DTEND;TZID=America/Chicago:" + endTime.Format("20060102T150405") + "\r\n")
		}

		// Event title
		summary := game.Team + " vs " + game.Opponent
		if game.HomeAway == "Away" {
			summary = game.Team + " @ " + game.Opponent
		}
		ical.WriteString("SUMMARY:" + escapeICalText(summary) + "\r\n")

		// Description with game details
		description := fmt.Sprintf("Team: %s\\nOpponent: %s\\nJersey: %s",
			game.Team, game.Opponent, game.HomeAway)
		if game.Score != "" && game.Score != "-" {
			description += "\\nScore: " + game.Score
		}
		ical.WriteString("DESCRIPTION:" + escapeICalText(description) + "\r\n")

		// Location
		if game.Location != nil {
			ical.WriteString("LOCATION:" + escapeICalText(game.Location.Name) + "\r\n")
		}

		ical.WriteString("END:VEVENT\r\n")
	}

	// Add note events (all-day events)
	for _, note := range notesToExport {
		// Parse date
		dateObj := parseDateForSorting(note.Date)
		if dateObj.Year() == 2099 {
			continue // Skip notes with invalid dates
		}

		// All-day event for notes
		startTime := time.Date(dateObj.Year(), dateObj.Month(), dateObj.Day(), 0, 0, 0, 0, time.UTC)
		endTime := startTime.Add(24 * time.Hour)

		// Create event UID
		uid := fmt.Sprintf("note-%s-%s@lightningschedule.local",
			dateObj.Format("20060102"),
			fmt.Sprintf("%x", strings.ReplaceAll(note.Text, " ", "")))

		// Strip HTML tags from note text for plain text summary
		plainText := stripHTMLTags(note.Text)

		ical.WriteString("BEGIN:VEVENT\r\n")
		ical.WriteString("UID:" + uid + "\r\n")
		ical.WriteString("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z") + "\r\n")
		ical.WriteString("DTSTART;VALUE=DATE:" + startTime.Format("20060102") + "\r\n")
		ical.WriteString("DTEND;VALUE=DATE:" + endTime.Format("20060102") + "\r\n")
		ical.WriteString("SUMMARY:" + escapeICalText(plainText) + "\r\n")
		ical.WriteString("DESCRIPTION:" + escapeICalText(plainText) + "\r\n")
		ical.WriteString("END:VEVENT\r\n")
	}

	ical.WriteString("END:VCALENDAR\r\n")

	// Write to file
	err := os.WriteFile(outputFile, []byte(ical.String()), 0644)
	if err != nil {
		return fmt.Errorf("error writing iCal file: %v", err)
	}

	return nil
}

// escapeICalText escapes special characters for iCal text fields
func escapeICalText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, "\r", "")
	return text
}

// stripHTMLTags removes HTML tags from text
func stripHTMLTags(html string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	return text
}

func main() {
	var allGames []Game

	// Fetch locations from Google Sheet
	var err error
	locations, err = fetchLocations()
	if err != nil {
		fmt.Printf("Error fetching locations: %v\n", err)
		locations = []Location{} // Use empty slice if fetch fails
	}

	// Fetch games from team URLs (skip teams without URLs)
	for displayName, Team := range AllTeams {
		if Team.URL != "" {
			games, err := scrapeTeamSchedule(displayName, Team.URL, Team.HTMLName, Team.CssClass)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			allGames = append(allGames, games...)
		}
	}

	// Fetch additional games from Google Sheet
	sheetGames, err := fetchGoogleSheetGames()
	if err != nil {
		fmt.Printf("Error fetching Google Sheet: %v\n", err)
	} else {
		allGames = append(allGames, sheetGames...)
	}

	if len(allGames) == 0 {
		fmt.Println("No games found. Please check the URLs and try again.")
		os.Exit(1)
	}

	// Fetch notes from Google Sheet
	allNotes, err := fetchGoogleSheetNotes()
	if err != nil {
		fmt.Printf("Error fetching notes from Google Sheet: %v\n", err)
		allNotes = []Note{} // Use empty slice if fetch fails
	}

	// Get output directory from command line argument or use default "dist"
	outputDir := "dist"
	if len(os.Args) > 1 {
		outputDir = os.Args[1]
	}

	// Expand tilde if present
	if strings.HasPrefix(outputDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			outputDir = filepath.Join(homeDir, outputDir[2:])
		}
	}

	// Use the path as-is if it's absolute, otherwise treat as relative
	var distDir string
	if filepath.IsAbs(outputDir) {
		distDir = outputDir
	} else {
		distDir = filepath.Join(".", outputDir)
	}

	err = os.MkdirAll(distDir, 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Generate combined schedule as index.html in output directory
	err = generateHTML(allGames, allNotes, filepath.Join(distDir, "index.html"), "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Generate combined iCal file
	err = generateICalendar(allGames, allNotes, filepath.Join(distDir, "schedule.ics"), "")
	if err != nil {
		fmt.Printf("Error generating combined iCal: %v\n", err)
	}

	// Generate individual team schedules in subfolders
	teamSet := make(map[string]bool)
	for _, game := range allGames {
		teamSet[game.Team] = true
	}

	for team := range teamSet {
		teamSlug := strings.ToLower(strings.ReplaceAll(team, " ", ""))
		teamDir := filepath.Join(distDir, teamSlug)
		err = os.MkdirAll(teamDir, 0755)
		if err != nil {
			fmt.Printf("Error creating team directory: %v\n", err)
			continue
		}

		// Generate HTML for team
		err = generateHTML(allGames, allNotes, filepath.Join(teamDir, "index.html"), team)
		if err != nil {
			fmt.Printf("Error generating HTML for %s: %v\n", team, err)
		}

		// Generate iCal for team
		err = generateICalendar(allGames, allNotes, filepath.Join(teamDir, "schedule.ics"), team)
		if err != nil {
			fmt.Printf("Error generating iCal for %s: %v\n", team, err)
		}
	}

	fmt.Printf("💪 Generated schedule with %d games and %d notes\n", len(allGames), len(allNotes))
}
