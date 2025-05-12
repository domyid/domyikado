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

// PostBimbinganPengajuan handles the submission of a sidang (thesis defense) application
func PostBimbinganPengajuan(respw http.ResponseWriter, req *http.Request) {
	// Authorization and input validation
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
	var pengajuan struct {
		DosenPenguji  string `json:"dosenpenguji"`
		NomorKelompok string `json:"nomorkelompok"`
	}

	err = json.NewDecoder(req.Body).Decode(&pengajuan)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validate required fields
	if pengajuan.DosenPenguji == "" || pengajuan.NomorKelompok == "" {
		respn.Status = "Error : Data tidak lengkap"
		respn.Response = "Dosen penguji dan nomor kelompok harus diisi"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get user information
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Check if user has enough bimbingan sessions (at least 1)
	bimbingans, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if len(bimbingans) < 1 {
		respn.Status = "Error : Tidak memenuhi syarat"
		respn.Response = "Anda membutuhkan minimal 1 bimbingan untuk mengajukan sidang"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Create a new BimbinganPengajuan
	newPengajuan := model.BimbinganPengajuan{
		Name:          docuser.Name,
		NPM:           docuser.NPM,
		NomorKelompok: pengajuan.NomorKelompok,
		DosenPenguji:  pengajuan.DosenPenguji,
		PhoneNumber:   docuser.PhoneNumber,
		Timestamp:     time.Now(),
		Status:        "pending", // Default status is pending
	}

	// Insert into database
	pengajuanID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan_pengajuan", newPengajuan)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan pengajuan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send notification to admin CS
	adminCSNumber := "6285312924192"
	message := "*Pengajuan Sidang Baru*\n" +
		"Mahasiswa : " + docuser.Name + "\n" +
		"NPM : " + docuser.NPM + "\n" +
		"Kelompok : " + pengajuan.NomorKelompok + "\n" +
		"Dosen Penguji : " + pengajuan.DosenPenguji + "\n" +
		"ID Pengajuan : " + pengajuanID.Hex()

	dt := &whatsauth.TextMessage{
		To:       adminCSNumber,
		IsGroup:  false,
		Messages: message,
	}
	_, _, _ = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)

	// Return success response
	at.WriteJSON(respw, http.StatusOK, model.Response{
		Status:   "Success",
		Response: "Pengajuan sidang berhasil disimpan",
	})
}

// GetBimbinganPengajuan gets all sidang applications (admin view)
func GetBimbinganPengajuan(respw http.ResponseWriter, req *http.Request) {
	// Authorization (only for admins/teachers)
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Check if user is admin/teacher
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil || !docuser.IsDosen {
		respn.Status = "Error : Tidak memiliki akses"
		respn.Response = "Hanya dosen yang dapat melihat data pengajuan sidang"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get all applications
	pengajuans, err := atdb.GetAllDoc[[]model.BimbinganPengajuan](config.Mongoconn, "bimbingan_pengajuan", primitive.M{})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data pengajuan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Return the applications
	at.WriteJSON(respw, http.StatusOK, pengajuans)
}

// GetBimbinganPengajuanByUser gets all sidang applications for a specific user
func GetBimbinganPengajuanByUser(respw http.ResponseWriter, req *http.Request) {
	// Authorization
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get applications for this user
	pengajuans, err := atdb.GetAllDoc[[]model.BimbinganPengajuan](config.Mongoconn, "bimbingan_pengajuan", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data pengajuan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Return the applications
	at.WriteJSON(respw, http.StatusOK, pengajuans)
}

// UpdateBimbinganPengajuanStatus updates the status of a sidang application (approve/reject)
func UpdateBimbinganPengajuanStatus(respw http.ResponseWriter, req *http.Request) {
	// Authorization (only for admins/teachers)
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Check if user is admin/teacher
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil || !docuser.IsDosen {
		respn.Status = "Error : Tidak memiliki akses"
		respn.Response = "Hanya dosen yang dapat memperbarui status pengajuan sidang"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get application ID from URL param
	id := at.GetParam(req)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Parse request body to get new status
	var updateReq struct {
		Status string `json:"status"` // "approved" or "rejected"
	}
	err = json.NewDecoder(req.Body).Decode(&updateReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validate status
	if updateReq.Status != "approved" && updateReq.Status != "rejected" {
		respn.Status = "Error : Status tidak valid"
		respn.Response = "Status harus 'approved' atau 'rejected'"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get the existing pengajuan to access student information
	pengajuan, err := atdb.GetOneDoc[model.BimbinganPengajuan](config.Mongoconn, "bimbingan_pengajuan", primitive.M{"_id": objectID})
	if err != nil {
		respn.Status = "Error : Data pengajuan tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update the application status
	_, err = atdb.UpdateOneDoc(config.Mongoconn, "bimbingan_pengajuan",
		primitive.M{"_id": objectID},
		primitive.M{"$set": primitive.M{"status": updateReq.Status}})

	if err != nil {
		respn.Status = "Error : Gagal memperbarui status"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send notification to student about status update
	var statusText string
	if updateReq.Status == "approved" {
		statusText = "DISETUJUI"
	} else {
		statusText = "DITOLAK"
	}

	message := "*Pengajuan Sidang " + statusText + "*\n" +
		"Mahasiswa: " + pengajuan.Name + "\n" +
		"NPM: " + pengajuan.NPM + "\n" +
		"Kelompok: " + pengajuan.NomorKelompok + "\n" +
		"Dosen Penguji: " + pengajuan.DosenPenguji + "\n" +
		"Status: " + statusText

	// Send to both student and admin CS
	studentMessage := &whatsauth.TextMessage{
		To:       pengajuan.PhoneNumber,
		IsGroup:  false,
		Messages: message + "\n\nSilakan hubungi dosen Anda untuk informasi lebih lanjut.",
	}
	_, _, _ = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, studentMessage, config.WAAPIMessage)

	adminMessage := &whatsauth.TextMessage{
		To:       "6285312924192", // Admin CS number
		IsGroup:  false,
		Messages: message + "\n\nDiperbarui oleh: " + docuser.Name,
	}
	_, _, _ = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, adminMessage, config.WAAPIMessage)

	// Return success response
	at.WriteJSON(respw, http.StatusOK, model.Response{
		Status:   "Success",
		Response: "Status pengajuan sidang berhasil diperbarui",
	})
}

// GetBimbinganPengajuanById gets a specific sidang application
func GetBimbinganPengajuanById(respw http.ResponseWriter, req *http.Request) {
	// Authorization
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get application ID from URL param
	id := at.GetParam(req)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get the application
	pengajuan, err := atdb.GetOneDoc[model.BimbinganPengajuan](config.Mongoconn, "bimbingan_pengajuan", primitive.M{"_id": objectID})
	if err != nil {
		respn.Status = "Error : Data pengajuan tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Security check - only the owner or dosen can view
	isOwner := pengajuan.PhoneNumber == payload.Id

	// Check if user is dosen
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	isDosen := err == nil && docuser.IsDosen

	if !isOwner && !isDosen {
		respn.Status = "Error : Tidak memiliki akses"
		respn.Response = "Anda tidak berhak melihat data pengajuan ini"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Return the application
	at.WriteJSON(respw, http.StatusOK, pengajuan)
}
