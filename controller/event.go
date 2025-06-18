package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateEvent - Owner membuat event baru
func CreateEvent(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
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
	if eventReq.Name == "" {
		respn.Status = "Error : Nama event tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if eventReq.Points <= 0 {
		respn.Status = "Error : Points harus lebih dari 0"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah user adalah owner (bisa ditambahkan validasi owner di sini)
	_, err = atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
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
	respn.Data = map[string]interface{}{
		"event_id": eventID.Hex(),
		"message":  "Event berhasil dibuat",
		"event":    event,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetEvents - Mendapatkan list event yang aktif
func GetEvents(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get active events
	events, err := atdb.GetAllDoc[[]model.Event](config.Mongoconn, "events", primitive.M{"isactive": true})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Cleanup expired claims first
	cleanupExpiredClaims()

	// Get user's active claims to check which events are claimed
	userClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"userphone": payload.Id,
		"isactive":  true,
		"expiresat": primitive.M{"$gt": time.Now()}, // Only non-expired claims
	})
	if err != nil {
		userClaims = []model.EventClaim{} // If error, assume no claims
	}

	// Get all active claims to check availability
	allActiveClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"isactive":  true,
		"expiresat": primitive.M{"$gt": time.Now()},
	})
	if err != nil {
		allActiveClaims = []model.EventClaim{} // If error, assume no claims
	}

	// Create map for quick lookup
	claimedEvents := make(map[string]bool)
	userClaimedEvents := make(map[string]model.EventClaim)

	for _, claim := range allActiveClaims {
		claimedEvents[claim.EventID.Hex()] = true
	}

	for _, claim := range userClaims {
		userClaimedEvents[claim.EventID.Hex()] = claim
	}

	// Prepare response with availability info
	var eventList []map[string]interface{}
	for _, event := range events {
		eventInfo := map[string]interface{}{
			"_id":          event.ID.Hex(),
			"name":         event.Name,
			"description":  event.Description,
			"points":       event.Points,
			"created_by":   event.CreatedBy,
			"created_at":   event.CreatedAt,
			"is_available": !claimedEvents[event.ID.Hex()],
		}

		// Add user claim info if exists
		if userClaim, exists := userClaimedEvents[event.ID.Hex()]; exists {
			eventInfo["user_claim"] = map[string]interface{}{
				"claim_id":     userClaim.ID.Hex(),
				"claimed_at":   userClaim.ClaimedAt,
				"expires_at":   userClaim.ExpiresAt,
				"is_completed": userClaim.IsCompleted,
				"task_link":    userClaim.TaskLink,
				"submitted_at": userClaim.SubmittedAt,
				"is_approved":  userClaim.IsApproved,
			}
		}

		eventList = append(eventList, eventInfo)
	}

	respn.Status = "Success"
	respn.Data = eventList
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ClaimEvent - User claim event dengan timer
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

	// Validasi input
	if claimReq.EventID == "" {
		respn.Status = "Error : Event ID tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if claimReq.TimerSec <= 0 {
		respn.Status = "Error : Timer harus lebih dari 0 detik"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert event ID to ObjectID
	eventObjID, err := primitive.ObjectIDFromHex(claimReq.EventID)
	if err != nil {
		respn.Status = "Error : Event ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah event exists dan aktif
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":      eventObjID,
		"isactive": true,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Event tidak ditemukan atau tidak aktif"
		} else {
			respn.Status = "Error : Gagal mengambil data event"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah event sudah di-claim oleh user lain dan masih aktif
	existingClaim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"eventid":   eventObjID,
		"isactive":  true,
		"expiresat": primitive.M{"$gt": time.Now()},
	})
	if err == nil {
		if existingClaim.UserPhone != payload.Id {
			respn.Status = "Error : Event sudah di-claim oleh user lain"
			at.WriteJSON(respw, http.StatusConflict, respn)
			return
		} else {
			respn.Status = "Error : Anda sudah claim event ini"
			at.WriteJSON(respw, http.StatusConflict, respn)
			return
		}
	}

	// Buat claim baru
	now := time.Now()
	expiresAt := now.Add(time.Duration(claimReq.TimerSec) * time.Second)

	eventClaim := model.EventClaim{
		EventID:     eventObjID,
		UserPhone:   payload.Id,
		ClaimedAt:   now,
		ExpiresAt:   expiresAt,
		TimerSec:    claimReq.TimerSec,
		IsActive:    true,
		IsCompleted: false,
		IsApproved:  false,
	}

	// Simpan claim ke database
	claimID, err := atdb.InsertOneDoc(config.Mongoconn, "eventclaims", eventClaim)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"claim_id":      claimID.Hex(),
		"message":       "Event berhasil di-claim",
		"event_name":    event.Name,
		"expires_at":    expiresAt,
		"timer_minutes": claimReq.TimerSec / 60, // Convert to minutes for display
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// SubmitEventTask - User submit link tugas untuk approval
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

	// Validasi input
	if submitReq.ClaimID == "" {
		respn.Status = "Error : Claim ID tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if submitReq.TaskLink == "" {
		respn.Status = "Error : Link tugas tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjID, err := primitive.ObjectIDFromHex(submitReq.ClaimID)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah claim exists dan milik user
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":       claimObjID,
		"userphone": payload.Id,
		"isactive":  true,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Claim tidak ditemukan atau bukan milik Anda"
		} else {
			respn.Status = "Error : Gagal mengambil data claim"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah claim masih aktif (belum expired)
	if time.Now().After(claim.ExpiresAt) {
		respn.Status = "Error : Claim sudah expired"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah sudah di-submit sebelumnya
	if claim.IsCompleted {
		respn.Status = "Error : Tugas sudah di-submit sebelumnya"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update claim dengan task link
	updateData := primitive.M{
		"tasklink":    submitReq.TaskLink,
		"submittedat": time.Now(),
		"iscompleted": true,
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjID}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get event info untuk notifikasi
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get user info
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send WhatsApp notification to owner (akan diimplementasi di fungsi terpisah)
	go sendEventApprovalNotification(event.CreatedBy, user.Name, event.Name, submitReq.TaskLink, claimObjID.Hex())

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"message":    "Tugas berhasil di-submit, menunggu approval dari owner",
		"event_name": event.Name,
		"task_link":  submitReq.TaskLink,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// ApproveEventTask - Owner approve/reject task submission
func ApproveEventTask(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var approvalReq model.EventApprovalRequest
	err = json.NewDecoder(req.Body).Decode(&approvalReq)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjID, err := primitive.ObjectIDFromHex(approvalReq.ClaimID)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim info
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":         claimObjID,
		"iscompleted": true,
		"isapproved":  false,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Claim tidak ditemukan atau belum di-submit"
		} else {
			respn.Status = "Error : Gagal mengambil data claim"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event info dan cek ownership
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Cek apakah user adalah owner event
	if event.CreatedBy != payload.Id {
		respn.Status = "Error : Anda bukan owner event ini"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	if approvalReq.IsApproved {
		// Approve: Buat bimbingan entry dan berikan points
		err = createBimbinganFromEventApproval(claim, event, payload.Id)
		if err != nil {
			respn.Status = "Error : Gagal membuat bimbingan"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}

		// Deactivate event setelah di-approve
		_, err = atdb.ReplaceOneDoc(config.Mongoconn, "events", primitive.M{"_id": event.ID}, primitive.M{"isactive": false})
		if err != nil {
			respn.Status = "Error : Gagal deactivate event"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}
	}

	// Update claim status
	updateData := primitive.M{
		"isapproved": approvalReq.IsApproved,
		"approvedat": time.Now(),
		"approvedby": payload.Id,
		"isactive":   false, // Deactivate claim regardless of approval status
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claimObjID}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update claim"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	status := "rejected"
	if approvalReq.IsApproved {
		status = "approved"
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"message":    "Task " + status + " successfully",
		"event_name": event.Name,
		"points":     event.Points,
		"approved":   approvalReq.IsApproved,
		"redirect_url": fmt.Sprintf("https://www.do.my.id/event/approved.html?event=%s&points=%d&approved_at=%s&approved_by=%s",
			event.Name, event.Points, time.Now().Format("2006-01-02 15:04:05"), payload.Id),
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetClaimDetails - Mendapatkan detail claim untuk approval
func GetClaimDetails(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Get claim ID from URL parameter
	claimID := at.GetParam(req)
	if claimID == "" {
		respn.Status = "Error : Claim ID tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert claim ID to ObjectID
	claimObjID, err := primitive.ObjectIDFromHex(claimID)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get claim info
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":         claimObjID,
		"iscompleted": true,
		"isapproved":  false,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Claim tidak ditemukan atau sudah diproses"
		} else {
			respn.Status = "Error : Gagal mengambil data claim"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event info
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get user info
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": claim.UserPhone})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Prepare response
	claimDetails := map[string]interface{}{
		"claim_id":          claim.ID.Hex(),
		"user_name":         user.Name,
		"user_phone":        user.PhoneNumber,
		"user_email":        user.Email,
		"event_name":        event.Name,
		"event_description": event.Description,
		"event_points":      event.Points,
		"task_link":         claim.TaskLink,
		"claimed_at":        claim.ClaimedAt,
		"submitted_at":      claim.SubmittedAt,
		"timer_sec":         claim.TimerSec,
		"expires_at":        claim.ExpiresAt,
	}

	respn.Status = "Success"
	respn.Data = claimDetails
	at.WriteJSON(respw, http.StatusOK, respn)
}

// Helper function untuk membuat bimbingan dari event approval
func createBimbinganFromEventApproval(claim model.EventClaim, event model.Event, approverPhone string) error {
	// Get user info
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": claim.UserPhone})
	if err != nil {
		return err
	}

	// Get approver info (asesor)
	asesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": approverPhone})
	if err != nil {
		return err
	}

	// Get next bimbingan number
	lastBimbingan, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": claim.UserPhone})
	nextBimbinganKe := 1
	if err == nil {
		nextBimbinganKe = lastBimbingan.BimbinganKe + 1
	}

	// Create bimbingan entry dengan points dari event
	bimbingan := model.ActivityScore{
		BimbinganKe: nextBimbinganKe,
		Approved:    true,
		Username:    user.Name,
		PhoneNumber: user.PhoneNumber,
		Asesor:      asesor,
		CreatedAt:   time.Now(),
		// Set semua activity scores ke 0 kecuali yang sesuai dengan event points
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
		Sponsor:         event.Points, // Assign event points to Sponsor field
		BukuKatalog:     "",
		BukPed:          0,
		JurnalWeb:       claim.TaskLink,
		Jurnal:          0,
		TotalScore:      event.Points,
		Komentar:        "Event Task Completed: " + event.Name + " - Link: " + claim.TaskLink,
		Validasi:        5, // Rating 5 untuk event
	}

	// Insert bimbingan
	bimbinganID, err := atdb.InsertOneDoc(config.Mongoconn, "bimbingan", bimbingan)
	if err != nil {
		return err
	}

	// Update claim dengan bimbingan ID
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claim.ID}, primitive.M{"bimbinganid": bimbinganID})
	if err != nil {
		return err
	}

	return nil
}

// Helper function untuk send WhatsApp notification
func sendEventApprovalNotification(ownerPhone, userName, eventName, taskLink, claimID string) {
	// Format pesan WhatsApp sesuai dengan format bimbingan
	message := "*Permintaan Approve Event*\n" +
		"Mahasiswa : " + userName + "\n" +
		"Event : " + eventName + "\n" +
		"Task Link : " + taskLink + "\n" +
		"Approve di: " + "https://www.do.my.id/event/#" + claimID

	// Kirim WhatsApp menggunakan API yang sudah ada seperti di bimbingan
	go func() {
		err := sendWhatsAppMessageWithAPI(ownerPhone, message)
		if err != nil {
			// Log error tapi jangan stop proses utama
			fmt.Printf("Failed to send WhatsApp notification: %v\n", err)
		}
	}()
}

// Helper function untuk kirim WhatsApp message menggunakan API yang sama dengan bimbingan
func sendWhatsAppMessageWithAPI(phoneNumber, message string) error {
	// Import yang diperlukan sudah ada di atas
	// Gunakan API yang sama seperti di controller bimbingan
	dt := &whatsauth.TextMessage{
		To:       phoneNumber,
		IsGroup:  false,
		Messages: message,
	}

	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		return fmt.Errorf("WhatsApp API error: %v, response: %s", err, resp.Info)
	}

	return nil
}

// cleanupExpiredClaims - Cleanup expired claims yang belum di-submit
func cleanupExpiredClaims() {
	// Deactivate expired claims yang belum di-submit
	filter := primitive.M{
		"isactive":    true,
		"iscompleted": false,
		"expiresat":   primitive.M{"$lt": time.Now()},
	}

	update := primitive.M{
		"$set": primitive.M{
			"isactive": false,
		},
	}

	result, err := config.Mongoconn.Collection("eventclaims").UpdateMany(context.TODO(), filter, update)
	if err != nil {
		log.Printf("Error cleaning up expired claims: %v", err)
		return
	}

	if result.ModifiedCount > 0 {
		log.Printf("Cleaned up %d expired claims", result.ModifiedCount)
	}
}
