package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
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

// Google Sheet ID for additional games
const googleSheetID = "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ"

var googleSheetCSVURL = fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv", googleSheetID)

// Location abbreviations
// Maps base location names (without court/gym) to shorter versions
var locationAbbreviations = map[string]string{
	"UBT South Sports Complex (Attack-Elite)": "UBT South",
	"Trinity Classical Academy":               "TCA",
	"Elkhorn North Ridge Middle School":       "ENRMS",
	"Elkhorn Valley View Middle School":       "Valley View",
	"Elkhorn Ridge Middle School":             "ERMS",
	"Elkhorn Middle School":                   "Elkhorn Middle",
	"Elkhorn Grandview Middle School":         "Grandview Middle",
	"EPS Woodbrook Elementary":                "Woodbrook",
	"EPS West Dodge Station Elementary":       "West Dodge Station",
	"EPS Arbor View Elementary":               "Arbor View",
	"Nebraska Basketball Academy":             "Nebraska Basketball Academy",
	"Iowa West Fieldhouse":                    "",
}

// TeamInfo holds team configuration
type TeamInfo struct {
	URL      string
	HTMLName string
	Color    string
}

// Team URLs - Add more teams here
// Format: displayName: { URL: "...", HTMLName: "exact name as it appears in the HTML", Color: "#RRGGBB" }
var teamURLs = map[string]TeamInfo{
	"Varsity": {
		URL:      "", // No URL - only from Google Sheet
		HTMLName: "",
		Color:    "#f59c44", // orange
	},
	"JV": {
		URL:      "", // No URL - only from Google Sheet
		HTMLName: "",
		Color:    "#44a15b", // green
	},
	"14U Gold": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h2025080322162058474d91e7d042e47",
		HTMLName: "Omaha Lightning Gold 8th",
		Color:    "#FFD700",
	},
	"14U White": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h20250803221620558cb62c45d697d46",
		HTMLName: "Omaha Lightning White 8th",
		Color:    "#FFFFFF",
	},
	"12U Blue": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263029c941335204c&IDTeam=h20250803221620486ddba884e17c748",
		HTMLName: "Omaha Lightning Blue 6th",
		Color:    "#5b9de9",
	},
	"10U Red": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263e6b6d69f385c49&IDTeam=h202508032216206132b484a6720f345",
		HTMLName: "Omaha Lightning Red 4th",
		Color:    "#d53a44",
	},
	"10U Black": {
		URL:      "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263934d14719c5d45&IDTeam=h202508032216205157e930ef2d5314d",
		HTMLName: "Omaha Lightning Black 3rd",
		Color:    "#000000",
	},
}

// Default team color for teams not in teamURLs
const defaultTeamColor = "#2196F3"

// Game represents a single game
type Game struct {
	Team     string
	Date     string
	Time     string
	Location string
	Opponent string
	HomeAway string
	Score    string
	Color    string
}

// getTeamColor returns the team color or default
func getTeamColor(teamName string) string {
	if teamInfo, ok := teamURLs[teamName]; ok {
		return teamInfo.Color
	}
	return defaultTeamColor
}

// getTeamTextColor returns white for dark backgrounds, black for light backgrounds
// Uses relative luminance calculation (WCAG formula)
func getTeamTextColor(backgroundColor string) string {
	normalizedColor := strings.ToLower(backgroundColor)

	// Parse the color to RGB values
	var r, g, b int

	// Handle hex colors (#RRGGBB or #RGB)
	if strings.HasPrefix(normalizedColor, "#") {
		hex := normalizedColor[1:]
		if len(hex) == 6 {
			r64, _ := strconv.ParseInt(hex[0:2], 16, 64)
			g64, _ := strconv.ParseInt(hex[2:4], 16, 64)
			b64, _ := strconv.ParseInt(hex[4:6], 16, 64)
			r, g, b = int(r64), int(g64), int(b64)
		} else if len(hex) == 3 {
			r64, _ := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 64)
			g64, _ := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 64)
			b64, _ := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 64)
			r, g, b = int(r64), int(g64), int(b64)
		}
	}

	// Calculate relative luminance using WCAG formula
	// https://www.w3.org/TR/WCAG20/#relativeluminancedef
	rsRGB := float64(r) / 255.0
	gsRGB := float64(g) / 255.0
	bsRGB := float64(b) / 255.0

	var rLinear, gLinear, bLinear float64
	if rsRGB <= 0.03928 {
		rLinear = rsRGB / 12.92
	} else {
		rLinear = math.Pow((rsRGB+0.055)/1.055, 2.4)
	}
	if gsRGB <= 0.03928 {
		gLinear = gsRGB / 12.92
	} else {
		gLinear = math.Pow((gsRGB+0.055)/1.055, 2.4)
	}
	if bsRGB <= 0.03928 {
		bLinear = bsRGB / 12.92
	} else {
		bLinear = math.Pow((bsRGB+0.055)/1.055, 2.4)
	}

	luminance := 0.2126*rLinear + 0.7152*gLinear + 0.0722*bLinear

	// Use white text for dark colors (luminance < 0.5), black text for light colors
	if luminance < 0.5 {
		return "white"
	}
	return "black"
}

// fetchGoogleSheetGames fetches and parses games from Google Sheets
func fetchGoogleSheetGames() ([]Game, error) {
	fmt.Println("Fetching additional games from Google Sheet...")

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
		if score == "" {
			score = "-"
		}
		if location == "" {
			location = "TBD"
		}

		games = append(games, Game{
			Team:     team,
			Date:     formattedDate,
			Time:     timeStr,
			Location: location,
			Opponent: opponent,
			HomeAway: homeAway,
			Score:    score,
			Color:    getTeamColor(team),
		})
	}

	fmt.Printf("Found %d games in Google Sheet\n", len(games))
	return games, nil
}

// scrapeTeamSchedule scrapes schedule data for a single team
func scrapeTeamSchedule(displayName, url, htmlName, color string) ([]Game, error) {
	fmt.Printf("Scraping %s...\n", displayName)

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

				if visitor == htmlName {
					opponent = home
					homeAway = "Away"
					if visitorScore != "×" && homeScore != "×" {
						score = fmt.Sprintf("%s-%s", visitorScore, homeScore)
					} else {
						score = "-"
					}
				} else if home == htmlName {
					opponent = visitor
					homeAway = "Home"
					if visitorScore != "×" && homeScore != "×" {
						score = fmt.Sprintf("%s-%s", homeScore, visitorScore)
					} else {
						score = "-"
					}
				} else {
					// Skip this row if it doesn't contain our team
					return
				}

				games = append(games, Game{
					Team:     displayName,
					Date:     currentDate,
					Time:     timeStr,
					Location: location,
					Opponent: opponent,
					HomeAway: homeAway,
					Score:    score,
					Color:    color,
				})
			}
		})
	})

	fmt.Printf("Found %d games for %s\n", len(games), displayName)
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

// LocationDisplay holds location display information
type LocationDisplay struct {
	Abbr        string
	CourtGym    string
	TooltipText string
}

// getLocationDisplay returns abbreviated location with full name for tooltip
func getLocationDisplay(location string) LocationDisplay {
	if location == "" || location == "TBD" {
		return LocationDisplay{
			Abbr:        "TBD",
			CourtGym:    "",
			TooltipText: "TBD",
		}
	}

	// Split on hyphen to separate main location from court/gym info
	parts := strings.Split(location, " - ")

	if len(parts) == 2 {
		baseLocation := strings.TrimSpace(parts[0])
		courtGymInfo := strings.ToLower(strings.TrimSpace(parts[1]))

		// Check if abbreviation exists in the map
		if abbreviated, ok := locationAbbreviations[baseLocation]; ok {
			// If abbreviation is blank/empty, show full location without tooltip
			if strings.TrimSpace(abbreviated) == "" {
				return LocationDisplay{
					Abbr:        "",
					CourtGym:    "",
					TooltipText: location,
				}
			}

			// Return abbreviated location with separate court/gym info
			return LocationDisplay{
				Abbr:        abbreviated,
				CourtGym:    courtGymInfo,
				TooltipText: baseLocation,
			}
		}

		// No abbreviation found in map, but still format with court/gym
		return LocationDisplay{
			Abbr:        baseLocation,
			CourtGym:    courtGymInfo,
			TooltipText: baseLocation,
		}
	}

	// No hyphen separator - check if there's an abbreviation for the whole thing
	if abbreviated, ok := locationAbbreviations[location]; ok {
		// If abbreviation is blank/empty, show full location without tooltip
		if strings.TrimSpace(abbreviated) == "" {
			return LocationDisplay{
				Abbr:        "",
				CourtGym:    "",
				TooltipText: location,
			}
		}

		return LocationDisplay{
			Abbr:        abbreviated,
			CourtGym:    "",
			TooltipText: location,
		}
	}

	// If no abbreviation exists, use the full name for both
	return LocationDisplay{
		Abbr:        location,
		CourtGym:    "",
		TooltipText: location,
	}
}

// generateHTML generates HTML schedule page
func generateHTML(allGames []Game, outputFile string, filterTeam string) error {
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

	// Sort games by date and time
	sortedGames := make([]Game, len(gamesToDisplay))
	copy(sortedGames, gamesToDisplay)
	sort.Slice(sortedGames, func(i, j int) bool {
		dateA := parseDateForSorting(sortedGames[i].Date)
		dateB := parseDateForSorting(sortedGames[j].Date)

		// First sort by date
		if !dateA.Equal(dateB) {
			return dateA.Before(dateB)
		}

		// If dates are the same, sort by time
		return parseTimeToMinutes(sortedGames[i].Time) < parseTimeToMinutes(sortedGames[j].Time)
	})

	// Get unique teams in the order they appear in teamURLs
	teamOrder := []string{"Varsity", "JV", "14U Gold", "14U White", "12U Blue", "10U Red", "10U Black"}
	teamSet := make(map[string]bool)
	for _, game := range allGames {
		teamSet[game.Team] = true
	}

	var teams []string
	for _, team := range teamOrder {
		if teamSet[team] {
			teams = append(teams, team)
		}
	}
	// Add any teams not in the order list (alphabetically)
	for team := range teamSet {
		found := false
		for _, t := range teams {
			if t == team {
				found = true
				break
			}
		}
		if !found {
			teams = append(teams, team)
		}
	}

	// Create a map of team names to colors
	teamColorMap := make(map[string]string)
	for _, game := range allGames {
		teamColorMap[game.Team] = game.Color
	}

	var html strings.Builder
	now := time.Now().UTC()

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lightning Game Schedule</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
            text-align: center;
        }
        .filter-buttons {
            text-align: center;
            margin: 20px 0;
        }
        .filter-btn {
            padding: 8px 16px;
            margin: 4px;
            border: none;
            cursor: pointer;
            border-radius: 4px;
            background-color: #999 !important;
            color: white !important;
            transition: all 0.2s;
        }
        .filter-btn:hover {
            background-color: #777 !important;
        }
        .filter-btn.active {
            /* Active styles set inline */
        }
        table {
            width: 100%;
            max-width: 1200px;
            margin: 0 auto;
            border-collapse: collapse;
            background-color: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th {
            background-color: #fbcb44;
            color: black;
            padding: 12px;
            text-align: left;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #ddd;
        }
        tr.month-end td {
            border-bottom: 3px solid #fbcb44;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .team-badge {
            display: inline-block;
            padding: 4px 8px;
            background-color: #2196F3;
            color: black;
            border-radius: 4px;
            font-size: 0.9em;
        }
        .home-away-badge {
            display: inline-block;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 0.8em;
            font-weight: bold;
            margin-left: 4px;
        }
        .home {
            background-color: #4CAF50;
            color: white;
        }
        .away {
            background-color: #FF9800;
            color: white;
        }
        .location-wrapper {
            position: relative;
            display: inline-block;
            cursor: help;
        }
        .location-abbr {
            text-decoration: underline;
            text-decoration-style: dotted;
            text-decoration-color: #999;
        }
        .location-tooltip {
            visibility: hidden;
            opacity: 0;
            position: absolute;
            z-index: 1000;
            background-color: #333;
            color: white;
            padding: 8px 12px;
            border-radius: 6px;
            font-size: 0.9em;
            white-space: nowrap;
            bottom: 125%;
            left: 50%;
            transform: translateX(-50%);
            transition: opacity 0.3s;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        .location-tooltip::after {
            content: "";
            position: absolute;
            top: 100%;
            left: 50%;
            margin-left: -5px;
            border-width: 5px;
            border-style: solid;
            border-color: #333 transparent transparent transparent;
        }
        .location-wrapper:hover .location-tooltip {
            visibility: visible;
            opacity: 1;
        }
        .location-wrapper.active .location-tooltip {
            visibility: visible;
            opacity: 1;
        }
        @media (max-width: 768px) {
            body {
                margin: 10px;
            }
            h1 {
                font-size: 1.5em;
            }
            .filter-buttons {
                margin: 15px 0;
            }
            .filter-btn {
                padding: 6px 10px;
                font-size: 0.85em;
                margin: 2px;
            }
            table {
                font-size: 0.7em;
                display: block;
                overflow-x: auto;
                -webkit-overflow-scrolling: touch;
                white-space: nowrap;
            }
            thead, tbody {
                display: table;
                width: 100%;
            }
            th, td {
                padding: 8px 4px;
                word-wrap: break-word;
                white-space: normal;
            }
            /* Make team column narrower */
            th:nth-child(1),
            td:nth-child(1) {
                min-width: 60px;
            }
            /* Time column (combined date+time) */
            th:nth-child(2),
            td:nth-child(2) {
                min-width: 120px;
            }
            /* Location needs more space */
            th:nth-child(3),
            td:nth-child(3) {
                min-width: 120px;
                max-width: 180px;
            }
            /* Jersey column */
            th:nth-child(4),
            td:nth-child(4) {
                min-width: 65px;
            }
            /* Opponent needs space */
            th:nth-child(5),
            td:nth-child(5) {
                min-width: 100px;
                max-width: 150px;
            }
            /* Score column */
            th:nth-child(6),
            td:nth-child(6) {
                min-width: 40px;
            }
            .team-badge {
                font-size: 0.85em;
                padding: 3px 5px;
            }
        }
        @media (max-width: 480px) {
            body {
                margin: 5px;
            }
            table {
                font-size: 0.65em;
            }
            th, td {
                padding: 6px 3px;
            }
            .team-badge {
                font-size: 0.8em;
                padding: 2px 4px;
            }
            .filter-btn {
                padding: 5px 8px;
                font-size: 0.8em;
            }
        }
    </style>
</head>
<body>
    <h1>⚡️ Lightning Game Schedule</h1>
    <p id="lastUpdated" style="text-align: center; color: #999; font-size: 0.75rem; margin: -10px 0 20px 0;" data-utc="` +
		now.Format(time.RFC3339) + `">Last updated on ` + now.Format("1/2/06") + ` at ` + now.Format("3:04PM") + ` UTC</p>
    <div class="filter-buttons">
`)

	// All Teams button
	activeClass := ""
	activeStyle := ""
	if filterTeam == "" {
		activeClass = "active"
		activeStyle = "background-color: #fbcb44 !important; color: black !important;"
	}
	linkHref := "./"
	if filterTeam != "" {
		linkHref = "../"
	}
	html.WriteString(fmt.Sprintf(`        <a href="%s" class="filter-btn %s" style="%s text-decoration: none; display: inline-block;">All Teams</a>
`, linkHref, activeClass, activeStyle))

	// Add filter buttons for each team
	for _, team := range teams {
		teamColor := teamColorMap[team]
		textColor := getTeamTextColor(teamColor)
		borderStyle := ""
		if teamColor == "#FFFFFF" {
			borderStyle = " border: 1px solid black;"
		}
		teamSlug := strings.ToLower(strings.ReplaceAll(team, " ", ""))
		activeClass := ""
		activeStyle := ""
		if filterTeam == team {
			activeClass = "active"
			activeStyle = fmt.Sprintf("background-color: %s !important; color: %s !important;%s", teamColor, textColor, borderStyle)
		}
		teamLink := teamSlug + "/"
		if filterTeam != "" {
			teamLink = "../" + teamSlug + "/"
		}
		html.WriteString(fmt.Sprintf(`        <a href="%s" class="filter-btn %s" style="%s text-decoration: none; display: inline-block;">%s</a>
`, teamLink, activeClass, activeStyle, team))
	}

	html.WriteString(`    </div>
    <table id="scheduleTable">
        <thead>
            <tr>
                <th>Team</th>
                <th>Time</th>
                <th>Location</th>
                <th>Jersey</th>
                <th>Opponent</th>
                <th>Score</th>
            </tr>
        </thead>
        <tbody>
`)

	// Add game rows
	for i, game := range sortedGames {
		// Determine if this is the last game of its month
		isMonthEnd := false
		if i < len(sortedGames)-1 {
			currentDate := parseDateForSorting(game.Date)
			nextDate := parseDateForSorting(sortedGames[i+1].Date)

			if currentDate.Month() != nextDate.Month() || currentDate.Year() != nextDate.Year() {
				isMonthEnd = true
			}
		} else {
			isMonthEnd = true
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

		// Get team color and text color
		teamColor := game.Color
		textColor := getTeamTextColor(teamColor)
		borderStyle := ""
		if teamColor == "#FFFFFF" {
			borderStyle = " border: 1px solid black;"
		}

		// Get location display
		locationDisplay := getLocationDisplay(game.Location)
		var locationHTML string

		if locationDisplay.Abbr == "" {
			// No abbreviation - show full location without tooltip
			locationHTML = locationDisplay.TooltipText
		} else if locationDisplay.CourtGym != "" {
			// Has abbreviation and court/gym info
			locationHTML = fmt.Sprintf(`<span class="location-wrapper"><span class="location-abbr">%s</span><span class="location-tooltip">%s</span></span> (%s)`,
				locationDisplay.Abbr, locationDisplay.TooltipText, locationDisplay.CourtGym)
		} else if locationDisplay.Abbr != locationDisplay.TooltipText {
			// Has abbreviation but no court/gym
			locationHTML = fmt.Sprintf(`<span class="location-wrapper"><span class="location-abbr">%s</span><span class="location-tooltip">%s</span></span>`,
				locationDisplay.Abbr, locationDisplay.TooltipText)
		} else {
			// Same value for both - no tooltip needed
			locationHTML = locationDisplay.Abbr
		}

		monthEndClass := ""
		if isMonthEnd {
			monthEndClass = " month-end"
		}

		opponent := game.Opponent
		if opponent == "" {
			opponent = "TBD"
		}
		score := game.Score
		if score == "" {
			score = "-"
		}

		html.WriteString(fmt.Sprintf(`            <tr class="game-row%s" data-team="%s">
                <td><span class="team-badge" style="background-color: %s; color: %s;%s">%s</span></td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
            </tr>
`, monthEndClass, game.Team, teamColor, textColor, borderStyle, game.Team, displayDateTime, locationHTML, jerseyText, opponent, score))
	}

	html.WriteString(`        </tbody>
    </table>
    <script>
        // Convert UTC timestamp to Central Time
        document.addEventListener('DOMContentLoaded', function() {
            const lastUpdatedEl = document.getElementById('lastUpdated');
            if (lastUpdatedEl) {
                const utcTime = lastUpdatedEl.getAttribute('data-utc');
                if (utcTime) {
                    try {
                        const date = new Date(utcTime);
                        // Format in Central Time (America/Chicago)
                        const options = {
                            timeZone: 'America/Chicago',
                            month: 'numeric',
                            day: 'numeric',
                            year: '2-digit',
                            hour: 'numeric',
                            minute: '2-digit',
                            hour12: true
                        };
                        const formatter = new Intl.DateTimeFormat('en-US', options);
                        const parts = formatter.formatToParts(date);

                        const month = parts.find(p => p.type === 'month').value;
                        const day = parts.find(p => p.type === 'day').value;
                        const year = parts.find(p => p.type === 'year').value;
                        const hour = parts.find(p => p.type === 'hour').value;
                        const minute = parts.find(p => p.type === 'minute').value;
                        const dayPeriod = parts.find(p => p.type === 'dayPeriod').value;

                        lastUpdatedEl.textContent = 'Last updated on ' + month + '/' + day + '/' + year +
                            ' at ' + hour + ':' + minute + dayPeriod;
                    } catch (e) {
                        // Keep the UTC fallback if conversion fails
                    }
                }
            }

            // Handle tooltip clicks on mobile devices
            const locationWrappers = document.querySelectorAll('.location-wrapper');

            locationWrappers.forEach(function(wrapper) {
                wrapper.addEventListener('click', function(e) {
                    e.stopPropagation();

                    // Close all other open tooltips
                    locationWrappers.forEach(function(otherWrapper) {
                        if (otherWrapper !== wrapper) {
                            otherWrapper.classList.remove('active');
                        }
                    });

                    // Toggle this tooltip
                    wrapper.classList.toggle('active');
                });
            });

            // Close tooltips when clicking outside
            document.addEventListener('click', function() {
                locationWrappers.forEach(function(wrapper) {
                    wrapper.classList.remove('active');
                });
            });
        });
    </script>
</body>
</html>
`)

	// Write to file
	err := os.WriteFile(outputFile, []byte(html.String()), 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("Generated %s\n", outputFile)
	return nil
}

func main() {
	fmt.Println("Starting schedule scraper...\n")

	var allGames []Game

	// Fetch games from team URLs (skip teams without URLs)
	for displayName, teamInfo := range teamURLs {
		if teamInfo.URL != "" {
			games, err := scrapeTeamSchedule(displayName, teamInfo.URL, teamInfo.HTMLName, teamInfo.Color)
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
		fmt.Println("\nNo games found. Please check the URLs and try again.")
		os.Exit(1)
	}

	fmt.Printf("\nTotal games found: %d\n", len(allGames))

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
	err = generateHTML(allGames, filepath.Join(distDir, "index.html"), "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
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
		err = generateHTML(allGames, filepath.Join(teamDir, "index.html"), team)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	fmt.Printf("\n✓ Done! Generated %s/index.html and individual team pages in %s/[team]/index.html.\n", outputDir, outputDir)
}
