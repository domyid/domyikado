package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
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
	expiryTime := time.Now().Add(300 * time.Second) // 5 minutes expiry
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
		time.Sleep(300 * time.Second)

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

	// If payment is still pending, check mempool
	if order.Status == "pending" {
		// Step 1: Check mempool
		mempoolStatus, txid, amount, err := checkMerchCoinMempool()

		if err != nil {
			// Error checking mempool
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking mempool failed: " + err.Error(),
				Step1Complete: false,
				Step3Complete: false,
			})
			return
		}

		if !mempoolStatus {
			// No transaction in mempool
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "No transaction in mempool.",
				Step1Complete: false,
				Step3Complete: false,
			})
			return
		}

		// If we found a transaction in mempool, proceed directly to step 3
		if txid != "" {
			// Step 3: Verify transaction details (skipping step 2)
			txDetails, actualAmount, err := checkMerchCoinTxDetails(txid)
			if err != nil {
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:       true,
					Status:        "pending",
					Message:       "Transaction found, but error checking details: " + err.Error(),
					Step1Complete: true,
					Step3Complete: false,
					TxID:          txid,
					Amount:        amount,
				})
				return
			}

			if !txDetails {
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:       true,
					Status:        "pending",
					Message:       "Transaction found, but details verification failed.",
					Step1Complete: true,
					Step3Complete: false,
					TxID:          txid,
					Amount:        amount,
				})
				return
			}

			// All steps complete, mark payment as successful
			// Update order status to success
			_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status": "success",
					"txid":   txid,
					"amount": actualAmount,
				}},
			)
			if err != nil {
				log.Printf("Error updating MerchCoin order status: %v", err)
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:       true,
					Status:        "pending",
					Message:       "Transaction verified but error updating order status: " + err.Error(),
					Step1Complete: true,
					Step3Complete: true,
					TxID:          txid,
					Amount:        actualAmount,
				})
				return
			}

			// Update payment totals
			updateMerchCoinPaymentTotal(actualAmount)

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

			// Return success response
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:       true,
				Status:        "success",
				Message:       "Payment confirmed successfully!",
				Step1Complete: true,
				Step3Complete: true,
				TxID:          txid,
				Amount:        actualAmount,
			})
			return
		}
	}

	// If payment is already processed (success or failed)
	at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
		Success: true,
		Status:  order.Status,
		TxID:    order.TxID,
		Amount:  order.Amount,
	})
}

// Step 1: Check mempool for transactions
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
		// Return the first transaction from mempool
		tx := mempoolResp.Result.Tx[0]
		amount := float64(tx.Satoshis) / 100000000 // Convert satoshis to MBC
		return true, tx.TxID, amount, nil
	}

	return false, "", 0, nil
}

// Step 2: Check if transaction exists in history
func checkMerchCoinTxHistory(txid string) (bool, error) {
	// API URL for checking transaction history
	url := "https://api.mbc.wiki/history/" + MerchCoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Parse response
	var historyResp model.MerchCoinHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		return false, err
	}

	// Check for errors in API response
	if historyResp.Error != nil {
		return false, errors.New("API error: " + *historyResp.Error)
	}

	// Check if transaction exists in history
	for _, tx := range historyResp.Result.Tx {
		if tx == txid {
			return true, nil
		}
	}

	return false, nil
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

	// Transaction amount in MBC
	amount := float64(txResp.Result.Amount) / 100000000

	// Transaction is valid
	return true, amount, nil
}

// ManuallyConfirmMerchCoinPayment manually confirms a MerchCoin payment
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
