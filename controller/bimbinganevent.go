package controller

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GenerateEventCode untuk generate kode referral (khusus owner)
func GenerateEventCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Cek apakah user adalah owner
	allowedNumbers := []string{"6285312924192", "6282117252716"}
	isOwner := false
	for _, num := range allowedNumbers {
		if payload.Id == num {
			isOwner = true
			break
		}
	}

	if !isOwner {
		respn.Status = "Error : Unauthorized"
		respn.Response = "Anda tidak memiliki akses untuk generate kode"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Generate random code
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		respn.Status = "Error : Gagal generate kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	code := "EVENT" + hex.EncodeToString(bytes)

	// Simpan ke database
	eventCode := model.EventCode{
		Code:      code,
		CreatedBy: payload.Id,
		CreatedAt: time.Now(),
		IsUsed:    false,
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventcodes", eventCode)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = code
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GenerateEventCodeTime untuk generate kode dengan waktu kadaluarsa (khusus owner)
func GenerateEventCodeTime(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Cek apakah user adalah owner
	allowedNumbers := []string{"6285312924192", "6282117252716"}
	isOwner := false
	for _, num := range allowedNumbers {
		if payload.Id == num {
			isOwner = true
			break
		}
	}

	if !isOwner {
		respn.Status = "Error : Unauthorized"
		respn.Response = "Anda tidak memiliki akses untuk generate kode"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Parse request body untuk mendapatkan duration
	var timeReq model.TimeCodeGenerateRequest
	err = json.NewDecoder(req.Body).Decode(&timeReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if timeReq.DurationSeconds <= 0 {
		respn.Status = "Error : Durasi harus lebih dari 0 detik"
		respn.Response = "Masukkan durasi yang valid"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Generate random code
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		respn.Status = "Error : Gagal generate kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	code := "TIME" + hex.EncodeToString(bytes)

	// Hitung waktu kadaluarsa
	now := time.Now()
	expiresAt := now.Add(time.Duration(timeReq.DurationSeconds) * time.Second)

	// Simpan ke database
	eventCodeTime := model.EventCodeTime{
		Code:        code,
		CreatedBy:   payload.Id,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		DurationSec: timeReq.DurationSeconds,
		IsActive:    true,
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventcodetime", eventCodeTime)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Return response sebagai custom JSON object, bukan menggunakan model.Response
	responseData := map[string]interface{}{
		"status": "Success",
		"data": map[string]interface{}{
			"code":       code,
			"expires_at": expiresAt.Format("2006-01-02 15:04:05"),
			"duration":   timeReq.DurationSeconds,
		},
	}

	at.WriteJSON(respw, http.StatusOK, responseData)
}

// ClaimEventCode untuk claim kode referral dan menambah bimbingan
func ClaimEventCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var claimReq model.TimeCodeClaimRequest
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek kode event
	eventCode, err := atdb.GetOneDoc[model.EventCode](config.Mongoconn, "eventcodes", bson.M{"code": claimReq.Code})
	if err != nil {
		respn.Status = "Error : Kode tidak valid"
		respn.Response = "Kode referral tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah kode sudah digunakan
	if eventCode.IsUsed {
		respn.Status = "Error : Kode sudah digunakan"
		respn.Response = "Kode referral sudah digunakan oleh pengguna lain"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get user data
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get existing bimbingan count untuk user ini
	allBimbingan, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", bson.M{"phonenumber": payload.Id})
	if err != nil {
		// Jika belum ada bimbingan sama sekali, mulai dari 1
		allBimbingan = []model.ActivityScore{}
	}

	// Tentukan bimbinganke selanjutnya
	nextBimbinganKe := len(allBimbingan) + 1

	// Set asesor tetap (Dirga F)
	asesor := model.Userdomyikado{
		ID:                   primitive.ObjectID{},
		Name:                 "Dirga F",
		PhoneNumber:          "6282268895372",
		Email:                "fdirga63@gmail.com",
		GithubUsername:       "Febriand1",
		GitlabUsername:       "Febriand1",
		Poin:                 495.45255417617057,
		SponsorName:          "dapa",
		SponsorPhoneNumber:   "6282117252716",
		StravaProfilePicture: "https://lh3.googleusercontent.com/a/ACg8ocK27sU9YXcfmLm9Zw_MtUW0kT--NA",
		NPM:                  "1214039",
		IsDosen:              true,
	}

	// Isi ObjectID untuk asesor
	asesorObjectID, _ := primitive.ObjectIDFromHex("6659322de7219a1a041fff04")
	asesor.ID = asesorObjectID

	// Create bimbingan entry dengan semua nilai 0
	bimbingan := model.ActivityScore{
		BimbinganKe: nextBimbinganKe,
		Approved:    true,
		Username:    docuser.Name,
		PhoneNumber: docuser.PhoneNumber,
		Asesor:      asesor,
		CreatedAt:   time.Now(),
		// Set semua activity scores ke 0
		Trackerdata:     0,
		Tracker:         0,
		StravaKM:        0,
		Strava:          0,
		IQresult:        0,
		IQ:              0,
		MBC:             0,
		MBCPoints:       0,
		RVN:             0,
		RavencoinPoints: 0,
		QRIS:            0,
		QRISPoints:      0,
		Pomokitsesi:     0,
		Pomokit:         0,
		GTMetrixResult:  "",
		GTMetrix:        0,
		WebHookpush:     0,
		WebHook:         0,
		PresensiHari:    0,
		Presensi:        0,
		Sponsordata:     0,
		Sponsor:         0,
		BukuKatalog:     "",
		BukPed:          0,
		JurnalWeb:       "",
		Jurnal:          0,
		TotalScore:      0,
		Komentar:        "Bonus Bimbingan dari Event Referral Code: " + claimReq.Code,
		Validasi:        5, // Rating 5 untuk event
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Update event code as used
	eventCode.IsUsed = true
	eventCode.UsedBy = payload.Id
	eventCode.UsedAt = time.Now()
	eventCode.BimbinganID = bimbinganID
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventcodes", bson.M{"_id": eventCode.ID}, eventCode)
	if err != nil {
		respn.Status = "Error : Gagal update status kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Kode berhasil diklaim! Bimbingan bonus telah ditambahkan."
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ClaimEventCodeTime untuk claim kode dengan waktu kadaluarsa
func ClaimEventCodeTime(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var claimReq model.TimeCodeClaimRequest
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek kode event time
	eventCodeTime, err := atdb.GetOneDoc[model.EventCodeTime](config.Mongoconn, "eventcodetime", bson.M{"code": claimReq.Code})
	if err != nil {
		respn.Status = "Error : Kode tidak valid"
		respn.Response = "Kode time event tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah kode masih aktif
	if !eventCodeTime.IsActive {
		respn.Status = "Error : Kode tidak aktif"
		respn.Response = "Kode sudah tidak aktif"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah kode sudah kadaluarsa
	now := time.Now()
	if now.After(eventCodeTime.ExpiresAt) {
		// Update status kode menjadi tidak aktif
		eventCodeTime.IsActive = false
		atdb.ReplaceOneDoc(config.Mongoconn, "eventcodetime", bson.M{"_id": eventCodeTime.ID}, eventCodeTime)

		respn.Status = "Error : Kode kadaluarsa"
		respn.Response = "Kode sudah kadaluarsa dan tidak dapat digunakan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user sudah pernah claim kode ini
	_, err = atdb.GetOneDoc[model.EventUserCodeTime](config.Mongoconn, "eventusercodetime", bson.M{
		"codeid":    eventCodeTime.ID,
		"userphone": payload.Id,
	})
	if err == nil {
		respn.Status = "Error : Sudah pernah claim"
		respn.Response = "Anda sudah pernah mengclaim kode ini sebelumnya"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get user data
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get existing bimbingan count untuk user ini
	allBimbingan, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", bson.M{"phonenumber": payload.Id})
	if err != nil {
		allBimbingan = []model.ActivityScore{}
	}

	// Tentukan bimbinganke selanjutnya
	nextBimbinganKe := len(allBimbingan) + 1

	// Set asesor tetap (Dirga F)
	asesor := model.Userdomyikado{
		ID:                   primitive.ObjectID{},
		Name:                 "Dirga F",
		PhoneNumber:          "6282268895372",
		Email:                "fdirga63@gmail.com",
		GithubUsername:       "Febriand1",
		GitlabUsername:       "Febriand1",
		Poin:                 495.45255417617057,
		SponsorName:          "dapa",
		SponsorPhoneNumber:   "6282117252716",
		StravaProfilePicture: "https://lh3.googleusercontent.com/a/ACg8ocK27sU9YXcfmLm9Zw_MtUW0kT--NA",
		NPM:                  "1214039",
		IsDosen:              true,
	}

	// Isi ObjectID untuk asesor
	asesorObjectID, _ := primitive.ObjectIDFromHex("6659322de7219a1a041fff04")
	asesor.ID = asesorObjectID

	// Create bimbingan entry dengan semua nilai 0
	bimbingan := model.ActivityScore{
		BimbinganKe: nextBimbinganKe,
		Approved:    true,
		Username:    docuser.Name,
		PhoneNumber: docuser.PhoneNumber,
		Asesor:      asesor,
		CreatedAt:   time.Now(),
		// Set semua activity scores ke 0
		Trackerdata:     0,
		Tracker:         0,
		StravaKM:        0,
		Strava:          0,
		IQresult:        0,
		IQ:              0,
		MBC:             0,
		MBCPoints:       0,
		RVN:             0,
		RavencoinPoints: 0,
		QRIS:            0,
		QRISPoints:      0,
		Pomokitsesi:     0,
		Pomokit:         0,
		GTMetrixResult:  "",
		GTMetrix:        0,
		WebHookpush:     0,
		WebHook:         0,
		PresensiHari:    0,
		Presensi:        0,
		Sponsordata:     0,
		Sponsor:         0,
		BukuKatalog:     "",
		BukPed:          0,
		JurnalWeb:       "",
		Jurnal:          0,
		TotalScore:      0,
		Komentar:        "Bonus Bimbingan dari Event Time Code: " + claimReq.Code,
		Validasi:        5, // Rating 5 untuk event
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Simpan record user claim
	eventUserCodeTime := model.EventUserCodeTime{
		CodeID:      eventCodeTime.ID,
		Code:        eventCodeTime.Code,
		UserPhone:   payload.Id,
		ClaimedAt:   time.Now(),
		BimbinganID: bimbinganID,
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventusercodetime", eventUserCodeTime)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan record claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Kode time berhasil diklaim! Bimbingan bonus telah ditambahkan."
	at.WriteJSON(respw, http.StatusOK, respn)
}

// CheckEventClaimStatus untuk cek status kode event - tidak lagi cek per user
func CheckEventClaimStatus(respw http.ResponseWriter, req *http.Request) {
	result := map[string]interface{}{
		"hasClaimed": false,
		"message":    "Silakan masukkan kode event yang valid",
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// CheckEventTimeClaimStatus untuk cek status user sudah pernah claim kode time
func CheckEventTimeClaimStatus(respw http.ResponseWriter, req *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		result := map[string]interface{}{
			"hasClaimedTime": false,
			"message":        "Token tidak valid",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	// Get query parameter untuk cek kode spesifik (optional)
	code := req.URL.Query().Get("code")
	if code == "" {
		result := map[string]interface{}{
			"hasClaimedTime": false,
			"message":        "Silakan masukkan kode time event yang valid",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	// Cek apakah user sudah pernah claim kode ini
	_, err = atdb.GetOneDoc[model.EventUserCodeTime](config.Mongoconn, "eventusercodetime", bson.M{
		"code":      code,
		"userphone": payload.Id,
	})

	if err == nil {
		// User sudah pernah claim
		result := map[string]interface{}{
			"hasClaimedTime": true,
			"message":        "Anda sudah pernah mengclaim kode ini",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	// User belum pernah claim
	result := map[string]interface{}{
		"hasClaimedTime": false,
		"message":        "Kode dapat diklaim",
	}
	at.WriteJSON(respw, http.StatusOK, result)
}
