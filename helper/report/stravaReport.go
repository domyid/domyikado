package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

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

func GenerateRekapPoinStravaMingguan(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	phoneNumberCount, err := getDataStravaMasukPerMinggu(db)
	if err != nil {
		return "", "", fmt.Errorf("gagal mengambil data Strava: %v", err)
	}

	msg = "*Laporan Total Aktivitas Strava:*\n\n"

	// Ubah map menjadi slice agar bisa diurutkan
	var userList []StravaInfo
	for _, info := range phoneNumberCount {
		userList = append(userList, info)
	}

	// Urutkan berdasarkan Count dari terbesar ke terkecil
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Count > userList[j].Count
	})

	var aktifitasAda, aktifitasKosong string

	// Loop data yang sudah diurutkan
	for _, info := range userList {
		if info.Count > 0 {
			aktifitasAda += "✅ " + info.Name + " (" + info.PhoneNumber + "): " + strconv.FormatFloat(info.Count, 'f', -1, 64) + " aktivitas " + " (" + strconv.FormatFloat(info.TotalKm, 'f', 1, 64) + " km)\n"
		} else {
			aktifitasKosong += "⛔ " + info.Name + " (" + info.PhoneNumber + "): 0 aktivitas\n"
		}
	}

	if aktifitasAda != "" {
		msg += "Data yang sudah melakukan aktivitas: \n" + aktifitasAda + "\n"
	}
	if aktifitasKosong != "" {
		msg += "Data yang belum melakukan aktivitas: \n" + aktifitasKosong + "\n"
	}

	msg += "\nJika dirasa sudah melakukan aktivitas namun tidak tercatat, mungkin ada yang salah dengan link strava profile picture. Silahkan lakukan update strava profile picture di do.my.id yang sesuai dengan link profile picture di Strava atau ketik keyword *'strava update in'* pada bot domyikado. Jika sudah silahkan cek kembali strava profile picture di do.my.id apakah sama dengan link yang di berikan oleh bot domyikado. LInk Strava Profile Picture bukan Link https://www.strava.com/athletes/111111111"

	return msg, "", nil
}

func getDataStravaMasukPerMinggu(db *mongo.Database) (map[string]StravaInfo, error) {
	users, err := getPhoneNumberAndNameFromStravaActivity(db)
	if err != nil {
		return nil, err
	}

	return countDuplicatePhoneNumbers(users), nil
}

func getPhoneNumberAndNameFromStravaActivity(db *mongo.Database) ([]StravaInfo, error) {
	// Ambil semua aktivitas Strava
	activities, err := getStravaActivitiesPerWeek(db)
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

func getStravaActivitiesPerWeek(db *mongo.Database) ([]model.StravaActivity, error) {
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil config url: %v", err)
	}

	_, doc, err := atapi.Get[[]model.StravaActivity](conf.StravaUrl)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil aktivitas Strava: %v", err)
	}

	// monday, sunday := getWeekStartEnd(time.Now())

	var filteredActivities []model.StravaActivity
	for _, activity := range doc {
		// activityTime := activity.CreatedAt
		if activity.Status == "Valid" /*&& !activityTime.Before(monday) && activityTime.Before(sunday)*/ {
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

// func getWeekStartEnd(t time.Time) (time.Time, time.Time) {
// 	weekday := int(t.Weekday())
// 	// Jika hari Minggu (0), kita mundur 6 hari ke Senin sebelumnya
// 	if weekday == 0 {
// 		weekday = 7
// 	}

// 	// Dapatkan Senin di awal minggu ini
// 	monday := t.AddDate(0, 0, -weekday+1)
// 	sunday := monday.AddDate(0, 0, 6) // Hitung Minggu

// 	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
// 	sunday = time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 999999999, t.Location())

// 	return monday, sunday
// }
