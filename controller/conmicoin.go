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
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// ReceiverWalletAddress is the wallet address where payments should be sent
	ReceiverWalletAddress = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"

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
	qrImageURL := "static/wonpay.png" // Default static QR image

	// Prepare successful response
	response.Success = true
	response.Message = "Payment order created successfully"
	response.OrderID = orderID
	response.WalletCode = requestData.WalletCode
	response.ExpiryTime = expiryTime
	response.QRImageURL = qrImageURL

	at.WriteJSON(w, http.StatusOK, response)
}

// CheckMerchCoinPayment checks the status of a payment with step-by-step verification
func CheckMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	var response model.MerchCoinPaymentStatusResponse

	// Get the verification step from query params, default to 0 (check all)
	step := r.URL.Query().Get("step")
	stepInt := 0
	if step != "" {
		var err error
		stepInt, err = strconv.Atoi(step)
		if err != nil {
			stepInt = 0
		}
	}

	// Get txHash from query params if provided (for step 3)
	txHash := r.URL.Query().Get("txHash")

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
		response.Status = "expired"
		response.Message = "Payment has expired"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// Calculate remaining time regardless of verification step
	remainingSeconds := int(time.Until(payment.ExpiryTime).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	response.RemainingTime = remainingSeconds

	// Process based on verification step
	if stepInt == 1 {
		// Step 1: Check mempool for transactions
		foundInMempool, txHash, err := checkMempoolForTransactions(payment.SenderWallet)
		if err != nil {
			fmt.Printf("Error checking mempool: %v\n", err)
		}

		response.Success = true
		response.Status = "pending"
		response.Message = "Checking mempool for transactions"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		response.FoundInMempool = foundInMempool

		if foundInMempool {
			response.TxHash = txHash
		}

		at.WriteJSON(w, http.StatusOK, response)
		return
	} else if stepInt == 2 {
		// Step 2: Check blockchain history for transactions
		foundInHistory, txHash, err := checkBlockchainHistory(payment.SenderWallet)
		if err != nil {
			fmt.Printf("Error checking blockchain history: %v\n", err)
		}

		response.Success = true
		response.Status = "pending"
		response.Message = "Checking blockchain history for transactions"
		response.OrderID = payment.OrderID
		response.WalletCode = payment.SenderWallet
		response.FoundInHistory = foundInHistory

		if foundInHistory {
			response.TxHash = txHash
		}

		at.WriteJSON(w, http.StatusOK, response)
		return
	} else if stepInt == 3 && txHash != "" {
		// Step 3: Verify transaction details
		verified, amount, err := verifyTransactionDetails(txHash, payment.SenderWallet)
		if err != nil {
			fmt.Printf("Error verifying transaction details: %v\n", err)
			response.Success = false
			response.Status = "pending"
			response.Message = fmt.Sprintf("Error verifying transaction details: %v", err)
			at.WriteJSON(w, http.StatusOK, response)
			return
		}

		if verified {
			// Transaction verified, update payment status
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
				fmt.Printf("Error updating payment: %v\n", updateErr)
				response.Success = false
				response.Status = "pending"
				response.Message = "Transaction verified but failed to update payment status"
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
		} else {
			response.Success = false
			response.Status = "pending"
			response.Message = "Transaction verification failed"
		}

		at.WriteJSON(w, http.StatusOK, response)
		return
	}

	// If no specific step is requested, check the transaction through all steps
	foundInMempool, mempoolTxHash, _ := checkMempoolForTransactions(payment.SenderWallet)
	if foundInMempool {
		// Found in mempool, verify transaction details
		verified, amount, err := verifyTransactionDetails(mempoolTxHash, payment.SenderWallet)
		if err == nil && verified {
			// Transaction verified, update payment status
			now := time.Now()
			_, updateErr := config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
				context.Background(),
				bson.M{"orderid": orderID},
				bson.M{"$set": bson.M{
					"status":    "success",
					"amount":    amount,
					"txhash":    mempoolTxHash,
					"updatedat": now,
				}},
			)

			if updateErr == nil {
				// Payment successful
				response.Success = true
				response.Status = "success"
				response.Message = "Payment has been successfully processed"
				response.OrderID = payment.OrderID
				response.WalletCode = payment.SenderWallet
				response.Amount = amount
				response.TxHash = mempoolTxHash
				response.CreatedAt = payment.CreatedAt
				response.ProcessedAt = now

				at.WriteJSON(w, http.StatusOK, response)
				return
			}
		}
	}

	// Check blockchain history for transactions
	foundInHistory, historyTxHash, _ := checkBlockchainHistory(payment.SenderWallet)
	if foundInHistory {
		// Found in history, verify transaction details
		verified, amount, err := verifyTransactionDetails(historyTxHash, payment.SenderWallet)
		if err == nil && verified {
			// Transaction verified, update payment status
			now := time.Now()
			_, updateErr := config.Mongoconn.Collection(MerchCoinCollection).UpdateOne(
				context.Background(),
				bson.M{"orderid": orderID},
				bson.M{"$set": bson.M{
					"status":    "success",
					"amount":    amount,
					"txhash":    historyTxHash,
					"updatedat": now,
				}},
			)

			if updateErr == nil {
				// Payment successful
				response.Success = true
				response.Status = "success"
				response.Message = "Payment has been successfully processed"
				response.OrderID = payment.OrderID
				response.WalletCode = payment.SenderWallet
				response.Amount = amount
				response.TxHash = historyTxHash
				response.CreatedAt = payment.CreatedAt
				response.ProcessedAt = now

				at.WriteJSON(w, http.StatusOK, response)
				return
			}
		}
	}

	// If we reach here, payment is still pending
	response.Success = true
	response.Status = "pending"
	response.Message = "Payment is pending. Waiting for transaction confirmation."
	response.OrderID = payment.OrderID
	response.WalletCode = payment.SenderWallet
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

	response.Response = "Payment confirmed successfully"
	response.Status = "success"
	at.WriteJSON(w, http.StatusOK, response)
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

// checkMempoolForTransactions checks the mempool for unconfirmed transactions
func checkMempoolForTransactions(senderWallet string) (bool, string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create API URL to check mempool
	mempoolURL := fmt.Sprintf("https://api.mbc.wiki/mempool/%s", ReceiverWalletAddress)

	// Make the request
	resp, err := client.Get(mempoolURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to connect to MicroBitcoin mempool API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("mempool API returned error status: %d", resp.StatusCode)
	}

	// Decode the response
	var mempoolResponse struct {
		Error  interface{} `json:"error"`
		ID     string      `json:"id"`
		Result struct {
			Tx      []string `json:"tx"`
			TxCount int      `json:"txcount"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&mempoolResponse); err != nil {
		return false, "", fmt.Errorf("failed to decode mempool API response: %v", err)
	}

	// Check if there are transactions in the mempool
	if mempoolResponse.Result.TxCount > 0 {
		// Return the first transaction hash
		if len(mempoolResponse.Result.Tx) > 0 {
			return true, mempoolResponse.Result.Tx[0], nil
		}
	}

	// No transactions found in mempool
	return false, "", nil
}

// checkBlockchainHistory checks the blockchain transaction history
func checkBlockchainHistory(senderWallet string) (bool, string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create API URL to check blockchain history
	historyURL := fmt.Sprintf("https://api.mbc.wiki/history/%s", ReceiverWalletAddress)

	// Make the request
	resp, err := client.Get(historyURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to connect to MicroBitcoin history API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("history API returned error status: %d", resp.StatusCode)
	}

	// Decode the response
	var historyResponse struct {
		Error  interface{} `json:"error"`
		ID     string      `json:"id"`
		Result struct {
			Tx      []string `json:"tx"`
			TxCount int      `json:"txcount"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&historyResponse); err != nil {
		return false, "", fmt.Errorf("failed to decode history API response: %v", err)
	}

	// Check if there are transactions in the history
	if historyResponse.Result.TxCount > 0 && len(historyResponse.Result.Tx) > 0 {
		// We need to verify each transaction to find one from our sender
		for _, txHash := range historyResponse.Result.Tx {
			// Verify if this transaction is from our sender
			verified, _, err := verifyTransactionDetails(txHash, senderWallet)
			if err != nil {
				fmt.Printf("Error verifying transaction %s: %v\n", txHash, err)
				continue
			}

			if verified {
				return true, txHash, nil
			}
		}
	}

	// No matching transactions found in history
	return false, "", nil
}

// verifyTransactionDetails verifies if a transaction is from the sender wallet to the receiver wallet
func verifyTransactionDetails(txHash string, senderWallet string) (bool, float64, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create API URL to check transaction details
	txURL := fmt.Sprintf("https://api.mbc.wiki/transaction/%s", txHash)

	// Make the request
	resp, err := client.Get(txURL)
	if err != nil {
		return false, 0, fmt.Errorf("failed to connect to MicroBitcoin transaction API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("transaction API returned error status: %d", resp.StatusCode)
	}

	// Decode the response
	var txResponse struct {
		Error  interface{} `json:"error"`
		ID     string      `json:"id"`
		Result struct {
			Amount        float64 `json:"amount"`
			TxID          string  `json:"txid"`
			Confirmations int     `json:"confirmations"`
			Vin           []struct {
				ScriptPubKey struct {
					Address   string   `json:"address"`
					Addresses []string `json:"addresses"`
				} `json:"scriptPubKey"`
				Value float64 `json:"value"`
			} `json:"vin"`
			Vout []struct {
				N            int     `json:"n"`
				Value        float64 `json:"value"`
				ScriptPubKey struct {
					Address   string   `json:"address"`
					Addresses []string `json:"addresses"`
					Type      string   `json:"type"`
				} `json:"scriptPubKey"`
			} `json:"vout"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&txResponse); err != nil {
		return false, 0, fmt.Errorf("failed to decode transaction API response: %v", err)
	}

	// Make sure transaction is valid
	if txResponse.Error != nil {
		return false, 0, fmt.Errorf("transaction API returned error: %v", txResponse.Error)
	}

	// Check if this transaction is from our sender wallet
	isSenderFound := false
	for _, input := range txResponse.Result.Vin {
		if input.ScriptPubKey.Address == senderWallet {
			isSenderFound = true
			break
		}

		// Check addresses array too
		for _, addr := range input.ScriptPubKey.Addresses {
			if addr == senderWallet {
				isSenderFound = true
				break
			}
		}

		if isSenderFound {
			break
		}
	}

	// If sender isn't found, this isn't our transaction
	if !isSenderFound {
		return false, 0, nil
	}

	// Check if the receiver wallet is in the outputs
	for _, output := range txResponse.Result.Vout {
		if output.ScriptPubKey.Address == ReceiverWalletAddress {
			// Found output to our receiver wallet
			return true, output.Value, nil
		}

		// Check addresses array too
		for _, addr := range output.ScriptPubKey.Addresses {
			if addr == ReceiverWalletAddress {
				return true, output.Value, nil
			}
		}
	}

	// Didn't find output to our receiver wallet
	return false, 0, nil
}
