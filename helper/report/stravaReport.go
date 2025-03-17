package report

import (
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
}

func GetAllPhoneNumbersFromStrava(users map[string]StravaInfo) []string {
	var phoneNumbers []string
	phoneSet := make(map[string]bool) // Untuk menghindari duplikasi

	for _, info := range users {
		if _, exists := phoneSet[info.PhoneNumber]; !exists {
			phoneSet[info.PhoneNumber] = true
			phoneNumbers = append(phoneNumbers, info.PhoneNumber)
		}
	}

	return phoneNumbers
}

func GetGrupIDFromProject(db *mongo.Database, phoneNumbers []string) ([]string, error) {
	// Filter mencari grup berdasarkan anggota dengan nomor telepon yang cocok
	filter := bson.M{
		"members": bson.M{
			"$elemMatch": bson.M{"phonenumber": bson.M{"$in": phoneNumbers}},
		},
	}

	// Ambil daftar wagroupid dari koleksi "project"
	wagroupIDs, err := atdb.GetAllDistinctDoc(db, filter, "wagroupid", "project")
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil wagroupid: %v", err)
	}

	// Konversi hasil ke slice string
	var result []string
	for _, id := range wagroupIDs {
		if str, ok := id.(string); ok {
			result = append(result, str)
		}
	}

	return result, nil
}

func GenerateRekapPoinStrava(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	dailyData, err := getTotalDataStravaMasuk(db, false)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava harian: %v", err)
	}
	totalData, err := getTotalDataStravaMasuk(db, true)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava total: %v", err)
	}

	msg = "*Laporan Aktivitas Strava:*"
	msg += "\n\n*Kemarin - Hari Ini:*\n"
	msg += formatStravaData(dailyData)

	msg += "\n\n*Total Keseluruhan:*\n"
	msg += formatStravaData(totalData)

	msg += "\n\nJika dirasa sudah melakukan aktivitas namun tidak tercatat, mungkin ada yang salah dengan link Strava profile picture. Silakan lakukan update Strava profile picture di do.my.id yang sesuai dengan link profile picture di Strava."

	return msg, "", nil
}

func getTotalDataStravaMasuk(db *mongo.Database, isTotal bool) (map[string]StravaInfo, error) {
	users, err := getPhoneNumberAndNameFromStravaActivity(db, isTotal)
	if err != nil {
		return nil, err
	}

	return countDuplicatePhoneNumbers(users), nil
}

func formatStravaData(data map[string]StravaInfo) string {
	var userList []StravaInfo
	for _, info := range data {
		userList = append(userList, info)
	}

	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Count > userList[j].Count
	})

	var result string
	for _, info := range userList {
		result += fmt.Sprintf("✅ %s (%s): %.0f aktivitas (%.1f km)\n", info.Name, info.PhoneNumber, info.Count, info.TotalKm)
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

	// Loop semua aktivitas Strava
	for _, activity := range activities {
		phone := activity.PhoneNumber // Gunakan Picture sebagai referensi ke database user

		// Cari user di database berdasarkan Strava profile picture
		doc, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phone})
		if err != nil {
			if err == mongo.ErrNoDocuments {
				continue // Lanjut ke aktivitas berikutnya jika tidak ditemukan
			}
			return nil, fmt.Errorf("gagal mengambil profil dari database: %v", err)
		}

		// Simpan hasil yang cocok
		distanceStr := strings.Replace(activity.Distance, " km", "", -1) // Hapus " km"
		distance, _ := strconv.ParseFloat(distanceStr, 64)

		users = append(users, StravaInfo{
			Name:        activity.Name,
			PhoneNumber: doc.PhoneNumber,
			Count:       1,
			TotalKm:     distance,
		})
	}

	// Jika tidak ada user yang cocok, return error
	if len(users) == 0 {
		return nil, fmt.Errorf("tidak ada profil Strava yang cocok di database")
	}

	return users, nil
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

func countDuplicatePhoneNumbers(users []StravaInfo) map[string]StravaInfo {
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
