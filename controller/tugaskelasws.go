package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	if tugasWS.AllTugas == nil {
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

	// Cari apakah ada data existing yang belum approved
	// var idtugasWS primitive.ObjectID

	allDoc, err := atdb.GetAllDoc[[]model.ScoreKelasWS](config.Mongoconn, "tugaskelasws", primitive.M{"phonenumber": tugasWS.PhoneNumber})
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

	// // kirim pesan ke user
	// message := "*Tugas " + string(rune(tugasWS.TugasKe)) + "*\n" + " berhasil di kirim"
	// dt := &whatsauth.TextMessage{
	// 	To:       tugasWS.PhoneNumber,
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
	at.WriteJSON(respw, http.StatusOK, tugasWS)
}
