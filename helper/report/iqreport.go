package report

import (
	"context"
	"strings"
	"time"

	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

// ✅ **Fungsi utama untuk menghasilkan rekap IQ Score yang akan dikirim ke WhatsApp**
func GenerateRekapPoinIqScore(db *mongo.Database, groupID string) (string, string, error) {
	// Ambil data IQ Score terbaru
	dataIqScore, err := GetTotalDataIqMasuk(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ Score: %v", err)
	}

	// Filter hanya data yang sesuai dengan Group ID
	var filteredData []IqScoreInfo
	for _, info := range dataIqScore {
		if info.WaGroupID == groupID { // ✅ Cek langsung sebagai string
			filteredData = append(filteredData, info)
		}
	}

	// Jika tidak ada data untuk grup ini, hentikan proses
	if len(filteredData) == 0 {
		return "", "", fmt.Errorf("tidak ada data IQ Score untuk grup %s", groupID)
	}

	// Buat pesan rekap
	msg := "*Laporan Total Skor Tes IQ*\n\n"
	for _, iq := range filteredData {
		msg += fmt.Sprintf("✅ *%s* - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	// Pilih perwakilan pertama sebagai nomor yang akan menerima pesan jika private chat
	perwakilanphone := filteredData[0].PhoneNumber

	return msg, perwakilanphone, nil
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

	msg += fmt.Sprintf("📊 *Total Seluruh*: %d peserta\n", len(total))
	for _, iq := range total {
		msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	msg += fmt.Sprintf("\n📅 *Minggu Ini*: %d peserta\n", len(thisWeek))
	for _, iq := range thisWeek {
		msg += fmt.Sprintf("✅ %s - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	msg += fmt.Sprintf("\n📆 *Minggu Lalu*: %d peserta\n", len(lastWeek))
	for _, iq := range lastWeek {
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
