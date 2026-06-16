package seed

import (
	"log"

	"gorm.io/gorm"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/models"
)

// Run 初始化内置管理员与种子业务数据（幂等）。
func Run(database *gorm.DB, adminUser, adminPass string) error {
	var count int64
	database.Model(&models.User{}).Where("username = ?", adminUser).Count(&count)
	if count == 0 {
		hash, err := auth.HashPassword(adminPass)
		if err != nil {
			return err
		}
		database.Create(&models.User{Username: adminUser, PasswordHash: hash, DisplayName: "平台管理员"})
		log.Println("已创建管理员账号")
	}

	var venueCount int64
	database.Model(&models.Venue{}).Count(&venueCount)
	if venueCount == 0 {
		venues := []models.Venue{
			{Name: "城北全民健身中心篮球馆", SportType: "basketball", Capacity: 200, HourlyPrice: 160, OpenHour: 8, CloseHour: 22, Status: "open"},
			{Name: "奥体中心游泳馆", SportType: "swimming", Capacity: 400, HourlyPrice: 80, OpenHour: 6, CloseHour: 21, Status: "open"},
			{Name: "市民广场羽毛球馆", SportType: "badminton", Capacity: 60, HourlyPrice: 50, OpenHour: 9, CloseHour: 22, Status: "maintenance"},
			{Name: "滨江足球公园", SportType: "football", Capacity: 500, HourlyPrice: 300, OpenHour: 8, CloseHour: 20, Status: "open"},
			{Name: "城东体育中心篮球馆A场", SportType: "basketball", Capacity: 150, HourlyPrice: 200, OpenHour: 8, CloseHour: 22, Status: "open"},
			{Name: "城东体育中心篮球馆B场", SportType: "basketball", Capacity: 150, HourlyPrice: 200, OpenHour: 8, CloseHour: 22, Status: "open"},
			{Name: "城西羽毛球馆1号场", SportType: "badminton", Capacity: 20, HourlyPrice: 60, OpenHour: 9, CloseHour: 22, Status: "open"},
			{Name: "城西羽毛球馆2号场", SportType: "badminton", Capacity: 20, HourlyPrice: 60, OpenHour: 9, CloseHour: 22, Status: "open"},
		}
		if err := database.Create(&venues).Error; err != nil {
			return err
		}
		log.Println("已创建种子场馆数据")
	}

	var teamCount int64
	database.Model(&models.Team{}).Count(&teamCount)
	if teamCount == 0 {
		teams := []models.Team{
			{Name: "飞鹰篮球队", Coach: "李明", Contact: "13800000001", SportType: "basketball"},
			{Name: "猛虎篮球队", Coach: "王强", Contact: "13800000002", SportType: "basketball"},
			{Name: "猎豹篮球队", Coach: "张伟", Contact: "13800000003", SportType: "basketball"},
			{Name: "雄狮篮球队", Coach: "刘洋", Contact: "13800000004", SportType: "basketball"},
			{Name: "飓风篮球队", Coach: "陈峰", Contact: "13800000005", SportType: "basketball"},
			{Name: "闪电篮球队", Coach: "赵磊", Contact: "13800000006", SportType: "basketball"},
			{Name: "烈焰篮球队", Coach: "孙涛", Contact: "13800000007", SportType: "basketball"},
			{Name: "旋风篮球队", Coach: "周杰", Contact: "13800000008", SportType: "basketball"},
			{Name: "羽翼羽毛球队", Coach: "林小燕", Contact: "13900000001", SportType: "badminton"},
			{Name: "飞羽羽毛球队", Coach: "黄丽萍", Contact: "13900000002", SportType: "badminton"},
		}
		if err := database.Create(&teams).Error; err != nil {
			return err
		}
		log.Println("已创建种子队伍数据")
	}

	var bookingCount int64
	database.Model(&models.Booking{}).Count(&bookingCount)
	if bookingCount == 0 {
		var venues []models.Venue
		database.Order("id").Find(&venues)

		bookings := []models.Booking{
			{VenueID: venues[0].ID, CustomerName: "陈刚", Phone: "13700001111", BookDate: "2026-07-01", StartHour: 18, EndHour: 20, Amount: 320, Status: "booked"},
			{VenueID: venues[0].ID, CustomerName: "周敏", Phone: "13700002222", BookDate: "2026-07-02", StartHour: 20, EndHour: 21, Amount: 160, Status: "booked"},
			{VenueID: venues[4].ID, CustomerName: "黄磊", Phone: "13700003333", BookDate: "2026-07-03", StartHour: 9, EndHour: 11, Amount: 400, Status: "booked"},
			{VenueID: venues[4].ID, CustomerName: "吴静", Phone: "13700004444", BookDate: "2026-07-03", StartHour: 14, EndHour: 16, Amount: 400, Status: "booked"},
			{VenueID: venues[5].ID, CustomerName: "郑浩", Phone: "13700005555", BookDate: "2026-07-04", StartHour: 10, EndHour: 12, Amount: 400, Status: "booked"},
		}
		if err := database.Create(&bookings).Error; err != nil {
			return err
		}
		log.Println("已创建种子预订数据")
	}

	log.Println("种子数据初始化完成")
	return nil
}
