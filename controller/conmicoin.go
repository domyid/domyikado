package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging
	MerchCoinDiscordWebhookURL = "https://discord.com/api/webhooks/1348044639818485790/DOsYYebYjrTN48wZVDOPrO4j20X5J3pMAbOdPOUkrJuiXk5niqOjV9ZZ2r06th0jXMhh"
	MerchCoinWalletAddress     = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"
)

// Discord embed structure
type MerchCoinDiscordEmbed struct {
	Title       string                       `json:"title"`
	Description string                       `json:"description,omitempty"`
	Color       int                          `json:"color"`
	Fields      []MerchCoinDiscordEmbedField `json:"fields,omitempty"`
	Footer      *MerchCoinDiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string                       `json:"timestamp,omitempty"` // ISO8601 timestamp
}

// Discord embed field
type MerchCoinDiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Discord embed footer
type MerchCoinDiscordEmbedFooter struct {
	Text string `json:"text"`
}

// Discord webhook message structure
type MerchCoinDiscordWebhookMessage struct {
	Username  string                  `json:"username,omitempty"`
	AvatarURL string                  `json:"avatar_url,omitempty"`
	Content   string                  `json:"content,omitempty"`
	Embeds    []MerchCoinDiscordEmbed `json:"embeds,omitempty"`
}

// Helper function to send logs to Discord with embeds
func sendMerchCoinDiscordEmbed(title, description string, color int, fields []MerchCoinDiscordEmbedField) {
	// Set timestamp to current time
	timestamp := time.Now().Format(time.RFC3339)

	// Create embed
	embed := MerchCoinDiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Footer: &MerchCoinDiscordEmbedFooter{
			Text: "MerchCoin Payment System",
		},
		Timestamp: timestamp,
	}

	// Create message with embed
	webhookMsg := MerchCoinDiscordWebhookMessage{
		Username:  "MerchCoin Payment Bot",
		AvatarURL: "https://cdn-icons-png.flaticon.com/512/2168/2168252.png", // QR code icon
		Embeds:    []MerchCoinDiscordEmbed{embed},
	}

	// Convert to JSON (only log errors, don't fail the transaction)
	jsonData, err := json.Marshal(webhookMsg)
	if err != nil {
		log.Printf("Error marshaling Discord embed: %v", err)
		return
	}

	// Send to Discord webhook asynchronously
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(MerchCoinDiscordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error sending embed to Discord: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("Discord webhook returned non-success status: %d", resp.StatusCode)
		}
	}()
}

// Initialize MerchCoin payment total
func InitializeMerchCoinPaymentTotal() {
	var total model.MerchCoinPaymentTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Total document doesn't exist, create it
		_, err = config.Mongoconn.Collection("merchcointotals").InsertOne(context.Background(), model.MerchCoinPaymentTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		if err != nil {
			log.Printf("Error initializing MerchCoin payment totals: %v", err)
			sendMerchCoinDiscordEmbed(
				"🔴 Error: MerchCoin Payment Totals Initialization Failed",
				"Failed to initialize MerchCoin payment totals database.",
				15548997, // ColorRed
				[]MerchCoinDiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
		} else {
			log.Println("Initialized MerchCoin payment totals successfully")
			sendMerchCoinDiscordEmbed(
				"✅ System: MerchCoin Payment Totals Initialized",
				"Successfully initialized the MerchCoin payment totals database.",
				5763719, // ColorGreen
				nil,
			)
		}
	}
}

// Initialize MerchCoin queue
func InitializeMerchCoinQueue() {
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Queue document doesn't exist, create it
		_, err = config.Mongoconn.Collection("merchcoinqueue").InsertOne(context.Background(), model.MerchCoinQueue{
			IsProcessing:   false,
			CurrentOrderID: "",
			ExpiryTime:     time.Time{},
		})

		if err != nil {
			log.Printf("Error initializing MerchCoin queue: %v", err)
			sendMerchCoinDiscordEmbed(
				"🔴 Error: MerchCoin Queue Initialization Failed",
				"Failed to initialize MerchCoin payment queue.",
				15548997, // ColorRed
				[]MerchCoinDiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
		} else {
			log.Println("Initialized MerchCoin queue successfully")
			sendMerchCoinDiscordEmbed(
				"✅ System: MerchCoin Queue Initialized",
				"MerchCoin payment queue initialized successfully.",
				5763719, // ColorGreen
				nil,
			)
		}
	}
}

// Helper function to update MerchCoin payment totals
func updateMerchCoinPaymentTotal(amount float64) {
	opts := options.FindOneAndUpdate().SetUpsert(true)

	update := bson.M{
		"$inc": bson.M{
			"totalAmount": amount,
			"count":       1,
		},
		"$set": bson.M{
			"lastUpdated": time.Now(),
		},
	}

	var result model.MerchCoinPaymentTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOneAndUpdate(
		context.Background(),
		bson.M{},
		update,
		opts,
	).Decode(&result)

	if err != nil {
		log.Printf("Error updating MerchCoin payment totals: %v", err)
		sendMerchCoinDiscordEmbed(
			"🔴 Error: MerchCoin Payment Totals Update Failed",
			"Failed to update MerchCoin payment totals in database.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Amount", Value: fmt.Sprintf("%f MBC", amount), Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
	} else {
		sendMerchCoinDiscordEmbed(
			"💰 MerchCoin Payment: Total Updated",
			"Successfully updated MerchCoin payment totals.",
			5763719, // ColorGreen
			[]MerchCoinDiscordEmbedField{
				{Name: "Amount Added", Value: fmt.Sprintf("%f MBC", amount), Inline: true},
			},
		)
	}
}

// CreateMerchCoinOrder creates a new MerchCoin payment order
func CreateMerchCoinOrder(w http.ResponseWriter, r *http.Request) {
	var request model.MerchCoinCreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Invalid Request",
			"Failed to process create MerchCoin order request.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request - in this case, we only need the wonpay code
	if request.WonpayCode == "" {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Invalid Order Parameters",
			"MerchCoin order creation failed due to missing Wonpay code.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Wonpay Code", Value: "Missing", Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Wonpay code is required",
		})
		return
	}

	// Check if someone is currently paying
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Initialize queue if it doesn't exist
		InitializeMerchCoinQueue()
	} else if queue.IsProcessing {
		sendMerchCoinDiscordEmbed(
			"⏳ Queue: MerchCoin Payment in Progress",
			"Another MerchCoin payment is already in progress.",
			16776960, // ColorYellow
			[]MerchCoinDiscordEmbedField{
				{Name: "Wonpay Code", Value: request.WonpayCode, Inline: true},
				{Name: "Status", Value: "Queued", Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:     false,
			Message:     "Sedang ada pembayaran berlangsung. Silakan tunggu.",
			QueueStatus: true,
			ExpiryTime:  queue.ExpiryTime,
		})
		return
	}

	// Create order ID
	orderID := uuid.New().String()

	// Create new order in database
	newOrder := model.MerchCoinOrder{
		OrderID:    orderID,
		WonpayCode: request.WonpayCode,
		Timestamp:  time.Now(),
		Status:     "pending",
	}

	_, err = config.Mongoconn.Collection("merchcoinorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Database Error",
			"Failed to create MerchCoin order in database.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	expiryTime := time.Now().Add(900 * time.Second) // 15 minutes expiry
	_, err = config.Mongoconn.Collection("merchcoinqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   true,
			"currentOrderId": orderID,
			"expiryTime":     expiryTime,
		}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Queue Update Failed",
			"Failed to update MerchCoin payment queue.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// Log successful order creation
	sendMerchCoinDiscordEmbed(
		"🛒 New MerchCoin Order Created",
		"A new MerchCoin payment order has been created.",
		3447003, // ColorBlue
		[]MerchCoinDiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Wonpay Code", Value: request.WonpayCode, Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)

	// Set up expiry timer
	go func() {
		time.Sleep(900 * time.Second) // 15 minutes expiry

		// Check if this order is still the current one
		var currentQueue model.MerchCoinQueue
		err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking MerchCoin queue for timeout: %v", err)
			return
		}

		if currentQueue.CurrentOrderID == orderID {
			// Update order status to failed
			_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{"status": "failed"}},
			)
			if err != nil {
				log.Printf("Error updating MerchCoin order status: %v", err)
				sendMerchCoinDiscordEmbed(
					"🔴 Error: Status Update Failed",
					"Failed to update expired MerchCoin order status.",
					15548997, // ColorRed
					[]MerchCoinDiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			// Reset queue
			_, err = config.Mongoconn.Collection("merchcoinqueue").UpdateOne(
				context.Background(),
				bson.M{},
				bson.M{"$set": bson.M{
					"isProcessing":   false,
					"currentOrderId": "",
					"expiryTime":     time.Time{},
				}},
			)
			if err != nil {
				log.Printf("Error resetting MerchCoin queue: %v", err)
				sendMerchCoinDiscordEmbed(
					"🔴 Error: Queue Reset Failed",
					"Failed to reset MerchCoin queue after order expiry.",
					15548997, // ColorRed
					[]MerchCoinDiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			sendMerchCoinDiscordEmbed(
				"⏱️ MerchCoin Order Expired",
				"A MerchCoin payment order has expired.",
				16776960, // ColorYellow
				[]MerchCoinDiscordEmbedField{
					{Name: "Order ID", Value: orderID, Inline: true},
					{Name: "Wonpay Code", Value: newOrder.WonpayCode, Inline: true},
					{Name: "Status", Value: "Expired", Inline: true},
				},
			)
		}
	}()

	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success:       true,
		OrderID:       orderID,
		ExpiryTime:    expiryTime,
		QRImageURL:    "wonpay.png",
		WalletAddress: MerchCoinWalletAddress,
	})
}

// Step 1: Check mempool for transactions and extract txid properly
func checkMerchCoinMempool() (bool, string, float64, error) {
	// API URL for checking mempool
	url := "https://api.mbc.wiki/mempool/" + MerchCoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var mempoolResp model.MerchCoinMempoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&mempoolResp); err != nil {
		return false, "", 0, err
	}

	// Check for errors in API response
	if mempoolResp.Error != nil {
		return false, "", 0, errors.New("API error: " + *mempoolResp.Error)
	}

	// Check if there are transactions in mempool
	if mempoolResp.Result.TxCount > 0 && len(mempoolResp.Result.Tx) > 0 {
		// Extract transaction ID from the first transaction in the mempool
		tx := mempoolResp.Result.Tx[0]
		// We only return the txid in Step 1, the actual amount will be determined in Step 3
		return true, tx.TxID, 0, nil
	}

	return false, "", 0, nil
}

// Step 2: Check if transaction exists in history
func checkMerchCoinTxHistory() (bool, string, error) {
	// API URL for checking transaction history
	url := "https://api.mbc.wiki/history/" + MerchCoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	// Parse response
	var historyResp model.MerchCoinHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		return false, "", err
	}

	// Check for errors in API response
	if historyResp.Error != nil {
		return false, "", errors.New("API error: " + *historyResp.Error)
	}

	// Check if there are any transactions in history
	if historyResp.Result.TxCount > 0 && len(historyResp.Result.Tx) > 0 {
		// Get the most recent transaction (first in the list)
		if len(historyResp.Result.Tx) >= 1 {
			return true, historyResp.Result.Tx[0], nil
		}
	}

	return false, "", nil
}

// Step 3: Verify transaction details
func checkMerchCoinTxDetails(txid string) (bool, float64, error) {
	// API URL for getting transaction details
	url := "https://api.mbc.wiki/transaction/" + txid

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var txResp model.MerchCoinTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return false, 0, err
	}

	// Check for errors in API response
	if txResp.Error != nil {
		return false, 0, errors.New("API error: " + *txResp.Error)
	}

	// Find the output that matches our wallet address
	var amount int64 = 0
	for _, vout := range txResp.Result.Vout {
		// Check if this output is to our wallet address
		for _, addr := range vout.ScriptPubKey.Addresses {
			if addr == MerchCoinWalletAddress {
				amount = vout.Value
				break
			}
		}
		// Alternative check using the single address field
		if vout.ScriptPubKey.Address == MerchCoinWalletAddress {
			amount = vout.Value
			break
		}
	}

	// Convert satoshis to MBC
	amountMBC := float64(amount) / 100000000

	// Transaction is valid if we found our address with some value
	return amount > 0, amountMBC, nil
}

// CheckMerchCoinPayment checks the status of a MerchCoin payment
func CheckMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	// Find the order
	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"❓ Check MerchCoin Payment",
			"MerchCoin payment status check for non-existent order.",
			16776960, // ColorYellow
			[]MerchCoinDiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Status", Value: "Not Found", Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// If payment is still pending, check mempool first (Step 1)
	if order.Status == "pending" {
		// Step 1: Check mempool
		mempoolStatus, mempoolTxid, _, err := checkMerchCoinMempool()

		if err != nil {
			// Return the error but continue with the flow
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking mempool failed: " + err.Error(),
				Step1Complete: false,
				Step2Complete: false,
				Step3Complete: false,
			})
			return
		}

		// If transaction found in mempool, return success for step 1 with txid
		if mempoolStatus && mempoolTxid != "" {
			// Step 1 is complete, return txid from mempool
			// Note: We're not returning the amount yet, that will be determined in Step 3
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Transaction found in mempool, waiting for confirmation.",
				Step1Complete: true,
				Step2Complete: false,
				Step3Complete: false,
				TxID:          mempoolTxid,
			})
			return
		}

		// If no transaction found in any check
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "No transaction found yet. Please make the payment or wait if you've already sent it.",
			Step1Complete: false,
			Step2Complete: false,
			Step3Complete: false,
		})
		return
	}

	// If payment is already processed (success or failed)
	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success: true,
		Status:  order.Status,
		TxID:    order.TxID,
		Amount:  order.Amount,
	})
}

// CheckStep2Handler checks transaction history after the 7-minute delay
func CheckStep2Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Step 2: Check history with the txid from step 1
	historyStatus, historyTxid, err := checkMerchCoinTxHistory()
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Checking transaction history failed: " + err.Error(),
			Step1Complete: true,
			Step2Complete: false,
			Step3Complete: false,
			TxID:          txid,
		})
		return
	}

	if historyStatus && historyTxid != "" {
		// Transaction found in history, step 2 complete
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction found in history, proceed to final verification.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
		})
		return
	}

	// Transaction not found in history
	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success:       true,
		Status:        "pending",
		Message:       "Transaction not found in history yet. Please wait.",
		Step1Complete: true,
		Step2Complete: false,
		Step3Complete: false,
		TxID:          txid,
	})
}

// UpdatedCheckStep3Handler incorporates point calculations
func CheckStep3Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Step 3: Verify transaction details and get the actual amount
	txDetails, actualAmount, err := checkMerchCoinTxDetails(txid)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Error checking transaction details: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
		})
		return
	}

	if !txDetails {
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction details verification failed.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
		})
		return
	}

	// Now that we have verified the transaction and gotten the actual amount,
	// update the order status to success with the final amount from Step 3
	_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status": "success",
			"txid":   txid,
			"amount": actualAmount, // This is the actual amount from Step 3
		}},
	)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction verified but error updating order status: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			Amount:        actualAmount,
		})
		return
	}

	// Update payment totals with the actual amount from Step 3
	updateMerchCoinPaymentTotal(actualAmount)

	// Process the transaction to calculate points
	err = ProcessMerchCoinTransaction(config.Mongoconn, order)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Point Calculation Failed",
			"Failed to calculate points for transaction.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		// Continue anyway, we don't want to fail the transaction just because point calculation failed
	}

	// Reset queue
	_, err = config.Mongoconn.Collection("merchcoinqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		log.Printf("Error resetting MerchCoin queue: %v", err)
	}

	sendMerchCoinDiscordEmbed(
		"✅ MerchCoin Payment Successful",
		"A MerchCoin payment has been confirmed automatically.",
		5763719, // ColorGreen
		[]MerchCoinDiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Wonpay Code", Value: order.WonpayCode, Inline: true},
			{Name: "Transaction ID", Value: txid, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f MBC", actualAmount), Inline: true},
		},
	)

	// Return success response with the actual amount from Step 3
	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success:       true,
		Status:        "success",
		Message:       "Payment confirmed successfully!",
		Step1Complete: true,
		Step2Complete: true,
		Step3Complete: true,
		TxID:          txid,
		Amount:        actualAmount, // The actual amount from Step 3
	})
}

// Updated ManuallyConfirmMerchCoinPayment to include point processing
func ManuallyConfirmMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	// Parse request body to get txid and amount
	var request model.MerchCoinConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate the request
	if request.TxID == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	if request.Amount <= 0 {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Amount must be greater than 0",
		})
		return
	}

	// Find the order
	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Manual Confirmation Failed",
			"Failed to confirm MerchCoin payment manually.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: "Order not found", Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status": "success",
			"txid":   request.TxID,
			"amount": request.Amount,
		}},
	)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update MerchCoin order status during manual confirmation.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Process the transaction to calculate points
	order.TxID = request.TxID
	order.Amount = request.Amount
	err = ProcessMerchCoinTransaction(config.Mongoconn, order)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Point Calculation Failed",
			"Failed to calculate points for transaction.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		// Continue anyway, we don't want to fail the transaction just because point calculation failed
	}

	// Update payment totals
	updateMerchCoinPaymentTotal(request.Amount)

	// Reset queue
	_, err = config.Mongoconn.Collection("merchcoinqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendMerchCoinDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset MerchCoin queue after manual confirmation.",
			15548997, // ColorRed
			[]MerchCoinDiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	sendMerchCoinDiscordEmbed(
		"✅ Manual MerchCoin Payment Confirmation",
		"A MerchCoin payment has been confirmed manually.",
		5763719, // ColorGreen
		[]MerchCoinDiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Wonpay Code", Value: order.WonpayCode, Inline: true},
			{Name: "Transaction ID", Value: request.TxID, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f MBC", request.Amount), Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

// GetMerchCoinQueueStatus gets the status of the MerchCoin payment queue
func GetMerchCoinQueueStatus(w http.ResponseWriter, r *http.Request) {
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// If no queue document exists, initialize it
		InitializeMerchCoinQueue()

		// Return empty queue status
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
			Success:      true,
			IsProcessing: false,
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success:      true,
		IsProcessing: queue.IsProcessing,
		ExpiryTime:   queue.ExpiryTime,
	})
}

// GetMerchCoinTotalPayments gets the total payments for MerchCoin
func GetMerchCoinTotalPayments(w http.ResponseWriter, r *http.Request) {
	var total model.MerchCoinPaymentTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Initialize totals if not found
		InitializeMerchCoinPaymentTotal()

		// Return empty totals
		at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, total)
}

// SimulateMerchCoinPayment simulates a MerchCoin payment for testing
func SimulateMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	var request model.MerchCoinSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Find the latest pending order
	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(
		context.Background(),
		bson.M{"status": "pending"},
		options.FindOne().SetSort(bson.M{"timestamp": -1}),
	).Decode(&order)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success: false,
				Message: "No pending order found to simulate payment",
			})
			return
		}

		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error finding pending order: " + err.Error(),
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": order.OrderID},
		bson.M{"$set": bson.M{
			"status": "success",
			"txid":   request.TxID,
			"amount": request.Amount,
		}},
	)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error updating order status: " + err.Error(),
		})
		return
	}

	// Update payment totals
	updateMerchCoinPaymentTotal(request.Amount)

	// Reset queue
	_, err = config.Mongoconn.Collection("merchcoinqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Error resetting queue: " + err.Error(),
		})
		return
	}

	sendMerchCoinDiscordEmbed(
		"🧪 Simulated MerchCoin Payment",
		"A MerchCoin payment has been simulated for testing.",
		10181046, // ColorPurple
		[]MerchCoinDiscordEmbedField{
			{Name: "Order ID", Value: order.OrderID, Inline: true},
			{Name: "Wonpay Code", Value: order.WonpayCode, Inline: true},
			{Name: "Transaction ID", Value: request.TxID, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f MBC", request.Amount), Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success: true,
		Message: "Payment simulated successfully",
		OrderID: order.OrderID,
	})
}

// ConfirmMerchCoinNotification processes webhook notifications from payment providers
func ConfirmMerchCoinNotification(w http.ResponseWriter, r *http.Request) {
	var request model.MerchCoinNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Log notification for debugging
	log.Printf("Received MerchCoin notification: %s", request.NotificationText)
	sendMerchCoinDiscordEmbed(
		"📥 MerchCoin Notification Received",
		"Received a MerchCoin payment notification.",
		3447003, // ColorBlue
		[]MerchCoinDiscordEmbedField{
			{Name: "Notification Text", Value: request.NotificationText, Inline: false},
		},
	)

	// This is a placeholder for processing notifications
	// In a real implementation, you would parse the notification
	// to extract transaction details

	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success: true,
		Message: "Notification received",
	})
}

// GetMerchCoinDailyReport sends daily MerchCoin transaction reports to all groups
func GetMerchCoinDailyReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Get all active project groups first
	projectFilter := bson.M{"closed": bson.M{"$ne": true}}
	projects, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", projectFilter)
	if err != nil {
		resp.Info = "Failed to query projects"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Get overall summary of MerchCoin transactions
	overallSummary := report.GetMerchCoinOverallSummary(config.Mongoconn)
	if overallSummary == "" {
		resp.Info = "No transactions to report"
		resp.Response = "No MerchCoin transactions found for the time period"
		at.WriteJSON(respw, http.StatusNotFound, resp)
		return
	}

	// Send to all active groups
	var successCount int = 0
	for _, project := range projects {
		if project.WAGroupID != "" {
			dt := &whatsauth.TextMessage{
				To:       project.WAGroupID,
				IsGroup:  true,
				Messages: overallSummary,
			}

			_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
			if err != nil {
				// Log error but continue with other groups
				fmt.Printf("Error sending to group %s: %v\n", project.WAGroupID, err)
				continue
			}

			successCount++
		}
	}

	// Also send to a specific default group (like in GetReportHariIni)
	defaultGroupID := "6281313112053-1492882006" // Same as in GetReportHariIni
	dt := &whatsauth.TextMessage{
		To:       defaultGroupID,
		IsGroup:  true,
		Messages: overallSummary,
	}

	_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Failed to send to default group"
		resp.Response = err.Error()
	} else {
		successCount++
	}

	// Also send to all distinct WAGroupIDs from yesterday's transactions
	filter := bson.M{"_id": report.YesterdayFilter()}
	wagroupidlist, err := atdb.GetAllDistinctDoc(config.Mongoconn, filter, "project.wagroupid", "merchcoinorders")
	if err == nil {
		for _, wagroupid := range wagroupidlist {
			// Type assertion to convert any to string
			groupID, ok := wagroupid.(string)
			if !ok || groupID == "" {
				continue
			}

			// Check if we've already sent to this group
			alreadySent := false
			for _, project := range projects {
				if project.WAGroupID == groupID {
					alreadySent = true
					break
				}
			}

			if !alreadySent && groupID != defaultGroupID {
				dt := &whatsauth.TextMessage{
					To:       groupID,
					IsGroup:  true,
					Messages: overallSummary,
				}

				_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
				if err != nil {
					fmt.Printf("Error sending to group %s: %v\n", groupID, err)
					continue
				}

				successCount++
			}
		}
	}

	resp.Status = "Success"
	resp.Response = fmt.Sprintf("MerchCoin daily reports sent to %d groups", successCount)
	at.WriteJSON(respw, http.StatusOK, resp)
}

// GetMerchCoinWeeklyReport sends weekly MerchCoin transaction reports to all groups
func GetMerchCoinWeeklyReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Get weekly summary
	weeklySummary := report.GetMerchCoinWeeklySummary(config.Mongoconn)
	if weeklySummary == "" {
		resp.Status = "Error"
		resp.Response = "No transactions found for weekly report"
		at.WriteJSON(respw, http.StatusNotFound, resp)
		return
	}

	// Get all active project groups
	projectFilter := bson.M{"closed": bson.M{"$ne": true}}
	projects, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", projectFilter)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to retrieve projects"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Send to all active groups
	var successCount int = 0
	for _, project := range projects {
		if project.WAGroupID != "" {
			dt := &whatsauth.TextMessage{
				To:       project.WAGroupID,
				IsGroup:  true,
				Messages: weeklySummary,
			}

			_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
			if err != nil {
				// Log error but continue with other groups
				fmt.Printf("Error sending to group %s: %v\n", project.WAGroupID, err)
				continue
			}

			successCount++
		}
	}

	// Also send to a specific default group (like in GetReportHariIni)
	defaultGroupID := "6281313112053-1492882006" // Same as in GetReportHariIni
	dt := &whatsauth.TextMessage{
		To:       defaultGroupID,
		IsGroup:  true,
		Messages: weeklySummary,
	}

	_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Failed to send to default group"
		resp.Response = err.Error()
	} else {
		successCount++
	}

	// Also send to all distinct WAGroupIDs from recent transactions
	filter := bson.M{
		"_id": bson.M{
			"$gte": primitive.NewObjectIDFromTimestamp(report.GetDateSekarang().AddDate(0, 0, -7)),
			"$lt":  primitive.NewObjectIDFromTimestamp(report.GetDateSekarang()),
		},
	}
	wagroupidlist, err := atdb.GetAllDistinctDoc(config.Mongoconn, filter, "project.wagroupid", "merchcoinorders")
	if err == nil {
		for _, wagroupid := range wagroupidlist {
			// Type assertion to convert any to string
			groupID, ok := wagroupid.(string)
			if !ok || groupID == "" {
				continue
			}

			// Check if we've already sent to this group
			alreadySent := false
			for _, project := range projects {
				if project.WAGroupID == groupID {
					alreadySent = true
					break
				}
			}

			if !alreadySent && groupID != defaultGroupID {
				dt := &whatsauth.TextMessage{
					To:       groupID,
					IsGroup:  true,
					Messages: weeklySummary,
				}

				_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
				if err != nil {
					fmt.Printf("Error sending to group %s: %v\n", groupID, err)
					continue
				}

				successCount++
			}
		}
	}

	resp.Status = "Success"
	resp.Response = fmt.Sprintf("MerchCoin weekly reports sent to %d groups", successCount)
	at.WriteJSON(respw, http.StatusOK, resp)
}

// ProcessMerchCoinTransaction calculates points and stores in merchcointosend collection
func ProcessMerchCoinTransaction(db *mongo.Database, order model.MerchCoinOrder) error {
	// Find user by WonpayCode (which contains phone number)
	userFilter := bson.M{"phonenumber": order.WonpayCode}
	user, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", userFilter)
	if err != nil {
		return fmt.Errorf("error finding user: %v", err)
	}

	// Calculate average transaction amount for today
	avgAmount, err := calculateAverageMerchCoinAmount(db)
	if err != nil {
		return fmt.Errorf("error calculating average amount: %v", err)
	}

	// Calculate points - 1 point is (amount / average) * 100
	var points float64 = 0
	if avgAmount > 0 {
		points = (order.Amount / avgAmount) * 100
	}

	// Create the MerchCoinToSend record
	merchCoinToSend := model.MerchCoinToSend{
		ID:            primitive.NewObjectID(),
		PhoneNumber:   user.PhoneNumber,
		Name:          user.Name,
		NPM:           user.NPM,
		WonpayWallet:  user.Wonpaywallet,
		OrderID:       order.OrderID,
		TxID:          order.TxID,
		Amount:        order.Amount,
		Points:        points,
		AverageAmount: avgAmount,
		Timestamp:     time.Now(),
		ProcessedDate: time.Now().Format("2006-01-02"),
		Reported:      false,
	}

	// Insert into merchcointosend collection
	_, err = atdb.InsertOneDoc(db, "merchcointosend", merchCoinToSend)
	if err != nil {
		return fmt.Errorf("error inserting MerchCoinToSend: %v", err)
	}

	// Log the point calculation
	sendMerchCoinDiscordEmbed(
		"💰 MerchCoin Points Calculated",
		fmt.Sprintf("Points calculated for transaction %s", order.OrderID),
		3447003, // ColorBlue
		[]MerchCoinDiscordEmbedField{
			{Name: "User", Value: user.Name + " (" + user.PhoneNumber + ")", Inline: true},
			{Name: "NPM", Value: user.NPM, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%.8f MBC", order.Amount), Inline: true},
			{Name: "Points", Value: fmt.Sprintf("%.2f", points), Inline: true},
			{Name: "Average Amount", Value: fmt.Sprintf("%.8f MBC", avgAmount), Inline: true},
		},
	)

	return nil
}

// calculateAverageMerchCoinAmount calculates the average transaction amount for today
func calculateAverageMerchCoinAmount(db *mongo.Database) (float64, error) {
	// Get today's date
	today := time.Now().Format("2006-01-02")
	startOfDay, _ := time.Parse("2006-01-02", today)
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Create filter for today's successful transactions
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": startOfDay,
			"$lt":  endOfDay,
		},
		"status": "success",
		"amount": bson.M{"$gt": 0},
	}

	// Find all successful transactions for today
	transactions, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil {
		return 0, err
	}

	if len(transactions) == 0 {
		// If no transactions today, use the last 7 days average
		lastWeek := startOfDay.Add(-7 * 24 * time.Hour)
		weekFilter := bson.M{
			"timestamp": bson.M{
				"$gte": lastWeek,
				"$lt":  endOfDay,
			},
			"status": "success",
			"amount": bson.M{"$gt": 0},
		}

		transactions, err = atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", weekFilter)
		if err != nil {
			return 0, err
		}

		if len(transactions) == 0 {
			// If still no transactions, use a default value
			return 0.01, nil // Default average amount
		}
	}

	// Calculate average
	var totalAmount float64
	for _, tx := range transactions {
		totalAmount += tx.Amount
	}

	return totalAmount / float64(len(transactions)), nil
}

// CreatePointSummary stores a daily summary of point calculations
func CreatePointSummary(db *mongo.Database) error {
	// Get today's date
	today := time.Now().Format("2006-01-02")

	// Find all transactions from yesterday
	filter := bson.M{
		"_id":    report.YesterdayFilter(),
		"status": "success",
	}

	transactions, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", filter)
	if err != nil {
		return err
	}

	if len(transactions) == 0 {
		return nil // No transactions, nothing to summarize
	}

	var totalAmount float64
	for _, tx := range transactions {
		totalAmount += tx.Amount
	}

	avgAmount := totalAmount / float64(len(transactions))

	// Create summary
	summary := model.MerchCoinPointSummary{
		TotalTransactions: len(transactions),
		TotalAmount:       totalAmount,
		AverageAmount:     avgAmount,
		Date:              today,
		CalculatedAt:      time.Now(),
	}

	// Store in database
	_, err = atdb.InsertOneDoc(db, "merchcoinpointsummary", summary)
	return err
}

// Add these functions directly to conmicoin.go

// GetMerchCoinPointsReport generates a report of MerchCoin points for specific groups
func GetMerchCoinPointsReport(db *mongo.Database) string {
	// Only send to specific group IDs
	allowedGroupIDs := []string{"120363298977628161", "120363022595651310"}
	groupIDStr := strings.Join(allowedGroupIDs, " and ")

	// Get today's date for filtering
	today := time.Now().Format("2006-01-02")

	// Query processed points from merchcointosend collection
	filter := bson.M{
		"processedDate": today,
		"reported":      false,
	}

	pointRecords, err := atdb.GetAllDoc[[]model.MerchCoinToSend](db, "merchcointosend", filter)
	if err != nil || len(pointRecords) == 0 {
		return fmt.Sprintf("*MerchCoin Points Report (Groups %s)*\n\nNo point calculations to report for today.", groupIDStr)
	}

	// Group points by user
	userPoints := make(map[string]struct {
		Name        string
		PhoneNumber string
		NPM         string
		TotalPoints float64
		Wallet      string
		TxCount     int
	})

	for _, record := range pointRecords {
		user, exists := userPoints[record.PhoneNumber]
		if !exists {
			user = struct {
				Name        string
				PhoneNumber string
				NPM         string
				TotalPoints float64
				Wallet      string
				TxCount     int
			}{
				Name:        record.Name,
				PhoneNumber: record.PhoneNumber,
				NPM:         record.NPM,
				TotalPoints: 0,
				Wallet:      record.WonpayWallet,
				TxCount:     0,
			}
		}

		user.TotalPoints += record.Points
		user.TxCount++
		userPoints[record.PhoneNumber] = user
	}

	// Create sorted list of users by points
	type UserRank struct {
		Name        string
		PhoneNumber string
		NPM         string
		Points      float64
		Wallet      string
		TxCount     int
	}

	var rankings []UserRank
	for _, info := range userPoints {
		rankings = append(rankings, UserRank{
			Name:        info.Name,
			PhoneNumber: info.PhoneNumber,
			NPM:         info.NPM,
			Points:      info.TotalPoints,
			Wallet:      info.Wallet,
			TxCount:     info.TxCount,
		})
	}

	// Sort by points (descending)
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Points > rankings[j].Points
	})

	// Build the message
	message := fmt.Sprintf("*MerchCoin Points Report (Groups %s)*\n", groupIDStr)
	message += fmt.Sprintf("Date: %s\n\n", time.Now().Format("Monday, January 2, 2006"))
	message += "*Point Rankings:*\n"

	for i, rank := range rankings {
		var displayWallet string = "Not Set"
		if rank.Wallet != "" {
			if len(rank.Wallet) > 12 {
				displayWallet = rank.Wallet[:6] + "..." + rank.Wallet[len(rank.Wallet)-6:]
			} else {
				displayWallet = rank.Wallet
			}
		}

		npmDisplay := rank.NPM
		if npmDisplay == "" {
			npmDisplay = "N/A"
		}

		message += fmt.Sprintf("%d. %s (%s)\n   NPM: %s\n   Points: %.2f\n   Wallet: %s\n   Transactions: %d\n\n",
			i+1,
			rank.Name,
			rank.PhoneNumber,
			npmDisplay,
			rank.Points,
			displayWallet,
			rank.TxCount)
	}

	message += "*Points are calculated as: (transaction amount / daily average) × 100*\n"
	message += "*Higher points are awarded for transactions above the daily average.*"

	// Update records to mark as reported
	for _, record := range pointRecords {
		_, err := db.Collection("merchcointosend").UpdateMany(
			context.Background(),
			bson.M{"phoneNumber": record.PhoneNumber, "processedDate": today},
			bson.M{"$set": bson.M{"reported": true}},
		)
		if err != nil {
			fmt.Printf("Error marking records as reported: %v\n", err)
		}
	}

	return message
}

// SendMerchCoinPointsReportToGroups sends the points report to specified groups
func SendMerchCoinPointsReportToGroups(db *mongo.Database) error {
	// Generate the report
	report := GetMerchCoinPointsReport(db)
	if report == "" {
		return errors.New("no report generated")
	}

	// Only send to specific group IDs
	allowedGroupIDs := []string{"120363298977628161", "120363022595651310"}

	// Send to each allowed group
	for _, groupID := range allowedGroupIDs {
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: report,
		}

		_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			return fmt.Errorf("failed to send report to group %s: %v", groupID, err)
		}
	}

	return nil
}

// RecalculateMerchCoinPoints recalculates all points for a given date range
func RecalculateMerchCoinPoints(db *mongo.Database, startDate, endDate time.Time) error {
	// Clear existing records for the date range
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	deleteFilter := bson.M{
		"processedDate": bson.M{
			"$gte": startDateStr,
			"$lte": endDateStr,
		},
	}

	_, err := db.Collection("merchcointosend").DeleteMany(context.Background(), deleteFilter)
	if err != nil {
		return fmt.Errorf("error clearing existing records: %v", err)
	}

	// Get all successful transactions in the date range
	txFilter := bson.M{
		"timestamp": bson.M{
			"$gte": startDate,
			"$lt":  endDate.Add(24 * time.Hour),
		},
		"status": "success",
	}

	transactions, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", txFilter)
	if err != nil {
		return fmt.Errorf("error retrieving transactions: %v", err)
	}

	// Calculate daily averages
	dailyAverages := make(map[string]float64)

	for _, tx := range transactions {
		dateStr := tx.Timestamp.Format("2006-01-02")

		// If we haven't calculated average for this day yet
		if _, exists := dailyAverages[dateStr]; !exists {
			// Filter for this specific day
			dayStart, _ := time.Parse("2006-01-02", dateStr)
			dayEnd := dayStart.Add(24 * time.Hour)

			dayFilter := bson.M{
				"timestamp": bson.M{
					"$gte": dayStart,
					"$lt":  dayEnd,
				},
				"status": "success",
			}

			dayTxs, err := atdb.GetAllDoc[[]model.MerchCoinOrder](db, "merchcoinorders", dayFilter)
			if err != nil {
				return fmt.Errorf("error getting transactions for %s: %v", dateStr, err)
			}

			if len(dayTxs) == 0 {
				dailyAverages[dateStr] = 0.01 // Default value
				continue
			}

			// Calculate average
			var total float64
			for _, dayTx := range dayTxs {
				total += dayTx.Amount
			}

			dailyAverages[dateStr] = total / float64(len(dayTxs))
		}
	}

	// Process each transaction
	for _, tx := range transactions {
		dateStr := tx.Timestamp.Format("2006-01-02")
		avgAmount := dailyAverages[dateStr]

		// Find user
		userFilter := bson.M{"phonenumber": tx.WonpayCode}
		user, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", userFilter)
		if err != nil {
			// Skip if user not found
			continue
		}

		// Calculate points
		var points float64
		if avgAmount > 0 {
			points = (tx.Amount / avgAmount) * 100
		}

		// Create record
		record := model.MerchCoinToSend{
			ID:            primitive.NewObjectID(),
			PhoneNumber:   user.PhoneNumber,
			Name:          user.Name,
			NPM:           user.NPM,
			WonpayWallet:  user.Wonpaywallet,
			OrderID:       tx.OrderID,
			TxID:          tx.TxID,
			Amount:        tx.Amount,
			Points:        points,
			AverageAmount: avgAmount,
			Timestamp:     tx.Timestamp,
			ProcessedDate: dateStr,
			Reported:      true, // Mark as already reported since this is historical data
		}

		// Insert record
		_, err = atdb.InsertOneDoc(db, "merchcointosend", record)
		if err != nil {
			return fmt.Errorf("error inserting record for tx %s: %v", tx.OrderID, err)
		}
	}

	return nil
}

// GetMerchCoinPointsDailyReport generates a daily report of MerchCoin points
func GetMerchCoinPointsDailyReport(db *mongo.Database) string {
	// Generate the report focusing on the specific groups
	allowedGroupIDs := []string{"120363298977628161", "120363022595651310"}
	groupIDStr := strings.Join(allowedGroupIDs, " and ")

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Query all points calculated for yesterday
	filter := bson.M{
		"processedDate": yesterday,
	}

	pointRecords, err := atdb.GetAllDoc[[]model.MerchCoinToSend](db, "merchcointosend", filter)
	if err != nil || len(pointRecords) == 0 {
		return fmt.Sprintf("*MerchCoin Points Daily Report (Groups %s)*\n\nNo point calculations for yesterday.", groupIDStr)
	}

	// Group points by user
	userPoints := make(map[string]struct {
		Name        string
		PhoneNumber string
		NPM         string
		TotalPoints float64
		TotalAmount float64
		Wallet      string
		TxCount     int
	})

	for _, record := range pointRecords {
		user, exists := userPoints[record.PhoneNumber]
		if !exists {
			user = struct {
				Name        string
				PhoneNumber string
				NPM         string
				TotalPoints float64
				TotalAmount float64
				Wallet      string
				TxCount     int
			}{
				Name:        record.Name,
				PhoneNumber: record.PhoneNumber,
				NPM:         record.NPM,
				TotalPoints: 0,
				TotalAmount: 0,
				Wallet:      record.WonpayWallet,
				TxCount:     0,
			}
		}

		user.TotalPoints += record.Points
		user.TotalAmount += record.Amount
		user.TxCount++
		userPoints[record.PhoneNumber] = user
	}

	// Create sorted list of users by points
	type UserRank struct {
		Name        string
		PhoneNumber string
		NPM         string
		Points      float64
		Amount      float64
		Wallet      string
		TxCount     int
	}

	var rankings []UserRank
	for _, info := range userPoints {
		rankings = append(rankings, UserRank{
			Name:        info.Name,
			PhoneNumber: info.PhoneNumber,
			NPM:         info.NPM,
			Points:      info.TotalPoints,
			Amount:      info.TotalAmount,
			Wallet:      info.Wallet,
			TxCount:     info.TxCount,
		})
	}

	// Sort by points (descending)
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Points > rankings[j].Points
	})

	// Build the message
	message := fmt.Sprintf("*MerchCoin Points Daily Report (Groups %s)*\n", groupIDStr)
	message += fmt.Sprintf("Date: %s\n\n", time.Now().AddDate(0, 0, -1).Format("Monday, January 2, 2006"))

	// Calculate overall statistics
	var totalPoints, totalAmount float64
	var totalTxCount int

	for _, rank := range rankings {
		totalPoints += rank.Points
		totalAmount += rank.Amount
		totalTxCount += rank.TxCount
	}

	message += fmt.Sprintf("Total Transactions: %d\n", totalTxCount)
	message += fmt.Sprintf("Total Amount: %.8f MBC\n", totalAmount)
	message += fmt.Sprintf("Total Points Awarded: %.2f\n\n", totalPoints)

	message += "*Point Rankings:*\n"

	for i, rank := range rankings {
		// Format wallet address to be shorter
		var displayWallet string = "Not Set"
		if rank.Wallet != "" {
			if len(rank.Wallet) > 12 {
				displayWallet = rank.Wallet[:6] + "..." + rank.Wallet[len(rank.Wallet)-6:]
			} else {
				displayWallet = rank.Wallet
			}
		}

		npmDisplay := rank.NPM
		if npmDisplay == "" {
			npmDisplay = "N/A"
		}

		message += fmt.Sprintf("%d. %s (%s)\n   NPM: %s\n   Points: %.2f\n   Amount: %.8f MBC\n   Wallet: %s\n   Transactions: %d\n\n",
			i+1,
			rank.Name,
			rank.PhoneNumber,
			npmDisplay,
			rank.Points,
			rank.Amount,
			displayWallet,
			rank.TxCount)
	}

	message += "*Points are calculated as: (transaction amount / daily average) × 100*\n"
	message += "*Higher points are awarded for transactions above the daily average.*"

	return message
}

// SendMerchCoinPointsDailyReportToGroups sends the daily points report to specified groups
func SendMerchCoinPointsDailyReportToGroups(db *mongo.Database) error {
	// Generate the report
	report := GetMerchCoinPointsDailyReport(db)
	if report == "" {
		return errors.New("no daily report generated")
	}

	// Only send to specific group IDs
	allowedGroupIDs := []string{"120363298977628161", "120363022595651310"}

	// Send to each allowed group
	for _, groupID := range allowedGroupIDs {
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: report,
		}

		_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			return fmt.Errorf("failed to send daily report to group %s: %v", groupID, err)
		}
	}

	return nil
}

// CalculateMerchCoinUserStats calculates cumulative statistics for users based on their MerchCoin transactions
func CalculateMerchCoinUserStats(db *mongo.Database) error {
	// Get all records from merchcointosend
	records, err := atdb.GetAllDoc[[]model.MerchCoinToSend](db, "merchcointosend", bson.M{})
	if err != nil {
		return fmt.Errorf("error retrieving point records: %v", err)
	}

	// Group by user
	userStats := make(map[string]struct {
		Name             string
		PhoneNumber      string
		NPM              string
		WalletAddress    string
		TotalPoints      float64
		TotalAmount      float64
		TransactionCount int
		LastTransaction  time.Time
	})

	for _, record := range records {
		stats, exists := userStats[record.PhoneNumber]
		if !exists {
			stats = struct {
				Name             string
				PhoneNumber      string
				NPM              string
				WalletAddress    string
				TotalPoints      float64
				TotalAmount      float64
				TransactionCount int
				LastTransaction  time.Time
			}{
				Name:             record.Name,
				PhoneNumber:      record.PhoneNumber,
				NPM:              record.NPM,
				WalletAddress:    record.WonpayWallet,
				TotalPoints:      0,
				TotalAmount:      0,
				TransactionCount: 0,
				LastTransaction:  time.Time{},
			}
		}

		stats.TotalPoints += record.Points
		stats.TotalAmount += record.Amount
		stats.TransactionCount++

		if record.Timestamp.After(stats.LastTransaction) {
			stats.LastTransaction = record.Timestamp
		}

		userStats[record.PhoneNumber] = stats
	}

	// Store user stats in database
	for phoneNumber, stats := range userStats {
		// Create document
		statDoc := bson.M{
			"phoneNumber":      phoneNumber,
			"name":             stats.Name,
			"npm":              stats.NPM,
			"walletAddress":    stats.WalletAddress,
			"totalPoints":      stats.TotalPoints,
			"totalAmount":      stats.TotalAmount,
			"transactionCount": stats.TransactionCount,
			"lastTransaction":  stats.LastTransaction,
			"lastUpdated":      time.Now(),
		}

		// Upsert to database
		updateOpts := options.Update().SetUpsert(true)
		_, err := db.Collection("merchcoinuserstats").UpdateOne(
			context.Background(),
			bson.M{"phoneNumber": phoneNumber},
			bson.M{"$set": statDoc},
			updateOpts,
		)

		if err != nil {
			fmt.Printf("Error updating stats for user %s: %v\n", phoneNumber, err)
			// Continue with other users even if one fails
		}
	}

	return nil
}

// GetMerchCoinLeaderboard generates a leaderboard of users with the most MerchCoin points
func GetMerchCoinLeaderboard(db *mongo.Database, limit int) (string, error) {
	if limit <= 0 {
		limit = 10 // Default to top 10
	}

	// Update user stats first
	err := CalculateMerchCoinUserStats(db)
	if err != nil {
		return "", fmt.Errorf("error calculating user stats: %v", err)
	}

	// Query for top users by total points
	findOptions := options.Find().SetSort(bson.M{"totalPoints": -1}).SetLimit(int64(limit))

	cursor, err := db.Collection("merchcoinuserstats").Find(context.Background(), bson.M{}, findOptions)
	if err != nil {
		return "", fmt.Errorf("error querying leaderboard: %v", err)
	}
	defer cursor.Close(context.Background())

	// Decode results
	type UserStats struct {
		Name             string    `bson:"name"`
		PhoneNumber      string    `bson:"phoneNumber"`
		NPM              string    `bson:"npm"`
		WalletAddress    string    `bson:"walletAddress"`
		TotalPoints      float64   `bson:"totalPoints"`
		TotalAmount      float64   `bson:"totalAmount"`
		TransactionCount int       `bson:"transactionCount"`
		LastTransaction  time.Time `bson:"lastTransaction"`
	}

	var topUsers []UserStats
	if err := cursor.All(context.Background(), &topUsers); err != nil {
		return "", fmt.Errorf("error decoding leaderboard results: %v", err)
	}

	if len(topUsers) == 0 {
		return "*MerchCoin Leaderboard*\n\nNo data available yet.", nil
	}

	// Build message
	message := "*MerchCoin Points Leaderboard*\n"
	message += fmt.Sprintf("Top %d Users by Total Points\n\n", limit)

	for i, user := range topUsers {
		npmDisplay := user.NPM
		if npmDisplay == "" {
			npmDisplay = "N/A"
		}

		var displayWallet string = "Not Set"
		if user.WalletAddress != "" {
			if len(user.WalletAddress) > 12 {
				displayWallet = user.WalletAddress[:6] + "..." + user.WalletAddress[len(user.WalletAddress)-6:]
			} else {
				displayWallet = user.WalletAddress
			}
		}

		lastTxDate := "Never"
		if !user.LastTransaction.IsZero() {
			lastTxDate = user.LastTransaction.Format("2006-01-02")
		}

		message += fmt.Sprintf("%d. %s (%s)\n   NPM: %s\n   Total Points: %.2f\n   Total Amount: %.8f MBC\n   Wallet: %s\n   Transactions: %d\n   Last Transaction: %s\n\n",
			i+1,
			user.Name,
			user.PhoneNumber,
			npmDisplay,
			user.TotalPoints,
			user.TotalAmount,
			displayWallet,
			user.TransactionCount,
			lastTxDate)
	}

	return message, nil
}

// SendMerchCoinLeaderboardToGroups sends the MerchCoin leaderboard to specified groups
func SendMerchCoinLeaderboardToGroups(db *mongo.Database) error {
	// Generate leaderboard
	leaderboard, err := GetMerchCoinLeaderboard(db, 10) // Top 10 users
	if err != nil {
		return fmt.Errorf("error generating leaderboard: %v", err)
	}

	// Only send to specific group IDs
	allowedGroupIDs := []string{"120363298977628161", "120363022595651310"}

	// Send to each allowed group
	for _, groupID := range allowedGroupIDs {
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: leaderboard,
		}

		_, _, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			return fmt.Errorf("failed to send leaderboard to group %s: %v", groupID, err)
		}
	}

	return nil
}

// MerchCoinPointsReportHandler handles sending MerchCoin points reports
func MerchCoinPointsReportHandler(w http.ResponseWriter, r *http.Request) {
	var resp model.Response

	err := SendMerchCoinPointsReportToGroups(config.Mongoconn)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to send MerchCoin points report"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, resp)
		return
	}

	resp.Status = "Success"
	resp.Info = "MerchCoin Points Report"
	resp.Response = "MerchCoin points report has been sent to the specified groups"
	at.WriteJSON(w, http.StatusOK, resp)
}

// MerchCoinPointsDailyReportHandler handles sending daily MerchCoin points reports
func MerchCoinPointsDailyReportHandler(w http.ResponseWriter, r *http.Request) {
	var resp model.Response

	err := SendMerchCoinPointsDailyReportToGroups(config.Mongoconn)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to send MerchCoin points daily report"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, resp)
		return
	}

	resp.Status = "Success"
	resp.Info = "MerchCoin Daily Points Report"
	resp.Response = "MerchCoin daily points report has been sent to the specified groups"
	at.WriteJSON(w, http.StatusOK, resp)
}

// RecalculateMerchCoinPointsHandler handles recalculating MerchCoin points for a date range
func RecalculateMerchCoinPointsHandler(w http.ResponseWriter, r *http.Request) {
	var resp model.Response

	// Parse start and end dates from query parameters
	startDateStr := r.URL.Query().Get("startDate")
	endDateStr := r.URL.Query().Get("endDate")

	if startDateStr == "" || endDateStr == "" {
		resp.Status = "Error"
		resp.Info = "Missing required parameters"
		resp.Response = "startDate and endDate are required query parameters (format: YYYY-MM-DD)"
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Invalid startDate format"
		resp.Response = "startDate must be in YYYY-MM-DD format"
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Invalid endDate format"
		resp.Response = "endDate must be in YYYY-MM-DD format"
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}

	// Ensure start date is before end date
	if startDate.After(endDate) {
		resp.Status = "Error"
		resp.Info = "Invalid date range"
		resp.Response = "startDate must be before endDate"
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}

	// Perform recalculation
	err = RecalculateMerchCoinPoints(config.Mongoconn, startDate, endDate)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to recalculate MerchCoin points"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, resp)
		return
	}

	resp.Status = "Success"
	resp.Info = "MerchCoin Points Recalculation"
	resp.Response = fmt.Sprintf("Successfully recalculated MerchCoin points for %s to %s", startDateStr, endDateStr)
	at.WriteJSON(w, http.StatusOK, resp)
}

// MerchCoinLeaderboardHandler handles generating and sending the MerchCoin leaderboard
func MerchCoinLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	var resp model.Response

	// Get limit parameter (optional)
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default to top 10

	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			resp.Status = "Error"
			resp.Info = "Invalid limit parameter"
			resp.Response = "limit must be a positive integer"
			at.WriteJSON(w, http.StatusBadRequest, resp)
			return
		}
	}

	// Get the leaderboard content
	leaderboard, err := GetMerchCoinLeaderboard(config.Mongoconn, limit)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to generate MerchCoin leaderboard"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, resp)
		return
	}

	// Return the leaderboard content directly
	resp.Status = "Success"
	resp.Info = "MerchCoin Leaderboard"
	resp.Response = leaderboard
	at.WriteJSON(w, http.StatusOK, resp)
}

// SendMerchCoinLeaderboardHandler handles sending the MerchCoin leaderboard to groups
func SendMerchCoinLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	var resp model.Response

	err := SendMerchCoinLeaderboardToGroups(config.Mongoconn)
	if err != nil {
		resp.Status = "Error"
		resp.Info = "Failed to send MerchCoin leaderboard"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, resp)
		return
	}

	resp.Status = "Success"
	resp.Info = "MerchCoin Leaderboard"
	resp.Response = "MerchCoin leaderboard has been sent to the specified groups"
	at.WriteJSON(w, http.StatusOK, resp)
}
