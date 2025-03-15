package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	// "github.com/gocroot/config"
	// "github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	// "github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetPomokitDataHarian mengambil data Pomokit untuk periode tertentu
func GetPomokitDataHarian(db *mongo.Database, filter bson.M) ([]model.PomodoroReport, error) {
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, errors.New("Config Not Found: " + err.Error())
    }

    fmt.Printf("DEBUG: Fetching Pomokit data from %s\n", conf.PomokitUrl)

    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.PomokitUrl)
    if err != nil {
        return nil, errors.New("API Connection Failed: " + err.Error())
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API Returned Status %d", resp.StatusCode)
    }

    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, errors.New("Error reading response: " + err.Error())
    }
    
    resp.Body.Close()
    resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

    var pomodoroReports []model.PomodoroReport
    if err := json.NewDecoder(resp.Body).Decode(&pomodoroReports); err != nil {
        resp.Body.Close()
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
        
        var apiResponse model.PomokitResponse
        if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
            fmt.Printf("ERROR: Failed to parse API response. Raw response: %s\n", string(responseBody))
            return nil, errors.New("Invalid API Response: " + err.Error())
        }
        pomodoroReports = apiResponse.Data
        fmt.Printf("DEBUG: Decoded as PomokitResponse struct with %d reports\n", len(pomodoroReports))
    } else {
        fmt.Printf("DEBUG: Decoded as direct array with %d reports\n", len(pomodoroReports))
    }

    if len(pomodoroReports) == 0 {
        fmt.Printf("WARNING: No reports found in API response\n")
        return nil, nil
    }

    fmt.Printf("DEBUG: Retrieved %d Pomokit reports from API\n", len(pomodoroReports))

    location, err := time.LoadLocation("Asia/Jakarta")
    if err != nil {
        location = time.Local // Fallback to local timezone
    }
    
    // Extract date range from the filter
    var startTime, endTime time.Time
    
    if filter != nil {
        if dateFilter, ok := filter["$gte"]; ok {
            if oid, ok := dateFilter.(primitive.ObjectID); ok {
                startTime = oid.Timestamp()
                fmt.Printf("DEBUG: Using filter start time: %v\n", startTime)
            }
        }
        
        if dateFilter, ok := filter["$lt"]; ok {
            if oid, ok := dateFilter.(primitive.ObjectID); ok {
                endTime = oid.Timestamp()
                fmt.Printf("DEBUG: Using filter end time: %v\n", endTime)
            }
        }
    }
    
    // If filter didn't provide valid dates, default to today
    if startTime.IsZero() || endTime.IsZero() {
        now := time.Now().In(location)
        startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
        endTime = startTime.Add(24 * time.Hour)
        fmt.Printf("DEBUG: Using default time range: %v to %v\n", startTime, endTime)
    }
    
    fmt.Printf("DEBUG: Filtering for activities between %v and %v\n", startTime, endTime)

    var filteredReports []model.PomodoroReport
    for _, report := range pomodoroReports {
        reportTime := report.CreatedAt.In(location)
        
        if (reportTime.Equal(startTime) || reportTime.After(startTime)) && reportTime.Before(endTime) {
            filteredReports = append(filteredReports, report)
            fmt.Printf("DEBUG: Including report for user %s (groupID: '%s', time: %v)\n", 
                report.PhoneNumber, report.WaGroupID, reportTime)
        } else {
            fmt.Printf("DEBUG: Excluding report for user %s (time: %v outside of %v - %v)\n", 
                report.PhoneNumber, reportTime, startTime, endTime)
        }
    }

    fmt.Printf("DEBUG: After date filtering, %d reports remain\n", len(filteredReports))
    
    phoneNumberCounts := make(map[string]int)
    for _, report := range filteredReports {
        phoneNumberCounts[report.PhoneNumber]++
    }
    
    for phone, count := range phoneNumberCounts {
        fmt.Printf("DEBUG: User %s has %d activities in time range\n", phone, count)
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
    
    // Filter data yang memiliki WAGroupID
    var filteredPomokitData []model.PomodoroReport
    for _, report := range pomokitData {
        if report.PhoneNumber != "" && report.WaGroupID != "" {
            filteredPomokitData = append(filteredPomokitData, report)
        } else {
            fmt.Printf("DEBUG: Melewati data Pomokit tanpa PhoneNumber atau WAGroupID: %s / %s\n",
                report.PhoneNumber, report.WaGroupID)
        }
    }
    
    if len(filteredPomokitData) == 0 {
        return "*Rekap Aktivitas Pomodoro Hari Ini:*\n\nTidak ada aktivitas Pomodoro dengan WAGroupID valid yang tercatat hari ini.", nil
    }
    
    pomokitCounts := CountPomokitActivity(filteredPomokitData)
    
    // Dapatkan semua nomor telepon unik dari data Pomokit
    allPhoneNumbers := make(map[string]struct{
        Name    string
        GroupID string
    })
    
    for _, report := range filteredPomokitData {
        // Simpan semua pengguna dengan WAGroupID valid
        if report.PhoneNumber != "" && report.WaGroupID != "" {
            allPhoneNumbers[report.PhoneNumber] = struct{
                Name    string
                GroupID string
            }{
                Name:    report.Name,
                GroupID: report.WaGroupID,
            }
        }
    }
    
    userActivityStatus := make(map[string]struct{
        Name       string
        Phone      string
        PointChange float64
        TotalPoint float64
        IsActive   bool
        GroupID    string
    })
    
    // Proses pengguna dengan aktivitas
    for phoneNumber, info := range pomokitCounts {
        userData, exists := allPhoneNumbers[phoneNumber]
        if !exists || userData.GroupID == "" {
            continue // Skip jika tidak ada atau tidak ada WAGroupID
        }
        
        groupID := userData.GroupID
        poin := info.Count
        totalPoin, err := AddPomokitPoints(db, phoneNumber, poin, filteredPomokitData, groupID)
        if err != nil {
            fmt.Printf("ERROR: Gagal menambah poin untuk %s: %v\n", phoneNumber, err)
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
    
    // Proses pengguna tanpa aktivitas (jika bukan hari libur)
    if !isLibur {
        for phoneNumber, userData := range allPhoneNumbers {
            if _, exists := userActivityStatus[phoneNumber]; !exists && userData.GroupID != "" {
                groupID := userData.GroupID
                name := userData.Name
                if name == "" {
                    name = "Pengguna " + phoneNumber
                }
                
                totalPoin, err := DeductPomokitPoints(db, phoneNumber, 1, groupID)
                if err != nil {
                    fmt.Printf("ERROR: Gagal mengurangi poin untuk %s: %v\n", phoneNumber, err)
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
                    Name:       name,
                    Phone:      phoneNumber,
                    PointChange: -1,
                    TotalPoint: totalPoin,
                    IsActive:   false,
                    GroupID:    groupID,
                }
            }
        }
    }
    
    // Format untuk output
    for _, status := range userActivityStatus {
        displayName := status.Name
        if displayName == "" {
            displayName = "Pengguna " + status.Phone
        }
        
        if status.IsActive {
            msg += fmt.Sprintf("✅ %s (%s): +%.0f = %.0f poin\n", 
                displayName, 
                status.Phone, 
                status.PointChange, 
                status.TotalPoint)
        } else {
            msg += fmt.Sprintf("⛔ %s (%s): %.0f = %.0f poin\n", 
                displayName, 
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

func GetAllPomokitData(db *mongo.Database) ([]model.PomodoroReport, error) {
    // Ambil konfigurasi
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, errors.New("Config Not Found: " + err.Error())
    }

    fmt.Printf("DEBUG: Mengambil semua data Pomokit dari %s\n", conf.PomokitUrl)

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

    // Read entire response for debugging and reprocessing if needed
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, errors.New("Error reading response: " + err.Error())
    }
    
    // Create a new reader from the response body for JSON decoding
    resp.Body.Close()
    resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

    // Try to decode as direct array first
    var pomodoroReports []model.PomodoroReport
    if err := json.NewDecoder(resp.Body).Decode(&pomodoroReports); err != nil {
        // Reset reader and try as wrapped response
        resp.Body.Close()
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
        
        var apiResponse model.PomokitResponse
        if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
            fmt.Printf("ERROR: Failed to parse API response. Raw response: %s\n", string(responseBody))
            return nil, errors.New("Invalid API Response: " + err.Error())
        }
        pomodoroReports = apiResponse.Data
        fmt.Printf("DEBUG: Decoded as PomokitResponse struct with %d reports\n", len(pomodoroReports))
    } else {
        fmt.Printf("DEBUG: Decoded as direct array with %d reports\n", len(pomodoroReports))
    }

    if len(pomodoroReports) == 0 {
        fmt.Printf("WARNING: No reports found in API response\n")
        return nil, nil
    }

    fmt.Printf("DEBUG: Retrieved %d total Pomokit reports from API\n", len(pomodoroReports))
    
    return pomodoroReports, nil
}

// TotalPomokitPoint menghitung total semua poin di koleksi pomokitpoin
func TotalPomokitPoint(db *mongo.Database) (map[string]float64, error) {
    // Ambil semua data dari collection pomokitpoin
    pomokitPoints, err := atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", bson.M{})
    if err != nil {
        return nil, fmt.Errorf("gagal mengambil data pomokitpoin: %v", err)
    }
    
    // Map untuk menyimpan total poin per pengguna
    totalPoints := make(map[string]float64)
    
    // Hitung total poin untuk setiap pengguna
    for _, record := range pomokitPoints {
        totalPoints[record.PhoneNumber] += record.PoinPomokit
    }
    
    fmt.Printf("DEBUG: Total poin dari %d record pomokitpoin untuk %d pengguna\n", 
        len(pomokitPoints), len(totalPoints))
    
    return totalPoints, nil
}

func GenerateTotalPomokitReport(db *mongo.Database) (string, error) {
    allPomokitData, err := GetAllPomokitData(db)
    if err != nil {
        return "", fmt.Errorf("gagal mengambil data Pomokit: %v", err)
    }
    
    if len(allPomokitData) == 0 {
        return "Tidak ada data Pomokit yang tersedia", nil
    }
    
    // Timezone Jakarta untuk konsistensi
    location, _ := time.LoadLocation("Asia/Jakarta")
    if location == nil {
        location = time.Local
    }
    
    userActivityCounts := make(map[string]float64)
    userInfo := make(map[string]struct {
        Name     string
        GroupID  string
    })
    
    dateSet := make(map[string]bool)
    earliestDate := time.Now()
    latestDate := time.Time{}
    
    for _, report := range allPomokitData {
        phoneNumber := report.PhoneNumber
        groupID := report.WaGroupID
        
        if phoneNumber == "" || groupID == "" {
            continue
        }
        
        userInfo[phoneNumber] = struct {
            Name     string
            GroupID  string
        }{
            Name:     report.Name,
            GroupID:  groupID,
        }
        
        userActivityCounts[phoneNumber]++
        
        activityTime := report.CreatedAt.In(location)
        dateStr := activityTime.Format("2006-01-02")
        
        if activityTime.Before(earliestDate) {
            earliestDate = activityTime
        }
        if activityTime.After(latestDate) {
            latestDate = activityTime
        }
        
        dateSet[dateStr] = true
    }
    
    var allDates []string
    for dateStr := range dateSet {
        allDates = append(allDates, dateStr)
    }
    
    sort.Strings(allDates)
    
    userPoints := make(map[string]float64)
    daysWithoutActivity := make(map[string]int)
    
    for phoneNumber := range userInfo {
        userPoints[phoneNumber] = userActivityCounts[phoneNumber]
        daysWithoutActivity[phoneNumber] = 0
        
        for _, dateStr := range allDates {
            date, _ := time.Parse("2006-01-02", dateStr)
            isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
            isHoliday := HariLibur(date)
            
            if isWeekend || isHoliday {
                continue
            }
            
            userHasActivityOnDate := false
            for _, report := range allPomokitData {
                if report.PhoneNumber == phoneNumber {
                    reportDate := report.CreatedAt.In(location).Format("2006-01-02")
                    if reportDate == dateStr {
                        userHasActivityOnDate = true
                        break
                    }
                }
            }
            
            if !userHasActivityOnDate {
                userPoints[phoneNumber] -= 1
                daysWithoutActivity[phoneNumber]++
            }
        }
    }
    
    for phoneNumber, points := range userPoints {
        info := userInfo[phoneNumber]
        
        pomokitPoin := PomokitPoin{
            ID:          primitive.NewObjectID(),
            UserID:      primitive.NewObjectID(),
            Name:        info.Name,
            PhoneNumber: phoneNumber,
            GroupID:     info.GroupID,
            PoinPomokit: points,
            CreatedAt:   time.Now(),
        }
        
        // Use upsert to avoid duplicates
        filter := bson.M{"phonenumber": phoneNumber}
        update := bson.M{"$set": pomokitPoin}
        opts := options.Update().SetUpsert(true)
        
        _, err := db.Collection("pomokitpoin").UpdateOne(context.Background(), filter, update, opts)
        if err != nil {
            fmt.Printf("ERROR: Failed to update pomokitpoin for %s: %v\n", phoneNumber, err)
        }
    }
    
    msg := "*Total Akumulasi Poin Pomokit Semua Waktu*\n\n"
    
    type UserPoint struct {
        Name         string
        PhoneNumber  string
        Points       float64
        ActivityCount float64
    }
    
    var userPointsList []UserPoint
    for phoneNumber, points := range userPoints {
        info := userInfo[phoneNumber]
        userPointsList = append(userPointsList, UserPoint{
            Name:         info.Name,
            PhoneNumber:  phoneNumber,
            Points:       points,
            ActivityCount: userActivityCounts[phoneNumber],
        })
    }
    
    sort.Slice(userPointsList, func(i, j int) bool {
        return userPointsList[i].Points > userPointsList[j].Points
    })
    
    for _, up := range userPointsList {
        displayName := up.Name
        if displayName == "" {
            displayName = "Pengguna " + up.PhoneNumber
        }
        
        if up.Points >= 0 {
            msg += fmt.Sprintf("✅ %s (%s): +%.0f poin (%d aktivitas)\n", 
                displayName, up.PhoneNumber, up.Points, int(up.ActivityCount))
        } else {
            msg += fmt.Sprintf("⛔ %s (%s): %.0f poin (%d aktivitas)\n", 
                displayName, up.PhoneNumber, up.Points, int(up.ActivityCount))
        }
    }
    
    msg += fmt.Sprintf("\n*Rentang data: %s s/d %s*\n", 
        earliestDate.Format("2006-01-02"), 
        latestDate.Format("2006-01-02"))
    
    msg += "\n*Catatan: +1 poin untuk setiap aktivitas Pomodoro, -1 poin untuk setiap hari kerja tanpa aktivitas*"
    
    return msg, nil
}


func CountPomokitActivity(reports []model.PomodoroReport) map[string]PomokitInfo {
    pomokitCount := make(map[string]PomokitInfo)

    for _, report := range reports {
        phoneNumber := report.PhoneNumber
        if phoneNumber != "" {
            if info, exists := pomokitCount[phoneNumber]; exists {
                // Update data yang sudah ada dengan increment yang benar
                info.Count++
                pomokitCount[phoneNumber] = info
                fmt.Printf("DEBUG: Incrementing count for %s to %f\n", phoneNumber, info.Count)
            } else {
                pomokitCount[phoneNumber] = PomokitInfo{
                    Count:       1,
                    Name:        report.Name,
                    PhoneNumber: report.PhoneNumber,
                }
                fmt.Printf("DEBUG: New user %s with count 1\n", phoneNumber)
            }
        }
    }

    // Final debug to verify all counts
    for phone, info := range pomokitCount {
        fmt.Printf("DEBUG: Final count for %s: %f activities\n", phone, info.Count)
    }

    return pomokitCount
}

func AddPomokitPoints(db *mongo.Database, phoneNumber string, points float64, pomokitData []model.PomodoroReport, groupID string) (totalPoin float64, err error) {
    // If GroupID is empty, return error
    if groupID == "" {
        return 0, fmt.Errorf("WAGroupID tidak boleh kosong")
    }
    
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
        "groupid": groupID,
    }
    
    var existingPoints []PomokitPoin
    existingPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    
    var latestActivityTime time.Time
    var userName string = ""
    
    // Get user info from Pomokit data
    for _, report := range pomokitData {
        if report.PhoneNumber == phoneNumber && report.WaGroupID == groupID {
            if userName == "" && report.Name != "" {
                userName = report.Name
            }
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
            fmt.Printf("INFO: Found new activity data for user %s (groupID: %s), updating points\n", 
                phoneNumber, groupID)
            
            deleteFilter := filter
            _, err = db.Collection("pomokitpoin").DeleteMany(context.Background(), deleteFilter)
            if err != nil {
                return 0, fmt.Errorf("gagal menghapus catatan lama: %v", err)
            }
        } else {
            fmt.Printf("INFO: No new activity data for user %s (groupID: %s), keeping existing records\n", 
                phoneNumber, groupID)
            return GetTotalPomokitPoints(db, phoneNumber)
        }
    } else {
        fmt.Printf("INFO: Adding new points for user %s (groupID: %s)\n", phoneNumber, groupID)
    }

    // Create the record
    var displayName string
    if userName != "" {
        displayName = userName
    } else {
        displayName = "Pengguna " + phoneNumber
    }
    
    pomokitPoin := PomokitPoin{
        ID:          primitive.NewObjectID(),
        UserID:      primitive.NewObjectID(),
        Name:        displayName,
        PhoneNumber: phoneNumber,
        GroupID:     groupID, 
        PoinPomokit: points,
        CreatedAt:   currentTime,
    }
    
    // Insert the record
    insertedID, err := atdb.InsertOneDoc(db, "pomokitpoin", pomokitPoin)
    if err != nil {
        return 0, fmt.Errorf("gagal menyimpan poin: %v", err)
    }
    
    fmt.Printf("INFO: Successfully saved points for %s with ID %v\n", 
        phoneNumber, insertedID)
    
    // Get total points
    totalPoin, err = GetTotalPomokitPoints(db, phoneNumber)
    if err != nil {
        return 0, err
    }
    
    return totalPoin, nil
}

func DeductPomokitPoints(db *mongo.Database, phoneNumber string, points float64, groupID string) (totalPoin float64, err error) {
    // If GroupID is empty, return error
    if groupID == "" {
        return 0, fmt.Errorf("WAGroupID tidak boleh kosong")
    }
    
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
        "groupid": groupID,
    }
    
    var existingDeductions []PomokitPoin
    existingDeductions, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    
    if err == nil && len(existingDeductions) > 0 {
        fmt.Printf("INFO: Found existing deductions for user %s (groupID: %s), updating\n", 
            phoneNumber, groupID)
        
        deleteFilter := filter
        _, err = db.Collection("pomokitpoin").DeleteMany(context.Background(), deleteFilter)
        if err != nil {
            return 0, fmt.Errorf("gagal menghapus catatan pengurangan lama: %v", err)
        }
    } else {
        fmt.Printf("INFO: Adding new deduction for user %s (groupID: %s)\n", phoneNumber, groupID)
    }

    // Try to get user info
    var userName string = "Pengguna " + phoneNumber
    
    // Try to get name from existing records in pomokitpoin
    existingUser, err := atdb.GetOneDoc[PomokitPoin](db, "pomokitpoin", bson.M{"phonenumber": phoneNumber})
    if err == nil && existingUser.Name != "" {
        userName = existingUser.Name
    } else {
        // Try to get name from user collection
        user, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phoneNumber})
        if err == nil && user.Name != "" {
            userName = user.Name
        }
    }
    
    // Create record with deduction
    pomokitPoin := PomokitPoin{
        ID:          primitive.NewObjectID(),
        UserID:      primitive.NewObjectID(),
        Name:        userName,
        PhoneNumber: phoneNumber,
        GroupID:     groupID,
        PoinPomokit: -points, // Negative value for deduction
        CreatedAt:   currentTime,
    }
    
    // Insert the record
    insertedID, err := atdb.InsertOneDoc(db, "pomokitpoin", pomokitPoin)
    if err != nil {
        return 0, fmt.Errorf("gagal menyimpan pengurangan poin: %v", err)
    }
    
    fmt.Printf("INFO: Successfully saved deduction for %s with ID %v\n", 
        phoneNumber, insertedID)
    
    // Get total points
    totalPoin, err = GetTotalPomokitPoints(db, phoneNumber)
    if err != nil {
        return 0, err
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