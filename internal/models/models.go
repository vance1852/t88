package models

import "time"

// User 后台用户（本平台仅 admin 一个管理员角色）。
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	DisplayName  string    `gorm:"size:64" json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// Venue 体育场馆。
type Venue struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	SportType   string    `gorm:"size:32" json:"sport_type"` // basketball / football / badminton / swimming ...
	Capacity    int       `json:"capacity"`
	HourlyPrice float64   `json:"hourly_price"`
	OpenHour    int       `json:"open_hour"`  // 开放起始小时，0-23
	CloseHour   int       `json:"close_hour"` // 关闭小时，1-24
	Status      string    `gorm:"size:16" json:"status"` // open / closed / maintenance
	CreatedAt   time.Time `json:"created_at"`
}

// Booking 场地预订。
type Booking struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VenueID      uint      `gorm:"index" json:"venue_id"`
	CustomerName string    `gorm:"size:64" json:"customer_name"`
	Phone        string    `gorm:"size:32" json:"phone"`
	BookDate     string    `gorm:"size:10;index" json:"book_date"` // YYYY-MM-DD
	StartHour    int       `json:"start_hour"`
	EndHour      int       `json:"end_hour"`
	Amount       float64   `json:"amount"`
	Status       string    `gorm:"size:16" json:"status"` // booked / cancelled / completed
	CreatedAt    time.Time `json:"created_at"`
}

// ---------- 赛事相关 ----------

// Tournament 赛事。
type Tournament struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:128;index" json:"name"`
	SportType       string    `gorm:"size:32" json:"sport_type"`
	Format          string    `gorm:"size:32" json:"format"`           // round_robin | knockout | group_knockout
	StartDate       string    `gorm:"size:10;index" json:"start_date"` // YYYY-MM-DD
	EndDate         string    `gorm:"size:10;index" json:"end_date"`
	GroupCount      int       `json:"group_count"`       // 小组数量（小组+淘汰制）
	KnockoutCount   int       `json:"knockout_count"`    // 晋级淘汰的队伍数
	MatchDuration   int       `json:"match_duration"`    // 每场比赛时长（小时）
	TransitionBuff  int       `json:"transition_buff"`   // 场地转换缓冲（小时）
	MaxMatchesPerDay int      `json:"max_matches_per_day"` // 每片场地每天最大场次
	Status          string    `gorm:"size:16" json:"status"` // draft / scheduled / ongoing / completed / cancelled
	Description     string    `gorm:"size:512" json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

// Team 参赛队伍/选手。
type Team struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	Coach     string    `gorm:"size:64" json:"coach"`
	Contact   string    `gorm:"size:32" json:"contact"`
	SportType string    `gorm:"size:32" json:"sport_type"`
	CreatedAt time.Time `json:"created_at"`
}

// TournamentTeam 赛事-队伍关联。
type TournamentTeam struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TournamentID uint      `gorm:"index;not null" json:"tournament_id"`
	TeamID       uint      `gorm:"index;not null" json:"team_id"`
	GroupNo      int       `json:"group_no"` // 小组编号（0表示无）
	SeedPosition int       `json:"seed_position"` // 种子位次
	Team         Team      `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Match 比赛场次。
type Match struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TournamentID  uint      `gorm:"index;not null" json:"tournament_id"`
	Round         string    `gorm:"size:32" json:"round"`      // 轮次：group / quarterfinal / semifinal / final / 3rd_place
	RoundOrder    int       `json:"round_order"`               // 轮次内排序
	Stage         string    `gorm:"size:16" json:"stage"`      // group | knockout
	GroupNo       int       `json:"group_no"`                  // 小组编号
	Team1ID       uint      `json:"team1_id"`
	Team2ID       uint      `json:"team2_id"`
	Team1Score    int       `json:"team1_score"`
	Team2Score    int       `json:"team2_score"`
	VenueID       uint      `gorm:"index" json:"venue_id"`
	MatchDate     string    `gorm:"size:10;index" json:"match_date"` // YYYY-MM-DD
	StartHour     int       `json:"start_hour"`
	EndHour       int       `json:"end_hour"`
	Status        string    `gorm:"size:16" json:"status"` // unscheduled | scheduled | ongoing | completed | cancelled
	IsFinal       bool      `json:"is_final"`
	WinnerID      uint      `json:"winner_id"`
	Notes         string    `gorm:"size:256" json:"notes"`
	Team1         Team      `gorm:"foreignKey:Team1ID" json:"team1,omitempty"`
	Team2         Team      `gorm:"foreignKey:Team2ID" json:"team2,omitempty"`
	Venue         Venue     `gorm:"foreignKey:VenueID" json:"venue,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GroupStanding 小组积分榜。
type GroupStanding struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	TournamentID  uint   `gorm:"index;not null" json:"tournament_id"`
	GroupNo       int    `json:"group_no"`
	TeamID        uint   `gorm:"index;not null" json:"team_id"`
	Played        int    `json:"played"`
	Wins          int    `json:"wins"`
	Draws         int    `json:"draws"`
	Losses        int    `json:"losses"`
	Points        int    `json:"points"`
	GoalsFor      int    `json:"goals_for"`
	GoalsAgainst  int    `json:"goals_against"`
	GoalDiff      int    `json:"goal_diff"`
	Team          Team   `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

// VenueOccupancy 场地占用记录（赛事占用时段，用于与散客库存协调）。
type VenueOccupancy struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VenueID      uint      `gorm:"index;not null" json:"venue_id"`
	TournamentID uint      `gorm:"index" json:"tournament_id"`
	MatchID      uint      `gorm:"index" json:"match_id"`
	OccupyDate   string    `gorm:"size:10;index" json:"occupy_date"` // YYYY-MM-DD
	StartHour    int       `json:"start_hour"`
	EndHour      int       `json:"end_hour"`
	SourceType   string    `gorm:"size:16" json:"source_type"` // tournament | blocking
	Status       string    `gorm:"size:16" json:"status"`      // active / released
	Remarks      string    `gorm:"size:256" json:"remarks"`
	CreatedAt    time.Time `json:"created_at"`
}

// BookingConflict 赛事与散客预订冲突记录。
type BookingConflict struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TournamentID  uint      `gorm:"index;not null" json:"tournament_id"`
	MatchID       uint      `gorm:"index" json:"match_id"`
	BookingID     uint      `gorm:"index;not null" json:"booking_id"`
	VenueID       uint      `gorm:"index;not null" json:"venue_id"`
	ConflictDate  string    `gorm:"size:10" json:"conflict_date"`
	ConflictStart int       `json:"conflict_start"`
	ConflictEnd   int       `json:"conflict_end"`
	HandleStatus  string    `gorm:"size:16" json:"handle_status"` // pending | rescheduled | cancelled | resolved
	HandleMethod  string    `gorm:"size:16" json:"handle_method"` // reschedule | cancel | noop
	Remarks       string    `gorm:"size:256" json:"remarks"`
	Booking       Booking   `gorm:"foreignKey:BookingID" json:"booking,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
