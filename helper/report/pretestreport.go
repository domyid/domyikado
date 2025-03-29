package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type PretestScoreInfo struct {
	Name        string `bson:"name"`
	PhoneNumber string `bson:"phonenumber"`
	Score       string `bson:"score"`
	Pretest     string `bson:"pretest"`
	WaGroupID   string `bson:"wagroupid"`
	CreatedAt   string `bson:"created_at"`
}

func GetTotalDataPretestMasuk(db *mongo.Database) ([]PretestScoreInfo, error) {
	collection := db.Collection("pretestanswer")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data Pretest dari database: %v", err)
	}
	defer cursor.Close(context.TODO())

	var results []PretestScoreInfo
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, fmt.Errorf("gagal membaca data Pretest: %v", err)
	}

	for i, item := range results {
		results[i].WaGroupID = strings.TrimSpace(item.WaGroupID)
	}

	return results, nil
}

func GenerateRekapPretestByDay(db *mongo.Database, groupID string) (string, string, error) {
	data, err := GetTotalDataPretestMasuk(db)
	if err != nil {
		return "", "", err
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Format("2006-01-02")

	var todayList []PretestScoreInfo

	for _, item := range data {
		if item.WaGroupID != groupID {
			continue
		}
		created, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(item.CreatedAt), loc)
		if err != nil {
			continue
		}
		if created.Format("2006-01-02") == today {
			todayList = append(todayList, item)
		}
	}

	if len(todayList) == 0 {
		msg := "*📝 Rekap Harian Pretest - " + today + "*\n\nBelum ada peserta yang mengerjakan pretest hari ini."
		return msg, "", nil
	}

	msg := "*📝 Rekap Harian Pretest - " + today + "*\n\n"
	msg += fmt.Sprintf("Total peserta hari ini: %d orang\n\n", len(todayList))
	for _, p := range todayList {
		msg += fmt.Sprintf("✅ %s - Skor: %s, Nilai: %s\n", p.Name, p.Score, p.Pretest)
	}

	return msg, todayList[0].PhoneNumber, nil
}

func GenerateRekapPretestByWeek(db *mongo.Database, groupID string) (string, string, error) {
	data, err := GetTotalDataPretestMasuk(db)
	if err != nil {
		return "", "", err
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	offset := (int(now.Weekday()) + 6) % 7
	seninIni := now.AddDate(0, 0, -offset).Truncate(24 * time.Hour)
	seninLalu := seninIni.AddDate(0, 0, -7)
	mingguLaluAkhir := seninIni.AddDate(0, 0, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	var thisWeek, lastWeek, total []PretestScoreInfo

	for _, item := range data {
		if item.WaGroupID != groupID {
			continue
		}
		created, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(item.CreatedAt), loc)
		if err != nil {
			continue
		}
		total = append(total, item)
		if created.After(seninIni) {
			thisWeek = append(thisWeek, item)
		} else if created.After(seninLalu) && created.Before(mingguLaluAkhir) {
			lastWeek = append(lastWeek, item)
		}
	}

	if len(total) == 0 {
		return "", "", fmt.Errorf("tidak ada data Pretest untuk grup %s", groupID)
	}

	msg := "*📝 Laporan Pretest Berdasarkan Minggu*\n\n"
	msg += fmt.Sprintf("📊 *Total Seluruh*: %d peserta\n", len(total))
	for _, p := range total {
		msg += fmt.Sprintf("✅ %s - Skor: %s, Nilai: %s\n", p.Name, p.Score, p.Pretest)
	}

	msg += fmt.Sprintf("\n📅 *Minggu Ini*: %d peserta\n", len(thisWeek))
	for _, p := range thisWeek {
		msg += fmt.Sprintf("✅ %s - Skor: %s, Nilai: %s\n", p.Name, p.Score, p.Pretest)
	}

	msg += fmt.Sprintf("\n📆 *Minggu Lalu*: %d peserta\n", len(lastWeek))
	for _, p := range lastWeek {
		msg += fmt.Sprintf("✅ %s - Skor: %s, Nilai: %s\n", p.Name, p.Score, p.Pretest)
	}

	return msg, total[0].PhoneNumber, nil
}
