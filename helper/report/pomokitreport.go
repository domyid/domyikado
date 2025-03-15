package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// GetPomokitDataHarian mengambil data Pomokit untuk periode tertentu
func GetPomokitDataHarian(db *mongo.Database, filter bson.M) ([]model.PomodoroReport, error) {
    // Ambil konfigurasi
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, errors.New("Config Not Found: " + err.Error())
    }

    fmt.Printf("DEBUG: Fetching Pomokit data from %s\n", conf.PomokitUrl)

    // HTTP Client request ke API Pomokit
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.PomokitUrl)
    if err != nil {
        return nil, errors.New("API Connection Failed: " + err.Error())
    }
    defer resp.Body.Close()

    // Handle non-200 status
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API Returned Status %d", resp.StatusCode)
    }

    // Decode response
    var pomodoroReports []model.PomodoroReport
    if err := json.NewDecoder(resp.Body).Decode(&pomodoroReports); err != nil {
        responseBody, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
        
        var apiResponse model.PomokitResponse
        if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
            fmt.Printf("ERROR: Failed to parse API response. Raw response: %s\n", string(responseBody))
            return nil, errors.New("Invalid API Response: " + err.Error())
        }
        pomodoroReports = apiResponse.Data
    }

    fmt.Printf("DEBUG: Retrieved %d Pomokit reports from API\n", len(pomodoroReports))

    var filteredReports []model.PomodoroReport
    today := time.Now().Truncate(24 * time.Hour)
    for _, report := range pomodoroReports {
        if report.CreatedAt.After(today) && 
           report.CreatedAt.Before(today.Add(24 * time.Hour)) {
            filteredReports = append(filteredReports, report)
            fmt.Printf("DEBUG: Including report for user %s (groupID: '%s')\n", 
                report.PhoneNumber, report.WaGroupID)
        }
    }

    fmt.Printf("DEBUG: After date filtering, %d reports remain\n", len(filteredReports))
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
        if report.PhoneNumber != "" {
            phoneToGroupID[report.PhoneNumber] = report.WaGroupID
            fmt.Printf("DEBUG: Mapped phone %s to groupID '%s'\n", 
                report.PhoneNumber, report.WaGroupID)
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
    currentTime := time.Now()
    today := currentTime.Truncate(24 * time.Hour)
    tomorrow := today.Add(24 * time.Hour)
    
    // Define the base filter
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
        filter["$or"] = []bson.M{
            {"groupid": ""},
            {"groupid": bson.M{"$exists": false}},
        }
    }
    
    var existingPoints []PomokitPoin
    existingPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    
    var latestActivityTime time.Time
    for _, report := range pomokitData {
        if report.PhoneNumber == phoneNumber {
            if report.CreatedAt.After(latestActivityTime) {
                latestActivityTime = report.CreatedAt
            }
        }
    }
    
    if err == nil && len(existingPoints) > 0 {
        mostRecentRecord := existingPoints[0]
        for _, record := range existingPoints {
            if record.CreatedAt.After(mostRecentRecord.CreatedAt) {
                mostRecentRecord = record
            }
        }
        
        if latestActivityTime.After(mostRecentRecord.CreatedAt) {
            fmt.Printf("INFO: Found newer activity data for user %s (groupID: %s), updating points\n", 
                phoneNumber, groupID)
            
            deleteFilter := filter
            _, err = db.Collection("pomokitpoin").DeleteMany(context.Background(), deleteFilter)
            if err != nil {
                return 0, fmt.Errorf("failed to delete old records: %v", err)
            }
            
        } else {
            fmt.Printf("INFO: No newer activity data for user %s (groupID: %s), keeping existing record\n", 
                phoneNumber, groupID)
            return GetTotalPomokitPoints(db, phoneNumber)
        }
    } else {
        fmt.Printf("INFO: Adding new points for user %s (groupID: %s)\n", phoneNumber, groupID)
    }

    // Ambil data user
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
        CreatedAt:   currentTime, // Use current time
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
    currentTime := time.Now()
    today := currentTime.Truncate(24 * time.Hour)
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
        filter["$or"] = []bson.M{
            {"groupid": ""},
            {"groupid": bson.M{"$exists": false}},
        }
    }
    
    var existingDeductions []PomokitPoin
    existingDeductions, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    
    if err == nil && len(existingDeductions) > 0 {
        
        fmt.Printf("INFO: Found existing deduction for user %s (groupID: %s), updating\n", 
            phoneNumber, groupID)
        
        deleteFilter := filter
        _, err = db.Collection("pomokitpoin").DeleteMany(context.Background(), deleteFilter)
        if err != nil {
            return 0, fmt.Errorf("failed to delete old deduction records: %v", err)
        }
        
    } else {
        fmt.Printf("INFO: Adding new deduction for user %s (groupID: %s)\n", phoneNumber, groupID)
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
        CreatedAt:   currentTime, // Use current time
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
    
    fmt.Printf("DEBUG: Total PoinPomokit for %s: %.2f from %d records\n", 
        phoneNumber, totalPoin, len(pomokitPoints))
    
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