package controller

import (
	"encoding/json"
	"net/http"
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

// PostPengajuanSidang handles the submission of thesis defense requests
func PostPengajuanSidang(respw http.ResponseWriter, req *http.Request) {
	// Authorize and validate input
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

	// Decode request body
	var pengajuan model.BimbinganPengajuan
	err = json.NewDecoder(req.Body).Decode(&pengajuan)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validate required fields
	if pengajuan.DosenPenguji == "" || pengajuan.NomorKelompok == "" {
		respn.Status = "Error : Data pengajuan tidak lengkap"
		respn.Response = "Isi Dosen Penguji dan Nomor Kelompok terlebih dahulu"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validate user existence
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validate if the user has at least 1 bimbingan sessions
	bimbinganList, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if len(bimbinganList) < 1 {
		respn.Status = "Error : Syarat bimbingan belum terpenuhi"
		respn.Response = "Anda memerlukan minimal 1 sesi bimbingan untuk mengajukan sidang"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get the last bimbingan record to get the current dosen asesor
	var lastBimbingan model.ActivityScore
	if len(bimbinganList) > 0 {
		// Sort the bimbingan by creation date (most recent first)
		// Note: In a real implementation, you might want to sort the array by createdAt field
		lastBimbingan = bimbinganList[len(bimbinganList)-1]
	} else {
		respn.Status = "Error : Tidak ada data bimbingan"
		respn.Response = "Tidak dapat menemukan data dosen pembimbing"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Prepare the submission data
	pengajuan.PhoneNumber = docuser.PhoneNumber
	pengajuan.Name = docuser.Name
	pengajuan.NPM = docuser.NPM
	pengajuan.Timestamp = time.Now()
	pengajuan.Status = "pending" // Default status

	// Set the DosenPembimbing field from the last bimbingan's asesor
	if lastBimbingan.Asesor.Name != "" {
		pengajuan.DosenPembimbing = lastBimbingan.Asesor.Name
		pengajuan.DosenPembimbingPhone = lastBimbingan.Asesor.PhoneNumber
	} else {
		respn.Status = "Error : Data dosen pembimbing tidak ditemukan"
		respn.Response = "Tidak dapat menemukan data dosen pembimbing dari riwayat bimbingan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Insert into database
	idPengajuan, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan_pengajuan", pengajuan)
	if err != nil {
		respn.Status = "Error : Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotModified, respn)
		return
	}

	// Send notification to the examiner
	message := "*Pengajuan Sidang*\n" +
		"Nama: " + docuser.Name +
		"\nNPM: " + docuser.NPM +
		"\nNomor Kelompok: " + pengajuan.NomorKelompok +
		"\nDosen Pembimbing: " + pengajuan.DosenPembimbing +
		"\nDosen Penguji: " + pengajuan.DosenPenguji +
		"\nTanggal Pengajuan: " + pengajuan.Timestamp.Format("02-01-2006 15:04:05")

	// You'll need to setup a way to get the examiner's phone number based on their name
	// For now, this is just a placeholder - in practice, you'd query the database
	examinerPhone := "6285312924192" // Replace with actual logic to get the examiner's phone

	dt := &whatsauth.TextMessage{
		To:       examinerPhone,
		IsGroup:  false,
		Messages: message,
	}
	_, respWA, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		// Still proceed even if notification fails
		respn.Status = "Warning: Pengajuan berhasil tetapi notifikasi gagal terkirim"
		respn.Response = "Pengajuan sidang berhasil disimpan dengan ID: " + idPengajuan.Hex()
		at.WriteJSON(respw, http.StatusOK, respn)
		return
	}

	// Log the WhatsApp notification response for troubleshooting
	if respWA.Status != "success" {
		respn.Status = "Warning: Pengajuan berhasil tetapi notifikasi mungkin tidak terkirim"
		respn.Response = "Pengajuan sidang berhasil disimpan dengan ID: " + idPengajuan.Hex()
		at.WriteJSON(respw, http.StatusOK, respn)
		return
	}

	// Return success response
	pengajuan.ID = idPengajuan
	at.WriteJSON(respw, http.StatusOK, pengajuan)
}

// GetPengajuanSidang gets the submission status for a user
func GetPengajuanSidang(respw http.ResponseWriter, req *http.Request) {
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

	// Get user's submission
	pengajuanList, err := atdb.GetAllDoc[[]model.BimbinganPengajuan](config.Mongoconn, "bimbingan_pengajuan", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data pengajuan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, pengajuanList)
}
