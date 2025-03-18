package report

import (
	"context"
	// "errors"
	"fmt"
	"sort"
	"strconv"

	// "time"

	// "github.com/gocroot/helper/atapi"
	// "github.com/gocroot/helper/atdb"
	// "github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Struct untuk menyimpan data IQ Score
type IqScoreInfo struct {
	Name        string
	PhoneNumber string
	Score       string
	IQ          string
	WaGroupID   []string
}

// ✅ **Fungsi utama untuk menghasilkan rekap IQ Score yang akan dikirim ke WhatsApp**
func GenerateRekapPoinIqScore(db *mongo.Database, groupID string) (string, string, error) {
	// **Ambil data IQ Score dari database**
	dataIqScore, err := GetTotalDataIqMasuk(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ Score: %v", err)
	}

	// **Filter hanya data yang sesuai dengan Group ID**
	var filteredData []IqScoreInfo
	for _, info := range dataIqScore {
		for _, gid := range info.WaGroupID {
			if gid == groupID {
				filteredData = append(filteredData, info)
				break // Stop iterasi setelah menemukan kecocokan pertama
			}
		}
	}

	// **Jika tidak ada data untuk grup ini, hentikan proses**
	if len(filteredData) == 0 {
		return "", "", fmt.Errorf("tidak ada data IQ Score untuk grup %s", groupID)
	}

	// **Buat pesan rekap IQ Score**
	msg := "*📊 Rekapitulasi Hasil Tes IQ Harian 📊*\n\n"
	msg += formatIqScoreData(filteredData)

	// **Pilih perwakilan pertama sebagai penerima jika private chat**
	perwakilanphone := filteredData[0].PhoneNumber

	return msg, perwakilanphone, nil
}

// ✅ **Fungsi untuk mengambil seluruh data IQ Score dari database**
func GetTotalDataIqMasuk(db *mongo.Database) ([]IqScoreInfo, error) {
	// **Ambil semua data dari koleksi `iqscore` langsung dari MongoDB**
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

	// **Ambil daftar grup WhatsApp berdasarkan nomor telepon**
	phoneNumbers := extractUniquePhoneNumbers(users)
	groupMap, err := GetGroupIDFromProject(db, phoneNumbers)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil grup WhatsApp: %v", err)
	}

	// **Tambahkan WaGroupID ke setiap pengguna**
	for i, user := range users {
		if waGroups, exists := groupMap[user.PhoneNumber]; exists {
			users[i].WaGroupID = waGroups
		}
	}

	return users, nil
}

// ✅ **Fungsi untuk memformat data IQ Score dalam pesan WhatsApp**
func formatIqScoreData(data []IqScoreInfo) string {
	if len(data) == 0 {
		return "Tidak ada skor IQ yang tercatat."
	}

	// **Urutkan berdasarkan skor tertinggi**
	sort.Slice(data, func(i, j int) bool {
		scoreI, _ := strconv.Atoi(data[i].Score)
		scoreJ, _ := strconv.Atoi(data[j].Score)
		return scoreI > scoreJ
	})

	var result string
	for _, iq := range data {
		result += fmt.Sprintf("✅ *%s* - Skor: %s, IQ: %s\n", iq.Name, iq.Score, iq.IQ)
	}

	return result
}

// ✅ **Fungsi untuk mengambil nomor telepon unik dari daftar IQ Score**
func extractUniquePhoneNumbers(users []IqScoreInfo) []string {
	phoneSet := make(map[string]bool)
	var phoneNumbers []string

	for _, user := range users {
		if !phoneSet[user.PhoneNumber] {
			phoneSet[user.PhoneNumber] = true
			phoneNumbers = append(phoneNumbers, user.PhoneNumber)
		}
	}
	return phoneNumbers
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
