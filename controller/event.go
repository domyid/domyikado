package controller

import (
	"encoding/json"
	"fmt"
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

// CreateEvent untuk membuat event baru (khusus owner)
func CreateEvent(respw http.ResponseWriter, req *http.Request) {
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
		respn.Status = "Error : Akses Ditolak"
		respn.Response = "Hanya owner yang dapat membuat event"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var eventReq model.EventCreateRequest
	err = json.NewDecoder(req.Body).Decode(&eventReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validasi input
	if eventReq.Name == "" || eventReq.Description == "" || eventReq.Points <= 0 {
		respn.Status = "Error : Data tidak lengkap"
		respn.Response = "Nama, deskripsi, dan poin harus diisi dengan benar"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Buat event baru
	event := model.Event{
		Name:        eventReq.Name,
		Description: eventReq.Description,
		Points:      eventReq.Points,
		CreatedBy:   payload.Id,
		CreatedAt:   time.Now(),
		IsActive:    true,
	}

	// Simpan ke database
	eventID, err := atdb.InsertOneDoc(config.Mongoconn, "events", event)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Event berhasil dibuat"
	respn.Data = map[string]interface{}{
		"event_id": eventID,
		"event":    event,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetAllEvents untuk mendapatkan semua event aktif
func GetAllEvents(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get all active events
	events, err := atdb.GetAllDoc[[]model.Event](config.Mongoconn, "events", primitive.M{"isactive": true})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get user's claims to check which events are already claimed by this user
	userClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"userphone": payload.Id,
		"status":    primitive.M{"$in": []string{"claimed", "submitted"}},
	})
	if err != nil {
		userClaims = []model.EventClaim{} // If error, assume no claims
	}

	// Get all active claims from any user to check if event is claimed by others
	allActiveClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"status": primitive.M{"$in": []string{"claimed", "submitted"}},
	})
	if err != nil {
		allActiveClaims = []model.EventClaim{} // If error, assume no claims
	}

	// Create map of claimed event IDs by this user
	userClaimedEventIDs := make(map[string]bool)
	for _, claim := range userClaims {
		userClaimedEventIDs[claim.EventID.Hex()] = true
	}

	// Create map of claimed event IDs by any user
	allClaimedEventIDs := make(map[string]bool)
	for _, claim := range allActiveClaims {
		allClaimedEventIDs[claim.EventID.Hex()] = true
	}

	// Add claim status to events
	var eventsWithStatus []map[string]interface{}
	for _, event := range events {
		eventData := map[string]interface{}{
			"_id":                event.ID,
			"name":               event.Name,
			"description":        event.Description,
			"points":             event.Points,
			"created_at":         event.CreatedAt,
			"is_claimed_by_user": userClaimedEventIDs[event.ID.Hex()],
			"is_claimed_by_any":  allClaimedEventIDs[event.ID.Hex()],
		}
		eventsWithStatus = append(eventsWithStatus, eventData)
	}

	respn.Status = "Success"
	respn.Response = "Data event berhasil diambil"
	respn.Data = eventsWithStatus
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ClaimEvent untuk claim event oleh user
func ClaimEvent(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var claimReq model.EventClaimRequest
	err = json.NewDecoder(req.Body).Decode(&claimReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert event ID to ObjectID
	eventObjectID, err := primitive.ObjectIDFromHex(claimReq.EventID)
	if err != nil {
		respn.Status = "Error : Event ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah event exists dan aktif
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":      eventObjectID,
		"isactive": true,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan atau tidak aktif"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah ada user lain yang sudah claim event ini dan masih aktif
	existingActiveClaim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"eventid": eventObjectID,
		"status":  primitive.M{"$in": []string{"claimed", "submitted"}},
	})
	if err == nil {
		respn.Status = "Error : Event sudah di-claim oleh user lain"
		respn.Response = "Event ini sudah di-claim oleh user lain dan sedang dalam proses"
		respn.Data = map[string]interface{}{
			"claimed_by": existingActiveClaim.UserPhone,
			"claimed_at": existingActiveClaim.ClaimedAt,
			"deadline":   existingActiveClaim.Deadline,
		}
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user ini sudah pernah claim event ini
	userExistingClaim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"eventid":   eventObjectID,
		"userphone": payload.Id,
		"status":    primitive.M{"$in": []string{"claimed", "submitted", "approved"}},
	})
	if err == nil {
		respn.Status = "Error : Anda sudah claim event ini"
		respn.Response = "Anda sudah claim event ini sebelumnya"
		respn.Data = userExistingClaim
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validasi deadline seconds
	deadlineSeconds := claimReq.DeadlineSeconds
	if deadlineSeconds <= 0 || deadlineSeconds > 3600 {
		respn.Status = "Error : Deadline tidak valid"
		respn.Response = "Deadline harus antara 1-3600 detik"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Buat claim baru dengan deadline sesuai input user
	now := time.Now()
	deadline := now.Add(time.Duration(deadlineSeconds) * time.Second)

	eventClaim := model.EventClaim{
		EventID:    eventObjectID,
		UserPhone:  payload.Id,
		ClaimedAt:  now,
		Deadline:   deadline,
		Status:     "claimed",
		IsApproved: false,
		Points:     event.Points,
	}

	// Simpan claim ke database
	claimID, err := atdb.InsertOneDoc(config.Mongoconn, "eventclaims", eventClaim)
	if err != nil {
		respn.Status = "Error : Gagal claim event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = "Event berhasil di-claim"
	respn.Data = map[string]interface{}{
		"claim_id":         claimID,
		"event":            event,
		"deadline":         deadline,
		"deadline_seconds": deadlineSeconds,
		"message":          fmt.Sprintf("Anda memiliki waktu %d detik (hingga %s) untuk menyelesaikan tugas", deadlineSeconds, deadline.Format("2006-01-02 15:04:05")),
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// SubmitEventTask untuk submit link tugas event
func SubmitEventTask(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var submitReq model.EventSubmitRequest
	err = json.NewDecoder(req.Body).Decode(&submitReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjectID, err := primitive.ObjectIDFromHex(submitReq.ClaimID)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah claim exists dan milik user ini
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":       claimObjectID,
		"userphone": payload.Id,
		"status":    "claimed",
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan atau sudah disubmit"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah masih dalam deadline
	if time.Now().After(claim.Deadline) {
		// Update status ke expired
		_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjectID}, model.EventClaim{
			ID:          claim.ID,
			EventID:     claim.EventID,
			UserPhone:   claim.UserPhone,
			ClaimedAt:   claim.ClaimedAt,
			Deadline:    claim.Deadline,
			Status:      "expired",
			TaskLink:    claim.TaskLink,
			SubmittedAt: claim.SubmittedAt,
			ApprovedAt:  claim.ApprovedAt,
			ApprovedBy:  claim.ApprovedBy,
			IsApproved:  claim.IsApproved, // Preserve existing value
			Points:      claim.Points,     // Preserve existing points
		})

		respn.Status = "Error : Deadline sudah terlewat"
		respn.Response = "Waktu untuk menyelesaikan tugas sudah habis"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Validasi task link
	if submitReq.TaskLink == "" {
		respn.Status = "Error : Link tugas harus diisi"
		respn.Response = "Silakan masukkan link tugas yang sudah dikerjakan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update claim dengan task link dan status submitted
	updatedClaim := model.EventClaim{
		ID:          claim.ID,
		EventID:     claim.EventID,
		UserPhone:   claim.UserPhone,
		ClaimedAt:   claim.ClaimedAt,
		Deadline:    claim.Deadline,
		Status:      "submitted",
		TaskLink:    submitReq.TaskLink,
		SubmittedAt: time.Now(),
		ApprovedAt:  claim.ApprovedAt,
		ApprovedBy:  claim.ApprovedBy,
		IsApproved:  claim.IsApproved, // Preserve existing value
		Points:      claim.Points,     // Preserve existing points
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjectID}, updatedClaim)
	if err != nil {
		respn.Status = "Error : Gagal submit tugas"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get user data untuk notifikasi
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event data
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Data event tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Send notification ke owner dengan link approval
	approvalLink := fmt.Sprintf("https://www.do.my.id/event/#%s", claimObjectID.Hex())

	// Prepare notification message for WhatsApp
	message := fmt.Sprintf("🎯 *Event Task Submitted*\n\n"+
		"📋 Event: %s\n"+
		"👤 User: %s (%s)\n"+
		"📱 Phone: %s\n"+
		"🔗 Task Link: %s\n"+
		"✅ Approval Link: %s\n\n"+
		"Klik link approval untuk menyetujui tugas ini.",
		event.Name, docuser.Name, docuser.NPM, docuser.PhoneNumber, submitReq.TaskLink, approvalLink)

	// Send to owner numbers
	ownerNumbers := []string{"6285312924192", "6282117252716"}
	for _, ownerNum := range ownerNumbers {
		// Send WhatsApp message to owner
		dt := &whatsauth.TextMessage{
			To:       ownerNum,
			IsGroup:  false,
			Messages: message,
		}

		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to send WhatsApp to %s: %v, info: %s\n", ownerNum, err, resp.Info)
		} else {
			fmt.Printf("WhatsApp sent successfully to %s\n", ownerNum)
		}
	}

	respn.Status = "Success"
	respn.Response = "Tugas berhasil disubmit dan menunggu approval dari owner"
	respn.Data = map[string]interface{}{
		"claim":         updatedClaim,
		"event":         event,
		"approval_link": approvalLink,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ApproveEventClaim untuk approve claim event (khusus owner)
func ApproveEventClaim(respw http.ResponseWriter, req *http.Request) {
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
		respn.Status = "Error : Akses Ditolak"
		respn.Response = "Hanya owner yang dapat approve event"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var approveReq model.EventApproveRequest
	err = json.NewDecoder(req.Body).Decode(&approveReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjectID, err := primitive.ObjectIDFromHex(approveReq.ClaimID)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim data
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":    claimObjectID,
		"status": "submitted",
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan atau belum disubmit"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event data
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": claim.UserPhone})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Update claim status to approved
	updatedClaim := model.EventClaim{
		ID:          claim.ID,
		EventID:     claim.EventID,
		UserPhone:   claim.UserPhone,
		ClaimedAt:   claim.ClaimedAt,
		Deadline:    claim.Deadline,
		Status:      "approved",
		TaskLink:    claim.TaskLink,
		SubmittedAt: claim.SubmittedAt,
		ApprovedAt:  time.Now(),
		ApprovedBy:  payload.Id,
		IsApproved:  true,
		Points:      event.Points,
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjectID}, updatedClaim)
	if err != nil {
		respn.Status = "Error : Gagal approve claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Add points to EventUserPoint collection
	eventUserPoint := model.EventUserPoint{
		Name:      user.Name,
		Phone:     user.PhoneNumber,
		NPM:       user.NPM,
		EventID:   event.ID,
		EventName: event.Name,
		Points:    event.Points,
		ClaimID:   claim.ID,
		CreatedAt: time.Now(),
	}

	// Insert to eventuserpoint collection
	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventuserpoint", eventUserPoint)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan poin user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Also add to bimbingan for backward compatibility
	// Get next bimbingan number
	allBimbingan, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": claim.UserPhone})
	if err != nil {
		allBimbingan = []model.ActivityScore{}
	}
	nextBimbinganKe := len(allBimbingan) + 1

	// Create asesor data
	asesor := model.Userdomyikado{
		Name:        "System Event",
		PhoneNumber: payload.Id,
	}

	// Create bimbingan entry for points
	bimbingan := model.ActivityScore{
		BimbinganKe: nextBimbinganKe,
		Approved:    true,
		Username:    user.Name,
		PhoneNumber: user.PhoneNumber,
		Asesor:      asesor,
		CreatedAt:   time.Now(),
		// Set all activity scores to 0 except for the event points
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
		TotalScore:      event.Points, // Add event points to total score
		Komentar:        fmt.Sprintf("Bonus Points dari Event: %s (%d points)", event.Name, event.Points),
		Validasi:        5, // Rating 5 untuk event
	}

	// Insert bimbingan
	_, err = atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		respn.Status = "Error : Gagal menambah points ke bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Event claim berhasil di-approve. User %s mendapat %d points", user.Name, event.Points)
	respn.Data = map[string]interface{}{
		"claim":  updatedClaim,
		"event":  event,
		"user":   user,
		"points": event.Points,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetUserEventClaims untuk mendapatkan event claims user
func GetUserEventClaims(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get user's claims (exclude approved ones as they should disappear from user view)
	claims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"userphone": payload.Id,
		"status":    primitive.M{"$in": []string{"claimed", "submitted"}},
	})
	if err != nil {
		claims = []model.EventClaim{}
	}

	// Get event details for each claim
	var claimsWithEvents []map[string]interface{}
	for _, claim := range claims {
		event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
		if err != nil {
			continue
		}

		// Check if expired
		isExpired := time.Now().After(claim.Deadline) && claim.Status == "claimed"

		claimData := map[string]interface{}{
			"claim_id":     claim.ID.Hex(),
			"event":        event,
			"claimed_at":   claim.ClaimedAt,
			"deadline":     claim.Deadline,
			"status":       claim.Status,
			"task_link":    claim.TaskLink,
			"submitted_at": claim.SubmittedAt,
			"is_expired":   isExpired,
		}
		claimsWithEvents = append(claimsWithEvents, claimData)
	}

	respn.Status = "Success"
	respn.Response = "Data claims berhasil diambil"
	respn.Data = claimsWithEvents
	at.WriteJSON(respw, http.StatusOK, respn)
}

// CheckExpiredClaims untuk mengecek dan update claims yang expired
func CheckExpiredClaims(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get all claimed events that are past deadline
	expiredClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"status":   "claimed",
		"deadline": primitive.M{"$lt": time.Now()},
	})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data expired claims"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	updatedCount := 0
	for _, claim := range expiredClaims {
		// Update status to expired
		updatedClaim := model.EventClaim{
			ID:          claim.ID,
			EventID:     claim.EventID,
			UserPhone:   claim.UserPhone,
			ClaimedAt:   claim.ClaimedAt,
			Deadline:    claim.Deadline,
			Status:      "expired",
			TaskLink:    claim.TaskLink,
			SubmittedAt: claim.SubmittedAt,
			ApprovedAt:  claim.ApprovedAt,
			ApprovedBy:  claim.ApprovedBy,
			IsApproved:  claim.IsApproved, // Preserve existing value
			Points:      claim.Points,     // Preserve existing points
		}

		_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claim.ID}, updatedClaim)
		if err == nil {
			updatedCount++
		}
	}

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Updated %d expired claims", updatedCount)
	respn.Data = map[string]interface{}{
		"expired_count": len(expiredClaims),
		"updated_count": updatedCount,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetEventApprovalData untuk mendapatkan data approval berdasarkan claim ID dari URL hash
func GetEventApprovalData(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get claim ID from URL parameter
	claimId := at.GetParam(req)
	if claimId == "" {
		respn.Status = "Error : Claim ID tidak ditemukan"
		respn.Response = "Claim ID harus disediakan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjectID, err := primitive.ObjectIDFromHex(claimId)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim data
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id": claimObjectID,
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event data
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id": claim.EventID,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{
		"phonenumber": claim.UserPhone,
	})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Prepare response data dengan format yang sesuai untuk frontend
	responseData := map[string]interface{}{
		"_id":         claim.ID.Hex(),
		"eventname":   event.Name,
		"description": event.Description,
		"points":      event.Points,
		"username":    user.Name,
		"npm":         user.NPM,
		"phonenumber": user.PhoneNumber,
		"email":       user.Email,
		"tasklink":    claim.TaskLink,
		"submittedat": claim.SubmittedAt,
		"deadline":    claim.Deadline,
		"status":      claim.Status,
		"isapproved":  claim.IsApproved,
		"approved":    claim.IsApproved, // untuk kompatibilitas dengan pola kambing
	}

	at.WriteJSON(respw, http.StatusOK, responseData)
}

// FixEventClaimPoints untuk memperbaiki points di eventclaims yang 0
func FixEventClaimPoints(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get all claims dengan points 0 atau tidak ada
	claims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"$or": []primitive.M{
			{"points": 0},
			{"points": primitive.M{"$exists": false}},
		},
	})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data claims"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	updatedCount := 0
	for _, claim := range claims {
		// Get event data untuk ambil points yang benar
		event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
			"_id": claim.EventID,
		})
		if err != nil {
			continue // Skip jika event tidak ditemukan
		}

		// Update claim dengan points yang benar
		claim.Points = event.Points
		if claim.IsApproved == false && claim.Status != "approved" {
			claim.IsApproved = false // Ensure default value
		}

		_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claim.ID}, claim)
		if err == nil {
			updatedCount++
		}
	}

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Fixed %d claims with correct points", updatedCount)
	respn.Data = map[string]interface{}{
		"total_claims":   len(claims),
		"updated_claims": updatedCount,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// PostEventApproval untuk approve claim event (mengikuti pola bimbingan POST)
func PostEventApproval(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get claim ID dari URL parameter (seperti bimbingan)
	claimId := at.GetParam(req)
	if claimId == "" {
		respn.Status = "Error : Claim ID tidak ditemukan"
		respn.Response = "Claim ID harus disediakan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjectID, err := primitive.ObjectIDFromHex(claimId)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Parse request body untuk approval data (seperti rating di bimbingan)
	var approvalData struct {
		Approved bool   `json:"approved"`
		Komentar string `json:"komentar,omitempty"`
	}
	err = json.NewDecoder(req.Body).Decode(&approvalData)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim data
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id": claimObjectID,
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah sudah approved
	if claim.IsApproved {
		respn.Status = "Error : Sudah di-approve"
		respn.Response = "Claim ini sudah di-approve sebelumnya"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah status submitted
	if claim.Status != "submitted" {
		respn.Status = "Error : Status tidak valid"
		respn.Response = "Claim harus dalam status submitted untuk di-approve"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get event data
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id": claim.EventID,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{
		"phonenumber": claim.UserPhone,
	})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Update claim dengan approval
	claim.IsApproved = approvalData.Approved
	claim.Status = "approved"
	claim.ApprovedAt = time.Now()
	claim.ApprovedBy = "owner" // Bisa disesuaikan dengan token jika perlu

	// Update claim di database
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjectID}, claim)
	if err != nil {
		respn.Status = "Error : Gagal update claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Add points to EventUserPoint collection
	eventUserPoint := model.EventUserPoint{
		Name:      user.Name,
		Phone:     user.PhoneNumber,
		NPM:       user.NPM,
		EventID:   event.ID,
		EventName: event.Name,
		Points:    event.Points,
		ClaimID:   claim.ID,
		CreatedAt: time.Now(),
	}

	// Insert to eventuserpoint collection
	_, err = atdb.InsertOneDoc(config.Mongoconn, "eventuserpoint", eventUserPoint)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan poin user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Event berhasil di-approve. User %s mendapat %d poin.", user.Name, event.Points)
	respn.Data = map[string]interface{}{
		"approved":    true,
		"user":        user.Name,
		"event":       event.Name,
		"points":      event.Points,
		"approved_at": claim.ApprovedAt,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetEventClaimDetails untuk mendapatkan detail claim berdasarkan ID
func GetEventClaimDetails(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get claim ID from URL parameter
	claimId := at.GetParam(req)
	if claimId == "" {
		respn.Status = "Error : Claim ID tidak ditemukan"
		respn.Response = "Claim ID harus disediakan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjectID, err := primitive.ObjectIDFromHex(claimId)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim data
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id": claimObjectID,
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event data
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id": claim.EventID,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{
		"phonenumber": claim.UserPhone,
	})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"claim": claim,
		"event": event,
		"user":  user,
	}

	respn.Status = "Success"
	respn.Response = "Detail claim berhasil diambil"
	respn.Data = responseData
	at.WriteJSON(respw, http.StatusOK, respn)
}
