package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func PostDosenAsesor(respw http.ResponseWriter, req *http.Request) {
	//otorisasi dan validasi inputan
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var lap model.ActivityScore
	err = json.NewDecoder(req.Body).Decode(&lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if lap.Asesor.PhoneNumber == "" {
		var respn model.Response
		respn.Status = "Error : No Telepon Asesor tidak diisi"
		respn.Response = "Isi lebih lengkap terlebih dahulu :" + lap.Asesor.PhoneNumber
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	//validasi eksistensi user di db
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}

	//validasi nomor telepon asesor
	lap.Asesor.PhoneNumber = ValidasiNoHandPhone(lap.Asesor.PhoneNumber)
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": lap.Asesor.PhoneNumber, "isdosen": true})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data asesor tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	if !docasesor.IsDosen {
		var respn model.Response
		respn.Status = "Error : Data asesor bukan dosen"
		respn.Response = "Data asesor bukan dosen"
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}

	//ambil data enroll
	// prjuser, err := atdb.GetOneDoc[model.MasterEnrool](config.Mongoconn, "enroll", primitive.M{"_id": docuser.ID})
	// if err != nil {
	// 	var respn model.Response
	// 	respn.Status = "Error : Data enroll tidak di temukan: " + docuser.ID.Hex()
	// 	respn.Response = err.Error()
	// 	at.WriteJSON(respw, http.StatusNotImplemented, respn)
	// 	return
	// }
	//logic inputan post
	// lap.Enroll = prjuser
	lap.PhoneNumber = docuser.PhoneNumber
	lap.Asesor = docasesor
	lap.CreatedAt = time.Now()

	idlap, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotModified, respn)
		return
	}
	message := "*Permintaan Bimbingan*\n" + "Petugas : " + docuser.Name + "\n Beri Nilai: " + "https://www.do.my.id/kambing/#" + idlap.Hex()
	dt := &whatsauth.TextMessage{
		To:       lap.Asesor.PhoneNumber,
		IsGroup:  false,
		Messages: message,
	}
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	at.WriteJSON(respw, http.StatusOK, lap)

}

func ValidasiNoHandPhone(nomor string) string {
	nomor = strings.ReplaceAll(nomor, " ", "")
	nomor = strings.ReplaceAll(nomor, "+", "")
	nomor = strings.ReplaceAll(nomor, "-", "")
	return nomor
}
