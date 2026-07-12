package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sameetpatro/go-qr-auth/internal/dto"
	importsvc "github.com/sameetpatro/go-qr-auth/internal/guests"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/notifications"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
	"github.com/sameetpatro/go-qr-auth/internal/tags"
)

func (s *GuestService) Create(ctx context.Context, req dto.CreateGuestRequest, userID int64, role models.UserRole, ip string) (*dto.GuestResponse, error) {
	ownerID, err := s.resolveGuestOwner(ctx, userID, role, req.LeaderID)
	if err != nil {
		return nil, err
	}
	resp, pending, err := s.createOwnedGuest(ctx, req, ownerID, userID, role, ip, true)
	if err != nil {
		return nil, err
	}
	_ = pending
	return resp, nil
}

// pendingQR holds work deferred until after a bulk import finishes inserting rows.
// Concurrent QR uploads during import corrupt lib/pq prepared statements when the
// DB URL goes through PgBouncer (Neon -pooler host).
type pendingQR struct {
	input qr.GuestQRInput
	phone string
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

func (s *GuestService) createOwnedGuest(ctx context.Context, req dto.CreateGuestRequest, ownerID int64, actorID int64, role models.UserRole, ip string, startQR bool) (*dto.GuestResponse, *pendingQR, error) {
	meta := qr.MergeMetadata(req.Metadata, req.Address, req.Department)
	address, department := metaString(meta, "address"), metaString(meta, "department")
	if req.Address != nil && *req.Address != "" {
		address = req.Address
	}
	if req.Department != nil && *req.Department != "" {
		department = req.Department
	}
	// Always persist a normalized member tag (defaults to "invitee") so the QR
	// card, scanner flash, and analytics have a consistent value to work with.
	rawTag := ""
	if req.Tag != nil {
		rawTag = *req.Tag
	} else if v, ok := meta["tag"].(string); ok {
		rawTag = v
	}
	meta["tag"] = tags.Normalize(rawTag)

	dup, err := s.guests.FindDuplicate(ctx, req.Name, req.PhoneNumber, req.Email)
	if err != nil {
		return nil, nil, err
	}
	if dup != nil {
		return nil, nil, fmt.Errorf("%w: guest '%s' already exists", ErrDuplicateGuest, dup.Name)
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
		return nil, nil, err
	}

	phone := ""
	if req.PhoneNumber != nil {
		phone = *req.PhoneNumber
	}
	pending := &pendingQR{
		input: qr.GuestQRInput{
			UUID:       guestUUID,
			GuestID:    guest.ID,
			Name:       req.Name,
			Phone:      req.PhoneNumber,
			Email:      req.Email,
			Address:    address,
			Department: department,
			Metadata:   meta,
		},
		phone: phone,
	}
	// QR card rendering + Cloudinary upload take ~1s each on small instances;
	// doing them inline made bulk imports exceed the HTTP write timeout (502s).
	// Single creates start immediately; bulk import defers until all rows insert
	// so concurrent UpdateQRImage cannot collide with Create/FindDuplicate on
	// PgBouncer (prepared-statement bind mismatches).
	if startQR {
		go s.generateAndStoreQR(pending.input, pending.phone)
	}

	s.audit.Log(ctx, &actorID, &role, models.AuditCreateGuest,
		fmt.Sprintf("Created guest %s", req.Name), ip)

	full, err := s.guests.FindByID(ctx, guest.ID)
	if err != nil {
		return nil, nil, err
	}
	resp := toGuestResponse(full)
	if s.ws != nil {
		s.ws.BroadcastGuestAdded(resp)
		s.ws.BroadcastDashboardUpdated()
	}
	return &resp, pending, nil
}

// generateAndStoreQR renders the invitation card, persists it (Cloudinary or
// disk), saves the resulting URL, and then sends the invitation notification.
// Runs detached from the request so slow storage never blocks HTTP responses.
func (s *GuestService) generateAndStoreQR(input qr.GuestQRInput, phone string) {
	// Bound concurrent renders/uploads so a large CSV import doesn't spike
	// CPU/memory on small instances.
	s.qrJobs <- struct{}{}
	defer func() { <-s.qrJobs }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, imageURL, err := s.qr.GenerateGuestQR(ctx, input)
	if err == nil && imageURL != "" {
		_ = s.guests.UpdateQRImage(ctx, input.GuestID, imageURL)
	}

	if phone != "" {
		_ = s.notifications.SendGuestInvitation(ctx, input.Name, phone,
			s.event.Name, s.event.Date, s.event.Location, imageURL)
	}
}

// deleteStoredCards removes the Cloudinary (or local-disk) QR images for
// deleted guests in the background; the DB rows are already gone, so failures
// here only leave harmless orphaned images behind.
func (s *GuestService) deleteStoredCards(cards map[int64]string) {
	s.qrJobs <- struct{}{}
	defer func() { <-s.qrJobs }()

	for guestID, name := range cards {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.qr.DeleteStoredCard(ctx, guestID, name); err != nil {
			log.Printf("qr: delete stored card for guest %d: %v", guestID, err)
		}
		cancel()
	}
}

func metaString(meta map[string]interface{}, key string) *string {
	if v, ok := meta[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

// isTransientDBError reports prepared-statement / bind races typical of
// PgBouncer transaction pooling with lib/pq under concurrent load.
func isTransientDBError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "08P01") ||
		strings.Contains(msg, "bind message supplies") ||
		strings.Contains(msg, "prepared statement") ||
		(strings.Contains(msg, "22P02") && strings.Contains(msg, "invalid input syntax for type bigint"))
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
	if req.Department != nil {
		meta["department"] = *req.Department
		delete(meta, "college")
	}
	if req.Tag != nil {
		meta["tag"] = tags.Normalize(*req.Tag)
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
	go s.deleteStoredCards(map[int64]string{id: existing.Name})
	s.audit.Log(ctx, &userID, &role, models.AuditDeleteGuest,
		fmt.Sprintf("Deleted guest ID %d", id), ip)
	if s.ws != nil {
		s.ws.BroadcastDashboardUpdated()
	}
	return nil
}

func (s *GuestService) DeleteAll(ctx context.Context, userID int64, role models.UserRole, leaderID *int64, ip string) (*dto.BulkDeleteResult, error) {
	var ownerID int64
	switch role {
	case models.RoleLeader:
		ownerID = userID
	case models.RoleMaster:
		if leaderID == nil || *leaderID <= 0 {
			return nil, fmt.Errorf("leader_id is required when deleting guests as master")
		}
		user, err := s.users.FindByID(ctx, *leaderID)
		if err != nil {
			return nil, err
		}
		if user == nil || user.Role != models.RoleLeader {
			return nil, fmt.Errorf("invalid leader_id")
		}
		ownerID = *leaderID
	default:
		return nil, ErrForbiddenAction
	}

	// Capture IDs/names before the rows disappear so we can clean up their
	// stored QR images afterwards.
	toDelete, _, err := s.guests.ListByCreator(ctx, ownerID, 100000, 0)
	if err != nil {
		return nil, err
	}

	deleted, err := s.guests.DeleteAllByCreator(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if len(toDelete) > 0 {
		cards := make(map[int64]string, len(toDelete))
		for _, g := range toDelete {
			cards[g.ID] = g.Name
		}
		go s.deleteStoredCards(cards)
	}
	s.audit.Log(ctx, &userID, &role, models.AuditBulkDeleteGuests,
		fmt.Sprintf("Bulk deleted %d guests for leader ID %d", deleted, ownerID), ip)
	if s.ws != nil {
		s.ws.BroadcastDashboardUpdated()
	}
	return &dto.BulkDeleteResult{Deleted: deleted}, nil
}

func (s *GuestService) GetRegistry(ctx context.Context, userID int64, role models.UserRole) ([]dto.GuestRegistryEntry, error) {
	var creatorID *int64
	switch role {
	case models.RoleLeader:
		creatorID = &userID
	case models.RoleMaster, models.RoleCoordinator:
		// full guest list
	default:
		return nil, ErrForbiddenAction
	}

	rows, err := s.guests.ListRegistry(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.GuestRegistryEntry, len(rows))
	for i, row := range rows {
		result[i] = dto.GuestRegistryEntry{
			GuestResponse: toGuestResponse(&row.GuestWithChecker),
			CheckInGate:   row.CheckInGate,
		}
	}
	return result, nil
}

func (s *GuestService) GetQRImage(ctx context.Context, id int64, userID int64, role models.UserRole) ([]byte, string, error) {
	guest, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if guest == nil {
		return nil, "", sql.ErrNoRows
	}
	if role == models.RoleLeader {
		if guest.CreatedBy == nil || *guest.CreatedBy != userID {
			return nil, "", ErrForbiddenAction
		}
	} else if role != models.RoleMaster {
		return nil, "", ErrForbiddenAction
	}

	meta := qr.RawMetadata(guest.Metadata)
	address, department := qr.MetadataAddressDepartment(meta)
	input := qr.GuestQRInput{
		UUID:       guest.UUID,
		GuestID:    guest.ID,
		Name:       guest.Name,
		Phone:      guest.PhoneNumber,
		Email:      guest.Email,
		Address:    address,
		Department: department,
		Metadata:   meta,
	}

	png, err := s.qr.RenderGuestQRPNG(input, guest.QRToken)
	if err != nil {
		return nil, "", err
	}

	// Self-heal: if the stored URL is not permanent storage yet (missing, or a
	// dead ephemeral-disk URL), upload the card now so future fetches use the CDN.
	if !s.qr.IsPermanentURL(guest.QRImageURL) {
		if imageURL, err := s.qr.RegenerateCard(ctx, input, guest.QRToken); err == nil && imageURL != "" {
			_ = s.guests.UpdateQRImage(ctx, guest.ID, imageURL)
		}
	}

	filename := fmt.Sprintf("%s_%d.png", qr.SanitizeFilename(guest.Name), guest.ID)
	return png, filename, nil
}

func (s *GuestService) ManualCheckIn(ctx context.Context, guestID int64, userID int64, role models.UserRole, req dto.ManualCheckInRequest, ip string) (*dto.GuestResponse, error) {
	if role != models.RoleMaster {
		return nil, ErrForbiddenAction
	}

	existing, err := s.guests.FindByID(ctx, guestID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, sql.ErrNoRows
	}

	updated, err := s.guests.ManualCheckIn(ctx, guestID, userID, req.GateName)
	if err != nil {
		return nil, err
	}

	s.audit.Log(ctx, &userID, &role, models.AuditManualCheckIn,
		fmt.Sprintf("Manual check-in for guest ID %d", guestID), ip)
	if s.ws != nil {
		s.ws.BroadcastDashboardUpdated()
	}
	resp := toGuestResponse(updated)
	return &resp, nil
}

func (s *GuestService) GetByID(ctx context.Context, id int64, userID int64, role models.UserRole) (*dto.GuestResponse, error) {
	guest, err := s.guests.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if guest == nil {
		return nil, sql.ErrNoRows
	}
	if role == models.RoleLeader {
		if guest.CreatedBy == nil || *guest.CreatedBy != userID {
			return nil, ErrForbiddenAction
		}
	}
	resp := toGuestResponse(guest)
	return &resp, nil
}

func (s *GuestService) Search(ctx context.Context, query string, page, perPage int, userID int64, role models.UserRole) (*dto.PaginatedGuestsResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var guests []models.GuestWithChecker
	var total int64
	var err error
	if role == models.RoleLeader {
		guests, total, err = s.guests.SearchByCreator(ctx, userID, query, perPage, offset)
	} else {
		guests, total, err = s.guests.Search(ctx, query, perPage, offset)
	}
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

func (s *GuestService) List(ctx context.Context, page, perPage int, userID int64, role models.UserRole) (*dto.PaginatedGuestsResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var guests []models.GuestWithChecker
	var total int64
	var err error
	if role == models.RoleLeader {
		guests, total, err = s.guests.ListByCreator(ctx, userID, perPage, offset)
	} else {
		guests, total, err = s.guests.List(ctx, perPage, offset)
	}
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

func (s *GuestService) SearchForVerification(ctx context.Context, query string, userID int64, role models.UserRole) ([]dto.GuestSearchResponse, error) {
	var guests []models.GuestWithChecker
	var err error
	if role == models.RoleLeader {
		guests, _, err = s.guests.SearchByCreator(ctx, userID, query, 50, 0)
	} else {
		guests, _, err = s.guests.Search(ctx, query, 50, 0)
	}
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
	pendingQRs := make([]*pendingQR, 0, len(rows))
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
		if row.Department != "" {
			req.Department = &row.Department
		}

		var (
			pending *pendingQR
			err     error
		)
		// Retry transient prepared-statement races (PgBouncer / concurrent load).
		for attempt := 0; attempt < 3; attempt++ {
			_, pending, err = s.createOwnedGuest(ctx, req, ownerID, userID, role, ip, false)
			if err == nil || !isTransientDBError(err) {
				break
			}
			time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
		}
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", i+2, err))
			continue
		}
		result.Imported++
		if pending != nil {
			pendingQRs = append(pendingQRs, pending)
		}
	}

	// Start QR generation only after every insert finished.
	for _, p := range pendingQRs {
		go s.generateAndStoreQR(p.input, p.phone)
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

	guests, _, err := s.guests.ListByCreator(ctx, userID, 100000, 0)
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
	tag := tags.DefaultKey
	if metadata != nil {
		if v, ok := metadata["tag"].(string); ok {
			tag = tags.Normalize(v)
		}
	}
	// Prefer the permanent CDN URL; ephemeral /storage/qr URLs from the old
	// disk-based flow fall back to the API endpoint, which regenerates on demand.
	qrURL := guestQRImageURL(g.ID)
	if g.QRImageURL != nil && strings.Contains(*g.QRImageURL, "res.cloudinary.com") {
		qrURL = *g.QRImageURL
	}
	return dto.GuestResponse{
		ID:               g.ID,
		UUID:             g.UUID,
		Name:             g.Name,
		PhoneNumber:      g.PhoneNumber,
		Email:            g.Email,
		QRImageURL:       &qrURL,
		Tag:              tag,
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
