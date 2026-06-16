package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/config"
	"venue-booking-admin/internal/db"
	"venue-booking-admin/internal/handlers"
	"venue-booking-admin/internal/seed"
)

func main() {
	cfg := config.Load()
	auth.SetSecret(cfg.JWTSecret)

	database, err := db.Connect(cfg.DSN)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err := seed.Run(database, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("种子数据初始化失败: %v", err)
	}

	h := &handlers.Handler{DB: database}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.POST("/auth/login", h.Login)

		secured := api.Group("")
		secured.Use(auth.Middleware(database))
		{
			secured.GET("/auth/me", h.Me)

			secured.GET("/venues", h.ListVenues)
			secured.POST("/venues", h.CreateVenue)
			secured.GET("/venues/:id", h.GetVenue)
			secured.PUT("/venues/:id", h.UpdateVenue)
			secured.DELETE("/venues/:id", h.DeleteVenue)
			secured.GET("/venues/:id/available-slots", h.GetVenueAvailableSlots)
			secured.GET("/venues/:id/calendar", h.GetVenueCalendar)

			secured.GET("/bookings", h.ListBookings)
			secured.POST("/bookings", h.CreateBooking)
			secured.PATCH("/bookings/:id/status", h.UpdateBookingStatus)

			secured.GET("/dashboard/stats", h.DashboardStats)

			// 队伍管理
			secured.GET("/teams", h.ListTeams)
			secured.POST("/teams", h.CreateTeam)
			secured.PUT("/teams/:id", h.UpdateTeam)
			secured.DELETE("/teams/:id", h.DeleteTeam)

			// 赛事管理
			secured.GET("/tournaments", h.ListTournaments)
			secured.POST("/tournaments", h.CreateTournament)
			secured.GET("/tournaments/:id", h.GetTournament)
			secured.PUT("/tournaments/:id", h.UpdateTournament)
			secured.DELETE("/tournaments/:id", h.DeleteTournament)

			// 赛事队伍
			secured.GET("/tournaments/:id/teams", h.GetTournamentTeams)
			secured.POST("/tournaments/:id/teams", h.AddTeamsToTournament)
			secured.DELETE("/tournaments/:id/teams/:team_id", h.RemoveTeamFromTournament)

			// 赛程编排
			secured.POST("/tournaments/:id/generate-schedule", h.GenerateSchedule)
			secured.GET("/tournaments/:id/standings", h.GetGroupStandings)
			secured.GET("/tournaments/:id/stats", h.GetTournamentStats)
			secured.GET("/tournaments/:id/conflict-report", h.GetScheduleConflictReport)
			secured.POST("/tournaments/:id/detect-conflicts", h.DetectConflicts)

			// 比赛管理
			secured.GET("/matches", h.ListMatches)
			secured.GET("/matches/:id", h.GetMatch)
			secured.PUT("/matches/:id", h.UpdateMatch)
			secured.POST("/matches/:id/score", h.RecordScore)

			// 场地占用与冲突
			secured.GET("/venue-occupancies", h.GetVenueOccupancy)
			secured.GET("/booking-conflicts", h.GetBookingConflicts)
			secured.POST("/booking-conflicts/:id/resolve", h.ResolveConflict)
		}
	}

	log.Printf("venue-booking-admin listening on :%s", cfg.Port)
	if err := r.Run("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
