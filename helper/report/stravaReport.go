package report

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type StravaInfo struct {
	Count       float64
	Name        string
	PhoneNumber string
	TotalKm     float64
	WaGroupID   []string
}

func GenerateRekapPoinStrava(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	// Ambil data Strava
	dailyData, err := GetTotalDataStravaMasuk(db, false)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava harian: %v", err)
	}
	totalData, err := GetTotalDataStravaMasuk(db, true)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava total: %v", err)
	}

	// Buat list untuk menyimpan user yang termasuk dalam groupId
	var filteredDailyData []StravaInfo
	var filteredTotalData []StravaInfo

	// Filter dailyData yang termasuk dalam groupId
	for _, info := range dailyData {
		for _, gid := range info.WaGroupID {
			if gid == groupId {
				filteredDailyData = append(filteredDailyData, info)
				break
			}
		}
	}

	// Filter totalData yang termasuk dalam groupId
	for _, info := range totalData {
		for _, gid := range info.WaGroupID {
			if gid == groupId {
				filteredTotalData = append(filteredTotalData, info)
				break
			}
		}
	}

	// Jika grup tidak memiliki anggota, jangan kirim pesan
	if len(filteredDailyData) == 0 && len(filteredTotalData) == 0 {
		return "", "", fmt.Errorf("tidak ada data Strava untuk grup %s", groupId)
	}

	// Format pesan
	msg = "*Laporan Aktivitas Strava:*"
	msg += "\n\n*Kemarin - Hari Ini:*\n"
	msg += formatStravaData(filteredDailyData)

	msg += "\n\n*Total Keseluruhan:*\n"
	msg += formatStravaData(filteredTotalData)

	msg += "\n\nJika dirasa sudah melakukan aktivitas namun tidak tercatat, mungkin ada yang salah dengan link Strava profile picture. Silakan lakukan update Strava profile picture di do.my.id yang sesuai dengan link profile picture di Strava."

	// Set perwakilan pertama sebagai nomor yang akan menerima pesan jika private
	if len(filteredDailyData) > 0 {
		perwakilanphone = filteredDailyData[0].PhoneNumber
	} else if len(filteredTotalData) > 0 {
		perwakilanphone = filteredTotalData[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

func GetTotalDataStravaMasuk(db *mongo.Database, isTotal bool) (map[string]StravaInfo, error) {
	users, err := getPhoneNumberAndNameFromStravaActivity(db, isTotal)
	if err != nil {
		return nil, err
	}

	// Hitung jumlah aktivitas per nomor telepon
	allData := duplicatePhoneNumbersCount(users)

	// Tidak perlu ambil grup WA lagi, karena sudah ada di `users`
	filteredData := make(map[string]StravaInfo)
	for phone, info := range allData {
		// Jika user sudah punya grup WA, langsung gunakan
		if len(info.WaGroupID) > 0 {
			filteredData[phone] = info
		}
	}

	return filteredData, nil
}

func formatStravaData(data []StravaInfo) string {
	if len(data) == 0 {
		return "Tidak ada aktivitas yang tercatat."
	}

	// Urutkan berdasarkan jumlah aktivitas tertinggi
	sort.Slice(data, func(i, j int) bool {
		return data[i].Count > data[j].Count
	})

	var result string
	for _, info := range data {
		// result += fmt.Sprintf("✅ %s (%s): %.0f aktivitas (%.1f km)\n", info.Name, info.PhoneNumber, info.Count, info.TotalKm)
		result += "✅ " + info.Name + " (" + info.PhoneNumber + "): " + strconv.FormatFloat(info.Count, 'f', -1, 64) + " aktivitas (" + strconv.FormatFloat(info.TotalKm, 'f', 1, 64) + " km)\n"

	}

	return result
}

func getPhoneNumberAndNameFromStravaActivity(db *mongo.Database, isTotal bool) ([]StravaInfo, error) {
	// Ambil semua aktivitas Strava
	var activities []model.StravaActivity
	var err error

	if isTotal {
		activities, err = getStravaActivitiesTotal(db)
	} else {
		activities, err = getStravaActivitiesPerDay(db)
	}

	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data aktivitas Strava: %v", err)
	}

	// Pastikan ada aktivitas
	if len(activities) == 0 {
		return nil, fmt.Errorf("tidak ada aktivitas Strava yang ditemukan")
	}

	var users []StravaInfo
	var phoneNumbers []string
	phoneSet := make(map[string]bool)

	// Kumpulkan semua nomor telepon unik
	for _, activity := range activities {
		phone := activity.PhoneNumber
		if !phoneSet[phone] {
			phoneSet[phone] = true
			phoneNumbers = append(phoneNumbers, phone)
		}
	}

	// Ambil daftar grup WA unik berdasarkan nomor telepon
	groupMap, err := getGrupIDFromProject(db, phoneNumbers)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil grup WhatsApp: %v", err)
	}

	// Loop aktivitas untuk membangun data StravaInfo
	for _, activity := range activities {
		phone := activity.PhoneNumber

		// Ambil user dari database berdasarkan nomor telepon
		_, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phone})
		if err != nil {
			if err == mongo.ErrNoDocuments {
				continue
			}
			return nil, fmt.Errorf("gagal mengambil profil dari database: %v", err)
		}

		// Konversi jarak dari string ke float
		distanceStr := strings.Replace(activity.Distance, " km", "", -1)
		distance, _ := strconv.ParseFloat(distanceStr, 64)

		// Ambil grup WA unik dari hasil query sebelumnya
		waGroupIDs := groupMap[phone]

		// Tambahkan ke hasil
		users = append(users, StravaInfo{
			Name:        activity.Name,
			PhoneNumber: phone,
			Count:       1,
			TotalKm:     distance,
			WaGroupID:   waGroupIDs,
		})
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("tidak ada profil Strava yang cocok di database")
	}

	return users, nil
}

func getGrupIDFromProject(db *mongo.Database, phoneNumbers []string) (map[string][]string, error) {
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
			if contains(phoneNumbers, phone) { // Pastikan nomor ada dalam daftar yang dicari
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

func getStravaActivitiesPerDay(db *mongo.Database) ([]model.StravaActivity, error) {
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil config url: %v", err)
	}

	_, doc, err := atapi.Get[[]model.StravaActivity](conf.StravaUrl)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil aktivitas Strava: %v", err)
	}

	startOfDay, endOfDay := getStartAndEndOfYesterday(time.Now())

	var filteredActivities []model.StravaActivity
	for _, activity := range doc {
		activityTime := activity.CreatedAt

		// Cek apakah aktivitas terjadi pada hari ini
		if activity.Status == "Valid" && activityTime.After(startOfDay) && activityTime.Before(endOfDay) {
			filteredActivities = append(filteredActivities, activity)
		}
	}

	if len(filteredActivities) == 0 {
		return nil, errors.New("tidak ada aktivitas yang tercatat kemarin")
	}

	return filteredActivities, nil
}

func getStravaActivitiesTotal(db *mongo.Database) ([]model.StravaActivity, error) {
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil config url: %v", err)
	}

	_, doc, err := atapi.Get[[]model.StravaActivity](conf.StravaUrl)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil aktivitas Strava: %v", err)
	}

	var filteredActivities []model.StravaActivity
	for _, activity := range doc {
		if activity.Status == "Valid" {
			filteredActivities = append(filteredActivities, activity)
		}
	}

	return filteredActivities, nil
}

func duplicatePhoneNumbersCount(users []StravaInfo) map[string]StravaInfo {
	phoneNumberCount := make(map[string]StravaInfo)

	for _, user := range users {
		key := user.PhoneNumber

		if info, exists := phoneNumberCount[key]; exists {
			info.Count++                 // Tambah jumlah aktivitas
			info.TotalKm += user.TotalKm // **Jumlahkan total jarak**
			phoneNumberCount[key] = info // Simpan kembali ke map
		} else {
			phoneNumberCount[key] = user // Simpan data baru
		}
	}

	return phoneNumberCount
}

func getStartAndEndOfYesterday(t time.Time) (time.Time, time.Time) {
	location, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location)
	yesterday := today.AddDate(0, 0, -1)
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
	end := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 999999999, location)

	return start, end
}

// Fungsi bantuan untuk mengecek apakah sebuah nilai ada dalam slice
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
