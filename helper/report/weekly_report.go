package report

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Disini filter untuk rentang waktu 7 hari
func WeeklyFilter() bson.M {
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)

	return bson.M{
		"$gte": primitive.NewObjectIDFromTimestamp(weekAgo),
		"$lt":  primitive.NewObjectIDFromTimestamp(time.Now()),
	}
}

func TrackerWeeklyFilter() bson.M {
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	now := time.Now()

	return bson.M{
		"tanggal_ambil": bson.M{
			"$gte": weekAgo,
			"$lt":  now,
		},
	}
}

// Get laporan mingguan dari satu grup wa
func GetDataRepoMasukMingguIniPerWaGroupID(db *mongo.Database, groupId string) (phoneNumberCount map[string]PhoneNumberInfo, err error) {
	filter := bson.M{"_id": WeeklyFilter(), "project.wagroupid": groupId}
	pushrepodata, err := atdb.GetAllDoc[[]model.PushReport](db, "pushrepo", filter)
	if err != nil {
		return
	}
	phoneNumberCount = CountDuplicatePhoneNumbersWithName(pushrepodata)
	return
}

func GetDataLaporanMingguIniPerWAGroupID(db *mongo.Database, waGroupId string) (phoneNumberCount map[string]PhoneNumberInfo, err error) {
	filter := bson.M{"_id": WeeklyFilter(), "project.wagroupid": waGroupId}
	laps, err := atdb.GetAllDoc[[]Laporan](db, "uxlaporan", filter)
	if err != nil {
		return
	}
	phoneNumberCount = CountDuplicatePhoneNumbersLaporan(laps)
	return
}
func GenerateRekapMessageMingguIniPerWAGroupID(db *mongo.Database, groupId string) (msg string, perwakilanphone string, err error) {
	pushReportCounts, err := GetDataRepoMasukMingguIniPerWaGroupID(db, groupId)
	if err != nil {
		return
	}
	laporanCounts, err := GetDataLaporanMingguIniPerWAGroupID(db, groupId)
	if err != nil {
		return
	}
	mergedCounts := MergePhoneNumberCounts(pushReportCounts, laporanCounts)
	if len(mergedCounts) == 0 {
		err = errors.New("tidak ada aktifitas push dan laporan")
		return
	}
	msg = "*Laporan Penambahan Poin Total Minggu Ini :*\n"
	var phoneSlice []string
	for phoneNumber, info := range mergedCounts {
		msg += "✅ " + info.Name + " (" + phoneNumber + ") : +" + strconv.FormatFloat(info.Count, 'f', -1, 64) + "\n"
		if info.Count > 2 { //klo lebih dari 2 maka tidak akan dikurangi masuk ke daftra putih
			phoneSlice = append(phoneSlice, phoneNumber)
		}
	}

	return
}

func GetScoreTrackerAllLastWeek(db *mongo.Database) (map[string]int, map[string]float64, error) {
	filter := bson.M{
		"_id": WeeklyFilter(),
		"$and": []bson.M{
			{
				"hostname": bson.M{"$nin": []string{"", "127.0.0.1", "3.27.215.75"}}, // Hostname domain tidak valid
			},
			{
				"hostname": bson.M{"$not": bson.M{"$regex": `^[a-z0-9]+--`}}, // Hostname tanpa prefix acak
			},
			{
				"hostname": bson.M{"$in": GetValidHostnames()}, // Hanya hostname dari domainProyek1 yang ditampilkan
			},
		},
	}

	laps, err := atdb.GetAllDoc[[]model.UserInfo](db, "trackeriptest", filter)
	if err != nil {
		return nil, nil, err
	}

	jumlah := make(map[string]int)
	for _, lap := range laps {
		jumlah[lap.Hostname]++
	}

	point := make(map[string]float64)
	for hostname, count := range jumlah {
		calculatedPoint := (float64(count) / 7) * 10
		point[hostname] = math.Min(calculatedPoint, 100) // Batasi maksimal 100
	}

	return jumlah, point, err
}
