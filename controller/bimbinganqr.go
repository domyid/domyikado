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

// QRSession struct untuk tracking session QR code
type QRSession struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	SessionID string             `bson:"sessionid" json:"sessionid"`
	CreatedBy string             `bson:"createdby" json:"createdby"`
	CreatedAt time.Time          `bson:"createdat" json:"createdat"`
	IsActive  bool               `bson:"isactive" json:"isactive"`
	StoppedAt time.Time          `bson:"stoppedat,omitempty" json:"stoppedat,omitempty"`
	StoppedBy string             `bson:"stoppedby,omitempty" json:"stoppedby,omitempty"`
	ExpiresAt time.Time          `bson:"expiresat" json:"expiresat"`
}

// EventQRClaimUsers struct untuk tracking user yang sudah claim via QR
type EventQRClaimUsers struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber string             `bson:"phonenumber" json:"phonenumber"`
	SessionID   string             `bson:"sessionid" json:"sessionid"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid" json:"bimbinganid"`
}

// GenerateQRSession untuk generate session QR code (khusus owner)
func GenerateQRSession(respw http.ResponseWriter, req *http.Request) {
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
		respn.Response = "Anda tidak memiliki akses untuk generate QR session"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Stop semua session yang masih aktif terlebih dahulu dengan loop
	activeSessions, _ := atdb.GetAllDoc[[]QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})
	for _, session := range activeSessions {
		session.IsActive = false
		session.StoppedAt = time.Now()
		session.StoppedBy = payload.Id
		atdb.ReplaceOneDoc(config.Mongoconn, "qrsessions", bson.M{"_id": session.ID}, session)
	}

	// Generate random session ID
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		respn.Status = "Error : Gagal generate session ID"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	sessionID := "QR" + hex.EncodeToString(bytes)

	// Buat session baru dengan expires 20 detik
	newSession := QRSession{
		SessionID: sessionID,
		CreatedBy: payload.Id,
		CreatedAt: time.Now(),
		IsActive:  true,
		ExpiresAt: time.Now().Add(20 * time.Second),
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "qrsessions", newSession)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan QR session"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	result := map[string]interface{}{
		"sessionId": sessionID,
		"expiresAt": newSession.ExpiresAt,
		"isActive":  true,
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// StopQRSession untuk menghentikan session QR code (khusus owner)
func StopQRSession(respw http.ResponseWriter, req *http.Request) {
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
		respn.Response = "Anda tidak memiliki akses untuk stop QR session"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Stop semua session yang masih aktif dengan loop
	activeSessions, err := atdb.GetAllDoc[[]QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})
	if err != nil {
		respn.Status = "Error : Gagal ambil session aktif"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	stopCount := 0
	for _, session := range activeSessions {
		session.IsActive = false
		session.StoppedAt = time.Now()
		session.StoppedBy = payload.Id
		_, err := atdb.ReplaceOneDoc(config.Mongoconn, "qrsessions", bson.M{"_id": session.ID}, session)
		if err == nil {
			stopCount++
		}
	}

	if err != nil {
		respn.Status = "Error : Gagal stop QR session"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "QR Session berhasil dihentikan"
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetActiveQRSession untuk mendapatkan session aktif
func GetActiveQRSession(respw http.ResponseWriter, req *http.Request) {
	// Cari session yang masih aktif dan belum expired
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions",
		bson.M{
			"isactive":  true,
			"expiresat": bson.M{"$gt": time.Now()},
		})

	if err != nil {
		result := map[string]interface{}{
			"isActive":  false,
			"sessionId": "",
			"message":   "Tidak ada session QR aktif",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	result := map[string]interface{}{
		"isActive":  true,
		"sessionId": activeSession.SessionID,
		"expiresAt": activeSession.ExpiresAt,
		"createdBy": activeSession.CreatedBy,
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// ClaimQRBimbingan untuk claim bimbingan via QR code
func ClaimQRBimbingan(respw http.ResponseWriter, req *http.Request) {
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
		SessionID string `json:"sessionId"`
	}
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user sudah pernah claim via QR
	existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers",
		bson.M{"phonenumber": payload.Id})
	if existingClaim.PhoneNumber != "" {
		respn.Status = "Error : Sudah pernah claim"
		respn.Response = "Anda sudah pernah menggunakan QR code untuk mendapatkan bimbingan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek session QR
	_, err = atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions",
		bson.M{
			"sessionid": claimReq.SessionID,
			"isactive":  true,
			"expiresat": bson.M{"$gt": time.Now()},
		})
	if err != nil {
		respn.Status = "Error : Session tidak valid"
		respn.Response = "Session QR tidak ditemukan atau sudah expired"
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
		Komentar:        "Bonus Bimbingan dari QR Code Session: " + claimReq.SessionID,
		Validasi:        5, // Rating 5 untuk QR
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Simpan record claim QR
	qrClaim := EventQRClaimUsers{
		PhoneNumber: payload.Id,
		SessionID:   claimReq.SessionID,
		ClaimedAt:   time.Now(),
		BimbinganID: bimbinganID,
	}
	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventqrclaimusers", qrClaim)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan record claim QR"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "QR Code berhasil diklaim! Bimbingan bonus telah ditambahkan."
	at.WriteJSON(respw, http.StatusOK, respn)
}

// CheckQRClaimStatus untuk cek apakah user sudah pernah claim via QR
func CheckQRClaimStatus(respw http.ResponseWriter, req *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		result := map[string]interface{}{
			"hasClaimed": false,
			"message":    "Token tidak valid",
		}
		at.WriteJSON(respw, http.StatusOK, result)
		return
	}

	// Cek apakah user sudah pernah claim via QR
	existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers",
		bson.M{"phonenumber": payload.Id})

	result := map[string]interface{}{
		"hasClaimed": existingClaim.PhoneNumber != "",
	}

	if existingClaim.PhoneNumber != "" {
		result["claimedAt"] = existingClaim.ClaimedAt
		result["sessionId"] = existingClaim.SessionID
		result["message"] = "Anda sudah pernah claim bimbingan via QR Code"
	} else {
		result["message"] = "Anda belum pernah claim bimbingan via QR Code"
	}

	at.WriteJSON(respw, http.StatusOK, result)
}
