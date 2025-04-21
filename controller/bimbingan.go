package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func PostDosenAsesor(respw http.ResponseWriter, req *http.Request) {
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
	var bimbingan model.ActivityScore
	err = json.NewDecoder(req.Body).Decode(&bimbingan)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if bimbingan.Asesor.PhoneNumber == "" {
		respn.Status = "Error : No Telepon Asesor tidak diisi"
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

	//validasi nomor telepon asesor
	bimbingan.Asesor.PhoneNumber = ValidasiNoHP(bimbingan.Asesor.PhoneNumber)
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": bimbingan.Asesor.PhoneNumber, "isdosen": true})
	if err != nil {
		respn.Status = "Error : Data asesor tidak di temukan"
		respn.Response = "Nomor Telepon bukan milik Dosen Asesor"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah sudah pernah di-approve
	_, err = atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": docuser.PhoneNumber, "approved": true})
	if err == nil {
		respn.Status = "Error : Data bimbingan sudah di approve"
		respn.Response = "Bimbingan sudah disetujui, tidak dapat mengajukan ulang."
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	score, _ := GetAllActivityScoreData(payload.Id)

	// logic inputan post
	bimbingan.Approved = false
	bimbingan.PhoneNumber = docuser.PhoneNumber
	bimbingan.Asesor = docasesor
	bimbingan.CreatedAt = time.Now()
	bimbingan.Trackerdata = score.Trackerdata
	bimbingan.Tracker = score.Tracker
	bimbingan.StravaKM = score.StravaKM
	bimbingan.Strava = score.Strava
	bimbingan.IQresult = score.IQresult
	bimbingan.IQ = score.IQ
	bimbingan.Pomokitsesi = score.Pomokitsesi
	bimbingan.Pomokit = score.Pomokit
	bimbingan.GTMetrixResult = score.GTMetrixResult
	bimbingan.GTMetrix = score.GTMetrix
	bimbingan.WebHookpush = score.WebHookpush
	bimbingan.WebHook = score.WebHook
	bimbingan.PresensiHari = score.PresensiHari
	bimbingan.Presensi = score.Presensi
	bimbingan.TotalScore = score.TotalScore

	// Cari apakah ada data existing yang belum approved
	existing, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": bimbingan.PhoneNumber, "approved": false})
	var idbimbingan primitive.ObjectID
	if err == nil {
		// Update data yang belum di-approve
		bimbingan.ID = existing.ID
		_, err := atdb.ReplaceOneDoc(config.Mongoconn, "bimbingan", primitive.M{"_id": existing.ID}, bimbingan)
		if err != nil {
			respn.Status = "Gagal Update Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
		idbimbingan = existing.ID
	} else {
		// Insert data baru
		idbimbingan, err = atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
		if err != nil {
			respn.Status = "Gagal Insert Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
	}

	// kirim pesan ke asesor
	message := "*Permintaan Bimbingan*\n" + "Mahasiswa : " + docuser.Name + "\n Beri Nilai: " + "https://www.do.my.id/kambing/#" + idbimbingan.Hex()
	dt := &whatsauth.TextMessage{
		To:       bimbingan.Asesor.PhoneNumber,
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
	at.WriteJSON(respw, http.StatusOK, bimbingan)
}

func GetDataBimbingan(respw http.ResponseWriter, req *http.Request) {
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
	bimbingan, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"_id": objectId})
	if err != nil {
		respn.Status = "Error : Data bimbingan tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, bimbingan)
}

func ReplaceDataBimbingan(respw http.ResponseWriter, req *http.Request) {
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
	var bim model.ActivityScore
	err = json.NewDecoder(req.Body).Decode(&bim)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	filter := primitive.M{"_id": objectId}
	bimbingan, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", filter)
	if err != nil {
		respn.Status = "Error : Data bimbingan tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	bimbingan.Validasi = bim.Validasi
	bimbingan.Komentar = bim.Komentar
	bimbingan.Approved = bim.Approved
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "bimbingan", filter, bimbingan)
	if err != nil {
		respn.Response = "Gagal replaceonedoc"
		respn.Info = err.Error()
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}

	// kirim pesan ke mahasiswa
	var message string
	if bimbingan.Approved {
		message = "Bimbingan Kamu *TELAH DI APPROVE* oleh Dosen " + bimbingan.Asesor.Name + "\n" + "Rate : " + strconv.Itoa(bim.Validasi) + "\n" + "Komentar : " + bim.Komentar + "\n" + "Silahkan lanjutkan bimbingan ke sesi berikutnya."
	} else {
		message = "Bimbingan Kamu *BELUM DI APPROVE* oleh Dosen " + bimbingan.Asesor.Name + "\n" + "Rate : " + strconv.Itoa(bim.Validasi) + "\n" + "Komentar : " + bim.Komentar + "\n" + "Silahkan mengajukan ulang bimbingan setelah perbaikan."
	}

	dt := &whatsauth.TextMessage{
		To:       bimbingan.PhoneNumber,
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
	at.WriteJSON(respw, http.StatusOK, bimbingan)
}

func LaporanKeOrangTua(w http.ResponseWriter, r *http.Request) {
	report.RekapToOrangTua(config.Mongoconn)
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil rekap data",
	})
}

func LaporanRiwayatBimbinganPerMinggu(w http.ResponseWriter, r *http.Request) {
	report.RiwayatBimbinganPerMinggu(config.Mongoconn, "6282117252716")
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil kirim Riwayat bimbingan data",
	})
}
