package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
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
	// Discord webhook URL for logging
	DiscordWebhookURL = "https://discord.com/api/webhooks/1348044639818485790/DOsYYebYjrTN48wZVDOPrO4j20X5J3pMAbOdPOUkrJuiXk5niqOjV9ZZ2r06th0jXMhh"
)

// Discord message structure
type DiscordMessage struct {
	Content string `json:"content"`
}

// Helper function to send logs to Discord
func sendDiscordLog(message string) {
	// Prepare the message
	discordMsg := DiscordMessage{
		Content: message,
	}
	
	// Convert to JSON
	jsonData, err := json.Marshal(discordMsg)
	if err != nil {
		log.Printf("Error marshaling Discord message: %v", err)
		return
	}
	
	// Send to Discord webhook
	resp, err := http.Post(DiscordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending message to Discord: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Discord webhook returned non-success status: %d", resp.StatusCode)
	}
}

// InitializePaymentTotal initializes the total payments collection if it doesn't exist
func InitializePaymentTotal() {
	var total model.PaymentTotal
	err := config.Mongoconn.Collection("merchtotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Total document doesn't exist, create it
		_, err = config.Mongoconn.Collection("merchtotals").InsertOne(context.Background(), model.PaymentTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		if err != nil {
			log.Printf("Error initializing payment totals: %v", err)
			sendDiscordLog("❌ **ERROR**: Failed to initialize payment totals: " + err.Error())
		} else {
			log.Println("Initialized payment totals successfully")
			sendDiscordLog("✅ **SUCCESS**: Payment totals initialized successfully")
		}
	}
}

// Helper function to update payment totals
func updatePaymentTotal(amount float64) {
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
	
	var result model.PaymentTotal
	err := config.Mongoconn.Collection("merchtotals").FindOneAndUpdate(
		context.Background(),
		bson.M{},
		update,
		opts,
	).Decode(&result)
	
	if err != nil {
		log.Printf("Error updating payment totals: %v", err)
		sendDiscordLog("❌ **ERROR**: Failed to update payment totals: " + err.Error())
	} else {
		sendDiscordLog("💰 **PAYMENT**: Updated total amount: +" + strconv.FormatFloat(amount, 'f', 2, 64) + " IDR")
	}
}

// CreateOrder handles the creation of a new payment order
func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var request model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendDiscordLog("❌ **ERROR**: Invalid request body in CreateOrder: " + err.Error())
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request
	if request.Name == "" || request.Amount <= 0 {
		sendDiscordLog("❌ **ERROR**: Invalid order parameters. Name: " + request.Name + ", Amount: " + strconv.FormatFloat(request.Amount, 'f', 2, 64))
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Name and valid amount are required",
		})
		return
	}

	// Check if someone is currently paying
	var queue model.Queue
	err := config.Mongoconn.Collection("merchqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err == nil && queue.IsProcessing {
		sendDiscordLog("⏳ **QUEUE**: Payment already in progress, request from " + request.Name + " for amount " + strconv.FormatFloat(request.Amount, 'f', 2, 64) + " IDR was queued")
		at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
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
	newOrder := model.Order{
		OrderID:   orderID,
		Name:      request.Name,
		Amount:    request.Amount,
		Timestamp: time.Now(),
		Status:    "pending",
	}

	_, err = config.Mongoconn.Collection("merchorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to create order in database: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	expiryTime := time.Now().Add(50 * time.Second)
	_, err = config.Mongoconn.Collection("merchqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   true,
			"currentOrderId": orderID,
			"expiryTime":     expiryTime,
		}},
	)

	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to update queue status: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// Log successful order creation
	sendDiscordLog(fmt.Sprintf("🛒 **NEW ORDER**: %s created order for %s IDR. Order ID: %s", 
		request.Name, 
		strconv.FormatFloat(request.Amount, 'f', 2, 64),
		orderID))

	// Set up expiry timer
	go func() {
		time.Sleep(50 * time.Second)

		// Check if this order is still the current one
		var currentQueue model.Queue
		err := config.Mongoconn.Collection("merchqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking queue for timeout: %v", err)
			return
		}

		if currentQueue.CurrentOrderID == orderID {
			// Update order status to failed
			_, err = config.Mongoconn.Collection("merchorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{"status": "failed"}},
			)
			if err != nil {
				log.Printf("Error updating order status: %v", err)
				sendDiscordLog("❌ **ERROR**: Failed to update expired order status: " + err.Error())
			}

			// Reset queue
			_, err = config.Mongoconn.Collection("merchqueue").UpdateOne(
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
				sendDiscordLog("❌ **ERROR**: Failed to reset queue after expiry: " + err.Error())
			}
			
			sendDiscordLog(fmt.Sprintf("⏱️ **EXPIRED**: Order %s for %s (%s IDR) has expired", 
				orderID, 
				newOrder.Name, 
				strconv.FormatFloat(newOrder.Amount, 'f', 2, 64)))
		}
	}()

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success:      true,
		OrderID:      orderID,
		ExpiryTime:   expiryTime,
		QRISImageURL: "/static/qris.jpeg",
	})
}

// CheckPayment checks the payment status of an order
func CheckPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.Order
	err := config.Mongoconn.Collection("merchorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendDiscordLog("❓ **CHECK**: Order not found - ID: " + orderID)
		at.WriteJSON(w, http.StatusNotFound, model.PaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Status:  order.Status,
	})
}

// ConfirmPayment confirms a payment manually (simulation)
func ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.Order
	err := config.Mongoconn.Collection("merchorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to confirm payment - Order not found: " + orderID)
		at.WriteJSON(w, http.StatusNotFound, model.PaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("merchorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{"status": "success"}},
	)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to update order status during confirmation: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updatePaymentTotal(order.Amount)

	// Reset queue
	_, err = config.Mongoconn.Collection("merchqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to reset queue after confirmation: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}
	
	sendDiscordLog(fmt.Sprintf("✅ **MANUAL CONFIRM**: Order %s for %s confirmed manually. Amount: %s IDR", 
		orderID, 
		order.Name, 
		strconv.FormatFloat(order.Amount, 'f', 2, 64)))

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

// GetQueueStatus returns the current status of the payment queue
func GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	var queue model.Queue
	err := config.Mongoconn.Collection("merchqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// If no queue document exists, initialize it
		InitializeQueue(w, r)
		return
	}

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success:      true,
		IsProcessing: queue.IsProcessing,
		ExpiryTime:   queue.ExpiryTime,
	})
}

// GetTotalPayments returns the total payment amount and count
func GetTotalPayments(w http.ResponseWriter, r *http.Request) {
	var total model.PaymentTotal
	err := config.Mongoconn.Collection("merchtotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Initialize totals if not found
		InitializePaymentTotal()
		at.WriteJSON(w, http.StatusOK, model.PaymentTotal{
			TotalAmount: 0,
			Count:       0,
			LastUpdated: time.Now(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, total)
}

// ConfirmByNotification processes QRIS payment notifications
func ConfirmByNotification(w http.ResponseWriter, r *http.Request) {
	var request model.NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendDiscordLog("❌ **ERROR**: Invalid notification request: " + err.Error())
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Log notification for debugging
	log.Printf("Received notification: %s", request.NotificationText)
	sendDiscordLog("📥 **NOTIFICATION**: Received notification: " + request.NotificationText)

	// Check if this is a QRIS payment notification
	if !strings.Contains(request.NotificationText, "Pembayaran QRIS") {
		sendDiscordLog("❌ **REJECTED**: Not a QRIS payment notification")
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Not a QRIS payment notification",
		})
		return
	}

	// Extract payment amount with regex - format: "Pembayaran QRIS Rp 1 di Informatika Digital Bisnis, PRNGPNG telah diterima."
	re := regexp.MustCompile(`Pembayaran QRIS Rp\s*(\d+(?:[.,]\d+)?)`)
	matches := re.FindStringSubmatch(request.NotificationText)
	
	if len(matches) < 2 {
		sendDiscordLog("❌ **ERROR**: Cannot extract payment amount from notification")
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Cannot extract payment amount from notification",
		})
		return
	}
	
	// Clean number format
	amountStr := strings.ReplaceAll(matches[1], ".", "")
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	
	// Convert to float
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Invalid payment amount: " + amountStr)
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid payment amount",
		})
		return
	}

	// Find order with EXACTLY matching amount and pending status
	ctx := context.Background()
	var order model.Order
	
	// Create filter for EXACT amount and pending status
	filter := bson.M{
		"amount": amount, // Exact match only
		"status": "pending",
	}
	
	// Add sort by latest timestamp
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})
	
	err = config.Mongoconn.Collection("merchorders").FindOne(ctx, filter, opts).Decode(&order)
	if err != nil {
		sendDiscordLog(fmt.Sprintf("❌ **PAYMENT FAILED**: No pending order found with exact amount: %s IDR", amountStr))
		at.WriteJSON(w, http.StatusNotFound, model.PaymentResponse{
			Success: false,
			Message: "No pending order found with amount: " + amountStr,
		})
		return
	}

	// Update order status to success
	_, err = config.Mongoconn.Collection("merchorders").UpdateOne(
		ctx,
		bson.M{"orderId": order.OrderID},
		bson.M{"$set": bson.M{"status": "success"}},
	)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to update order status: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updatePaymentTotal(amount)

	// Reset queue
	_, err = config.Mongoconn.Collection("merchqueue").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendDiscordLog("❌ **ERROR**: Failed to reset queue after notification confirmation: " + err.Error())
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Log successful confirmation
	log.Printf("Payment confirmed from notification for amount: Rp%v, Order ID: %s", amount, order.OrderID)
	sendDiscordLog(fmt.Sprintf("✅ **PAYMENT SUCCESS**: Order %s for %s confirmed via notification. Amount: %s IDR", 
		order.OrderID, 
		order.Name, 
		strconv.FormatFloat(amount, 'f', 2, 64)))

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

// InitializeQueue creates the queue document if it doesn't exist
func InitializeQueue(w http.ResponseWriter, r *http.Request) {
	var queue model.Queue
	err := config.Mongoconn.Collection("merchqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Queue document doesn't exist, create it
		_, err = config.Mongoconn.Collection("merchqueue").InsertOne(context.Background(), model.Queue{
			IsProcessing:   false,
			CurrentOrderID: "",
			ExpiryTime:     time.Time{},
		})
		
		if err != nil {
			sendDiscordLog("❌ **ERROR**: Failed to initialize queue: " + err.Error())
			at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
				Success: false,
				Message: "Error initializing queue",
			})
			return
		}
		
		// Initialize payment totals as well
		InitializePaymentTotal()
		
		log.Println("Initialized payment queue successfully")
		sendDiscordLog("✅ **SYSTEM**: Payment queue initialized successfully")
		at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
			Success: true,
			Message: "Queue initialized successfully",
		})
		return
	}
	
	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success:      true,
		Message:      "Queue already exists",
		IsProcessing: queue.IsProcessing,
		ExpiryTime:   queue.ExpiryTime,
	})
}