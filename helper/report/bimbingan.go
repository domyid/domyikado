package report

import (
	"fmt"
	"sort"
	"time"

	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func ReportBimbinganToOrangTua(db *mongo.Database) (msg string, perwakilanphone string, err error) {
	var allBimbingan []model.ActivityScore
	allBimbingan, err = atdb.GetAllDoc[[]model.ActivityScore](db, "bimbingan", bson.M{})
	if err != nil || len(allBimbingan) == 0 {
		return "Belum ada bimbingan sama sekali.", "", err
	}

	weekMap := make(map[string]bool)
	for _, b := range allBimbingan {
		year, week := b.CreatedAt.ISOWeek()
		key := fmt.Sprintf("%d-%02d", year, week)
		weekMap[key] = true
	}

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": allBimbingan[0].PhoneNumber})
	if err != nil {
		return "", "", err
	}

	nowYear, nowWeek := time.Now().ISOWeek()
	thisWeekKey := fmt.Sprintf("%d-%02d", nowYear, nowWeek)
	if !weekMap[thisWeekKey] {
		msg += "\n⚠️ *Minggu ini belum ada bimbingan!* "
	} else {
		msg += "\n✅ Minggu ini sudah melakukan bimbingan."
	}

	return msg, docuser.SponsorPhoneNumber, nil
}

func GetRiwayatBimbinganPerMinggu(db *mongo.Database, phonenumber string) (string, error) {
	// Ambil semua bimbingan berdasarkan nomor telepon
	filter := bson.M{"phonenumber": phonenumber}
	bimbinganList, err := atdb.GetAllDoc[[]model.ActivityScore](db, "bimbingan", filter)
	if err != nil {
		return "", err
	}

	if len(bimbinganList) == 0 {
		return "Belum ada riwayat bimbingan.", nil
	}

	// Map untuk mengelompokkan data berdasarkan minggu
	riwayat := make(map[string][]model.ActivityScore)
	var mingguKeys []string

	for _, b := range bimbinganList {
		year, week := b.CreatedAt.ISOWeek()
		key := fmt.Sprintf("%d-%02d", year, week)
		if _, found := riwayat[key]; !found {
			mingguKeys = append(mingguKeys, key)
		}
		riwayat[key] = append(riwayat[key], b)
	}

	sort.Strings(mingguKeys)

	output := "📚 *Riwayat Bimbingan per Minggu:*\n"
	for i, key := range mingguKeys {
		tanggal := riwayat[key][0].CreatedAt.Format("02 Jan 2006")
		jumlah := len(riwayat[key])
		status := "✅ Sudah bimbingan"
		if jumlah == 0 {
			status = "⚠️ Belum ada bimbingan"
		}
		output += fmt.Sprintf("🗓️ Week %d (%s): %s (%d kali)\n", i+1, tanggal, status, jumlah)
	}

	return output, nil
}

// func ReportBimbinganToOrangTua(db *mongo.Database) (map[string]string, error) {
// 	var allBimbingan []model.ActivityScore
// 	allBimbingan, err := atdb.GetAllDoc[[]model.ActivityScore](db, "bimbingan", bson.M{})
// 	if err != nil || len(allBimbingan) == 0 {
// 		return map[string]string{"": "Belum ada bimbingan sama sekali."}, err
// 	}

// 	// Buat map[phonenumber][]time
// 	mahasiswaWeekMap := make(map[string]map[string]bool)
// 	for _, b := range allBimbingan {
// 		year, week := b.CreatedAt.ISOWeek()
// 		key := fmt.Sprintf("%d-%02d", year, week)
// 		if _, ok := mahasiswaWeekMap[b.PhoneNumber]; !ok {
// 			mahasiswaWeekMap[b.PhoneNumber] = make(map[string]bool)
// 		}
// 		mahasiswaWeekMap[b.PhoneNumber][key] = true
// 	}

// 	// Ambil semua mahasiswa yang punya sponsor
// 	allMahasiswa, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", bson.M{"sponsorphonenumber": bson.M{"$ne": ""}})
// 	if err != nil {
// 		return nil, err
// 	}

// 	nowYear, nowWeek := time.Now().ISOWeek()
// 	thisWeekKey := fmt.Sprintf("%d-%02d", nowYear, nowWeek)

// 	// Siapkan laporan per sponsor
// 	laporan := make(map[string]string)
// 	for _, mhs := range allMahasiswa {
// 		mhsWeekMap := mahasiswaWeekMap[mhs.PhoneNumber]
// 		if !mhsWeekMap[thisWeekKey] {
// 			msg := fmt.Sprintf("⚠️ *%s* belum melakukan bimbingan minggu ini.", mhs.Name)
// 			laporan[mhs.SponsorPhoneNumber] += msg + "\n"
// 		}
// 	}

// 	return laporan, nil
// }
