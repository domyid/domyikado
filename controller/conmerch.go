package controller

import (
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

// WebSocket clients
var (
	clients = make(map[*http.ResponseWriter]bool)
)

// CreateOrder handles the creation of a new payment order
func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var request model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request
	if request.Name == "" || request.Amount <= 0 {
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
		at.WriteJSON(w, http.StatusConflict, model.PaymentResponse{
			Success:     false,
			Message:     "Sedang ada pembayaran berlangsung. Silakan tunggu.",
			QueueStatus: true,
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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

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
			}
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
	err := config.Mongoconn.Collection("orders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Notify all WebSocket clients (would be implemented with proper WebSocket handling)
	// notifyPaymentSuccess(orderID)

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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error checking queue",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success:      true,
		IsProcessing: queue.IsProcessing,
		ExpiryTime:   queue.ExpiryTime,
	})
}

// ConfirmPaymentByAmount confirms payment based on the amount received
func ConfirmPaymentByAmount(w http.ResponseWriter, r *http.Request) {
	var request model.ConfirmByAmountRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validation
	if request.Amount <= 0 {
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid amount",
		})
		return
	}

	// Find order with matching amount and pending status
	ctx := context.Background()
	var order model.Order

	// Create filter to find order with pending status and matching amount
	filter := bson.M{
		"amount": request.Amount,
		"status": "pending",
	}

	// Add sort by latest timestamp to get the latest order with that amount
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})

	err := config.Mongoconn.Collection("merchorders").FindOne(ctx, filter, opts).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.PaymentResponse{
			Success: false,
			Message: "No pending order found with this amount",
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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Log successful confirmation
	log.Printf("Payment confirmed by amount: Rp%v for Order ID: %s", request.Amount, order.OrderID)

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

// ConfirmByNotification processes QRIS payment notifications from MacroDroid
func ConfirmByNotification(w http.ResponseWriter, r *http.Request) {
	var request model.NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Log notification for debugging
	log.Printf("Received notification: %s", request.NotificationText)

	// Check if this is a QRIS payment notification
	if !strings.Contains(request.NotificationText, "Pembayaran QRIS") {
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Not a QRIS payment notification",
		})
		return
	}

	// Extract payment amount with regex
	re := regexp.MustCompile(`Rp[\s]*([0-9.,]+)`)
	matches := re.FindStringSubmatch(request.NotificationText)

	if len(matches) < 2 {
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
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid payment amount",
		})
		return
	}

	// Find order with matching amount and pending status
	ctx := context.Background()
	var order model.Order

	// Create filter to find order with pending status and matching amount
	filter := bson.M{
		"amount": amount,
		"status": "pending",
	}

	// Add sort by latest timestamp
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})

	err = config.Mongoconn.Collection("merchorders").FindOne(ctx, filter, opts).Decode(&order)
	if err != nil {
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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

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
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Log successful confirmation
	log.Printf("Payment confirmed from notification for amount: Rp%v, Order ID: %s", amount, order.OrderID)

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

// InitializeQueue creates the queue document if it doesn't exist
func InitializeQueue() {
	var queue model.Queue
	err := config.Mongoconn.Collection("merchqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Queue document doesn't exist, create it
		config.Mongoconn.Collection("merchqueue").InsertOne(context.Background(), model.Queue{
			IsProcessing:   false,
			CurrentOrderID: "",
			ExpiryTime:     time.Time{},
		})
		log.Println("Initialized payment queue successfully")
	}
}
