package tournament

import (
	"time"

	"gorm.io/gorm"

	"venue-booking-admin/internal/models"
)

// ---------- 资源协调：赛事占用与散客库存 ----------

// DetectBookingConflicts 检测赛事安排与散客预订的冲突。
func DetectBookingConflicts(db *gorm.DB, tournamentID uint, matches []models.Match) ([]models.BookingConflict, error) {
	var conflicts []models.BookingConflict

	for _, match := range matches {
		if match.VenueID == 0 || match.Status == "cancelled" || match.Status == "unscheduled" {
			continue
		}

		var overlappingBookings []models.Booking
		err := db.Where("venue_id = ? AND book_date = ? AND status <> ?",
			match.VenueID, match.MatchDate, "cancelled").
			Where("start_hour < ? AND end_hour > ?", match.EndHour, match.StartHour).
			Find(&overlappingBookings).Error
		if err != nil {
			return nil, err
		}

		for _, booking := range overlappingBookings {
			// 计算冲突时段
			conflictStart := match.StartHour
			if booking.StartHour > conflictStart {
				conflictStart = booking.StartHour
			}
			conflictEnd := match.EndHour
			if booking.EndHour < conflictEnd {
				conflictEnd = booking.EndHour
			}

			var existingConflict models.BookingConflict
			err := db.Where("tournament_id = ? AND match_id = ? AND booking_id = ?",
				tournamentID, match.ID, booking.ID).First(&existingConflict).Error
			if err == nil {
				continue
			}

			conflict := models.BookingConflict{
				TournamentID:  tournamentID,
				MatchID:       match.ID,
				BookingID:     booking.ID,
				VenueID:       match.VenueID,
				ConflictDate:  match.MatchDate,
				ConflictStart: conflictStart,
				ConflictEnd:   conflictEnd,
				HandleStatus:  "pending",
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts, nil
}

// CreateVenueOccupancies 为赛事创建场地占用记录。
func CreateVenueOccupancies(db *gorm.DB, tournamentID uint, matches []models.Match) error {
	for _, match := range matches {
		if match.VenueID == 0 || match.Status == "cancelled" || match.Status == "unscheduled" {
			continue
		}

		var existing models.VenueOccupancy
		err := db.Where("match_id = ?", match.ID).First(&existing).Error
		if err == nil {
			continue
		}

		occupancy := models.VenueOccupancy{
			VenueID:      match.VenueID,
			TournamentID: tournamentID,
			MatchID:      match.ID,
			OccupyDate:   match.MatchDate,
			StartHour:    match.StartHour,
			EndHour:      match.EndHour,
			SourceType:   "tournament",
			Status:       "active",
			Remarks:      "赛事占用",
		}
		if err := db.Create(&occupancy).Error; err != nil {
			return err
		}
	}
	return nil
}

// RemoveVenueOccupancies 移除赛事的场地占用记录。
func RemoveVenueOccupancies(db *gorm.DB, tournamentID uint) error {
	return db.Where("tournament_id = ? AND source_type = ?", tournamentID, "tournament").
		Delete(&models.VenueOccupancy{}).Error
}

// ResolveConflict 处理冲突（取消散客预订或改约）。
func ResolveConflict(db *gorm.DB, conflictID uint, method string, remarks string) error {
	var conflict models.BookingConflict
	if err := db.First(&conflict, conflictID).Error; err != nil {
		return err
	}

	if method == "cancel" {
		// 取消散客预订
		var booking models.Booking
		if err := db.First(&booking, conflict.BookingID).Error; err != nil {
			return err
		}
		booking.Status = "cancelled"
		if err := db.Save(&booking).Error; err != nil {
			return err
		}
		conflict.HandleStatus = "cancelled"
	} else if method == "reschedule" {
		conflict.HandleStatus = "rescheduled"
	} else {
		conflict.HandleStatus = "resolved"
	}

	conflict.HandleMethod = method
	conflict.Remarks = remarks
	return db.Save(&conflict).Error
}

// GetAvailableSlots 获取指定日期场地的可用时段（扣除赛事占用和散客预订）。
func GetAvailableSlots(db *gorm.DB, venueID uint, date string) ([]TimeRange, error) {
	var venue models.Venue
	if err := db.First(&venue, venueID).Error; err != nil {
		return nil, err
	}

	// 获取当日所有占用
	var occupancies []models.VenueOccupancy
	db.Where("venue_id = ? AND occupy_date = ? AND status = ?", venueID, date, "active").
		Find(&occupancies)

	var bookings []models.Booking
	db.Where("venue_id = ? AND book_date = ? AND status <> ?", venueID, date, "cancelled").
		Find(&bookings)

	// 合并所有占用时段
	var allSlots []timeSlot
	for _, occ := range occupancies {
		allSlots = append(allSlots, timeSlot{start: occ.StartHour, end: occ.EndHour})
	}
	for _, bk := range bookings {
		allSlots = append(allSlots, timeSlot{start: bk.StartHour, end: bk.EndHour})
	}

	// 排序
	sortTimeSlots(allSlots)

	// 计算可用时段
	var available []TimeRange
	current := venue.OpenHour

	for _, slot := range allSlots {
		if slot.start > current {
			available = append(available, TimeRange{Start: current, End: slot.start})
		}
		if slot.end > current {
			current = slot.end
		}
	}

	if current < venue.CloseHour {
		available = append(available, TimeRange{Start: current, End: venue.CloseHour})
	}

	return available, nil
}

// TimeRange 时间范围。
type TimeRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func sortTimeSlots(slots []timeSlot) {
	for i := 0; i < len(slots); i++ {
		for j := i + 1; j < len(slots); j++ {
			if slots[j].start < slots[i].start {
				slots[i], slots[j] = slots[j], slots[i]
			}
		}
	}
}

// ---------- 晋级逻辑 ----------

// UpdateStandings 比赛结束后更新小组积分。
func UpdateStandings(db *gorm.DB, match models.Match) error {
	if match.Stage != "group" || match.Status != "completed" {
		return nil
	}

	// 计算两队积分
	points1 := 0
	points2 := 0
	if match.Team1Score > match.Team2Score {
		points1 = 3
	} else if match.Team1Score < match.Team2Score {
		points2 = 3
	} else {
		points1 = 1
		points2 = 1
	}

	// 更新Team1积分
	if match.Team1ID != nil {
		if err := updateTeamStanding(db, match.TournamentID, match.GroupNo, *match.Team1ID,
			match.Team1Score, match.Team2Score, points1); err != nil {
			return err
		}
	}

	// 更新Team2积分
	if match.Team2ID != nil {
		if err := updateTeamStanding(db, match.TournamentID, match.GroupNo, *match.Team2ID,
			match.Team2Score, match.Team1Score, points2); err != nil {
			return err
		}
	}

	return nil
}

func updateTeamStanding(db *gorm.DB, tournamentID uint, groupNo int, teamID uint, goalsFor, goalsAgainst, points int) error {
	var standing models.GroupStanding
	err := db.Where("tournament_id = ? AND group_no = ? AND team_id = ?",
		tournamentID, groupNo, teamID).First(&standing).Error

	if err != nil {
		// 新建
		standing = models.GroupStanding{
			TournamentID: tournamentID,
			GroupNo:      groupNo,
			TeamID:       teamID,
			Played:       1,
		}
		if points == 3 {
			standing.Wins = 1
		} else if points == 1 {
			standing.Draws = 1
		} else {
			standing.Losses = 1
		}
		standing.Points = points
		standing.GoalsFor = goalsFor
		standing.GoalsAgainst = goalsAgainst
		standing.GoalDiff = goalsFor - goalsAgainst
		return db.Create(&standing).Error
	}

	standing.Played++
	if points == 3 {
		standing.Wins++
	} else if points == 1 {
		standing.Draws++
	} else {
		standing.Losses++
	}
	standing.Points += points
	standing.GoalsFor += goalsFor
	standing.GoalsAgainst += goalsAgainst
	standing.GoalDiff = standing.GoalsFor - standing.GoalsAgainst
	return db.Save(&standing).Error
}

// GetGroupStandings 获取小组积分榜（按积分、净胜球、进球数排序）。
func GetGroupStandings(db *gorm.DB, tournamentID uint, groupNo int) ([]models.GroupStanding, error) {
	var standings []models.GroupStanding
	err := db.Where("tournament_id = ? AND group_no = ?", tournamentID, groupNo).
		Preload("Team").
		Order("points desc, goal_diff desc, goals_for desc").
		Find(&standings).Error
	return standings, err
}

// PromoteToKnockout 从小组赛晋级到淘汰赛（更新淘汰赛对阵队伍）。
// knockoutCount: 每小组晋级几支队伍
func PromoteToKnockout(db *gorm.DB, tournamentID uint, groupCount int, knockoutCount int) error {
	// 获取所有小组的晋级队伍
	var allQualified []uint
	for g := 1; g <= groupCount; g++ {
		standings, err := GetGroupStandings(db, tournamentID, g)
		if err != nil {
			return err
		}
		for i := 0; i < knockoutCount && i < len(standings); i++ {
			allQualified = append(allQualified, standings[i].TeamID)
		}
	}

	// 获取淘汰赛第一轮的比赛
	var knockoutMatches []models.Match
	err := db.Where("tournament_id = ? AND stage = ?", tournamentID, "knockout").
		Order("round_order asc").
		Find(&knockoutMatches).Error
	if err != nil {
		return err
	}

	if len(knockoutMatches) == 0 {
		return nil
	}

	// 找出第一轮的比赛（按Round排序，取前 N/2 场）
	firstRoundCount := len(allQualified) / 2
	if firstRoundCount > len(knockoutMatches) {
		firstRoundCount = len(knockoutMatches)
	}

	// 交叉对阵：小组第一 vs 另一组第二
	for i := 0; i < firstRoundCount && i < len(allQualified)/2; i++ {
		team1Idx := i * 2
		team2Idx := i*2 + 1
		if team2Idx < len(allQualified) && i < len(knockoutMatches) {
			t1 := allQualified[team1Idx]
			t2 := allQualified[team2Idx]
			knockoutMatches[i].Team1ID = &t1
			knockoutMatches[i].Team2ID = &t2
			if err := db.Save(&knockoutMatches[i]).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// AdvanceKnockout 淘汰赛晋级：比赛结束后，胜者进入下一轮。
func AdvanceKnockout(db *gorm.DB, match models.Match) error {
	if match.Stage != "knockout" || match.Status != "completed" || match.WinnerID == nil {
		return nil
	}

	// 获取该赛事所有淘汰赛
	var allMatches []models.Match
	err := db.Where("tournament_id = ? AND stage = ?", match.TournamentID, "knockout").
		Order("round_order asc").
		Find(&allMatches).Error
	if err != nil {
		return err
	}

	// 找到当前比赛的索引
	currentIdx := -1
	for i, m := range allMatches {
		if m.ID == match.ID {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return nil
	}

	// 找出下一轮对应的比赛
	// 简化处理：找到下一个Team1或Team2为0的后续比赛
	// 更准确的做法是按对阵树结构

	// 计算当前是第几轮
	currentRound := match.Round
	var nextRound string
	switch currentRound {
	case "round_of_16":
		nextRound = "quarterfinal"
	case "quarterfinal":
		nextRound = "semifinal"
	case "semifinal":
		nextRound = "final"
	default:
		return nil
	}

	// 找到下一轮对应的比赛位置
	var nextMatches []models.Match
	db.Where("tournament_id = ? AND stage = ? AND round = ?", match.TournamentID, "knockout", nextRound).
		Order("round_order asc").
		Find(&nextMatches)

	if len(nextMatches) == 0 {
		return nil
	}

	// 找出是当前轮次的第几场
	var currentRoundMatches []models.Match
	db.Where("tournament_id = ? AND stage = ? AND round = ?", match.TournamentID, "knockout", currentRound).
		Order("round_order asc").
		Find(&currentRoundMatches)

	matchPos := 0
	for i, m := range currentRoundMatches {
		if m.ID == match.ID {
			matchPos = i
			break
		}
	}

	// 胜者进入下一轮对应位置
	nextMatchIdx := matchPos / 2
	if nextMatchIdx < len(nextMatches) {
		nextMatch := nextMatches[nextMatchIdx]
		if matchPos%2 == 0 {
			// 上半区胜者 -> Team1
			if nextMatch.Team1ID == nil {
				nextMatch.Team1ID = match.WinnerID
			}
		} else {
			// 下半区胜者 -> Team2
			if nextMatch.Team2ID == nil {
				nextMatch.Team2ID = match.WinnerID
			}
		}
		db.Save(&nextMatch)
	}

	return nil
}

// CheckTournamentProgress 检查赛事进度状态。
func CheckTournamentProgress(db *gorm.DB, tournamentID uint) (string, error) {
	var matches []models.Match
	err := db.Where("tournament_id = ?", tournamentID).Find(&matches).Error
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "draft", nil
	}

	scheduledCount := 0
	completedCount := 0
	cancelledCount := 0
	for _, m := range matches {
		if m.Status == "scheduled" || m.Status == "ongoing" {
			scheduledCount++
		}
		if m.Status == "completed" {
			completedCount++
		}
		if m.Status == "cancelled" {
			cancelledCount++
		}
	}

	if scheduledCount == 0 && completedCount == 0 {
		return "draft", nil
	}

	totalActive := len(matches) - cancelledCount
	if completedCount >= totalActive && totalActive > 0 {
		return "completed", nil
	}

	if completedCount > 0 {
		return "ongoing", nil
	}

	return "scheduled", nil
}

// ---------- 统计 ----------

// TournamentStats 赛事统计。
type TournamentStats struct {
	TotalMatches    int     `json:"total_matches"`
	CompletedMatches int   `json:"completed_matches"`
	TotalVenues     int     `json:"total_venues"`
	TotalDays       int     `json:"total_days"`
	TotalTeams      int     `json:"total_teams"`
	TotalOccupancy  float64 `json:"total_occupancy_hours"`
}

// GetTournamentStats 获取赛事统计。
func GetTournamentStats(db *gorm.DB, tournamentID uint) (TournamentStats, error) {
	var stats TournamentStats

	var matches []models.Match
	db.Where("tournament_id = ?", tournamentID).Find(&matches)

	stats.TotalMatches = len(matches)

	venueSet := make(map[uint]bool)
	dateSet := make(map[string]bool)

	for _, m := range matches {
		if m.Status == "completed" {
			stats.CompletedMatches++
		}
		if m.VenueID > 0 && m.Status != "cancelled" && m.Status != "unscheduled" {
			venueSet[m.VenueID] = true
			dateSet[m.MatchDate] = true
			stats.TotalOccupancy += float64(m.EndHour - m.StartHour)
		}
	}

	stats.TotalVenues = len(venueSet)
	stats.TotalDays = len(dateSet)

	var teamCount int64
	db.Model(&models.TournamentTeam{}).Where("tournament_id = ?", tournamentID).Count(&teamCount)
	stats.TotalTeams = int(teamCount)

	return stats, nil
}

// VenueDailyOccupancy 场地某日占用情况。
type VenueDailyOccupancy struct {
	VenueID   uint   `json:"venue_id"`
	VenueName string `json:"venue_name"`
	Date      string `json:"date"`
	BookedHours float64 `json:"booked_hours"`
	TournamentHours float64 `json:"tournament_hours"`
	FreeHours float64 `json:"free_hours"`
	TotalHours float64 `json:"total_hours"`
}

// GetVenueCalendar 获取场地日历。
func GetVenueCalendar(db *gorm.DB, venueID uint, startDate, endDate string) ([]VenueDailyOccupancy, error) {
	var venue models.Venue
	if err := db.First(&venue, venueID).Error; err != nil {
		return nil, err
	}

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	var result []VenueDailyOccupancy
	totalHours := float64(venue.CloseHour - venue.OpenHour)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		var bookings []models.Booking
		db.Where("venue_id = ? AND book_date = ? AND status <> ?", venueID, dateStr, "cancelled").
			Find(&bookings)

		var occupancies []models.VenueOccupancy
		db.Where("venue_id = ? AND occupy_date = ? AND status = ?", venueID, dateStr, "active").
			Find(&occupancies)

		bookedHours := 0.0
		for _, b := range bookings {
			bookedHours += float64(b.EndHour - b.StartHour)
		}

		tournamentHours := 0.0
		for _, o := range occupancies {
			tournamentHours += float64(o.EndHour - o.StartHour)
		}

		// 注意：可能有重叠，这里简化处理
		result = append(result, VenueDailyOccupancy{
			VenueID:         venueID,
			VenueName:       venue.Name,
			Date:            dateStr,
			BookedHours:     bookedHours,
			TournamentHours: tournamentHours,
			FreeHours:       totalHours - bookedHours - tournamentHours,
			TotalHours:      totalHours,
		})
	}

	return result, nil
}
