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

func GenerateRekapPoinIqScore(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	// Ambil data dari koleksi iqscore
	dailyScoreData, err := GetTotalDataIqMasuk(db, false)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ harian: %v", err)
	}
	totalScoreData, err := GetTotalDataIqMasuk(db, true)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data IQ total: %v", err)
	}

	// Buat list untuk menyimpan user yang termasuk dalam groupId
	var filteredDailyScoreData []IqScoreInfo
	var filteredTotalScoreData []IqScoreInfo

	// Filter dailyScoreData yang termasuk dalam groupId
	for _, info := range dailyScoreData {
		for _, gid := range info.WaGroupID {
			if gid == groupId {
				filteredDailyScoreData = append(filteredDailyScoreData, info)
				break
			}
		}
	}

	// Filter totalData yang termasuk dalam groupId
	for _, info := range totalScoreData {
		for _, gid := range info.WaGroupID {
			if gid == groupId {
				filteredTotalScoreData = append(filteredTotalScoreData, info)
				break
			}
		}
	}

	// Jika grup tidak memiliki anggota, jangan kirim pesan
	if len(filteredDailyScoreData) == 0 && len(filteredTotalScoreData) == 0 {
		return "", "", fmt.Errorf("tidak ada data Score IQ untuk grup %s", groupId)
	}

	// Format pesan
	msg = "*Laporan Score IQ:*"
	msg += "\n\n*Kemarin - Hari Ini:*\n"
	msg += formatIqScoreData(filteredDailyScoreData)

	msg += "\n\n*Total Keseluruhan:*\n"
	msg += formatIqScoreData(filteredTotalScoreData)

	msg += "\n\nJika dirasa sudah melakukan test namun tidak tercatat, mungkin ada yang salah dengan nomor hp. Silakan lakukan update nomor hp di do.my.id yang sesuai dengan nomor di whatsapp."

	// Set perwakilan pertama sebagai nomor yang akan menerima pesan jika private
	if len(filteredDailyScoreData) > 0 {
		perwakilanphone = filteredDailyScoreData[0].PhoneNumber
	} else if len(filteredTotalScoreData) > 0 {
		perwakilanphone = filteredTotalScoreData[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

func GetTotalDataIqMasuk(db *mongo.Database, isTotal bool) (map[string]IqScoreInfo, error) {
	users, err := getPhoneNumberAndNameFromIqScore(db, isTotal)
	if err != nil {
		return nil, err
	}

	// Hitung jumlah aktivitas per nomor telepon
	allData := ByduplicatePhoneNumbersCount(users)

	// Tidak perlu ambil grup WA lagi, karena sudah ada di `users`
	filteredData := make(map[string]IqScoreInfo)
	for phone, info := range allData {
		// Jika user sudah punya grup WA, langsung gunakan
		if len(info.WaGroupID) > 0 {
			filteredData[phone] = info
		}
	}

	return filteredData, nil
}

// Fungsi untuk memformat data IQ Score dalam pesan WhatsApp
func formatIqScoreData(data []IqScoreInfo) string {
	if len(data) == 0 {
		return "Tidak ada skor IQ yang tercatat."
	}

	// Urutkan berdasarkan skor tertinggi
	sort.Slice(data, func(i, j int) bool {
		// Mengubah string skor menjadi integer agar bisa dibandingkan
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

// Fungsi untuk mengambil data IQ Score berdasarkan nomor telepon
func getPhoneNumberAndNameFromIqScore(db *mongo.Database, isTotal bool) ([]IqScoreInfo, error) {
	collection := db.Collection("iqscore")

	// Ambil semua data dari koleksi iqscore
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data IQ Score: %v", err)
	}
	defer cursor.Close(context.TODO())

	var users []IqScoreInfo
	if err = cursor.All(context.TODO(), &users); err != nil {
		return nil, fmt.Errorf("gagal membaca data IQ Score: %v", err)
	}

	// Kumpulkan nomor telepon unik
	var phoneNumbers []string
	phoneSet := make(map[string]bool)
	for _, user := range users {
		if !phoneSet[user.PhoneNumber] {
			phoneSet[user.PhoneNumber] = true
			phoneNumbers = append(phoneNumbers, user.PhoneNumber)
		}
	}

	// Ambil daftar grup WhatsApp berdasarkan nomor telepon
	groupMap, err := GetGroupIDFromProject(db, phoneNumbers)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil grup WhatsApp: %v", err)
	}

	// Tambahkan WaGroupID ke setiap pengguna
	for i, user := range users {
		if waGroups, exists := groupMap[user.PhoneNumber]; exists {
			users[i].WaGroupID = waGroups
		}
	}

	return users, nil
}

// Fungsi untuk menghitung jumlah peserta IQ Score berdasarkan nomor telepon
func ByduplicatePhoneNumbersCount(users []IqScoreInfo) map[string]IqScoreInfo {
	phoneNumberCount := make(map[string]IqScoreInfo)

	for _, user := range users {
		key := user.PhoneNumber

		if info, exists := phoneNumberCount[key]; exists {
			// Tambah skor ke dalam map (misal: sum total skor atau tetap dengan skor terakhir)
			info.Score = user.Score
			info.IQ = user.IQ
			phoneNumberCount[key] = info // Simpan kembali ke map
		} else {
			phoneNumberCount[key] = user // Simpan data baru
		}
	}

	return phoneNumberCount
}

func GetGroupIDFromProject(db *mongo.Database, phoneNumbers []string) (map[string][]string, error) {
	// Filter mencari grup berdasarkan anggota dengan nomor telepon yang cocok
	filter := bson.M{
		"members": bson.M{
			"$elemMatch": bson.M{"phonenumber": bson.M{"$in": phoneNumbers}},
		},
	}

	// Ambil daftar semua dokumen yang sesuai
	cursor, err := db.Collection("project").Find(context.TODO(), filter)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data proyek: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Map untuk menyimpan grup ID unik berdasarkan nomor telepon
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

		// Simpan grup ID berdasarkan nomor telepon yang sesuai
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

	// Konversi map ke slice unik
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
