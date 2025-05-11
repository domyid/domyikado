package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func PostTugasKelasAI(respw http.ResponseWriter, req *http.Request) {
	//otorisasi dan validasi inputan
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var tugasAI model.ScoreKelasAI
	err = json.NewDecoder(req.Body).Decode(&tugasAI)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if tugasAI.Kelas == "" {
		respn.Status = "Error : Kelas tidak boleh kosong"
		respn.Response = "Isi lebih lengkap terlebih dahulu"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	//validasi eksistensi user di db
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	score, _ := GetLastWeekScoreKelasAIData(payload.Id)

	// logic inputan post
	tugasAI.Username = docuser.Name
	tugasAI.PhoneNumber = docuser.PhoneNumber
	tugasAI.CreatedAt = time.Now()
	tugasAI.StravaKM = score.StravaKM
	tugasAI.Strava = score.Strava
	tugasAI.IQresult = score.IQresult
	tugasAI.IQ = score.IQ
	tugasAI.MBC = score.MBC
	tugasAI.MBCPoints = score.MBCPoints
	tugasAI.RVN = score.RVN
	tugasAI.RavencoinPoints = score.RavencoinPoints
	tugasAI.QRIS = score.QRIS
	tugasAI.QRISPoints = score.QRISPoints
	tugasAI.Pomokitsesi = score.Pomokitsesi
	tugasAI.Pomokit = score.Pomokit
	tugasAI.AllTugas = score.AllTugas

	startTime, endTime, err := GetWeeklyFridayRange(time.Now())
	if err != nil {
		respn.Status = "Error : Gagal mendapatkan range waktu"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	filter := primitive.M{
		"phonenumber": payload.Id,
		"createdAt": primitive.M{
			"$gte": startTime.UTC(),
			"$lt":  endTime.UTC(),
		},
	}

	// Cari apakah ada data existing yang belum approved
	existing, err := atdb.GetOneDoc[model.ScoreKelasAI](config.Mongoconn, "tugaskelasai", filter)
	if err == nil {
		// Update data yang di minggu ini
		tugasAI.ID = existing.ID
		tugasAI.TugasKe = existing.TugasKe
		tugasAI.CreatedAt = existing.CreatedAt
		_, err := atdb.ReplaceOneDoc(config.Mongoconn, "tugaskelasai", primitive.M{"_id": existing.ID}, tugasAI)
		if err != nil {
			respn.Status = "Error : Gagal Update Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
	} else {
		allDoc, err := atdb.GetAllDoc[[]model.ScoreKelasAI](config.Mongoconn, "tugaskelasai", primitive.M{"phonenumber": payload.Id})
		if err != nil {
			respn.Status = "Error : Data tugasAI tidak di temukan"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusBadRequest, respn)
			return
		}
		// Insert data baru
		tugasAI.TugasKe = len(allDoc) + 1
		_, err = atdb.InsertOneDoc(config.Mongoconn, "tugaskelasai", tugasAI)
		if err != nil {
			respn.Status = "Error : Gagal Insert Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
	}

	at.WriteJSON(respw, http.StatusOK, tugasAI)
}

func GetDataTugasAIById(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	id := at.GetParam(req)
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	tugasai, err := atdb.GetOneDoc[model.ScoreKelasAI](config.Mongoconn, "tugaskelasai", primitive.M{"_id": objectId})
	if err != nil {
		respn.Status = "Error : Data tugas ai tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, tugasai)
}

func GetDataTugasAI(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	type TugasAI struct {
		ID      primitive.ObjectID `json:"_id" bson:"_id"`
		TugasKe int                `json:"tugaske" bson:"tugaske"`
		Phonenumber string `json:"phonenumber" bson:"phonenumber"`
	}

	tugasailist, err := atdb.GetAllDoc[[]TugasAI](config.Mongoconn, "tugaskelasai", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data tugas ai"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, tugasailist)
}

func GetPomokitDataKelasAI(db *mongo.Database, phonenumber string) ([]model.TugasPomodoro, error) {
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		return nil, err
	}

	pomokitApi := conf.PomokitUrl + "/" + phonenumber
	scode, pomodoros, err := atapi.Get[[]model.TugasPomodoro](pomokitApi)
	if err != nil || scode != http.StatusOK {
		return nil, err
	}

	if len(pomodoros) == 0 {
		return nil, fmt.Errorf("no pomodoros found for user %s", phonenumber)
	}

	// Filter pomodoros based on the current week
	startTime, endTime, err := GetWeeklyFridayRange(time.Now())
	if err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")

	seenUrls := make(map[string]bool)
	var filteredPomodoros []model.TugasPomodoro
	for _, pomodoro := range pomodoros {
		createdAtLocal := pomodoro.CreatedAt.In(loc)
		if createdAtLocal.After(startTime) && createdAtLocal.Before(endTime) {
			urlKey := pomodoro.URLPekerjaan
			if strings.Contains(pomodoro.URLPekerjaan, "gtmetrix.com") {
				urlKey = pomodoro.GTMetrixURLTarget
			}
			if _, exists := seenUrls[urlKey]; !exists {
				filteredPomodoros = append(filteredPomodoros, pomodoro)
				seenUrls[urlKey] = true
			}
		}
	}

	if len(filteredPomodoros) == 0 {
		return nil, fmt.Errorf("no pomodoros found for user %s in the current week", phonenumber)
	}

	return filteredPomodoros, nil
}

func GetWeeklyFridayRange(times time.Time) (startTime time.Time, endTime time.Time, err error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := times.In(loc)

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Minggu jadi 7
	}

	// Mundur ke Jumat terakhir
	// daysSinceFriday := (weekday + 2) % 7
	// lastFriday := now.AddDate(0, 0, -daysSinceFriday)
	daysSinceSaturday := (weekday + 1) % 7
	lastSaturday := now.AddDate(0, 0, -daysSinceSaturday)

	// Mulai dari Sabtu pukul 00:01 WIB
	startTime = time.Date(lastSaturday.Year(), lastSaturday.Month(), lastSaturday.Day(), 0, 1, 0, 0, loc)

	// Selesai Sabtu pukul 00:00 WIB
	nextFriday := lastSaturday.AddDate(0, 0, 7)
	endTime = time.Date(nextFriday.Year(), nextFriday.Month(), nextFriday.Day(), 0, 0, 0, 0, loc)

	return startTime, endTime, nil
}
