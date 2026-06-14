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
	"github.com/sameetpatro/go-qr-auth/internal/notifications"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
)

func (s *GuestService) Create(ctx context.Context, req dto.CreateGuestRequest, userID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	ownerID, err := s.resolveGuestOwner(ctx, userID, role, req.LeaderID)
	if err != nil {
		return nil, err
	}
	return s.createOwnedGuest(ctx, req, ownerID, userID, role, ip)
}

func (s *GuestService) resolveGuestOwner(ctx context.Context, userID int64, role models.UserRole, leaderID *int64) (int64, error) {
	switch role {
	case models.RoleLeader:
		return userID, nil
	case models.RoleMaster:
		if leaderID == nil || *leaderID <= 0 {
			return 0, fmt.Errorf("leader_id is required when creating guests as master")
		}
		user, err := s.users.FindByID(ctx, *leaderID)
		if err != nil {
			return 0, err
		}
		if user == nil || user.Role != models.RoleLeader {
			return 0, fmt.Errorf("invalid leader_id")
		}
		if !user.IsActive {
			return 0, fmt.Errorf("leader is disabled")
		}
		return *leaderID, nil
	default:
		return 0, ErrForbiddenAction
	}
}

func (s *GuestService) createOwnedGuest(ctx context.Context, req dto.CreateGuestRequest, ownerID int64, actorID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	meta := qr.MergeMetadata(req.Metadata, req.Address, req.College)
	address, college := metaString(meta, "address"), metaString(meta, "college")
	if req.Address != nil && *req.Address != "" {
		address = req.Address
	}
	if req.College != nil && *req.College != "" {
		college = req.College
	}

	dup, err := s.guests.FindDuplicate(ctx, req.Name, req.PhoneNumber, req.Email, address, college)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, fmt.Errorf("%w: guest '%s' already exists", ErrDuplicateGuest, dup.Name)
	}

	guestUUID := uuid.New()
	token := s.qr.SignToken(guestUUID)

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		metaJSON = []byte(`{}`)
	}

	guest := &models.Guest{
		UUID:        guestUUID,
		Name:        req.Name,
		PhoneNumber: req.PhoneNumber,
		Email:       req.Email,
		QRToken:     token,
		Metadata:    metaJSON,
		CreatedBy:   &ownerID,
	}
	if err := s.guests.Create(ctx, guest); err != nil {
		return nil, err
	}

	_, imageURL, err := s.qr.GenerateGuestQR(qr.GuestQRInput{
		UUID:     guestUUID,
		GuestID:  guest.ID,
		Name:     req.Name,
		Phone:    req.PhoneNumber,
		Email:    req.Email,
		Address:  address,
		College:  college,
		Metadata: meta,
	})
	if err == nil && imageURL != "" {
		guest.QRImageURL = &imageURL
		_ = s.guests.UpdateQRImage(ctx, guest.ID, imageURL)
	}

	if req.PhoneNumber != nil && *req.PhoneNumber != "" {
		phone := *req.PhoneNumber
		go func() {
			_ = s.notifications.SendGuestInvitation(context.Background(), req.Name, phone,
				s.event.Name, s.event.Date, s.event.Location, imageURL)
		}()
	}

	s.audit.Log(ctx, &actorID, &role, models.AuditCreateGuest,
		fmt.Sprintf("Created guest %s", req.Name), ip)

	full, err := s.guests.FindByID(ctx, guest.ID)
	if err != nil {
		return nil, err
	}
	resp := toGuestResponse(full)
	if s.ws != nil {
		s.ws.BroadcastGuestAdded(resp)
		s.ws.BroadcastDashboardUpdated()
	}
	return &resp, nil
}

func metaString(meta map[string]interface{}, key string) *string {
	if v, ok := meta[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

func (s *GuestService) Update(ctx context.Context, id int64, req dto.UpdateGuestRequest, userID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	if role != models.RoleLeader {
		return nil, ErrForbiddenAction
	}

	existing, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, sql.ErrNoRows
	}
	if existing.CreatedBy == nil || *existing.CreatedBy != userID {
		return nil, ErrForbiddenAction
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

	meta := qr.RawMetadata(existing.Metadata)
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			meta[k] = v
		}
	}
	if req.Address != nil {
		meta["address"] = *req.Address
	}
	if req.College != nil {
		meta["college"] = *req.College
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	existing.Metadata = metaJSON

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
	if role != models.RoleLeader && role != models.RoleMaster {
		return ErrForbiddenAction
	}

	existing, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return sql.ErrNoRows
	}
	if role == models.RoleLeader {
		if existing.CreatedBy == nil || *existing.CreatedBy != userID {
			return ErrForbiddenAction
		}
	}

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
		result[i] = dto.GuestSearchResponse{
			Guest:       toGuestResponse(&g),
			QRStatus:    "VALID",
			EntryStatus: entryStatus,
		}
	}
	return result, nil
}

func (s *GuestService) Import(ctx context.Context, filename string, r io.Reader, userID int64, role models.UserRole, leaderID *int64, ip string) (*dto.ImportResult, error) {
	if role != models.RoleLeader && role != models.RoleMaster {
		return nil, ErrForbiddenAction
	}

	ownerID, err := s.resolveGuestOwner(ctx, userID, role, leaderID)
	if err != nil {
		return nil, err
	}

	rows, err := importsvc.ParseFile(ctx, filename, r)
	if err != nil {
		return nil, err
	}

	result := &dto.ImportResult{TotalRows: len(rows)}
	for i, row := range rows {
		req := dto.CreateGuestRequest{
			Name:     row.Name,
			Metadata: importsvc.RowToMetadata(row),
		}
		if row.PhoneNumber != "" {
			req.PhoneNumber = &row.PhoneNumber
		}
		if row.Email != "" {
			req.Email = &row.Email
		}
		if row.Address != "" {
			req.Address = &row.Address
		}
		if row.College != "" {
			req.College = &row.College
		}
		if _, err := s.createOwnedGuest(ctx, req, ownerID, userID, role, ip); err != nil {
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

func (s *GuestService) InviteAll(ctx context.Context, userID int64, role models.UserRole, ip string) (*dto.InviteAllResult, error) {
	if role != models.RoleLeader {
		return nil, ErrForbiddenAction
	}

	guests, _, err := s.guests.List(ctx, 100000, 0)
	if err != nil {
		return nil, err
	}

	result := &dto.InviteAllResult{Total: len(guests)}
	for _, guest := range guests {
		if guest.CreatedBy == nil || *guest.CreatedBy != userID {
			continue
		}

		phone := ""
		if guest.PhoneNumber != nil {
			phone = *guest.PhoneNumber
		}
		if phone == "" {
			result.Skipped++
			result.SkippedGuests = append(result.SkippedGuests, guest.Name)
			continue
		}

		qrURL := ""
		if guest.QRImageURL != nil {
			qrURL = *guest.QRImageURL
		}
		message := notifications.BuildGuestInvitationMessage(
			guest.Name, s.event.Name, s.event.Date, s.event.Location, qrURL,
		)

		if err := s.notifications.SendGuestInvitation(ctx, guest.Name, phone,
			s.event.Name, s.event.Date, s.event.Location, qrURL); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", guest.Name, err))
			continue
		}

		result.Sent++
		result.Invitations = append(result.Invitations, dto.GuestInvitation{
			GuestID:     guest.ID,
			Name:        guest.Name,
			PhoneNumber: phone,
			Message:     message,
			QRImageURL:  qrURL,
		})
	}

	s.audit.Log(ctx, &userID, &role, models.AuditInviteGuests,
		fmt.Sprintf("Bulk WhatsApp invite: %d sent, %d skipped, %d failed", result.Sent, result.Skipped, result.Failed), ip)

	return result, nil
}

func toGuestResponse(g *models.GuestWithChecker) dto.GuestResponse {
	metadata := qr.RawMetadata(g.Metadata)
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
		CreatedBy:        g.CreatedBy,
		CreatedByEmail:   g.CreatedByEmail,
		Metadata:         metadata,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}
}
