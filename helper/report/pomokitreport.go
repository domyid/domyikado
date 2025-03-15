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

// CountPomokitActivity menghitung aktivitas pomokit berdasarkan nomor telepon
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
				// Tambahkan data baru
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
		// Coba alternatif format response
		var apiResponse model.PomokitResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
			return nil, errors.New("Invalid API Response: " + err.Error())
		}
		pomodoroReports = apiResponse.Data
	}

	// Filter berdasarkan tanggal dari CreateAt
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

// TambahPoinPomokitByPhoneNumber menambahkan poin berdasarkan aktivitas Pomokit
func TambahPoinPomokitByPhoneNumber(db *mongo.Database, phonenumber string, poin float64) (model.Userdomyikado, error) {
	usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		return usr, err
	}
	usr.Poin = usr.Poin + poin
	_, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phonenumber}, usr)
	if err != nil {
		return usr, err
	}

	logpoin := LogPoin{
		UserID:      usr.ID,
		Name:        usr.Name,
		PhoneNumber: usr.PhoneNumber,
		Email:       usr.Email,
		Poin:        poin,
		Activity:    "Pomodoro Session",
	}
	
	// Simpan log poin
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return usr, err
	}

	return usr, nil
}

// GeneratePomokitRekapHarian membuat rekap aktivitas Pomokit harian
func GeneratePomokitRekapHarian(db *mongo.Database) (msg string, err error) {
	// Ambil data Pomokit hari ini
	pomokitData, err := GetPomokitDataHarian(db, TodayFilter())
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data Pomokit: %v", err)
	}
	
	// Buat pesan rekap
	msg = "*Rekap Aktivitas Pomodoro Hari Ini:*\n\n"
	
	// Cek apakah hari libur
	isLibur := HariLibur(GetDateSekarang())
	
	// Hitung aktivitas per pengguna
	pomokitCounts := CountPomokitActivity(pomokitData)
	
	// Ambil semua pengguna
	allUsers, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", bson.M{})
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data pengguna: %v", err)
	}
	
	// Buat map untuk status aktivitas
	userActivityStatus := make(map[string]struct{
		Name       string
		Phone      string
		PointChange float64
		TotalPoint float64
		IsActive   bool
		GroupID    string
	})
	
	// Isi map untuk pengguna yang aktif
	for phoneNumber, info := range pomokitCounts {
		// Dapatkan GroupID dari data Pomokit
		groupID := ""
		for _, report := range pomokitData {
			if report.PhoneNumber == phoneNumber {
				groupID = report.WaGroupID
				break
			}
		}
		
		// Tambahkan poin untuk setiap sesi
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
	
	// Jika bukan hari libur, kurangi poin untuk pengguna yang tidak aktif
	if !isLibur {
		for _, user := range allUsers {
			if user.PhoneNumber != "" {
				// Cek apakah user tidak ada di daftar aktif
				if _, exists := userActivityStatus[user.PhoneNumber]; !exists {
					// Cari GroupID dari proyek yang terkait dengan pengguna
					groupID := ""
					projectFilter := bson.M{"members.phonenumber": user.PhoneNumber}
					projects, _ := atdb.GetAllDoc[[]model.Project](db, "project", projectFilter)
					if len(projects) > 0 {
						groupID = projects[0].WAGroupID
					}
					
					// Kurangi poin
					totalPoin, err := DeductPomokitPoints(db, user.PhoneNumber, 1, groupID)
					if err != nil {
						continue
					}
					
					// Tambahkan ke status
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
    // Check if points already added today for this phone number
    today := time.Now().Truncate(24 * time.Hour)
    tomorrow := today.Add(24 * time.Hour)
    
    // Look for existing points record for today
    var existingPoints []PomokitPoin
    filter := bson.M{
        "phonenumber": phoneNumber,
        "groupid": groupID,
        "activity": "Pomodoro Session",
        "createdat": bson.M{
            "$gte": today,
            "$lt": tomorrow,
        },
    }
    
    existingPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", filter)
    if err == nil && len(existingPoints) > 0 {
        // Points already added today, return the current total
        return GetTotalPomokitPoints(db, phoneNumber)
    }

    // Proceed with adding points as before
    usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phoneNumber})
    if err != nil {
        return 0, err
    }
    
    // Buat entry untuk pomokitpoin dengan timestamp
    pomokitPoin := PomokitPoin{
        UserID:      usr.ID,
        Name:        usr.Name,
        PhoneNumber: usr.PhoneNumber,
        Email:       usr.Email,
        GroupID:     groupID,
        Poin:        points,
        Activity:    "Pomodoro Session",
        CreatedAt:   time.Now(), // Add current timestamp
    }
    
    // Save to pomokitpoin collection
    _, err = atdb.InsertOneDoc(db, "pomokitpoin", pomokitPoin)
    if err != nil {
        return 0, err
    }
    
    // Get total points from pomokitpoin collection
    totalPoin, err = GetTotalPomokitPoints(db, phoneNumber)
    if err != nil {
        return 0, err
    }
    
    // Update user's points in user collection
    usr.Poin = totalPoin
    _, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phoneNumber}, usr)
    if err != nil {
        return totalPoin, nil
    }
    
    return totalPoin, nil
}

// Helper function untuk mengurangi poin Pomokit
func DeductPomokitPoints(db *mongo.Database, phoneNumber string, points float64, groupID string) (totalPoin float64, err error) {
    // Check if points already deducted today for this phone number
    today := time.Now().Truncate(24 * time.Hour)
    tomorrow := today.Add(24 * time.Hour)
    
    // Look for existing deduction record for today
    var existingDeductions []PomokitPoin
    filter := bson.M{
        "phonenumber": phoneNumber,
        "groupid": groupID,
        "activity": "No Pomodoro Session",
        "createdat": bson.M{
            "$gte": today,
            "$lt": tomorrow,
        },
    }
    
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
        GroupID:     groupID,
        Poin:        -points,
        Activity:    "No Pomodoro Session",
        CreatedAt:   time.Now(), // Add current timestamp
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
        return totalPoin, nil
    }
    
    return totalPoin, nil
}

func GeneratePomokitRekapHarianByGroupID(db *mongo.Database, groupID string) (msg string, err error) {
    filter := bson.M{}
    if groupID != "" {
        filter = bson.M{"wagroupid": groupID}
    }
    
    pomokitData, err := GetPomokitDataHarian(db, filter)
    if err != nil {
        return "", fmt.Errorf("gagal mengambil data Pomokit: %v", err)
    }
	
	// Filter data berdasarkan WAGroupID jika groupID tidak kosong
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
			// Ambil semua member dari project
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
	
	// Buat map untuk status aktivitas
	userActivityStatus := make(map[string]struct{
		Name       string
		Phone      string
		PointChange float64
		TotalPoint float64
		IsActive   bool
	})
	
	// Isi map untuk pengguna yang aktif
	for phoneNumber, info := range pomokitCounts {
		// Tambahkan poin untuk setiap sesi
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
	
	// Jika bukan hari libur, kurangi poin untuk anggota grup yang tidak aktif
	if !isLibur && len(groupMembers) > 0 {
		for _, user := range groupMembers {
			// Cek apakah user tidak ada di daftar aktif
			if _, exists := userActivityStatus[user.PhoneNumber]; !exists && user.PhoneNumber != "" {
				// Kurangi poin
				totalPoin, err := DeductPomokitPoints(db, user.PhoneNumber, 1, groupID)
				if err != nil {
					continue
				}
				
				// Tambahkan ke status
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

func GetTotalPomokitPoints(db *mongo.Database, phoneNumber string) (totalPoin float64, err error) {
	// Ambil semua record pomokitpoin untuk nomor telepon tertentu
	var pomokitPoints []PomokitPoin
	pomokitPoints, err = atdb.GetAllDoc[[]PomokitPoin](db, "pomokitpoin", bson.M{"phonenumber": phoneNumber})
	if err != nil {
		return 0, err
	}
	
	// Hitung total poin
	totalPoin = 0
	for _, record := range pomokitPoints {
		totalPoin += record.Poin
	}
	
	return totalPoin, nil
}

// Helper function untuk cek hyphen dalam string
func containsHyphen(s string) bool {
	for _, char := range s {
		if char == '-' {
			return true
		}
	}
	return false
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