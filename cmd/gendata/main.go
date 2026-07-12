// Command gendata generates bulk-import sample files for guests:
//
//   - sample_data/import_template.csv   – small CSV template (column reference)
//   - sample_data/import_template.xlsx  – small XLSX template (column reference)
//   - sample_data/guests_batch_1..3.xlsx – 350 dummy members each
//   - sample_data/guests_batch_1..3.csv  – same data as CSV
//
// The XLSX batches deliberately store most phone numbers as *numeric* cells so
// they exercise the scientific-notation import fix. Run from the repo root:
//
//	go run ./cmd/gendata
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/xuri/excelize/v2"
)

var headers = []string{"name", "phone_number", "email", "address", "department", "tag"}

var firstNames = []string{
	"Aarav", "Vivaan", "Aditya", "Vihaan", "Arjun", "Sai", "Reyansh", "Krishna", "Ishaan", "Rohan",
	"Ananya", "Diya", "Aadhya", "Saanvi", "Ira", "Myra", "Anika", "Navya", "Priya", "Riya",
	"Kabir", "Aryan", "Dev", "Karan", "Nikhil", "Rahul", "Siddharth", "Yash", "Aman", "Harsh",
	"Meera", "Kavya", "Pooja", "Neha", "Tanvi", "Sneha", "Isha", "Nisha", "Simran", "Divya",
}

var lastNames = []string{
	"Sharma", "Verma", "Gupta", "Iyer", "Nair", "Reddy", "Rao", "Patel", "Shah", "Mehta",
	"Singh", "Chopra", "Bose", "Ghosh", "Das", "Kapoor", "Malhotra", "Joshi", "Desai", "Menon",
	"Pillai", "Naidu", "Kulkarni", "Deshpande", "Bhat", "Shetty", "Chauhan", "Yadav", "Mishra", "Tiwari",
}

var cities = []string{
	"12 Connaught Place, New Delhi",
	"45 Hill Road, Bandra West, Mumbai",
	"7 Brigade Road, Bengaluru",
	"22 Park Street, Kolkata",
	"90 Anna Salai, Chennai",
	"3 Banjara Hills, Hyderabad",
	"56 FC Road, Pune",
	"18 MG Road, Gurugram",
	"101 Civil Lines, Jaipur",
	"64 Ashram Road, Ahmedabad",
}

var departments = []string{
	"Delhi University", "Mumbai University", "IIT Bombay", "IISc Bengaluru",
	"Jadavpur University", "Anna University", "University of Hyderabad",
	"Savitribai Phule Pune University", "BITS Pilani", "Gujarat University",
}

// tagCycle spreads guests across categories with a realistic bias toward
// invitees, and includes blanks to verify the "blank => invitee" default.
var tagCycle = []string{
	"vip", "vvip", "volunteer", "invitee", "organiser", "awardee", "core team", "media",
	"invitee", "", "invitee", "volunteer", "invitee", "media", "invitee", "vip",
	"invitee", "", "core team", "invitee",
}

type member struct {
	name       string
	phone      string
	phoneIsNum bool
	email      string
	address    string
	department string
	tag        string
}

func buildMember(globalIdx int) member {
	first := firstNames[globalIdx%len(firstNames)]
	last := lastNames[(globalIdx/len(firstNames))%len(lastNames)]
	digits := 9000000000 + globalIdx // unique 10-digit number starting with 9
	// ~70% numeric cells (to exercise the sci-notation fix), rest E.164 text.
	numeric := globalIdx%10 < 7
	phone := strconv.Itoa(digits)
	if !numeric {
		phone = "+91" + phone
	}
	return member{
		name:       fmt.Sprintf("%s %s", first, last),
		phone:      phone,
		phoneIsNum: numeric,
		email:      fmt.Sprintf("%s.%s.%d@example.com", lower(first), lower(last), globalIdx),
		address:    cities[globalIdx%len(cities)],
		department: departments[globalIdx%len(departments)],
		tag:        tagCycle[globalIdx%len(tagCycle)],
	}
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func (m member) row() []string {
	return []string{m.name, m.phone, m.email, m.address, m.department, m.tag}
}

func main() {
	outDir := "sample_data"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	// Templates: a handful of example rows covering every tag + a blank one.
	template := []member{
		{name: "Aarav Sharma", phone: "9810000001", phoneIsNum: true, email: "aarav.sharma@example.com", address: "12 Connaught Place, New Delhi", department: "Delhi University", tag: "vip"},
		{name: "Priya Verma", phone: "+919810000002", email: "priya.verma@example.com", address: "45 Hill Road, Mumbai", department: "Mumbai University", tag: "vvip"},
		{name: "Rahul Nair", phone: "9810000003", phoneIsNum: true, email: "rahul.nair@example.com", address: "7 Brigade Road, Bengaluru", department: "IISc Bengaluru", tag: "media"},
		{name: "Sneha Rao", phone: "9810000004", phoneIsNum: true, email: "sneha.rao@example.com", address: "22 Park Street, Kolkata", department: "Jadavpur University", tag: "core team"},
		{name: "Dev Patel", phone: "9810000005", phoneIsNum: true, email: "dev.patel@example.com", address: "64 Ashram Road, Ahmedabad", department: "Gujarat University", tag: ""},
	}
	if err := writeCSV(filepath.Join(outDir, "import_template.csv"), template); err != nil {
		log.Fatalf("template csv: %v", err)
	}
	if err := writeXLSX(filepath.Join(outDir, "import_template.xlsx"), template); err != nil {
		log.Fatalf("template xlsx: %v", err)
	}
	fmt.Println("Wrote sample_data/import_template.csv and import_template.xlsx")

	// Three batches of 350 dummy members, in both CSV and XLSX.
	const batches = 3
	const perBatch = 350
	global := 0
	for b := 1; b <= batches; b++ {
		members := make([]member, 0, perBatch)
		for i := 0; i < perBatch; i++ {
			members = append(members, buildMember(global))
			global++
		}
		csvPath := filepath.Join(outDir, fmt.Sprintf("guests_batch_%d.csv", b))
		xlsxPath := filepath.Join(outDir, fmt.Sprintf("guests_batch_%d.xlsx", b))
		if err := writeCSV(csvPath, members); err != nil {
			log.Fatalf("batch %d csv: %v", b, err)
		}
		if err := writeXLSX(xlsxPath, members); err != nil {
			log.Fatalf("batch %d xlsx: %v", b, err)
		}
		fmt.Printf("Wrote %s and %s (%d members)\n", csvPath, xlsxPath, perBatch)
	}

	fmt.Printf("Done. Generated templates + %d batches of %d members.\n", batches, perBatch)
}

func writeCSV(path string, members []member) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return err
	}
	for _, m := range members {
		if err := w.Write(m.row()); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeXLSX(path string, members []member) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	// Header row.
	for c, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		if err := f.SetCellStr(sheet, cell, h); err != nil {
			return err
		}
	}
	// Data rows (row index starts at 2).
	for r, m := range members {
		rowNum := r + 2
		values := m.row()
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			// Column index 1 (0-based) is phone_number; store numeric phones as
			// real numbers so the importer's scientific-notation fix is tested.
			if c == 1 && m.phoneIsNum {
				n, convErr := strconv.ParseInt(v, 10, 64)
				if convErr == nil {
					if err := f.SetCellInt(sheet, cell, n); err != nil {
						return err
					}
					continue
				}
			}
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	return f.SaveAs(path)
}
