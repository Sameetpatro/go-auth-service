package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"

	importsvc "github.com/sameetpatro/go-qr-auth/internal/guests"
	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
)

func (s *GuestService) Create(ctx context.Context, req dto.CreateGuestRequest, userID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	guestUUID := uuid.New()
	token, imageURL, err := s.qr.GenerateGuestQR(guestUUID)
	if err != nil {
		return nil, err
	}

	meta, err := json.Marshal(req.Metadata)
	if err != nil {
		meta = []byte(`{}`)
	}

	guest := &models.Guest{
		UUID:        guestUUID,
		Name:        req.Name,
		PhoneNumber: req.PhoneNumber,
		Email:       req.Email,
		QRToken:     token,
		QRImageURL:  &imageURL,
		Metadata:    meta,
	}
	if err := s.guests.Create(ctx, guest); err != nil {
		return nil, err
	}

	phone := ""
	if req.PhoneNumber != nil {
		phone = *req.PhoneNumber
	}
	go func() {
		_ = s.notifications.SendGuestInvitation(context.Background(), req.Name, phone,
			s.event.Name, s.event.Date, s.event.Location, imageURL)
	}()

	s.audit.Log(ctx, &userID, &role, models.AuditCreateGuest,
		fmt.Sprintf("Created guest %s", req.Name), ip)

	resp := toGuestResponse(&models.GuestWithChecker{Guest: *guest})
	if s.ws != nil {
		s.ws.BroadcastGuestAdded(resp)
		s.ws.BroadcastDashboardUpdated()
	}
	return &resp, nil
}

func (s *GuestService) Update(ctx context.Context, id int64, req dto.UpdateGuestRequest, userID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	existing, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, sql.ErrNoRows
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.PhoneNumber != nil {
		existing.PhoneNumber = req.PhoneNumber
	}
	if req.Email != nil {
		existing.Email = req.Email
	}
	if req.Metadata != nil {
		meta, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, err
		}
		existing.Metadata = meta
	}

	if err := s.guests.Update(ctx, &existing.Guest); err != nil {
		return nil, err
	}

	s.audit.Log(ctx, &userID, &role, models.AuditUpdateGuest,
		fmt.Sprintf("Updated guest ID %d", id), ip)

	updated, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toGuestResponse(updated)
	return &resp, nil
}

func (s *GuestService) Delete(ctx context.Context, id int64, userID int64, role models.UserRole, ip string) error {
	if err := s.guests.Delete(ctx, id); err != nil {
		return err
	}
	s.audit.Log(ctx, &userID, &role, models.AuditDeleteGuest,
		fmt.Sprintf("Deleted guest ID %d", id), ip)
	if s.ws != nil {
		s.ws.BroadcastDashboardUpdated()
	}
	return nil
}

func (s *GuestService) GetByID(ctx context.Context, id int64) (*dto.GuestResponse, error) {
	guest, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if guest == nil {
		return nil, sql.ErrNoRows
	}
	resp := toGuestResponse(guest)
	return &resp, nil
}

func (s *GuestService) Search(ctx context.Context, query string, page, perPage int) (*dto.PaginatedGuestsResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	guests, total, err := s.guests.Search(ctx, query, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]dto.GuestResponse, len(guests))
	for i, g := range guests {
		data[i] = toGuestResponse(&g)
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return &dto.PaginatedGuestsResponse{
		Data: data,
		Meta: dto.PaginationMeta{
			Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
		},
	}, nil
}

func (s *GuestService) List(ctx context.Context, page, perPage int) (*dto.PaginatedGuestsResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	guests, total, err := s.guests.List(ctx, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]dto.GuestResponse, len(guests))
	for i, g := range guests {
		data[i] = toGuestResponse(&g)
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return &dto.PaginatedGuestsResponse{
		Data: data,
		Meta: dto.PaginationMeta{
			Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
		},
	}, nil
}

func (s *GuestService) SearchForVerification(ctx context.Context, query string) ([]dto.GuestSearchResponse, error) {
	guests, _, err := s.guests.Search(ctx, query, 50, 0)
	if err != nil {
		return nil, err
	}

	result := make([]dto.GuestSearchResponse, len(guests))
	for i, g := range guests {
		entryStatus := "PENDING"
		if g.IsCheckedIn {
			entryStatus = "CHECKED_IN"
		}
		qrStatus := "VALID"
		result[i] = dto.GuestSearchResponse{
			Guest:       toGuestResponse(&g),
			QRStatus:    qrStatus,
			EntryStatus: entryStatus,
		}
	}
	return result, nil
}

func (s *GuestService) Import(ctx context.Context, filename string, r io.Reader, userID int64, role models.UserRole, ip string) (*dto.ImportResult, error) {
	rows, err := importsvc.ParseFile(ctx, filename, r)
	if err != nil {
		return nil, err
	}

	result := &dto.ImportResult{TotalRows: len(rows)}
	for i, row := range rows {
		req := dto.CreateGuestRequest{
			Name:        row.Name,
			Metadata:    importsvc.RowToMetadata(row),
		}
		if row.PhoneNumber != "" {
			req.PhoneNumber = &row.PhoneNumber
		}
		if row.Email != "" {
			req.Email = &row.Email
		}
		if _, err := s.Create(ctx, req, userID, role, ip); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", i+2, err))
		} else {
			result.Imported++
		}
	}

	s.audit.Log(ctx, &userID, &role, models.AuditImportGuests,
		fmt.Sprintf("Imported %d/%d guests from %s", result.Imported, result.TotalRows, filename), ip)

	if s.ws != nil {
		s.ws.BroadcastDashboardUpdated()
	}
	return result, nil
}

func toGuestResponse(g *models.GuestWithChecker) dto.GuestResponse {
	var metadata map[string]interface{}
	if len(g.Metadata) > 0 {
		_ = json.Unmarshal(g.Metadata, &metadata)
	}
	return dto.GuestResponse{
		ID:               g.ID,
		UUID:             g.UUID,
		Name:             g.Name,
		PhoneNumber:      g.PhoneNumber,
		Email:            g.Email,
		QRImageURL:       g.QRImageURL,
		IsCheckedIn:      g.IsCheckedIn,
		CheckedInAt:      g.CheckedInAt,
		CheckedInBy:      g.CheckedInBy,
		CheckedInByEmail: g.CheckedInByEmail,
		Metadata:         metadata,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}
}
