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

// EventCode struct untuk menyimpan kode referral
type EventCode struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Code        string             `bson:"code" json:"code"`
	CreatedBy   string             `bson:"createdby" json:"createdby"`
	CreatedAt   time.Time          `bson:"createdat" json:"createdat"`
	IsUsed      bool               `bson:"isused" json:"isused"`
	UsedBy      string             `bson:"usedby,omitempty" json:"usedby,omitempty"`
	UsedAt      time.Time          `bson:"usedat,omitempty" json:"usedat,omitempty"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid,omitempty" json:"bimbinganid,omitempty"`
}

// UserEventClaim struct untuk tracking user yang sudah claim
type UserEventClaim struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber string             `bson:"phonenumber" json:"phonenumber"`
	EventCode   string             `bson:"eventcode" json:"eventcode"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
}

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
	eventCode := EventCode{
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
	var claimReq struct {
		Code string `json:"code"`
	}
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user sudah pernah claim
	existingClaim, _ := atdb.GetOneDoc[UserEventClaim](config.Mongoconn, "usereventclaims", bson.M{"phonenumber": payload.Id})
	if existingClaim.PhoneNumber != "" {
		respn.Status = "Error : Sudah pernah claim"
		respn.Response = "Anda sudah pernah menggunakan kode referral event"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek kode event
	eventCode, err := atdb.GetOneDoc[EventCode](config.Mongoconn, "eventcodes", bson.M{"code": claimReq.Code})
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

	// Get activity score data
	score, _ := GetAllActivityScoreData(payload.Id)

	// Create bimbingan entry
	bimbingan := model.ActivityScore{
		BimbinganKe:     99, // Special marker for event bimbingan
		Approved:        true,
		Username:        docuser.Name,
		PhoneNumber:     docuser.PhoneNumber,
		CreatedAt:       time.Now(),
		Trackerdata:     score.Trackerdata,
		Tracker:         score.Tracker,
		StravaKM:        score.StravaKM,
		Strava:          score.Strava,
		IQresult:        score.IQresult,
		IQ:              score.IQ,
		MBC:             score.MBC,
		MBCPoints:       score.MBCPoints,
		RVN:             score.RVN,
		RavencoinPoints: score.RavencoinPoints,
		QRIS:            score.QRIS,
		QRISPoints:      score.QRISPoints,
		Pomokitsesi:     score.Pomokitsesi,
		Pomokit:         score.Pomokit,
		GTMetrixResult:  score.GTMetrixResult,
		GTMetrix:        score.GTMetrix,
		WebHookpush:     score.WebHookpush,
		WebHook:         score.WebHook,
		PresensiHari:    score.PresensiHari,
		Presensi:        score.Presensi,
		Sponsordata:     score.Sponsordata,
		Sponsor:         score.Sponsor,
		BukuKatalog:     score.BukuKatalog,
		BukPed:          score.BukPed,
		JurnalWeb:       score.JurnalWeb,
		Jurnal:          score.Jurnal,
		TotalScore:      score.TotalScore,
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

	// Save user claim record
	userClaim := UserEventClaim{
		PhoneNumber: payload.Id,
		EventCode:   claimReq.Code,
		ClaimedAt:   time.Now(),
	}
	_, err = atdb.InsertOneDoc(config.Mongoconn, "usereventclaims", userClaim)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan record claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Kode berhasil diklaim! Bimbingan bonus telah ditambahkan."
	at.WriteJSON(respw, http.StatusOK, respn)
}

// CheckEventClaimStatus untuk cek apakah user sudah claim
func CheckEventClaimStatus(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Cek apakah user sudah pernah claim
	existingClaim, _ := atdb.GetOneDoc[UserEventClaim](config.Mongoconn, "usereventclaims", bson.M{"phonenumber": payload.Id})

	result := map[string]interface{}{
		"hasClaimed": existingClaim.PhoneNumber != "",
	}

	if existingClaim.PhoneNumber != "" {
		result["claimedAt"] = existingClaim.ClaimedAt
		result["eventCode"] = existingClaim.EventCode
	}

	at.WriteJSON(respw, http.StatusOK, result)
}
