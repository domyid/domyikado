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

// QRCodeSession struct untuk tracking QR code session
type QRCodeSession struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Code       string             `bson:"code" json:"code"`
	CreatedBy  string             `bson:"createdby" json:"createdby"`
	CreatedAt  time.Time          `bson:"createdat" json:"createdat"`
	ExpiresAt  time.Time          `bson:"expiresat" json:"expiresat"`
	IsActive   bool               `bson:"isactive" json:"isactive"`
	ClaimedBy  []string           `bson:"claimedby,omitempty" json:"claimedby,omitempty"`
	ClaimCount int                `bson:"claimcount" json:"claimcount"`
}

// EventQRClaimUsers struct untuk tracking user yang sudah claim QR
type EventQRClaimUsers struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber string             `bson:"phonenumber" json:"phonenumber"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
	QRCode      string             `bson:"qrcode" json:"qrcode"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid,omitempty" json:"bimbinganid,omitempty"`
}

// QRCodeStatus struct untuk tracking status QR system
type QRCodeStatus struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	IsActive  bool               `bson:"isactive" json:"isactive"`
	UpdatedBy string             `bson:"updatedby" json:"updatedby"`
	UpdatedAt time.Time          `bson:"updatedat" json:"updatedat"`
}

// StartQRCodeSession untuk memulai sesi QR code (khusus owner)
func StartQRCodeSession(respw http.ResponseWriter, req *http.Request) {
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
		respn.Response = "Anda tidak memiliki akses untuk memulai sesi QR"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Update status QR system menjadi aktif
	qrStatus := QRCodeStatus{
		IsActive:  true,
		UpdatedBy: payload.Id,
		UpdatedAt: time.Now(),
	}

	// Upsert status
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "qrcodestatus", bson.M{}, qrStatus)
	if err != nil {
		// Jika tidak ada document, insert baru
		_, err = atdb.InsertOneDoc(config.Mongoconn, "qrcodestatus", qrStatus)
		if err != nil {
			respn.Status = "Error : Gagal mengaktifkan sesi QR"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}
	}

	respn.Status = "Success"
	respn.Response = "Sesi QR code telah dimulai"
	at.WriteJSON(respw, http.StatusOK, respn)
}

// StopQRCodeSession untuk menghentikan sesi QR code (khusus owner)
func StopQRCodeSession(respw http.ResponseWriter, req *http.Request) {
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
		respn.Response = "Anda tidak memiliki akses untuk menghentikan sesi QR"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Update status QR system menjadi tidak aktif
	qrStatus := QRCodeStatus{
		IsActive:  false,
		UpdatedBy: payload.Id,
		UpdatedAt: time.Now(),
	}

	// Update status
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "qrcodestatus", bson.M{}, qrStatus)
	if err != nil {
		respn.Status = "Error : Gagal menghentikan sesi QR"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Sesi QR code telah dihentikan"
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GenerateQRCode untuk generate QR code baru setiap 20 detik
func GenerateQRCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Cek status QR system
	qrStatus, err := atdb.GetOneDoc[QRCodeStatus](config.Mongoconn, "qrcodestatus", bson.M{})
	if err != nil || !qrStatus.IsActive {
		respn.Status = "Error : Sesi QR tidak aktif"
		respn.Response = "Sesi QR code sedang tidak aktif. Hubungi admin."
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Generate random code
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		respn.Status = "Error : Gagal generate kode"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	code := "QR" + hex.EncodeToString(bytes)

	// Create QR session
	qrSession := QRCodeSession{
		Code:       code,
		CreatedBy:  "system",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(25 * time.Second), // 25 detik untuk buffer
		IsActive:   true,
		ClaimedBy:  []string{},
		ClaimCount: 0,
	}

	// Deactivate previous QR codes
	activeQRSessions, err := atdb.GetAllDoc[[]QRCodeSession](config.Mongoconn, "qrcodesessions", bson.M{"isactive": true})
	if err == nil {
		for _, session := range activeQRSessions {
			session.IsActive = false
			_, _ = atdb.ReplaceOneDoc(config.Mongoconn, "qrcodesessions", bson.M{"_id": session.ID}, session)
		}
	}

	// Insert new QR session
	_, err = atdb.InsertOneDoc(config.Mongoconn, "qrcodesessions", qrSession)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan QR session"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	result := map[string]interface{}{
		"qrcode":    code,
		"expiresAt": qrSession.ExpiresAt,
		"isActive":  true,
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// ClaimQRCode untuk claim bimbingan melalui QR code
func ClaimQRCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var claimReq struct {
		QRCode string `json:"qrcode"`
	}
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user sudah pernah claim QR sebelumnya
	existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers", bson.M{"phonenumber": payload.Id})
	if existingClaim.PhoneNumber != "" {
		respn.Status = "Error : Sudah pernah claim"
		respn.Response = "Anda sudah pernah menggunakan QR code claim bimbingan sebelumnya"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek QR code session
	qrSession, err := atdb.GetOneDoc[QRCodeSession](config.Mongoconn, "qrcodesessions", bson.M{
		"code":      claimReq.QRCode,
		"isactive":  true,
		"expiresat": bson.M{"$gt": time.Now()},
	})
	if err != nil {
		respn.Status = "Error : QR code tidak valid"
		respn.Response = "QR code tidak ditemukan atau sudah kadaluarsa"
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
		Komentar:        "Bonus Bimbingan dari QR Code Claim: " + claimReq.QRCode,
		Validasi:        5, // Rating 5 untuk QR claim
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Update QR session dengan claim info
	qrSession.ClaimedBy = append(qrSession.ClaimedBy, payload.Id)
	qrSession.ClaimCount++
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "qrcodesessions", bson.M{"_id": qrSession.ID}, qrSession)
	if err != nil {
		// Log error but continue
	}

	// Save user claim record
	userClaim := EventQRClaimUsers{
		PhoneNumber: payload.Id,
		ClaimedAt:   time.Now(),
		QRCode:      claimReq.QRCode,
		BimbinganID: bimbinganID,
	}
	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventqrclaimusers", userClaim)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan record claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "QR Code berhasil diklaim! Bimbingan bonus telah ditambahkan."
	at.WriteJSON(respw, http.StatusOK, respn)
}

// CheckQRClaimStatus untuk cek apakah user sudah pernah claim QR
func CheckQRClaimStatus(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Cek apakah user sudah pernah claim
	existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers", bson.M{"phonenumber": payload.Id})

	result := map[string]interface{}{
		"hasClaimed": existingClaim.PhoneNumber != "",
	}

	if existingClaim.PhoneNumber != "" {
		result["claimedAt"] = existingClaim.ClaimedAt
		result["qrCode"] = existingClaim.QRCode
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// GetQRSystemStatus untuk cek status sistem QR
func GetQRSystemStatus(respw http.ResponseWriter, req *http.Request) {
	qrStatus, err := atdb.GetOneDoc[QRCodeStatus](config.Mongoconn, "qrcodestatus", bson.M{})
	if err != nil {
		result := map[string]interface{}{
			"isActive": false,
			"message":  "Sistem QR belum diinisialisasi",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	result := map[string]interface{}{
		"isActive":  qrStatus.IsActive,
		"updatedBy": qrStatus.UpdatedBy,
		"updatedAt": qrStatus.UpdatedAt,
	}

	at.WriteJSON(respw, http.StatusOK, result)
}
