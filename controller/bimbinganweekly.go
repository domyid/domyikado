package controller

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProcessWeeklyBimbingan processes weekly activity scores for all students
// and stores them in the bimbinganweekly collection
func ProcessWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Check if it's actually Monday 5:00 PM
	now := time.Now()
	weekday := int(now.Weekday())
	hour := now.Hour()

	if weekday != 1 || hour != 17 { // 1 is Monday, 17 is 5:00 PM
		log.Printf("Not running weekly bimbingan processing: Current time is %s (weekday: %d, hour: %d)",
			now.Format("2006-01-02 15:04:05"), weekday, hour)
		at.WriteJSON(w, http.StatusOK, model.Response{
			Status: "Skipped: Not Monday 5:00 PM",
		})
		return
	}

	// Calculate current week number
	currentWeek := calculateCurrentWeekNumber()

	// Get all students
	allUsers, err := atdb.GetAllDoc[[]model.Userdomyikado](config.Mongoconn, "user", bson.M{"isdosen": false})
	if err != nil {
		log.Printf("Error getting users: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Response: fmt.Sprintf("Failed to get users: %v", err),
		})
		return
	}

	// Process each student
	var processed, failed int
	for _, user := range allUsers {
		err := processStudentWeeklyData(user, currentWeek)
		if err != nil {
			log.Printf("Error processing user %s: %v", user.PhoneNumber, err)
			failed++
		} else {
			processed++
		}
	}

	// Notify system admin about the process completion
	notifyAdmin(processed, failed, currentWeek)

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status: "Success",
		Response: fmt.Sprintf("Processed %d users, failed %d users for week %d",
			processed, failed, currentWeek),
	})
}

// calculateCurrentWeekNumber calculates the current week number based on
// the start date of the program/semester
func calculateCurrentWeekNumber() int {
	// Define the start date of the program/semester (adjust as needed)
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.Local) // Example: Jan 6, 2025
	now := time.Now()

	// Calculate the number of days since the start date
	daysSinceStart := int(now.Sub(startDate).Hours() / 24)

	// Calculate the week number (1-indexed)
	weekNumber := (daysSinceStart / 7) + 1

	return weekNumber
}

// processStudentWeeklyData processes and stores weekly activity data for a student
func processStudentWeeklyData(user model.Userdomyikado, weekNumber int) error {
	// Get the student's activity score for the week
	activityScore, err := GetLastWeekActivityScoreData(user.PhoneNumber)
	if err != nil {
		return fmt.Errorf("failed to get activity score: %v", err)
	}

	// Check if a record already exists for this user and week
	existingRecord, err := atdb.GetOneDoc[model.BimbinganWeekly](
		config.Mongoconn,
		"bimbinganweekly",
		bson.M{
			"phonenumber": user.PhoneNumber,
			"weeknumber":  weekNumber,
		},
	)

	// Create the weekly record
	weeklyRecord := model.BimbinganWeekly{
		PhoneNumber: user.PhoneNumber,
		Name:        user.Name,
		CreatedAt:   time.Now(),
		WeekNumber:  weekNumber,
		ActivityScore: model.ActivityScore{
			Sponsor:         activityScore.Sponsor,
			Strava:          activityScore.Strava,
			StravaKM:        activityScore.StravaKM,
			IQ:              activityScore.IQ,
			IQresult:        activityScore.IQresult,
			Pomokit:         activityScore.Pomokit,
			Pomokitsesi:     activityScore.Pomokitsesi,
			MBC:             activityScore.MBC,
			MBCPoints:       activityScore.MBCPoints,
			BlockChain:      activityScore.BlockChain,
			RVN:             activityScore.RVN,
			RavencoinPoints: activityScore.RavencoinPoints,
			QRIS:            activityScore.QRIS,
			QRISPoints:      activityScore.QRISPoints,
			Rupiah:          activityScore.Rupiah,
			Tracker:         activityScore.Tracker,
			Trackerdata:     activityScore.Trackerdata,
			BukPed:          activityScore.BukPed,
			BukuKatalog:     activityScore.BukuKatalog,
			Jurnal:          activityScore.Jurnal,
			JurnalWeb:       activityScore.JurnalWeb,
			GTMetrix:        activityScore.GTMetrix,
			GTMetrixResult:  activityScore.GTMetrixResult,
			WebHook:         activityScore.WebHook,
			WebHookpush:     activityScore.WebHookpush,
			Presensi:        activityScore.Presensi,
			PresensiHari:    activityScore.PresensiHari,
			TotalScore:      activityScore.TotalScore,
		},
		Approved: false,
	}

	// If record exists, update it; otherwise, create a new one
	if err == nil && existingRecord.ID != primitive.NilObjectID {
		weeklyRecord.ID = existingRecord.ID
		_, err = atdb.ReplaceOneDoc(
			config.Mongoconn,
			"bimbinganweekly",
			bson.M{"_id": existingRecord.ID},
			weeklyRecord,
		)
		if err != nil {
			return fmt.Errorf("failed to update existing record: %v", err)
		}
	} else {
		_, err = atdb.InsertOneDoc(config.Mongoconn, "bimbinganweekly", weeklyRecord)
		if err != nil {
			return fmt.Errorf("failed to insert new record: %v", err)
		}
	}

	// Also notify the student about their weekly summary
	notifyStudent(user, weeklyRecord)

	return nil
}

// notifyAdmin sends a notification to the system admin about the weekly process
func notifyAdmin(processed, failed, weekNumber int) {
	adminPhone := "6285312924192" // Replace with actual admin phone number

	message := fmt.Sprintf("*Weekly Bimbingan Processing Completed*\n\n"+
		"Week: %d\n"+
		"Processed: %d students\n"+
		"Failed: %d students\n\n"+
		"Time: %s",
		weekNumber, processed, failed, time.Now().Format("2006-01-02 15:04:05"))

	dt := &whatsauth.TextMessage{
		To:       adminPhone,
		IsGroup:  false,
		Messages: message,
	}

	_, _, err := atapi.PostStructWithToken[model.Response](
		"Token",
		config.WAAPIToken,
		dt,
		config.WAAPIMessage,
	)

	if err != nil {
		log.Printf("Failed to notify admin: %v", err)
	}
}

// notifyStudent sends a notification to the student about their weekly summary
func notifyStudent(user model.Userdomyikado, weeklyData model.BimbinganWeekly) {
	// Create a summary message for the student
	message := fmt.Sprintf("*Ringkasan Aktivitas Mingguan*\n\n"+
		"Mahasiswa: %s\n"+
		"Minggu ke-%d\n\n"+
		"*Skor Kegiatan:*\n"+
		"- Sponsor: %d poin\n"+
		"- Strava: %.2f km (%d poin)\n"+
		"- Test IQ: %d (%d poin)\n"+
		"- Pomokit: %d sesi (%d poin)\n"+
		"- Web Tracker: %d visits (%.0f poin)\n"+
		"- BukPed: %d poin\n"+
		"- GTMetrix: %s (%d poin)\n"+
		"- WebHook: %d pushes (%d poin)\n"+
		"- Presensi: %d hari (%d poin)\n\n"+
		"*Total Skor: %d*\n\n"+
		"_Laporan mingguan ini otomatis dibuat setiap Senin jam 5 sore._",
		user.Name, weeklyData.WeekNumber,
		weeklyData.ActivityScore.Sponsor,
		weeklyData.ActivityScore.StravaKM, weeklyData.ActivityScore.Strava,
		weeklyData.ActivityScore.IQresult, weeklyData.ActivityScore.IQ,
		weeklyData.ActivityScore.Pomokitsesi, weeklyData.ActivityScore.Pomokit,
		weeklyData.ActivityScore.Trackerdata, weeklyData.ActivityScore.Tracker,
		weeklyData.ActivityScore.BukPed,
		weeklyData.ActivityScore.GTMetrixResult, weeklyData.ActivityScore.GTMetrix,
		weeklyData.ActivityScore.WebHookpush, weeklyData.ActivityScore.WebHook,
		weeklyData.ActivityScore.PresensiHari, weeklyData.ActivityScore.Presensi,
		weeklyData.ActivityScore.TotalScore)

	dt := &whatsauth.TextMessage{
		To:       user.PhoneNumber,
		IsGroup:  false,
		Messages: message,
	}

	_, _, err := atapi.PostStructWithToken[model.Response](
		"Token",
		config.WAAPIToken,
		dt,
		config.WAAPIMessage,
	)

	if err != nil {
		log.Printf("Failed to notify student %s: %v", user.PhoneNumber, err)
	}
}

// RefreshWeeklyBimbingan is the HTTP handler for manually triggering the weekly bimbingan process
func RefreshWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Force process regardless of the current time
	currentWeek := calculateCurrentWeekNumber()

	// Get all students
	allUsers, err := atdb.GetAllDoc[[]model.Userdomyikado](config.Mongoconn, "user", bson.M{"isdosen": false})
	if err != nil {
		log.Printf("Error getting users: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Response: fmt.Sprintf("Failed to get users: %v", err),
		})
		return
	}

	// Process each student
	var processed, failed int
	for _, user := range allUsers {
		err := processStudentWeeklyData(user, currentWeek)
		if err != nil {
			log.Printf("Error processing user %s: %v", user.PhoneNumber, err)
			failed++
		} else {
			processed++
		}
	}

	// Notify system admin about the process completion
	notifyAdmin(processed, failed, currentWeek)

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status: "Success",
		Response: fmt.Sprintf("Processed %d users, failed %d users for week %d",
			processed, failed, currentWeek),
	})
}

// GetBimbinganWeeklyByWeek gets the bimbingan weekly data for a specific week
func GetBimbinganWeeklyByWeek(w http.ResponseWriter, r *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Get week parameter from query string
	weekStr := r.URL.Query().Get("week")
	if weekStr == "" {
		respn.Status = "Error : Parameter week harus diisi"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	weekNumber, err := strconv.Atoi(weekStr)
	if err != nil || weekNumber < 1 {
		respn.Status = "Error : Parameter week tidak valid, harus >= 1"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Get the weekly bimbingan data
	weeklyData, err := atdb.GetOneDoc[model.BimbinganWeekly](
		config.Mongoconn,
		"bimbinganweekly",
		bson.M{
			"phonenumber": payload.Id,
			"weeknumber":  weekNumber,
		},
	)

	if err != nil {
		respn.Status = "Error : Data bimbingan mingguan tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	at.WriteJSON(w, http.StatusOK, weeklyData)
}
