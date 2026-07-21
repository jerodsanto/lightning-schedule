package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// Characterization tests for the current W/L determination logic. These pin
// existing behavior (including quirks like ties counting as losses) so any
// intentional change to the logic shows up as explicit test updates.

func TestParseGoogleSheetGamesScoreResult(t *testing.T) {
	AllTeams = []Team{{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}}
	defer func() { AllTeams = nil }()

	tests := []struct {
		name       string
		score      string
		wantScore  string
		wantResult string
	}{
		{"win", "45-30", "W 45-30", "W"},
		{"loss", "30-45", "L 30-45", "L"},
		{"tie counts as loss", "30-30", "L 30-30", "L"},
		{"whitespace around scores trimmed", "45 - 30", "W 45-30", "W"},
		{"empty score untouched", "", "", ""},
		{"bare dash untouched", "-", "-", ""},
		{"three-part score untouched", "45-30-2", "45-30-2", ""},
		{"non-numeric score untouched", "abc-def", "abc-def", ""},
		{"already-prefixed score untouched, no result", "W 45-30", "W 45-30", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvData := "Team,Date,Time,Location,Jersey,Opponent,Score\n" +
				fmt.Sprintf("Blue,10/18/2025,1:00 PM,GYM,Home,Rivals,%s\n", tt.score)
			games, err := parseGoogleSheetGames(strings.NewReader(csvData))
			if err != nil {
				t.Fatalf("parseGoogleSheetGames failed: %v", err)
			}
			if len(games) != 1 {
				t.Fatalf("got %d games, want 1", len(games))
			}
			if games[0].Score != tt.wantScore {
				t.Errorf("Score = %q, want %q", games[0].Score, tt.wantScore)
			}
			if games[0].Result != tt.wantResult {
				t.Errorf("Result = %q, want %q", games[0].Result, tt.wantResult)
			}
		})
	}
}

func tourneyPage(rows string) string {
	return `<html><body><table>
<tr><th>Saturday, October 18, 2025</th></tr>
` + rows + `
</table></body></html>`
}

func tourneyRow(gameNum, timeStr, visitor, visitorScore, homeScore, home string) string {
	return fmt.Sprintf(
		"<tr><td>%s</td><td>%s</td><td>Some Gym</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td></tr>",
		gameNum, timeStr, visitor, visitorScore, homeScore, home)
}

func scrapeTourneyRows(t *testing.T, rows string) []Game {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, tourneyPage(rows))
	}))
	defer server.Close()

	games, err := scrapeTeamSchedule("Blue", server.URL, "Lightning", "blue")
	if err != nil {
		t.Fatalf("scrapeTeamSchedule failed: %v", err)
	}
	return games
}

func TestScrapeTeamScheduleScoreResult(t *testing.T) {
	tests := []struct {
		name         string
		row          string
		wantScore    string
		wantResult   string
		wantHomeAway string
	}{
		{
			"win as visitor",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "45", "30", "Rivals"),
			"W 45-30", "W", "Away",
		},
		{
			"loss as visitor",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "30", "45", "Rivals"),
			"L 30-45", "L", "Away",
		},
		{
			"win as home",
			tourneyRow("1", "6:00 PM", "Rivals", "20", "50", "Lightning Blue"),
			"W 50-20", "W", "Home",
		},
		{
			"loss as home",
			tourneyRow("1", "6:00 PM", "Rivals", "50", "20", "Lightning Blue"),
			"L 20-50", "L", "Home",
		},
		{
			"tie counts as loss",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "25", "25", "Rivals"),
			"L 25-25", "L", "Away",
		},
		{
			"unplayed game (× markers) has no score or result",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "×", "×", "Rivals"),
			"", "", "Away",
		},
		{
			"empty scores have no score or result",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "", "", "Rivals"),
			"", "", "Away",
		},
		{
			"non-numeric score treated as zero",
			tourneyRow("1", "6:00 PM", "Lightning Blue", "F", "12", "Rivals"),
			"L F-12", "L", "Away",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			games := scrapeTourneyRows(t, tt.row)
			if len(games) != 1 {
				t.Fatalf("got %d games, want 1", len(games))
			}
			if games[0].Score != tt.wantScore {
				t.Errorf("Score = %q, want %q", games[0].Score, tt.wantScore)
			}
			if games[0].Result != tt.wantResult {
				t.Errorf("Result = %q, want %q", games[0].Result, tt.wantResult)
			}
			if games[0].HomeAway != tt.wantHomeAway {
				t.Errorf("HomeAway = %q, want %q", games[0].HomeAway, tt.wantHomeAway)
			}
		})
	}
}

func TestScrapeTeamScheduleSkipsOtherTeams(t *testing.T) {
	games := scrapeTourneyRows(t, tourneyRow("1", "6:00 PM", "Sharks", "45", "30", "Rivals"))
	if len(games) != 0 {
		t.Fatalf("got %d games, want 0 for a row without our team", len(games))
	}
}

func TestGenerateHTMLTeamRecord(t *testing.T) {
	setupGenerateHTMLTest()
	team := &Team{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}
	games := []Game{
		{Team: team, Date: "Saturday, October 18, 2025", Time: "1:00 PM", Opponent: "Rivals", Score: "W 45-30", Result: "W"},
		{Team: team, Date: "Saturday, October 25, 2025", Time: "1:00 PM", Opponent: "Sharks", Score: "W 40-20", Result: "W"},
		{Team: team, Date: "Saturday, November 1, 2025", Time: "1:00 PM", Opponent: "Hawks", Score: "L 25-25", Result: "L"},
		{Team: team, Date: "Saturday, November 8, 2025", Time: "1:00 PM", Opponent: "Wolves"},
	}
	out := t.TempDir() + "/index.html"
	if err := generateHTML(games, nil, out, team); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	html := readFile(t, out)
	if !strings.Contains(html, "[2-1]") {
		t.Error("expected team record [2-1] in output (unplayed games excluded)")
	}
}

func TestGenerateHTMLTeamRecordOmittedWithNoResults(t *testing.T) {
	setupGenerateHTMLTest()
	team := &Team{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}
	games := []Game{
		{Team: team, Date: "Saturday, October 18, 2025", Time: "1:00 PM", Opponent: "Rivals"},
	}
	out := t.TempDir() + "/index.html"
	if err := generateHTML(games, nil, out, team); err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}
	if strings.Contains(readFile(t, out), "[0-0]") {
		t.Error("record should be omitted when no games have results")
	}
}
