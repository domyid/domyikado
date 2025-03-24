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

func GeneratePomokitReportKemarin(db *mongo.Database, groupID string) (string, error) {
	// Ambil semua data Pomokit
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
	
	// Mendapatkan waktu kemarin di zona waktu Jakarta
	todayJkt := time.Now().In(location).Truncate(24 * time.Hour)
	yesterdayJkt := todayJkt.AddDate(0, 0, -1)
	
	// Filter dan hitung aktivitas berdasarkan WAGroupID dan waktu kemarin
	userActivityCounts := make(map[string]int)
	userInfo := make(map[string]struct {
		Name     string
		GroupID  string
	})
	
	totalAktivitasKemarin := 0
	
	for _, report := range allPomokitData {
		phoneNumber := report.PhoneNumber
		reportGroupID := report.WaGroupID
		
		if reportGroupID != groupID || phoneNumber == "" {
			continue
		}
		
		activityTime := report.CreatedAt.In(location)
		
		// Cek apakah aktivitas terjadi kemarin
		if activityTime.After(yesterdayJkt) && activityTime.Before(todayJkt) {
			userInfo[phoneNumber] = struct {
				Name     string
				GroupID  string
			}{
				Name:     report.Name,
				GroupID:  reportGroupID,
			}
			
			// Hitung setiap sesi individual
			userActivityCounts[phoneNumber]++
			totalAktivitasKemarin++
		}
	}
	
	// Cek apakah ada data yang difilter
	if len(userActivityCounts) == 0 {
		return fmt.Sprintf("Tidak ada aktivitas Pomokit kemarin untuk GroupID %s", groupID), nil
	}
	
	// Hitung poin
	userPoints := make(map[string]float64)
	
	for phoneNumber := range userInfo {
		// Setiap sesi bernilai 20 poin
		userPoints[phoneNumber] = float64(userActivityCounts[phoneNumber] * 20)
	}
	
	// Tanggal kemarin dalam format Indonesia
	hariKemarin := yesterdayJkt.Format("02-01-2006")
	
	// Format pesan laporan
	msg := fmt.Sprintf("*Laporan Aktivitas Pomokit Kemarin (%s)*\n\n", hariKemarin)
	
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
		
		// Tampilkan data tanpa emoji peringkat
		msg += fmt.Sprintf("✅ %s (%s): %d sesi (+%.0f poin)\n", 
			displayName, up.PhoneNumber, up.ActivityCount, up.Points)
	}
	
	// Tambahkan motivasi berdasarkan total sesi
	msg += "\n"
	if totalAktivitasKemarin > 10 {
		msg += "💪 *Kerja bagus tim! Pertahankan semangat Pomodoro!*"
	} else if totalAktivitasKemarin > 5 {
		msg += "👍 *Semangat terus tim! Teknik Pomodoro membantu produktivitas.*"
	} else {
		msg += "🚀 *Mari tingkatkan sesi Pomodoro hari ini!*"
	}
	
	// Tambahkan catatan poin
	msg += "\n\n*Catatan: Setiap sesi Pomodoro bernilai 20 poin*"
	
	return msg, nil
}
