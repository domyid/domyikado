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
		// Initialize claim/submission/approval fields
		ClaimedBy:   "",
		IsSubmitted: false,
		IsApproved:  false,
		TimerSec:    0,
		Validasi:    0,
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

	// Cleanup expired claims first
	cleanupExpiredEvents()

	// Get active events (not approved yet)
	events, err := atdb.GetAllDoc[[]model.Event](config.Mongoconn, "events", primitive.M{
		"isactive":   true,
		"isapproved": false, // Only show events that haven't been approved
	})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Prepare response with availability info
	var eventList []map[string]interface{}
	for _, event := range events {
		// Determine availability
		isAvailable := true
		var userClaim map[string]interface{}

		// Check if event is claimed
		if event.ClaimedBy != "" {
			isAvailable = false

			// If claimed by current user, add claim info
			if event.ClaimedBy == payload.Id {
				userClaim = map[string]interface{}{
					"claimed_at":   event.ClaimedAt,
					"expires_at":   event.ExpiresAt,
					"timer_sec":    event.TimerSec,
					"is_submitted": event.IsSubmitted,
					"task_link":    event.TaskLink,
					"submitted_at": event.SubmittedAt,
					"claimed_user": event.ClaimedUser,
				}
			}
		}

		eventInfo := map[string]interface{}{
			"_id":          event.ID.Hex(),
			"name":         event.Name,
			"description":  event.Description,
			"points":       event.Points,
			"created_by":   event.CreatedBy,
			"created_at":   event.CreatedAt,
			"is_available": isAvailable,
		}

		// Add user claim info if exists
		if userClaim != nil {
			eventInfo["user_claim"] = userClaim
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

	// Cek apakah event exists dan available
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":        eventObjID,
		"isactive":   true,
		"isapproved": false,
		"claimedby":  "", // Event belum di-claim
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Event tidak ditemukan, tidak aktif, atau sudah di-claim"
		} else {
			respn.Status = "Error : Gagal mengambil data event"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user info untuk menyimpan data lengkap
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Update event dengan claim info
	now := time.Now()
	expiresAt := now.Add(time.Duration(claimReq.TimerSec) * time.Second)

	updateData := primitive.M{
		"claimedby":   payload.Id,
		"claimedat":   now,
		"expiresat":   expiresAt,
		"timersec":    claimReq.TimerSec,
		"issubmitted": false,
		"tasklink":    "",
		// Tambah data user untuk referensi
		"claimeduser": map[string]interface{}{
			"name":        user.Name,
			"phonenumber": user.PhoneNumber,
			"email":       user.Email,
		},
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "events", primitive.M{"_id": eventObjID}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"event_id":      event.ID.Hex(),
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
	if submitReq.EventID == "" {
		respn.Status = "Error : Event ID tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	if submitReq.TaskLink == "" {
		respn.Status = "Error : Link tugas tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert event ID to ObjectID
	eventObjID, err := primitive.ObjectIDFromHex(submitReq.EventID)
	if err != nil {
		respn.Status = "Error : Event ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah event exists dan di-claim oleh user
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":         eventObjID,
		"claimedby":   payload.Id,
		"isactive":    true,
		"isapproved":  false,
		"issubmitted": false,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Event tidak ditemukan, bukan milik Anda, atau sudah di-submit"
		} else {
			respn.Status = "Error : Gagal mengambil data event"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah claim masih aktif (belum expired)
	if time.Now().After(event.ExpiresAt) {
		respn.Status = "Error : Claim sudah expired"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update event dengan task submission
	updateData := primitive.M{
		"tasklink":    submitReq.TaskLink,
		"submittedat": time.Now(),
		"issubmitted": true, // Mark as submitted
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "events", primitive.M{"_id": eventObjID}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update event"
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

	// Send WhatsApp notification to event owner
	message := fmt.Sprintf("Permintaan approve event nanti jika di approve mahasiswa ini dapet point\nMahasiswa : %s\nhttps://www.do.my.id/event/#%s", user.Name, event.ID.Hex())
	dt := &whatsauth.TextMessage{
		To:       event.CreatedBy,
		IsGroup:  false,
		Messages: message,
	}

	_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		log.Printf("Warning: Failed to send WhatsApp notification: %v", err)
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"message":    "Tugas berhasil di-submit, menunggu approval dari owner",
		"event_id":   event.ID.Hex(),
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

	// Convert event ID to ObjectID
	eventObjID, err := primitive.ObjectIDFromHex(approvalReq.EventID)
	if err != nil {
		respn.Status = "Error : Event ID tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get event info dan cek ownership
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":         eventObjID,
		"issubmitted": true,
		"isapproved":  false,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respn.Status = "Error : Event tidak ditemukan atau belum di-submit"
		} else {
			respn.Status = "Error : Gagal mengambil data event"
			respn.Response = err.Error()
		}
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Cek apakah user adalah owner event
	if event.CreatedBy != payload.Id {
		respn.Status = "Error : Anda bukan owner event ini"
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Update event dengan approval status
	updateData := primitive.M{
		"isapproved": approvalReq.IsApproved,
		"approvedat": time.Now(),
		"approvedby": payload.Id,
		"komentar":   approvalReq.Komentar,
		"validasi":   approvalReq.Validasi,
	}

	if approvalReq.IsApproved {
		// Approve: Buat entry di eventuserpoint dan berikan points
		err = createEventUserPointFromEvent(event, payload.Id)
		if err != nil {
			respn.Status = "Error : Gagal menyimpan point user"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}

		// Deactivate event setelah di-approve
		updateData["isactive"] = false
	} else {
		// If rejected, reset event untuk available lagi
		updateData["claimedby"] = ""
		updateData["claimedat"] = time.Time{}
		updateData["expiresat"] = time.Time{}
		updateData["timersec"] = 0
		updateData["tasklink"] = ""
		updateData["submittedat"] = time.Time{}
		updateData["issubmitted"] = false
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "events", primitive.M{"_id": eventObjID}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update event"
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

// Helper function untuk menyimpan point user dari event approval
func createEventUserPointFromEvent(event model.Event, approverPhone string) error {
	// Create event user point entry
	eventUserPoint := model.EventUserPoint{
		UserPhone:  event.ClaimedBy,
		EventID:    event.ID,
		EventName:  event.Name,
		Points:     event.Points,
		TaskLink:   event.TaskLink,
		ApprovedBy: approverPhone,
		ApprovedAt: time.Now(),
		CreatedAt:  time.Now(),
	}

	// Insert ke collection eventuserpoint
	pointID, err := atdb.InsertOneDoc(config.Mongoconn, "eventuserpoint", eventUserPoint)
	if err != nil {
		return err
	}

	log.Printf("Event user point created: %s, User: %s, Points: %d", pointID.Hex(), event.ClaimedBy, event.Points)
	return nil
}

// GetEventClaimsByPhoneNumber - Mendapatkan event claims berdasarkan phone number untuk approval
func GetEventClaimsByPhoneNumber(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get phone number from query parameter
	phoneNumber := req.URL.Query().Get("phonenumber")
	if phoneNumber == "" {
		respn.Status = "Error : Phone number tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Log who is accessing this endpoint
	log.Printf("GetEventClaimsByPhoneNumber accessed by: %s, searching for: %s", payload.Id, phoneNumber)

	// Get user's submitted event claims (completed but not approved yet)
	// Sekarang kita cek dari event-approval collection untuk yang belum approved
	approvals, err := atdb.GetAllDoc[[]model.EventApproval](config.Mongoconn, "event-approval", primitive.M{
		"userphone": phoneNumber,
		"approved":  false,
	})
	if err != nil {
		approvals = []model.EventApproval{} // If error, return empty array
	}

	// Convert approvals to claim details format
	var claimDetails []map[string]interface{}
	for _, approval := range approvals {
		claimDetail := map[string]interface{}{
			"claim_id":          approval.ClaimID.Hex(),
			"event_id":          approval.EventID.Hex(),
			"event_name":        approval.EventName,
			"event_description": "Event Task Submission",
			"event_points":      approval.EventPoints,
			"user_name":         approval.UserName,
			"user_phone":        approval.UserPhone,
			"user_email":        approval.UserEmail,
			"task_link":         approval.TaskLink,
			"claimed_at":        approval.CreatedAt,
			"submitted_at":      approval.CreatedAt,
			"timer_sec":         0,
			"expires_at":        approval.CreatedAt,
			"is_completed":      true,
		}
		claimDetails = append(claimDetails, claimDetail)
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"phone_number": phoneNumber,
		"claims_count": len(claimDetails),
		"claims":       claimDetails,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetEventClaimDetails - Mendapatkan detail claim berdasarkan claim ID untuk approval individual
func GetEventClaimDetails(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get claim ID from URL path
	claimIDStr := req.URL.Path[len("/api/event/claim/"):]
	if claimIDStr == "" {
		respn.Status = "Error : Claim ID tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Convert to ObjectID
	claimID, err := primitive.ObjectIDFromHex(claimIDStr)
	if err != nil {
		respn.Status = "Error : Claim ID tidak valid"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Log access
	log.Printf("GetEventClaimDetails accessed by: %s, claim ID: %s", payload.Id, claimIDStr)

	// Get claim details
	claim, err := atdb.GetOneDoc[model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"_id":         claimID,
		"iscompleted": true,
	})
	if err != nil {
		respn.Status = "Error : Claim tidak ditemukan atau sudah diproses"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get event info
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{"_id": claim.EventID})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Get user info
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": claim.UserPhone})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	claimDetail := map[string]interface{}{
		"claim_id":          claim.ID.Hex(),
		"event_id":          event.ID.Hex(),
		"event_name":        event.Name,
		"event_description": event.Description,
		"event_points":      event.Points,
		"user_name":         user.Name,
		"user_phone":        user.PhoneNumber,
		"user_email":        user.Email,
		"task_link":         claim.TaskLink,
		"claimed_at":        claim.ClaimedAt,
		"submitted_at":      claim.SubmittedAt,
		"timer_sec":         claim.TimerSec,
		"expires_at":        claim.ExpiresAt,
		"is_completed":      claim.IsCompleted,
	}

	respn.Status = "Success"
	respn.Data = claimDetail
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetEventApprovalById - Mendapatkan data event untuk approval berdasarkan ID
func GetEventApprovalById(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	id := at.GetParam(req)
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get event yang sudah di-submit untuk approval
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":         objectId,
		"issubmitted": true,
		"isapproved":  false,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan atau belum di-submit"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get user info
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": event.ClaimedBy})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Prepare response data
	approvalData := map[string]interface{}{
		"_id":         event.ID.Hex(),
		"eventname":   event.Name,
		"eventpoints": event.Points,
		"username":    user.Name,
		"userphone":   user.PhoneNumber,
		"useremail":   user.Email,
		"tasklink":    event.TaskLink,
		"createdat":   event.SubmittedAt,
		"approved":    event.IsApproved,
		"komentar":    event.Komentar,
		"validasi":    event.Validasi,
	}

	at.WriteJSON(respw, http.StatusOK, approvalData)
}

// ReplaceEventApproval - Approve/reject event task (menggunakan event collection)
func ReplaceEventApproval(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	id := at.GetParam(req)
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	var approval model.EventApprovalRequest
	err = json.NewDecoder(req.Body).Decode(&approval)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Get event yang akan di-approve
	event, err := atdb.GetOneDoc[model.Event](config.Mongoconn, "events", primitive.M{
		"_id":         objectId,
		"issubmitted": true,
		"isapproved":  false,
	})
	if err != nil {
		respn.Status = "Error : Event tidak ditemukan atau sudah diproses"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Update event dengan approval data
	updateData := primitive.M{
		"isapproved": approval.IsApproved,
		"approvedat": time.Now(),
		"komentar":   approval.Komentar,
		"validasi":   approval.Validasi,
	}

	if approval.IsApproved {
		// Jika approved, buat EventUserPoint dan deactivate event
		err = createEventUserPointFromEvent(event, "system") // Use system as approver for now
		if err != nil {
			log.Printf("Warning: Failed to create EventUserPoint: %v", err)
		}
		updateData["isactive"] = false
	} else {
		// Jika rejected, reset event untuk available lagi
		updateData["claimedby"] = ""
		updateData["claimedat"] = time.Time{}
		updateData["expiresat"] = time.Time{}
		updateData["timersec"] = 0
		updateData["tasklink"] = ""
		updateData["submittedat"] = time.Time{}
		updateData["issubmitted"] = false
		updateData["claimeduser"] = nil
	}

	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "events", primitive.M{"_id": objectId}, updateData)
	if err != nil {
		respn.Status = "Error : Gagal update event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}

	// Send WhatsApp notification to student
	var message string
	if approval.IsApproved {
		message = fmt.Sprintf("Event Task Kamu *TELAH DI APPROVE*\nEvent : %s\nPoints : %d\nKomentar : %s\nSelamat! Points sudah ditambahkan ke akun Anda.",
			event.Name, event.Points, approval.Komentar)
	} else {
		message = fmt.Sprintf("Event Task Kamu *BELUM DI APPROVE*\nEvent : %s\nKomentar : %s\nSilahkan perbaiki dan submit ulang.",
			event.Name, approval.Komentar)
	}

	dt := &whatsauth.TextMessage{
		To:       event.ClaimedBy,
		IsGroup:  false,
		Messages: message,
	}

	_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		log.Printf("Warning: Failed to send WhatsApp notification: %v", err)
	}

	// Prepare response
	responseData := map[string]interface{}{
		"_id":         event.ID.Hex(),
		"eventname":   event.Name,
		"eventpoints": event.Points,
		"approved":    approval.IsApproved,
		"komentar":    approval.Komentar,
		"validasi":    approval.Validasi,
		"approvedat":  time.Now(),
	}

	at.WriteJSON(respw, http.StatusOK, responseData)
}

// GetUserEventPoints - Mendapatkan total point user dari event
func GetUserEventPoints(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Get all user event points
	userPoints, err := atdb.GetAllDoc[[]model.EventUserPoint](config.Mongoconn, "eventuserpoint", primitive.M{
		"userphone": payload.Id,
	})
	if err != nil {
		userPoints = []model.EventUserPoint{} // If error, return empty array
	}

	// Calculate total points
	totalPoints := 0
	for _, point := range userPoints {
		totalPoints += point.Points
	}

	// Prepare response
	responseData := map[string]interface{}{
		"user_phone":    payload.Id,
		"total_points":  totalPoints,
		"event_count":   len(userPoints),
		"event_history": userPoints,
	}

	respn.Status = "Success"
	respn.Data = responseData
	at.WriteJSON(respw, http.StatusOK, respn)
}

// GetAllUserEventPoints - Admin endpoint untuk melihat semua user points
func GetAllUserEventPoints(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// TODO: Add admin role check here if needed
	_ = payload // For now, any authenticated user can access

	// Get all event points
	allPoints, err := atdb.GetAllDoc[[]model.EventUserPoint](config.Mongoconn, "eventuserpoint", primitive.M{})
	if err != nil {
		respn.Status = "Error : Gagal mengambil data point"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Group by user phone
	userPointsMap := make(map[string][]model.EventUserPoint)
	userTotalMap := make(map[string]int)

	for _, point := range allPoints {
		userPointsMap[point.UserPhone] = append(userPointsMap[point.UserPhone], point)
		userTotalMap[point.UserPhone] += point.Points
	}

	// Prepare response
	var userSummaries []map[string]interface{}
	for userPhone, points := range userPointsMap {
		summary := map[string]interface{}{
			"user_phone":    userPhone,
			"total_points":  userTotalMap[userPhone],
			"event_count":   len(points),
			"event_history": points,
		}
		userSummaries = append(userSummaries, summary)
	}

	respn.Status = "Success"
	respn.Data = map[string]interface{}{
		"total_users": len(userSummaries),
		"users":       userSummaries,
	}
	at.WriteJSON(respw, http.StatusOK, respn)
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

// cleanupExpiredEvents - Cleanup expired events yang belum di-submit
func cleanupExpiredEvents() {
	// Reset expired events yang belum di-submit
	filter := primitive.M{
		"claimedby":   primitive.M{"$ne": ""},
		"issubmitted": false,
		"isapproved":  false,
		"expiresat":   primitive.M{"$lt": time.Now()},
	}

	update := primitive.M{
		"$set": primitive.M{
			"claimedby": "",
			"claimedat": time.Time{},
			"expiresat": time.Time{},
			"timersec":  0,
		},
	}

	result, err := config.Mongoconn.Collection("events").UpdateMany(context.TODO(), filter, update)
	if err != nil {
		log.Printf("Error cleaning up expired events: %v", err)
		return
	}

	if result.ModifiedCount > 0 {
		log.Printf("Cleaned up %d expired events", result.ModifiedCount)
	}
}
