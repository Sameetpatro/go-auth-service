package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/middleware"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/service"
	"github.com/sameetpatro/go-qr-auth/pkg/response"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Login godoc
// @Summary Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.AuthResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.auth.Login(c.Request.Context(), req, middleware.GetClientIP(c))
	if errors.Is(err, service.ErrInvalidCredentials) {
		response.Unauthorized(c, "Invalid email or password")
		return
	}
	if errors.Is(err, service.ErrUserInactive) {
		response.Forbidden(c, "Account is disabled")
		return
	}
	if err != nil {
		response.InternalError(c, "Login failed")
		return
	}
	response.Success(c, "Login successful", resp)
}

// Refresh godoc
// @Summary Refresh access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} dto.AuthResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if errors.Is(err, service.ErrUnauthorized) {
		response.Unauthorized(c, "Invalid refresh token")
		return
	}
	if err != nil {
		response.InternalError(c, "Token refresh failed")
		return
	}
	response.Success(c, "Token refreshed", resp)
}

type CoordinatorHandler struct {
	coordinators *service.CoordinatorService
	ws           CoordinatorBroadcaster
}

type CoordinatorBroadcaster interface {
	BroadcastCoordinatorCreated(data interface{})
}

func NewCoordinatorHandler(coordinators *service.CoordinatorService, ws CoordinatorBroadcaster) *CoordinatorHandler {
	return &CoordinatorHandler{coordinators: coordinators, ws: ws}
}

// Create godoc
// @Summary Create coordinator (Master only)
// @Tags Coordinators
// @Security BearerAuth
// @Produce json
// @Success 201 {object} dto.CreateCoordinatorResponse
// @Router /api/v1/coordinators [post]
func (h *CoordinatorHandler) Create(c *gin.Context) {
	resp, err := h.coordinators.Create(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if err != nil {
		response.InternalError(c, "Failed to create coordinator")
		return
	}
	if h.ws != nil {
		h.ws.BroadcastCoordinatorCreated(resp)
	}
	response.Created(c, "Coordinator created", resp)
}

// List godoc
// @Summary List coordinators (Master only)
// @Tags Coordinators
// @Security BearerAuth
// @Produce json
// @Success 200 {array} dto.CoordinatorResponse
// @Router /api/v1/coordinators [get]
func (h *CoordinatorHandler) List(c *gin.Context) {
	resp, err := h.coordinators.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to list coordinators")
		return
	}
	response.Success(c, "Coordinators retrieved", resp)
}

// Disable godoc
// @Summary Disable coordinator (Master only)
// @Tags Coordinators
// @Security BearerAuth
// @Param id path int true "Coordinator ID"
// @Success 200 {object} dto.SuccessResponse
// @Router /api/v1/coordinators/{id}/disable [patch]
func (h *CoordinatorHandler) Disable(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid coordinator ID")
		return
	}
	if err := h.coordinators.Disable(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), id, middleware.GetClientIP(c)); err != nil {
		response.InternalError(c, "Failed to disable coordinator")
		return
	}
	response.Success(c, "Coordinator disabled", nil)
}

// ResetPassword godoc
// @Summary Reset coordinator password (Master only)
// @Tags Coordinators
// @Security BearerAuth
// @Param id path int true "Coordinator ID"
// @Success 200 {object} dto.ResetPasswordResponse
// @Router /api/v1/coordinators/{id}/reset-password [post]
func (h *CoordinatorHandler) ResetPassword(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid coordinator ID")
		return
	}
	resp, err := h.coordinators.ResetPassword(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), id, middleware.GetClientIP(c))
	if err != nil {
		response.NotFound(c, "Coordinator not found")
		return
	}
	response.Success(c, "Password reset", resp)
}

type LeaderHandler struct {
	leaders *service.LeaderService
	ws      CoordinatorBroadcaster
}

func NewLeaderHandler(leaders *service.LeaderService, ws CoordinatorBroadcaster) *LeaderHandler {
	return &LeaderHandler{leaders: leaders, ws: ws}
}

func (h *LeaderHandler) Create(c *gin.Context) {
	var req dto.CreateLeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.leaders.Create(c.Request.Context(), middleware.GetUserID(c), req, middleware.GetClientIP(c))
	if errors.Is(err, service.ErrLeaderEmailExists) {
		response.BadRequest(c, "Leader email already exists")
		return
	}
	if errors.Is(err, service.ErrInvalidUsername) {
		response.BadRequest(c, "Invalid username. Use letters, numbers, dots, dashes (2-50 chars).")
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to create leader")
		return
	}
	if h.ws != nil {
		h.ws.BroadcastCoordinatorCreated(resp)
	}
	response.Created(c, "Leader created", resp)
}

func (h *LeaderHandler) List(c *gin.Context) {
	resp, err := h.leaders.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to list leaders")
		return
	}
	response.Success(c, "Leaders retrieved", resp)
}

func (h *LeaderHandler) Disable(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid leader ID")
		return
	}
	if err := h.leaders.Disable(c.Request.Context(), middleware.GetUserID(c), id, middleware.GetClientIP(c)); err != nil {
		if errors.Is(err, service.ErrLeaderNotFound) {
			response.NotFound(c, "Leader not found")
			return
		}
		response.InternalError(c, "Failed to disable leader")
		return
	}
	response.Success(c, "Leader disabled", nil)
}

func (h *LeaderHandler) ResetPassword(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid leader ID")
		return
	}
	resp, err := h.leaders.ResetPassword(c.Request.Context(), middleware.GetUserID(c), id, middleware.GetClientIP(c))
	if errors.Is(err, service.ErrLeaderNotFound) {
		response.NotFound(c, "Leader not found")
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to reset password")
		return
	}
	response.Success(c, "Password reset", resp)
}

type GuestHandler struct {
	guests *service.GuestService
}

func NewGuestHandler(guests *service.GuestService) *GuestHandler {
	return &GuestHandler{guests: guests}
}

// Create godoc
// @Summary Create guest
// @Tags Guests
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateGuestRequest true "Guest data"
// @Success 201 {object} dto.GuestResponse
// @Router /api/v1/guests [post]
func (h *GuestHandler) Create(c *gin.Context) {
	var req dto.CreateGuestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.guests.Create(c.Request.Context(), req, middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if errors.Is(err, service.ErrDuplicateGuest) {
		response.BadRequest(c, err.Error())
		return
	}
	if errors.Is(err, service.ErrForbiddenAction) {
		response.Forbidden(c, "Insufficient permissions")
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to create guest")
		return
	}
	response.Created(c, "Guest created", resp)
}

// List godoc
// @Summary List guests
// @Tags Guests
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number"
// @Param per_page query int false "Items per page"
// @Success 200 {object} dto.PaginatedGuestsResponse
// @Router /api/v1/guests [get]
func (h *GuestHandler) List(c *gin.Context) {
	page, perPage := pagination(c)
	resp, err := h.guests.List(c.Request.Context(), page, perPage)
	if err != nil {
		response.InternalError(c, "Failed to list guests")
		return
	}
	response.Success(c, "Guests retrieved", resp)
}

// Get godoc
// @Summary Get guest by ID
// @Tags Guests
// @Security BearerAuth
// @Param id path int true "Guest ID"
// @Success 200 {object} dto.GuestResponse
// @Router /api/v1/guests/{id} [get]
func (h *GuestHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid guest ID")
		return
	}
	resp, err := h.guests.GetByID(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Guest not found")
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to get guest")
		return
	}
	response.Success(c, "Guest retrieved", resp)
}

// Update godoc
// @Summary Update guest (Master only)
// @Tags Guests
// @Security BearerAuth
// @Accept json
// @Param id path int true "Guest ID"
// @Param request body dto.UpdateGuestRequest true "Guest data"
// @Success 200 {object} dto.GuestResponse
// @Router /api/v1/guests/{id} [put]
func (h *GuestHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid guest ID")
		return
	}
	var req dto.UpdateGuestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.guests.Update(c.Request.Context(), id, req, middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Guest not found")
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to update guest")
		return
	}
	response.Success(c, "Guest updated", resp)
}

// Delete godoc
// @Summary Delete guest (Master only)
// @Tags Guests
// @Security BearerAuth
// @Param id path int true "Guest ID"
// @Success 200 {object} dto.SuccessResponse
// @Router /api/v1/guests/{id} [delete]
func (h *GuestHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "Invalid guest ID")
		return
	}
	if err := h.guests.Delete(c.Request.Context(), id, middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c)); err != nil {
		if errors.Is(err, service.ErrForbiddenAction) {
			response.Forbidden(c, "You can only remove guests you invited")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "Guest not found")
			return
		}
		response.InternalError(c, "Failed to delete guest")
		return
	}
	response.Success(c, "Guest deleted", nil)
}

// Search godoc
// @Summary Search guests
// @Tags Guests
// @Security BearerAuth
// @Param q query string true "Search query"
// @Param page query int false "Page number"
// @Param per_page query int false "Items per page"
// @Success 200 {object} dto.PaginatedGuestsResponse
// @Router /api/v1/guests/search [get]
func (h *GuestHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.BadRequest(c, "Search query is required")
		return
	}
	page, perPage := pagination(c)
	resp, err := h.guests.Search(c.Request.Context(), query, page, perPage)
	if err != nil {
		response.InternalError(c, "Search failed")
		return
	}
	response.Success(c, "Search results", resp)
}

// VerifySearch godoc
// @Summary Manual verification search
// @Tags Guests
// @Security BearerAuth
// @Param q query string true "Search query"
// @Success 200 {array} dto.GuestSearchResponse
// @Router /api/v1/guests/verify [get]
func (h *GuestHandler) VerifySearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.BadRequest(c, "Search query is required")
		return
	}
	resp, err := h.guests.SearchForVerification(c.Request.Context(), query)
	if err != nil {
		response.InternalError(c, "Verification search failed")
		return
	}
	response.Success(c, "Verification results", resp)
}

// Import godoc
// @Summary Import guests from CSV/XLSX
// @Tags Guests
// @Security BearerAuth
// @Accept multipart/form-data
// @Param file formData file true "CSV or XLSX file"
// @Success 200 {object} dto.ImportResult
// @Router /api/v1/guests/import [post]
func (h *GuestHandler) Import(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "File is required")
		return
	}
	defer file.Close()

	resp, err := h.guests.Import(c.Request.Context(), header.Filename, file,
		middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, "Import completed", resp)
}

// InviteAll godoc
// @Summary Send WhatsApp invitations to all guests (Master only)
// @Tags Guests
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.InviteAllResult
// @Router /api/v1/guests/invite-all [post]
func (h *GuestHandler) InviteAll(c *gin.Context) {
	resp, err := h.guests.InviteAll(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if err != nil {
		response.InternalError(c, "Failed to send invitations")
		return
	}
	response.Success(c, "Invitations processed", resp)
}

type ScanHandler struct {
	scan *service.ScanService
}

func NewScanHandler(scan *service.ScanService) *ScanHandler {
	return &ScanHandler{scan: scan}
}

// Scan godoc
// @Summary Scan QR code for entry
// @Tags Scan
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.ScanRequest true "QR token"
// @Success 200 {object} dto.ScanResponse
// @Router /api/v1/scan [post]
func (h *ScanHandler) Scan(c *gin.Context) {
	var req dto.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.scan.Scan(c.Request.Context(), req, middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if errors.Is(err, service.ErrForbiddenAction) {
		response.Forbidden(c, "Master accounts cannot scan guests")
		return
	}
	if err != nil {
		response.InternalError(c, "Scan failed")
		return
	}
	status := http.StatusOK
	if resp.Result == string(models.ScanResultEntryDenied) {
		status = http.StatusBadRequest
	}
	c.JSON(status, dto.SuccessResponse{Message: resp.Message, Data: resp})
}

type AnalyticsHandler struct {
	analytics *service.AnalyticsService
	insights  *service.InsightsService
	reports   *service.ReportService
}

func NewAnalyticsHandler(analytics *service.AnalyticsService, insights *service.InsightsService, reports *service.ReportService) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics, insights: insights, reports: reports}
}

// Dashboard godoc
// @Summary Get dashboard analytics
// @Tags Analytics
// @Security BearerAuth
// @Success 200 {object} dto.AnalyticsResponse
// @Router /api/v1/analytics/dashboard [get]
func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	resp, err := h.analytics.GetDashboard(c.Request.Context(), middleware.GetRole(c))
	if err != nil {
		response.InternalError(c, "Failed to get analytics")
		return
	}
	response.Success(c, "Analytics retrieved", resp)
}

// Insights godoc
// @Summary Get insights
// @Tags Insights
// @Security BearerAuth
// @Success 200 {object} dto.InsightsResponse
// @Router /api/v1/insights [get]
func (h *AnalyticsHandler) Insights(c *gin.Context) {
	resp, err := h.insights.GetInsights(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to get insights")
		return
	}
	response.Success(c, "Insights retrieved", resp)
}

// ExportCSV godoc
// @Summary Export guests CSV report
// @Tags Reports
// @Security BearerAuth
// @Produce text/csv
// @Router /api/v1/reports/export/csv [get]
func (h *AnalyticsHandler) ExportCSV(c *gin.Context) {
	data, err := h.reports.ExportCSV(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}
	c.Header("Content-Disposition", "attachment; filename=guests_report.csv")
	c.Data(http.StatusOK, "text/csv", data)
}

// ExportPDF godoc
// @Summary Export PDF report
// @Tags Reports
// @Security BearerAuth
// @Produce application/pdf
// @Router /api/v1/reports/export/pdf [get]
func (h *AnalyticsHandler) ExportPDF(c *gin.Context) {
	data, err := h.reports.ExportPDF(c.Request.Context(), middleware.GetUserID(c), middleware.GetRole(c), middleware.GetClientIP(c))
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}
	c.Header("Content-Disposition", "attachment; filename=event_report.pdf")
	c.Data(http.StatusOK, "application/pdf", data)
}

func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	return page, perPage
}

type AdminHandler struct {
	reset *service.ResetService
}

func NewAdminHandler(reset *service.ResetService) *AdminHandler {
	return &AdminHandler{reset: reset}
}

// ResetData godoc
// @Summary Reset all guest and coordinator data (Master only)
// @Tags Admin
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Router /api/v1/admin/reset [post]
func (h *AdminHandler) ResetData(c *gin.Context) {
	if err := h.reset.ResetAllData(c.Request.Context()); err != nil {
		response.InternalError(c, "Failed to reset data")
		return
	}
	response.Success(c, "All event data has been reset (master account kept)", nil)
}

// Health godoc
// @Summary Health check
// @Tags Health
// @Success 200 {object} map[string]string
// @Router /health [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
