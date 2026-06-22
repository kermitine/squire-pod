package productivity

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestMatchStandingsVoiceCommand(t *testing.T) {
	tests := []struct {
		text string
		kind string
		ok   bool
	}{
		{"show me the eastern conference standings", standingsNBAEast, true},
		{"show the eastern standings", standingsNBAEast, true},
		{"what are the NBA western conference rankings", standingsNBAWest, true},
		{"show the NBA standings", standingsNBAAll, true},
		{"show Formula 1 driver standings", standingsF1Drivers, true},
		{"F1 constructor leaderboard", standingsF1Constructors, true},
		{"constructor standings", standingsF1Constructors, true},
		{"help me understand NBA teams", "", false},
		{"how is Ferrari doing", "", false},
	}
	for _, tt := range tests {
		kind, ok := MatchStandingsVoiceCommand(tt.text)
		if kind != tt.kind || ok != tt.ok {
			t.Errorf("MatchStandingsVoiceCommand(%q) = %q, %v; want %q, %v", tt.text, kind, ok, tt.kind, tt.ok)
		}
	}
}

func TestNBAStandingsPagesIncludeFullConference(t *testing.T) {
	response := espnStandingsResponse{Children: []espnStandingsGroup{standingsNBAGroup("Eastern Conference", "E", 15), standingsNBAGroup("Western Conference", "W", 15)}}
	pages, err := nbaStandingsPages(context.Background(), response, standingsNBAEast)
	if err != nil {
		t.Fatalf("nbaStandingsPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("NBA East pages = %d, want 3", len(pages))
	}
	speech := standingsPageSpeech(pages)
	for position := 1; position <= 15; position++ {
		if !strings.Contains(speech, fmt.Sprintf("Eastern Team %d", position)) {
			t.Errorf("speech omitted Eastern Team %d", position)
		}
	}
	if len(pages[0].FaceData) == 0 {
		t.Fatal("NBA standings face data is empty")
	}

	allPages, err := nbaStandingsPages(context.Background(), response, standingsNBAAll)
	if err != nil || len(allPages) != 6 {
		t.Fatalf("all NBA pages = %d, %v; want 6", len(allPages), err)
	}
}

func TestF1StandingsPagesIncludeEveryDriverAndConstructor(t *testing.T) {
	response := espnStandingsResponse{Children: []espnStandingsGroup{standingsF1Group("Driver Standings", false, 22), standingsF1Group("Constructor Standings", true, 11)}}
	driverPages, err := f1StandingsPages(response, standingsF1Drivers)
	if err != nil || len(driverPages) != 5 {
		t.Fatalf("driver pages = %d, %v; want 5", len(driverPages), err)
	}
	constructorPages, err := f1StandingsPages(response, standingsF1Constructors)
	if err != nil || len(constructorPages) != 3 {
		t.Fatalf("constructor pages = %d, %v; want 3", len(constructorPages), err)
	}
	if !strings.Contains(standingsPageSpeech(driverPages), "Driver 22") {
		t.Fatal("driver speech omitted the final driver")
	}
	if !strings.Contains(standingsPageSpeech(constructorPages), "Constructor 11") {
		t.Fatal("constructor speech omitted the final constructor")
	}
}

func TestF1OrdinalLong(t *testing.T) {
	for value, want := range map[int]string{1: "first", 10: "tenth", 11: "11th", 12: "12th", 13: "13th", 21: "21st", 22: "22nd"} {
		if got := f1OrdinalLong(value); got != want {
			t.Errorf("f1OrdinalLong(%d) = %q, want %q", value, got, want)
		}
	}
}

func standingsNBAGroup(name, prefix string, count int) espnStandingsGroup {
	group := espnStandingsGroup{Name: name}
	for position := 1; position <= count; position++ {
		entry := espnStandingsEntry{}
		entry.Team.DisplayName = fmt.Sprintf("%s Team %d", name[:7], position)
		entry.Team.Abbreviation = fmt.Sprintf("%s%02d", prefix, position)
		entry.Stats = []espnStandingStat{
			{Name: "playoffSeed", DisplayValue: fmt.Sprint(position)},
			{Name: "wins", DisplayValue: fmt.Sprint(60 - position)},
			{Name: "losses", DisplayValue: fmt.Sprint(20 + position)},
		}
		group.Standings.Entries = append(group.Standings.Entries, entry)
	}
	return group
}

func standingsF1Group(name string, constructors bool, count int) espnStandingsGroup {
	group := espnStandingsGroup{Name: name}
	for position := 1; position <= count; position++ {
		entry := espnStandingsEntry{}
		if constructors {
			entry.Team.DisplayName = fmt.Sprintf("Constructor %d", position)
			entry.Team.ShortDisplayName = fmt.Sprintf("C%d", position)
			entry.Team.Color = "FF0000"
		} else {
			entry.Athlete.ID = fmt.Sprint(1000 + position)
			entry.Athlete.DisplayName = fmt.Sprintf("Driver %d", position)
		}
		entry.Stats = []espnStandingStat{{Name: "rank", DisplayValue: fmt.Sprint(position)}, {Name: "championshipPts", DisplayValue: fmt.Sprint(300 - position)}}
		group.Standings.Entries = append(group.Standings.Entries, entry)
	}
	return group
}

func standingsPageSpeech(pages []TaskPage) string {
	parts := make([]string, len(pages))
	for index, page := range pages {
		parts[index] = page.Speech
	}
	return strings.Join(parts, " ")
}
