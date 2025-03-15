package report

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gocroot/config"
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
}

func GenerateRekapPoinStravaMingguan(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	phoneNumberCount, err := getDataStravaMasukPerMinggu(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava: %v", err)
	}

	msg = "*Laporan Aktivitas Strava Minggu ini :*\n"
	for _, info := range phoneNumberCount {
		msg += "✅ " + info.Name + " (" + info.PhoneNumber + "): " + strconv.FormatFloat(info.Count, 'f', -1, 64) + " aktivitas\n"
	}

	return msg, "", nil
}

func getDataStravaMasukPerMinggu(db *mongo.Database) (map[string]StravaInfo, error) {
	users, err := getPhoneNumberAndNameFromStravaActivity(db)
	if err != nil {
		return nil, err
	}

	return countDuplicatePhoneNumbers(users), nil
}

func getWeekStartEnd(t time.Time) (time.Time, time.Time) {
	weekday := int(t.Weekday())
	// Jika hari Minggu (0), kita mundur 6 hari ke Senin sebelumnya
	if weekday == 0 {
		weekday = 7
	}

	// Dapatkan Senin di awal minggu ini
	monday := t.AddDate(0, 0, -weekday+1)
	sunday := monday.AddDate(0, 0, 6) // Hitung Minggu

	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	sunday = time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 999999999, t.Location())

	return monday, sunday
}

func getStravaActivities() ([]model.StravaActivity, error) {
	_, doc, err := atapi.Get[[]model.StravaActivity](config.StravaActivityAPI)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil aktivitas Strava: %v", err)
	}

	monday, sunday := getWeekStartEnd(time.Now())

	var filteredActivities []model.StravaActivity
	for _, activity := range doc {
		activityTime := activity.CreatedAt
		if activity.Status == "Valid" && !activityTime.Before(monday) && activityTime.Before(sunday) {
			filteredActivities = append(filteredActivities, activity)
		}
	}

	return filteredActivities, nil
}

func getPhoneNumberAndNameFromStravaActivity(db *mongo.Database) ([]StravaInfo, error) {
	// Ambil semua aktivitas Strava
	activities, err := getStravaActivities()
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
		profile := activity.Picture // Gunakan Picture sebagai referensi ke database user

		// Cari user di database berdasarkan Strava profile picture
		doc, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"stravaprofilepicture": profile})
		if err != nil {
			if err == mongo.ErrNoDocuments {
				continue // Lanjut ke aktivitas berikutnya jika tidak ditemukan
			}
			return nil, fmt.Errorf("gagal mengambil profil dari database: %v", err)
		}

		// Simpan hasil yang cocok
		users = append(users, StravaInfo{
			Name:        doc.Name,
			PhoneNumber: doc.PhoneNumber,
			Count:       1, // Awalnya 1, nanti ditambahkan jika ada duplikasi
		})
	}

	// Jika tidak ada user yang cocok, return error
	if len(users) == 0 {
		return nil, fmt.Errorf("tidak ada profil Strava yang cocok di database")
	}

	return users, nil
}

func countDuplicatePhoneNumbers(users []StravaInfo) map[string]StravaInfo {
	phoneNumberCount := make(map[string]StravaInfo)

	for _, user := range users {
		key := user.PhoneNumber // Gunakan PhoneNumber sebagai key

		if info, exists := phoneNumberCount[key]; exists {
			info.Count++
			phoneNumberCount[key] = info
		} else {
			phoneNumberCount[key] = StravaInfo{
				Name:        user.Name,
				PhoneNumber: user.PhoneNumber,
				Count:       1,
			}
		}
	}

	return phoneNumberCount
}
