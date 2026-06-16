package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/models"
	"venue-booking-admin/internal/tournament"
)

// ---------- 赛事管理 ----------

type tournamentReq struct {
	Name            string `json:"name" binding:"required"`
	SportType       string `json:"sport_type"`
	Format          string `json:"format" binding:"required"` // round_robin | knockout | group_knockout
	StartDate       string `json:"start_date" binding:"required"`
	EndDate         string `json:"end_date"`
	GroupCount      int    `json:"group_count"`
	KnockoutCount   int    `json:"knockout_count"`
	MatchDuration   int    `json:"match_duration"`
	TransitionBuff  int    `json:"transition_buff"`
	MaxMatchesPerDay int   `json:"max_matches_per_day"`
	Description     string `json:"description"`
}

func (h *Handler) ListTournaments(c *gin.Context) {
	var tournaments []models.Tournament
	q := h.DB.Order("id desc")
	if sport := c.Query("sport_type"); sport != "" {
		q = q.Where("sport_type = ?", sport)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&tournaments)
	c.JSON(http.StatusOK, tournaments)
}

func (h *Handler) CreateTournament(c *gin.Context) {
	var req tournamentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	if req.MatchDuration <= 0 {
		req.MatchDuration = 2
	}

	t := models.Tournament{
		Name:             req.Name,
		SportType:        req.SportType,
		Format:           req.Format,
		StartDate:        req.StartDate,
		EndDate:          req.EndDate,
		GroupCount:       req.GroupCount,
		KnockoutCount:    req.KnockoutCount,
		MatchDuration:    req.MatchDuration,
		TransitionBuff:   req.TransitionBuff,
		MaxMatchesPerDay: req.MaxMatchesPerDay,
		Status:           "draft",
		Description:      req.Description,
	}
	h.DB.Create(&t)
	c.JSON(http.StatusCreated, t)
}

func (h *Handler) GetTournament(c *gin.Context) {
	var t models.Tournament
	if err := h.DB.First(&t, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "赛事不存在"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handler) UpdateTournament(c *gin.Context) {
	var t models.Tournament
	if err := h.DB.First(&t, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "赛事不存在"})
		return
	}

	var req tournamentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	t.Name = req.Name
	t.SportType = req.SportType
	t.Format = req.Format
	t.StartDate = req.StartDate
	t.EndDate = req.EndDate
	t.GroupCount = req.GroupCount
	t.KnockoutCount = req.KnockoutCount
	t.MatchDuration = req.MatchDuration
	t.TransitionBuff = req.TransitionBuff
	t.MaxMatchesPerDay = req.MaxMatchesPerDay
	t.Description = req.Description

	h.DB.Save(&t)
	c.JSON(http.StatusOK, t)
}

func (h *Handler) DeleteTournament(c *gin.Context) {
	var t models.Tournament
	if err := h.DB.First(&t, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "赛事不存在"})
		return
	}

	tx := h.DB.Begin()
	tx.Where("tournament_id = ?", t.ID).Delete(&models.Match{})
	tx.Where("tournament_id = ?", t.ID).Delete(&models.TournamentTeam{})
	tx.Where("tournament_id = ?", t.ID).Delete(&models.GroupStanding{})
	tx.Where("tournament_id = ?", t.ID).Delete(&models.VenueOccupancy{})
	tx.Where("tournament_id = ?", t.ID).Delete(&models.BookingConflict{})
	tx.Delete(&t)
	tx.Commit()

	c.Status(http.StatusNoContent)
}

// ---------- 队伍管理 ----------

type teamReq struct {
	Name      string `json:"name" binding:"required"`
	Coach     string `json:"coach"`
	Contact   string `json:"contact"`
	SportType string `json:"sport_type"`
}

func (h *Handler) ListTeams(c *gin.Context) {
	var teams []models.Team
	q := h.DB.Order("id desc")
	if sport := c.Query("sport_type"); sport != "" {
		q = q.Where("sport_type = ?", sport)
	}
	q.Find(&teams)
	c.JSON(http.StatusOK, teams)
}

func (h *Handler) CreateTeam(c *gin.Context) {
	var req teamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	team := models.Team{
		Name:      req.Name,
		Coach:     req.Coach,
		Contact:   req.Contact,
		SportType: req.SportType,
	}
	h.DB.Create(&team)
	c.JSON(http.StatusCreated, team)
}

func (h *Handler) UpdateTeam(c *gin.Context) {
	var team models.Team
	if err := h.DB.First(&team, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "队伍不存在"})
		return
	}
	var req teamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	team.Name = req.Name
	team.Coach = req.Coach
	team.Contact = req.Contact
	team.SportType = req.SportType
	h.DB.Save(&team)
	c.JSON(http.StatusOK, team)
}

func (h *Handler) DeleteTeam(c *gin.Context) {
	h.DB.Delete(&models.Team{}, c.Param("id"))
	c.Status(http.StatusNoContent)
}

// ---------- 赛事队伍管理 ----------

type addTeamReq struct {
	TeamIDs      []uint `json:"team_ids" binding:"required"`
	GroupNo      int    `json:"group_no"`
	SeedPosition int    `json:"seed_position"`
}

func (h *Handler) GetTournamentTeams(c *gin.Context) {
	tournamentID := c.Param("id")
	var tt []models.TournamentTeam
	h.DB.Where("tournament_id = ?", tournamentID).
		Preload("Team").
		Order("group_no, seed_position").
		Find(&tt)
	c.JSON(http.StatusOK, tt)
}

func (h *Handler) AddTeamsToTournament(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))
	var req addTeamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	tx := h.DB.Begin()
	for i, tid := range req.TeamIDs {
		tt := models.TournamentTeam{
			TournamentID: uint(tournamentID),
			TeamID:       tid,
			GroupNo:      req.GroupNo,
			SeedPosition: req.SeedPosition + i,
		}
		tx.Create(&tt)
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"added": len(req.TeamIDs)})
}

func (h *Handler) RemoveTeamFromTournament(c *gin.Context) {
	tournamentID := c.Param("id")
	teamID := c.Param("team_id")
	h.DB.Where("tournament_id = ? AND team_id = ?", tournamentID, teamID).
		Delete(&models.TournamentTeam{})
	c.Status(http.StatusNoContent)
}

// ---------- 赛程编排 ----------

type scheduleReq struct {
	VenueIDs      []uint `json:"venue_ids" binding:"required"`
	StartDate     string `json:"start_date"`
	Method        string `json:"method"` // optimized | greedy | compare
	NoBackToBack  bool   `json:"no_back_to_back"`
	StartHour     int    `json:"start_hour"`
	EndHour       int    `json:"end_hour"`
}

func (h *Handler) GenerateSchedule(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))

	var t models.Tournament
	if err := h.DB.First(&t, tournamentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "赛事不存在"})
		return
	}

	var req scheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	// 获取赛事队伍
	var tt []models.TournamentTeam
	h.DB.Where("tournament_id = ?", tournamentID).Order("seed_position").Find(&tt)
	if len(tt) < 2 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "参赛队伍不足"})
		return
	}

	// 获取场地
	var venues []models.Venue
	if len(req.VenueIDs) > 0 {
		h.DB.Where("id IN ? AND status = ?", req.VenueIDs, "open").Find(&venues)
	}
	if len(venues) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "没有可用场地"})
		return
	}

	// 生成对阵
	var scheduledMatches []tournament.ScheduledMatch

	if t.Format == "round_robin" || t.Format == "group_knockout" {
		// 单循环或小组赛
		teamIDs := make([]uint, 0)
		for _, team := range tt {
			teamIDs = append(teamIDs, team.TeamID)
		}
		scheduledMatches = tournament.GenerateRoundRobin(teamIDs)
		for i := range scheduledMatches {
			scheduledMatches[i].Stage = "group"
			scheduledMatches[i].GroupNo = 1
		}
	}

	if t.Format == "knockout" || t.Format == "group_knockout" {
		// 淘汰赛
		teamIDs := make([]uint, 0)
		for _, team := range tt {
			teamIDs = append(teamIDs, team.TeamID)
		}
		knockoutMatches := tournament.GenerateKnockout(teamIDs)
		// 调整 round_order
		baseOrder := len(scheduledMatches)
		for i := range knockoutMatches {
			knockoutMatches[i].RoundOrder = baseOrder + i
		}
		scheduledMatches = append(scheduledMatches, knockoutMatches...)
	}

	startDate := req.StartDate
	if startDate == "" {
		startDate = t.StartDate
	}

	constraints := tournament.SchedulingConstraints{
		TransitionBuff:    t.TransitionBuff,
		MaxMatchesPerDay:  t.MaxMatchesPerDay,
		MatchDuration:     t.MatchDuration,
		StartHour:         req.StartHour,
		EndHour:           req.EndHour,
		NoBackToBack:      req.NoBackToBack,
	}

	method := req.Method
	if method == "" {
		method = "optimized"
	}

	if method == "compare" {
		// 对比两种排程方式
		optResult, err := tournament.ScheduleOptimized(scheduledMatches, venues, startDate, constraints)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		greedyResult, err := tournament.ScheduleGreedy(scheduledMatches, venues, startDate, constraints)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"optimized": gin.H{
				"total_days":     optResult.TotalDays,
				"total_matches":  optResult.TotalMatches,
				"conflicts":      optResult.Conflicts,
				"matches_count":  len(optResult.Matches),
			},
			"greedy": gin.H{
				"total_days":     greedyResult.TotalDays,
				"total_matches":  greedyResult.TotalMatches,
				"conflicts":      greedyResult.Conflicts,
				"matches_count":  len(greedyResult.Matches),
			},
			"day_reduction":  greedyResult.TotalDays - optResult.TotalDays,
		})
		return
	}

	var result tournament.ScheduleResult
	var err error

	if method == "greedy" {
		result, err = tournament.ScheduleGreedy(scheduledMatches, venues, startDate, constraints)
	} else {
		result, err = tournament.ScheduleOptimized(scheduledMatches, venues, startDate, constraints)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	// 保存到数据库
	tx := h.DB.Begin()

	// 删除旧的赛程
	tx.Where("tournament_id = ?", tournamentID).Delete(&models.Match{})
	tx.Where("tournament_id = ?", tournamentID).Delete(&models.VenueOccupancy{})
	tx.Where("tournament_id = ?", tournamentID).Delete(&models.GroupStanding{})

	for _, m := range result.Matches {
		match := models.Match{
			TournamentID: uint(tournamentID),
			Round:        m.Round,
			RoundOrder:   m.RoundOrder,
			Stage:        m.Stage,
			GroupNo:      m.GroupNo,
			Team1ID:      m.Team1ID,
			Team2ID:      m.Team2ID,
			VenueID:      m.VenueID,
			MatchDate:    m.MatchDate,
			StartHour:    m.StartHour,
			EndHour:      m.EndHour,
			Status:       "scheduled",
			IsFinal:      m.IsFinal,
		}
		tx.Create(&match)

		if m.VenueID > 0 {
			occ := models.VenueOccupancy{
				VenueID:      m.VenueID,
				TournamentID: uint(tournamentID),
				MatchID:      match.ID,
				OccupyDate:   m.MatchDate,
				StartHour:    m.StartHour,
				EndHour:      m.EndHour,
				SourceType:   "tournament",
				Status:       "active",
				Remarks:      t.Name + " - 第" + m.Round + "轮",
			}
			tx.Create(&occ)
		}
	}

	// 更新赛事状态
	t.Status = "scheduled"
	tx.Save(&t)

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"total_days":    result.TotalDays,
		"total_matches": result.TotalMatches,
		"method":        result.Method,
		"conflicts":     result.Conflicts,
	})
}

// ---------- 赛程查询 ----------

func (h *Handler) ListMatches(c *gin.Context) {
	var matches []models.Match
	q := h.DB.Order("match_date, start_hour, venue_id")

	if tid := c.Query("tournament_id"); tid != "" {
		q = q.Where("tournament_id = ?", tid)
	}
	if vid := c.Query("venue_id"); vid != "" {
		q = q.Where("venue_id = ?", vid)
	}
	if date := c.Query("date"); date != "" {
		q = q.Where("match_date = ?", date)
	}
	if teamID := c.Query("team_id"); teamID != "" {
		q = q.Where("team1_id = ? OR team2_id = ?", teamID, teamID)
	}
	if stage := c.Query("stage"); stage != "" {
		q = q.Where("stage = ?", stage)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}

	q.Preload("Team1").Preload("Team2").Preload("Venue").Find(&matches)
	c.JSON(http.StatusOK, matches)
}

func (h *Handler) GetMatch(c *gin.Context) {
	var match models.Match
	if err := h.DB.Preload("Team1").Preload("Team2").Preload("Venue").
		First(&match, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "比赛不存在"})
		return
	}
	c.JSON(http.StatusOK, match)
}

// ---------- 手动调整赛程 ----------

type updateMatchReq struct {
	VenueID   uint   `json:"venue_id"`
	MatchDate string `json:"match_date"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	Validate  bool   `json:"validate"`
}

func (h *Handler) UpdateMatch(c *gin.Context) {
	var match models.Match
	if err := h.DB.First(&match, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "比赛不存在"})
		return
	}

	var req updateMatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	// 获取赛事信息以获取约束参数
	var t models.Tournament
	h.DB.First(&t, match.TournamentID)

	// 获取同赛事的其他比赛
	var otherMatches []models.Match
	h.DB.Where("tournament_id = ?", match.TournamentID).Find(&otherMatches)

	// 获取场地
	var venue models.Venue
	if err := h.DB.First(&venue, req.VenueID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "场地不存在"})
		return
	}

	endHour := req.EndHour
	if endHour == 0 {
		endHour = req.StartHour + t.MatchDuration
	}

	// 构建待校验的比赛
	testMatch := match
	testMatch.VenueID = req.VenueID
	testMatch.MatchDate = req.MatchDate
	testMatch.StartHour = req.StartHour
	testMatch.EndHour = endHour

	constraints := tournament.SchedulingConstraints{
		TransitionBuff:    t.TransitionBuff,
		MaxMatchesPerDay:  t.MaxMatchesPerDay,
		MatchDuration:     t.MatchDuration,
		NoBackToBack:      true,
	}

	violations := tournament.ValidateMatchConstraints(testMatch, otherMatches, venue, constraints)

	if req.Validate && len(violations) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"violations": violations,
			"detail":     "存在约束冲突",
		})
		return
	}

	// 更新比赛
	match.VenueID = req.VenueID
	match.MatchDate = req.MatchDate
	match.StartHour = req.StartHour
	match.EndHour = endHour
	if match.Status == "unscheduled" {
		match.Status = "scheduled"
	}

	tx := h.DB.Begin()
	tx.Save(&match)

	// 更新场地占用记录
	tx.Where("match_id = ?", match.ID).Delete(&models.VenueOccupancy{})
	if match.VenueID > 0 && match.Status != "cancelled" {
		occ := models.VenueOccupancy{
			VenueID:      match.VenueID,
			TournamentID: match.TournamentID,
			MatchID:      match.ID,
			OccupyDate:   match.MatchDate,
			StartHour:    match.StartHour,
			EndHour:      match.EndHour,
			SourceType:   "tournament",
			Status:       "active",
			Remarks:      "赛事占用",
		}
		tx.Create(&occ)
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"match":      match,
		"violations": violations,
	})
}

// ---------- 比分录入与晋级 ----------

type scoreReq struct {
	Team1Score int `json:"team1_score" binding:"required"`
	Team2Score int `json:"team2_score" binding:"required"`
}

func (h *Handler) RecordScore(c *gin.Context) {
	var match models.Match
	if err := h.DB.First(&match, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "比赛不存在"})
		return
	}

	var req scoreReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	match.Team1Score = req.Team1Score
	match.Team2Score = req.Team2Score
	match.Status = "completed"

	// 计算胜者
	if req.Team1Score > req.Team2Score {
		match.WinnerID = match.Team1ID
	} else if req.Team2Score > req.Team1Score {
		match.WinnerID = match.Team2ID
	}

	tx := h.DB.Begin()
	tx.Save(&match)

	// 更新小组积分
	if match.Stage == "group" {
		tournament.UpdateStandings(tx, match)
	}

	// 淘汰赛晋级
	if match.Stage == "knockout" {
		tournament.AdvanceKnockout(tx, match)
	}

	tx.Commit()

	// 检查赛事进度
	status, _ := tournament.CheckTournamentProgress(h.DB, match.TournamentID)
	if status != "" {
		h.DB.Model(&models.Tournament{}).Where("id = ?", match.TournamentID).Update("status", status)
	}

	c.JSON(http.StatusOK, match)
}

// ---------- 小组积分榜 ----------

func (h *Handler) GetGroupStandings(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))
	groupNo, _ := strconv.Atoi(c.DefaultQuery("group_no", "1"))

	standings, err := tournament.GetGroupStandings(h.DB, uint(tournamentID), groupNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, standings)
}

// ---------- 场地占用与资源协调 ----------

func (h *Handler) GetVenueOccupancy(c *gin.Context) {
	venueID, _ := strconv.Atoi(c.Query("venue_id"))
	date := c.Query("date")

	var occupancies []models.VenueOccupancy
	q := h.DB.Where("status = ?", "active")
	if venueID > 0 {
		q = q.Where("venue_id = ?", venueID)
	}
	if date != "" {
		q = q.Where("occupy_date = ?", date)
	}
	q.Find(&occupancies)

	c.JSON(http.StatusOK, occupancies)
}

func (h *Handler) GetVenueAvailableSlots(c *gin.Context) {
	venueID, _ := strconv.Atoi(c.Param("id"))
	date := c.Query("date")

	slots, err := tournament.GetAvailableSlots(h.DB, uint(venueID), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, slots)
}

func (h *Handler) GetBookingConflicts(c *gin.Context) {
	tournamentID := c.Query("tournament_id")

	var conflicts []models.BookingConflict
	q := h.DB.Preload("Booking").Order("id desc")
	if tournamentID != "" {
		q = q.Where("tournament_id = ?", tournamentID)
	}
	q.Find(&conflicts)

	c.JSON(http.StatusOK, conflicts)
}

type resolveConflictReq struct {
	Method  string `json:"method" binding:"required"` // cancel | reschedule
	Remarks string `json:"remarks"`
}

func (h *Handler) ResolveConflict(c *gin.Context) {
	conflictID, _ := strconv.Atoi(c.Param("id"))

	var req resolveConflictReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	if err := tournament.ResolveConflict(h.DB, uint(conflictID), req.Method, req.Remarks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DetectConflicts(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))

	var matches []models.Match
	h.DB.Where("tournament_id = ?", tournamentID).Find(&matches)

	conflicts, err := tournament.DetectBookingConflicts(h.DB, uint(tournamentID), matches)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	// 保存新检测到的冲突
	for i := range conflicts {
		h.DB.Create(&conflicts[i])
	}

	// 重新查询带预加载的冲突
	var allConflicts []models.BookingConflict
	h.DB.Preload("Booking").Where("tournament_id = ?", tournamentID).Order("id desc").Find(&allConflicts)

	c.JSON(http.StatusOK, gin.H{
		"new_conflicts": len(conflicts),
		"conflicts":     allConflicts,
	})
}

// ---------- 统计 ----------

func (h *Handler) GetTournamentStats(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))

	stats, err := tournament.GetTournamentStats(h.DB, uint(tournamentID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) GetVenueCalendar(c *gin.Context) {
	venueID, _ := strconv.Atoi(c.Param("id"))
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	data, err := tournament.GetVenueCalendar(h.DB, uint(venueID), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ---------- 赛程冲突报告 ----------

func (h *Handler) GetScheduleConflictReport(c *gin.Context) {
	tournamentID, _ := strconv.Atoi(c.Param("id"))

	var matches []models.Match
	h.DB.Where("tournament_id = ?", tournamentID).Find(&matches)

	var t models.Tournament
	h.DB.First(&t, tournamentID)

	constraints := tournament.SchedulingConstraints{
		TransitionBuff:   t.TransitionBuff,
		MaxMatchesPerDay: t.MaxMatchesPerDay,
		MatchDuration:    t.MatchDuration,
		NoBackToBack:     true,
	}

	// 检查所有比赛之间的冲突
	var venueConflicts []gin.H
	var teamConflicts []gin.H

	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			m1 := matches[i]
			m2 := matches[j]

			if m1.VenueID == m2.VenueID && m1.MatchDate == m2.MatchDate {
				m1Start := m1.StartHour - constraints.TransitionBuff
				m1End := m1.EndHour + constraints.TransitionBuff
				if m1Start < m2.EndHour && m1End > m2.StartHour {
					venueConflicts = append(venueConflicts, gin.H{
						"match1_id": m1.ID,
						"match2_id": m2.ID,
						"type":      "venue_overlap",
					})
				}
			}

			// 检查队伍背靠背
			if m1.Team1ID == m2.Team1ID || m1.Team1ID == m2.Team2ID ||
				m1.Team2ID == m2.Team1ID || m1.Team2ID == m2.Team2ID {
				d1, _ := parseDate(m1.MatchDate)
				d2, _ := parseDate(m2.MatchDate)
				diff := d1.Sub(d2).Hours()
				if diff < 0 {
					diff = -diff
				}
				if diff < 24 && diff > 0 {
					teamConflicts = append(teamConflicts, gin.H{
						"match1_id": m1.ID,
						"match2_id": m2.ID,
						"type":      "back_to_back",
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"venue_conflicts": venueConflicts,
		"team_conflicts":  teamConflicts,
		"total_venue_conflicts": len(venueConflicts),
		"total_team_conflicts":  len(teamConflicts),
	})
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
