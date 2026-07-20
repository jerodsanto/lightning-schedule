package main

import (
	"strings"
	"testing"
	"testing/fstest"
)

const validConfigJSON = `{
  "name": "Test",
  "domain": "example.com",
  "sheetID": "abc123",
  "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"},
  "themeColor": "#ffffff",
  "icalProdID": "-//Test//Schedule//EN",
  "icalCalName": "Test Schedule"
}`

func testFS(config string) fstest.MapFS {
	return fstest.MapFS{
		"programs/test/config.json": &fstest.MapFile{Data: []byte(config)},
	}
}

func TestLoadProgramConfig(t *testing.T) {
	c, err := loadProgramConfig(testFS(validConfigJSON), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "Test" {
		t.Errorf("Name = %q, want %q", c.Name, "Test")
	}
	if c.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", c.Domain, "example.com")
	}
	want := "https://docs.google.com/spreadsheets/d/abc123/export?format=csv&gid=1"
	if got := c.csvURL("notes"); got != want {
		t.Errorf("csvURL(notes) = %q, want %q", got, want)
	}
}

func TestLoadProgramConfigUnknownProgram(t *testing.T) {
	_, err := loadProgramConfig(testFS(validConfigJSON), "nope")
	if err == nil {
		t.Fatal("expected error for unknown program")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error should list available programs, got: %v", err)
	}
}

func TestLoadProgramConfigMissingSheetID(t *testing.T) {
	_, err := loadProgramConfig(testFS(`{"name": "Test", "gids": {"schedule": "0", "notes": "1", "locations": "2", "teams": "3"}}`), "test")
	if err == nil {
		t.Fatal("expected error for missing sheetID")
	}
	if !strings.Contains(err.Error(), "sheetID") {
		t.Errorf("error should mention sheetID, got: %v", err)
	}
}

func TestLoadProgramConfigMissingGid(t *testing.T) {
	_, err := loadProgramConfig(testFS(`{"name": "Test", "sheetID": "abc", "gids": {"schedule": "0"}}`), "test")
	if err == nil {
		t.Fatal("expected error for missing gids")
	}
	if !strings.Contains(err.Error(), "gids.notes") {
		t.Errorf("error should mention gids.notes, got: %v", err)
	}
}

func TestLoadEmbeddedLightningConfig(t *testing.T) {
	c, err := loadProgramConfig(programsFS, "lightning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.SheetID != "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ" {
		t.Errorf("wrong sheetID: %q", c.SheetID)
	}
}
