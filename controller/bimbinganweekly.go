package controller

import (
	"context"
	"encoding/json"
	"fmt"
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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InitializeBimbinganWeeklyStatus initializes the weekly status if it doesn't exist
func InitializeBimbinganWeeklyStatus() error {
	// Check if status document exists
	var status model.BimbinganWeeklyStatus
	err := config.Mongoconn.Collection("bimbinganweeklystatus").FindOne(context.Background(), bson.M{}).Decode(&status)

	if err == mongo.ErrNoDocuments {
		// Create new status document with initial values (Week 1)
		now := time.Now()

		// Calculate start (Monday) and end (Sunday) of the current week
		weekday := int(now.Weekday())
		if weekday == 0 { // Sunday is 0, we want it to be 7
			weekday = 7
		}

		startDate := now.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)                       // Monday at 00:00
		endDate := startDate.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second) // Sunday at 23:59:59

		status = model.BimbinganWeeklyStatus{
			CurrentWeek: 1,
			WeekLabel:   "week1",
			StartDate:   startDate,
			EndDate:     endDate,
			LastUpdated: now,
			UpdatedBy:   "system_init",
		}

		_, err = config.Mongoconn.Collection("bimbinganweeklystatus").InsertOne(context.Background(), status)
		if err != nil {
			return fmt.Errorf("failed to initialize bimbingan weekly status: %v", err)
		}

		fmt.Println("Initialized bimbingan weekly status with Week 1")
	} else if err != nil {
		return fmt.Errorf("error checking bimbingan weekly status: %v", err)
	}

	return nil
}

// GetCurrentWeekStatus returns the current active week status
func GetCurrentWeekStatus() (model.BimbinganWeeklyStatus, error) {
	var status model.BimbinganWeeklyStatus

	// Ensure the status collection is initialized
	err := InitializeBimbinganWeeklyStatus()
	if err != nil {
		return status, err
	}

	// Get the current status
	err = config.Mongoconn.Collection("bimbinganweeklystatus").FindOne(context.Background(), bson.M{}).Decode(&status)
	if err != nil {
		return status, fmt.Errorf("error fetching current week status: %v", err)
	}

	return status, nil
}

// GetBimbinganWeeklyStatus returns the current weekly status information
func GetBimbinganWeeklyStatus(w http.ResponseWriter, r *http.Request) {
	// Validate token if needed (optional)
	_, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Get the current week status
	status, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to get current week status",
			Response: err.Error(),
		})
		return
	}

	// Return the week status
	at.WriteJSON(w, http.StatusOK, status)
}

// ProcessWeeklyBimbingan processes weekly bimbingan data for all users
func ProcessWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Get the current week status
	weekStatus, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to get current week status",
			Response: err.Error(),
		})
		return
	}

	// Process data for the current week
	processed, failed, err := refreshWeeklyBimbinganData(weekStatus.CurrentWeek, weekStatus.WeekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to process weekly bimbingan data",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Processed %d users, %d failed", processed, failed),
		Response: "Weekly bimbingan data has been processed",
	})
}

// RefreshWeeklyBimbingan forces a refresh of weekly bimbingan data
func RefreshWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Get week parameter, default to current week
	weekParam := r.URL.Query().Get("week")

	var weekNumber int
	var err error

	if weekParam != "" {
		weekNumber, err = strconv.Atoi(weekParam)
		if err != nil || weekNumber < 1 {
			at.WriteJSON(w, http.StatusBadRequest, model.Response{
				Status:   "Error",
				Info:     "Invalid week parameter",
				Response: "Week must be a positive integer",
			})
			return
		}
	} else {
		// Get current week from status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			at.WriteJSON(w, http.StatusInternalServerError, model.Response{
				Status:   "Error",
				Info:     "Failed to get current week status",
				Response: err.Error(),
			})
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Generate week label
	weekLabel := fmt.Sprintf("week%d", weekNumber)

	// Force refresh for the specified week
	processed, failed, err := refreshWeeklyBimbinganData(weekNumber, weekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to refresh weekly bimbingan data",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Refreshed %d users, %d failed for week %d", processed, failed, weekNumber),
		Response: "Weekly bimbingan data has been refreshed",
	})
}

// getIncrementalActivityScore calculates incremental activity score for a specific week
// by retrieving all approved previous weeks' scores and subtracting them from the current total score
func getIncrementalActivityScore(phoneNumber string, weekNumber int) (model.ActivityScore, error) {
	// Get current total activity score
	currentScore, err := GetAllActivityScoreData(phoneNumber)
	if err != nil {
		return model.ActivityScore{}, fmt.Errorf("failed to get activity score: %v", err)
	}

	// If this is week 1, return the current score as is
	if weekNumber <= 1 {
		return currentScore, nil
	}

	// Get all previous approved weekly scores
	var previousScores []model.BimbinganWeekly
	filter := bson.M{
		"phonenumber": phoneNumber,
		"weeknumber":  bson.M{"$lt": weekNumber},
		"approved":    true, // Only consider approved weeks
	}

	// Sort by weeknumber ascending to process in chronological order
	opts := options.Find().SetSort(bson.M{"weeknumber": 1})

	cursor, err := config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
	if err != nil {
		return model.ActivityScore{}, fmt.Errorf("failed to get previous weeks data: %v", err)
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &previousScores); err != nil {
		return model.ActivityScore{}, fmt.Errorf("failed to parse previous weeks data: %v", err)
	}

	// If no previous approved weeks, return current score
	if len(previousScores) == 0 {
		return currentScore, nil
	}

	// Calculate total scores from all previous approved weeks
	var totalPreviousScore model.ActivityScore

	// Sum up all previous weeks' scores
	for _, prev := range previousScores {
		totalPreviousScore.Sponsor += prev.ActivityScore.Sponsor
		totalPreviousScore.Strava += prev.ActivityScore.Strava
		totalPreviousScore.IQ += prev.ActivityScore.IQ
		totalPreviousScore.Pomokitsesi += prev.ActivityScore.Pomokitsesi
		totalPreviousScore.Pomokit += prev.ActivityScore.Pomokit
		totalPreviousScore.MBC += prev.ActivityScore.MBC
		totalPreviousScore.MBCPoints += prev.ActivityScore.MBCPoints
		totalPreviousScore.Rupiah += prev.ActivityScore.Rupiah
		totalPreviousScore.QRIS += prev.ActivityScore.QRIS
		totalPreviousScore.QRISPoints += prev.ActivityScore.QRISPoints
		totalPreviousScore.Trackerdata += prev.ActivityScore.Trackerdata
		totalPreviousScore.Tracker += prev.ActivityScore.Tracker
		totalPreviousScore.BukPed += prev.ActivityScore.BukPed
		totalPreviousScore.Jurnal += prev.ActivityScore.Jurnal
		totalPreviousScore.GTMetrix += prev.ActivityScore.GTMetrix
		totalPreviousScore.WebHookpush += prev.ActivityScore.WebHookpush
		totalPreviousScore.WebHook += prev.ActivityScore.WebHook
		totalPreviousScore.PresensiHari += prev.ActivityScore.PresensiHari
		totalPreviousScore.Presensi += prev.ActivityScore.Presensi
		totalPreviousScore.RVN += prev.ActivityScore.RVN
		totalPreviousScore.RavencoinPoints += prev.ActivityScore.RavencoinPoints
		totalPreviousScore.TotalScore += prev.ActivityScore.TotalScore
	}

	// Calculate incremental score (current total - previous approved total)
	incrementalScore := model.ActivityScore{
		// Preserve string/descriptive values
		Sponsordata:    currentScore.Sponsordata,
		StravaKM:       currentScore.StravaKM,
		IQresult:       currentScore.IQresult,
		GTMetrixResult: currentScore.GTMetrixResult,
		BukuKatalog:    currentScore.BukuKatalog,

		// Calculate numeric values by subtraction
		Sponsor:         currentScore.Sponsor - totalPreviousScore.Sponsor,
		Trackerdata:     currentScore.Trackerdata - totalPreviousScore.Trackerdata,
		Tracker:         currentScore.Tracker - totalPreviousScore.Tracker,
		Strava:          currentScore.Strava - totalPreviousScore.Strava,
		IQ:              currentScore.IQ - totalPreviousScore.IQ,
		Pomokitsesi:     currentScore.Pomokitsesi - totalPreviousScore.Pomokitsesi,
		Pomokit:         currentScore.Pomokit - totalPreviousScore.Pomokit,
		BukPed:          currentScore.BukPed - totalPreviousScore.BukPed,
		GTMetrix:        currentScore.GTMetrix - totalPreviousScore.GTMetrix,
		WebHookpush:     currentScore.WebHookpush - totalPreviousScore.WebHookpush,
		WebHook:         currentScore.WebHook - totalPreviousScore.WebHook,
		PresensiHari:    currentScore.PresensiHari - totalPreviousScore.PresensiHari,
		Presensi:        currentScore.Presensi - totalPreviousScore.Presensi,
		MBC:             currentScore.MBC - totalPreviousScore.MBC,
		MBCPoints:       currentScore.MBCPoints - totalPreviousScore.MBCPoints,
		BlockChain:      currentScore.BlockChain - totalPreviousScore.BlockChain,
		RVN:             currentScore.RVN - totalPreviousScore.RVN,
		RavencoinPoints: currentScore.RavencoinPoints - totalPreviousScore.RavencoinPoints,
		Rupiah:          currentScore.Rupiah - totalPreviousScore.Rupiah,
		QRIS:            currentScore.QRIS - totalPreviousScore.QRIS,
		QRISPoints:      currentScore.QRISPoints - totalPreviousScore.QRISPoints,
		TotalScore:      currentScore.TotalScore - totalPreviousScore.TotalScore,
	}

	// Ensure no negative values
	ensureNonNegativeScores(&incrementalScore)

	return incrementalScore, nil
}

// ensureNonNegativeScores ensures all score values are non-negative
func ensureNonNegativeScores(score *model.ActivityScore) {
	if score.Sponsor < 0 {
		score.Sponsor = 0
	}
	if score.Strava < 0 {
		score.Strava = 0
	}
	if score.IQ < 0 {
		score.IQ = 0
	}
	if score.Pomokit < 0 {
		score.Pomokit = 0
	}
	if score.BlockChain < 0 {
		score.BlockChain = 0
	}
	if score.QRIS < 0 {
		score.QRIS = 0
	}
	if float64(score.Tracker) < 0 {
		score.Tracker = 0
	}
	if score.BukPed < 0 {
		score.BukPed = 0
	}
	if score.Jurnal < 0 {
		score.Jurnal = 0
	}
	if score.GTMetrix < 0 {
		score.GTMetrix = 0
	}
	if score.WebHook < 0 {
		score.WebHook = 0
	}
	if score.Presensi < 0 {
		score.Presensi = 0
	}
	if score.TotalScore < 0 {
		score.TotalScore = 0
	}
	if score.MBC < 0 {
		score.MBC = 0
	}
	if score.MBCPoints < 0 {
		score.MBCPoints = 0
	}
	if score.RVN < 0 {
		score.RVN = 0
	}
	if score.RavencoinPoints < 0 {
		score.RavencoinPoints = 0
	}
	if score.QRISPoints < 0 {
		score.QRISPoints = 0
	}
	if score.Rupiah < 0 {
		score.Rupiah = 0
	}
	if score.Trackerdata < 0 {
		score.Trackerdata = 0
	}
	if score.WebHookpush < 0 {
		score.WebHookpush = 0
	}
	if score.PresensiHari < 0 {
		score.PresensiHari = 0
	}
	if score.Pomokitsesi < 0 {
		score.Pomokitsesi = 0
	}
}

// refreshWeeklyBimbinganData updates or creates bimbingan records for all users for a specific week
func refreshWeeklyBimbinganData(weekNumber int, weekLabel string) (processed int, failed int, err error) {
	// Get all users
	users, err := atdb.GetAllDoc[[]model.Userdomyikado](config.Mongoconn, "user", bson.M{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get users: %v", err)
	}

	processed = 0
	failed = 0

	// Process each user
	for _, user := range users {
		if user.PhoneNumber == "" {
			failed++
			continue
		}

		// Calculate incremental activity score for this week
		activityScore, err := getIncrementalActivityScore(user.PhoneNumber, weekNumber)
		if err != nil {
			fmt.Printf("Error calculating incremental score for user %s: %v\n", user.PhoneNumber, err)
			failed++
			continue
		}

		// Check if a record for this user and week already exists
		var existingWeekly model.BimbinganWeekly
		filter := bson.M{
			"phonenumber": user.PhoneNumber,
			"weeknumber":  weekNumber,
		}

		err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingWeekly)

		now := time.Now()

		if err == mongo.ErrNoDocuments {
			// Create new weekly record
			newWeekly := model.BimbinganWeekly{
				PhoneNumber:   user.PhoneNumber,
				WeekNumber:    weekNumber,
				WeekLabel:     weekLabel,
				ActivityScore: activityScore,
				Approved:      false, // Default to not approved
				CreatedAt:     now,
				UpdatedAt:     now,
			}

			_, err = config.Mongoconn.Collection("bimbinganweekly").InsertOne(context.Background(), newWeekly)
			if err != nil {
				failed++
				continue
			}
		} else if err != nil {
			failed++
			continue
		} else {
			// Update existing record but preserve approval status and assessor data
			update := bson.M{
				"$set": bson.M{
					"activityscore": activityScore,
					"updatedAt":     now,
				},
			}

			_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
			if err != nil {
				failed++
				continue
			}
		}

		processed++
	}

	return processed, failed, nil
}

// GetBimbinganWeeklyByWeek returns bimbingan data for a specific user and week
func GetBimbinganWeeklyByWeek(w http.ResponseWriter, r *http.Request) {
	// Get token from header
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Get week parameter, default to current week
	weekParam := r.URL.Query().Get("week")

	var weekNumber int

	if weekParam != "" {
		weekNumber, err = strconv.Atoi(weekParam)
		if err != nil || weekNumber < 1 {
			at.WriteJSON(w, http.StatusBadRequest, model.Response{
				Status:   "Error",
				Info:     "Invalid week parameter",
				Response: "Week must be a positive integer",
			})
			return
		}
	} else {
		// Get current week from status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			at.WriteJSON(w, http.StatusInternalServerError, model.Response{
				Status:   "Error",
				Info:     "Failed to get current week status",
				Response: err.Error(),
			})
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Get user's bimbingan data for the specified week
	filter := bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	if err == mongo.ErrNoDocuments {
		// If no data exists, try to create it first by refreshing
		weekLabel := fmt.Sprintf("week%d", weekNumber)
		_, _, err = refreshWeeklyBimbinganDataForUser(payload.Id, weekNumber, weekLabel)

		if err != nil {
			at.WriteJSON(w, http.StatusNotFound, model.Response{
				Status:   "Error",
				Info:     "No weekly data found and failed to create it",
				Response: err.Error(),
			})
			return
		}

		// Try to get the data again
		err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
		if err != nil {
			at.WriteJSON(w, http.StatusNotFound, model.Response{
				Status:   "Error",
				Info:     "Weekly data not found even after refresh",
				Response: err.Error(),
			})
			return
		}
	} else if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to fetch weekly data",
			Response: err.Error(),
		})
		return
	}

	// Return the weekly data
	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// GetAllBimbinganWeekly returns all available bimbingan weekly data for a user
func GetAllBimbinganWeekly(w http.ResponseWriter, r *http.Request) {
	// Get token from header
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Get all weekly data for this user
	filter := bson.M{
		"phonenumber": payload.Id,
	}

	// Sort by weeknumber ascending
	opts := options.Find().SetSort(bson.M{"weeknumber": 1})

	cursor, err := config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to fetch weekly data",
			Response: err.Error(),
		})
		return
	}
	defer cursor.Close(context.Background())

	var weeklyData []model.BimbinganWeekly
	if err = cursor.All(context.Background(), &weeklyData); err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to parse weekly data",
			Response: err.Error(),
		})
		return
	}

	if len(weeklyData) == 0 {
		// If no data exists, create at least the current week
		status, err := GetCurrentWeekStatus()
		if err == nil {
			// Try to refresh for the current week
			refreshWeeklyBimbinganDataForUser(payload.Id, status.CurrentWeek, status.WeekLabel)

			// Try to get the data again
			cursor, err = config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
			if err == nil {
				defer cursor.Close(context.Background())
				cursor.All(context.Background(), &weeklyData)
			}
		}
	}

	// Return the weekly data
	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// refreshWeeklyBimbinganDataForUser refreshes data for a single user for a specific week
func refreshWeeklyBimbinganDataForUser(phoneNumber string, weekNumber int, weekLabel string) (bool, error, error) {
	// Check if user exists
	_, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": phoneNumber})
	if err != nil {
		return false, fmt.Errorf("failed to get user data: %v", err), err
	}

	// Get the incremental activity scores for this week
	activityScore, err := getIncrementalActivityScore(phoneNumber, weekNumber)
	if err != nil {
		return false, fmt.Errorf("failed to get incremental activity score: %v", err), err
	}

	// Check if a record for this user and week already exists
	var existingWeekly model.BimbinganWeekly
	filter := bson.M{
		"phonenumber": phoneNumber,
		"weeknumber":  weekNumber,
	}

	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingWeekly)

	now := time.Now()

	if err == mongo.ErrNoDocuments {
		// Create new weekly record
		newWeekly := model.BimbinganWeekly{
			PhoneNumber:   phoneNumber,
			WeekNumber:    weekNumber,
			WeekLabel:     weekLabel,
			ActivityScore: activityScore,
			Approved:      false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		_, err = config.Mongoconn.Collection("bimbinganweekly").InsertOne(context.Background(), newWeekly)
		if err != nil {
			return false, fmt.Errorf("failed to create weekly record: %v", err), err
		}

		return true, nil, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check for existing weekly record: %v", err), err
	} else {
		// Update existing record but preserve approval status and assessor data
		update := bson.M{
			"$set": bson.M{
				"activityscore": activityScore,
				"updatedAt":     now,
			},
		}

		_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
		if err != nil {
			return false, fmt.Errorf("failed to update weekly record: %v", err), err
		}

		return true, nil, nil
	}
}

// PostBimbinganWeeklyRequest submits a bimbingan request for approval
func PostBimbinganWeeklyRequest(w http.ResponseWriter, r *http.Request) {
	// Validate token
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

	// Parse request body
	var request struct {
		AsesorPhoneNumber string `json:"asesorPhoneNumber"`
		WeekNumber        int    `json:"weekNumber,omitempty"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validate asesor phone number
	if request.AsesorPhoneNumber == "" {
		respn.Status = "Error : No Telepon Asesor tidak diisi"
		respn.Response = "Isi lebih lengkap terlebih dahulu"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validate user exists
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validate asesor exists and is a dosen
	request.AsesorPhoneNumber = ValidasiNoHP(request.AsesorPhoneNumber)
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": request.AsesorPhoneNumber, "isdosen": true})
	if err != nil {
		respn.Status = "Error : Data asesor tidak di temukan"
		respn.Response = "Nomor Telepon bukan milik Dosen Asesor"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Determine which week to use
	weekNumber := request.WeekNumber
	if weekNumber <= 0 {
		// Get current week from status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			respn.Status = "Error : Gagal mendapatkan status minggu saat ini"
			respn.Response = err.Error()
			at.WriteJSON(w, http.StatusInternalServerError, respn)
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Check if the weekly data already exists and is already approved
	filter := bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
		"approved":    true,
	}

	var existingApproved model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingApproved)
	if err == nil {
		// Already approved
		respn.Status = "Info : Data bimbingan sudah di approve"
		respn.Response = "Bimbingan sudah disetujui, tidak dapat mengajukan ulang."
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Find or create the weekly data for this user and week
	weekLabel := fmt.Sprintf("week%d", weekNumber)
	_, _, err = refreshWeeklyBimbinganDataForUser(payload.Id, weekNumber, weekLabel)
	if err != nil {
		respn.Status = "Error : Gagal memperbarui data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Get the weekly data
	filter = bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
	if err != nil {
		respn.Status = "Error : Gagal mendapatkan data bimbingan mingguan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Update with asesor information
	update := bson.M{
		"$set": bson.M{
			"asesor":    docasesor,
			"updatedAt": time.Now(),
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
	if err != nil {
		respn.Status = "Error : Gagal update data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Send notification to asesor
	message := fmt.Sprintf("*Permintaan Bimbingan Minggu %d*\n"+
		"Mahasiswa : %s\n"+
		"Beri Nilai: %s/%d",
		weekNumber, docuser.Name, "https://www.do.my.id/kambing/#bimbingan", weekNumber)

	dt := &whatsauth.TextMessage{
		To:       docasesor.PhoneNumber,
		IsGroup:  false,
		Messages: message,
	}

	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusUnauthorized, resp)
		return
	}

	// Get the updated data
	config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// ApproveBimbinganWeekly approves or rejects a weekly bimbingan request
func ApproveBimbinganWeekly(w http.ResponseWriter, r *http.Request) {
	// Validate token (only dosen should be able to approve)
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

	// Validate that the approver is a dosen
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id, "isdosen": true})
	if err != nil {
		respn.Status = "Error : Anda bukan dosen asesor"
		respn.Response = "Hanya dosen asesor yang dapat memberikan persetujuan"
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var request struct {
		StudentPhoneNumber string `json:"studentPhoneNumber"`
		WeekNumber         int    `json:"weekNumber"`
		Approved           bool   `json:"approved"`
		Validasi           int    `json:"validasi"`
		Komentar           string `json:"komentar"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validate student phone number and week number
	if request.StudentPhoneNumber == "" || request.WeekNumber <= 0 {
		respn.Status = "Error : Data tidak lengkap"
		respn.Response = "Nomor telepon mahasiswa dan minggu harus diisi"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Check if the bimbingan request exists
	filter := bson.M{
		"phonenumber": request.StudentPhoneNumber,
		"weeknumber":  request.WeekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
	if err != nil {
		respn.Status = "Error : Data bimbingan tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	// Check if the approver is the assigned asesor
	if weeklyData.Asesor.PhoneNumber != payload.Id {
		respn.Status = "Error : Anda bukan asesor yang ditugaskan"
		respn.Response = "Hanya asesor yang ditugaskan yang dapat memberikan persetujuan"
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Update the bimbingan data
	update := bson.M{
		"$set": bson.M{
			"approved":  request.Approved,
			"validasi":  request.Validasi,
			"komentar":  request.Komentar,
			"updatedAt": time.Now(),
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
	if err != nil {
		respn.Status = "Error : Gagal update data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Get student data
	docstudent, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": request.StudentPhoneNumber})
	if err == nil {
		// Send notification to student
		var message string
		if request.Approved {
			message = fmt.Sprintf("Bimbingan Minggu %d Kamu *TELAH DI APPROVE* oleh Dosen %s\n"+
				"Rate : %d\n"+
				"Komentar : %s\n"+
				"Silahkan lanjutkan bimbingan ke sesi berikutnya.",
				request.WeekNumber, docasesor.Name, request.Validasi, request.Komentar)
		} else {
			message = fmt.Sprintf("Bimbingan Minggu %d Kamu *BELUM DI APPROVE* oleh Dosen %s\n"+
				"Rate : %d\n"+
				"Komentar : %s\n"+
				"Silahkan mengajukan ulang bimbingan setelah perbaikan.",
				request.WeekNumber, docasesor.Name, request.Validasi, request.Komentar)
		}

		dt := &whatsauth.TextMessage{
			To:       docstudent.PhoneNumber,
			IsGroup:  false,
			Messages: message,
		}

		atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	}

	// Get the updated data
	config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// ChangeWeekNumber changes the current active week
func ChangeWeekNumber(w http.ResponseWriter, r *http.Request) {
	// Validate token (admin authorization should be implemented here)
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

	// Parse request body
	var request model.ChangeWeekRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Invalid request body",
			Response: err.Error(),
		})
		return
	}

	if request.WeekNumber < 1 {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Invalid week number",
			Response: "Week number must be positive",
		})
		return
	}

	// Set default week label if not provided
	if request.WeekLabel == "" {
		request.WeekLabel = fmt.Sprintf("week%d", request.WeekNumber)
	}

	// Set default updatedBy if not provided
	if request.UpdatedBy == "" {
		request.UpdatedBy = payload.Id
	}

	// Get current week status
	currentStatus, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to get current week status",
			Response: err.Error(),
		})
		return
	}

	// Prevent changing to the same week
	if currentStatus.CurrentWeek == request.WeekNumber {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Week already active",
			Response: fmt.Sprintf("Week %d is already the active week", request.WeekNumber),
		})
		return
	}

	// Calculate new start and end dates
	now := time.Now()

	// Calculate start (Monday) and end (Sunday) of the current week
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday is 0, we want it to be 7
		weekday = 7
	}

	startDate := now.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)                       // Monday at 00:00
	endDate := startDate.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second) // Sunday at 23:59:59

	// Update the week status
	update := bson.M{
		"$set": bson.M{
			"currentweek": request.WeekNumber,
			"weeklabel":   request.WeekLabel,
			"startdate":   startDate,
			"enddate":     endDate,
			"lastupdated": now,
			"updatedby":   request.UpdatedBy,
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweeklystatus").UpdateOne(
		context.Background(),
		bson.M{},
		update,
		options.Update().SetUpsert(true),
	)

	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to update week number",
			Response: err.Error(),
		})
		return
	}

	// Process weekly data for the new week
	processed, failed, err := refreshWeeklyBimbinganData(request.WeekNumber, request.WeekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Failed to process data for the new week",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Changed to week %d and processed %d users, %d failed", request.WeekNumber, processed, failed),
		Response: "Week number has been updated",
	})
}
