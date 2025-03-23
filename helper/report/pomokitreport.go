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


	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

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

func GenerateTotalPomokitReportNoPenalty(db *mongo.Database) (string, error) {
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
    
    // Count every individual activity
    userActivityCounts := make(map[string]int)
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
        
        // Count each activity individually
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
    
    // Convert date set to slice for iteration
    var allDates []string
    for dateStr := range dateSet {
        allDates = append(allDates, dateStr)
    }
    
    // Sort dates from oldest to newest
    sort.Strings(allDates)
    
    // Hitung poin (tanpa penalti)
    userPoints := make(map[string]float64)
    
    for phoneNumber := range userInfo {
        // Hanya menghitung aktivitas sebagai poin positif, tanpa penalti
        userPoints[phoneNumber] = float64(userActivityCounts[phoneNumber])
    }
    
    msg := "*Total Akumulasi Poin Pomokit Semua Waktu*\n\n"
    
    type UserPoint struct {
        Name         string
        PhoneNumber  string
        Points       float64
        ActivityCount int
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
        
        msg += fmt.Sprintf("✅ %s (%s): +%.0f poin (total: %.0f)\n", 
            displayName, up.PhoneNumber, up.Points, up.Points)
    }
    
    msg += fmt.Sprintf("\n*Rentang data: %s s/d %s*\n", 
        earliestDate.Format("2006-01-02"), 
        latestDate.Format("2006-01-02"))
    
    msg += "\n*Catatan: +1 poin untuk setiap aktivitas Pomodoro*"
    
    return msg, nil
}

func GenerateTotalPomokitReportByGroupID(db *mongo.Database, groupID string) (string, error) {
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
    
    userActivityCounts := make(map[string]int)
    userInfo := make(map[string]struct {
        Name     string
        GroupID  string
    })
    
    dateSet := make(map[string]bool)
    earliestDate := time.Now()
    latestDate := time.Time{}
    
    // Filter dan hitung aktivitas berdasarkan WAGroupID
    for _, report := range allPomokitData {
        phoneNumber := report.PhoneNumber
        reportGroupID := report.WaGroupID
        
        // Lewati record yang tidak cocok dengan groupID yang dicari
        if reportGroupID != groupID {
            continue
        }
        
        if phoneNumber == "" {
            continue
        }
        
        userInfo[phoneNumber] = struct {
            Name     string
            GroupID  string
        }{
            Name:     report.Name,
            GroupID:  reportGroupID,
        }
        
        // Hitung setiap aktivitas individual
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
    
    // Cek apakah ada data yang difilter
    if len(userActivityCounts) == 0 {
        return fmt.Sprintf("Tidak ada data Pomokit yang tersedia untuk GroupID %s", groupID), nil
    }
    
    // Hitung poin (tanpa penalti)
    userPoints := make(map[string]float64)
    
    for phoneNumber := range userInfo {
        // Hanya menghitung aktivitas sebagai poin positif, tanpa penalti
        userPoints[phoneNumber] = float64(userActivityCounts[phoneNumber])
    }
    
    // Coba dapatkan nama proyek dari database
    var projectName string = "Grup " + groupID
    project, err := atdb.GetOneDoc[model.Project](db, "project", bson.M{"wagroupid": groupID})
    if err == nil {
        projectName = project.Name
    }
    
    // Format pesan laporan
    msg := fmt.Sprintf("*Total Akumulasi Poin Pomokit untuk %s*\n\n", projectName)
    
    type UserPoint struct {
        Name         string
        PhoneNumber  string
        Points       float64
        ActivityCount int
    }
    
    // Konversi map ke slice untuk pengurutan
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
    
    // Urutkan berdasarkan poin (dari tertinggi ke terendah)
    sort.Slice(userPointsList, func(i, j int) bool {
        return userPointsList[i].Points > userPointsList[j].Points
    })
    
    // Tambahkan detail user ke pesan
    for _, up := range userPointsList {
        displayName := up.Name
        if displayName == "" {
            displayName = "Pengguna " + up.PhoneNumber
        }
        
        msg += fmt.Sprintf("✅ %s (%s): +%.0f poin (total: %.0f)\n", 
            displayName, up.PhoneNumber, up.Points, up.Points)
    }
    
    // Tambahkan informasi rentang waktu
    if !earliestDate.Equal(time.Now()) && !latestDate.IsZero() {
        msg += fmt.Sprintf("\n*Rentang data: %s s/d %s*\n", 
            earliestDate.Format("2006-01-02"), 
            latestDate.Format("2006-01-02"))
    }
    
    // Tambahkan catatan
    msg += "\n*Catatan: +1 poin untuk setiap aktivitas Pomodoro*"
    
    return msg, nil
}