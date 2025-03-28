package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CrowdfundingInfo stores payment data for reporting
type CrowdfundingInfo struct {
	Name          string              `bson:"name"`
	PhoneNumber   string              `bson:"phoneNumber"`
	Amount        float64             `bson:"amount"`
	PaymentMethod model.PaymentMethod `bson:"paymentMethod"`
	Timestamp     time.Time           `bson:"timestamp"`
	Status        string              `bson:"status"`
	WaGroupID     string              // Will be populated from project collection
}

// Helper function to format MBC amount for display
// Convert 0.0002 to "2 coin MBC"
func formatMBCAmount(amount float64) string {
	// Convert to coin MBC (multiply by 10000)
	coinAmount := amount * 10000

	// Format with no decimal places
	return fmt.Sprintf("%.0f coin MBC", coinAmount)
}

// GetJumlahMBCLastWeek returns the total MicroBitcoin amount contributed by a user in the last week
func GetJumlahMBCLastWeek(db *mongo.Database, phoneNumber string) (float64, error) {
	// Calculate the date one week ago from now
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Create a filter to find successful MicroBitcoin payments from the specified phone number in the last week
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.MicroBitcoin,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	// Query the collection
	cursor, err := db.Collection("crowdfundingorders").Find(context.TODO(), filter)
	if err != nil {
		return 0, fmt.Errorf("error querying MicroBitcoin payments: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Calculate the total
	var totalAmount float64
	for cursor.Next(context.TODO()) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			return 0, fmt.Errorf("error decoding payment: %v", err)
		}
		totalAmount += payment.Amount
	}

	if err := cursor.Err(); err != nil {
		return 0, fmt.Errorf("cursor error: %v", err)
	}

	return totalAmount, nil
}

// GetJumlahQRISLastWeek returns the total QRIS amount (in IDR) contributed by a user in the last week
func GetJumlahQRISLastWeek(db *mongo.Database, phoneNumber string) (float64, error) {
	// Calculate the date one week ago from now
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Create a filter to find successful QRIS payments from the specified phone number in the last week
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.QRIS,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	// Query the collection
	cursor, err := db.Collection("crowdfundingorders").Find(context.TODO(), filter)
	if err != nil {
		return 0, fmt.Errorf("error querying QRIS payments: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Calculate the total
	var totalAmount float64
	for cursor.Next(context.TODO()) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			return 0, fmt.Errorf("error decoding payment: %v", err)
		}
		totalAmount += payment.Amount
	}

	if err := cursor.Err(); err != nil {
		return 0, fmt.Errorf("cursor error: %v", err)
	}

	return totalAmount, nil
}

// GetTotalDataCrowdfundingMasuk retrieves all successful crowdfunding payments
func GetTotalDataCrowdfundingMasuk(db *mongo.Database, isDaily bool) ([]CrowdfundingInfo, error) {
	// Create the base filter for successful payments
	filter := bson.M{"status": "success"}

	// Add time filter if getting daily data
	if isDaily {
		// Get yesterday's date
		yesterday := time.Now().AddDate(0, 0, -1)
		startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		endOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, yesterday.Location())

		filter["timestamp"] = bson.M{
			"$gte": startOfYesterday,
			"$lte": endOfYesterday,
		}
	}

	// Query the collection with sorting by timestamp (newest first)
	opts := options.Find().SetSort(bson.M{"timestamp": -1})
	cursor, err := db.Collection("crowdfundingorders").Find(context.TODO(), filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error querying crowdfunding payments: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Extract payments
	var payments []CrowdfundingInfo
	for cursor.Next(context.TODO()) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			return nil, fmt.Errorf("error decoding payment: %v", err)
		}

		// Add to our slice
		payments = append(payments, CrowdfundingInfo{
			Name:          payment.Name,
			PhoneNumber:   payment.PhoneNumber,
			Amount:        payment.Amount,
			PaymentMethod: payment.PaymentMethod,
			Timestamp:     payment.Timestamp,
			Status:        payment.Status,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	// If we have payments, try to get their WA groups
	if len(payments) > 0 {
		// Extract unique phone numbers
		phoneNumbers := extractUniquePaymentPhoneNumbers(payments)

		// Get the WA Group IDs for these phone numbers
		groupMap, err := GetGroupIDFromProject(db, phoneNumbers)
		if err != nil {
			// Continue even if there's an error, just without group info
			fmt.Printf("Warning: Could not get group IDs: %v\n", err)
		} else {
			// Add group IDs to the payment info
			for i, payment := range payments {
				if groups, ok := groupMap[payment.PhoneNumber]; ok && len(groups) > 0 {
					payments[i].WaGroupID = groups[0] // Just use the first group
				}
			}
		}
	}

	return payments, nil
}

// Helper function to extract unique phone numbers from payment data
func extractUniquePaymentPhoneNumbers(payments []CrowdfundingInfo) []string {
	phoneSet := make(map[string]bool)
	var phoneNumbers []string

	for _, payment := range payments {
		if !phoneSet[payment.PhoneNumber] {
			phoneSet[payment.PhoneNumber] = true
			phoneNumbers = append(phoneNumbers, payment.PhoneNumber)
		}
	}
	return phoneNumbers
}

// GenerateRekapCrowdfundingDaily generates a daily crowdfunding recap message for a specific WhatsApp group
func GenerateRekapCrowdfundingDaily(db *mongo.Database, groupID string) (string, string, error) {
	// Get yesterday's crowdfunding data
	payments, err := GetTotalDataCrowdfundingMasuk(db, true)
	if err != nil {
		return "", "", fmt.Errorf("failed to get crowdfunding data: %v", err)
	}

	// Filter payments for the specified group
	var groupPayments []CrowdfundingInfo
	for _, payment := range payments {
		if payment.WaGroupID == groupID {
			groupPayments = append(groupPayments, payment)
		}
	}

	// If no payments for this group, return a message
	if len(groupPayments) == 0 {
		return "Tidak ada transaksi crowdfunding kemarin untuk grup ini.", "", nil
	}

	// Prepare the message
	msg := "*📊 Rekap Crowdfunding Harian 📊*\n\n"
	msg += "Berikut ini adalah ringkasan aktifitasi-crownfunding kemarin:\n\n"

	// Separate QRIS and MicroBitcoin payments
	var qrisPayments, mbcPayments []CrowdfundingInfo
	var totalQRIS, totalMBC float64

	for _, payment := range groupPayments {
		if payment.PaymentMethod == model.QRIS {
			qrisPayments = append(qrisPayments, payment)
			totalQRIS += payment.Amount
		} else if payment.PaymentMethod == model.MicroBitcoin {
			mbcPayments = append(mbcPayments, payment)
			totalMBC += payment.Amount
		}
	}

	// Add QRIS payments to the message
	if len(qrisPayments) > 0 {
		msg += "*QRIS Payments:*\n"
		for _, payment := range qrisPayments {
			msg += fmt.Sprintf("• %s: Rp %.2f\n", payment.Name, payment.Amount)
		}
		msg += fmt.Sprintf("Total QRIS: Rp %.2f\n\n", totalQRIS)
	}

	// Add MicroBitcoin payments to the message
	if len(mbcPayments) > 0 {
		msg += "*MicroBitcoin Payments:*\n"
		for _, payment := range mbcPayments {
			msg += fmt.Sprintf("• %s: %s\n", payment.Name, formatMBCAmount(payment.Amount))
		}
		msg += fmt.Sprintf("Total MBC: %s\n\n", formatMBCAmount(totalMBC))
	}

	// Add overall total
	msg += fmt.Sprintf("*Jumlah Transaksi:* %d\n", len(groupPayments))
	msg += fmt.Sprintf("*Total QRIS:* Rp %.2f\n", totalQRIS)
	msg += fmt.Sprintf("*Total MBC:* %s\n", formatMBCAmount(totalMBC))

	// Use first payment's phone number as representative phone
	perwakilanphone := ""
	if len(groupPayments) > 0 {
		perwakilanphone = groupPayments[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

// GenerateRekapCrowdfundingWeekly generates a weekly crowdfunding recap message for a specific WhatsApp group
func GenerateRekapCrowdfundingWeekly(db *mongo.Database, groupID string) (string, string, error) {
	// Calculate the date one week ago from now
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Create a filter for payments in the last week
	filter := bson.M{
		"status": "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	// Query the collection with sorting by timestamp (newest first)
	opts := options.Find().SetSort(bson.M{"timestamp": -1})
	cursor, err := db.Collection("crowdfundingorders").Find(context.TODO(), filter, opts)
	if err != nil {
		return "", "", fmt.Errorf("error querying crowdfunding payments: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Extract payments
	var payments []CrowdfundingInfo
	for cursor.Next(context.TODO()) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			return "", "", fmt.Errorf("error decoding payment: %v", err)
		}

		// Add to our slice
		payments = append(payments, CrowdfundingInfo{
			Name:          payment.Name,
			PhoneNumber:   payment.PhoneNumber,
			Amount:        payment.Amount,
			PaymentMethod: payment.PaymentMethod,
			Timestamp:     payment.Timestamp,
			Status:        payment.Status,
		})
	}

	if err := cursor.Err(); err != nil {
		return "", "", fmt.Errorf("cursor error: %v", err)
	}

	// If we have payments, get their WA groups
	if len(payments) > 0 {
		// Extract unique phone numbers
		phoneNumbers := extractUniquePaymentPhoneNumbers(payments)

		// Get the WA Group IDs for these phone numbers
		groupMap, err := GetGroupIDFromProject(db, phoneNumbers)
		if err != nil {
			// Continue even if there's an error, just without group info
			fmt.Printf("Warning: Could not get group IDs: %v\n", err)
		} else {
			// Add group IDs to the payment info
			for i, payment := range payments {
				if groups, ok := groupMap[payment.PhoneNumber]; ok && len(groups) > 0 {
					payments[i].WaGroupID = groups[0] // Just use the first group
				}
			}
		}
	}

	// Filter payments for the specified group
	var groupPayments []CrowdfundingInfo
	for _, payment := range payments {
		if payment.WaGroupID == groupID {
			groupPayments = append(groupPayments, payment)
		}
	}

	// If no payments for this group, return a message
	if len(groupPayments) == 0 {
		return "Tidak ada transaksi crowdfunding minggu ini untuk grup ini.", "", nil
	}

	// Prepare the message
	msg := "*📊 Rekap Crowdfunding Mingguan 📊*\n\n"
	msg += "Berikut ini adalah ringkasan aktifitasi-crownfunding selama seminggu terakhir:\n\n"

	// Group payments by user
	userPayments := make(map[string]struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	})

	var totalQRIS, totalMBC float64
	var totalQRISCount, totalMBCCount int

	for _, payment := range groupPayments {
		userInfo, exists := userPayments[payment.PhoneNumber]
		if !exists {
			userInfo = struct {
				Name         string
				PhoneNumber  string
				QRISAmount   float64
				QRISCount    int
				MBCAmount    float64
				MBCCount     int
				TotalPayment int
			}{
				Name:        payment.Name,
				PhoneNumber: payment.PhoneNumber,
			}
		}

		if payment.PaymentMethod == model.QRIS {
			userInfo.QRISAmount += payment.Amount
			userInfo.QRISCount++
			totalQRIS += payment.Amount
			totalQRISCount++
		} else if payment.PaymentMethod == model.MicroBitcoin {
			userInfo.MBCAmount += payment.Amount
			userInfo.MBCCount++
			totalMBC += payment.Amount
			totalMBCCount++
		}

		userInfo.TotalPayment++
		userPayments[payment.PhoneNumber] = userInfo
	}

	// Sort users by total payment
	type UserPaymentInfo struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	}

	var sortedUsers []UserPaymentInfo
	for _, info := range userPayments {
		sortedUsers = append(sortedUsers, UserPaymentInfo{
			Name:         info.Name,
			PhoneNumber:  info.PhoneNumber,
			QRISAmount:   info.QRISAmount,
			QRISCount:    info.QRISCount,
			MBCAmount:    info.MBCAmount,
			MBCCount:     info.MBCCount,
			TotalPayment: info.TotalPayment,
		})
	}

	// Sort by total payment count
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].TotalPayment > sortedUsers[j].TotalPayment
	})

	// Add user payments to the message
	for _, user := range sortedUsers {
		msg += fmt.Sprintf("*%s*\n", user.Name)
		if user.QRISCount > 0 {
			msg += fmt.Sprintf("- QRIS: Rp %.2f (%d transaksi)\n", user.QRISAmount, user.QRISCount)
		}
		if user.MBCCount > 0 {
			msg += fmt.Sprintf("- MBC: %s (%d transaksi)\n", formatMBCAmount(user.MBCAmount), user.MBCCount)
		}
		msg += fmt.Sprintf("- Total: %d transaksi\n\n", user.TotalPayment)
	}

	// Add overall total
	msg += "*RINGKASAN MINGGUAN*\n"
	msg += fmt.Sprintf("Jumlah crownfunding: %d\n", len(sortedUsers))
	msg += fmt.Sprintf("Total Transaksi: %d\n", len(groupPayments))
	msg += fmt.Sprintf("Total QRIS: Rp %.2f (%d transaksi)\n", totalQRIS, totalQRISCount)
	msg += fmt.Sprintf("Total MBC: %s (%d transaksi)\n", formatMBCAmount(totalMBC), totalMBCCount)

	// Use first payment's phone number as representative phone
	perwakilanphone := ""
	if len(sortedUsers) > 0 {
		perwakilanphone = sortedUsers[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

// GenerateRekapCrowdfundingAll generates a complete crowdfunding recap message for a specific WhatsApp group
func GenerateRekapCrowdfundingAll(db *mongo.Database, groupID string) (string, string, error) {
	// Get all crowdfunding data
	payments, err := GetTotalDataCrowdfundingMasuk(db, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to get crowdfunding data: %v", err)
	}

	// Filter payments for the specified group
	var groupPayments []CrowdfundingInfo
	for _, payment := range payments {
		if payment.WaGroupID == groupID {
			groupPayments = append(groupPayments, payment)
		}
	}

	// If no payments for this group, return a message
	if len(groupPayments) == 0 {
		return "Belum ada transaksi crowdfunding untuk grup ini.", "", nil
	}

	// Prepare the message
	msg := "*📊 Rekap Total Crowdfunding 📊*\n\n"
	msg += "Berikut ini adalah ringkasan seluruh aktifitasi-crownfunding:\n\n"

	// Group payments by user
	userPayments := make(map[string]struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	})

	var totalQRIS, totalMBC float64
	var totalQRISCount, totalMBCCount int

	for _, payment := range groupPayments {
		userInfo, exists := userPayments[payment.PhoneNumber]
		if !exists {
			userInfo = struct {
				Name         string
				PhoneNumber  string
				QRISAmount   float64
				QRISCount    int
				MBCAmount    float64
				MBCCount     int
				TotalPayment int
			}{
				Name:        payment.Name,
				PhoneNumber: payment.PhoneNumber,
			}
		}

		if payment.PaymentMethod == model.QRIS {
			userInfo.QRISAmount += payment.Amount
			userInfo.QRISCount++
			totalQRIS += payment.Amount
			totalQRISCount++
		} else if payment.PaymentMethod == model.MicroBitcoin {
			userInfo.MBCAmount += payment.Amount
			userInfo.MBCCount++
			totalMBC += payment.Amount
			totalMBCCount++
		}

		userInfo.TotalPayment++
		userPayments[payment.PhoneNumber] = userInfo
	}

	// Sort users by total payment
	type UserPaymentInfo struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	}

	var sortedUsers []UserPaymentInfo
	for _, info := range userPayments {
		sortedUsers = append(sortedUsers, UserPaymentInfo{
			Name:         info.Name,
			PhoneNumber:  info.PhoneNumber,
			QRISAmount:   info.QRISAmount,
			QRISCount:    info.QRISCount,
			MBCAmount:    info.MBCAmount,
			MBCCount:     info.MBCCount,
			TotalPayment: info.TotalPayment,
		})
	}

	// Sort by total payment amount (QRIS + MBC if we had a conversion rate)
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].TotalPayment > sortedUsers[j].TotalPayment
	})

	// List all users with their donation details
	msg += "*DAFTAR SEMUA crownfunding*\n\n"
	for i, user := range sortedUsers {
		msg += fmt.Sprintf("%d. *%s* (%s)\n", i+1, user.Name, user.PhoneNumber)
		if user.QRISCount > 0 {
			msg += fmt.Sprintf("   - QRIS: Rp %.2f (%d transaksi)\n", user.QRISAmount, user.QRISCount)
		}
		if user.MBCCount > 0 {
			msg += fmt.Sprintf("   - MBC: %s (%d transaksi)\n", formatMBCAmount(user.MBCAmount), user.MBCCount)
		}
		msg += fmt.Sprintf("   - Total: %d transaksi\n", user.TotalPayment)
	}

	// Add overall total
	msg += "\n*STATISTIK KESELURUHAN*\n"
	msg += fmt.Sprintf("Jumlah crownfunding: %d\n", len(sortedUsers))
	msg += fmt.Sprintf("Total Transaksi: %d\n", len(groupPayments))
	msg += fmt.Sprintf("Total QRIS: Rp %.2f (%d transaksi)\n", totalQRIS, totalQRISCount)
	msg += fmt.Sprintf("Total MBC: %s (%d transaksi)\n", formatMBCAmount(totalMBC), totalMBCCount)

	// Use first payment's phone number as representative phone
	perwakilanphone := ""
	if len(sortedUsers) > 0 {
		perwakilanphone = sortedUsers[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

// RekapCrowdfundingHarian sends daily crowdfunding recap to specified WhatsApp groups
func RekapCrowdfundingHarian(db *mongo.Database) error {
	// List of allowed group IDs
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	var lastErr error

	for _, groupID := range allowedGroups {
		// Generate daily report for this group
		msg, perwakilanphone, err := GenerateRekapCrowdfundingDaily(db, groupID)
		if err != nil {
			lastErr = fmt.Errorf("failed to generate daily report for group %s: %v", groupID, err)
			continue
		}

		// If no data, skip this group
		if strings.Contains(msg, "Tidak ada transaksi") {
			continue
		}

		// Prepare message to send
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// If it's a private chat (contains hyphen), send to representative
		if strings.Contains(groupID, "-") {
			if perwakilanphone == "" {
				continue // Skip if no representative found
			}
			dt.To = perwakilanphone
			dt.IsGroup = false
		}

		// Send the message
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("failed to send daily report to group %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	return lastErr
}

// RekapCrowdfundingMingguan sends weekly crowdfunding recap to specified WhatsApp groups
func RekapCrowdfundingMingguan(db *mongo.Database) error {
	// List of allowed group IDs
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	var lastErr error

	for _, groupID := range allowedGroups {
		// Generate weekly report for this group
		msg, perwakilanphone, err := GenerateRekapCrowdfundingWeekly(db, groupID)
		if err != nil {
			lastErr = fmt.Errorf("failed to generate weekly report for group %s: %v", groupID, err)
			continue
		}

		// If no data, skip this group
		if strings.Contains(msg, "Tidak ada transaksi") {
			continue
		}

		// Prepare message to send
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// If it's a private chat (contains hyphen), send to representative
		if strings.Contains(groupID, "-") {
			if perwakilanphone == "" {
				continue // Skip if no representative found
			}
			dt.To = perwakilanphone
			dt.IsGroup = false
		}

		// Send the message
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("failed to send weekly report to group %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	return lastErr
}

// RekapCrowdfundingTotal sends total crowdfunding recap to specified WhatsApp groups
func RekapCrowdfundingTotal(db *mongo.Database) error {
	// List of allowed group IDs
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	var lastErr error

	for _, groupID := range allowedGroups {
		// Generate total report for this group
		msg, perwakilanphone, err := GenerateRekapCrowdfundingAll(db, groupID)
		if err != nil {
			lastErr = fmt.Errorf("failed to generate total report for group %s: %v", groupID, err)
			continue
		}

		// If no data, skip this group
		if strings.Contains(msg, "Belum ada transaksi") {
			continue
		}

		// Prepare message to send
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// If it's a private chat (contains hyphen), send to representative
		if strings.Contains(groupID, "-") {
			if perwakilanphone == "" {
				continue // Skip if no representative found
			}
			dt.To = perwakilanphone
			dt.IsGroup = false
		}

		// Send the message
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("failed to send total report to group %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	return lastErr
}

// GenerateRekapCrowdfundingGlobal generates a complete global crowdfunding recap of all users
func GenerateRekapCrowdfundingGlobal(db *mongo.Database) (string, string, error) {
	// Get all successful crowdfunding data without daily filter
	filter := bson.M{"status": "success"}

	// Query the collection with sorting by timestamp (newest first)
	opts := options.Find().SetSort(bson.M{"timestamp": -1})
	cursor, err := db.Collection("crowdfundingorders").Find(context.TODO(), filter, opts)
	if err != nil {
		return "", "", fmt.Errorf("error querying crowdfunding payments: %v", err)
	}
	defer cursor.Close(context.TODO())

	// Extract payments
	var payments []CrowdfundingInfo
	for cursor.Next(context.TODO()) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			return "", "", fmt.Errorf("error decoding payment: %v", err)
		}

		// Add to our slice
		payments = append(payments, CrowdfundingInfo{
			Name:          payment.Name,
			PhoneNumber:   payment.PhoneNumber,
			Amount:        payment.Amount,
			PaymentMethod: payment.PaymentMethod,
			Timestamp:     payment.Timestamp,
			Status:        payment.Status,
		})
	}

	if err := cursor.Err(); err != nil {
		return "", "", fmt.Errorf("cursor error: %v", err)
	}

	// If no payments, return a message
	if len(payments) == 0 {
		return "Belum ada transaksi crowdfunding yang tercatat.", "", nil
	}

	// Prepare the message global
	msg := "*📊 Rekap Total Crowdfunding 📊*\n\n"
	msg += "Berikut ini adalah rekap seluruh transaksi crowdfunding dari semua pengguna:\n\n"

	// Group payments by user and payment method
	userPayments := make(map[string]struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	})

	var totalQRIS, totalMBC float64
	var totalQRISCount, totalMBCCount int

	for _, payment := range payments {
		userInfo, exists := userPayments[payment.PhoneNumber]
		if !exists {
			userInfo = struct {
				Name         string
				PhoneNumber  string
				QRISAmount   float64
				QRISCount    int
				MBCAmount    float64
				MBCCount     int
				TotalPayment int
			}{
				Name:        payment.Name,
				PhoneNumber: payment.PhoneNumber,
			}
		}

		if payment.PaymentMethod == model.QRIS {
			userInfo.QRISAmount += payment.Amount
			userInfo.QRISCount++
			totalQRIS += payment.Amount
			totalQRISCount++
		} else if payment.PaymentMethod == model.MicroBitcoin {
			userInfo.MBCAmount += payment.Amount
			userInfo.MBCCount++
			totalMBC += payment.Amount
			totalMBCCount++
		}

		userInfo.TotalPayment++
		userPayments[payment.PhoneNumber] = userInfo
	}

	// Sort users by total payment
	type UserPaymentInfo struct {
		Name         string
		PhoneNumber  string
		QRISAmount   float64
		QRISCount    int
		MBCAmount    float64
		MBCCount     int
		TotalPayment int
	}

	var sortedMBCUsers []UserPaymentInfo
	var sortedQRISUsers []UserPaymentInfo

	for _, info := range userPayments {
		userInfo := UserPaymentInfo{
			Name:         info.Name,
			PhoneNumber:  info.PhoneNumber,
			QRISAmount:   info.QRISAmount,
			QRISCount:    info.QRISCount,
			MBCAmount:    info.MBCAmount,
			MBCCount:     info.MBCCount,
			TotalPayment: info.TotalPayment,
		}

		// Add to MBC list if they have MBC transactions
		if info.MBCCount > 0 {
			sortedMBCUsers = append(sortedMBCUsers, userInfo)
		}

		// Add to QRIS list if they have QRIS transactions
		if info.QRISCount > 0 {
			sortedQRISUsers = append(sortedQRISUsers, userInfo)
		}
	}

	// Sort MBC users by MBC amount (highest first)
	sort.Slice(sortedMBCUsers, func(i, j int) bool {
		return sortedMBCUsers[i].MBCAmount > sortedMBCUsers[j].MBCAmount
	})

	// Sort QRIS users by QRIS amount (highest first)
	sort.Slice(sortedQRISUsers, func(i, j int) bool {
		return sortedQRISUsers[i].QRISAmount > sortedQRISUsers[j].QRISAmount
	})

	// First list all MBC users
	if len(sortedMBCUsers) > 0 {
		msg += "*DAFTAR aktifitasi-crownfunding MBC*\n\n"
		for i, user := range sortedMBCUsers {
			msg += fmt.Sprintf("%d. *%s* (%s)\n", i+1, user.Name, user.PhoneNumber)
			msg += fmt.Sprintf("   - MBC: %s (%d transaksi)\n", formatMBCAmount(user.MBCAmount), user.MBCCount)
		}
		msg += "\n"
	}

	// Then list all QRIS users
	if len(sortedQRISUsers) > 0 {
		msg += "*DAFTAR aktifitasi-crownfunding QRIS*\n\n"
		for i, user := range sortedQRISUsers {
			msg += fmt.Sprintf("%d. *%s* (%s)\n", i+1, user.Name, user.PhoneNumber)
			msg += fmt.Sprintf("   - QRIS: Rp %.2f (%d transaksi)\n", user.QRISAmount, user.QRISCount)
		}
		msg += "\n"
	}

	// Add overall total stats
	msg += "*STATISTIK KESELURUHAN*\n"
	msg += fmt.Sprintf("Total Pengguna: %d\n", len(userPayments))
	msg += fmt.Sprintf("Total Transaksi: %d\n", len(payments))
	msg += fmt.Sprintf("Total QRIS: Rp %.2f (%d transaksi)\n", totalQRIS, totalQRISCount)
	msg += fmt.Sprintf("Total MBC: %s (%d transaksi)\n", formatMBCAmount(totalMBC), totalMBCCount)

	// Use first payment's phone number as representative phone
	perwakilanphone := ""
	if len(payments) > 0 {
		perwakilanphone = payments[0].PhoneNumber
	}

	return msg, perwakilanphone, nil
}

// RekapCrowdfundingGlobal sends a global crowdfunding recap to specified WhatsApp groups
func RekapCrowdfundingGlobal(db *mongo.Database) error {
	// List of allowed group IDs
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	// Generate the global report
	msg, perwakilanphone, err := GenerateRekapCrowdfundingGlobal(db)
	if err != nil {
		return fmt.Errorf("failed to generate global report: %v", err)
	}

	// If no data, return
	if strings.Contains(msg, "Belum ada transaksi") {
		return fmt.Errorf("no crowdfunding transactions found")
	}

	var lastErr error

	// Send the same report to all groups
	for _, groupID := range allowedGroups {
		// Prepare message to send
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// If it's a private chat (contains hyphen), send to representative
		if strings.Contains(groupID, "-") {
			if perwakilanphone == "" {
				continue // Skip if no representative found
			}
			dt.To = perwakilanphone
			dt.IsGroup = false
		}

		// Send the message
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("failed to send global report to group %s: %v, info: %s", groupID, err, resp.Info)
			continue
		}
	}

	return lastErr
}
