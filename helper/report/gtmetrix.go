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
	"strings"
	"time"

	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Fungsi untuk mengkonversi grade GTMetrix ke poin
func gradeToPoints(grade string) float64 {
	switch strings.ToUpper(grade) {
	case "A":
		return 100
	case "B":
		return 75
	case "C":
		return 50
	case "D":
		return 25
	default:
		return 0
	}
}

// Fungsi untuk mengambil data GTMetrix dari API
func GetGTMetrixData(db *mongo.Database, onlyYesterday bool, onlyLastWeek bool) ([]model.GTMetrixInfo, error) {
    // Ambil konfigurasi
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, errors.New("Config Not Found: " + err.Error())
    }

    fmt.Printf("DEBUG: Mengambil data GTMetrix dari %s\n", conf.PomokitUrl)

    // HTTP Client request ke API GTMetrix
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
    var gtmetrixInfo []model.GTMetrixInfo
    if err := json.NewDecoder(resp.Body).Decode(&gtmetrixInfo); err != nil {
        // Reset reader and try as wrapped response
        resp.Body.Close()
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
        
        var apiResponse struct {
            Success bool                `json:"success"`
            Data    []model.GTMetrixInfo `json:"data"`
            Message string              `json:"message,omitempty"`
        }
        
        if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
            fmt.Printf("ERROR: Failed to parse API response. Raw response: %s\n", string(responseBody))
            return nil, errors.New("Invalid API Response: " + err.Error())
        }
        
        gtmetrixInfo = apiResponse.Data
        fmt.Printf("DEBUG: Decoded as wrapped response with %d GTMetrix reports\n", len(gtmetrixInfo))
    } else {
        fmt.Printf("DEBUG: Decoded as direct array with %d GTMetrix reports\n", len(gtmetrixInfo))
    }

    if len(gtmetrixInfo) == 0 {
        fmt.Printf("WARNING: No GTMetrix reports found in API response\n")
        return nil, nil
    }

    // Apply time filters if specified
    var results []model.GTMetrixInfo
    
    if onlyYesterday {
        startOfYesterday, endOfYesterday := getStartAndEndOfYesterday(time.Now())
        for _, info := range gtmetrixInfo {
            if (info.CreatedAt.After(startOfYesterday) || info.CreatedAt.Equal(startOfYesterday)) && 
               info.CreatedAt.Before(endOfYesterday) {
                results = append(results, info)
            }
        }
    } else if onlyLastWeek {
        weekAgo := time.Now().AddDate(0, 0, -7)
        for _, info := range gtmetrixInfo {
            if info.CreatedAt.After(weekAgo) {
                results = append(results, info)
            }
        }
    } else {
        results = gtmetrixInfo
    }

    fmt.Printf("DEBUG: Retrieved %d GTMetrix reports after filtering\n", len(results))
    
    return results, nil
}

// Fungsi untuk generate rekap GTMetrix (kemarin)
func GenerateGTMetrixReportYesterday(db *mongo.Database, groupID string) (string, error) {
	// Ambil data GTMetrix kemarin
	data, err := GetGTMetrixData(db, true, false)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
	}

	// Filter data berdasarkan groupID jika ada
	var filteredData []model.GTMetrixInfo
	for _, info := range data {
		if groupID == "" || info.WaGroupID == groupID {
			filteredData = append(filteredData, info)
		}
	}

	// Jika tidak ada data, return pesan kosong
	if len(filteredData) == 0 {
		return fmt.Sprintf("Tidak ada data GTMetrix kemarin untuk grup %s", groupID), nil
	}

	// Urutkan berdasarkan poin tertinggi
	sort.Slice(filteredData, func(i, j int) bool {
		return filteredData[i].Points > filteredData[j].Points
	})

	// Generate pesan
	yesterday := time.Now().AddDate(0, 0, -1).Format("02-01-2006")
	msg := fmt.Sprintf("*Laporan Poin GTMetrix Kemarin (%s)*\n\n", yesterday)

	for _, info := range filteredData {
		msg += fmt.Sprintf("✅ *%s* (%s): Grade %s (+%.0f poin)\n", 
			info.Name, info.PhoneNumber, info.Grade, info.Points)
	}

	return msg, nil
}

// Fungsi untuk generate rekap GTMetrix (seminggu terakhir)
func GenerateGTMetrixReportLastWeek(db *mongo.Database, groupID string) (string, error) {
	// Ambil data GTMetrix seminggu terakhir
	data, err := GetGTMetrixData(db, false, true)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
	}

	// Filter data berdasarkan groupID jika ada dan ambil data terbaru per user
	userLatestData := make(map[string]model.GTMetrixInfo)
	for _, info := range data {
		if groupID == "" || info.WaGroupID == groupID {
			existing, exists := userLatestData[info.PhoneNumber]
			if !exists || info.CreatedAt.After(existing.CreatedAt) {
				userLatestData[info.PhoneNumber] = info
			}
		}
	}

	// Konversi map ke slice
	var filteredData []model.GTMetrixInfo
	for _, info := range userLatestData {
		filteredData = append(filteredData, info)
	}

	// Jika tidak ada data, return pesan kosong
	if len(filteredData) == 0 {
		return fmt.Sprintf("Tidak ada data GTMetrix seminggu terakhir untuk grup %s", groupID), nil
	}

	// Urutkan berdasarkan poin tertinggi
	sort.Slice(filteredData, func(i, j int) bool {
		return filteredData[i].Points > filteredData[j].Points
	})

	// Generate pesan
	weekAgo := time.Now().AddDate(0, 0, -7).Format("02-01-2006")
	today := time.Now().Format("02-01-2006")
	msg := fmt.Sprintf("*Laporan Poin GTMetrix Seminggu Terakhir (%s s/d %s)*\n\n", weekAgo, today)

	for _, info := range filteredData {
		msg += fmt.Sprintf("✅ *%s* (%s): Grade %s (+%.0f poin)\n", 
			info.Name, info.PhoneNumber, info.Grade, info.Points)
	}

	return msg, nil
}

// Fungsi untuk generate rekap GTMetrix (total/keseluruhan)
func GenerateGTMetrixReportTotal(db *mongo.Database, groupID string) (string, error) {
	// Ambil data GTMetrix (data terakhir per user)
	data, err := GetGTMetrixData(db, false, false)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
	}

	// Filter data berdasarkan groupID jika ada
	var filteredData []model.GTMetrixInfo
	for _, info := range data {
		if groupID == "" || info.WaGroupID == groupID {
			filteredData = append(filteredData, info)
		}
	}

	// Jika tidak ada data, return pesan kosong
	if len(filteredData) == 0 {
		return fmt.Sprintf("Tidak ada data GTMetrix untuk grup %s", groupID), nil
	}

	// Urutkan berdasarkan poin tertinggi
	sort.Slice(filteredData, func(i, j int) bool {
		return filteredData[i].Points > filteredData[j].Points
	})

	// Generate pesan
	msg := "*Laporan Total Poin GTMetrix (Data Terbaru)*\n\n"

	for _, info := range filteredData {
		performanceInfo := ""
		if info.PerformanceScore != "" && info.StructureScore != "" {
			performanceInfo = fmt.Sprintf(" | Perf: %s, Struct: %s", 
				info.PerformanceScore, info.StructureScore)
		}
		
		msg += fmt.Sprintf("✅ *%s* (%s): Grade %s (+%.0f poin)%s\n", 
			info.Name, info.PhoneNumber, info.Grade, info.Points, performanceInfo)
	}

	// Tambahkan tabel konversi grade ke poin
	msg += "\n*Konversi Grade GTMetrix ke Poin:*\n"
	msg += "A = 100 poin\n"
	msg += "B = 75 poin\n"
	msg += "C = 50 poin\n"
	msg += "D = 25 poin\n"

	return msg, nil
}

// Fungsi untuk mendapatkan rentang waktu kemarin
// func getStartAndEndOfYesterday(t time.Time) (time.Time, time.Time) {
// 	location, _ := time.LoadLocation("Asia/Jakarta")
// 	today := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location)
// 	yesterday := today.AddDate(0, 0, -1)
// 	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
// 	end := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)

// 	return start, end
// }

// Fungsi untuk mengirim rekap GTMetrix ke grup WhatsApp
func SendGTMetrixReportToGroup(db *mongo.Database, groupID string, reportType string, token string, apiUrl string) (string, error) {
	var msg string
	var err error

	// Generate laporan berdasarkan tipe
	switch reportType {
	case "yesterday":
		msg, err = GenerateGTMetrixReportYesterday(db, groupID)
	case "lastweek":
		msg, err = GenerateGTMetrixReportLastWeek(db, groupID)
	case "total":
		msg, err = GenerateGTMetrixReportTotal(db, groupID)
	default:
		return "", fmt.Errorf("tipe laporan tidak valid: %s", reportType)
	}

	if err != nil {
		return "", err
	}

	// Jika tidak ada data, hanya kembalikan pesan tanpa mengirim
	if strings.Contains(msg, "Tidak ada data GTMetrix") {
		return msg, nil
	}

	// Siapkan pesan untuk dikirim ke WhatsApp
	dt := &whatsauth.TextMessage{
		To:       groupID,
		IsGroup:  true,
		Messages: msg,
	}

	// Kirim pesan ke API WhatsApp
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", token, dt, apiUrl)
	if err != nil {
		return "", fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
	}

	return "Laporan GTMetrix berhasil dikirim ke grup " + groupID, nil
}

// RekapGTMetrixHarian menjalankan rekap otomatis harian untuk semua grup
func RekapGTMetrixHarian(db *mongo.Database, token string, apiUrl string) error {
	// Ambil data GTMetrix kemarin
	data, err := GetGTMetrixData(db, true, false)
	if err != nil {
		return fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("tidak ada data GTMetrix untuk direkap")
	}

	// Kumpulkan semua group ID unik
	groupIDSet := make(map[string]bool)
	for _, info := range data {
		if info.WaGroupID != "" {
			groupIDSet[info.WaGroupID] = true
		}
	}

	// Jika tidak ada grup, hentikan proses
	if len(groupIDSet) == 0 {
		return fmt.Errorf("tidak ada grup dengan aktivitas GTMetrix")
	}

	var lastErr error

	// Kirim laporan ke setiap grup
	for groupID := range groupIDSet {
		// Skip jika groupID memiliki format tidak valid (private chat)
		if strings.Contains(groupID, "-") {
			continue
		}

		// Generate dan kirim laporan
		msg, err := GenerateGTMetrixReportYesterday(db, groupID)
		if err != nil {
			lastErr = err
			continue
		}

		// Skip jika tidak ada data
		if strings.Contains(msg, "Tidak ada data GTMetrix") {
			continue
		}

		// Kirim pesan
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", token, dt, apiUrl)
		if err != nil {
			lastErr = fmt.Errorf("gagal mengirim pesan ke %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

// RekapGTMetrixMingguan menjalankan rekap otomatis mingguan untuk semua grup
func RekapGTMetrixMingguan(db *mongo.Database, token string, apiUrl string) error {
	// Ambil data GTMetrix seminggu terakhir
	data, err := GetGTMetrixData(db, false, true)
	if err != nil {
		return fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("tidak ada data GTMetrix untuk direkap")
	}

	// Kumpulkan semua group ID unik
	groupIDSet := make(map[string]bool)
	for _, info := range data {
		if info.WaGroupID != "" {
			groupIDSet[info.WaGroupID] = true
		}
	}

	// Jika tidak ada grup, hentikan proses
	if len(groupIDSet) == 0 {
		return fmt.Errorf("tidak ada grup dengan aktivitas GTMetrix")
	}

	var lastErr error

	// Kirim laporan ke setiap grup
	for groupID := range groupIDSet {
		// Skip jika groupID memiliki format tidak valid (private chat)
		if strings.Contains(groupID, "-") {
			continue
		}

		// Generate dan kirim laporan
		msg, err := GenerateGTMetrixReportLastWeek(db, groupID)
		if err != nil {
			lastErr = err
			continue
		}

		// Skip jika tidak ada data
		if strings.Contains(msg, "Tidak ada data GTMetrix") {
			continue
		}

		// Kirim pesan
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", token, dt, apiUrl)
		if err != nil {
			lastErr = fmt.Errorf("gagal mengirim pesan ke %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}