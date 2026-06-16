package tournament

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"venue-booking-admin/internal/models"
)

// ---------- 辅助函数 ----------

func safeUintMatch(a, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// SchedulingConstraints 排程约束参数。
type SchedulingConstraints struct {
	TransitionBuff  int // 场地转换缓冲小时
	MaxMatchesPerDay int // 每片场地每天最大场次
	MatchDuration   int // 每场比赛时长（小时）
	StartHour       int // 每天开赛最早小时
	EndHour         int // 每天最晚结束小时
	NoBackToBack    bool // 是否禁止队伍背靠背连赛
}

// ScheduledMatch 待排场次。
type ScheduledMatch struct {
	ID          uint
	Team1ID     uint
	Team2ID     uint
	Stage       string
	Round       string
	RoundOrder  int
	GroupNo     int
	IsFinal     bool
}

// ScheduleResult 排程结果。
type ScheduleResult struct {
	Matches      []models.Match
	TotalDays    int
	TotalMatches int
	Conflicts    int
	Method       string // optimized / greedy
}

// ---------- 对阵生成 ----------

// GenerateRoundRobin 生成单循环对阵。
func GenerateRoundRobin(teamIDs []uint) []ScheduledMatch {
	n := len(teamIDs)
	if n < 2 {
		return nil
	}

	var matches []ScheduledMatch
	teams := make([]uint, len(teamIDs))
	copy(teams, teamIDs)

	// 轮转法：如果队伍数为奇数，添加一个轮空位
	if n%2 != 0 {
		teams = append(teams, 0)
		n++
	}
	half := n / 2

	matchIdx := 0
	for round := 0; round < n-1; round++ {
		for i := 0; i < half; i++ {
			t1 := teams[i]
			t2 := teams[n-1-i]
			if t1 != 0 && t2 != 0 {
				matches = append(matches, ScheduledMatch{
					Team1ID:    t1,
					Team2ID:    t2,
					Stage:      "group",
					Round:      fmt.Sprintf("round_%d", round+1),
					RoundOrder: matchIdx,
				})
				matchIdx++
			}
		}
		// 轮转：固定第一位，其余顺时针移动
		last := teams[n-1]
		for j := n - 1; j > 1; j-- {
			teams[j] = teams[j-1]
		}
		teams[1] = last
	}
	return matches
}

// GenerateKnockout 生成淘汰赛对阵（单败淘汰）。
func GenerateKnockout(teamIDs []uint) []ScheduledMatch {
	n := len(teamIDs)
	if n < 2 {
		return nil
	}

	// 计算下一个2的幂
	nextPow2 := 1
	for nextPow2 < n {
		nextPow2 *= 2
	}

	// 如果队伍数不是2的幂，前面的种子轮空直接晋级
	// 用种子对位法生成完整对阵树
	return generateFullKnockoutWithRounds(teamIDs, nextPow2)
}

func generateFullKnockoutWithRounds(teamIDs []uint, totalTeams int) []ScheduledMatch {
	// 生成完整的淘汰赛对阵树，后续轮次的队伍ID为0（待决出）
	var matches []ScheduledMatch
	matchIdx := 0

	numRounds := int(math.Log2(float64(totalTeams)))
	matchesPerRound := totalTeams / 2

	// 第一轮：按种子安排
	roundName := getRoundName(numRounds, 1)
	for i := 0; i < matchesPerRound; i++ {
		t1 := uint(0)
		t2 := uint(0)
		if i < len(teamIDs) {
			t1 = teamIDs[i]
		}
		opponentIdx := totalTeams - 1 - i
		if opponentIdx < len(teamIDs) {
			t2 = teamIDs[opponentIdx]
		}
		matches = append(matches, ScheduledMatch{
			Team1ID:    t1,
			Team2ID:    t2,
			Stage:      "knockout",
			Round:      roundName,
			RoundOrder: matchIdx,
		})
		matchIdx++
	}

	// 后续轮次：占位
	for round := 2; round <= numRounds; round++ {
		matchesPerRound = matchesPerRound / 2
		roundName = getRoundName(numRounds, round)
		isFinal := round == numRounds
		for i := 0; i < matchesPerRound; i++ {
			matches = append(matches, ScheduledMatch{
				Team1ID:    0,
				Team2ID:    0,
				Stage:      "knockout",
				Round:      roundName,
				RoundOrder: matchIdx,
				IsFinal:    isFinal,
			})
			matchIdx++
		}
	}

	return matches
}

func generateFullKnockoutBracket(teamIDs []uint, nextPow2 int) []ScheduledMatch {
	return generateFullKnockoutWithRounds(teamIDs, nextPow2)
}

func getRoundName(totalRounds, currentRound int) string {
	remainingRounds := totalRounds - currentRound + 1
	switch remainingRounds {
	case 1:
		return "final"
	case 2:
		return "semifinal"
	case 3:
		return "quarterfinal"
	case 4:
		return "round_of_16"
	default:
		return fmt.Sprintf("round_%d", currentRound)
	}
}

// ---------- 排程算法 ----------

// ScheduleOptimized 优化排程（考虑所有约束，尽量压缩总天数）。
// 使用贪心算法 + 最早可用时间优先。
func ScheduleOptimized(
	matches []ScheduledMatch,
	venues []models.Venue,
	startDate string,
	constraints SchedulingConstraints,
) (ScheduleResult, error) {
	result := ScheduleResult{
		Method:       "optimized",
		TotalMatches: len(matches),
	}

	// 解析开始日期
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return result, fmt.Errorf("日期格式错误: %v", err)
	}

	// 按轮次分组，先排小组赛再排淘汰赛
	// 同组比赛尽量分散，避免队伍连续比赛
	sortedMatches := make([]ScheduledMatch, len(matches))
	copy(sortedMatches, matches)

	// 按RoundOrder排序
	sort.Slice(sortedMatches, func(i, j int) bool {
		return sortedMatches[i].RoundOrder < sortedMatches[j].RoundOrder
	})

	// 场地每日使用跟踪
	// key: date+"_"+venueID
	type dayVenue struct {
		date     string
		venueID  uint
	}
	venueDayCount := make(map[dayVenue]int) // 每天每场地已安排场次
	venueSchedule := make(map[dayVenue][]timeSlot) // 每天每场地已安排时段
	teamLastMatchDate := make(map[uint]string) // 队伍上次比赛日期

	currentDate := start
	scheduledCount := 0

	// 对于每一场比赛，找到最早可用的场地和时段
	for _, match := range sortedMatches {
		scheduled := false
		daysTried := 0
		maxDaysToTry := 365

		// 占位赛（队伍未确定）跳过约束检查，直接安排最早可用时间
		isPlaceholder := match.Team1ID == 0 || match.Team2ID == 0

		log.Printf("DEBUG-SCHED: 处理比赛 round=%s team1=%d team2=%d isPlaceholder=%v", match.Round, match.Team1ID, match.Team2ID, isPlaceholder)

		for !scheduled && daysTried < maxDaysToTry {
			dateStr := currentDate.AddDate(0, 0, daysTried).Format("2006-01-02")

			// 遍历场地，找可用的
			for _, venue := range venues {
				// 检查当天该场地是否还能安排
				dv := dayVenue{date: dateStr, venueID: venue.ID}
				log.Printf("DEBUG-SCHED:   尝试场地=%d 日期=%s venueDayCount=%d MaxMatchesPerDay=%d isPlaceholder=%v", venue.ID, dateStr, venueDayCount[dv], constraints.MaxMatchesPerDay, isPlaceholder)
				if !isPlaceholder && constraints.MaxMatchesPerDay > 0 && venueDayCount[dv] >= constraints.MaxMatchesPerDay {
					log.Printf("DEBUG-SCHED:   跳过：场地容量已满")
					continue
				}

				// 计算当天可用的最早开始时间
				earliestStart := constraints.StartHour
				if earliestStart < venue.OpenHour {
					earliestStart = venue.OpenHour
				}

				// 按已安排的时段，找第一个可用的空当
				daySlots := venueSchedule[dv]
				sort.Slice(daySlots, func(i, j int) bool { return daySlots[i].start < daySlots[j].start })

				candidateStart := earliestStart

				for _, slot := range daySlots {
					log.Printf("DEBUG-SCHED:   已有时段 %d-%d", slot.start, slot.end)
					// 候选时间需要包含转换缓冲
					if candidateStart+constraints.MatchDuration+constraints.TransitionBuff <= slot.start {
						// 这个空当可以安排
						break
					}
					// 移到这场比赛结束 + 缓冲
					candidateStart = slot.end + constraints.TransitionBuff
				}

				// 检查是否在开放时间内
				candidateEnd := candidateStart + constraints.MatchDuration
				latestEnd := constraints.EndHour
				if latestEnd > venue.CloseHour {
					latestEnd = venue.CloseHour
				}
				log.Printf("DEBUG-SCHED:   candidateStart=%d candidateEnd=%d latestEnd=%d", candidateStart, candidateEnd, latestEnd)
				if candidateEnd > latestEnd {
					log.Printf("DEBUG-SCHED:   跳过：超出开放时间")
					continue
				}

				// 检查队伍是否背靠背（连续两天或同一天有比赛）
				// 占位赛不检查背靠背（队伍未确定）
				if !isPlaceholder && constraints.NoBackToBack {
					backToBack := false
					matchDate, _ := time.Parse("2006-01-02", dateStr)
					if match.Team1ID > 0 {
						if lastDate, ok := teamLastMatchDate[match.Team1ID]; ok {
							lastT, _ := time.Parse("2006-01-02", lastDate)
							diffDays := int(matchDate.Sub(lastT).Hours() / 24)
							if diffDays <= 1 {
								backToBack = true
							}
						}
					}
					if match.Team2ID > 0 && !backToBack {
						if lastDate, ok := teamLastMatchDate[match.Team2ID]; ok {
							lastT, _ := time.Parse("2006-01-02", lastDate)
							diffDays := int(matchDate.Sub(lastT).Hours() / 24)
							if diffDays <= 1 {
								backToBack = true
							}
						}
					}
					if backToBack {
						log.Printf("DEBUG-SCHED:   跳过：背靠背")
						continue
					}
				}

				log.Printf("DEBUG-SCHED:   安排成功！时段 %d-%d", candidateStart, candidateEnd)
				// 可以安排
				// 转换 uint 到 *uint，0 值转为 nil（避免外键约束）
				var t1ID *uint
				var t2ID *uint
				if match.Team1ID > 0 {
					t1ID = &match.Team1ID
				}
				if match.Team2ID > 0 {
					t2ID = &match.Team2ID
				}
				result.Matches = append(result.Matches, models.Match{
					ID:         match.ID,
					Team1ID:    t1ID,
					Team2ID:    t2ID,
					Stage:      match.Stage,
					Round:      match.Round,
					RoundOrder: match.RoundOrder,
					GroupNo:    match.GroupNo,
					IsFinal:    match.IsFinal,
					VenueID:    venue.ID,
					MatchDate:  dateStr,
					StartHour:  candidateStart,
					EndHour:    candidateEnd,
					Status:     "scheduled",
				})

				venueDayCount[dv]++
				venueSchedule[dv] = append(venueSchedule[dv], timeSlot{start: candidateStart, end: candidateEnd})

				if match.Team1ID > 0 {
					teamLastMatchDate[match.Team1ID] = dateStr
				}
				if match.Team2ID > 0 {
					teamLastMatchDate[match.Team2ID] = dateStr
				}

				scheduled = true
				scheduledCount++
				break
			}

			daysTried++
		}
	}

	// 计算总天数
	if len(result.Matches) > 0 {
		var firstDate, lastDate string
		firstDate = result.Matches[0].MatchDate
		lastDate = result.Matches[0].MatchDate
		for _, m := range result.Matches {
			if m.MatchDate < firstDate {
				firstDate = m.MatchDate
			}
			if m.MatchDate > lastDate {
				lastDate = m.MatchDate
			}
		}
		f, _ := time.Parse("2006-01-02", firstDate)
		l, _ := time.Parse("2006-01-02", lastDate)
		result.TotalDays = int(l.Sub(f).Hours()/24) + 1
	}

	result.Conflicts = len(matches) - scheduledCount
	return result, nil
}

type timeSlot struct {
	start int
	end   int
}

// ScheduleGreedy 顺序硬排（简单贪心，不做优化，作为对照组）。
func ScheduleGreedy(
	matches []ScheduledMatch,
	venues []models.Venue,
	startDate string,
	constraints SchedulingConstraints,
) (ScheduleResult, error) {
	result := ScheduleResult{
		Method:       "greedy",
		TotalMatches: len(matches),
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return result, fmt.Errorf("日期格式错误: %v", err)
	}

	if len(venues) == 0 {
		return result, fmt.Errorf("没有可用场地")
	}

	// 只用第一个场地，依次往后排，不考虑背靠背和优化
	venue := venues[0]
	currentDate := start
	currentHour := venue.OpenHour

	dayMatches := 0

	for _, match := range matches {
		// 检查是否超过当天关闭时间或当天最大场次
		latestEnd := venue.CloseHour
		if constraints.EndHour > 0 && constraints.EndHour < venue.CloseHour {
			latestEnd = constraints.EndHour
		}

		maxPerDay := constraints.MaxMatchesPerDay
		if maxPerDay == 0 {
			maxPerDay = 100
		}

		// 如果当天排不下，换到第二天
		for currentHour+constraints.MatchDuration > latestEnd || dayMatches >= maxPerDay {
			currentDate = currentDate.AddDate(0, 0, 1)
			currentHour = venue.OpenHour
			dayMatches = 0
		}

		dateStr := currentDate.Format("2006-01-02")
		// 转换 uint 到 *uint，0 值转为 nil（避免外键约束）
		var t1ID *uint
		var t2ID *uint
		if match.Team1ID > 0 {
			t1ID = &match.Team1ID
		}
		if match.Team2ID > 0 {
			t2ID = &match.Team2ID
		}
		result.Matches = append(result.Matches, models.Match{
			ID:         match.ID,
			Team1ID:    t1ID,
			Team2ID:    t2ID,
			Stage:      match.Stage,
			Round:      match.Round,
			RoundOrder: match.RoundOrder,
			GroupNo:    match.GroupNo,
			IsFinal:    match.IsFinal,
			VenueID:    venue.ID,
			MatchDate:  dateStr,
			StartHour:  currentHour,
			EndHour:    currentHour + constraints.MatchDuration,
			Status:     "scheduled",
		})

		currentHour += constraints.MatchDuration + constraints.TransitionBuff
		dayMatches++
	}

	if len(result.Matches) > 0 {
		f, _ := time.Parse("2006-01-02", result.Matches[0].MatchDate)
		l, _ := time.Parse("2006-01-02", result.Matches[len(result.Matches)-1].MatchDate)
		result.TotalDays = int(l.Sub(f).Hours()/24) + 1
	}

	return result, nil
}

// ValidateMatchConstraints 校验单场比赛的约束（用于手动调整时校验）。
func ValidateMatchConstraints(
	match models.Match,
	otherMatches []models.Match,
	venue models.Venue,
	constraints SchedulingConstraints,
) []string {
	var violations []string

	// 检查场馆开放时间
	if match.StartHour < venue.OpenHour || match.EndHour > venue.CloseHour {
		violations = append(violations, "比赛时段超出场馆开放时间")
	}

	// 检查每天最大场次
	dayCount := 0
	for _, m := range otherMatches {
		if m.VenueID == match.VenueID && m.MatchDate == match.MatchDate && m.ID != match.ID && m.Status != "cancelled" {
			dayCount++
		}
	}
	if constraints.MaxMatchesPerDay > 0 && dayCount >= constraints.MaxMatchesPerDay {
		violations = append(violations, "该场地当天已达最大场次")
	}

	// 检查场地时间重叠（含缓冲）
	for _, m := range otherMatches {
		if m.VenueID != match.VenueID || m.MatchDate != match.MatchDate || m.ID == match.ID || m.Status == "cancelled" {
			continue
		}
		// 带缓冲的重叠检查
		mStart := m.StartHour - constraints.TransitionBuff
		mEnd := m.EndHour + constraints.TransitionBuff
		if match.StartHour < mEnd && match.EndHour > mStart {
			violations = append(violations, fmt.Sprintf("与比赛 #%d 场地时间冲突（含转换缓冲）", m.ID))
		}
	}

	// 检查队伍背靠背（同一天或连续第二天）
	if constraints.NoBackToBack && len(otherMatches) > 0 {
		matchDate, _ := time.Parse("2006-01-02", match.MatchDate)
		team1 := match.Team1ID
		team2 := match.Team2ID

		for _, m := range otherMatches {
			if m.ID == match.ID || m.Status == "cancelled" {
				continue
			}
			if !safeUintMatch(m.Team1ID, team1) && !safeUintMatch(m.Team2ID, team1) &&
				!safeUintMatch(m.Team1ID, team2) && !safeUintMatch(m.Team2ID, team2) {
				continue
			}
			mDate, _ := time.Parse("2006-01-02", m.MatchDate)
			diffDays := int(matchDate.Sub(mDate).Hours() / 24)
			if diffDays < 0 {
				diffDays = -diffDays
			}
			if diffDays <= 1 {
				violations = append(violations, "存在队伍背靠背比赛")
				break
			}
		}
	}

	return violations
}
