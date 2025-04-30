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

	// kirim pesan ke asesor
	// message := "*Permintaan Bimbingan*\n" + "Mahasiswa : " + docuser.Name + "\n Beri Nilai: " + "https://www.do.my.id/kambing/#" + idTugasAI.Hex()
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
