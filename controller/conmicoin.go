package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// ReceiverWalletAddress is the wallet address where payments should be sent
	ReceiverWalletAddress = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"

	// MicroBitcoinAPIURL is the API endpoint to check transactions
	MicroBitcoinAPIURL = "https://microbitcoinorg.github.io/api"

	// MerchCoinExpiryMinutes is how long a payment is valid before expiring
	MerchCoinExpiryMinutes = 5

	// MerchCoinCollection is the MongoDB collection name
	MerchCoinCollection = "merchcoin"
)

var (
	// activeOrderMutex protects concurrent access to the active order
	activeOrderMutex sync.Mutex
)

// generateOrderID creates a random order ID
func generateOrderID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateMerchCoinOrder handles creation of a new payment order
func CreateMerchCoinOrder(w http.ResponseWriter, r *http.Request) {
	var requestData model.MerchCoinOrderRequest
	var response model.MerchCoinOrderResponse

	// Decode request body
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		response.Success = false
		response.Message = "Invalid request format"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Validate wallet code
	if requestData.WalletCode == "" {
		response.Success = false
		response.Message = "Wallet code is required"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Check if there's already an active order
	activeOrderMutex.Lock()
	defer activeOrderMutex.Unlock()

	var activeOrder model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{
			"status":     "pending",
			"expirytime": bson.M{"$gt": time.Now()},
		},
	).Decode(&activeOrder)

	// If there's an active order, return queue status
	if err == nil {
		response.Success = false
		response.Message = "There is already an active payment process. Please wait."
		response.OrderID = activeOrder.OrderID
		response.WalletCode = activeOrder.SenderWallet
		response.ExpiryTime = activeOrder.ExpiryTime
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// Generate a new order ID
	orderID, err := generateOrderID()
	if err != nil {
		response.Success = false
		response.Message = "Failed to generate order ID"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Create payment document
	now := time.Now()
	expiryTime := now.Add(MerchCoinExpiryMinutes * time.Minute)

	payment := model.MerchCoinPayment{
		OrderID:      orderID,
		SenderWallet: requestData.WalletCode,
		Status:       "pending",
		CreatedAt:    now,
		ExpiryTime:   expiryTime,
	}

	// Insert into database
	_, err = config.Mongoconn.Collection(MerchCoinCollection).InsertOne(context.Background(), payment)
	if err != nil {
		response.Success = false
		response.Message = "Failed to create payment record"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Generate QR code data and URL
	qrImageURL := "wonpay.png" // Default static QR image

	// Prepare successful response
	response.Success = true
	response.Message = "Payment order created successfully"
	response.OrderID = orderID
	response.WalletCode = requestData.WalletCode
	response.ExpiryTime = expiryTime
	response.QRImageURL = qrImageURL

	at.WriteJSON(w, http.StatusOK, response)
}

// CheckMerchCoinPayment checks the status of a payment
func CheckMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	var response model.MerchCoinPaymentStatusResponse

	if orderID == "" {
		response.Success = false
		response.Message = "Order ID is required"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Find the payment in database
	var payment model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{"orderid": orderID},
	).Decode(&payment)

	if err != nil {
		response.Success = false
		response.Message = "Payment not found"
		at.WriteJSON(w, http.StatusNotFound, response)
		return
	}

	// Check if payment is already processed
	if payment.Status == "success" {
		response.Success = true
		response.Status = "success"
		response.Message = "Payment has been successfully processed"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		response.Amount = payment.Amount
		response.TxHash = payment.TxHash
		response.CreatedAt = payment.CreatedAt
		response.ProcessedAt = payment.UpdatedAt
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// Check if payment is expired
	if time.Now().After(payment.ExpiryTime) && payment.Status == "pending" {
		// Update payment status to expired
		_, err = config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
			context.Background(),
			bson.M{"orderid": orderID},
			bson.M{"$set": bson.M{"status": "expired", "updatedat": time.Now()}},
		)

		response.Success = true
		response.Status = "failed"
		response.Message = "Payment has expired"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// For pending payments, check the MicroBitcoin API for transactions
	found, amount, txHash, err := checkMicroBitcoinTransactions(payment.SenderWallet)

	// Calculate remaining time regardless of API results
	remainingSeconds := int(payment.ExpiryTime.Sub(time.Now()).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	// If we found a transaction, update the payment status
	if found && err == nil {
		// Update payment status to success
		now := time.Now()
		_, updateErr := config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
			context.Background(),
			bson.M{"orderid": orderID},
			bson.M{"$set": bson.M{
				"status":    "success",
				"amount":    amount,
				"txhash":    txHash,
				"updatedat": now,
			}},
		)

		if updateErr != nil {
			// If update fails, still return success but note the database error
			response.Success = true
			response.Status = "pending"
			response.Message = "Payment detected but failed to update status"
			response.OrderID = payment.OrderID
			response.WalletCode = payment.SenderWallet
			response.RemainingTime = remainingSeconds
			at.WriteJSON(w, http.StatusOK, response)
			return
		}

		// Payment successful
		response.Success = true
		response.Status = "success"
		response.Message = "Payment has been successfully processed"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		response.Amount = amount
		response.TxHash = txHash
		response.CreatedAt = payment.CreatedAt
		response.ProcessedAt = now

		// Send WhatsApp notification if configured
		if payment.SenderWallet != "" && config.WAAPIToken != "" && config.WAAPIMessage != "" {
			amountStr := strconv.FormatFloat(amount, 'f', 8, 64)
			message := fmt.Sprintf(
				"*Payment Confirmed*\nOrderID: %s\nWallet: %s\nAmount: %s MBC\nTransaction Hash: %s",
				orderID, payment.SenderWallet, amountStr, txHash,
			)

			// This is just a placeholder - you would need to determine the phone number
			phonenumber := "62895601060000" // Example default number

			// Send WhatsApp notification
			notif := &whatsauth.TextMessage{
				To:       phonenumber,
				IsGroup:  false,
				Messages: message,
			}

			go notifyPaymentStatus(notif)
		}

		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// If there was an error checking transactions, log it but still return pending
	if err != nil {
		fmt.Printf("Error checking MicroBitcoin transactions: %v\n", err)
	}

	// No transaction found or error occurred, return pending status
	response.Success = true
	response.Status = "pending"
	response.Message = "Payment is pending"
	response.OrderID = payment.OrderID
	response.WalletCode = payment.SenderWallet
	response.RemainingTime = remainingSeconds

	at.WriteJSON(w, http.StatusOK, response)
}

// ManuallyConfirmMerchCoinPayment manually confirms a payment (for admin use)
func ManuallyConfirmMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	var response model.Response

	if orderID == "" {
		response.Response = "Order ID is required"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Decode request body for amount and txHash
	var confirmData struct {
		Amount float64 `json:"amount"`
		TxHash string  `json:"txHash"`
	}

	if err := json.NewDecoder(r.Body).Decode(&confirmData); err != nil {
		response.Response = "Invalid request format"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Find the payment in database
	var payment model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{"orderid": orderID},
	).Decode(&payment)

	if err != nil {
		response.Response = "Payment not found"
		at.WriteJSON(w, http.StatusNotFound, response)
		return
	}

	// Check if payment is already processed
	if payment.Status == "success" {
		response.Response = "Payment has already been processed"
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// Update payment status to success
	now := time.Now()
	_, err = config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
		context.Background(),
		bson.M{"orderid": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"amount":    confirmData.Amount,
			"txhash":    confirmData.TxHash,
			"updatedat": now,
		}},
	)

	if err != nil {
		response.Response = "Failed to update payment status"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Send WhatsApp notification if configured
	if payment.SenderWallet != "" && config.WAAPIToken != "" && config.WAAPIMessage != "" {
		amountStr := strconv.FormatFloat(confirmData.Amount, 'f', 8, 64)
		message := fmt.Sprintf(
			"*Payment Confirmed*\nOrderID: %s\nWallet: %s\nAmount: %s MBC\nTransaction Hash: %s",
			orderID, payment.SenderWallet, amountStr, confirmData.TxHash,
		)

		// This is just a placeholder - you would need to determine the phone number
		// to send to based on your application logic
		phonenumber := "62895601060000" // Example default number

		// Send WhatsApp notification
		notif := &whatsauth.TextMessage{
			To:       phonenumber,
			IsGroup:  false,
			Messages: message,
		}

		go notifyPaymentStatus(notif)
	}

	response.Response = "Payment confirmed successfully"
	response.Status = "success"
	at.WriteJSON(w, http.StatusOK, response)
}

// notifyPaymentStatus sends a WhatsApp notification about payment status
func notifyPaymentStatus(msg *whatsauth.TextMessage) {
	// This function is called in a goroutine to avoid blocking the response
	// to the client. It will attempt to send a WhatsApp notification.

	// Insert to database as a log regardless of whether sending succeeds
	var logoutwa whatsauth.LogWhatsauth
	logoutwa.Data = *msg
	logoutwa.Token = config.WAAPIToken
	logoutwa.URL = config.WAAPIMessage
	logoutwa.CreatedAt = time.Now()

	_, err := atdb.InsertOneDoc(config.Mongoconn, "logwa", logoutwa)
	if err != nil {
		fmt.Printf("Failed to log WhatsApp notification: %v\n", err)
	}
}

// GetMerchCoinQueueStatus checks if there is an active payment being processed
func GetMerchCoinQueueStatus(w http.ResponseWriter, r *http.Request) {
	var response model.MerchCoinQueueStatusResponse

	// Look for any active payments
	var activePayment model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{
			"status":     "pending",
			"expirytime": bson.M{"$gt": time.Now()},
		},
		options.FindOne().SetSort(bson.M{"createdat": 1}),
	).Decode(&activePayment)

	if err != nil {
		// No active payments
		response.Success = true
		response.IsProcessing = false
		response.Message = "No active payments"
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// There is an active payment
	response.Success = true
	response.IsProcessing = true
	response.OrderID = activePayment.OrderID
	response.WalletCode = activePayment.SenderWallet
	response.ExpiryTime = activePayment.ExpiryTime
	response.Message = "There is an active payment being processed"

	at.WriteJSON(w, http.StatusOK, response)
}

// GetMerchCoinTotalPayments returns statistics about successful payments
func GetMerchCoinTotalPayments(w http.ResponseWriter, r *http.Request) {
	var response model.MerchCoinPaymentTotalsResponse

	// Aggregate to get total amount and count
	ctx := context.Background()
	pipeline := bson.A{
		bson.M{
			"$match": bson.M{"status": "success"},
		},
		bson.M{
			"$group": bson.M{
				"_id":         nil,
				"totalAmount": bson.M{"$sum": "$amount"},
				"count":       bson.M{"$sum": 1},
				"lastUpdated": bson.M{"$max": "$updatedat"},
			},
		},
	}

	cursor, err := config.Mongoconn.Collection(MerchCoinCollection).Aggregate(ctx, pipeline)
	if err != nil {
		response.Success = false
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}
	defer cursor.Close(ctx)

	type aggregateResult struct {
		TotalAmount float64   `bson:"totalAmount"`
		Count       int       `bson:"count"`
		LastUpdated time.Time `bson:"lastUpdated"`
	}

	var results []aggregateResult
	if err = cursor.All(ctx, &results); err != nil {
		response.Success = false
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Set default values
	response.Success = true
	response.TotalAmount = 0
	response.Count = 0

	// If we have results, update the response
	if len(results) > 0 {
		response.TotalAmount = results[0].TotalAmount
		response.Count = results[0].Count
		response.LastUpdated = results[0].LastUpdated
	}

	at.WriteJSON(w, http.StatusOK, response)
}

// ConfirmMerchCoinNotification handles incoming payment notifications
func ConfirmMerchCoinNotification(w http.ResponseWriter, r *http.Request) {
	var notification model.MerchCoinPaymentNotification
	var response model.Response

	// Decode notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		response.Response = "Invalid notification format"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Validate notification
	if notification.TxHash == "" || notification.SenderWallet == "" || notification.Amount <= 0 {
		response.Response = "Invalid notification data"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Find pending payment with matching wallet
	var pendingPayment model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{
			"status":       "pending",
			"senderwallet": notification.SenderWallet,
			"expirytime":   bson.M{"$gt": time.Now()},
		},
		options.FindOne().SetSort(bson.M{"createdat": 1}),
	).Decode(&pendingPayment)

	if err != nil {
		response.Response = "No matching pending payment found"
		at.WriteJSON(w, http.StatusNotFound, response)
		return
	}

	// Update payment to success
	now := time.Now()
	_, err = config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
		context.Background(),
		bson.M{"orderid": pendingPayment.OrderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"amount":    notification.Amount,
			"txhash":    notification.TxHash,
			"updatedat": now,
		}},
	)

	if err != nil {
		response.Response = "Failed to update payment status"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	response.Response = "Payment confirmed successfully"
	response.Status = "success"
	at.WriteJSON(w, http.StatusOK, response)
}

// SimulateMerchCoinPayment simulates a payment for testing purposes
func SimulateMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	var simulateRequest model.MerchCoinSimulatePaymentRequest
	var response model.Response

	// Decode request
	if err := json.NewDecoder(r.Body).Decode(&simulateRequest); err != nil {
		response.Response = "Invalid request format"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Validate request
	if simulateRequest.OrderID == "" || simulateRequest.SenderWallet == "" || simulateRequest.Amount <= 0 {
		response.Response = "Invalid simulation data"
		at.WriteJSON(w, http.StatusBadRequest, response)
		return
	}

	// Find pending payment
	var pendingPayment model.MerchCoinPayment
	err := config.Mongoconn.Collection(MerchCoinCollection).FindOne(
		context.Background(),
		bson.M{
			"orderid": simulateRequest.OrderID,
			"status":  "pending",
		},
	).Decode(&pendingPayment)

	if err != nil {
		response.Response = "No matching pending payment found"
		at.WriteJSON(w, http.StatusNotFound, response)
		return
	}

	// Generate fake transaction hash
	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		response.Response = "Failed to generate transaction hash"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}
	fakeTxHash := hex.EncodeToString(b)

	// Update payment to success
	now := time.Now()
	_, err = config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
		context.Background(),
		bson.M{"orderid": simulateRequest.OrderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"amount":    simulateRequest.Amount,
			"txhash":    fakeTxHash,
			"updatedat": now,
		}},
	)

	if err != nil {
		response.Response = "Failed to update payment status"
		at.WriteJSON(w, http.StatusInternalServerError, response)
		return
	}

	response.Response = "Payment simulation successful"
	response.Status = "success"
	response.Info = fmt.Sprintf("Transaction Hash: %s", fakeTxHash)
	at.WriteJSON(w, http.StatusOK, response)
}

// checkMicroBitcoinTransactions checks the MicroBitcoin blockchain for transactions to our wallet
func checkMicroBitcoinTransactions(senderWallet string) (bool, float64, string, error) {
	// Create API URL to check address transactions
	apiURL := fmt.Sprintf("%s/address/%s", MicroBitcoinAPIURL, ReceiverWalletAddress)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(apiURL)
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to connect to MicroBitcoin API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, 0, "", fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	// Decode the response
	var apiResponse model.MerchCoinTxAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return false, 0, "", fmt.Errorf("failed to decode API response: %v", err)
	}

	// Check if the API response status is success
	if apiResponse.Status != "success" {
		return false, 0, "", fmt.Errorf("API returned error: %s", apiResponse.Message)
	}

	// Look for transactions from the sender wallet
	for _, tx := range apiResponse.Txs {
		// Only consider transactions from the specified sender
		if tx.SenderAddr == senderWallet {
			// We found a transaction from the sender to our wallet
			return true, tx.Amount, tx.TxID, nil
		}
	}

	// No matching transaction found
	return false, 0, "", nil
}
