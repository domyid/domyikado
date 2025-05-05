package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	if tugasAI.AllTugas == nil {
		respn.Status = "Error : Tugas tidak boleh kosong"
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

	// Cari apakah ada data existing yang belum approved
	// var idTugasAI primitive.ObjectID

	allDoc, err := atdb.GetAllDoc[[]model.ScoreKelasAI](config.Mongoconn, "tugaskelasai", primitive.M{"phonenumber": tugasAI.PhoneNumber})
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

	// // kirim pesan ke user
	// message := "*Tugas " + string(rune(tugasAI.TugasKe)) + "*\n" + " berhasil di kirim"
	// dt := &whatsauth.TextMessage{
	// 	To:       tugasAI.PhoneNumber,
	// 	IsGroup:  false,
	// 	Messages: message,
	// }
	// _, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	// if err != nil {
	// 	resp.Info = "Tidak berhak"
	// 	resp.Response = err.Error()
	// 	at.WriteJSON(respw, http.StatusUnauthorized, resp)
	// 	return
	// }
	at.WriteJSON(respw, http.StatusOK, tugasAI)
}

func PostTugasKelasAI1(respw http.ResponseWriter, req *http.Request) {
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

	// pomokitKelasAI, err := GetPomokitDataKelasAI(config.Mongoconn, payload.Id)
	// if err != nil {
	// 	respn.Status = "Error : Gagal mendapatkan data pomokit"
	// 	respn.Response = err.Error()
	// 	at.WriteJSON(respw, http.StatusBadRequest, respn)
	// 	return
	// }
	// if len(pomokitKelasAI) == 0 {
	// 	respn.Status = "Error : Gagal mendapatkan data pomokit"
	// 	respn.Response = "No pomodoros found for user " + payload.Id
	// 	at.WriteJSON(respw, http.StatusBadRequest, respn)
	// 	return
	// }

	// urls := make([]string, 0, len(pomokitKelasAI))
	// for _, pomokit := range pomokitKelasAI {
	// 	urls = append(urls, pomokit.URLPekerjaan)
	// }
	// tugasAI.AllTugas = urls

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

	allDoc, err := atdb.GetAllDoc[[]model.ScoreKelasAI](config.Mongoconn, "tugaskelasai1", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data tugasAI tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	// Insert data baru
	tugasAI.TugasKe = len(allDoc) + 1
	_, err = atdb.InsertOneDoc(config.Mongoconn, "tugaskelasai1", tugasAI)
	if err != nil {
		respn.Status = "Error : Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotModified, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, tugasAI)
}

// func GetPomokitKelasAI(respw http.ResponseWriter, req *http.Request) {
// 	//otorisasi dan validasi inputan
// 	var respn model.Response
// 	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
// 	if err != nil {
// 		respn.Status = "Error : Token Tidak Valid"
// 		respn.Info = at.GetSecretFromHeader(req)
// 		respn.Location = "Decode Token Error"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusForbidden, respn)
// 		return
// 	}

// 	pomokitKelasAI, err := getPomokitKelasAI(config.Mongoconn, payload.Id)
// 	if err != nil {
// 		respn.Status = "Error : Gagal mendapatkan data pomokit"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusBadRequest, respn)
// 		return
// 	}

// 	at.WriteJSON(respw, http.StatusOK, pomokitKelasAI)
// }

func GetPomokitDataKelasAI(db *mongo.Database, phonenumber string) ([]model.PomodoroReport, error) {
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		return nil, err
	}

	pomokitApi := conf.PomokitUrl + "/" + phonenumber
	scode, pomodoros, err := atapi.Get[[]model.PomodoroReport](pomokitApi)
	if err != nil || scode != http.StatusOK {
		return nil, err
	}
	if len(pomodoros) == 0 {
		return nil, fmt.Errorf("no pomodoros found for user %s", phonenumber)
	}

	return pomodoros, nil
}
