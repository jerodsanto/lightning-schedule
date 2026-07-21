package main

import (
	"strings"
	"testing"
)

func TestParseGoogleSheetGamesRoleColumn(t *testing.T) {
	AllTeams = []Team{{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}}
	defer func() { AllTeams = nil }()

	tests := []struct {
		name         string
		role         string
		wantHomeAway string
	}{
		{"home", "Home", "Home"},
		{"away", "Away", "Away"},
		{"tbd", "TBD", ""},
		{"empty", "", ""},
		{"light jersey means home", "Light", "Home"},
		{"dark jersey means away", "Dark", "Away"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvData := "Team,Date,Time,Location,Role,Opponent,Score\n" +
				"Blue,10/18/2025,1:00 PM,GYM," + tt.role + ",Rivals,\n"
			games, err := parseGoogleSheetGames(strings.NewReader(csvData))
			if err != nil {
				t.Fatalf("parseGoogleSheetGames failed: %v", err)
			}
			if len(games) != 1 {
				t.Fatalf("got %d games, want 1", len(games))
			}
			if games[0].HomeAway != tt.wantHomeAway {
				t.Errorf("HomeAway = %q, want %q", games[0].HomeAway, tt.wantHomeAway)
			}
		})
	}
}

func TestParseGoogleSheetGamesJerseyColumnFallback(t *testing.T) {
	AllTeams = []Team{{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}}
	defer func() { AllTeams = nil }()

	csvData := "Team,Date,Time,Location,Jersey,Opponent,Score\n" +
		"Blue,10/18/2025,1:00 PM,GYM,Dark,Rivals,\n"
	games, err := parseGoogleSheetGames(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parseGoogleSheetGames failed: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("got %d games, want 1", len(games))
	}
	if games[0].HomeAway != "Away" {
		t.Errorf("HomeAway = %q, want %q via legacy Jersey column", games[0].HomeAway, "Away")
	}
}

func TestFormatRole(t *testing.T) {
	tests := []struct {
		sport    string
		homeAway string
		style    string
		want     string
	}{
		{"basketball", "Home", "html", "⬜️"},
		{"basketball", "Away", "html", "⬛️"},
		{"basketball", "", "html", "TBD"},
		{"basketball", "Home", "cal", "Home (Light)"},
		{"basketball", "Away", "cal", "Away (Dark)"},
		{"basketball", "", "cal", "TBD"},
		{"volleyball", "Home", "html", "Home"},
		{"volleyball", "Away", "html", "Away"},
		{"volleyball", "", "html", "TBD"},
		{"volleyball", "Home", "cal", "Home"},
		{"volleyball", "Away", "cal", "Away"},
		{"volleyball", "", "cal", "TBD"},
	}

	for _, tt := range tests {
		t.Run(tt.sport+"/"+tt.style+"/"+tt.homeAway, func(t *testing.T) {
			cfg = &ProgramConfig{Sport: tt.sport}
			got := formatRole(&Game{HomeAway: tt.homeAway}, tt.style)
			if got != tt.want {
				t.Errorf("formatRole = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateHTMLRoleColumnBySport(t *testing.T) {
	team := &Team{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}
	games := []Game{{
		Team:     team,
		Date:     "Saturday, October 18, 2025",
		Time:     "1:00 PM",
		Opponent: "Rivals",
		HomeAway: "Away",
	}}

	tests := []struct {
		sport      string
		wantHeader string
		wantCell   string
	}{
		{"basketball", "<th>Jersey</th>", "⬛️"},
		{"volleyball", "<th>Role</th>", ">Away<"},
	}

	for _, tt := range tests {
		t.Run(tt.sport, func(t *testing.T) {
			setupGenerateHTMLTest()
			cfg.Sport = tt.sport
			out := t.TempDir() + "/index.html"
			if err := generateHTML(games, nil, out, nil); err != nil {
				t.Fatalf("generateHTML failed: %v", err)
			}
			html := readFile(t, out)
			if !strings.Contains(html, tt.wantHeader) {
				t.Errorf("expected column header %q in %s output", tt.wantHeader, tt.sport)
			}
			if !strings.Contains(html, tt.wantCell) {
				t.Errorf("expected cell content %q in %s output", tt.wantCell, tt.sport)
			}
		})
	}
}

func TestGenerateICalendarRoleLabelBySport(t *testing.T) {
	team := &Team{Name: "Blue", Slug: "blue", CssClass: "blue", Order: 1}
	games := []Game{{
		Team:     team,
		Date:     "Saturday, October 18, 2025",
		Time:     "1:00 PM",
		Opponent: "Rivals",
		HomeAway: "Away",
	}}

	tests := []struct {
		sport string
		want  string
	}{
		{"basketball", "Jersey: Away (Dark)"},
		{"volleyball", "Role: Away"},
	}

	for _, tt := range tests {
		t.Run(tt.sport, func(t *testing.T) {
			setupGenerateHTMLTest()
			cfg.Sport = tt.sport
			out := t.TempDir() + "/schedule.ics"
			if err := generateICalendar(games, nil, out, nil); err != nil {
				t.Fatalf("generateICalendar failed: %v", err)
			}
			if !strings.Contains(readFile(t, out), tt.want) {
				t.Errorf("expected %q in %s iCal output", tt.want, tt.sport)
			}
		})
	}
}
