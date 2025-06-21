package controller

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

// sendNewEventNotificationToGroup sends notification to specific WhatsApp group when new event is created
func sendNewEventNotificationToGroup() {
	// Target group ID as specified
	targetGroupID := "120363298977628161"

	// Message content as specified
	message := "Hai..Hai..Hai.. Buat kalian yang masih butuh bimbingan tambahan atau merasa bimbingannya masih kurang, jangan khawatir karena kami akan memberikan kalian event tambahan untuk menambah bimbingan kalian yang tertinggal! Yuk, cek (https://www.do.my.id/dashboard/#proyek/bimbinganevent) Jangan sampai ketinggalan, ya!"

	// Prepare WhatsApp message
	dt := &whatsauth.TextMessage{
		To:       targetGroupID,
		IsGroup:  true,
		Messages: message,
	}

	// Send message asynchronously to not block the main request
	go func() {
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			fmt.Printf("Failed to send WhatsApp notification to group %s: %v, info: %s\n", targetGroupID, err, resp.Info)
		} else {
			fmt.Printf("WhatsApp notification sent successfully to group %s\n", targetGroupID)
		}
	}()
}

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
	allowedNumbers := []string{"6285312924192", "6282117252716", "6285179935117", "6285759790334"}
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
	if eventReq.Name == "" || eventReq.Description == "" || eventReq.Points <= 0 || eventReq.DeadlineSeconds <= 0 {
		respn.Status = "Error : Data tidak lengkap"
		respn.Response = "Nama, deskripsi, poin, dan deadline harus diisi dengan benar"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// No maximum deadline validation - owner can set any deadline they want

	// Buat event baru
	event := model.Event{
		Name:            eventReq.Name,
		Description:     eventReq.Description,
		Points:          eventReq.Points,
		DeadlineSeconds: eventReq.DeadlineSeconds,
		CreatedBy:       payload.Id,
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// Simpan ke database
	eventID, err := atdb.InsertOneDoc(config.Mongoconn, "events", event)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "🎯 **New Event Created!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "New Event Created",
				Description: "A new event has been created by owner",
				Color:       5763719, // Green color
				Fields: []DiscordEmbedeventField{
					{Name: "📋 Event Name", Value: event.Name, Inline: true},
					{Name: "🎯 Points", Value: fmt.Sprintf("%d", event.Points), Inline: true},
					{Name: "⏰ Deadline", Value: fmt.Sprintf("%d seconds", event.DeadlineSeconds), Inline: true},
					{Name: "👤 Created By", Value: payload.Id, Inline: true},
					{Name: "🆔 Event ID", Value: fmt.Sprintf("%v", eventID), Inline: true},
					{Name: "📝 Description", Value: event.Description, Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

	// Send WhatsApp notification to group
	sendNewEventNotificationToGroup()

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
	// Include all statuses including approved to completely hide claimed events
	userClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"userphone": payload.Id,
		"status":    primitive.M{"$in": []string{"claimed", "submitted", "approved"}},
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

	// Create map of claimed event IDs by this user (untuk exclude dari list)
	userClaimedEventIDs := make(map[string]bool)
	for _, claim := range userClaims {
		userClaimedEventIDs[claim.EventID.Hex()] = true
	}

	// Create map of claimed event IDs by any user (untuk show status)
	allClaimedEventIDs := make(map[string]bool)
	for _, claim := range allActiveClaims {
		allClaimedEventIDs[claim.EventID.Hex()] = true
	}

	// Filter events - exclude events yang sudah di-claim user ini
	var eventsWithStatus []map[string]interface{}
	for _, event := range events {
		// Skip event jika user sudah pernah claim
		if userClaimedEventIDs[event.ID.Hex()] {
			continue // Jangan tampilkan event yang sudah di-claim user
		}

		eventData := map[string]interface{}{
			"_id":                event.ID,
			"name":               event.Name,
			"description":        event.Description,
			"points":             event.Points,
			"deadline_seconds":   event.DeadlineSeconds,
			"created_at":         event.CreatedAt,
			"is_claimed_by_user": false, // Selalu false karena sudah di-filter
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

	// Gunakan deadline seconds dari event (yang sudah ditentukan owner)
	deadlineSeconds := event.DeadlineSeconds

	// Buat claim baru dengan deadline sesuai setting event
	now := time.Now()
	deadline := now.Add(time.Duration(deadlineSeconds) * time.Second)

	eventClaim := model.EventClaim{
		EventID:    eventObjectID,
		UserPhone:  payload.Id,
		ClaimedAt:  now,
		Deadline:   deadline,
		Status:     "claimed",
		IsApproved: false,
	}

	// Simpan claim ke database
	claimID, err := atdb.InsertOneDoc(config.Mongoconn, "eventclaims", eventClaim)
	if err != nil {
		respn.Status = "Error : Gagal claim event"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Get user data for Discord notification
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		// If user not found, use phone number only
		docuser.Name = "Unknown User"
		docuser.NPM = "Unknown"
		docuser.PhoneNumber = payload.Id
	}

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "📋 **Event Claimed!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "Event Claimed",
				Description: "A user has claimed an event",
				Color:       39423, // Blue color
				Fields: []DiscordEmbedeventField{
					{Name: "📋 Event Name", Value: event.Name, Inline: true},
					{Name: "👤 User Name", Value: docuser.Name, Inline: true},
					{Name: "🎓 NPM", Value: docuser.NPM, Inline: true},
					{Name: "📱 Phone", Value: docuser.PhoneNumber, Inline: true},
					{Name: "🎯 Points", Value: fmt.Sprintf("%d", event.Points), Inline: true},
					{Name: "⏰ Deadline", Value: deadline.Format("2006-01-02 15:04:05"), Inline: true},
					{Name: "🆔 Claim ID", Value: fmt.Sprintf("%v", claimID), Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

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
	ownerNumbers := []string{"6285312924192", "6282117252716", "6285179935117", "6285759790334"}
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

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "📝 **Task Submitted!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "Task Submitted",
				Description: "A user has submitted their task for approval",
				Color:       16755200, // Orange color
				Fields: []DiscordEmbedeventField{
					{Name: "📋 Event Name", Value: event.Name, Inline: true},
					{Name: "👤 User Name", Value: docuser.Name, Inline: true},
					{Name: "🎓 NPM", Value: docuser.NPM, Inline: true},
					{Name: "📱 Phone", Value: docuser.PhoneNumber, Inline: true},
					{Name: "🎯 Points", Value: fmt.Sprintf("%d", event.Points), Inline: true},
					{Name: "📅 Submitted At", Value: time.Now().Format("2006-01-02 15:04:05"), Inline: true},
					{Name: "🔗 Task Link", Value: submitReq.TaskLink, Inline: false},
					{Name: "✅ Approval Link", Value: approvalLink, Inline: false},
					{Name: "🆔 Claim ID", Value: claimObjectID.Hex(), Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

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
	allowedNumbers := []string{"6285312924192", "6282117252716", "6285179935117", "6285759790334"}
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

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "✅ **Event Approved!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "Event Approved",
				Description: "An event task has been approved and points awarded",
				Color:       5763719, // Green color
				Fields: []DiscordEmbedeventField{
					{Name: "📋 Event Name", Value: event.Name, Inline: true},
					{Name: "👤 User Name", Value: user.Name, Inline: true},
					{Name: "🎓 NPM", Value: user.NPM, Inline: true},
					{Name: "📱 Phone", Value: user.PhoneNumber, Inline: true},
					{Name: "🎯 Points Awarded", Value: fmt.Sprintf("%d", event.Points), Inline: true},
					{Name: "👨‍💼 Approved By", Value: payload.Id, Inline: true},
					{Name: "📅 Approved At", Value: time.Now().Format("2006-01-02 15:04:05"), Inline: true},
					{Name: "🔗 Task Link", Value: claim.TaskLink, Inline: false},
					{Name: "🆔 Claim ID", Value: claimObjectID.Hex(), Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

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

		// Calculate approval deadline (24 jam dari submitted_at)
		var approvalDeadline time.Time
		if !claim.SubmittedAt.IsZero() {
			approvalDeadline = claim.SubmittedAt.Add(86400 * time.Second) // 24 jam
		}

		claimData := map[string]interface{}{
			"claim_id":          claim.ID.Hex(),
			"event":             event,
			"claimed_at":        claim.ClaimedAt,
			"deadline":          claim.Deadline,
			"status":            claim.Status,
			"task_link":         claim.TaskLink,
			"submitted_at":      claim.SubmittedAt,
			"approval_deadline": approvalDeadline,
			"is_expired":        isExpired,
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

	// Update user's total event points (PointEvent)
	user.PointEvent += event.Points
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": claim.UserPhone}, user)
	if err != nil {
		respn.Status = "Error : Gagal update poin user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "✅ **Event Approved (POST)!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "Event Approved (POST)",
				Description: "An event task has been approved via POST endpoint",
				Color:       5763719, // Green color
				Fields: []DiscordEmbedeventField{
					{Name: "📋 Event Name", Value: event.Name, Inline: true},
					{Name: "👤 User Name", Value: user.Name, Inline: true},
					{Name: "🎓 NPM", Value: user.NPM, Inline: true},
					{Name: "📱 Phone", Value: user.PhoneNumber, Inline: true},
					{Name: "🎯 Points Awarded", Value: fmt.Sprintf("%d", event.Points), Inline: true},
					{Name: "📊 Total Event Points", Value: fmt.Sprintf("%d", user.PointEvent), Inline: true},
					{Name: "📅 Approved At", Value: claim.ApprovedAt.Format("2006-01-02 15:04:05"), Inline: true},
					{Name: "🆔 Claim ID", Value: claimObjectID.Hex(), Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Event berhasil di-approve. User %s mendapat %d poin. Total poin event: %d", user.Name, event.Points, user.PointEvent)
	respn.Data = map[string]interface{}{
		"approved":           true,
		"user":               user.Name,
		"event":              event.Name,
		"points":             event.Points,
		"total_event_points": user.PointEvent,
		"approved_at":        claim.ApprovedAt,
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

// CheckExpiredApprovals untuk check dan recovery expired approvals (24 jam timeout)
func CheckExpiredApprovals(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response

	// Jalankan fungsi internal untuk check dan fix expired approvals
	processedCount := runExpiredApprovalsCheck()

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Processed %d expired approvals", processedCount)
	respn.Data = map[string]interface{}{
		"processed": processedCount,
		"timestamp": time.Now(),
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// runExpiredApprovalsCheck - fungsi internal untuk check dan recovery
func runExpiredApprovalsCheck() int {
	now := time.Now()
	processedCount := 0

	// 1. Check deadline timeout (user tidak submit tepat waktu)
	expiredDeadlineClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"status":   "claimed",
		"deadline": primitive.M{"$lt": now},
	})
	if err == nil {
		for _, claim := range expiredDeadlineClaims {
			// Update claim status ke expired
			claim.Status = "expired"
			_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claim.ID}, claim)
			if err != nil {
				continue
			}

			// Set event kembali aktif
			_, err = atdb.UpdateOneDoc(config.Mongoconn, "events", primitive.M{"_id": claim.EventID}, primitive.M{
				"isactive": true,
			})
			if err == nil {
				processedCount++
				fmt.Printf("✅ Recovered deadline expired event: %s\n", claim.EventID.Hex())
			}
		}
	}

	// 2. Check approval timeout (24 jam setelah submit)
	timeoutLimit := now.Add(-86400 * time.Second) // 24 jam yang lalu

	expiredApprovalClaims, err := atdb.GetAllDoc[[]model.EventClaim](config.Mongoconn, "eventclaims", primitive.M{
		"status":      "submitted",
		"isapproved":  false,
		"submittedat": primitive.M{"$lt": timeoutLimit},
	})
	if err == nil {
		for _, claim := range expiredApprovalClaims {
			// Update claim status ke expired
			claim.Status = "expired"
			_, err = atdb.ReplaceOneDoc(config.Mongoconn, "eventclaims", primitive.M{"_id": claim.ID}, claim)
			if err != nil {
				continue
			}

			// Set event kembali aktif
			_, err = atdb.UpdateOneDoc(config.Mongoconn, "events", primitive.M{"_id": claim.EventID}, primitive.M{
				"isactive": true,
			})
			if err == nil {
				processedCount++
				fmt.Printf("✅ Recovered approval expired event: %s\n", claim.EventID.Hex())
			}
		}
	}

	return processedCount
}

// GetUserEventPoints untuk mendapatkan total poin event user
func GetUserEventPoints(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		fmt.Printf("❌ GetUserEventPoints: Token decode error: %v\n", err)
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	fmt.Printf("✅ GetUserEventPoints: Token valid for user: %s\n", payload.Id)

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{
		"phonenumber": payload.Id,
	})
	if err != nil {
		fmt.Printf("❌ GetUserEventPoints: User not found for phone: %s, error: %v\n", payload.Id, err)
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	fmt.Printf("✅ GetUserEventPoints: User found: %s, PointEvent: %d\n", user.Name, user.PointEvent)

	// Get user's event history
	userEventPoints, err := atdb.GetAllDoc[[]model.EventUserPoint](config.Mongoconn, "eventuserpoint", primitive.M{
		"phone": user.PhoneNumber,
	})
	if err != nil {
		fmt.Printf("⚠️ GetUserEventPoints: Event history error: %v\n", err)
		userEventPoints = []model.EventUserPoint{} // If error, assume no points
	}

	fmt.Printf("✅ GetUserEventPoints: Event history count: %d\n", len(userEventPoints))

	respn.Status = "Success"
	respn.Response = "Data poin event user berhasil diambil"
	respn.Data = map[string]interface{}{
		"user":               user.Name,
		"phone":              user.PhoneNumber,
		"total_event_points": user.PointEvent,
		"event_history":      userEventPoints,
	}

	fmt.Printf("✅ GetUserEventPoints: Sending response with points: %d\n", user.PointEvent)
	at.WriteJSON(respw, http.StatusOK, respn)
}

// BuyBimbinganCode untuk membeli code bimbingan dengan pointevent
func BuyBimbinganCode(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// No request body needed for buying event code (sekali pakai)

	// Get user data
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{
		"phonenumber": payload.Id,
	})
	if err != nil {
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Check if user has enough points
	requiredPoints := 15
	if user.PointEvent < requiredPoints {
		respn.Status = "Error : Poin tidak cukup"
		respn.Response = fmt.Sprintf("Anda memiliki %d poin, butuh %d poin untuk membeli code bimbingan", user.PointEvent, requiredPoints)
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Generate bimbingan code (sama seperti Generate Event Code)
	generatedCode := generateBimbinganCode()

	// Deduct points from user
	user.PointEvent -= requiredPoints
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id}, user)
	if err != nil {
		respn.Status = "Error : Gagal update poin user"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Create EventCode entry (sama seperti GenerateEventCode untuk referral)
	eventCode := model.EventCode{
		Code:      generatedCode,
		CreatedBy: payload.Id,
		CreatedAt: time.Now(),
		IsUsed:    false,
	}

	// Save to eventcodes collection
	eventCodeID, err := atdb.InsertOneDoc(config.Mongoconn, "eventcodes", eventCode)
	if err != nil {
		// Rollback points if failed to save event code
		user.PointEvent += requiredPoints
		atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id}, user)

		respn.Status = "Error : Gagal menyimpan code bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Send Discord notification
	discordPayload := DiscordWebhookPayload{
		Content: "🛒 **Bimbingan Code Purchased!**",
		Embeds: []DiscordEmbedevent{
			{
				Title:       "Bimbingan Code Purchased",
				Description: "A user has purchased a bimbingan code",
				Color:       10038476, // Purple color
				Fields: []DiscordEmbedeventField{
					{Name: "👤 User Name", Value: user.Name, Inline: true},
					{Name: "🎓 NPM", Value: user.NPM, Inline: true},
					{Name: "📱 Phone", Value: user.PhoneNumber, Inline: true},
					{Name: "🎫 Generated Code", Value: generatedCode, Inline: true},
					{Name: "💰 Points Used", Value: fmt.Sprintf("%d", requiredPoints), Inline: true},
					{Name: "📊 Remaining Points", Value: fmt.Sprintf("%d", user.PointEvent), Inline: true},
					{Name: "📅 Created At", Value: eventCode.CreatedAt.Format("2006-01-02 15:04:05"), Inline: true},
					{Name: "🆔 Code ID", Value: fmt.Sprintf("%v", eventCodeID), Inline: false},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	go sendDiscordNotification(discordPayload)

	respn.Status = "Success"
	respn.Response = fmt.Sprintf("Code bimbingan berhasil dibeli! Poin dikurangi %d, sisa %d poin", requiredPoints, user.PointEvent)
	respn.Data = map[string]interface{}{
		"code":             generatedCode,
		"eventcode_id":     eventCodeID,
		"points_used":      requiredPoints,
		"remaining_points": user.PointEvent,
		"created_at":       eventCode.CreatedAt,
		"message":          fmt.Sprintf("Selamat! Anda mendapat code bimbingan: %s (sekali pakai)", generatedCode),
		"usage_note":       "Code ini dapat digunakan sekali oleh satu user untuk mendapatkan bimbingan",
	}
	at.WriteJSON(respw, http.StatusOK, respn)
}

// generateBimbinganCode untuk generate code bimbingan dengan prefix GLR-
func generateBimbinganCode() string {
	// Generate random code dengan prefix GLR-
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return "GLR-" + string(code)
}
