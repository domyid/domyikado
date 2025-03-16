package report

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MerchCoinInfo struct {
	Count       float64
	Amount      float64
	Name        string
	PhoneNumber string
	WonpayCode  string
}

// GetMerchCoinTransactionsReport generates a daily report of MerchCoin transactions
func GetMerchCoinTransactionsReport(db *mongo.Database) (msg string, err error) {
	// Get transactions from the last 24 hours
	transactions, err := getMerchCoinTransactionsLast24Hours(db)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data transaksi MerchCoin: %v", err)
	}

	msg = "*Laporan Transaksi MerchCoin 24 Jam Terakhir :*\n\n"

	if len(transactions) == 0 {
		msg += "Tidak ada transaksi MerchCoin dalam 24 jam terakhir."
		return msg, nil
	}

	// Group transactions by phone number
	userTransactions := groupTransactionsByUser(transactions)

	// Convert map to slice for sorting
	var userList []MerchCoinInfo
	for _, info := range userTransactions {
		userList = append(userList, info)
	}

	// Sort by total amount (highest to lowest)
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Amount > userList[j].Amount
	})

	// Format the report
	for _, info := range userList {
		msg += "💰 " + info.Name + " (" + info.WonpayCode + "): " +
			strconv.FormatFloat(info.Amount, 'f', 8, 64) + " MBC (" +
			strconv.FormatFloat(info.Count, 'f', 0, 64) + " transaksi)\n"
	}

	// Add summary
	totalAmount := getTotalAmount(userList)
	totalTransactions := getTotalTransactions(userList)

	msg += "\n*Ringkasan:*\n"
	msg += "- Total Transaksi: " + strconv.FormatInt(totalTransactions, 10) + "\n"
	msg += "- Total MBC: " + strconv.FormatFloat(totalAmount, 'f', 8, 64) + "\n"
	msg += "\nUntuk melakukan pembayaran dengan MerchCoin, silahkan kunjungi https://www.do.my.id/micro"

	return msg, nil
}

// GetMerchCoinWeeklyReport generates a weekly report of MerchCoin transactions
func GetMerchCoinWeeklyReport(db *mongo.Database) (msg string, err error) {
	// Get transactions from the last week
	transactions, err := getMerchCoinTransactionsLastWeek(db)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data transaksi MerchCoin: %v", err)
	}

	msg = "*Laporan Transaksi MerchCoin Minggu Ini :*\n\n"

	if len(transactions) == 0 {
		msg += "Tidak ada transaksi MerchCoin minggu ini."
		return msg, nil
	}

	// Group transactions by phone number
	userTransactions := groupTransactionsByUser(transactions)

	// Convert map to slice for sorting
	var userList []MerchCoinInfo
	for _, info := range userTransactions {
		userList = append(userList, info)
	}

	// Sort by total amount (highest to lowest)
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Amount > userList[j].Amount
	})

	// Split into active and inactive users
	var activeUsers, inactiveUsers string

	for _, info := range userList {
		if info.Count > 0 {
			activeUsers += "💰 " + info.Name + " (" + info.WonpayCode + "): " +
				strconv.FormatFloat(info.Amount, 'f', 8, 64) + " MBC (" +
				strconv.FormatFloat(info.Count, 'f', 0, 64) + " transaksi)\n"
		} else {
			inactiveUsers += "⛔ " + info.Name + " (" + info.WonpayCode + "): 0 MBC\n"
		}
	}

	if activeUsers != "" {
		msg += "Pengguna yang melakukan transaksi: \n" + activeUsers + "\n"
	}

	// Add summary
	totalAmount := getTotalAmount(userList)
	totalTransactions := getTotalTransactions(userList)

	msg += "\n*Ringkasan Mingguan:*\n"
	msg += "- Total Transaksi: " + strconv.FormatInt(totalTransactions, 10) + "\n"
	msg += "- Total MBC: " + strconv.FormatFloat(totalAmount, 'f', 8, 64) + "\n"
	msg += "\nUntuk melakukan pembayaran dengan MerchCoin, silahkan kunjungi https://www.do.my.id/micro"

	return msg, nil
}

// SendMerchCoinDailyReport sends the daily report to all relevant WhatsApp groups
func SendMerchCoinDailyReport(db *mongo.Database) error {
	// Generate the report
	reportMsg, err := GetMerchCoinTransactionsReport(db)
	if err != nil {
		return err
	}

	// Get all unique group IDs
	groupIDs, err := getMerchCoinGroupIDs(db)
	if err != nil {
		return err
	}

	// Send the report to each group
	for _, groupID := range groupIDs {
		err = sendWhatsAppMessage(groupID, reportMsg, true)
		if err != nil {
			// Continue to the next group even if one fails
			fmt.Printf("Error sending MerchCoin report to group %s: %v\n", groupID, err)
		}
	}

	return nil
}

// SendMerchCoinWeeklyReport sends the weekly report to all relevant WhatsApp groups
func SendMerchCoinWeeklyReport(db *mongo.Database) error {
	// Generate the report
	reportMsg, err := GetMerchCoinWeeklyReport(db)
	if err != nil {
		return err
	}

	// Get all unique group IDs
	groupIDs, err := getMerchCoinGroupIDs(db)
	if err != nil {
		return err
	}

	// Send the report to each group
	for _, groupID := range groupIDs {
		err = sendWhatsAppMessage(groupID, reportMsg, true)
		if err != nil {
			// Continue to the next group even if one fails
			fmt.Printf("Error sending MerchCoin weekly report to group %s: %v\n", groupID, err)
		}
	}

	return nil
}

// Helper functions

// getMerchCoinTransactionsLast24Hours gets all successful MerchCoin transactions in the last 24 hours
func getMerchCoinTransactionsLast24Hours(db *mongo.Database) ([]model.MerchCoinOrder, error) {
	// Calculate time 24 hours ago
	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)

	// Query for successful transactions in the last 24 hours
	filter := bson.M{
		"status":    "success",
		"timestamp": bson.M{"$gte": twentyFourHoursAgo},
	}

	// Get transactions from database
	transactions, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// getMerchCoinTransactionsLastWeek gets all successful MerchCoin transactions in the last week
func getMerchCoinTransactionsLastWeek(db *mongo.Database) ([]model.MerchCoinOrder, error) {
	// Calculate the start and end of the week
	monday, sunday := getMerchCoinWeekStartEnd(time.Now())

	// Query for successful transactions in the specified week
	filter := bson.M{
		"status":    "success",
		"timestamp": bson.M{"$gte": monday, "$lte": sunday},
	}

	// Get transactions from database
	transactions, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// groupTransactionsByUser groups transactions by user and calculates totals
func groupTransactionsByUser(transactions []model.MerchCoinOrder) map[string]MerchCoinInfo {
	userTransactions := make(map[string]MerchCoinInfo)

	for _, tx := range transactions {
		// Use WonpayCode as the key
		key := tx.WonpayCode

		if info, exists := userTransactions[key]; exists {
			// Increment count and amount for existing user
			info.Count++
			info.Amount += tx.Amount
			userTransactions[key] = info
		} else {
			// Create new entry for user
			userTransactions[key] = MerchCoinInfo{
				Name:        getUserNameByWonpayCode(tx.WonpayCode),  // You'll need to implement this
				PhoneNumber: getUserPhoneByWonpayCode(tx.WonpayCode), // You'll need to implement this
				WonpayCode:  tx.WonpayCode,
				Count:       1,
				Amount:      tx.Amount,
			}
		}
	}

	return userTransactions
}

// getUserNameByWonpayCode gets a user name from their Wonpay code
func getUserNameByWonpayCode(wonpayCode string) string {
	// Implementation depends on how you store this data
	// This is a placeholder - you should replace with actual lookup
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"wonpaywallet": wonpayCode})
	if err != nil {
		return wonpayCode // Return wonpay code if user not found
	}
	return user.Name
}

// getUserPhoneByWonpayCode gets a user phone number from their Wonpay code
func getUserPhoneByWonpayCode(wonpayCode string) string {
	// Implementation depends on how you store this data
	// This is a placeholder - you should replace with actual lookup
	user, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"wonpaywallet": wonpayCode})
	if err != nil {
		return "" // Return empty string if user not found
	}
	return user.PhoneNumber
}

// getMerchCoinGroupIDs gets all group IDs that should receive MerchCoin reports
func getMerchCoinGroupIDs(db *mongo.Database) ([]string, error) {
	// This function should return all group IDs that should receive the report
	// For example, you might get them from projects that use MerchCoin

	// Placeholder implementation - get all group IDs from projects
	projects, err := atdb.GetAllDoc[[]model.Project](db, "project", bson.M{})
	if err != nil {
		return nil, err
	}

	// Collect unique group IDs
	uniqueGroups := make(map[string]bool)
	var groupIDs []string

	for _, project := range projects {
		if project.WAGroupID != "" {
			uniqueGroups[project.WAGroupID] = true
		}
	}

	for groupID := range uniqueGroups {
		groupIDs = append(groupIDs, groupID)
	}

	return groupIDs, nil
}

// Helper function to send a WhatsApp message
func sendWhatsAppMessage(to string, message string, isGroup bool) error {
	dt := &whatsauth.TextMessage{
		To:       to,
		IsGroup:  isGroup,
		Messages: message,
	}

	_, _, err := atapi.PostStructWithToken[model.Response](
		"Token",
		config.WAAPIToken,
		dt,
		config.WAAPIMessage,
	)
	return err
}

// getTotalAmount calculates the total amount of MBC from a list of user info
func getTotalAmount(userList []MerchCoinInfo) float64 {
	var total float64
	for _, info := range userList {
		total += info.Amount
	}
	return total
}

// getTotalTransactions counts the total number of transactions
func getTotalTransactions(userList []MerchCoinInfo) int64 {
	var total int64
	for _, info := range userList {
		total += int64(info.Count)
	}
	return total
}

func getMerchCoinWeekStartEnd(t time.Time) (time.Time, time.Time) {
	weekday := int(t.Weekday())
	// If Sunday (0), we go back 6 days to the previous Monday
	if weekday == 0 {
		weekday = 7
	}

	// Get Monday of this week
	monday := t.AddDate(0, 0, -weekday+1)
	sunday := monday.AddDate(0, 0, 6) // Calculate Sunday

	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	sunday = time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 999999999, t.Location())

	return monday, sunday
}
