package controller

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// AuthCredentials stores basic auth credentials
type AuthCredentials struct {
	Username string    `bson:"username" json:"username"`
	Password string    `bson:"password" json:"password"`
	Created  time.Time `bson:"created" json:"created"`
}

// Basic Auth middleware that fetches credentials from MongoDB
func BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Authentication required",
			})
			return
		}

		// Parse auth header
		authParts := strings.Split(authHeader, " ")
		if len(authParts) != 2 || authParts[0] != "Basic" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication format",
			})
			return
		}

		// Decode credentials
		payload, err := base64.StdEncoding.DecodeString(authParts[1])
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication credentials",
			})
			return
		}

		// Split username and password
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication credentials",
			})
			return
		}

		// Get credentials from database
		var dbCreds AuthCredentials
		err = config.Mongoconn.Collection("merchsecret").FindOne(context.Background(), bson.M{}).Decode(&dbCreds)
		if err != nil {
			// Log error but continue with default credentials
			log.Printf("Warning: Could not retrieve auth credentials from database: %v.", err)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Server error retrieving credentials",
			})
			return
		}

		// Verify credentials using constant time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(pair[0]), []byte(dbCreds.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pair[1]), []byte(dbCreds.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			sendDiscordEmbed(
				"🔒 Authentication Failed",
				"Someone tried to access the API with invalid credentials.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "IP Address", Value: r.RemoteAddr, Inline: true},
					{Name: "URL Path", Value: r.URL.Path, Inline: true},
					{Name: "User Agent", Value: r.UserAgent(), Inline: false},
					{Name: "Provided Username", Value: pair[0], Inline: true},
				},
			)
			at.WriteJSON(w, http.StatusUnauthorized, model.PaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid username or password",
			})
			return
		}

		// Authentication successful, call the next handler
		next(w, r)
	}
}

// Discord embed structure
type DiscordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"` // ISO8601 timestamp
}

// Discord embed field
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Discord embed footer
type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

// Discord webhook message structure
type DiscordWebhookMessage struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// Helper function to send logs to Discord with embeds
func sendDiscordEmbed(title, description string, color int, fields []DiscordEmbedField) {
	// Set timestamp to current time
	timestamp := time.Now().Format(time.RFC3339)
	
	// Create embed
	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Footer: &DiscordEmbedFooter{
			Text: "QRIS Payment System",
		},
		Timestamp: timestamp,
	}
	
	// Create message with embed
	webhookMsg := DiscordWebhookMessage{
		Username:  "QRIS Payment Bot",
		AvatarURL: "https://cdn-icons-png.flaticon.com/512/2168/2168252.png", // QR code icon
		Embeds:    []DiscordEmbed{embed},
	}
	
	// Convert to JSON
	jsonData, err := json.Marshal(webhookMsg)
	if err != nil {
		log.Printf("Error marshaling Discord embed: %v", err)
		return
	}
	
	// Send to Discord webhook
	resp, err := http.Post(DiscordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending embed to Discord: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Discord webhook returned non-success status: %d", resp.StatusCode)
	}
}

// Constants for Discord embed colors
const (
	ColorGreen  = 5763719  // Success color (green)
	ColorRed    = 15548997 // Error color (red)
	ColorBlue   = 3447003  // Info color (blue)
	ColorYellow = 16776960 // Warning color (yellow)
	ColorPurple = 10181046 // Special color (purple)
)

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
			sendDiscordEmbed(
				"🔴 Error: Payment Totals Initialization Failed",
				"Failed to initialize payment totals database.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
		} else {
			log.Println("Initialized payment totals successfully")
			sendDiscordEmbed(
				"✅ System: Payment Totals Initialized",
				"Successfully initialized the payment totals database.",
				ColorGreen,
				nil,
			)
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
		sendDiscordEmbed(
			"🔴 Error: Payment Totals Update Failed",
			"Failed to update payment totals in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
	} else {
		sendDiscordEmbed(
			"💰 Payment: Total Updated",
			"Successfully updated payment totals.",
			ColorGreen,
			[]DiscordEmbedField{
				{Name: "Amount Added", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
			},
		)
	}
}
// CreateOrderHandler TANPA Basic Auth
func CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
    CreateOrder(w, r) // Langsung panggil CreateOrder tanpa Basic Auth
}
func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var request model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendDiscordEmbed(
			"🔴 Error: Invalid Request",
			"Failed to process create order request.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request
	if request.Name == "" || request.Amount <= 0 {
		sendDiscordEmbed(
			"🔴 Error: Invalid Order Parameters",
			"Order creation failed due to invalid parameters.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Name", Value: request.Name, Inline: true},
				{Name: "Amount", Value: strconv.FormatFloat(request.Amount, 'f', 2, 64), Inline: true},
			},
		)
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
		sendDiscordEmbed(
			"⏳ Queue: Payment in Progress",
			"Another payment is already in progress.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Customer", Value: request.Name, Inline: true},
				{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(request.Amount, 'f', 2, 64)), Inline: true},
				{Name: "Status", Value: "Queued", Inline: true},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Database Error",
			"Failed to create order in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	expiryTime := time.Now().Add(300 * time.Second)
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
		sendDiscordEmbed(
			"🔴 Error: Queue Update Failed",
			"Failed to update payment queue.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// Log successful order creation
	sendDiscordEmbed(
		"🛒 New Order Created",
		"A new payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: request.Name, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(request.Amount, 'f', 2, 64)), Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)

	// Set up expiry timer
	go func() {
		time.Sleep(300 * time.Second)

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
				sendDiscordEmbed(
					"🔴 Error: Status Update Failed",
					"Failed to update expired order status.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
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
				sendDiscordEmbed(
					"🔴 Error: Queue Reset Failed",
					"Failed to reset queue after order expiry.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}
			
			sendDiscordEmbed(
				"⏱️ Order Expired",
				"A payment order has expired.",
				ColorYellow,
				[]DiscordEmbedField{
					{Name: "Order ID", Value: orderID, Inline: true},
					{Name: "Customer", Value: newOrder.Name, Inline: true},
					{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(newOrder.Amount, 'f', 2, 64)), Inline: true},
					{Name: "Status", Value: "Expired", Inline: true},
				},
			)
		}
	}()

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success:      true,
		OrderID:      orderID,
		ExpiryTime:   expiryTime,
		QRISImageURL: "qris.png",
	})
}

func CheckPaymentHandler(w http.ResponseWriter, r *http.Request) {
    CheckPayment(w, r) // Langsung panggil CheckPayment tanpa Basic Auth
}

func CheckPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.Order
	err := config.Mongoconn.Collection("merchorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendDiscordEmbed(
			"❓ Check Payment",
			"Payment status check for non-existent order.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Status", Value: "Not Found", Inline: true},
			},
		)
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

func ConfirmPaymentHandler(w http.ResponseWriter, r *http.Request) {
    ConfirmPayment(w, r) // Langsung panggil ConfirmPayment tanpa Basic Auth
}

func ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.Order
	err := config.Mongoconn.Collection("merchorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendDiscordEmbed(
			"🔴 Error: Manual Confirmation Failed",
			"Failed to confirm payment manually.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: "Order not found", Inline: false},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update order status during manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset queue after manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}
	
	sendDiscordEmbed(
		"✅ Manual Payment Confirmation",
		"A payment has been confirmed manually.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(order.Amount, 'f', 2, 64)), Inline: true},
			{Name: "Status", Value: "Confirmed", Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

func GetQueueStatusHandler(w http.ResponseWriter, r *http.Request) {
    GetQueueStatus(w, r) // Langsung panggil GetQueueStatus tanpa Basic Auth
}

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

func GetTotalPaymentsHandler(w http.ResponseWriter, r *http.Request) {
    GetTotalPayments(w, r) // Langsung panggil GetTotalPayments tanpa Basic Auth
}

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
func ConfirmByNotificationHandler(w http.ResponseWriter, r *http.Request) {
	BasicAuth(ConfirmByNotification)(w, r)
}

func ConfirmByNotification(w http.ResponseWriter, r *http.Request) {
	var request model.NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendDiscordEmbed(
			"🔴 Error: Invalid Notification",
			"Failed to process payment notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.PaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Log notification for debugging
	log.Printf("Received notification: %s", request.NotificationText)
	sendDiscordEmbed(
		"📥 Notification Received",
		"Received a payment notification.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Notification Text", Value: request.NotificationText, Inline: false},
		},
	)

	// Check if this is a QRIS payment notification
	if !strings.Contains(request.NotificationText, "Pembayaran QRIS") {
		sendDiscordEmbed(
			"❌ Notification Rejected",
			"The received notification is not a QRIS payment.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Notification Text", Value: request.NotificationText, Inline: false},
				{Name: "Reason", Value: "Not a QRIS payment notification", Inline: false},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Amount Extraction Failed",
			"Could not extract payment amount from notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Notification Text", Value: request.NotificationText, Inline: false},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Invalid Amount",
			"The extracted payment amount is invalid.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Extracted Amount", Value: amountStr, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
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
		// Log the attempted payment even if it doesn't match any order
		sendDiscordEmbed(
			"❌ Payment Failed",
			"No pending order found with the exact amount.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
				{Name: "Status", Value: "Failed to Match", Inline: true},
				{Name: "Notification", Value: request.NotificationText, Inline: false},
			},
		)
		
		// Store this as a failed payment attempt in database
		failedPayment := model.Order{
			OrderID:   "unmatched-" + uuid.New().String(),
			Name:      "Unknown",
			Amount:    amount,
			Timestamp: time.Now(),
			Status:    "failed", // Mark as failed directly
		}
		
		_, dbErr := config.Mongoconn.Collection("merchorders").InsertOne(context.Background(), failedPayment)
		if dbErr != nil {
			log.Printf("Error storing failed payment: %v", dbErr)
		}
		
		at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
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
		sendDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update order status after notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: order.OrderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
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
		sendDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset queue after payment confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Log successful confirmation
	log.Printf("Payment confirmed from notification for amount: Rp%v, Order ID: %s", amount, order.OrderID)
	sendDiscordEmbed(
		"✅ Payment Successful",
		"A payment has been confirmed via notification.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: order.OrderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
			{Name: "Status", Value: "Confirmed", Inline: true},
			{Name: "Notification", Value: request.NotificationText, Inline: false},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.PaymentResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

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
			sendDiscordEmbed(
				"🔴 Error: Queue Initialization Failed",
				"Failed to initialize payment queue.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
			at.WriteJSON(w, http.StatusInternalServerError, model.PaymentResponse{
				Success: false,
				Message: "Error initializing queue",
			})
			return
		}
		
		// Initialize payment totals as well
		InitializePaymentTotal()
		
		log.Println("Initialized payment queue successfully")
		sendDiscordEmbed(
			"✅ System Initialized",
			"Payment queue initialized successfully.",
			ColorGreen,
			nil,
		)
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