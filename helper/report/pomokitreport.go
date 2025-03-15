package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	// "github.com/gocroot/config"
	// "github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	// "github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CountPomokitActivity(reports []model.PomodoroReport) map[string]PomokitInfo {
	pomokitCount := make(map[string]PomokitInfo)

	for _, report := range reports {
		phoneNumber := report.PhoneNumber
		if phoneNumber != "" {
			if info, exists := pomokitCount[phoneNumber]; exists {
				// Update data yang sudah ada
				info.Count++
				pomokitCount[phoneNumber] = info
			} else {
				pomokitCount[phoneNumber] = PomokitInfo{
					Count:       1,
					Name:        report.Name,
					PhoneNumber: report.PhoneNumber,
				}
			}
		}
	}

	return pomokitCount
}

func GetPomokitDataHarian(db *mongo.Database, filter bson.M) ([]model.PomodoroReport, error) {
	var conf model.Config
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
	if err != nil {
		return nil, errors.New("Config Not Found: " + err.Error())
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.PomokitUrl)
	if err != nil {
		return nil, errors.New("API Connection Failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Returned Status %d", resp.StatusCode)
	}

	var pomodoroReports []model.PomodoroReport
	if err := json.NewDecoder(resp.Body).Decode(&pomodoroReports); err != nil {
		var apiResponse model.PomokitResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
			return nil, errors.New("Invalid API Response: " + err.Error())
		}
		pomodoroReports = apiResponse.Data
	}

	var filteredReports []model.PomodoroReport
	today := time.Now().Truncate(24 * time.Hour)
	for _, report := range pomodoroReports {
		if report.CreatedAt.After(today) && 
		   report.CreatedAt.Before(today.Add(24 * time.Hour)) {
			filteredReports = append(filteredReports, report)
		}
	}

	return filteredReports, nil
}

func GeneratePomokitRekapHarian(db *mongo.Database) (msg string, err error) {
    pomokitData, err := GetPomokitDataHarian(db, TodayFilter())
    if err != nil {
        return "", fmt.Errorf("gagal mengambil data Pomokit: %v", err)
    }
    
    msg = "*Rekap Aktivitas Pomodoro Hari Ini:*\n\n"
    
    isLibur := HariLibur(GetDateSekarang())
    
    pomokitCounts := CountPomokitActivity(pomokitData)
    
    allUsers, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", bson.M{})
    if err != nil {
        return "", fmt.Errorf("gagal mengambil data pengguna: %v", err)
    }
    
    userActivityStatus := make(map[string]struct{
        Name       string
        Phone      string
        PointChange float64
        TotalPoint float64
        IsActive   bool
        GroupID    string
    })
    
    phoneToGroupID := make(map[string]string)
    
    for _, report := range pomokitData {
        if report.PhoneNumber != "" && report.WaGroupID != "" {
            phoneToGroupID[report.PhoneNumber] = report.WaGroupID
        }
    }
    
    for phoneNumber, info := range pomokitCounts {
        groupID := phoneToGroupID[phoneNumber]
        
        poin := info.Count
        totalPoin, err := AddPomokitPoints(db, phoneNumber, poin, pomokitData, groupID)
        if err != nil {
            continue
        }
        
        userActivityStatus[phoneNumber] = struct{
            Name       string
            Phone      string
            PointChange float64
            TotalPoint float64
            IsActive   bool
            GroupID    string
        }{
            Name:       info.Name,
            Phone:      phoneNumber,
            PointChange: poin,
            TotalPoint: totalPoin,
            IsActive:   true,
            GroupID:    groupID,
        }
    }
    
    if !isLibur {
        for _, user := range allUsers {
            if user.PhoneNumber != "" {
                if _, exists := userActivityStatus[user.PhoneNumber]; !exists {
                    groupID := ""
                    projectFilter := bson.M{"members.phonenumber": user.PhoneNumber}
                    projects, _ := atdb.GetAllDoc[[]model.Project](db, "project", projectFilter)
                    if len(projects) > 0 {
                        groupID = projects[0].WAGroupID
                    }
                    
                    totalPoin, err := DeductPomokitPoints(db, user.PhoneNumber, 1, groupID)
                    if err != nil {
                        continue
                    }
                    
                    userActivityStatus[user.PhoneNumber] = struct{
                        Name       string
                        Phone      string
                        PointChange float64
                        TotalPoint float64
                        IsActive   bool
                        GroupID    string
                    }{
                        Name:       user.Name,
                        Phone:      user.PhoneNumber,
                        PointChange: -1,
                        TotalPoint: totalPoin,
                        IsActive:   false,
                        GroupID:    groupID,
                    }
                }
            }
        }
    }
    
    for _, status := range userActivityStatus {
        if status.IsActive {
            msg += fmt.Sprintf("✅ %s (%s): +%.0f = %.0f poin\n", 
                status.Name, 
                status.Phone, 
                status.PointChange, 
                status.TotalPoint)
        } else {
            msg += fmt.Sprintf("⛔ %s (%s): %.0f = %.0f poin\n", 
                status.Name, 
                status.Phone, 
                status.PointChange, 
                status.TotalPoint)
        }
    }

	if !isLibur {
        msg += "\n*Catatan: Pengguna tanpa aktivitas Pomodoro hari ini akan dikurangi 1 poin*"
    } else {
        msg += "\n*Catatan: Hari ini adalah hari libur, tidak ada pengurangan poin*"
    }

    return msg, nil
}

func GeneratePomokitRekapHarianByGroupID(db *mongo.Database, groupID string) (msg string, err error) {
    pomokitData, err := GetPomokitDataHarian(db, TodayFilter())
    if err != nil {
        return "", fmt.Errorf("gagal mengambil data Pomokit: %v", err)
    }
    
    var filteredPomokitData []model.PomodoroReport
    if groupID != "" {
        for _, report := range pomokitData {
            if report.WaGroupID == groupID {
                filteredPomokitData = append(filteredPomokitData, report)
            }
        }
    } else {
        filteredPomokitData = pomokitData
    }
    
    if len(filteredPomokitData) == 0 && groupID != "" {
        var projectName string = "Grup Ini"
        project, err := atdb.GetOneDoc[model.Project](db, "project", bson.M{"wagroupid": groupID})
        if err == nil {
            projectName = project.Name
        }
        
        return fmt.Sprintf("*Rekap Aktivitas Pomodoro Hari Ini untuk %s:*\n\nTidak ada aktivitas Pomodoro yang tercatat hari ini untuk grup ini.", projectName), nil
    } else if len(filteredPomokitData) == 0 {
        return "*Rekap Aktivitas Pomodoro Hari Ini:*\n\nTidak ada aktivitas Pomodoro yang tercatat hari ini.", nil
    }
    
    if groupID != "" {
        msg = fmt.Sprintf("*Rekap Aktivitas Pomodoro Hari Ini untuk GroupID %s:*\n\n", groupID)
    } else {
        msg = "*Rekap Aktivitas Pomodoro Hari Ini:*\n\n"
    }
    
    isLibur := HariLibur(GetDateSekarang())
    
    pomokitCounts := CountPomokitActivity(filteredPomokitData)
    
    var groupMembers []model.Userdomyikado
    if groupID != "" {
        project, err := atdb.GetOneDoc[model.Project](db, "project", bson.M{"wagroupid": groupID})
        if err == nil {
            for _, member := range project.Members {
                user, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": member.PhoneNumber})
                if err == nil {
                    groupMembers = append(groupMembers, user)
                }
            }
        }
    } else {
        allUsers, _ := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", bson.M{})
        groupMembers = allUsers
    }
    
    userActivityStatus := make(map[string]struct{
        Name       string
        Phone      string
        PointChange float64
        TotalPoint float64
        IsActive   bool
    })
    
    for phoneNumber, info := range pomokitCounts {
        poin := info.Count
        totalPoin, err := AddPomokitPoints(db, phoneNumber, poin, filteredPomokitData, groupID)
        if err != nil {
            continue
        }
        
        userActivityStatus[phoneNumber] = struct{
            Name       string
            Phone      string
            PointChange float64
            TotalPoint float64
            IsActive   bool
        }{
            Name:       info.Name,
            Phone:      phoneNumber,
            PointChange: poin,
            TotalPoint: totalPoin,
            IsActive:   true,
        }
    }
    
    if !isLibur && len(groupMembers) > 0 {
        for _, user := range groupMembers {
            if _, exists := userActivityStatus[user.PhoneNumber]; !exists && user.PhoneNumber != "" {
                totalPoin, err := DeductPomokitPoints(db, user.PhoneNumber, 1, groupID)
                if err != nil {
                    continue
                }
                
                userActivityStatus[user.PhoneNumber] = struct{
                    Name       string
                    Phone      string
                    PointChange float64
                    TotalPoint float64
                    IsActive   bool
                }{
                    Name:       user.Name,
                    Phone:      user.PhoneNumber,
                    PointChange: -1,
                    TotalPoint: totalPoin,
                    IsActive:   false,
                }
            }
        }
    }
    
    // Format untuk output
    for _, status := range userActivityStatus {
        if status.IsActive {
            msg += fmt.Sprintf("✅ %s (%s): +%.0f = %.0f poin\n", 
                status.Name, 
                status.Phone, 
                status.PointChange, 
                status.TotalPoint)
        } else {
            msg += fmt.Sprintf("⛔ %s (%s): %.0f = %.0f poin\n", 
                status.Name, 
                status.Phone, 
                status.PointChange, 
                status.TotalPoint)
        }
    }
    
    // Tambahkan catatan jika bukan hari libur
    if !isLibur {
        msg += "\n*Catatan: Pengguna tanpa aktivitas Pomodoro hari ini akan dikurangi 1 poin*"
    } else {
        msg += "\n*Catatan: Hari ini adalah hari libur, tidak ada pengurangan poin*"
    }

    return msg, nil
}

func AddPomokitPoints(db *mongo.Database, phoneNumber string, points float64, pomokitData []model.PomodoroReport, groupID string) (totalPoin float64, err error) {
    today := time.Now().Truncate(24 * time.Hour)
    tomorrow := today.Add(24 * time.Hour)
    
    filter := bson.M{
        "phonenumber": phoneNumber,
        "createdat": bson.M{
            "$gte": today,
            "$lt": tomorrow,
        },
    }

    if groupID != "" {
        filter["groupid"] = groupID
    } else {
        filter["groupid"] = ""
    }
    
    var existingPoints []PomokitPoin
    existingPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    if err == nil && len(existingPoints) > 0 {
        return GetTotalPomokitPoints(db, phoneNumber)
    }

    usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phoneNumber})
    if err != nil {
        return 0, err
    }
    
    pomokitPoin := PomokitPoin{
        UserID:      usr.ID,
        Name:        usr.Name,
        PhoneNumber: usr.PhoneNumber,
        Email:       usr.Email,
        GroupID:     groupID, // Use the groupID as provided (could be empty)
        PoinPomokit: points,  // Use new field name
        CreatedAt:   time.Now(),
    }
    
    _, err = atdb.InsertOneDoc(db, "pomokitpoin", pomokitPoin)
    if err != nil {
        return 0, err
    }
    
    totalPoin, err = GetTotalPomokitPoints(db, phoneNumber)
    if err != nil {
        return 0, err
    }
    
    usr.Poin = totalPoin
    _, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phoneNumber}, usr)
    if err != nil {
        return totalPoin, nil // Tetap kembalikan total poin meskipun ada error saat update user
    }
    
    return totalPoin, nil
}

func DeductPomokitPoints(db *mongo.Database, phoneNumber string, points float64, groupID string) (totalPoin float64, err error) {
    today := time.Now().Truncate(24 * time.Hour)
    tomorrow := today.Add(24 * time.Hour)
    
    filter := bson.M{
        "phonenumber": phoneNumber,
        "poinpomokit": bson.M{"$lt": 0}, // Look for negative points (deductions)
        "createdat": bson.M{
            "$gte": today,
            "$lt": tomorrow,
        },
    }
    
    if groupID != "" {
        filter["groupid"] = groupID
    } else {
        filter["groupid"] = ""
    }
    
    var existingDeductions []PomokitPoin
    existingDeductions, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    if err == nil && len(existingDeductions) > 0 {
        return GetTotalPomokitPoints(db, phoneNumber)
    }

    usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phoneNumber})
    if err != nil {
        return 0, err
    }
    
    pomokitPoin := PomokitPoin{
        UserID:      usr.ID,
        Name:        usr.Name,
        PhoneNumber: usr.PhoneNumber,
        Email:       usr.Email,
        GroupID:     groupID, // Use groupID as provided (could be empty)
        PoinPomokit: -points, // Use new field name with negative value for deduction
        CreatedAt:   time.Now(),
    }
    
    _, err = atdb.InsertOneDoc(db, "pomokitpoin", pomokitPoin)
    if err != nil {
        return 0, err
    }
    
    totalPoin, err = GetTotalPomokitPoints(db, phoneNumber)
    if err != nil {
        return 0, err
    }
    
    usr.Poin = totalPoin
    _, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phoneNumber}, usr)
    if err != nil {
        return totalPoin, nil // Tetap kembalikan total poin meskipun ada error saat update user
    }
    
    return totalPoin, nil
}

func GetTotalPomokitPoints(db *mongo.Database, phoneNumber string) (totalPoin float64, err error) {
    var pomokitPoints []PomokitPoin
    pomokitPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", bson.M{"phonenumber": phoneNumber})
    if err != nil {
        return 0, err
    }
    
    totalPoin = 0
    for _, record := range pomokitPoints {
        totalPoin += record.PoinPomokit
    }
    
    return totalPoin, nil
}

// Helper function untuk mendapatkan nomor owner grup
func getGroupOwnerPhone(db *mongo.Database, groupID string) (string, error) {
	project, err := atdb.GetOneDoc[model.Project](db, "project", bson.M{"wagroupid": groupID})
	if err != nil {
		return "", errors.New("Gagal mendapatkan data project: " + err.Error())
	}
	return project.Owner.PhoneNumber, nil
}

// // GeneratePomokitRekapMingguan membuat rekap aktivitas Pomokit mingguan
// func GeneratePomokitRekapMingguan(db *mongo.Database) (msg string, err error) {
// 	// Implementasi sama seperti harian tapi dengan filter WeeklyFilter()
// 	// ...
// 	return "Rekap Mingguan Pomokit", nil
// }