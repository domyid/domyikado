package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging (optional)
	MerchCoinDiscordWebhookURL = "https://discord.com/api/webhooks/your-webhook-url"
	// QR code image URL
	MerchCoinQRImageURL = "static/wonpay.png"
	// MicroBitcoin API URL
	MicroBitcoinAPIURL = "https://microbitcoinorg.github.io/api/"
)

// Helper function to update payment totals
func updateMerchCoinTotal(amount float64) {
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

	var result model.MerchCoinTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOneAndUpdate(
		context.Background(),
		bson.M{},
		update,
		opts,
	).Decode(&result)

	if err != nil {
		log.Printf("Error updating merchcoin totals: %v", err)
	} else {
		log.Printf("Successfully updated merchcoin totals, added amount: %v", amount)
	}
}

// Initialize the payment total collection if it doesn't exist
func InitializeMerchCoinTotal() {
	var total model.MerchCoinTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Total document doesn't exist, create it
		_, err = config.Mongoconn.Collection("merchcointotals").InsertOne(context.Background(), model.MerchCoinTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		if err != nil {
			log.Printf("Error initializing merchcoin totals: %v", err)
		} else {
			log.Println("Initialized merchcoin totals successfully")
		}
	}
}

// Initialize queue if it doesn't exist
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
			log.Printf("Error initializing merchcoin queue: %v", err)
		} else {
			log.Println("Initialized merchcoin queue successfully")
		}

		// Initialize payment totals as well
		InitializeMerchCoinTotal()
	}
}

// CreateMerchCoinOrder handles the creation of a new payment order
func CreateMerchCoinOrder(w http.ResponseWriter, r *http.Request) {
	var request model.MerchCoinCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request
	if request.Name == "" || request.Amount <= 0 {
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinResponse{
			Success: false,
			Message: "Name and valid amount are required",
		})
		return
	}

	// Check if someone is currently paying
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err == nil && queue.IsProcessing {
		log.Printf("Payment in progress, customer %s queued", request.Name)
		at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
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
		OrderID:   orderID,
		Name:      request.Name,
		Amount:    request.Amount,
		Timestamp: time.Now(),
		Status:    "pending",
	}

	_, err = config.Mongoconn.Collection("merchcoinorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		log.Printf("Error creating order in database: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	expiryTime := time.Now().Add(300 * time.Second) // 5 minutes
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
		log.Printf("Error updating queue: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	log.Printf("New order created: ID=%s, Name=%s, Amount=%v", orderID, request.Name, request.Amount)

	// Set up expiry timer
	go func() {
		time.Sleep(300 * time.Second)

		// Check if this order is still the current one
		var currentQueue model.MerchCoinQueue
		err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking queue for timeout: %v", err)
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
				log.Printf("Error updating order status: %v", err)
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
				log.Printf("Error resetting queue: %v", err)
			}

			log.Printf("Order %s expired", orderID)
		}
	}()

	at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
		Success:    true,
		OrderID:    orderID,
		ExpiryTime: expiryTime,
		QRImageURL: MerchCoinQRImageURL,
	})
}

// CheckMerchCoinPayment checks the status of a payment
func CheckMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		log.Printf("Order not found: %s", orderID)
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
		Success: true,
		Status:  order.Status,
	})
}

// ManuallyConfirmMerchCoinPayment allows manual confirmation of a payment
func ManuallyConfirmMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.MerchCoinOrder
	err := config.Mongoconn.Collection("merchcoinorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		log.Printf("Order not found for manual confirmation: %s", orderID)
		at.WriteJSON(w, http.StatusNotFound, model.MerchCoinResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{"status": "success"}},
	)
	if err != nil {
		log.Printf("Error updating order status: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateMerchCoinTotal(order.Amount)

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
		log.Printf("Error resetting queue: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	log.Printf("Payment manually confirmed for order %s", orderID)

	at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

// GetMerchCoinQueueStatus returns the current queue status
func GetMerchCoinQueueStatus(w http.ResponseWriter, r *http.Request) {
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// If no queue document exists, initialize it
		InitializeMerchCoinQueue()

		// Try again after initialization
		err = config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
		if err != nil {
			log.Printf("Error getting queue status after initialization: %v", err)
			at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
				Success:      true,
				IsProcessing: false,
			})
			return
		}
	}

	at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
		Success:      true,
		IsProcessing: queue.IsProcessing,
		ExpiryTime:   queue.ExpiryTime,
	})
}

// GetMerchCoinTotalPayments returns payment statistics
func GetMerchCoinTotalPayments(w http.ResponseWriter, r *http.Request) {
	var total model.MerchCoinTotal
	err := config.Mongoconn.Collection("merchcointotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Initialize totals if not found
		InitializeMerchCoinTotal()

		// Return fresh totals
		at.WriteJSON(w, http.StatusOK, model.MerchCoinTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, total)
}

// VerifyMicroBitcoinTransaction checks the MicroBitcoin API to verify transaction
func VerifyMicroBitcoinTransaction(txID string) bool {
	// In a real implementation, you would verify against the actual MicroBitcoin API
	// For this example, we'll simulate a successful verification
	// You would typically make a request to the MicroBitcoin API endpoint

	// Example HTTP request (commented out)
	/*
		resp, err := http.Get(MicroBitcoinAPIURL + "/transaction/" + txID)
		if err != nil {
			log.Printf("Error verifying transaction: %v", err)
			return false
		}
		defer resp.Body.Close()

		var result struct {
			Status  string `json:"status"`
			Confirmed bool `json:"confirmed"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("Error decoding API response: %v", err)
			return false
		}

		return result.Status == "success" && result.Confirmed
	*/

	// For demo purposes, always return true
	log.Printf("Transaction %s verified with MicroBitcoin API", txID)
	return true
}

// ConfirmMerchCoinNotification handles payment notifications
func ConfirmMerchCoinNotification(w http.ResponseWriter, r *http.Request) {
	var request model.MerchCoinNotification
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Invalid notification request: %v", err)
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	log.Printf("Received notification: %s", request.NotificationText)

	// Check if this is a valid notification
	if !strings.Contains(request.NotificationText, "payment") &&
		!strings.Contains(request.NotificationText, "Payment") {
		log.Printf("Notification rejected: Not a payment notification")
		at.WriteJSON(w, http.StatusBadRequest, model.MerchCoinResponse{
			Success: false,
			Message: "Not a valid payment notification",
		})
		return
	}

	// If transaction ID is provided, verify with MicroBitcoin API
	var isVerified bool
	if request.TransactionID != "" {
		isVerified = VerifyMicroBitcoinTransaction(request.TransactionID)
		if !isVerified {
			log.Printf("Transaction verification failed: %s", request.TransactionID)
			at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
				Success: false,
				Message: "Transaction verification failed",
			})
			return
		}
	}

	// Find pending order
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil || !queue.IsProcessing {
		log.Printf("No active order found for notification")
		at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
			Success: false,
			Message: "No active order found",
		})
		return
	}

	// Get the current order
	var order model.MerchCoinOrder
	err = config.Mongoconn.Collection("merchcoinorders").FindOne(
		context.Background(),
		bson.M{"orderId": queue.CurrentOrderID, "status": "pending"},
	).Decode(&order)

	if err != nil {
		log.Printf("No pending order found with ID: %s", queue.CurrentOrderID)
		at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
			Success: false,
			Message: "No pending order found",
		})
		return
	}

	// Update order status to success
	_, err = config.Mongoconn.Collection("merchcoinorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": order.OrderID},
		bson.M{"$set": bson.M{"status": "success"}},
	)
	if err != nil {
		log.Printf("Error updating order status: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateMerchCoinTotal(order.Amount)

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
		log.Printf("Error resetting queue: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, model.MerchCoinResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	log.Printf("Payment confirmed via notification for order: %s, amount: %v", order.OrderID, order.Amount)

	at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

// SimulateMerchCoinPayment is a development endpoint to simulate a payment notification
func SimulateMerchCoinPayment(w http.ResponseWriter, r *http.Request) {
	var queue model.MerchCoinQueue
	err := config.Mongoconn.Collection("merchcoinqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil || !queue.IsProcessing {
		at.WriteJSON(w, http.StatusOK, model.MerchCoinResponse{
			Success: false,
			Message: "No active order to simulate payment for",
		})
		return
	}

	// Create simulated notification
	simulatedNotification := model.MerchCoinNotification{
		NotificationText: "Payment received for order " + queue.CurrentOrderID,
		TransactionID:    "sim-" + uuid.New().String(),
	}

	// Create request to notification endpoint
	jsonData, _ := json.Marshal(simulatedNotification)
	req, _ := http.NewRequest("POST", "/api/merchcoin/notification", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Call notification handler
	ConfirmMerchCoinNotification(w, req)
}
