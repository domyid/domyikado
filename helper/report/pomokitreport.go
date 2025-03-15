package report

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PomokitWeeklySummary stores user weekly activity summary
type PomokitWeeklySummary struct {
	PhoneNumber   string
	Name          string
	TotalSessions int
	TotalPoin     float64
}

// GetPomokitReportYesterday filters Pomokit data from yesterday
func GetPomokitReportYesterday(db *mongo.Database) ([]model.PomodoroReport, error) {
	// Get yesterday's date
	yesterday := GetDateKemarin()
	today := GetDateSekarang()

	// Create a filter for yesterday's data
	filter := bson.M{
		"createdAt": bson.M{
			"$gte": yesterday,
			"$lt":  today,
		},
	}

	// Get all pomokit reports from yesterday
	reports, err := atdb.GetAllDoc[[]model.PomodoroReport](db, "pomokit", filter)
	if err != nil {
		return nil, err
	}

	return reports, nil
}

// GetPomokitReportWeekly filters Pomokit data from the past week
func GetPomokitReportWeekly(db *mongo.Database) ([]model.PomodoroReport, error) {
	// Get date for 7 days ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	startOfWeek := time.Date(oneWeekAgo.Year(), oneWeekAgo.Month(), oneWeekAgo.Day(), 0, 0, 0, 0, oneWeekAgo.Location())
	today := GetDateSekarang()

	// Create a filter for the past week's data
	filter := bson.M{
		"createdAt": bson.M{
			"$gte": startOfWeek,
			"$lt":  today,
		},
	}

	// Get all pomokit reports from the past week
	reports, err := atdb.GetAllDoc[[]model.PomodoroReport](db, "pomokit", filter)
	if err != nil {
		return nil, err
	}

	return reports, nil
}

// CalculatePomokitPoints calculates points based on session count
// Ignores the 'cycle' field and counts each entry as 1 session = 1 point
func CalculatePomokitPoints(reports []model.PomodoroReport) map[string]PomokitUserSummary {
	userSummary := make(map[string]PomokitUserSummary)

	// Group reports by user and count sessions
	for _, report := range reports {
		// Skip entries with empty phone numbers
		if report.PhoneNumber == "" {
			continue
		}
		
		if summary, exists := userSummary[report.PhoneNumber]; exists {
			// Update existing summary
			summary.Sessions++
			summary.Poin = float64(summary.Sessions) // 1 point per session, regardless of cycle value
			if report.CreatedAt.After(summary.LastActive) {
				summary.LastActive = report.CreatedAt
			}
			userSummary[report.PhoneNumber] = summary
		} else {
			// Create new summary
			userSummary[report.PhoneNumber] = PomokitUserSummary{
				PhoneNumber: report.PhoneNumber,
				Name:        report.Name,
				Sessions:    1,
				Poin:        1.0, // 1 point per session
				LastActive:  report.CreatedAt,
			}
		}
	}

	return userSummary
}

// UpdatePomokitUserPoints updates user points in the database
func UpdatePomokitUserPoints(db *mongo.Database, summary map[string]PomokitUserSummary) error {
	for _, userSummary := range summary {
		// Check if user exists in pomokitpoin collection
		var userPoin PomokitPoin
		err := db.Collection("pomokitpoin").FindOne(
			context.Background(),
			bson.M{"phonenumber": userSummary.PhoneNumber},
		).Decode(&userPoin)

		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				// User doesn't exist, create new record
				newUserPoin := PomokitPoin{
					Name:        userSummary.Name,
					PhoneNumber: userSummary.PhoneNumber,
					Poin:        userSummary.Poin,
					LastUpdated: time.Now(),
				}
				_, err = db.Collection("pomokitpoin").InsertOne(context.Background(), newUserPoin)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			// User exists, update points
			userPoin.Poin += userSummary.Poin
			userPoin.LastUpdated = time.Now()
			_, err = db.Collection("pomokitpoin").ReplaceOne(
				context.Background(),
				bson.M{"phonenumber": userSummary.PhoneNumber},
				userPoin,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// CalculateWeeklyPomokitSummary processes past week's data into weekly summaries
func CalculateWeeklyPomokitSummary(reports []model.PomodoroReport) []PomokitWeeklySummary {
	// Map to store user summaries
	userSummaries := make(map[string]PomokitWeeklySummary)
	
	// Group reports by user and count sessions
	for _, report := range reports {
		if report.PhoneNumber == "" {
			continue
		}
		
		// Update user summary
		if summary, exists := userSummaries[report.PhoneNumber]; exists {
			summary.TotalSessions++
			summary.TotalPoin = float64(summary.TotalSessions)
			userSummaries[report.PhoneNumber] = summary
		} else {
			userSummaries[report.PhoneNumber] = PomokitWeeklySummary{
				PhoneNumber:   report.PhoneNumber,
				Name:          report.Name,
				TotalSessions: 1,
				TotalPoin:     1.0,
			}
		}
	}
	
	// Convert map to slice
	var result []PomokitWeeklySummary
	for _, summary := range userSummaries {
		result = append(result, summary)
	}
	
	// Sort by total sessions (descending)
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSessions > result[j].TotalSessions
	})
	
	return result
}

// GenerateWeeklyReportMessage creates a simplified weekly report message
func GenerateWeeklyReportMessage(weeklySummaries []PomokitWeeklySummary) string {
	msg := "*Laporan Mingguan Aktivitas Pomokit:*\n\n"
	
	if len(weeklySummaries) == 0 {
		return msg + "Tidak ada aktivitas Pomokit yang tercatat minggu ini."
	}
	
	// Sort summaries by total points (sessions) in descending order
	sort.Slice(weeklySummaries, func(i, j int) bool {
		return weeklySummaries[i].TotalPoin > weeklySummaries[j].TotalPoin
	})
	
	// Show simple user summaries
	for _, summary := range weeklySummaries {
		msg += summary.Name + " (" + summary.PhoneNumber + "): " + 
			strconv.FormatFloat(summary.TotalPoin, 'f', 0, 64) + " poin\n"
	}
	
	return msg
}

// ProcessPomokitWeeklySummary handles the complete Pomokit weekly reporting process
func ProcessPomokitWeeklySummary(db *mongo.Database) error {
	// Get the past week's Pomokit reports
	reports, err := GetPomokitReportWeekly(db)
	if err != nil {
		return err
	}
	
	// Calculate weekly summaries
	weeklySummaries := CalculateWeeklyPomokitSummary(reports)
	
	// Generate report message
	msg := GenerateWeeklyReportMessage(weeklySummaries)
	
	// Send to the same groups as daily reports
	return SendPomokitWeeklyReport(db, msg)
}

// SendPomokitWeeklyReport sends the weekly report to appropriate groups
func SendPomokitWeeklyReport(db *mongo.Database, msg string) error {
	// Hardcoded WAGroupID for testing
	hardcodedGroupID := "120363298977628161" // Replace with your specific WAGroupID

	// First try to send to the hardcoded group
	dt := &whatsauth.TextMessage{
		To:       hardcodedGroupID,
		IsGroup:  true,
		Messages: msg,
	}

	// Send message to hardcoded group
	_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		// If hardcoded group fails, try WAGroupIDs from Pomokit data
		// Get unique WAGroupIDs from the past week's Pomokit reports
		reports, err := GetPomokitReportWeekly(db)
		if err != nil {
			return err
		}
		
		// Create a map to track unique group IDs
		uniqueGroupIDs := make(map[string]bool)
		
		// Extract unique WAGroupIDs
		for _, report := range reports {
			if report.WaGroupID != "" && report.WaGroupID != hardcodedGroupID {
				uniqueGroupIDs[report.WaGroupID] = true
			}
		}
		
		// Send message to each unique group
		for groupID := range uniqueGroupIDs {
			// Skip groups with hyphen in ID as they're not supported
			if strings.Contains(groupID, "-") {
				continue
			}
			
			dt := &whatsauth.TextMessage{
				To:       groupID,
				IsGroup:  true,
				Messages: msg,
			}
			
			// Send message
			_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
			if err != nil {
				// Continue with other groups even if one fails
				continue
			}
		}
	}
	
	return nil
}

// DeductPointsForInactiveUsers reduces points for users without activity yesterday
func DeductPointsForInactiveUsers(db *mongo.Database, activeUsers map[string]PomokitUserSummary) error {
	// Get all users from pomokitpoin collection
	var allUsers []PomokitPoin
	cursor, err := db.Collection("pomokitpoin").Find(context.Background(), bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &allUsers); err != nil {
		return err
	}

	// Check each user for inactivity
	for _, user := range allUsers {
		if _, isActive := activeUsers[user.PhoneNumber]; !isActive && !HariLibur(GetDateKemarin()) {
			// User was inactive yesterday and it wasn't a holiday
			user.Poin -= 1.0 // Deduct 1 point
			user.LastUpdated = time.Now()
			_, err = db.Collection("pomokitpoin").ReplaceOne(
				context.Background(),
				bson.M{"phonenumber": user.PhoneNumber},
				user,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GeneratePomokitReportMessage creates a formatted report message
func GeneratePomokitReportMessage(db *mongo.Database, activeUsers map[string]PomokitUserSummary) string {
	msg := "*Laporan Aktivitas Pomokit Kemarin:*\n\n"

	// Get all users sorted by points
	var allUsers []PomokitPoin
	opts := options.Find().SetSort(bson.D{{Key: "poin", Value: -1}})
	cursor, err := db.Collection("pomokitpoin").Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return msg + "Error retrieving user data: " + err.Error()
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &allUsers); err != nil {
		return msg + "Error processing user data: " + err.Error()
	}

	// Format active users section
	for _, user := range allUsers {
		if summary, isActive := activeUsers[user.PhoneNumber]; isActive {
			msg += "✅ " + user.Name + " (" + user.PhoneNumber + "): +" + 
				strconv.Itoa(summary.Sessions) + ", total: " + 
				strconv.FormatFloat(user.Poin, 'f', 0, 64) + " poin\n"
		} else if !HariLibur(GetDateKemarin()) {
			msg += "⛔ " + user.Name + " (" + user.PhoneNumber + "): -1, total: " + 
				strconv.FormatFloat(user.Poin, 'f', 0, 64) + " poin\n"
		}
	}
	
	return msg
}

// ProcessPomokitDailyReport handles the complete Pomokit reporting process
func ProcessPomokitDailyReport(db *mongo.Database) error {
	// Get yesterday's Pomokit reports
	reports, err := GetPomokitReportYesterday(db)
	if err != nil {
		return err
	}

	// Calculate points for active users
	activeUsers := CalculatePomokitPoints(reports)

	// Update points for active users
	if err := UpdatePomokitUserPoints(db, activeUsers); err != nil {
		return err
	}

	// Deduct points for inactive users (if not a holiday)
	if err := DeductPointsForInactiveUsers(db, activeUsers); err != nil {
		return err
	}

	// Generate and send report messages
	return SendPomokitReportToGroups(db, activeUsers)
}

// SendPomokitReportToGroups sends Pomokit reports to all relevant WA groups
func SendPomokitReportToGroups(db *mongo.Database, activeUsers map[string]PomokitUserSummary) error {
	// Generate report message
	msg := GeneratePomokitReportMessage(db, activeUsers)

	// Get unique WAGroupIDs from yesterday's Pomokit reports
	reports, err := GetPomokitReportYesterday(db)
	if err != nil {
		return err
	}

	// Create a map to track unique group IDs
	uniqueGroupIDs := make(map[string]bool)
	
	// Extract unique WAGroupIDs
	for _, report := range reports {
		if report.WaGroupID != "" {
			uniqueGroupIDs[report.WaGroupID] = true
		}
	}

	// No fallback to project collection - if no WAGroupIDs found in Pomokit data,
	// we'll send individual messages to users instead

	// Send message to each unique group
	for groupID := range uniqueGroupIDs {
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// Skip groups with hyphen in ID as they're not supported
		if strings.Contains(groupID, "-") {
			continue
		}

		// Send message
		_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			// Continue with other groups even if one fails
			continue
		}
	}
	
	// If no groups found or all failed, send individual messages to all active users
	if len(uniqueGroupIDs) == 0 {
		// Get all users who have Pomokit data
		var pomokitUsers []PomokitPoin
		cursor, err := db.Collection("pomokitpoin").Find(context.Background(), bson.M{})
		if err != nil {
			return err
		}
		defer cursor.Close(context.Background())
		
		if err = cursor.All(context.Background(), &pomokitUsers); err != nil {
			return err
		}
		
		// Send individual messages to users
		for _, user := range pomokitUsers {
			if user.PhoneNumber == "" {
				continue
			}
			
			dt := &whatsauth.TextMessage{
				To:       user.PhoneNumber,
				IsGroup:  false,
				Messages: msg,
			}
			
			// Send message
			_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
			if err != nil {
				// Continue with other users even if one fails
				continue
			}
		}
	}

	return nil
}

// GetAllPomokitUsers mendapatkan semua user dari koleksi pomokitpoin
func GetAllPomokitUsers(db *mongo.Database) ([]PomokitPoin, error) {
	var users []PomokitPoin
	cursor, err := db.Collection("pomokitpoin").Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	
	if err = cursor.All(context.Background(), &users); err != nil {
		return nil, err
	}
	
	return users, nil
}

// ProcessSingleUser memproses data satu user untuk batch processing
func ProcessSingleUser(db *mongo.Database, user PomokitPoin) error {
	// Dapatkan aktivitas user kemarin
	filter := bson.M{
		"phonenumber": user.PhoneNumber,
		"createdAt": bson.M{
			"$gte": GetDateKemarin(),
			"$lt":  GetDateSekarang(),
		},
	}
	
	var reports []model.PomodoroReport
	cursor, err := db.Collection("pomokit").Find(context.Background(), filter)
	if err != nil {
		return err
	}
	defer cursor.Close(context.Background())
	
	if err = cursor.All(context.Background(), &reports); err != nil {
		return err
	}
	
	// Jika ada aktivitas, tambahkan poin
	if len(reports) > 0 {
		user.Poin += float64(len(reports))
		_, err = db.Collection("pomokitpoin").ReplaceOne(
			context.Background(),
			bson.M{"phonenumber": user.PhoneNumber},
			user,
		)
		if err != nil {
			return err
		}
	} else if !HariLibur(GetDateKemarin()) {
		// Jika tidak ada aktivitas dan bukan hari libur, kurangi poin
		user.Poin -= 1.0
		_, err = db.Collection("pomokitpoin").ReplaceOne(
			context.Background(),
			bson.M{"phonenumber": user.PhoneNumber},
			user,
		)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// ProcessPomokitWithConcurrency memproses data Pomokit dengan concurrency untuk performa lebih baik
func ProcessPomokitWithConcurrency(db *mongo.Database) error {
	// Dapatkan aktivitas kemarin
	reports, err := GetPomokitReportYesterday(db)
	if err != nil {
		return err
	}
	
	// Hitung poin dari aktivitas
	activeUsers := CalculatePomokitPoints(reports)
	
	// Update poin dalam goroutines
	var wg sync.WaitGroup
	errorCh := make(chan error, 2) // Channel untuk error
	
	// Goroutine untuk update poin pengguna aktif
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := UpdatePomokitUserPoints(db, activeUsers); err != nil {
			errorCh <- errors.New("Gagal memperbarui poin pengguna aktif: " + err.Error())
		}
	}()
	
	// Goroutine untuk mengurangi poin pengguna tidak aktif
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DeductPointsForInactiveUsers(db, activeUsers); err != nil {
			errorCh <- errors.New("Gagal mengurangi poin pengguna tidak aktif: " + err.Error())
		}
	}()
	
	// Tunggu semua goroutine selesai
	wg.Wait()
	close(errorCh)
	
	// Check for errors
	select {
	case err := <-errorCh:
		return err
	default:
		// No errors
	}
	
	return nil
}

// ProcessActiveUsersBatch memproses pengguna aktif dalam batch untuk sistem dengan banyak user
func ProcessActiveUsersBatch(db *mongo.Database, activeUsers map[string]PomokitUserSummary, batchSize int) error {
	// Convert map to slice for batching
	var userList []PomokitUserSummary
	for _, summary := range activeUsers {
		userList = append(userList, summary)
	}
	
	// Process in batches
	for i := 0; i < len(userList); i += batchSize {
		end := i + batchSize
		if end > len(userList) {
			end = len(userList)
		}
		
		batch := userList[i:end]
		
		// Process batch concurrently
		var wg sync.WaitGroup
		errCh := make(chan error, len(batch))
		
		for _, userSummary := range batch {
			wg.Add(1)
			go func(summary PomokitUserSummary) {
				defer wg.Done()
				
				// Get user from database
				var user PomokitPoin
				err := db.Collection("pomokitpoin").FindOne(
					context.Background(),
					bson.M{"phonenumber": summary.PhoneNumber},
				).Decode(&user)
				
				if err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						// User doesn't exist, create new
						newUser := PomokitPoin{
							Name:        summary.Name,
							PhoneNumber: summary.PhoneNumber,
							Poin:        summary.Poin,
						}
						_, err = db.Collection("pomokitpoin").InsertOne(context.Background(), newUser)
						if err != nil {
							errCh <- err
						}
					} else {
						errCh <- err
					}
					return
				}
				
				// Update user points
				user.Poin += summary.Poin
				_, err = db.Collection("pomokitpoin").ReplaceOne(
					context.Background(),
					bson.M{"phonenumber": summary.PhoneNumber},
					user,
				)
				if err != nil {
					errCh <- err
				}
			}(userSummary)
		}
		
		// Wait for all goroutines in this batch
		wg.Wait()
		close(errCh)
		
		// Check for errors
		for err := range errCh {
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}