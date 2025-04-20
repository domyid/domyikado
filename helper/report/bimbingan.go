package report

import (
	"fmt"
	"time"

	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func ReportBimbinganToOrangTua(db *mongo.Database) (map[string]string, error) {
	var allBimbingan []model.ActivityScore
	allBimbingan, err := atdb.GetAllDoc[[]model.ActivityScore](db, "bimbingan", bson.M{})
	if err != nil || len(allBimbingan) == 0 {
		return map[string]string{"": "Belum ada bimbingan sama sekali."}, err
	}

	// Buat map[phonenumber][]time
	mahasiswaWeekMap := make(map[string]map[string]bool)
	for _, b := range allBimbingan {
		year, week := b.CreatedAt.ISOWeek()
		key := fmt.Sprintf("%d-%02d", year, week)
		if _, ok := mahasiswaWeekMap[b.PhoneNumber]; !ok {
			mahasiswaWeekMap[b.PhoneNumber] = make(map[string]bool)
		}
		mahasiswaWeekMap[b.PhoneNumber][key] = true
	}

	// Ambil semua mahasiswa yang punya sponsor
	allMahasiswa, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", bson.M{"sponsorphonenumber": bson.M{"$ne": ""}})
	if err != nil {
		return nil, err
	}

	nowYear, nowWeek := time.Now().ISOWeek()
	thisWeekKey := fmt.Sprintf("%d-%02d", nowYear, nowWeek)

	// Siapkan laporan per sponsor
	laporan := make(map[string]string)
	for _, mhs := range allMahasiswa {
		mhsWeekMap := mahasiswaWeekMap[mhs.PhoneNumber]
		if !mhsWeekMap[thisWeekKey] {
			msg := fmt.Sprintf("⚠️ *%s* belum melakukan bimbingan minggu ini.", mhs.Name)
			laporan[mhs.SponsorPhoneNumber] += msg + "\n"
		}
	}

	return laporan, nil
}
