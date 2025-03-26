package report

import (
	"context"
	"fmt"
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

// Fungsi untuk mengambil data GTMetrix dari webhook
func GetGTMetrixData(db *mongo.Database, onlyYesterday bool, onlyLastWeek bool) ([]model.GTMetrixInfo, error) {
	// Buat filter waktu berdasarkan parameter
	var timeFilter bson.M

	if onlyYesterday {
		// Filter untuk kemarin
		startOfYesterday, endOfYesterday := getStartAndEndOfYesterday(time.Now())
		timeFilter = bson.M{
			"createdAt": bson.M{
				"$gte": startOfYesterday,
				"$lt":  endOfYesterday,
			},
		}
	} else if onlyLastWeek {
		// Filter untuk seminggu terakhir
		weekAgo := time.Now().AddDate(0, 0, -7)
		timeFilter = bson.M{
			"createdAt": bson.M{
				"$gte": weekAgo,
			},
		}
	} else {
		// Tidak ada filter waktu, ambil semua data
		timeFilter = bson.M{}
	}

	// Tambahkan filter hanya untuk data yang memiliki gtmetrix_grade
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

	// Ambil data dari collection webhook_pomokit
	cursor, err := db.Collection("webhook_pomokit").Find(context.TODO(), filter)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data GTMetrix: %v", err)
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
			return nil, fmt.Errorf("gagal decode dokumen: %v", err)
		}

		// Skip jika grade kosong
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

		// Jika kita hanya perlu data terbaru untuk setiap nomor telepon
		if !onlyYesterday && !onlyLastWeek {
			// Simpan hanya data terbaru untuk setiap nomor telepon
			existing, exists := phoneToLatestData[doc.PhoneNumber]
			if !exists || doc.CreatedAt.After(existing.CreatedAt) {
				phoneToLatestData[doc.PhoneNumber] = info
			}
		} else {
			// Untuk kemarin atau seminggu terakhir, simpan semua data
			results = append(results, info)
		}
	}

	// Jika kita hanya mengambil data terbaru, konversi map ke slice
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