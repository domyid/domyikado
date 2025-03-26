package report

import (
	"bytes"
	"context"
	"encoding/json"
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

func GetGTMetrixData(db *mongo.Database, onlyYesterday bool, onlyLastWeek bool) ([]model.GTMetrixInfo, error) {
    // Get configuration for the API URL
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := db.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, fmt.Errorf("Config Not Found: %v", err)
    }
    
    // Pastikan URL tersedia
    if conf.PomokitUrl == "" {
        return nil, fmt.Errorf("PomokitUrl not configured")
    }
    
    fmt.Printf("DEBUG: Fetching GTMetrix data from %s\n", conf.PomokitUrl)
    
    // Make HTTP request to the API
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.PomokitUrl)
    if err != nil {
        return nil, fmt.Errorf("API Connection Failed: %v", err)
    }
    defer resp.Body.Close()
    
    // Handle non-200 status
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API Returned Status %d", resp.StatusCode)
    }
    
    // Read response body
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("Error reading response: %v", err)
    }
    
    // Create new reader for JSON decoding
    resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
    
    // Struktur data dari API Pomokit
    type PomokitData struct {
        ID                 struct {
            Oid string     `json:"$oid"`
        } `json:"_id"`
        Name               string `json:"name"`
        PhoneNumber        string `json:"phonenumber"`
        Cycle              int    `json:"cycle"`
        Hostname           string `json:"hostname"`
        IP                 string `json:"ip"`
        Screenshots        int    `json:"screenshots"`
        Pekerjaan          string `json:"pekerjaan"`
        Token              string `json:"token"`
        URLPekerjaan       string `json:"urlpekerjaan"`
        WaGroupID          string `json:"wagroupid"`
        GTMetrixURLTarget  string `json:"gtmetrix_url_target"`
        GTMetrixGrade      string `json:"gtmetrix_grade"`
        GTMetrixPerf       string `json:"gtmetrix_performance"`
        GTMetrixStruct     string `json:"gtmetrix_structure"`
        LCP                string `json:"lcp"`
        TBT                string `json:"tbt"`
        CLS                string `json:"cls"`
        CreatedAt          struct {
            Date string     `json:"$date"`
        } `json:"createdAt"`
    }
    
    // Decode ke struktur yang sesuai dengan API
    var pomokitData []PomokitData
    if err := json.NewDecoder(resp.Body).Decode(&pomokitData); err != nil {
        // Coba format alternatif jika gagal
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
        var wrappedData struct {
            Success bool         `json:"success"`
            Data    []PomokitData `json:"data"`
            Message string       `json:"message,omitempty"`
        }
        
        if err := json.NewDecoder(resp.Body).Decode(&wrappedData); err != nil {
            fmt.Printf("ERROR: Failed to parse API response. Raw response: %s\n", string(responseBody))
            return nil, fmt.Errorf("Invalid API Response: %v", err)
        }
        
        pomokitData = wrappedData.Data
        fmt.Printf("DEBUG: Decoded as wrapped response struct with %d reports\n", len(pomokitData))
    } else {
        fmt.Printf("DEBUG: Decoded as direct array with %d reports\n", len(pomokitData))
    }
    
    if len(pomokitData) == 0 {
        fmt.Printf("WARNING: No reports found in API response\n")
        return nil, nil
    }
    
    // Konversi dan filter data yang memiliki GTMetrix grade
    var gtMetrixInfos []model.GTMetrixInfo
    for _, data := range pomokitData {
        // Skip jika tidak ada grade GTMetrix
        if data.GTMetrixGrade == "" {
            continue
        }
        
        // Parse created date
        var createdAt time.Time
        var parseErr error
        
        if data.CreatedAt.Date != "" {
            // Coba beberapa format tanggal yang mungkin
            formats := []string{
                time.RFC3339,
                time.RFC3339Nano,
                "2006-01-02T15:04:05.999Z",
                "2006-01-02T15:04:05Z",
            }
            
            for _, format := range formats {
                createdAt, parseErr = time.Parse(format, data.CreatedAt.Date)
                if parseErr == nil {
                    break
                }
            }
            
            if parseErr != nil {
                fmt.Printf("WARNING: Failed to parse date %s, using current time\n", data.CreatedAt.Date)
                createdAt = time.Now()
            }
        } else {
            // Jika tanggal tidak ada, gunakan waktu sekarang
            createdAt = time.Now()
        }
        
        // Konversi ke model GTMetrixInfo
        info := model.GTMetrixInfo{
            Name:             data.Name,
            PhoneNumber:      data.PhoneNumber,
            Grade:            data.GTMetrixGrade,
            Points:           gradeToPoints(data.GTMetrixGrade),
            WaGroupID:        data.WaGroupID,
            CreatedAt:        createdAt,
            PerformanceScore: data.GTMetrixPerf,
            StructureScore:   data.GTMetrixStruct,
            LCP:              data.LCP,
            TBT:              data.TBT,
            CLS:              data.CLS,
        }
        
        // Tambahkan ke hasil
        gtMetrixInfos = append(gtMetrixInfos, info)
    }
    
    fmt.Printf("DEBUG: Found %d records with GTMetrix data\n", len(gtMetrixInfos))
    
    // Jika tidak ada data GTMetrix valid, fallback ke database
    if len(gtMetrixInfos) == 0 {
        fmt.Printf("DEBUG: No valid GTMetrix data from API, falling back to database\n")
        return getGTMetrixDataFromDB(db, onlyYesterday, onlyLastWeek)
    }
    
    // Filter berdasarkan waktu jika diperlukan
    if onlyYesterday || onlyLastWeek {
        var filteredResults []model.GTMetrixInfo
        
        for _, info := range gtMetrixInfos {
            if shouldIncludeGTMetrixResult(info, onlyYesterday, onlyLastWeek) {
                filteredResults = append(filteredResults, info)
            }
        }
        
        fmt.Printf("DEBUG: After time filtering: %d records\n", len(filteredResults))
        
        // Jika hasil filter kosong, coba dari database
        if len(filteredResults) == 0 {
            fmt.Printf("DEBUG: No data after time filtering, falling back to database\n")
            return getGTMetrixDataFromDB(db, onlyYesterday, onlyLastWeek)
        }
        
        return filteredResults, nil
    }
    
    return gtMetrixInfos, nil
}


func shouldIncludeGTMetrixResult(info model.GTMetrixInfo, onlyYesterday bool, onlyLastWeek bool) bool {
    if !onlyYesterday && !onlyLastWeek {
        return true
    }
    
    createdAt := info.CreatedAt
    
    if onlyYesterday {
        start, end := getStartAndEndOfYesterday(time.Now())
        result := createdAt.After(start) && createdAt.Before(end)
        fmt.Printf("DEBUG: Yesterday Filter - Date: %v, Start: %v, End: %v, Include: %v\n", 
                  createdAt.Format(time.RFC3339), 
                  start.Format(time.RFC3339), 
                  end.Format(time.RFC3339), 
                  result)
        return result
    }
    
    if onlyLastWeek {
        weekAgo := time.Now().AddDate(0, 0, -7)
        result := createdAt.After(weekAgo)
        fmt.Printf("DEBUG: LastWeek Filter - Date: %v, WeekAgo: %v, Include: %v\n", 
                  createdAt.Format(time.RFC3339), 
                  weekAgo.Format(time.RFC3339), 
                  result)
        return result
    }
    
    return false
}

func getGTMetrixDataFromDB(db *mongo.Database, onlyYesterday bool, onlyLastWeek bool) ([]model.GTMetrixInfo, error) {
    // Create time filter based on parameters
    var timeFilter bson.M

    if onlyYesterday {
        // Filter for yesterday
        startOfYesterday, endOfYesterday := getStartAndEndOfYesterday(time.Now())
        timeFilter = bson.M{
            "createdAt": bson.M{
                "$gte": startOfYesterday,
                "$lt":  endOfYesterday,
            },
        }
    } else if onlyLastWeek {
        // Filter for last week
        weekAgo := time.Now().AddDate(0, 0, -7)
        timeFilter = bson.M{
            "createdAt": bson.M{
                "$gte": weekAgo,
            },
        }
    } else {
        // No time filter, get all data
        timeFilter = bson.M{}
    }

    // Add filter only for data with gtmetrix_grade
    filter := bson.M{
        "$and": []bson.M{
            timeFilter,
            {
                "gtmetrix_grade": bson.M{
                    "$exists": true,
                },
            },
        },
    }

    // Get data from webhook_pomokit collection
    cursor, err := db.Collection("webhook_pomokit").Find(context.TODO(), filter)
    if err != nil {
        return nil, fmt.Errorf("failed to get GTMetrix data: %v", err)
    }
    defer cursor.Close(context.TODO())

    var results []model.GTMetrixInfo
    var phoneToLatestData = make(map[string]model.GTMetrixInfo)

    for cursor.Next(context.TODO()) {
        var doc struct {
            Name                string    `bson:"name"`
            PhoneNumber         string    `bson:"phonenumber"`
            WaGroupID           string    `bson:"wagroupid"`
            GTMetrixGrade       string    `bson:"gtmetrix_grade"`
            GTMetrixPerformance string    `bson:"gtmetrix_performance"`
            GTMetrixStructure   string    `bson:"gtmetrix_structure"`
            LCP                 string    `bson:"lcp"`
            TBT                 string    `bson:"tbt"`
            CLS                 string    `bson:"cls"`
            CreatedAt           time.Time `bson:"createdAt"`
        }

        if err := cursor.Decode(&doc); err != nil {
            return nil, fmt.Errorf("failed to decode document: %v", err)
        }

        // Skip if grade is empty
        if doc.GTMetrixGrade == "" {
            continue
        }

        info := model.GTMetrixInfo{
            Name:             doc.Name,
            PhoneNumber:      doc.PhoneNumber,
            Grade:            doc.GTMetrixGrade,
            Points:           gradeToPoints(doc.GTMetrixGrade),
            WaGroupID:        doc.WaGroupID,
            CreatedAt:        doc.CreatedAt,
            PerformanceScore: doc.GTMetrixPerformance,
            StructureScore:   doc.GTMetrixStructure,
            LCP:              doc.LCP,
            TBT:              doc.TBT,
            CLS:              doc.CLS,
        }

        // If we only need latest data for each phone number
        if !onlyYesterday && !onlyLastWeek {
            // Save only the latest data for each phone number
            existing, exists := phoneToLatestData[doc.PhoneNumber]
            if !exists || doc.CreatedAt.After(existing.CreatedAt) {
                phoneToLatestData[doc.PhoneNumber] = info
            }
        } else {
            // For yesterday or last week, save all data
            results = append(results, info)
        }
    }

    // If we're only getting latest data, convert map to slice
    if !onlyYesterday && !onlyLastWeek {
        for _, info := range phoneToLatestData {
            results = append(results, info)
        }
    }

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