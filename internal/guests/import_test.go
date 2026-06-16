package importsvc

import (
	"os"
	"strings"
	"testing"
)

func TestParseCSV_leaderFiles(t *testing.T) {
	files := []string{
		"../../scripts/test_leader1_guests.csv",
		"../../scripts/test_leader2_guests.csv",
		"../../scripts/test_leader3_guests.csv",
		"../../scripts/test_leader4_guests.csv",
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		rows, err := ParseFile(nil, "guests.csv", f)
		f.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(rows) != 25 {
			t.Fatalf("%s: want 25 rows, got %d", path, len(rows))
		}
		if strings.TrimSpace(rows[0].Name) == "" {
			t.Fatalf("%s: first row name empty", path)
		}
	}
}

func TestParseCSV_bomHeader(t *testing.T) {
	csv := "\xef\xbb\xbfname,phone_number,email\nTest User,+911234567890,test@example.com\n"
	rows, err := NewParser().ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parse bom csv: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Test User" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestParseFile_missingExtension(t *testing.T) {
	csv := "name,phone_number,email\nTest User,+911234567890,test@example.com\n"
	rows, err := ParseFile(nil, "document", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parse extensionless csv: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Test User" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
