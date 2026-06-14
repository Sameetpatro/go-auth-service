package service

import (
	"context"
	"fmt"

	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
)

type AnalyticsService struct {
	analytics *repository.AnalyticsRepository
}

func NewAnalyticsService(analytics *repository.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{analytics: analytics}
}

func (s *AnalyticsService) GetDashboard(ctx context.Context, role models.UserRole) (*dto.AnalyticsResponse, error) {
	if role == models.RoleCoordinator {
		return s.getCoordinatorDashboard(ctx)
	}
	return s.getFullDashboard(ctx)
}

func (s *AnalyticsService) getCoordinatorDashboard(ctx context.Context) (*dto.AnalyticsResponse, error) {
	total, err := s.analytics.TotalGuests(ctx)
	if err != nil {
		return nil, err
	}
	checkedIn, err := s.analytics.TotalCheckedIn(ctx)
	if err != nil {
		return nil, err
	}

	pending := total - checkedIn
	var pct float64
	if total > 0 {
		pct = float64(checkedIn) / float64(total) * 100
	}

	statsDTO, err := s.buildLeaderGuestStats(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.AnalyticsResponse{
		Overview: dto.AnalyticsOverview{
			TotalGuests:       total,
			TotalCheckedIn:    checkedIn,
			TotalPending:      pending,
			CheckInPercentage: pct,
		},
		LeaderGuestStats: statsDTO,
	}, nil
}

func (s *AnalyticsService) getFullDashboard(ctx context.Context) (*dto.AnalyticsResponse, error) {
	total, err := s.analytics.TotalGuests(ctx)
	if err != nil {
		return nil, err
	}
	checkedIn, err := s.analytics.TotalCheckedIn(ctx)
	if err != nil {
		return nil, err
	}
	today, err := s.analytics.TodayEntries(ctx)
	if err != nil {
		return nil, err
	}
	vip, err := s.analytics.VIPEntries(ctx)
	if err != nil {
		return nil, err
	}

	pending := total - checkedIn
	var pct float64
	if total > 0 {
		pct = float64(checkedIn) / float64(total) * 100
	}

	hourly, err := s.analytics.HourlyEntryCount(ctx)
	if err != nil {
		return nil, err
	}
	hourlyDTO := make([]dto.HourlyEntryCount, len(hourly))
	for i, h := range hourly {
		hourlyDTO[i] = dto.HourlyEntryCount{Hour: h.Hour, Count: h.Count}
	}

	coordEntries, err := s.analytics.EntriesByRole(ctx, models.RoleCoordinator)
	if err != nil {
		return nil, err
	}
	coordDTO := make([]dto.CoordinatorEntryCount, len(coordEntries))
	for i, c := range coordEntries {
		coordDTO[i] = dto.CoordinatorEntryCount{UserID: c.UserID, Email: c.Email, Count: c.Count}
	}

	masterEntries, err := s.analytics.EntriesByRole(ctx, models.RoleMaster)
	if err != nil {
		return nil, err
	}
	masterDTO := make([]dto.CoordinatorEntryCount, len(masterEntries))
	for i, m := range masterEntries {
		masterDTO[i] = dto.CoordinatorEntryCount{UserID: m.UserID, Email: m.Email, Count: m.Count}
	}

	leaderEntries, err := s.analytics.EntriesByRole(ctx, models.RoleLeader)
	if err != nil {
		return nil, err
	}
	leaderDTO := make([]dto.CoordinatorEntryCount, len(leaderEntries))
	for i, l := range leaderEntries {
		leaderDTO[i] = dto.CoordinatorEntryCount{UserID: l.UserID, Email: l.Email, Count: l.Count}
	}

	leaderStats, err := s.analytics.LeaderGuestStats(ctx)
	if err != nil {
		return nil, err
	}
	statsDTO := leaderStatsToDTO(leaderStats)

	return &dto.AnalyticsResponse{
		Overview: dto.AnalyticsOverview{
			TotalGuests:       total,
			TotalCheckedIn:    checkedIn,
			TotalPending:      pending,
			CheckInPercentage: pct,
			TodayEntries:      today,
			VIPEntries:        vip,
		},
		HourlyEntryCount:   hourlyDTO,
		CoordinatorEntries: coordDTO,
		LeaderEntries:      leaderDTO,
		MasterEntries:      masterDTO,
		LeaderGuestStats:   statsDTO,
	}, nil
}

func (s *AnalyticsService) buildLeaderGuestStats(ctx context.Context) ([]dto.LeaderGuestStats, error) {
	leaderStats, err := s.analytics.LeaderGuestStats(ctx)
	if err != nil {
		return nil, err
	}
	return leaderStatsToDTO(leaderStats), nil
}

func leaderStatsToDTO(leaderStats []struct {
	UserID      int64   `db:"user_id"`
	Email       string  `db:"email"`
	DisplayName *string `db:"display_name"`
	TotalGuests int64   `db:"total_guests"`
	CheckedIn   int64   `db:"checked_in"`
}) []dto.LeaderGuestStats {
	statsDTO := make([]dto.LeaderGuestStats, len(leaderStats))
	for i, ls := range leaderStats {
		username := ls.Email
		if ls.DisplayName != nil {
			username = *ls.DisplayName
		} else if parts := splitEmail(ls.Email); len(parts) > 0 {
			username = parts[0]
		}
		statsDTO[i] = dto.LeaderGuestStats{
			UserID:        ls.UserID,
			Email:         ls.Email,
			Username:      username,
			TotalGuests:   ls.TotalGuests,
			CheckedIn:     ls.CheckedIn,
			PendingGuests: ls.TotalGuests - ls.CheckedIn,
		}
	}
	return statsDTO
}

func splitEmail(email string) []string {
	for i, r := range email {
		if r == '@' {
			return []string{email[:i]}
		}
	}
	return []string{email}
}

type InsightsService struct {
	analytics *repository.AnalyticsRepository
}

func NewInsightsService(analytics *repository.AnalyticsRepository) *InsightsService {
	return &InsightsService{analytics: analytics}
}

func (s *InsightsService) GetInsights(ctx context.Context) (*dto.InsightsResponse, error) {
	total, err := s.analytics.TotalGuests(ctx)
	if err != nil {
		return nil, err
	}
	checkedIn, err := s.analytics.TotalCheckedIn(ctx)
	if err != nil {
		return nil, err
	}

	addedPerDay, err := s.analytics.GuestsAddedPerDay(ctx, 30)
	if err != nil {
		return nil, err
	}
	addedDTO := make([]dto.DailyCount, len(addedPerDay))
	for i, d := range addedPerDay {
		addedDTO[i] = dto.DailyCount{Date: d.Date, Count: d.Count}
	}

	hourly, err := s.analytics.HourlyEntryCount(ctx)
	if err != nil {
		return nil, err
	}
	hourlyDTO := make([]dto.HourlyEntryCount, len(hourly))
	for i, h := range hourly {
		hourlyDTO[i] = dto.HourlyEntryCount{Hour: h.Hour, Count: h.Count}
	}

	coordEntries, err := s.analytics.EntriesByRole(ctx, models.RoleCoordinator)
	if err != nil {
		return nil, err
	}
	var mostActive *dto.CoordinatorEntryCount
	if len(coordEntries) > 0 {
		top := coordEntries[0]
		mostActive = &dto.CoordinatorEntryCount{UserID: top.UserID, Email: top.Email, Count: top.Count}
	}

	peakHour, err := s.analytics.PeakEntryHour(ctx)
	if err != nil {
		return nil, err
	}
	var peakTime *string
	if peakHour != nil {
		t := fmt.Sprintf("%02d:00 - %02d:59", *peakHour, *peakHour)
		peakTime = &t
	}

	dupes, err := s.analytics.DuplicateScanAttempts(ctx)
	if err != nil {
		return nil, err
	}
	failed, err := s.analytics.FailedScanAttempts(ctx)
	if err != nil {
		return nil, err
	}

	gates, err := s.analytics.TopScanningGates(ctx, 10)
	if err != nil {
		return nil, err
	}
	gatesDTO := make([]dto.GateCount, len(gates))
	for i, g := range gates {
		gatesDTO[i] = dto.GateCount{GateName: g.GateName, Count: g.Count}
	}

	avgRate, err := s.analytics.AverageEntryRate(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.InsightsResponse{
		GuestsAddedPerDay:     addedDTO,
		EntriesPerHour:        hourlyDTO,
		MostActiveCoordinator: mostActive,
		PeakEntryTime:         peakTime,
		PendingGuests:         total - checkedIn,
		DuplicateScanAttempts: dupes,
		FailedScanAttempts:    failed,
		TopScanningGates:      gatesDTO,
		AverageEntryRate:      avgRate,
	}, nil
}
