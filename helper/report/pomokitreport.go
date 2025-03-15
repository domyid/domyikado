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
		return nil, errors.New("Invalid API Response: " + err.Error())
	}

	// Filter berdasarkan tanggal dari ObjectID
	var filteredReports []model.PomodoroReport
	for _, report := range pomodoroReports {
		// Asumsikan CreatedAt sudah diset di model
		if report.CreatedAt.After(time.Now().AddDate(0, 0, -1)) && 
		   report.CreatedAt.Before(time.Now()) {
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
	})
	
	// Isi map untuk pengguna yang aktif
	for phoneNumber, info := range pomokitCounts {
		// Tambahkan poin untuk setiap sesi
		user, err := TambahPoinPomokitByPhoneNumber(db, phoneNumber, info.Count)
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
			PointChange: info.Count,
			TotalPoint: user.Poin,
			IsActive:   true,
		}
	}
	
	// Jika bukan hari libur, kurangi poin untuk pengguna yang tidak aktif
	if !isLibur {
		for _, user := range allUsers {
			if user.PhoneNumber != "" {
				// Cek apakah user tidak ada di daftar aktif
				if _, exists := userActivityStatus[user.PhoneNumber]; !exists {
					// Kurangi poin
					KurangPoinUserbyPhoneNumber(db, user.PhoneNumber, 1)
					updatedUser, _ := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": user.PhoneNumber})
					
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
						TotalPoint: updatedUser.Poin,
						IsActive:   false,
					}
					
					// Log pengurangan poin
					logpoin := LogPoin{
						UserID:      user.ID,
						Name:        user.Name,
						PhoneNumber: user.PhoneNumber,
						Email:       user.Email,
						Poin:        -1,
						Activity:    "No Pomodoro Session",
					}
					atdb.InsertOneDoc(db, "logpoin", logpoin)
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