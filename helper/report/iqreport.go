package report

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fmt"

	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Struct untuk menyimpan data IQ Score
type IqScoreInfo struct {
	Name        string `bson:"name"`
	PhoneNumber string `bson:"phonenumber"`
	Score       string `bson:"score"`
	IQ          string `bson:"iq"`
	WaGroupID   string `bson:"wagroupid"`  // ✅ Ubah ke string, lalu kita proses ke slice
	CreatedAt   string `bson:"created_at"` // ✅ Tambahan untuk filter waktu
}

func GenerateRekapIqScoreByDay(db *mongo.Database, groupID string) (string, string, error) {
	dataIqScore, err := GetTotalDataIqMasuk(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ Score: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	var todayList []IqScoreInfo

	for _, info := range dataIqScore {
		if info.WaGroupID != groupID {
			continue
		}

		createdAt, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(info.CreatedAt), loc)
		if err != nil {
			continue
		}

		if createdAt.Format("2006-01-02") == today {
			todayList = append(todayList, info)
		}
	}

	// ✅ Jika tidak ada data hari ini, tetap kembalikan pesan biasa
	if len(todayList) == 0 {
		msg := "*🧠 Rekap Harian Tes IQ - " + today + "*\n\n"
		msg += "Belum ada peserta yang mengikuti tes IQ hari ini."
		return msg, "", nil
	}

	// ✅ Jika ada peserta
	msg := "*🧠 Rekap Harian Tes IQ - " + today + "*\n\n"
	msg += fmt.Sprintf("Total peserta hari ini: %d orang\n\n", len(todayList))

	for _, iq := range todayList {
		msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	return msg, todayList[0].PhoneNumber, nil
}

func GenerateRekapIqScoreByWeek(db *mongo.Database, groupID string) (string, string, error) {
	// Ambil semua data IQ
	dataIqScore, err := GetTotalDataIqMasuk(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ Score: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// Hitung awal minggu ini dan minggu lalu
	offset := (int(now.Weekday()) + 6) % 7 // Senin = 0
	seninIni := now.AddDate(0, 0, -offset).Truncate(24 * time.Hour)
	seninLalu := seninIni.AddDate(0, 0, -7)
	mingguLaluAkhir := seninIni.AddDate(0, 0, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	var thisWeek, lastWeek, total []IqScoreInfo

	for _, info := range dataIqScore {
		if info.WaGroupID != groupID {
			continue
		}

		// ✅ Parse created_at string → time.Time
		createdAt, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(info.CreatedAt), loc)
		if err != nil {
			continue // skip jika gagal parsing
		}

		// Masukkan semua ke total
		total = append(total, info)

		// ✅ Kategorikan berdasarkan minggu
		if createdAt.After(seninIni) {
			thisWeek = append(thisWeek, info)
		} else if createdAt.After(seninLalu) && createdAt.Before(mingguLaluAkhir) {
			lastWeek = append(lastWeek, info)
		}
	}

	if len(total) == 0 {
		return "", "", fmt.Errorf("tidak ada data IQ Score untuk grup %s", groupID)
	}

	// ✅ Susun pesan rekap
	msg := "*🧠 Laporan Tes IQ Berdasarkan Minggu*\n\n"

	// msg += fmt.Sprintf("📊 *Total Seluruh*: %d peserta\n", len(total))
	// for _, iq := range total {
	// 	msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	// }

	msg += fmt.Sprintf("\n📆 *Minggu Lalu*: %d peserta\n", len(lastWeek))
	for _, iq := range lastWeek {
		msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	msg += fmt.Sprintf("\n📅 *Minggu Ini*: %d peserta\n", len(thisWeek))
	for _, iq := range thisWeek {
		msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	return msg, total[0].PhoneNumber, nil
}

// ✅ **Fungsi untuk mengambil seluruh data IQ Score dari database**
func GetTotalDataIqMasuk(db *mongo.Database) ([]IqScoreInfo, error) {
	collection := db.Collection("iqscore")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data IQ Score dari database: %v", err)
	}
	defer cursor.Close(context.TODO())

	var users []IqScoreInfo
	if err = cursor.All(context.TODO(), &users); err != nil {
		return nil, fmt.Errorf("gagal membaca data IQ Score: %v", err)
	}

	// **Konversi wagroupid dari string ke slice**
	for i, user := range users {
		// **Pastikan tidak ada spasi ekstra atau koma di akhir**
		cleanedGroupID := strings.TrimSpace(user.WaGroupID)
		users[i].WaGroupID = cleanedGroupID
	}

	return users, nil
}

// ✅ **Fungsi untuk mendapatkan Group ID berdasarkan nomor telepon**
func GetGroupIDFromProject(db *mongo.Database, phoneNumbers []string) (map[string][]string, error) {
	// **Filter mencari grup berdasarkan anggota dengan nomor telepon yang cocok**
	filter := bson.M{
		"members": bson.M{
			"$elemMatch": bson.M{"phonenumber": bson.M{"$in": phoneNumbers}},
		},
	}

	// **Ambil daftar semua dokumen yang sesuai**
	cursor, err := db.Collection("project").Find(context.TODO(), filter)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data proyek: %v", err)
	}
	defer cursor.Close(context.TODO())

	// **Map untuk menyimpan grup ID unik berdasarkan nomor telepon**
	groupMap := make(map[string]map[string]bool)

	for cursor.Next(context.TODO()) {
		var project struct {
			Members []struct {
				PhoneNumber string `bson:"phonenumber"`
			} `bson:"members"`
			WaGroupID string `bson:"wagroupid"`
		}

		if err := cursor.Decode(&project); err != nil {
			return nil, fmt.Errorf("gagal mendekode proyek: %v", err)
		}

		// **Simpan grup ID berdasarkan nomor telepon yang sesuai**
		for _, member := range project.Members {
			phone := member.PhoneNumber
			if Contain(phoneNumbers, phone) { // Pastikan nomor ada dalam daftar yang dicari
				if _, exists := groupMap[phone]; !exists {
					groupMap[phone] = make(map[string]bool)
				}
				groupMap[phone][project.WaGroupID] = true
			}
		}
	}

	// **Konversi map ke slice unik**
	finalGroupMap := make(map[string][]string)
	for phone, groups := range groupMap {
		for groupID := range groups {
			finalGroupMap[phone] = append(finalGroupMap[phone], groupID)
		}
	}

	return finalGroupMap, nil
}

// Fungsi bantuan untuk mengecek apakah sebuah nilai ada dalam slice
func Contain(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func GetLastWeekDataIQScores(db *mongo.Database, phonenumber string) (model.ActivityScore, error) {
	var activityscore model.ActivityScore

	// Hitung waktu 7 hari yang lalu
	lastWeek := time.Now().AddDate(0, 0, -7)

	// Filter data berdasarkan nomor telepon dan tanggal dalam seminggu terakhir
	filter := bson.M{
		"phonenumber": phonenumber,
		"created_at": bson.M{
			"$gte": lastWeek.Format("2006-01-02 15:04:05"),
		},
	}

	sort := bson.M{"created_at": -1} // Urutkan berdasarkan tanggal paling terbaru

	// Ambil data pertama yang sesuai
	cursor, err := db.Collection("iqscore").Find(context.TODO(), filter, options.Find().SetSort(sort).SetLimit(1))
	if err != nil {
		return activityscore, err
	}
	defer cursor.Close(context.TODO())

	if cursor.Next(context.TODO()) {
		var iqDoc model.UserWithIqScore
		if err := cursor.Decode(&iqDoc); err != nil {
			return activityscore, err
		}

		scoreInt, _ := strconv.Atoi(iqDoc.Score)
		iqInt, _ := strconv.Atoi(iqDoc.IQ)

		activityscore.IQ = iqInt
		activityscore.IQresult = scoreInt
		activityscore.PhoneNumber = phonenumber
		activityscore.CreatedAt = time.Now()
	} else {
		return activityscore, fmt.Errorf("data IQ tidak ditemukan dalam seminggu terakhir untuk nomor telepon %s", phonenumber)
	}

	return activityscore, nil
}

func GetCreated_At(t time.Time) string {
	// Load zona waktu Asia/Jakarta (WIB)
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Jika gagal load lokasi, fallback ke UTC
		loc = time.UTC
	}

	// Ambil waktu saat ini dalam zona WIB
	now := time.Now().In(loc)

	// Hitung created_at (Senin = 1, Minggu = 7)
	created_at := int(now.Weekday())
	if created_at == 0 {
		created_at = 7 // Minggu
	}

	// Dapatkan tanggal Senin minggu ini pukul 17:01:00 WIB
	monday := now.AddDate(0, 0, -created_at+1)
	startOfWeek := time.Date(monday.Year(), monday.Month(), monday.Day(), 17, 1, 0, 0, loc)

	// Jika sekarang sebelum Senin jam 17:01, kita hitung mundur ke minggu sebelumnya
	if now.Before(startOfWeek) {
		startOfWeek = startOfWeek.AddDate(0, 0, -7)
	}

	// Ambil ISO week dan tahun dari startOfWeek
	year, week := startOfWeek.ISOWeek()

	// Format: "2025_18"
	return fmt.Sprintf("%d_%02d", year, week)
}

func GetLastWeekDataIQScoress(db *mongo.Database, phonenumber, mode string) (model.ActivityScore, error) {
	var activityscore model.ActivityScore

	// WIB
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	var startTime, endTime time.Time

	switch mode {
	case "kelasws":
		// minggu berjalan Jumat 00:00 WIB → Jumat depan 00:00
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		// cari Jumat terakhir (weekday==5)
		daysSinceFri := (wd + 2) % 7 // jika today is Fri (5), daysSinceFri=0
		lastFri := now.AddDate(0, 0, -daysSinceFri)
		startTime = time.Date(lastFri.Year(), lastFri.Month(), lastFri.Day(), 0, 0, 0, 0, loc)
		endTime = startTime.AddDate(0, 0, 7)

	case "proyek1":
		// minggu Senin 17:01 → Senin depan 17:00
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		// jika hari ini Senin sebelum jam 17:01, geser ke minggu kemarin
		if wd == 1 && (now.Hour() < 17 || (now.Hour() == 17 && now.Minute() < 1)) {
			now = now.AddDate(0, 0, -1)
			wd = int(now.Weekday())
			if wd == 0 {
				wd = 7
			}
		}
		mon := now.AddDate(0, 0, -wd+1)
		startTime = time.Date(mon.Year(), mon.Month(), mon.Day(), 17, 1, 0, 0, loc)
		// Senin depan jam 17:00
		nxtMon := startTime.AddDate(0, 0, 7)
		endTime = time.Date(nxtMon.Year(), nxtMon.Month(), nxtMon.Day(), 17, 0, 0, 0, loc)

	default:
		// fallback: 7×24 jam terakhir
		startTime = now.AddDate(0, 0, -7)
		endTime = now
	}

	// MongoDB menyimpan created_at sebagai string "YYYY-MM-DD HH:MM:SS"
	// filter lexikografis cukup jika formatnya konsisten
	filter := bson.M{
		"phonenumber": phonenumber,
		"created_at": bson.M{
			"$gte": startTime.Format("2006-01-02 15:04:05"),
			"$lt":  endTime.Format("2006-01-02 15:04:05"),
		},
	}

	// ambil satu dokumen terbaru
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(1)

	cursor, err := db.Collection("iqscore").
		Find(context.TODO(), filter, opts)
	if err != nil {
		return activityscore, fmt.Errorf("query iqscore: %w", err)
	}
	defer cursor.Close(context.TODO())

	if !cursor.Next(context.TODO()) {
		return activityscore, fmt.Errorf("tidak ada data IQ Score untuk mode %q", mode)
	}

	// decode ke struct yang punya Score & IQ sebagai string
	var doc struct {
		Score string `bson:"score"`
		IQ    string `bson:"iq"`
	}
	if err := cursor.Decode(&doc); err != nil {
		return activityscore, fmt.Errorf("decode iqscore: %w", err)
	}

	// konversi ke int
	scoreInt, _ := strconv.Atoi(doc.Score)
	iqInt, _ := strconv.Atoi(doc.IQ)

	activityscore.PhoneNumber = phonenumber
	activityscore.IQresult = scoreInt
	activityscore.IQ = iqInt
	activityscore.CreatedAt = time.Now() // catat kapan dipanggil

	return activityscore, nil
}

func GetLastWeekDataIQScoreKelas(db *mongo.Database, phonenumber string, usedIDs []primitive.ObjectID) (resultid []primitive.ObjectID, activityscore model.ActivityScore, err error) {
	oneWeekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02 15:04:05")

	// Buat filter untuk stravapoin1 agar id nya tidak ada di usedIDs
	filter1 := bson.M{
		"_id":         bson.M{"$nin": usedIDs},
		"phonenumber": phonenumber,
		"created_at": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	type IQDoc struct {
		ID    primitive.ObjectID `bson:"_id"`
		Score string             `bson:"score"`
		IQ    string             `bson:"iq"`
	}

	doc, err := atdb.GetOneDoc[IQDoc](db, "iqscore", filter1)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, activityscore, nil
		}
		return nil, activityscore, err
	}

	// Ambil data dari dokumen yang ditemukan
	scoreInt, _ := strconv.Atoi(doc.Score)
	iqInt, _ := strconv.Atoi(doc.IQ)

	activityscore.IQresult = scoreInt
	activityscore.IQ = iqInt

	// Ambil ID dokumen yang digunakan
	resultid = append(resultid, doc.ID)

	return resultid, activityscore, nil
}
