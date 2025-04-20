package report

import (
	"fmt"
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

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": allBimbingan[0].PhoneNumber})
	if err != nil {
		return "", "", err
	}

	weekMap := make(map[string]bool)
	for _, b := range allBimbingan {
		year, week := b.CreatedAt.ISOWeek()
		key := fmt.Sprintf("%d-%02d", year, week)
		weekMap[key] = true
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
