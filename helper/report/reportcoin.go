package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UserTransactionSummary holds summarized transaction data for a user
type UserTransactionSummary struct {
	PhoneNumber string
	Name        string
	WalletAddr  string
	TotalAmount float64
	TxCount     int
}

// GetMerchCoinDailyTransactions generates a message with the daily transactions for a specific wallet
func GetMerchCoinDailyTransactions(db *mongo.Database, walletAddr string) string {
	// Query successful transactions from yesterday for the given wallet
	filter := bson.M{
		"_id":               YesterdayFilter(),
		"user.wonpaywallet": walletAddr,
		"status":            "success",
	}

	orders, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil || len(orders) == 0 {
		return "" // No transactions or error
	}

	// Find the user associated with this wallet
	userFilter := bson.M{"wonpaywallet": walletAddr}
	users, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", userFilter)
	if err != nil || len(users) == 0 {
		return "" // User not found
	}

	user := users[0]

	// Prepare the message
	totalAmount := 0.0
	for _, order := range orders {
		totalAmount += order.Amount
	}

	message := fmt.Sprintf("*MerchCoin Daily Report for %s*\n", user.Name)
	message += fmt.Sprintf("Wallet: %s\n", walletAddr)
	message += fmt.Sprintf("Date: %s\n\n", time.Now().AddDate(0, 0, -1).Format("Monday, January 2, 2006"))
	message += fmt.Sprintf("Successful Transactions: %d\n", len(orders))
	message += fmt.Sprintf("Total Amount: %.8f MBC\n\n", totalAmount)

	// List individual transactions
	message += "*Transaction Details:*\n"
	for i, order := range orders {
		txTime := order.Timestamp.Format("15:04:05")
		message += fmt.Sprintf("%d. TxID: %s\n   Amount: %.8f MBC\n   Time: %s\n",
			i+1,
			shortenTxID(order.TxID),
			order.Amount,
			txTime)
	}

	return message
}

// GetMerchCoinOverallSummary generates a summary of all MerchCoin transactions for the day
func GetMerchCoinOverallSummary(db *mongo.Database) string {
	// Query all successful transactions from yesterday
	filter := bson.M{
		"_id":    YesterdayFilter(),
		"status": "success",
	}

	orders, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil || len(orders) == 0 {
		return "" // No transactions or error
	}

	// Group transactions by wallet address
	walletSummaries := make(map[string]UserTransactionSummary)

	for _, order := range orders {
		// Skip if no amount
		if order.Amount <= 0 {
			continue
		}

		walletAddr := ""
		userName := "Unknown User"
		phoneNumber := ""

		// Get user info if available
		if order.WonpayCode != "" {
			// Try to find the user by code
			userFilter := bson.M{"phonenumber": order.WonpayCode}
			users, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", userFilter)
			if err == nil && len(users) > 0 {
				walletAddr = users[0].Wonpaywallet
				userName = users[0].Name
				phoneNumber = users[0].PhoneNumber
			}
		}

		// Update summary for this wallet
		summary, exists := walletSummaries[walletAddr]
		if !exists {
			summary = UserTransactionSummary{
				PhoneNumber: phoneNumber,
				Name:        userName,
				WalletAddr:  walletAddr,
				TotalAmount: 0,
				TxCount:     0,
			}
		}

		summary.TotalAmount += order.Amount
		summary.TxCount++
		walletSummaries[walletAddr] = summary
	}

	// Calculate totals
	totalTransactions := 0
	totalAmount := 0.0

	for _, summary := range walletSummaries {
		totalTransactions += summary.TxCount
		totalAmount += summary.TotalAmount
	}

	// Prepare the message
	message := "*MerchCoin Daily Overview*\n"
	message += fmt.Sprintf("Date: %s\n\n", time.Now().AddDate(0, 0, -1).Format("Monday, January 2, 2006"))
	message += fmt.Sprintf("Total Transactions: %d\n", totalTransactions)
	message += fmt.Sprintf("Total Volume: %.8f MBC\n", totalAmount)
	message += fmt.Sprintf("Active Users: %d\n\n", len(walletSummaries))

	// List user summaries
	message += "*User Activity:*\n"
	i := 1
	for _, summary := range walletSummaries {
		userName := summary.Name
		if userName == "Unknown User" && summary.WalletAddr != "" {
			userName = shortenWalletAddr(summary.WalletAddr)
		}

		message += fmt.Sprintf("%d. %s: %.8f MBC (%d tx)\n",
			i,
			userName,
			summary.TotalAmount,
			summary.TxCount)
		i++
	}

	return message
}

// GetMerchCoinWeeklySummary generates a weekly summary report of MerchCoin transactions
func GetMerchCoinWeeklySummary(db *mongo.Database) string {
	// Get last 7 days of transactions
	endDate := GetDateSekarang()
	startDate := endDate.AddDate(0, 0, -7)

	filter := bson.M{
		"_id": bson.M{
			"$gte": primitive.NewObjectIDFromTimestamp(startDate),
			"$lt":  primitive.NewObjectIDFromTimestamp(endDate),
		},
		"status": "success",
	}

	orders, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil || len(orders) == 0 {
		return "" // No transactions or error
	}

	// Group transactions by day and by user
	dailyVolume := make(map[string]float64)
	userSummaries := make(map[string]UserTransactionSummary)

	for _, order := range orders {
		// Skip if no amount
		if order.Amount <= 0 {
			continue
		}

		// Get date string for grouping
		dateStr := order.Timestamp.Format("2006-01-02")

		// Update daily volume
		dailyVolume[dateStr] += order.Amount

		// Update user summary
		walletAddr := ""
		userName := "Unknown User"
		phoneNumber := ""

		// Try to find the user
		userFilter := bson.M{"phonenumber": order.WonpayCode}
		users, err := atdb.GetAllDoc[[]model.Userdomyikado](db, "user", userFilter)
		if err == nil && len(users) > 0 {
			walletAddr = users[0].Wonpaywallet
			userName = users[0].Name
			phoneNumber = users[0].PhoneNumber
		}

		// Use user ID or wallet address as key
		userKey := phoneNumber
		if userKey == "" {
			userKey = walletAddr
		}

		// Update summary for this user
		summary, exists := userSummaries[userKey]
		if !exists {
			summary = UserTransactionSummary{
				PhoneNumber: phoneNumber,
				Name:        userName,
				WalletAddr:  walletAddr,
				TotalAmount: 0,
				TxCount:     0,
			}
		}

		summary.TotalAmount += order.Amount
		summary.TxCount++
		userSummaries[userKey] = summary
	}

	// Calculate totals
	totalTransactions := 0
	totalAmount := 0.0

	for _, summary := range userSummaries {
		totalTransactions += summary.TxCount
		totalAmount += summary.TotalAmount
	}

	// Prepare the message
	message := "*MerchCoin Weekly Report*\n"
	message += fmt.Sprintf("Period: %s - %s\n\n",
		startDate.Format("Jan 2"),
		endDate.AddDate(0, 0, -1).Format("Jan 2, 2006"))

	message += fmt.Sprintf("Total Transactions: %d\n", totalTransactions)
	message += fmt.Sprintf("Total Volume: %.8f MBC\n", totalAmount)
	message += fmt.Sprintf("Active Users: %d\n\n", len(userSummaries))

	// Daily volume chart
	message += "*Daily Volume:*\n"
	for i := 0; i < 7; i++ {
		date := endDate.AddDate(0, 0, -7+i)
		dateStr := date.Format("2006-01-02")
		dayStr := date.Format("Mon (Jan 2)")
		volume := dailyVolume[dateStr]

		// Create a simple bar chart with emoji
		barLength := int(volume * 10 / totalAmount)
		if barLength > 10 {
			barLength = 10
		}

		bar := strings.Repeat("▓", barLength) + strings.Repeat("░", 10-barLength)
		message += fmt.Sprintf("%s: %.4f MBC %s\n", dayStr, volume, bar)
	}

	message += "\n*Top Users This Week:*\n"

	// Sort users by volume and list top 5
	type UserRank struct {
		Name        string
		TotalAmount float64
		TxCount     int
	}

	var userRanks []UserRank
	for _, summary := range userSummaries {
		userName := summary.Name
		if userName == "Unknown User" && summary.WalletAddr != "" {
			userName = shortenWalletAddr(summary.WalletAddr)
		}

		userRanks = append(userRanks, UserRank{
			Name:        userName,
			TotalAmount: summary.TotalAmount,
			TxCount:     summary.TxCount,
		})
	}

	// Sort by total amount (descending)
	for i := 0; i < len(userRanks); i++ {
		for j := i + 1; j < len(userRanks); j++ {
			if userRanks[i].TotalAmount < userRanks[j].TotalAmount {
				userRanks[i], userRanks[j] = userRanks[j], userRanks[i]
			}
		}
	}

	// List top 5 (or fewer if there are less than 5)
	topCount := 5
	if len(userRanks) < 5 {
		topCount = len(userRanks)
	}

	for i := 0; i < topCount; i++ {
		message += fmt.Sprintf("%d. %s: %.8f MBC (%d tx)\n",
			i+1,
			userRanks[i].Name,
			userRanks[i].TotalAmount,
			userRanks[i].TxCount)
	}

	return message
}

// GetMerchCoinStats generates statistics about MerchCoin usage to be stored
func GetMerchCoinStats(db *mongo.Database) error {
	// Get yesterday's date
	yesterday := GetDateKemarin()
	dateStr := yesterday.Format("2006-01-02")

	// Get all successful transactions from yesterday
	filter := bson.M{
		"_id":    YesterdayFilter(),
		"status": "success",
	}

	orders, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil {
		return err
	}

	// Calculate statistics
	txCount := len(orders)
	totalAmount := 0.0
	userSet := make(map[string]bool)

	for _, order := range orders {
		totalAmount += order.Amount
		userSet[order.WonpayCode] = true
	}

	activeUsers := len(userSet)

	// Create stats object
	stats := bson.M{
		"date":          dateStr,
		"txCount":       txCount,
		"totalAmount":   totalAmount,
		"activeUsers":   activeUsers,
		"averageAmount": 0.0,
	}

	if txCount > 0 {
		stats["averageAmount"] = totalAmount / float64(txCount)
	}

	// Store stats in database
	statsCollection := db.Collection("merchcoinstats")
	_, err = statsCollection.UpdateOne(
		context.Background(),
		bson.M{"date": dateStr},
		bson.M{"$set": stats},
		options.Update().SetUpsert(true),
	)

	return err
}

// Helper function to shorten transaction IDs for display
func shortenTxID(txID string) string {
	if len(txID) <= 12 {
		return txID
	}
	return txID[:6] + "..." + txID[len(txID)-6:]
}

// Helper function to shorten wallet addresses for display
func shortenWalletAddr(addr string) string {
	if len(addr) <= 12 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-6:]
}

// GenerateMerchCoinReportsForCronJob combines all report generation tasks into one function
func GenerateMerchCoinReportsForCronJob(db *mongo.Database) error {
	// 1. Generate and save statistics
	if err := GetMerchCoinStats(db); err != nil {
		return err
	}

	// 2. Send daily transaction notifications
	// This would need to be implemented in the controller

	// 3. Send weekly reports on specific days (e.g., Mondays)
	today := time.Now().Weekday()
	if today == time.Monday {
		// Flag for weekly report
		// This would need to be implemented in the controller
	}

	return nil
}
