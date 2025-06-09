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

// QRSession struct untuk menyimpan session QR code
type QRSession struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	SessionID string             `bson:"sessionid" json:"sessionid"`
	CreatedBy string             `bson:"createdby" json:"createdby"`
	CreatedAt time.Time          `bson:"createdat" json:"createdat"`
	IsActive  bool               `bson:"isactive" json:"isactive"`
	StoppedAt time.Time          `bson:"stoppedat,omitempty" json:"stoppedat,omitempty"`
	StoppedBy string             `bson:"stoppedby,omitempty" json:"stoppedby,omitempty"`
}

// QRCode struct untuk menyimpan QR code individual
type QRCode struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	SessionID   string             `bson:"sessionid" json:"sessionid"`
	Code        string             `bson:"code" json:"code"`
	CreatedAt   time.Time          `bson:"createdat" json:"createdat"`
	ExpiredAt   time.Time          `bson:"expiredat" json:"expiredat"`
	IsUsed      bool               `bson:"isused" json:"isused"`
	UsedBy      string             `bson:"usedby,omitempty" json:"usedby,omitempty"`
	UsedAt      time.Time          `bson:"usedat,omitempty" json:"usedat,omitempty"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid,omitempty" json:"bimbinganid,omitempty"`
}

// EventQRClaimUsers struct untuk tracking user yang sudah claim
type EventQRClaimUsers struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber string             `bson:"phonenumber" json:"phonenumber"`
	SessionID   string             `bson:"sessionid" json:"sessionid"`
	QRCode      string             `bson:"qrcode" json:"qrcode"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid" json:"bimbinganid"`
}

// StartQRSession untuk memulai session QR code (khusus owner)
func StartQRSession(respw http.ResponseWriter, req *http.Request) {
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
		respn.Response = "Anda tidak memiliki akses untuk memulai session QR"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Cek apakah ada session yang masih aktif
	activeSession, _ := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})
	if activeSession.SessionID != "" {
		respn.Status = "Error : Session aktif sudah ada"
		respn.Response = "Harap stop session yang aktif terlebih dahulu. Session ID: " + activeSession.SessionID
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Generate session ID
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		respn.Status = "Error : Gagal generate session ID"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	sessionID := "QRS" + hex.EncodeToString(bytes)

	// Buat session baru
	qrSession := QRSession{
		SessionID: sessionID,
		CreatedBy: payload.Id,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "qrsessions", qrSession)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan session"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Generate QR code pertama
	firstQRCode, err := generateQRCode(sessionID)
	if err != nil {
		respn.Status = "Error : Gagal generate QR code pertama"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	result := map[string]interface{}{
		"sessionID": sessionID,
		"qrCode":    firstQRCode.Code,
		"message":   "Session QR berhasil dimulai",
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// GetCurrentQRCode untuk mendapatkan QR code yang sedang aktif
func GetCurrentQRCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Cek session yang aktif
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})
	if err != nil || activeSession.SessionID == "" {
		respn.Status = "Error : Tidak ada session aktif"
		respn.Response = "Tidak ada session QR yang sedang berjalan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek QR code yang masih valid (belum expired)
	now := time.Now()
	validQRCode, err := atdb.GetOneDoc[QRCode](config.Mongoconn, "qrcodes", bson.M{
		"sessionid": activeSession.SessionID,
		"expiredat": bson.M{"$gt": now},
		"isused":    false,
	})

	if err != nil || validQRCode.Code == "" {
		// Generate QR code baru jika tidak ada yang valid
		newQRCode, err := generateQRCode(activeSession.SessionID)
		if err != nil {
			respn.Status = "Error : Gagal generate QR code baru"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}
		validQRCode = *newQRCode
	}

	result := map[string]interface{}{
		"sessionID": activeSession.SessionID,
		"qrCode":    validQRCode.Code,
		"expiredAt": validQRCode.ExpiredAt,
		"isActive":  true,
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// StopQRSession untuk menghentikan session QR (khusus owner)
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
		respn.Response = "Anda tidak memiliki akses untuk menghentikan session QR"
		at.WriteJSON(respw, http.StatusUnauthorized, respn)
		return
	}

	// Cek session yang aktif
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})
	if err != nil || activeSession.SessionID == "" {
		respn.Status = "Error : Tidak ada session aktif"
		respn.Response = "Tidak ada session QR yang sedang berjalan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update session menjadi tidak aktif
	activeSession.IsActive = false
	activeSession.StoppedAt = time.Now()
	activeSession.StoppedBy = payload.Id

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "qrsessions", bson.M{"_id": activeSession.ID}, activeSession)
	if err != nil {
		respn.Status = "Error : Gagal menghentikan session"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Session QR berhasil dihentikan"
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ClaimQRCode untuk claim bimbingan via QR code
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

	// Cek apakah QR code valid
	qrCode, err := atdb.GetOneDoc[QRCode](config.Mongoconn, "qrcodes", bson.M{"code": claimReq.QRCode})
	if err != nil {
		respn.Status = "Error : QR Code tidak valid"
		respn.Response = "QR Code tidak ditemukan atau sudah expired"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah QR code sudah expired
	if time.Now().After(qrCode.ExpiredAt) {
		respn.Status = "Error : QR Code expired"
		respn.Response = "QR Code sudah tidak berlaku"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah QR code sudah digunakan
	if qrCode.IsUsed {
		respn.Status = "Error : QR Code sudah digunakan"
		respn.Response = "QR Code sudah digunakan oleh pengguna lain"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah session masih aktif
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{
		"sessionid": qrCode.SessionID,
		"isactive":  true,
	})
	if err != nil || activeSession.SessionID == "" {
		respn.Status = "Error : Session tidak aktif"
		respn.Response = "Session QR sudah berakhir"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user sudah pernah claim untuk session ini
	existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers", bson.M{
		"phonenumber": payload.Id,
		"sessionid":   qrCode.SessionID,
	})
	if existingClaim.PhoneNumber != "" {
		respn.Status = "Error : Sudah pernah claim"
		respn.Response = "Anda sudah mengambil bimbingan untuk session ini"
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
		Komentar:        "Bonus Bimbingan dari QR Code Session: " + qrCode.SessionID,
		Validasi:        5, // Rating 5 untuk QR code
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Update QR code as used
	qrCode.IsUsed = true
	qrCode.UsedBy = payload.Id
	qrCode.UsedAt = time.Now()
	qrCode.BimbinganID = bimbinganID
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "qrcodes", bson.M{"_id": qrCode.ID}, qrCode)
	if err != nil {
		respn.Status = "Error : Gagal update QR code"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Save user claim record
	userClaim := EventQRClaimUsers{
		PhoneNumber: payload.Id,
		SessionID:   qrCode.SessionID,
		QRCode:      claimReq.QRCode,
		ClaimedAt:   time.Now(),
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

// generateQRCode helper function untuk generate QR code
func generateQRCode(sessionID string) (*QRCode, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}

	code := "QR" + hex.EncodeToString(bytes)
	now := time.Now()
	expiredAt := now.Add(20 * time.Second) // Expired setelah 20 detik

	qrCode := QRCode{
		SessionID: sessionID,
		Code:      code,
		CreatedAt: now,
		ExpiredAt: expiredAt,
		IsUsed:    false,
	}

	_, err := atdb.InsertOneDoc(config.Mongoconn, "qrcodes", qrCode)
	if err != nil {
		return nil, err
	}

	return &qrCode, nil
}

// GetQRSessionStatus untuk cek status session QR
func GetQRSessionStatus(respw http.ResponseWriter, req *http.Request) {
	// Cek session yang aktif
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})

	result := map[string]interface{}{
		"isActive": false,
		"message":  "Tidak ada session QR yang aktif",
	}

	if err == nil && activeSession.SessionID != "" {
		result["isActive"] = true
		result["sessionID"] = activeSession.SessionID
		result["createdAt"] = activeSession.CreatedAt
		result["message"] = "Session QR sedang aktif"
	}

	at.WriteJSON(respw, http.StatusOK, result)
}

// CheckUserQRClaimStatus untuk cek apakah user sudah claim untuk session aktif
func CheckUserQRClaimStatus(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Cek session yang aktif
	activeSession, err := atdb.GetOneDoc[QRSession](config.Mongoconn, "qrsessions", bson.M{"isactive": true})

	result := map[string]interface{}{
		"hasClaimed": false,
		"canClaim":   false,
		"message":    "Tidak ada session QR yang aktif",
	}

	if err == nil && activeSession.SessionID != "" {
		// Cek apakah user sudah claim untuk session ini
		existingClaim, _ := atdb.GetOneDoc[EventQRClaimUsers](config.Mongoconn, "eventqrclaimusers", bson.M{
			"phonenumber": payload.Id,
			"sessionid":   activeSession.SessionID,
		})

		if existingClaim.PhoneNumber != "" {
			result["hasClaimed"] = true
			result["canClaim"] = false
			result["message"] = "Anda sudah mengambil bimbingan untuk session ini"
			result["claimedAt"] = existingClaim.ClaimedAt
		} else {
			result["hasClaimed"] = false
			result["canClaim"] = true
			result["message"] = "Anda dapat mengambil bimbingan dengan scan QR code"
		}
		result["sessionID"] = activeSession.SessionID
	}

	at.WriteJSON(respw, http.StatusOK, result)
}
