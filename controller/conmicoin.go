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
	MerchCoinDiscordWebhookURL = "https://discord.com/api/webhooks/1348599740664647772/svXeCm0CPQ1uVK-R7TkYLtVX2DRpmlhW7tfiBEqY9J4Mc0IpLIpCdKFm-1rv7kx0zIyc"
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
	expiryTime := time.Now().Add(10 * 60 * time.Second) // 10 minutes expiry
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
		time.Sleep(10 * 60 * time.Second) // 10 minutes

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
				Step2Complete: false,
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
				Step2Complete: false,
				Step3Complete: false,
			})
			return
		}

		// If we found a transaction in mempool, save it and return a response indicating
		// we need to wait 3 minutes before proceeding to step 2
		if txid != "" {
			if !order.Step1Complete {
				// Only set these values if Step 1 is not already complete
				step1CompleteTime := time.Now()
				delayStep2Until := step1CompleteTime.Add(3 * time.Minute)

				log.Printf("Setting Step 1 complete. Current time: %v, Delay until: %v",
					step1CompleteTime.Format(time.RFC3339), delayStep2Until.Format(time.RFC3339))

				// Update order with step 1 completion info
				updateResult, err := config.Mongoconn.Collection("merchcoinorders").UpdateOne(
					context.Background(),
					bson.M{"orderId": orderID},
					bson.M{"$set": bson.M{
						"step1Complete":     true,
						"txid":              txid,
						"pendingAmount":     amount,
						"step1CompleteTime": step1CompleteTime,
						"delayStep2Until":   delayStep2Until,
					}},
				)
				if err != nil {
					log.Printf("Error updating order with step 1 completion: %v", err)
				} else {
					log.Printf("Updated step 1 completion: %v document(s) modified", updateResult.ModifiedCount)
				}

				// Verify the update by retrieving the document again
				var updatedOrder model.MerchCoinOrder
				err = config.Mongoconn.Collection("merchcoinorders").FindOne(
					context.Background(),
					bson.M{"orderId": orderID},
				).Decode(&updatedOrder)

				if err != nil {
					log.Printf("Error retrieving updated order: %v", err)
				} else {
					log.Printf("Verified Step1Complete: %v, DelayStep2Until: %v",
						updatedOrder.Step1Complete, updatedOrder.DelayStep2Until.Format(time.RFC3339))
				}

				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in mempool. Waiting 3 minutes before checking transaction history...",
					Step1Complete:     true,
					Step2Complete:     false,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: step1CompleteTime,
					DelayStep2Until:   delayStep2Until,
				})
				return
			}

			// Check if we've already passed step 1
		} else if order.Step1Complete && order.TxID != "" {
			// Get the transaction ID from the order
			txid := order.TxID
			amount := order.PendingAmount

			// Check if we should start Step 2 or are still in delay period
			now := time.Now()
			if now.Before(order.DelayStep2Until) {
				// Still waiting for the 3-minute delay after Step 1
				remainingTime := order.DelayStep2Until.Sub(now).Seconds()
				log.Printf("Still waiting for Step 2 delay. Remaining time: %.0f seconds", remainingTime)
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           fmt.Sprintf("Transaction found in mempool. Waiting %.0f seconds before checking transaction history...", remainingTime),
					Step1Complete:     true,
					Step2Complete:     false,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
				})
				return
			}

			// It's time to check Step 2 - check if transaction ID exists in history
			txHistory, err := checkMerchCoinTxHistory(txid)
			if err != nil {
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in mempool, but error checking history: " + err.Error(),
					Step1Complete:     true,
					Step2Complete:     false,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
				})
				return
			}

			if !txHistory {
				// Transaction not yet in history
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in mempool, checking transaction history... (This may take a few moments)",
					Step1Complete:     true,
					Step2Complete:     false,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
				})
				return
			}

			// Step 2 completed - update order and set 30-second delay for Step 3
			if !order.Step2Complete {
				// Only set these values if Step 2 is not already complete
				step2CompleteTime := time.Now()
				delayStep3Until := step2CompleteTime.Add(30 * time.Second)

				log.Printf("Setting Step 2 complete. Current time: %v, Delay until: %v",
					step2CompleteTime.Format(time.RFC3339), delayStep3Until.Format(time.RFC3339))

				// Update order status with Step 2 complete
				updateResult, err := config.Mongoconn.Collection("merchcoinorders").UpdateOne(
					context.Background(),
					bson.M{"orderId": orderID},
					bson.M{"$set": bson.M{
						"step2Complete":     true,
						"step2CompleteTime": step2CompleteTime,
						"delayStep3Until":   delayStep3Until,
					}},
				)
				if err != nil {
					log.Printf("Error updating order with step 2 completion: %v", err)
				} else {
					log.Printf("Updated step 2 completion: %v document(s) modified", updateResult.ModifiedCount)
				}

				// Verify the update by retrieving the document again
				var updatedOrder model.MerchCoinOrder
				err = config.Mongoconn.Collection("merchcoinorders").FindOne(
					context.Background(),
					bson.M{"orderId": orderID},
				).Decode(&updatedOrder)

				if err != nil {
					log.Printf("Error retrieving updated order: %v", err)
				} else {
					log.Printf("Verified Step2Complete: %v, DelayStep3Until: %v",
						updatedOrder.Step2Complete, updatedOrder.DelayStep3Until.Format(time.RFC3339))
				}

				// Return response indicating Step 2 is complete and waiting for Step 3
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in history. Waiting 30 seconds before checking transaction details...",
					Step1Complete:     true,
					Step2Complete:     true,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
					Step2CompleteTime: step2CompleteTime,
					DelayStep3Until:   delayStep3Until,
				})
				return
			}

			// If we've already completed Step 2 before, just return the current state
			at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
				Success:           true,
				Status:            "pending",
				Message:           "Transaction found in history. Waiting before checking transaction details...",
				Step1Complete:     true,
				Step2Complete:     true,
				Step3Complete:     false,
				TxID:              txid,
				Amount:            amount,
				Step1CompleteTime: order.Step1CompleteTime,
				DelayStep2Until:   order.DelayStep2Until,
				Step2CompleteTime: order.Step2CompleteTime,
				DelayStep3Until:   order.DelayStep3Until,
			})
			return
		} else if order.Step1Complete && order.Step2Complete && !order.Step3Complete {
			// We've completed Step 2, now check if we should start Step 3 or are still in delay period
			now := time.Now()
			txid := order.TxID
			amount := order.PendingAmount

			if now.Before(order.DelayStep3Until) {
				// Still waiting for the 30-second delay after Step 2
				remainingTime := order.DelayStep3Until.Sub(now).Seconds()
				log.Printf("Still waiting for Step 3 delay. Remaining time: %.0f seconds", remainingTime)
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           fmt.Sprintf("Transaction found in history. Waiting %.0f seconds before checking transaction details...", remainingTime),
					Step1Complete:     true,
					Step2Complete:     true,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
					Step2CompleteTime: order.Step2CompleteTime,
					DelayStep3Until:   order.DelayStep3Until,
				})
				return
			} else {
				log.Printf("Delay period for Step 3 has passed. Current time: %v, DelayStep3Until: %v",
					now.Format(time.RFC3339), order.DelayStep3Until.Format(time.RFC3339))
			}

			// Time to process Step 3 - verify transaction details
			txDetails, actualAmount, err := checkMerchCoinTxDetails(txid)
			if err != nil {
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in history, but error checking details: " + err.Error(),
					Step1Complete:     true,
					Step2Complete:     true,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
					Step2CompleteTime: order.Step2CompleteTime,
					DelayStep3Until:   order.DelayStep3Until,
				})
				return
			}

			if !txDetails {
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction found in history, but details verification failed.",
					Step1Complete:     true,
					Step2Complete:     true,
					Step3Complete:     false,
					TxID:              txid,
					Amount:            amount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
					Step2CompleteTime: order.Step2CompleteTime,
					DelayStep3Until:   order.DelayStep3Until,
				})
				return
			}
			// All steps complete, mark payment as successful
			// Update order status to success
			_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status":        "success",
					"step3Complete": true,
					"amount":        actualAmount,
				}},
			)
			if err != nil {
				log.Printf("Error updating MerchCoin order status: %v", err)
				at.WriteJSON(w, http.StatusOK, model.MerchCoinPaymentResponse{
					Success:           true,
					Status:            "pending",
					Message:           "Transaction verified but error updating order status: " + err.Error(),
					Step1Complete:     true,
					Step2Complete:     true,
					Step3Complete:     true,
					TxID:              txid,
					Amount:            actualAmount,
					Step1CompleteTime: order.Step1CompleteTime,
					DelayStep2Until:   order.DelayStep2Until,
					Step2CompleteTime: order.Step2CompleteTime,
					DelayStep3Until:   order.DelayStep3Until,
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
				Success:           true,
				Status:            "success",
				Message:           "Payment confirmed successfully!",
				Step1Complete:     true,
				Step2Complete:     true,
				Step3Complete:     true,
				TxID:              txid,
				Amount:            actualAmount,
				Step1CompleteTime: order.Step1CompleteTime,
				DelayStep2Until:   order.DelayStep2Until,
				Step2CompleteTime: order.Step2CompleteTime,
				DelayStep3Until:   order.DelayStep3Until,
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

	// Transaction not found in history - but we don't consider this an error
	// The caller should keep trying until it's found or timeout occurs
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
			"status":        "success",
			"txid":          request.TxID,
			"amount":        request.Amount,
			"step1Complete": true,
			"step2Complete": true,
			"step3Complete": true,
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
			"status":        "success",
			"txid":          request.TxID,
			"amount":        request.Amount,
			"step1Complete": true,
			"step2Complete": true,
			"step3Complete": true,
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
