package service

import (
	"context"
	"fmt"

	"github.com/sameetpatro/go-qr-auth/internal/audit"
	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
)

type ScanService struct {
	guests *repository.GuestRepository
	scans  *repository.ScanRepository
	audit  *audit.Service
	ws     ScanWebSocketBroadcaster
}

type ScanWebSocketBroadcaster interface {
	BroadcastGuestCheckedIn(data interface{})
	BroadcastDashboardUpdated()
	BroadcastInsightsUpdated()
}

func NewScanService(
	guests *repository.GuestRepository,
	scans *repository.ScanRepository,
	audit *audit.Service,
	ws ScanWebSocketBroadcaster,
) *ScanService {
	return &ScanService{guests: guests, scans: scans, audit: audit, ws: ws}
}

func (s *ScanService) Scan(ctx context.Context, req dto.ScanRequest, userID int64, role models.UserRole, ip string) (*dto.ScanResponse, error) {
	if role == models.RoleMaster {
		return nil, ErrForbiddenAction
	}
	guest, result, err := s.guests.CheckIn(ctx, req.QRToken, userID)
	if err != nil {
		return nil, err
	}

	var guestID *int64
	if guest != nil {
		guestID = &guest.ID
	}
	token := req.QRToken
	attempt := &models.ScanAttempt{
		GuestID:   guestID,
		QRToken:   &token,
		UserID:    userID,
		Result:    string(result),
		GateName:  req.GateName,
		IPAddress: &ip,
	}
	_ = s.scans.Create(ctx, attempt)

	s.audit.Log(ctx, &userID, &role, models.AuditScan,
		fmt.Sprintf("Scan result: %s for token", result), ip)

	resp := &dto.ScanResponse{Result: string(result)}

	switch result {
	case models.ScanResultEntryAllowed:
		g := toGuestResponse(guest)
		resp.Guest = &g
		resp.CheckedInAt = guest.CheckedInAt
		resp.CheckedInBy = guest.CheckedInBy
		resp.CheckedInByEmail = guest.CheckedInByEmail
		resp.Message = "Entry allowed. Welcome!"
		if s.ws != nil {
			s.ws.BroadcastGuestCheckedIn(g)
			s.ws.BroadcastDashboardUpdated()
			s.ws.BroadcastInsightsUpdated()
		}
	case models.ScanResultAlreadyEntered:
		g := toGuestResponse(guest)
		resp.Guest = &g
		resp.CheckedInAt = guest.CheckedInAt
		resp.CheckedInBy = guest.CheckedInBy
		resp.CheckedInByEmail = guest.CheckedInByEmail
		resp.Message = "Guest has already entered."
	case models.ScanResultEntryDenied:
		resp.Message = "Invalid QR code. Entry denied."
	}

	return resp, nil
}
