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

func PostTugasKelasWS(respw http.ResponseWriter, req *http.Request) {
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
	var tugasWS model.ScoreKelasWS
	err = json.NewDecoder(req.Body).Decode(&tugasWS)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if tugasWS.Kelas == "" {
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

	score, _ := GetLastWeekScoreKelasWSData(payload.Id)

	// logic inputan post
	tugasWS.Username = docuser.Name
	tugasWS.PhoneNumber = docuser.PhoneNumber
	tugasWS.CreatedAt = time.Now()
	tugasWS.StravaKM = score.StravaKM
	tugasWS.Strava = score.Strava
	tugasWS.IQresult = score.IQresult
	tugasWS.IQ = score.IQ
	tugasWS.MBC = score.MBC
	tugasWS.MBCPoints = score.MBCPoints
	tugasWS.RVN = score.RVN
	tugasWS.RavencoinPoints = score.RavencoinPoints
	tugasWS.QRIS = score.QRIS
	tugasWS.QRISPoints = score.QRISPoints
	tugasWS.Pomokitsesi = score.Pomokitsesi
	tugasWS.Pomokit = score.Pomokit
	tugasWS.AllTugas = score.AllTugas

	startTime, endTime, err := GetEveryWeeklyFridayRange(time.Now())
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
	existing, err := atdb.GetOneDoc[model.ScoreKelasWS](config.Mongoconn, "tugaskelasws", filter)
	if err == nil {
		// Update data yang di minggu ini
		tugasWS.ID = existing.ID
		tugasWS.TugasKe = existing.TugasKe
		tugasWS.CreatedAt = existing.CreatedAt
		_, err := atdb.ReplaceOneDoc(config.Mongoconn, "tugaskelasws", primitive.M{"_id": existing.ID}, tugasWS)
		if err != nil {
			respn.Status = "Error : Gagal Update Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
	} else {
		allDoc, err := atdb.GetAllDoc[[]model.ScoreKelasWS](config.Mongoconn, "tugaskelasws", primitive.M{"phonenumber": payload.Id})
		if err != nil {
			respn.Status = "Error : Data tugasWS tidak di temukan"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusBadRequest, respn)
			return
		}
		// Insert data baru
		tugasWS.TugasKe = len(allDoc) + 1
		_, err = atdb.InsertOneDoc(config.Mongoconn, "tugaskelasws", tugasWS)
		if err != nil {
			respn.Status = "Error : Gagal Insert Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
	}

	at.WriteJSON(respw, http.StatusOK, tugasWS)
}

func GetDataTugasWSById(respw http.ResponseWriter, req *http.Request) {
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
	tugasws, err := atdb.GetOneDoc[model.ScoreKelasWS](config.Mongoconn, "tugaskelasws", primitive.M{"_id": objectId})
	if err != nil {
		respn.Status = "Error : Data tugas ws tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, tugasws)
}

func GetDataTugasWS(respw http.ResponseWriter, req *http.Request) {
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

	type TugasWS struct {
		ID      primitive.ObjectID `json:"_id" bson:"_id"`
		TugasKe int                `json:"tugaske" bson:"tugaske"`
	}

	tugaswslist, err := atdb.GetAllDoc[[]TugasWS](config.Mongoconn, "tugaskelasws", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data tugas ws"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, tugaswslist)
}

func GetPomokitDataKelasWS(db *mongo.Database, phonenumber string) ([]model.TugasPomodoro, error) {
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
	startTime, endTime, err := GetEveryWeeklyFridayRange(time.Now())
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

func GetEveryWeeklyFridayRange(times time.Time) (startTime time.Time, endTime time.Time, err error) {
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
