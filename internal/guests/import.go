package importsvc

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

type GuestRow struct {
	Name        string
	PhoneNumber string
	Email       string
	Address     string
	College     string
	Extra       map[string]string
}

var coreFields = map[string]bool{
	"name": true, "phone_number": true, "email": true, "address": true, "college": true,
}

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) ParseCSV(r io.Reader) ([]GuestRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("csv must have header and at least one row")
	}
	return p.parseRecords(records[0], records[1:])
}

func (p *Parser) ParseXLSX(r io.Reader) ([]GuestRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse xlsx: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("xlsx must have header and at least one row")
	}
	return p.parseRecords(rows[0], rows[1:])
}

func (p *Parser) parseRecords(headers []string, records [][]string) ([]GuestRow, error) {
	colMap := make(map[int]string)
	for i, h := range headers {
		colMap[i] = normalizeField(strings.TrimSpace(h))
	}

	var guests []GuestRow
	for rowIdx, record := range records {
		if isEmptyRow(record) {
			continue
		}
		row := GuestRow{Extra: make(map[string]string)}
		for i, val := range record {
			field, ok := colMap[i]
			if !ok {
				continue
			}
			val = strings.TrimSpace(val)
			switch field {
			case "name":
				row.Name = val
			case "phone_number", "phone":
				row.PhoneNumber = val
			case "email":
				row.Email = val
			case "address":
				row.Address = val
			case "college":
				row.College = val
			default:
				if val != "" {
					row.Extra[field] = val
				}
			}
		}
		if row.Name == "" {
			return nil, fmt.Errorf("row %d: name is required", rowIdx+2)
		}
		guests = append(guests, row)
	}
	return guests, nil
}

func normalizeField(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.ReplaceAll(h, " ", "_")
	switch h {
	case "phone", "phonenumber", "phone_no", "mobile":
		return "phone_number"
	default:
		return h
	}
}

func isEmptyRow(record []string) bool {
	for _, v := range record {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func RowToMetadata(row GuestRow) map[string]interface{} {
	meta := make(map[string]interface{})
	for k, v := range row.Extra {
		meta[k] = v
	}
	return meta
}

func ParseFile(ctx context.Context, filename string, r io.Reader) ([]GuestRow, error) {
	parser := NewParser()
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return parser.ParseCSV(r)
	case strings.HasSuffix(lower, ".xlsx"), strings.HasSuffix(lower, ".xls"):
		return parser.ParseXLSX(r)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", filename)
	}
}
