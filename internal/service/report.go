package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/sameetpatro/go-qr-auth/internal/audit"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
)

type ReportService struct {
	guests    *repository.GuestRepository
	analytics *repository.AnalyticsRepository
	audit     *audit.Service
}

func NewReportService(
	guests *repository.GuestRepository,
	analytics *repository.AnalyticsRepository,
	audit *audit.Service,
) *ReportService {
	return &ReportService{guests: guests, analytics: analytics, audit: audit}
}

func (s *ReportService) ExportCSV(ctx context.Context, userID int64, role models.UserRole, ip string) ([]byte, error) {
	var guests []models.GuestWithChecker
	var err error
	if role == models.RoleLeader {
		guests, _, err = s.guests.ListByCreator(ctx, userID, 100000, 0)
	} else {
		guests, _, err = s.guests.List(ctx, 100000, 0)
	}
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "UUID", "Name", "Phone", "Email", "Checked In", "Checked In At", "Checked In By"})

	for _, g := range guests {
		phone, email := "", ""
		if g.PhoneNumber != nil {
			phone = *g.PhoneNumber
		}
		if g.Email != nil {
			email = *g.Email
		}
		checkedInAt := ""
		if g.CheckedInAt != nil {
			checkedInAt = g.CheckedInAt.Format(time.RFC3339)
		}
		checkedBy := ""
		if g.CheckedInByEmail != nil {
			checkedBy = *g.CheckedInByEmail
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", g.ID),
			g.UUID.String(),
			g.Name, phone, email,
			fmt.Sprintf("%t", g.IsCheckedIn),
			checkedInAt, checkedBy,
		})
	}
	w.Flush()

	s.audit.Log(ctx, &userID, &role, models.AuditExportReport, "Exported CSV report", ip)
	return buf.Bytes(), w.Error()
}

func (s *ReportService) ExportBackupCSV(ctx context.Context, userID int64, role models.UserRole, ip string) ([]byte, error) {
	if role != models.RoleMaster {
		return nil, fmt.Errorf("forbidden")
	}

	guests, err := s.guests.ListRegistry(ctx, nil)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"ID", "UUID", "Name", "Phone", "Email", "Address", "Department",
		"Invited By", "Invited By Email", "Checked In", "Checked In At",
		"Checked In By", "Gate", "Created At",
	})

	for _, row := range guests {
		g := row.GuestWithChecker
		phone, email, address, department := "", "", "", ""
		if g.PhoneNumber != nil {
			phone = *g.PhoneNumber
		}
		if g.Email != nil {
			email = *g.Email
		}
		meta := map[string]interface{}{}
		if len(g.Metadata) > 0 {
			_ = json.Unmarshal(g.Metadata, &meta)
		}
		if v, ok := meta["address"].(string); ok {
			address = v
		}
		if v, ok := meta["department"].(string); ok {
			department = v
		} else if v, ok := meta["college"].(string); ok {
			department = v
		}
		invitedBy := ""
		if g.CreatedBy != nil {
			invitedBy = fmt.Sprintf("%d", *g.CreatedBy)
		}
		invitedByEmail := ""
		if g.CreatedByEmail != nil {
			invitedByEmail = *g.CreatedByEmail
		}
		checkedInAt := ""
		if g.CheckedInAt != nil {
			checkedInAt = g.CheckedInAt.Format(time.RFC3339)
		}
		checkedBy := ""
		if g.CheckedInByEmail != nil {
			checkedBy = *g.CheckedInByEmail
		}
		gate := ""
		if row.CheckInGate != nil {
			gate = *row.CheckInGate
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", g.ID),
			g.UUID.String(),
			g.Name, phone, email, address, department,
			invitedBy, invitedByEmail,
			fmt.Sprintf("%t", g.IsCheckedIn),
			checkedInAt, checkedBy, gate,
			g.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()

	s.audit.Log(ctx, &userID, &role, models.AuditExportReport, "Exported backup CSV", ip)
	return buf.Bytes(), w.Error()
}

func (s *ReportService) ExportPDF(ctx context.Context, userID int64, role models.UserRole, ip string) ([]byte, error) {
	total, _ := s.analytics.TotalGuests(ctx)
	checkedIn, _ := s.analytics.TotalCheckedIn(ctx)
	today, _ := s.analytics.TodayEntries(ctx)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Event Entry Report")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 8, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(10)
	pdf.Cell(40, 8, fmt.Sprintf("Total Guests: %d", total))
	pdf.Ln(8)
	pdf.Cell(40, 8, fmt.Sprintf("Total Checked In: %d", checkedIn))
	pdf.Ln(8)
	pdf.Cell(40, 8, fmt.Sprintf("Today's Entries: %d", today))
	pdf.Ln(8)
	pending := total - checkedIn
	pdf.Cell(40, 8, fmt.Sprintf("Pending: %d", pending))
	pdf.Ln(8)
	var pct float64
	if total > 0 {
		pct = float64(checkedIn) / float64(total) * 100
	}
	pdf.Cell(40, 8, fmt.Sprintf("Check-in Rate: %.1f%%", pct))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	s.audit.Log(ctx, &userID, &role, models.AuditExportReport, "Exported PDF report", ip)
	return buf.Bytes(), nil
}
