package controller

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging
	CrowdfundingDiscordWebhookURL = "https://discord.com/api/webhooks/1370349053321154581/wxAIb8_Lszb1aOBG4kceE1DtCXbCtnE4RclptWqNvbLor1-hAWjaaVrR0NbJmtsulTaI"
	MicroBitcoinWalletAddress     = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"
	RavencoinWalletAddress        = "RKJpSmjTq5MPDaBx2ubTx1msVB2uZcKA5j"

	// Expiry times for different payment methods
	QRISExpirySeconds         = 3600 // 60 minutes
	MicroBitcoinExpirySeconds = 1500 // 15 minutes
	RavencoinExpirySeconds    = 1500 // 15 minutes
)

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

// Constants for Discord embed colors
const (
	ColorGreen  = 5763719  // Success color (green)
	ColorRed    = 15548997 // Error color (red)
	ColorBlue   = 3447003  // Info color (blue)
	ColorYellow = 16776960 // Warning color (yellow)
	ColorPurple = 10181046 // Special color (purple)
)

// Helper function to send logs to Discord with embeds
func sendCrowdfundingDiscordEmbed(title, description string, color int, fields []DiscordEmbedField) {
	// Set timestamp to current time
	timestamp := time.Now().Format(time.RFC3339)

	// Create embed
	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Footer: &DiscordEmbedFooter{
			Text: "Crowdfunding Payment System",
		},
		Timestamp: timestamp,
	}

	// Create message with embed
	webhookMsg := DiscordWebhookMessage{
		Username:  "Crowdfunding Payment Bot",
		AvatarURL: "https://cdn-icons-png.flaticon.com/512/2168/2168252.png",
		Embeds:    []DiscordEmbed{embed},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(webhookMsg)
	if err != nil {
		log.Printf("Error marshaling Discord embed: %v", err)
		return
	}

	// Send to Discord webhook asynchronously
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(CrowdfundingDiscordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
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

// InitializeCrowdfundingTotal initializes the total payments collection if it doesn't exist
func InitializeCrowdfundingTotal() {
	var total model.CrowdfundingTotal
	err := config.Mongoconn.Collection("crowdfundingtotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Total document doesn't exist, create it
		_, err = config.Mongoconn.Collection("crowdfundingtotals").InsertOne(context.Background(), model.CrowdfundingTotal{
			TotalQRISAmount:      0,
			QRISCount:            0,
			TotalBitcoinAmount:   0,
			BitcoinCount:         0,
			TotalRavencoinAmount: 0, // Initialize Ravencoin totals
			RavencoinCount:       0, // Initialize Ravencoin count
			TotalAmount:          0,
			TotalCount:           0,
			LastUpdated:          time.Now(),
		})
		if err != nil {
			log.Printf("Error initializing crowdfunding totals: %v", err)
			sendCrowdfundingDiscordEmbed(
				"🔴 Error: Crowdfunding Totals Initialization Failed",
				"Failed to initialize crowdfunding totals database.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
		} else {
			log.Println("Initialized crowdfunding totals successfully")
			sendCrowdfundingDiscordEmbed(
				"✅ System: Crowdfunding Totals Initialized",
				"Successfully initialized the crowdfunding totals database.",
				ColorGreen,
				nil,
			)
		}
	}
}

// Initialize Crowdfunding queue
func InitializeCrowdfundingQueue() {
	var queue model.CrowdfundingQueue
	err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Queue document doesn't exist, create it
		_, err = config.Mongoconn.Collection("crowdfundingqueue").InsertOne(context.Background(), model.CrowdfundingQueue{
			IsProcessing:   false,
			CurrentOrderID: "",
			ExpiryTime:     time.Time{},
		})

		if err != nil {
			log.Printf("Error initializing crowdfunding queue: %v", err)
			sendCrowdfundingDiscordEmbed(
				"🔴 Error: Crowdfunding Queue Initialization Failed",
				"Failed to initialize crowdfunding payment queue.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
				},
			)
		} else {
			log.Println("Initialized crowdfunding queue successfully")
			sendCrowdfundingDiscordEmbed(
				"✅ System: Crowdfunding Queue Initialized",
				"Crowdfunding payment queue initialized successfully.",
				ColorGreen,
				nil,
			)
		}
	}
}

// Helper function to update payment totals
func updateCrowdfundingTotal(amount float64, paymentMethod model.PaymentMethod) {
	opts := options.FindOneAndUpdate().SetUpsert(true)

	// Create update based on payment method
	var update bson.M

	if paymentMethod == model.QRIS {
		update = bson.M{
			"$inc": bson.M{
				"totalQRISAmount": amount,
				"qrisCount":       1,
				"totalAmount":     amount,
				"totalCount":      1,
			},
			"$set": bson.M{
				"lastUpdated": time.Now(),
			},
		}
	} else if paymentMethod == model.MicroBitcoin {
		update = bson.M{
			"$inc": bson.M{
				"totalBitcoinAmount": amount,
				"bitcoinCount":       1,
				"totalAmount":        amount,
				"totalCount":         1,
			},
			"$set": bson.M{
				"lastUpdated": time.Now(),
			},
		}
	} else if paymentMethod == model.Ravencoin {
		update = bson.M{
			"$inc": bson.M{
				"totalRavencoinAmount": amount,
				"ravencoinCount":       1,
				"totalAmount":          amount,
				"totalCount":           1,
			},
			"$set": bson.M{
				"lastUpdated": time.Now(),
			},
		}
	}

	var result model.CrowdfundingTotal
	err := config.Mongoconn.Collection("crowdfundingtotals").FindOneAndUpdate(
		context.Background(),
		bson.M{},
		update,
		opts,
	).Decode(&result)

	if err != nil {
		log.Printf("Error updating crowdfunding totals: %v", err)
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Crowdfunding Totals Update Failed",
			"Failed to update crowdfunding totals in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Payment Method", Value: string(paymentMethod), Inline: true},
				{Name: "Amount", Value: formatAmount(amount, paymentMethod), Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
	} else {
		sendCrowdfundingDiscordEmbed(
			"💰 Payment: Total Updated",
			"Successfully updated crowdfunding totals.",
			ColorGreen,
			[]DiscordEmbedField{
				{Name: "Payment Method", Value: string(paymentMethod), Inline: true},
				{Name: "Amount Added", Value: formatAmount(amount, paymentMethod), Inline: true},
			},
		)
	}
}

// Helper function to format amount based on payment method
func formatAmount(amount float64, paymentMethod model.PaymentMethod) string {
	if paymentMethod == model.QRIS {
		return fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64))
	} else if paymentMethod == model.MicroBitcoin {
		return fmt.Sprintf("%f MBC", amount)
	} else if paymentMethod == model.Ravencoin {
		return fmt.Sprintf("%f RVN", amount)
	}
	return strconv.FormatFloat(amount, 'f', 8, 64)
}

// Extract user info from token
func extractUserInfoFromToken(r *http.Request) (phoneNumber, name, npm, wonpaywallet, rvnwallet string, err error) {
	// Get login token from header - gunakan 'login' bukan 'Authorization'
	token := at.GetLoginFromHeader(r)
	if token == "" {
		return "", "", "", "", "", errors.New("token not found in header")
	}

	// Decode token menggunakan metode yang sama dengan iqsoal.go
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("invalid token: %v", err)
	}

	// Extract phone number from payload
	phoneNumber = payload.Id
	if phoneNumber == "" {
		return "", "", "", "", "", errors.New("phone number not found in token")
	}

	// Debugging - tambahkan log seperti pada iqsoal.go
	fmt.Println("✅ Phonenumber dari Token:", phoneNumber)

	// Cari data user di koleksi `user` berdasarkan `phonenumber`
	userCollection := config.Mongoconn.Collection("user")
	var user model.Userdomyikado
	err = userCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		// User not found, tetapi kita masih punya phoneNumber dan menggunakan alias dari token
		return phoneNumber, payload.Alias, "", "", "", nil
	}

	// Jika user ditemukan, gunakan data dari database
	return user.PhoneNumber, user.Name, user.NPM, user.Wonpaywallet, user.RVNwallet, nil
}

// GetUserInfo returns the user information extracted from the authentication token
func GetUserInfo(w http.ResponseWriter, r *http.Request) {
	// Decode token menggunakan `at.GetLoginFromHeader(r)`
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Ambil `phonenumber` dari payload
	phoneNumber := payload.Id
	if phoneNumber == "" {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Status:   "Error: Missing Phonenumber",
			Info:     "Nomor telepon tidak ditemukan dalam token",
			Location: "Token Parsing",
			Response: "Invalid Payload",
		})
		return
	}

	// Debugging
	fmt.Println("✅ Phonenumber dari Token:", phoneNumber)

	// Cari data user di koleksi `user` berdasarkan `phonenumber`
	userCollection := config.Mongoconn.Collection("user")
	var user model.Userdomyikado
	err = userCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		// Jika user tidak ditemukan, tetap beri respons dengan data minimal
		at.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"phoneNumber":  phoneNumber,
			"name":         payload.Alias,
			"npm":          "",
			"wonpaywallet": "",
			"rvnwallet":    "",
		})
		return
	}

	// Jika user ditemukan, berikan data lengkap
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"phoneNumber":  user.PhoneNumber,
		"name":         user.Name,
		"npm":          user.NPM,
		"wonpaywallet": user.Wonpaywallet,
		"rvnwallet":    user.RVNwallet,
	})
}

// CleanupExpiredQueue automatically cleans up any expired payment queue entries
func CleanupExpiredQueue() {
	// Get current queue status
	var queue model.CrowdfundingQueue
	err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)

	if err != nil {
		// If queue document doesn't exist, no cleanup needed
		return
	}

	// Check if there's an active payment that has expired
	if queue.IsProcessing && !queue.ExpiryTime.IsZero() && time.Now().After(queue.ExpiryTime) {
		log.Printf("Found expired payment in queue, order ID: %s, expiry time: %v", queue.CurrentOrderID, queue.ExpiryTime)

		// The payment has expired, reset the queue
		_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
			context.Background(),
			bson.M{},
			bson.M{"$set": bson.M{
				"isProcessing":   false,
				"currentOrderId": "",
				"paymentMethod":  "",
				"expiryTime":     time.Time{},
			}},
		)

		if err != nil {
			log.Printf("Error resetting expired queue: %v", err)
			sendCrowdfundingDiscordEmbed(
				"🔴 Error: Queue Reset Failed",
				"Failed to reset expired queue during cleanup.",
				ColorRed,
				[]DiscordEmbedField{
					{Name: "Error", Value: err.Error(), Inline: false},
					{Name: "Order ID", Value: queue.CurrentOrderID, Inline: true},
					{Name: "Expiry Time", Value: queue.ExpiryTime.Format(time.RFC3339), Inline: true},
				},
			)
		} else {
			log.Println("Successfully reset expired payment queue")
			sendCrowdfundingDiscordEmbed(
				"🕒 Queue: Expired Payment Cleaned Up",
				"Successfully reset expired payment queue during cleanup.",
				ColorYellow,
				[]DiscordEmbedField{
					{Name: "Order ID", Value: queue.CurrentOrderID, Inline: true},
					{Name: "Payment Method", Value: string(queue.PaymentMethod), Inline: true},
					{Name: "Expiry Time", Value: queue.ExpiryTime.Format(time.RFC3339), Inline: true},
				},
			)

			// Also update the order status if it exists
			if queue.CurrentOrderID != "" {
				_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
					context.Background(),
					bson.M{"orderId": queue.CurrentOrderID},
					bson.M{"$set": bson.M{
						"status":    "failed",
						"updatedAt": time.Now(),
					}},
				)

				if err != nil {
					log.Printf("Error updating expired order status: %v", err)
					sendCrowdfundingDiscordEmbed(
						"🔴 Error: Order Status Update Failed",
						"Failed to update expired order status during cleanup.",
						ColorRed,
						[]DiscordEmbedField{
							{Name: "Error", Value: err.Error(), Inline: false},
							{Name: "Order ID", Value: queue.CurrentOrderID, Inline: true},
						},
					)
				} else {
					log.Printf("Successfully updated order status to 'failed' for expired payment: %s", queue.CurrentOrderID)
				}
			}
		}
	}
}

// CheckQueueStatus checks if there's an active payment in the queue
func CheckQueueStatus(w http.ResponseWriter, r *http.Request) {
	// Clean up any expired queues first
	CleanupExpiredQueue()

	var queue model.CrowdfundingQueue
	err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// If no queue document exists, initialize it
		InitializeCrowdfundingQueue()
		InitializeCrowdfundingTotal()

		// Return empty queue status
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:      true,
			IsProcessing: false,
		})
		return
	}

	// Additional check for expired payments
	// This is a double-check in case cleanup didn't work for some reason
	if queue.IsProcessing && !queue.ExpiryTime.IsZero() && time.Now().After(queue.ExpiryTime) {
		log.Printf("Detected expired payment in CheckQueueStatus: %s, expired at %v",
			queue.CurrentOrderID, queue.ExpiryTime)

		// Try once more to reset the queue
		_, updateErr := config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
			context.Background(),
			bson.M{},
			bson.M{"$set": bson.M{
				"isProcessing":   false,
				"currentOrderId": "",
				"paymentMethod":  "",
				"expiryTime":     time.Time{},
			}},
		)

		if updateErr != nil {
			log.Printf("Error in CheckQueueStatus when trying to reset expired queue: %v", updateErr)
		}

		// If payment has expired, return non-processing status regardless of DB update
		// This ensures front-end can proceed even if DB update failed
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:      true,
			IsProcessing: false,
			Message:      "Previous payment session expired",
		})
		return
	}

	// Normal response with current queue status
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		IsProcessing:  queue.IsProcessing,
		ExpiryTime:    queue.ExpiryTime,
		PaymentMethod: queue.PaymentMethod,
	})
}

// CreateQRISOrder creates a new QRIS payment order
func CreateQRISOrder(w http.ResponseWriter, r *http.Request) {
	var request model.CreateQRISOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Request",
			"Failed to process create QRIS order request.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Extract user info from token - updated to handle 6 return values
	phoneNumber, name, npm, _, _, err := extractUserInfoFromToken(r)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Authentication Failed",
			"Failed to extract user information from token.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Authentication failed: " + err.Error(),
		})
		return
	}

	// Validate request
	if request.Amount <= 0 {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Order Parameters",
			"QRIS order creation failed due to invalid amount.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Name", Value: name, Inline: true},
				{Name: "Phone", Value: phoneNumber, Inline: true},
				{Name: "Amount", Value: strconv.FormatFloat(request.Amount, 'f', 2, 64), Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Valid amount is required",
		})
		return
	}

	// Check if someone is currently paying
	var queue model.CrowdfundingQueue
	err = config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Initialize queue if it doesn't exist
		InitializeCrowdfundingQueue()
	} else if queue.IsProcessing {
		sendCrowdfundingDiscordEmbed(
			"⏳ Queue: Payment in Progress",
			"Another payment is already in progress.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Customer", Value: name, Inline: true},
				{Name: "Phone", Value: phoneNumber, Inline: true},
				{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(request.Amount, 'f', 2, 64)), Inline: true},
				{Name: "Status", Value: "Queued", Inline: true},
				{Name: "Payment Method", Value: string(queue.PaymentMethod), Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "Sedang ada pembayaran berlangsung. Silakan tunggu.",
			QueueStatus:   true,
			ExpiryTime:    queue.ExpiryTime,
			PaymentMethod: queue.PaymentMethod,
		})
		return
	}

	// Create order ID
	orderID := uuid.New().String()

	// Set expiry time
	expiryTime := time.Now().Add(QRISExpirySeconds * time.Second)

	// Create new order in database
	newOrder := model.CrowdfundingOrder{
		OrderID:       orderID,
		Name:          name,
		PhoneNumber:   phoneNumber,
		NPM:           npm,
		Amount:        request.Amount,
		PaymentMethod: model.QRIS,
		Timestamp:     time.Now(),
		ExpiryTime:    expiryTime,
		Status:        "pending",
	}

	_, err = config.Mongoconn.Collection("crowdfundingorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Database Error",
			"Failed to create QRIS order in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   true,
			"currentOrderId": orderID,
			"paymentMethod":  model.QRIS,
			"expiryTime":     expiryTime,
		}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Update Failed",
			"Failed to update payment queue.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// Log successful order creation
	sendCrowdfundingDiscordEmbed(
		"🛒 New QRIS Order Created",
		"A new QRIS payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: name, Inline: true},
			{Name: "Phone", Value: phoneNumber, Inline: true},
			{Name: "NPM", Value: npm, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(request.Amount, 'f', 2, 64)), Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)

	// Set up expiry timer
	go func() {
		time.Sleep(QRISExpirySeconds * time.Second)

		// Check if this order is still the current one
		var currentQueue model.CrowdfundingQueue
		err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking queue for timeout: %v", err)
			return
		}

		if currentQueue.CurrentOrderID == orderID {
			// Update order status to failed
			_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status":    "failed",
					"updatedAt": time.Now(),
				}},
			)
			if err != nil {
				log.Printf("Error updating order status: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Status Update Failed",
					"Failed to update expired order status.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			// Reset queue
			_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
				context.Background(),
				bson.M{},
				bson.M{"$set": bson.M{
					"isProcessing":   false,
					"currentOrderId": "",
					"paymentMethod":  "",
					"expiryTime":     time.Time{},
				}},
			)
			if err != nil {
				log.Printf("Error resetting queue: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Queue Reset Failed",
					"Failed to reset queue after order expiry.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			sendCrowdfundingDiscordEmbed(
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

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		OrderID:       orderID,
		ExpiryTime:    expiryTime,
		QRISImageURL:  "qris.png",
		PaymentMethod: model.QRIS,
	})
}

// CreateMicroBitcoinOrder creates a new MicroBitcoin payment order
func CreateMicroBitcoinOrder(w http.ResponseWriter, r *http.Request) {
	// Extract user info from token
	phoneNumber, name, npm, wonpaywallet, _, err := extractUserInfoFromToken(r)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Authentication Failed",
			"Failed to extract user information from token.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Authentication failed: " + err.Error(),
		})
		return
	}

	// Check if someone is currently paying
	var queue model.CrowdfundingQueue
	err = config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Initialize queue if it doesn't exist
		InitializeCrowdfundingQueue()
	} else if queue.IsProcessing {
		sendCrowdfundingDiscordEmbed(
			"⏳ Queue: MicroBitcoin Payment in Progress",
			"Another payment is already in progress.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Customer", Value: name, Inline: true},
				{Name: "Phone", Value: phoneNumber, Inline: true},
				{Name: "Status", Value: "Queued", Inline: true},
				{Name: "Payment Method", Value: string(queue.PaymentMethod), Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "Sedang ada pembayaran berlangsung. Silakan tunggu.",
			QueueStatus:   true,
			ExpiryTime:    queue.ExpiryTime,
			PaymentMethod: queue.PaymentMethod,
		})
		return
	}

	// Create order ID
	orderID := uuid.New().String()

	// Set expiry time
	expiryTime := time.Now().Add(MicroBitcoinExpirySeconds * time.Second)

	// Create new order in database
	newOrder := model.CrowdfundingOrder{
		OrderID:       orderID,
		Name:          name,
		PhoneNumber:   phoneNumber,
		NPM:           npm,
		WalletAddress: MicroBitcoinWalletAddress,
		PaymentMethod: model.MicroBitcoin,
		Timestamp:     time.Now(),
		ExpiryTime:    expiryTime,
		Status:        "pending",
		Wonpaywallet:  wonpaywallet, // Include user's Wonpay wallet
	}

	_, err = config.Mongoconn.Collection("crowdfundingorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Database Error",
			"Failed to create MicroBitcoin order in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   true,
			"currentOrderId": orderID,
			"paymentMethod":  model.MicroBitcoin,
			"expiryTime":     expiryTime,
		}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Update Failed",
			"Failed to update MicroBitcoin payment queue.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// For MicroBitcoin Discord notification:
	var userWalletField DiscordEmbedField
	if wonpaywallet != "" {
		userWalletField = DiscordEmbedField{
			Name:   "User Wallet",
			Value:  wonpaywallet,
			Inline: true,
		}
	} else {
		userWalletField = DiscordEmbedField{
			Name:   "User Wallet",
			Value:  "Not provided",
			Inline: true,
		}
	}

	sendCrowdfundingDiscordEmbed(
		"🛒 New MicroBitcoin Order Created",
		"A new MicroBitcoin payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: name, Inline: true},
			{Name: "Phone", Value: phoneNumber, Inline: true},
			{Name: "NPM", Value: npm, Inline: true},
			userWalletField,
			{Name: "Destination", Value: MicroBitcoinWalletAddress, Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)
	// Set up expiry timer
	go func() {
		time.Sleep(MicroBitcoinExpirySeconds * time.Second)

		// Check if this order is still the current one
		var currentQueue model.CrowdfundingQueue
		err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking MicroBitcoin queue for timeout: %v", err)
			return
		}

		if currentQueue.CurrentOrderID == orderID {
			// Update order status to failed
			_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status":    "failed",
					"updatedAt": time.Now(),
				}},
			)
			if err != nil {
				log.Printf("Error updating MicroBitcoin order status: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Status Update Failed",
					"Failed to update expired MicroBitcoin order status.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			// Reset queue
			_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
				context.Background(),
				bson.M{},
				bson.M{"$set": bson.M{
					"isProcessing":   false,
					"currentOrderId": "",
					"paymentMethod":  "",
					"expiryTime":     time.Time{},
				}},
			)
			if err != nil {
				log.Printf("Error resetting MicroBitcoin queue: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Queue Reset Failed",
					"Failed to reset MicroBitcoin queue after order expiry.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			sendCrowdfundingDiscordEmbed(
				"⏱️ MicroBitcoin Order Expired",
				"A MicroBitcoin payment order has expired.",
				ColorYellow,
				[]DiscordEmbedField{
					{Name: "Order ID", Value: orderID, Inline: true},
					{Name: "Customer", Value: newOrder.Name, Inline: true},
					{Name: "Status", Value: "Expired", Inline: true},
				},
			)
		}
	}()

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		OrderID:       orderID,
		ExpiryTime:    expiryTime,
		QRImageURL:    "wonpay.png",
		WalletAddress: MicroBitcoinWalletAddress,
		PaymentMethod: model.MicroBitcoin,
	})
}

// CheckPayment checks the status of a payment
func CheckPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"❓ Check Payment",
			"Payment status check for non-existent order.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Status", Value: "Not Found", Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// For QRIS payments, just return the status
	if order.PaymentMethod == model.QRIS {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        order.Status,
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// For MicroBitcoin payments, if the payment is pending, check mempool
	if order.PaymentMethod == model.MicroBitcoin && order.Status == "pending" {
		// Check mempool (Step 1)
		mempoolStatus, mempoolTxid, _, err := checkMicroBitcoinMempool()

		if err != nil {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking mempool failed: " + err.Error(),
				Step1Complete: false,
				Step2Complete: false,
				Step3Complete: false,
				PaymentMethod: model.MicroBitcoin,
			})
			return
		}

		if mempoolStatus && mempoolTxid != "" {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Transaction found in mempool, waiting for confirmation.",
				Step1Complete: true,
				Step2Complete: false,
				Step3Complete: false,
				TxID:          mempoolTxid,
				PaymentMethod: model.MicroBitcoin,
			})
			return
		}

		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "No transaction found yet. Please make the payment or wait if you've already sent it.",
			Step1Complete: false,
			Step2Complete: false,
			Step3Complete: false,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	// For Ravencoin payments, if the payment is pending, check address
	if order.PaymentMethod == model.Ravencoin && order.Status == "pending" {
		// Check Ravencoin address for unconfirmed transactions (Step 1)
		addressStatus, txid, amount, err := checkRavencoinAddressAPI()

		if err != nil {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking Ravencoin address failed: " + err.Error(),
				Step1Complete: false,
				Step2Complete: false,
				Step3Complete: false,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		if addressStatus && txid != "" {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Transaction found in unconfirmed transactions, waiting for confirmation.",
				Step1Complete: true,
				Step2Complete: false,
				Step3Complete: false,
				TxID:          txid,
				Amount:        amount,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "No transaction found yet. Please make the payment or wait if you've already sent it.",
			Step1Complete: false,
			Step2Complete: false,
			Step3Complete: false,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// For already processed payments
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        order.Status,
		TxID:          order.TxID,
		Amount:        order.Amount,
		PaymentMethod: order.PaymentMethod,
	})
}

// Step 1: Check mempool for transactions and extract txid properly
func checkMicroBitcoinMempool() (bool, string, float64, error) {
	// API URL for checking mempool
	url := "https://api.mbc.wiki/mempool/" + MicroBitcoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var mempoolResp model.MicroBitcoinMempoolResponse
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
func checkMicroBitcoinTxHistory() (bool, string, error) {
	// API URL for checking transaction history
	url := "https://api.mbc.wiki/history/" + MicroBitcoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	// Parse response
	var historyResp model.MicroBitcoinHistoryResponse
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
func checkMicroBitcoinTxDetails(txid string) (bool, float64, error) {
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
	var txResp model.MicroBitcoinTransactionResponse
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
			if addr == MicroBitcoinWalletAddress {
				amount = vout.Value
				break
			}
		}
		// Alternative check using the single address field
		if vout.ScriptPubKey.Address == MicroBitcoinWalletAddress {
			amount = vout.Value
			break
		}
	}

	// Convert satoshis to MBC
	amountMBC := float64(amount) / 100000000

	// Transaction is valid if we found our address with some value
	return amount > 0, amountMBC, nil
}

// CheckStep2Handler checks transaction history after the 7-minute delay
func CheckStep2Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a MicroBitcoin payment
	if order.PaymentMethod != model.MicroBitcoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        order.Status,
			Message:       "This endpoint is only for MicroBitcoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Step 2: Check history with the txid from step 1
	historyStatus, historyTxid, err := checkMicroBitcoinTxHistory()
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Checking transaction history failed: " + err.Error(),
			Step1Complete: true,
			Step2Complete: false,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	if historyStatus && historyTxid != "" {
		// Transaction found in history, step 2 complete
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction found in history, proceed to final verification.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	// Transaction not found in history
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "pending",
		Message:       "Transaction not found in history yet. Please wait.",
		Step1Complete: true,
		Step2Complete: false,
		Step3Complete: false,
		TxID:          txid,
		PaymentMethod: model.MicroBitcoin,
	})
}

// CheckStep3Handler finalizes the payment after the step 2 delay
func CheckStep3Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a MicroBitcoin payment
	if order.PaymentMethod != model.MicroBitcoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        order.Status,
			Message:       "This endpoint is only for MicroBitcoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Step 3: Verify transaction details and get the actual amount
	txDetails, actualAmount, err := checkMicroBitcoinTxDetails(txid)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Error checking transaction details: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	if !txDetails {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction details verification failed.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	// Now that we have verified the transaction and gotten the actual amount,
	// update the order status to success with the final amount from Step 3
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"txid":      txid,
			"amount":    actualAmount,
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction verified but error updating order status: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			Amount:        actualAmount,
			PaymentMethod: model.MicroBitcoin,
		})
		return
	}

	// Update payment totals with the actual amount from Step 3
	updateCrowdfundingTotal(actualAmount, model.MicroBitcoin)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		log.Printf("Error resetting crowdfunding queue: %v", err)
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	sendCrowdfundingDiscordEmbed(
		"✅ MicroBitcoin Payment Successful",
		"A MicroBitcoin payment has been confirmed automatically.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Transaction ID", Value: txid, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f MBC", actualAmount), Inline: true},
		},
	)

	// Return success response with the actual amount from Step 3
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "success",
		Message:       "Payment confirmed successfully!",
		Step1Complete: true,
		Step2Complete: true,
		Step3Complete: true,
		TxID:          txid,
		Amount:        actualAmount,
		PaymentMethod: model.MicroBitcoin,
	})
}

// ConfirmQRISPayment manually confirms a QRIS payment
func ConfirmQRISPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Manual Confirmation Failed",
			"Failed to confirm QRIS payment manually.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: "Order not found", Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a QRIS payment
	if order.PaymentMethod != model.QRIS {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "This endpoint is only for QRIS payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update QRIS order status during manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateCrowdfundingTotal(order.Amount, model.QRIS)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset queue after manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	sendCrowdfundingDiscordEmbed(
		"✅ Manual QRIS Payment Confirmation",
		"A QRIS payment has been confirmed manually.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(order.Amount, 'f', 2, 64)), Inline: true},
			{Name: "Status", Value: "Confirmed", Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

// ConfirmMicroBitcoinPayment manually confirms a MicroBitcoin payment
func ConfirmMicroBitcoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	// Parse request body to get txid and amount
	var request struct {
		TxID   string  `json:"txid"`
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate the request
	if request.TxID == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	if request.Amount <= 0 {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Amount must be greater than 0",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Manual Confirmation Failed",
			"Failed to confirm MicroBitcoin payment manually.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: "Order not found", Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a MicroBitcoin payment
	if order.PaymentMethod != model.MicroBitcoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "This endpoint is only for MicroBitcoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"txid":      request.TxID,
			"amount":    request.Amount,
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update MicroBitcoin order status during manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateCrowdfundingTotal(request.Amount, model.MicroBitcoin)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{"isProcessing": false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset MicroBitcoin queue after manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	sendCrowdfundingDiscordEmbed(
		"✅ Manual MicroBitcoin Payment Confirmation",
		"A MicroBitcoin payment has been confirmed manually.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Transaction ID", Value: request.TxID, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f MBC", request.Amount), Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

func ProcessQRISNotificationHandler(w http.ResponseWriter, r *http.Request) {
	BasicAuth(ProcessQRISNotification)(w, r)
}

// ProcessQRISNotification processes QRIS payment notifications from payment gateway
func ProcessQRISNotification(w http.ResponseWriter, r *http.Request) {
	var request model.NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Notification",
			"Failed to process QRIS payment notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Log notification for debugging
	log.Printf("Received QRIS notification: %s", request.NotificationText)
	sendCrowdfundingDiscordEmbed(
		"📥 QRIS Notification Received",
		"Received a QRIS payment notification.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Notification Text", Value: request.NotificationText, Inline: false},
		},
	)

	// Check if this is a QRIS payment notification
	if !strings.Contains(request.NotificationText, "Pembayaran QRIS") {
		sendCrowdfundingDiscordEmbed(
			"❌ Notification Rejected",
			"The received notification is not a QRIS payment.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Notification Text", Value: request.NotificationText, Inline: false},
				{Name: "Reason", Value: "Not a QRIS payment notification", Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Not a QRIS payment notification",
		})
		return
	}

	// Extract payment amount with regex - format: "Pembayaran QRIS Rp 1 di Informatika Digital Bisnis, PRNGPNG telah diterima."
	re := regexp.MustCompile(`Pembayaran QRIS Rp\s*(\d+(?:[.,]\d+)?)`)
	matches := re.FindStringSubmatch(request.NotificationText)

	if len(matches) < 2 {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Amount Extraction Failed",
			"Could not extract payment amount from notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Notification Text", Value: request.NotificationText, Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
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
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Amount",
			"The extracted payment amount is invalid.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Extracted Amount", Value: amountStr, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Invalid payment amount",
		})
		return
	}

	// Find order with EXACTLY matching amount, pending status, and QRIS payment method
	ctx := context.Background()
	var order model.CrowdfundingOrder

	// Create filter for EXACT amount, pending status, and QRIS payment method
	filter := bson.M{
		"amount":        amount,
		"status":        "pending",
		"paymentMethod": model.QRIS,
	}

	// Add sort by latest timestamp
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})

	err = config.Mongoconn.Collection("crowdfundingorders").FindOne(ctx, filter, opts).Decode(&order)
	if err != nil {
		// Log the attempted payment even if it doesn't match any order
		sendCrowdfundingDiscordEmbed(
			"❌ QRIS Payment Failed",
			"No pending QRIS order found with the exact amount.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
				{Name: "Status", Value: "Failed to Match", Inline: true},
				{Name: "Notification", Value: request.NotificationText, Inline: false},
			},
		)

		// Store this as a failed payment attempt in database with a generated order ID
		failedPayment := model.CrowdfundingOrder{
			OrderID:       "unmatched-" + uuid.New().String(),
			Name:          "Unknown",
			PhoneNumber:   "Unknown",
			Amount:        amount,
			PaymentMethod: model.QRIS,
			Timestamp:     time.Now(),
			Status:        "failed", // Mark as failed directly
		}

		_, dbErr := config.Mongoconn.Collection("crowdfundingorders").InsertOne(context.Background(), failedPayment)
		if dbErr != nil {
			log.Printf("Error storing failed payment: %v", dbErr)
		}

		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "No pending QRIS order found with amount: " + amountStr,
		})
		return
	}

	// Update order status to success
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		ctx,
		bson.M{"orderId": order.OrderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update order status after notification.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: order.OrderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateCrowdfundingTotal(amount, model.QRIS)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset queue after payment confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	// Log successful confirmation
	log.Printf("QRIS Payment confirmed from notification for amount: Rp%v, Order ID: %s", amount, order.OrderID)
	sendCrowdfundingDiscordEmbed(
		"✅ QRIS Payment Successful",
		"A QRIS payment has been confirmed via notification.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: order.OrderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("Rp %s", strconv.FormatFloat(amount, 'f', 2, 64)), Inline: true},
			{Name: "Status", Value: "Confirmed", Inline: true},
			{Name: "Notification", Value: request.NotificationText, Inline: false},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success: true,
		Message: "Payment confirmed",
		OrderID: order.OrderID,
	})
}

// GetCrowdfundingTotal gets the total payments for all payment methods
func GetCrowdfundingTotal(w http.ResponseWriter, r *http.Request) {
	var total model.CrowdfundingTotal
	err := config.Mongoconn.Collection("crowdfundingtotals").FindOne(context.Background(), bson.M{}).Decode(&total)
	if err != nil {
		// Initialize totals if not found
		InitializeCrowdfundingTotal()

		// Return empty totals
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingTotal{
			TotalQRISAmount:      0,
			QRISCount:            0,
			TotalBitcoinAmount:   0,
			BitcoinCount:         0,
			TotalRavencoinAmount: 0,
			RavencoinCount:       0,
			TotalAmount:          0,
			TotalCount:           0,
			LastUpdated:          time.Now(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, total)
}

// GetUserCrowdfundingHistory gets the payment history for a specific user
func GetUserCrowdfundingHistory(w http.ResponseWriter, r *http.Request) {
	// Extract user info from token - updated to handle 6 return values
	phoneNumber, _, _, _, _, err := extractUserInfoFromToken(r)
	if err != nil {
		at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Authentication failed: " + err.Error(),
		})
		return
	}

	// Find all orders for this user
	filter := bson.M{"phoneNumber": phoneNumber}
	opts := options.Find().SetSort(bson.M{"timestamp": -1}) // Most recent first

	cursor, err := config.Mongoconn.Collection("crowdfundingorders").Find(context.Background(), filter, opts)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error retrieving payment history",
		})
		return
	}
	defer cursor.Close(context.Background())

	// Decode results
	var orders []model.CrowdfundingOrder
	if err := cursor.All(context.Background(), &orders); err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error parsing payment history",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, orders)
}

// AuthCredentials stores basic auth credentials
type AuthCredentials struct {
	Username string    `bson:"username" json:"username"`
	Password string    `bson:"password" json:"password"`
	Created  time.Time `bson:"created" json:"created"`
}

// Basic Auth middleware yang mengambil kredensial dari MongoDB collection merchsecret
func BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Dapatkan header auth
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Authentication required",
			})
			return
		}

		// Parse header auth
		authParts := strings.Split(authHeader, " ")
		if len(authParts) != 2 || authParts[0] != "Basic" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication format",
			})
			return
		}

		// Decode kredensial
		payload, err := base64.StdEncoding.DecodeString(authParts[1])
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication credentials",
			})
			return
		}

		// Split username dan password
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid authentication credentials",
			})
			return
		}

		// Dapatkan kredensial dari database collection merchsecret
		var dbCreds AuthCredentials
		err = config.Mongoconn.Collection("merchsecret").FindOne(context.Background(), bson.M{}).Decode(&dbCreds)
		if err != nil {
			log.Printf("Warning: Could not retrieve auth credentials from database: %v.", err)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Server error retrieving credentials",
			})
			return
		}

		// Verifikasi kredensial dengan data dari database
		if subtle.ConstantTimeCompare([]byte(pair[0]), []byte(dbCreds.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pair[1]), []byte(dbCreds.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
				Success: false,
				Message: "Unauthorized: Invalid username or password",
			})
			return
		}

		// Autentikasi berhasil, panggil handler berikutnya
		next(w, r)
	}
}

// GetCrowdfundingDailyReport sends a daily crowdfunding report to WhatsApp groups
func GetCrowdfundingDailyReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Run the daily crowdfunding report
	err := report.RekapCrowdfundingHarian(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengirim rekap crowdfunding harian",
			Response: err.Error(),
		})
		return
	}

	// Success response
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap crowdfunding harian berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}

// GetCrowdfundingWeeklyReport sends a weekly crowdfunding report to WhatsApp groups
func GetCrowdfundingWeeklyReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Run the weekly crowdfunding report
	err := report.RekapCrowdfundingMingguan(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengirim rekap crowdfunding mingguan",
			Response: err.Error(),
		})
		return
	}

	// Success response
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap crowdfunding mingguan berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}

// GetCrowdfundingTotalReport sends a total crowdfunding report to WhatsApp groups
func GetCrowdfundingTotalReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Run the total crowdfunding report
	err := report.RekapCrowdfundingTotal(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengirim rekap crowdfunding total",
			Response: err.Error(),
		})
		return
	}

	// Success response
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap crowdfunding total berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}

// GetCrowdfundingUserData gets crowdfunding data for a specific user by phone number
func GetCrowdfundingUserData(w http.ResponseWriter, r *http.Request) {
	// Get the phone number from URL parameter
	phoneNumber := r.URL.Query().Get("phonenumber")
	if phoneNumber == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Parameter 'phonenumber' diperlukan",
			Response: "Missing parameter",
		})
		return
	}

	// Get database connection
	var db *mongo.Database = config.Mongoconn

	// Get MBC and QRIS data for the last week
	mbcAmount, err1 := report.GetJumlahMBCLastWeek(db, phoneNumber)
	qrisAmount, err2 := report.GetJumlahQRISLastWeek(db, phoneNumber)

	if err1 != nil || err2 != nil {
		var errMsg string
		if err1 != nil {
			errMsg += "MBC error: " + err1.Error() + ". "
		}
		if err2 != nil {
			errMsg += "QRIS error: " + err2.Error()
		}

		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengambil data crowdfunding",
			Response: errMsg,
		})
		return
	}

	// Success response
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "Success",
		"phoneNumber": phoneNumber,
		"lastWeek": map[string]interface{}{
			"mbcAmount":  mbcAmount,
			"qrisAmount": qrisAmount,
		},
	})
}

// GetLogCrowdfundingDailyReport generates daily crowdfunding report logs for all specified groups without sending to WhatsApp
func GetLogCrowdfundingDailyReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Grup yang sudah ditentukan
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	// Menyimpan hasil laporan untuk semua grup
	var results []map[string]interface{}

	// Generate laporan untuk setiap grup
	for _, groupID := range allowedGroups {
		// Generate laporan harian tanpa mengirim
		msg, _, err := report.GenerateRekapCrowdfundingDaily(db, groupID)

		result := map[string]interface{}{
			"groupID": groupID,
		}

		if err != nil {
			result["status"] = "Error"
			result["message"] = err.Error()
		} else {
			result["status"] = "Success"
			result["report"] = msg
		}

		results = append(results, result)
	}

	// Return semua laporan sebagai respons
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "Success",
		"info":    "Log rekap crowdfunding harian untuk semua grup",
		"reports": results,
	})
}

// GetLogCrowdfundingWeeklyReport generates weekly crowdfunding report logs for all specified groups without sending to WhatsApp
func GetLogCrowdfundingWeeklyReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Grup yang sudah ditentukan
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	// Menyimpan hasil laporan untuk semua grup
	var results []map[string]interface{}

	// Generate laporan untuk setiap grup
	for _, groupID := range allowedGroups {
		// Generate laporan mingguan tanpa mengirim
		msg, _, err := report.GenerateRekapCrowdfundingWeekly(db, groupID)

		result := map[string]interface{}{
			"groupID": groupID,
		}

		if err != nil {
			result["status"] = "Error"
			result["message"] = err.Error()
		} else {
			result["status"] = "Success"
			result["report"] = msg
		}

		results = append(results, result)
	}

	// Return semua laporan sebagai respons
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "Success",
		"info":    "Log rekap crowdfunding mingguan untuk semua grup",
		"reports": results,
	})
}

// GetLogCrowdfundingTotalReport generates total crowdfunding report logs for all specified groups without sending to WhatsApp
func GetLogCrowdfundingTotalReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Grup yang sudah ditentukan
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	// Menyimpan hasil laporan untuk semua grup
	var results []map[string]interface{}

	// Generate laporan untuk setiap grup
	for _, groupID := range allowedGroups {
		// Generate laporan total tanpa mengirim
		msg, _, err := report.GenerateRekapCrowdfundingAll(db, groupID)

		result := map[string]interface{}{
			"groupID": groupID,
		}

		if err != nil {
			result["status"] = "Error"
			result["message"] = err.Error()
		} else {
			result["status"] = "Success"
			result["report"] = msg
		}

		results = append(results, result)
	}

	// Return semua laporan sebagai respons
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "Success",
		"info":    "Log rekap crowdfunding total untuk semua grup",
		"reports": results,
	})
}

// GetCrowdfundingGlobalReport sends a global crowdfunding report (without grouping by WAGroupID)
func GetCrowdfundingGlobalReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Run the global crowdfunding report
	err := report.RekapCrowdfundingGlobal(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengirim rekap crowdfunding global",
			Response: err.Error(),
		})
		return
	}

	// Success response
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap crowdfunding global berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}

// GetLogCrowdfundingGlobalReport generates a global crowdfunding report log without sending to WhatsApp
func GetLogCrowdfundingGlobalReport(w http.ResponseWriter, r *http.Request) {
	// Get the database connection
	var db *mongo.Database = config.Mongoconn

	// Generate the global report without sending
	msg, _, err := report.GenerateRekapCrowdfundingGlobal(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal membuat log rekap crowdfunding global",
			Response: err.Error(),
		})
		return
	}

	// Return the report as a response
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Log rekap crowdfunding global",
		Response: msg,
	})
}

// CreateRavencoinOrder creates a new Ravencoin payment order
func CreateRavencoinOrder(w http.ResponseWriter, r *http.Request) {
	// Extract user info from token
	phoneNumber, name, npm, _, rvnwallet, err := extractUserInfoFromToken(r)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Authentication Failed",
			"Failed to extract user information from token.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusUnauthorized, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Authentication failed: " + err.Error(),
		})
		return
	}

	// Check if someone is currently paying
	var queue model.CrowdfundingQueue
	err = config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&queue)
	if err != nil {
		// Initialize queue if it doesn't exist
		InitializeCrowdfundingQueue()
	} else if queue.IsProcessing {
		sendCrowdfundingDiscordEmbed(
			"⏳ Queue: Ravencoin Payment in Progress",
			"Another payment is already in progress.",
			ColorYellow,
			[]DiscordEmbedField{
				{Name: "Customer", Value: name, Inline: true},
				{Name: "Phone", Value: phoneNumber, Inline: true},
				{Name: "Status", Value: "Queued", Inline: true},
				{Name: "Payment Method", Value: string(queue.PaymentMethod), Inline: true},
			},
		)
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "Sedang ada pembayaran berlangsung. Silakan tunggu.",
			QueueStatus:   true,
			ExpiryTime:    queue.ExpiryTime,
			PaymentMethod: queue.PaymentMethod,
		})
		return
	}

	// Initialize Ravencoin last transactions before creating the order
	// This ensures we have a baseline transaction count for comparison
	initErr := InitializeRavencoinLastTransactions()
	if initErr != nil {
		log.Printf("Warning: Error initializing Ravencoin transaction count: %v", initErr)
		// Continue anyway, as we can still attempt to process the payment
	}

	// Create order ID
	orderID := uuid.New().String()

	// Set expiry time
	expiryTime := time.Now().Add(RavencoinExpirySeconds * time.Second)

	// Create new order in database
	newOrder := model.CrowdfundingOrder{
		OrderID:       orderID,
		Name:          name,
		PhoneNumber:   phoneNumber,
		NPM:           npm,
		WalletAddress: RavencoinWalletAddress,
		PaymentMethod: model.Ravencoin,
		Timestamp:     time.Now(),
		ExpiryTime:    expiryTime,
		Status:        "pending",
		RVNwallet:     rvnwallet, // Include user's Ravencoin wallet
	}

	_, err = config.Mongoconn.Collection("crowdfundingorders").InsertOne(context.Background(), newOrder)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Database Error",
			"Failed to create Ravencoin order in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error creating order",
		})
		return
	}

	// Update queue status
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   true,
			"currentOrderId": orderID,
			"paymentMethod":  model.Ravencoin,
			"expiryTime":     expiryTime,
		}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Update Failed",
			"Failed to update Ravencoin payment queue.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)

		// Try to clean up the created order since queue update failed
		_, cleanupErr := config.Mongoconn.Collection("crowdfundingorders").DeleteOne(
			context.Background(),
			bson.M{"orderId": orderID},
		)
		if cleanupErr != nil {
			log.Printf("Error cleaning up order after queue update failure: %v", cleanupErr)
		}

		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// For Ravencoin Discord notification:
	var userRvnWalletField DiscordEmbedField
	if rvnwallet != "" {
		userRvnWalletField = DiscordEmbedField{
			Name:   "User Wallet",
			Value:  rvnwallet,
			Inline: true,
		}
	} else {
		userRvnWalletField = DiscordEmbedField{
			Name:   "User Wallet",
			Value:  "Not provided",
			Inline: true,
		}
	}

	sendCrowdfundingDiscordEmbed(
		"🛒 New Ravencoin Order Created",
		"A new Ravencoin payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: name, Inline: true},
			{Name: "Phone", Value: phoneNumber, Inline: true},
			{Name: "NPM", Value: npm, Inline: true},
			userRvnWalletField,
			{Name: "Destination", Value: RavencoinWalletAddress, Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)

	// Set up expiry timer
	go func() {
		time.Sleep(RavencoinExpirySeconds * time.Second)

		// Check if this order is still the current one
		var currentQueue model.CrowdfundingQueue
		err := config.Mongoconn.Collection("crowdfundingqueue").FindOne(context.Background(), bson.M{}).Decode(&currentQueue)
		if err != nil {
			log.Printf("Error checking Ravencoin queue for timeout: %v", err)
			return
		}

		if currentQueue.CurrentOrderID == orderID {
			// Update order status to failed
			_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status":    "failed",
					"updatedAt": time.Now(),
				}},
			)
			if err != nil {
				log.Printf("Error updating Ravencoin order status: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Status Update Failed",
					"Failed to update expired Ravencoin order status.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			// Reset queue
			_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
				context.Background(),
				bson.M{},
				bson.M{"$set": bson.M{
					"isProcessing":   false,
					"currentOrderId": "",
					"paymentMethod":  "",
					"expiryTime":     time.Time{},
				}},
			)
			if err != nil {
				log.Printf("Error resetting Ravencoin queue: %v", err)
				sendCrowdfundingDiscordEmbed(
					"🔴 Error: Queue Reset Failed",
					"Failed to reset Ravencoin queue after order expiry.",
					ColorRed,
					[]DiscordEmbedField{
						{Name: "Error", Value: err.Error(), Inline: false},
					},
				)
			}

			sendCrowdfundingDiscordEmbed(
				"⏱️ Ravencoin Order Expired",
				"A Ravencoin payment order has expired.",
				ColorYellow,
				[]DiscordEmbedField{
					{Name: "Order ID", Value: orderID, Inline: true},
					{Name: "Customer", Value: newOrder.Name, Inline: true},
					{Name: "Status", Value: "Expired", Inline: true},
				},
			)
		}
	}()

	// Return success response with order details
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		OrderID:       orderID,
		ExpiryTime:    expiryTime,
		QRImageURL:    "ravencoin.png",
		WalletAddress: RavencoinWalletAddress,
		PaymentMethod: model.Ravencoin,
	})
}

// Step 1: Check Ravencoin address for new transactions
func checkRavencoinAddressAPI() (bool, string, float64, error) {
	// API URL for checking Ravencoin address
	url := "https://blockbook.ravencoin.org/api/v2/address/" + RavencoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var addressResp model.RavencoinAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addressResp); err != nil {
		return false, "", 0, err
	}

	// Check current transaction count against last saved transaction count
	var lastTxData model.RavencoinLastTransactions
	err = config.Mongoconn.Collection("crowdfundinglastraventxs").FindOne(context.Background(), bson.M{}).Decode(&lastTxData)

	// If no record found, initialize with current txs count
	if err == mongo.ErrNoDocuments {
		_, err = config.Mongoconn.Collection("crowdfundinglastraventxs").InsertOne(
			context.Background(),
			bson.M{
				"lastTxCount": addressResp.Txs,
				"lastUpdated": time.Now(),
			},
		)
		if err != nil {
			log.Printf("Error initializing Ravencoin last transactions: %v", err)
		}
		return false, "", 0, nil
	} else if err != nil {
		return false, "", 0, err
	}

	// No new transactions
	if addressResp.Txs <= lastTxData.LastTxCount {
		return false, "", 0, nil
	}

	// New transaction(s) found! Get the latest txid (first in the list)
	if len(addressResp.Txids) > 0 {
		latestTxid := addressResp.Txids[0]

		// We'll get the actual amount in Step 3, for now just indicate we found a transaction
		return true, latestTxid, 0, nil
	}

	return false, "", 0, nil
}

// Step 2: Check if transaction count has changed in the blockchain
func checkRavencoinTxHistory(txid string) (bool, error) {
	// API URL for checking Ravencoin address
	url := "https://blockbook.ravencoin.org/api/v2/address/" + RavencoinWalletAddress

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Parse response
	var addressResp model.RavencoinAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addressResp); err != nil {
		return false, err
	}

	// Get last transaction count
	var lastTxData model.RavencoinLastTransactions
	err = config.Mongoconn.Collection("crowdfundinglastraventxs").FindOne(context.Background(), bson.M{}).Decode(&lastTxData)
	if err != nil {
		return false, err
	}

	// If transaction count has increased, update the record and return success
	if addressResp.Txs > lastTxData.LastTxCount {
		// Update the last transaction count
		_, err = config.Mongoconn.Collection("crowdfundinglastraventxs").UpdateOne(
			context.Background(),
			bson.M{},
			bson.M{"$set": bson.M{
				"lastTxCount": addressResp.Txs,
				"lastUpdated": time.Now(),
			}},
		)
		if err != nil {
			log.Printf("Error updating Ravencoin last transactions: %v", err)
		}

		return true, nil
	}

	// No change in transaction count
	return false, nil
}

// Step 3: Verify transaction details and get the actual amount
func checkRavencoinTxDetails(txid string) (bool, float64, error) {
	// API URL for getting transaction details
	url := "https://blockbook.ravencoin.org/api/tx/" + txid

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var txResp model.RavencoinTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return false, 0, err
	}

	// Find the output that matches our wallet address
	var amount float64 = 0
	for _, vout := range txResp.Vout {
		// Check if this output is to our wallet address
		for _, addr := range vout.ScriptPubKey.Addresses {
			if addr == RavencoinWalletAddress {
				// Convert from string to float
				outputAmount, err := strconv.ParseFloat(vout.Value, 64)
				if err != nil {
					continue
				}
				amount = outputAmount
				break
			}
		}
		if amount > 0 {
			break
		}
	}

	// Transaction is valid if we found our address with some value
	return amount > 0, amount, nil
}

// CheckRavencoinStep2Handler checks transaction history without the 7-minute delay
func CheckRavencoinStep2Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a Ravencoin payment
	if order.PaymentMethod != model.Ravencoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        order.Status,
			Message:       "This endpoint is only for Ravencoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Step 2: Check if transaction count has changed (no more timer)
	historyStatus, err := checkRavencoinTxHistory(txid)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Checking transaction history failed: " + err.Error(),
			Step1Complete: true,
			Step2Complete: false,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	if historyStatus {
		// Transaction count has changed, step 2 complete - proceed directly to step 3
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction confirmed in blockchain, proceeding to verification.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// Transaction count hasn't changed yet, continue polling
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "pending",
		Message:       "Waiting for transaction confirmation. Please keep checking.",
		Step1Complete: true,
		Step2Complete: false,
		Step3Complete: false,
		TxID:          txid,
		PaymentMethod: model.Ravencoin,
	})
}

// CheckRavencoinStep3Handler finalizes the payment without waiting
func CheckRavencoinStep3Handler(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)
	txid := r.URL.Query().Get("txid")

	if txid == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a Ravencoin payment
	if order.PaymentMethod != model.Ravencoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        order.Status,
			Message:       "This endpoint is only for Ravencoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Step 3: Verify transaction details and get the actual amount (no delay)
	txDetails, actualAmount, err := checkRavencoinTxDetails(txid)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Error checking transaction details: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	if !txDetails {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction details verification failed.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// Now that we have verified the transaction and gotten the actual amount,
	// update the order status to success with the final amount from Step 3
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"txid":      txid,
			"amount":    actualAmount,
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction verified but error updating order status: " + err.Error(),
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			Amount:        actualAmount,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// Update payment totals with the actual amount from Step 3
	updateCrowdfundingTotal(actualAmount, model.Ravencoin)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		log.Printf("Error resetting crowdfunding queue: %v", err)
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	sendCrowdfundingDiscordEmbed(
		"✅ Ravencoin Payment Successful",
		"A Ravencoin payment has been confirmed automatically.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Transaction ID", Value: txid, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f RVN", actualAmount), Inline: true},
		},
	)

	// Return success response with the actual amount from Step 3
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "success",
		Message:       "Payment confirmed successfully!",
		Step1Complete: true,
		Step2Complete: true,
		Step3Complete: true,
		TxID:          txid,
		Amount:        actualAmount,
		PaymentMethod: model.Ravencoin,
	})
}

// ConfirmRavencoinPayment manually confirms a Ravencoin payment
func ConfirmRavencoinPayment(w http.ResponseWriter, r *http.Request) {
	orderID := at.GetParam(r)

	// Parse request body to get txid and amount
	var request struct {
		TxID   string  `json:"txid"`
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate the request
	if request.TxID == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Transaction ID is required",
		})
		return
	}

	if request.Amount <= 0 {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Amount must be greater than 0",
		})
		return
	}

	// Find the order
	var order model.CrowdfundingOrder
	err := config.Mongoconn.Collection("crowdfundingorders").FindOne(context.Background(), bson.M{"orderId": orderID}).Decode(&order)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Manual Confirmation Failed",
			"Failed to confirm Ravencoin payment manually.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: "Order not found", Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusNotFound, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Order not found",
		})
		return
	}

	// Verify this is a Ravencoin payment
	if order.PaymentMethod != model.Ravencoin {
		at.WriteJSON(w, http.StatusBadRequest, model.CrowdfundingPaymentResponse{
			Success:       false,
			Message:       "This endpoint is only for Ravencoin payments",
			PaymentMethod: order.PaymentMethod,
		})
		return
	}

	// Update order status
	_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
		context.Background(),
		bson.M{"orderId": orderID},
		bson.M{"$set": bson.M{
			"status":    "success",
			"txid":      request.TxID,
			"amount":    request.Amount,
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Status Update Failed",
			"Failed to update Ravencoin order status during manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Order ID", Value: orderID, Inline: true},
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating order status",
		})
		return
	}

	// Update payment totals
	updateCrowdfundingTotal(request.Amount, model.Ravencoin)

	// Reset queue
	_, err = config.Mongoconn.Collection("crowdfundingqueue").UpdateOne(
		context.Background(),
		bson.M{},
		bson.M{"$set": bson.M{
			"isProcessing":   false,
			"currentOrderId": "",
			"paymentMethod":  "",
			"expiryTime":     time.Time{},
		}},
	)
	if err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Queue Reset Failed",
			"Failed to reset Ravencoin queue after manual confirmation.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error resetting queue",
		})
		return
	}

	// Calculate payment points after successful payment
	RecalculatePointsAfterPayment()

	sendCrowdfundingDiscordEmbed(
		"✅ Manual Ravencoin Payment Confirmation",
		"A Ravencoin payment has been confirmed manually.",
		ColorGreen,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: order.Name, Inline: true},
			{Name: "Phone", Value: order.PhoneNumber, Inline: true},
			{Name: "NPM", Value: order.NPM, Inline: true},
			{Name: "Transaction ID", Value: request.TxID, Inline: true},
			{Name: "Amount", Value: fmt.Sprintf("%f RVN", request.Amount), Inline: true},
		},
	)

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success: true,
		Message: "Payment confirmed",
	})
}

// InitializeRavencoinLastTransactions initializes the last Ravencoin transactions collection if it doesn't exist
func InitializeRavencoinLastTransactions() error {
	// Always fetch the current transaction count from API for consistency
	url := "https://blockbook.ravencoin.org/api/v2/address/" + RavencoinWalletAddress
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Error fetching Ravencoin transaction count: %v", err)
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Ravencoin API Error",
			"Failed to fetch transaction count from Ravencoin API.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		return err
	}
	defer resp.Body.Close()

	// Parse response
	var addressResp model.RavencoinAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addressResp); err != nil {
		log.Printf("Error parsing Ravencoin address response: %v", err)
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Ravencoin API Response Error",
			"Failed to parse response from Ravencoin API.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		return err
	}

	// Check if we have an existing record
	var existingRecord model.RavencoinLastTransactions
	findErr := config.Mongoconn.Collection("crowdfundinglastraventxs").FindOne(
		context.Background(),
		bson.M{},
	).Decode(&existingRecord)

	// Log the current transaction count and existing record for debugging
	if findErr == nil {
		log.Printf("Current Ravencoin tx count from API: %d, Existing record count: %d",
			addressResp.Txs, existingRecord.LastTxCount)
	} else {
		log.Printf("Current Ravencoin tx count from API: %d, No existing record found",
			addressResp.Txs)
	}

	// Update with current transaction count using upsert
	updateResult, err := config.Mongoconn.Collection("crowdfundinglastraventxs").UpdateOne(
		context.Background(),
		bson.M{}, // Empty filter to match any document (there should be only one)
		bson.M{"$set": bson.M{
			"lastTxCount": addressResp.Txs,
			"lastUpdated": time.Now(),
		}},
		options.Update().SetUpsert(true), // Create if it doesn't exist
	)

	if err != nil {
		log.Printf("Error updating Ravencoin last transactions: %v", err)
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Database Update Failed",
			"Failed to update Ravencoin transaction count in database.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Error", Value: err.Error(), Inline: false},
			},
		)
		return err
	}

	// Log the update result
	if updateResult.UpsertedCount > 0 {
		log.Printf("Created new Ravencoin transaction count record with count: %d", addressResp.Txs)
		sendCrowdfundingDiscordEmbed(
			"✅ System: Ravencoin Transactions Initialized",
			"Successfully initialized the Ravencoin transactions tracking.",
			ColorGreen,
			[]DiscordEmbedField{
				{Name: "Initial Transaction Count", Value: fmt.Sprintf("%d", addressResp.Txs), Inline: false},
			},
		)
	} else if updateResult.ModifiedCount > 0 {
		log.Printf("Updated existing Ravencoin transaction count to: %d", addressResp.Txs)
		sendCrowdfundingDiscordEmbed(
			"✅ System: Ravencoin Transactions Updated",
			"Successfully updated the Ravencoin transactions count.",
			ColorGreen,
			[]DiscordEmbedField{
				{Name: "Transaction Count", Value: fmt.Sprintf("%d", addressResp.Txs), Inline: false},
			},
		)
	} else {
		log.Printf("Ravencoin transaction count already up to date: %d", addressResp.Txs)
	}

	return nil
}

// RecalculatePointsAfterPayment recalculates all user payment points
func RecalculatePointsAfterPayment() {
	// Run the calculation in a goroutine to not block the current request
	go func() {
		err := CalculatePaymentPoints()
		if err != nil {
			log.Printf("Error recalculating payment points after new payment: %v", err)
		} else {
			log.Println("Successfully recalculated payment points after new payment")
		}
	}()
}

// CalculatePaymentPoints recalculates all payment points and updates the database
func CalculatePaymentPoints() error {
	log.Println("Starting payment points calculation...")

	ctx := context.Background()
	db := config.Mongoconn

	// Get all successful payments
	filter := bson.M{"status": "success"}
	cursor, err := db.Collection("crowdfundingorders").Find(ctx, filter)
	if err != nil {
		log.Printf("Error fetching crowdfunding orders: %v", err)
		return err
	}
	defer cursor.Close(ctx)

	// Group payments by user and payment method
	userPaymentsMap := make(map[string]*struct {
		Name            string
		PhoneNumber     string
		QRISAmount      float64
		QRISCount       int
		MBCAmount       float64
		MBCCount        int
		RavencoinAmount float64
		RavencoinCount  int
		TotalCount      int
	})

	var allPayments []model.CrowdfundingOrder

	// Process each payment
	for cursor.Next(ctx) {
		var payment model.CrowdfundingOrder
		if err := cursor.Decode(&payment); err != nil {
			log.Printf("Error decoding payment: %v", err)
			continue
		}

		allPayments = append(allPayments, payment)

		// Get or create user data
		userData, exists := userPaymentsMap[payment.PhoneNumber]
		if !exists {
			userData = &struct {
				Name            string
				PhoneNumber     string
				QRISAmount      float64
				QRISCount       int
				MBCAmount       float64
				MBCCount        int
				RavencoinAmount float64
				RavencoinCount  int
				TotalCount      int
			}{
				Name:        payment.Name,
				PhoneNumber: payment.PhoneNumber,
			}
			userPaymentsMap[payment.PhoneNumber] = userData
		}

		// Update user data based on payment method
		switch payment.PaymentMethod {
		case model.QRIS:
			userData.QRISAmount += payment.Amount
			userData.QRISCount++
		case model.MicroBitcoin:
			userData.MBCAmount += payment.Amount
			userData.MBCCount++
		case model.Ravencoin:
			userData.RavencoinAmount += payment.Amount
			userData.RavencoinCount++
		}

		// Update total
		userData.TotalCount++
	}

	if cursor.Err() != nil {
		log.Printf("Cursor error: %v", cursor.Err())
		return cursor.Err()
	}

	// Calculate averages for each payment method
	var qrisTotal, mbcTotal, ravencoinTotal float64
	var qrisCount, mbcCount, ravencoinCount int

	for _, payment := range allPayments {
		switch payment.PaymentMethod {
		case model.QRIS:
			qrisTotal += payment.Amount
			qrisCount++
		case model.MicroBitcoin:
			mbcTotal += payment.Amount
			mbcCount++
		case model.Ravencoin:
			ravencoinTotal += payment.Amount
			ravencoinCount++
		}
	}

	// Calculate averages
	qrisAvg := 0.0
	if qrisCount > 0 {
		qrisAvg = qrisTotal / float64(qrisCount)
	}

	mbcAvg := 0.0
	if mbcCount > 0 {
		mbcAvg = mbcTotal / float64(mbcCount)
	}

	ravencoinAvg := 0.0
	if ravencoinCount > 0 {
		ravencoinAvg = ravencoinTotal / float64(ravencoinCount)
	}

	log.Printf("Averages - QRIS: %f, MBC: %f, Ravencoin: %f", qrisAvg, mbcAvg, ravencoinAvg)

	// Calculate points for each user and update the database
	pointsCollection := db.Collection("crowdfundingpoints")

	// For each user, update or insert their points
	for phoneNumber, userData := range userPaymentsMap {
		// Calculate points for each payment method
		qrisPoints := 0.0
		if userData.QRISCount > 0 && qrisAvg > 0 {
			qrisPoints = (userData.QRISAmount / qrisAvg) * 100
		}

		mbcPoints := 0.0
		if userData.MBCCount > 0 && mbcAvg > 0 {
			mbcPoints = (userData.MBCAmount / mbcAvg) * 100
		}

		ravencoinPoints := 0.0
		if userData.RavencoinCount > 0 && ravencoinAvg > 0 {
			ravencoinPoints = (userData.RavencoinAmount / ravencoinAvg) * 100
		}

		// Calculate total points
		totalPoints := qrisPoints + mbcPoints + ravencoinPoints

		// Prepare data for update/insert
		pointsData := bson.M{
			"phoneNumber":     phoneNumber,
			"name":            userData.Name,
			"qrisPoints":      qrisPoints,
			"qrisAmount":      userData.QRISAmount,
			"qrisCount":       userData.QRISCount,
			"mbcPoints":       mbcPoints,
			"mbcAmount":       userData.MBCAmount,
			"mbcCount":        userData.MBCCount,
			"ravencoinPoints": ravencoinPoints,
			"ravencoinAmount": userData.RavencoinAmount,
			"ravencoinCount":  userData.RavencoinCount,
			"totalPoints":     totalPoints,
			"totalCount":      userData.TotalCount,
			"lastUpdated":     time.Now(),
		}

		// Update or insert
		filter := bson.M{"phoneNumber": phoneNumber}
		update := bson.M{"$set": pointsData}
		opts := options.Update().SetUpsert(true)

		_, err := pointsCollection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Printf("Error updating points for user %s: %v", phoneNumber, err)
			continue
		}
	}

	log.Printf("Successfully updated payment points for %d users", len(userPaymentsMap))
	return nil
}

// point semuanya
// GetAllDataMicroBitcoinScore retrieves the MicroBitcoin (MBC) payments and calculates score
func GetAllDataMicroBitcoinScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Query successful MicroBitcoin payments for this user
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.MicroBitcoin,
		"status":        "success",
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalMBC float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total MBC amount
	for _, payment := range payments {
		totalMBC += float32(payment.Amount)
	}

	// Calculate blockchain score based on total MBC
	blockchainScore := calculateBlockchainScore(db, totalMBC, model.MicroBitcoin)

	// Calculate MBC points - we'll use a similar approach as in PaymentPointsData
	mbcPoints := calculateMBCPoints(db, totalMBC)

	// Set the activity score values
	activityScore.MBC = totalMBC
	activityScore.MBCPoints = mbcPoints
	activityScore.BlockChain = blockchainScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// GetLastWeekDataMicroBitcoinScore gets MBC data for the last week only
func GetLastWeekDataMicroBitcoinScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful MicroBitcoin payments for this user from the last week
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.MicroBitcoin,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalMBC float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total MBC amount
	for _, payment := range payments {
		totalMBC += float32(payment.Amount)
	}

	// Calculate blockchain score based on total MBC
	blockchainScore := calculateBlockchainScore(db, totalMBC, model.MicroBitcoin)

	// Calculate MBC points
	mbcPoints := calculateMBCPoints(db, totalMBC)

	// Set the activity score values
	activityScore.MBC = totalMBC
	activityScore.MBCPoints = mbcPoints
	activityScore.BlockChain = blockchainScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// GetAllDataRavencoinScore retrieves the Ravencoin (RVN) payments and calculates score
func GetAllDataRavencoinScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Query successful Ravencoin payments for this user
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.Ravencoin,
		"status":        "success",
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRVN float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total RVN amount
	for _, payment := range payments {
		totalRVN += float32(payment.Amount)
	}

	// Calculate Ravencoin points
	ravencoinPoints := calculateRavencoinPoints(db, totalRVN)

	// Calculate Ravencoin score using the new function
	ravencoinScore := calculateRavencoinScore(db, totalRVN)

	// Set the activity score values
	activityScore.RVN = totalRVN
	activityScore.RavencoinPoints = ravencoinPoints
	// Add the Ravencoin score to the Blockchain field
	activityScore.BlockChain = ravencoinScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// GetLastWeekDataRavencoinScore gets RVN data for the last week only
func GetLastWeekDataRavencoinScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful Ravencoin payments for this user from the last week
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.Ravencoin,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRVN float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total RVN amount
	for _, payment := range payments {
		totalRVN += float32(payment.Amount)
	}

	// Calculate Ravencoin points
	ravencoinPoints := calculateRavencoinPoints(db, totalRVN)

	// Calculate Ravencoin score using the new function
	ravencoinScore := calculateRavencoinScore(db, totalRVN)

	// Set the activity score values
	activityScore.RVN = totalRVN
	activityScore.RavencoinPoints = ravencoinPoints
	// Add the Ravencoin score to the Blockchain field
	activityScore.BlockChain = ravencoinScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// GetAllDataQRISScore retrieves the QRIS payments and calculates score
func GetAllDataQRISScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Query successful QRIS payments for this user
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.QRIS,
		"status":        "success",
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRupiah int = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total QRIS amount
	for _, payment := range payments {
		totalRupiah += int(payment.Amount)
	}

	// Calculate QRIS score based on total amount
	qrisScore := calculateQRISScore(db, totalRupiah)

	// Calculate QRIS points
	qrisPoints := calculateQRISPoints(db, totalRupiah)

	// Set the activity score values
	activityScore.Rupiah = totalRupiah
	activityScore.QRISPoints = qrisPoints
	activityScore.QRIS = qrisScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// GetLastWeekDataQRISScore gets QRIS data for the last week only
func GetLastWeekDataQRISScore(db *mongo.Database, phoneNumber string) (model.ActivityScore, error) {
	var activityScore model.ActivityScore

	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful QRIS payments for this user from the last week
	filter := bson.M{
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.QRIS,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRupiah int = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return activityScore, err
	}

	// Sum up the total QRIS amount
	for _, payment := range payments {
		totalRupiah += int(payment.Amount)
	}

	// Calculate QRIS score based on total amount
	qrisScore := calculateQRISScore(db, totalRupiah)

	// Calculate QRIS points
	qrisPoints := calculateQRISPoints(db, totalRupiah)

	// Set the activity score values
	activityScore.Rupiah = totalRupiah
	activityScore.QRISPoints = qrisPoints
	activityScore.QRIS = qrisScore
	activityScore.PhoneNumber = phoneNumber
	activityScore.CreatedAt = time.Now()

	return activityScore, nil
}

// Helper function to calculate MBC points
func calculateMBCPoints(db *mongo.Database, amount float32) float64 {
	// Get average MBC amount
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": model.MicroBitcoin,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		avgAmount = 0.001 // Default average for MBC
	}

	// Calculate points: (user's amount / average amount) * 100
	points := (float64(amount) / avgAmount) * 100
	// Cap points at 100
	if points > 100 {
		points = 100
	}

	return points
}

// Helper function to calculate Ravencoin points
func calculateRavencoinPoints(db *mongo.Database, amount float32) float64 {
	// Get average Ravencoin amount
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": model.Ravencoin,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		avgAmount = 1 // Default average for Ravencoin
	}

	// Calculate points: (user's amount / average amount) * 100
	points := (float64(amount) / avgAmount) * 100

	if points > 100 {
		points = 100
	}

	return points
}

// Helper function to calculate QRIS points
func calculateQRISPoints(db *mongo.Database, amount int) float64 {
	// Get average QRIS payment amount
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": model.QRIS,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		avgAmount = 10000 // Default average QRIS amount (IDR)
	}

	// Calculate points: (user's amount / average amount) * 100
	points := (float64(amount) / avgAmount) * 100

	// Cap points at 100
	if points > 100 {
		points = 100
	}

	return points
}

// Helper function to calculate QRIS score based on payment amount
func calculateQRISScore(db *mongo.Database, amount int) int {
	// Get average QRIS payment amount
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": model.QRIS,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		avgAmount = 10000 // Default average QRIS amount (IDR)
	}

	// Calculate score: (user's amount / average amount) * 100
	score := int((float64(amount) / avgAmount) * 100)

	// if score > 100 {
	//     score = 100
	// }

	return score
}

// Helper function to calculate Ravencoin score based on payment amount
func calculateRavencoinScore(db *mongo.Database, amount float32) int {
	// Get average Ravencoin payment amount
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": model.Ravencoin,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		avgAmount = 1 // Default average for Ravencoin
	}

	// Calculate score: (user's amount / average amount) * 100
	score := int((float64(amount) / avgAmount) * 100)

	// No capping at 100 for score calculation
	return score
}

// Helper function to calculate blockchain score based on payment amount
func calculateBlockchainScore(db *mongo.Database, amount float32, paymentMethod model.PaymentMethod) int {
	// Get average payment amount for this payment method
	var avgAmount float64

	// Aggregate to find average - using properly keyed fields
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"paymentMethod": paymentMethod,
			"status":        "success",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       nil,
			"avgAmount": bson.M{"$avg": "$amount"},
		}}},
	}

	cursor, err := db.Collection("crowdfundingorders").Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(context.Background())

	// Extract result
	var result struct {
		AvgAmount float64 `bson:"avgAmount"`
	}

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0
		}
		avgAmount = result.AvgAmount
	}

	// If no average found or it's zero, use a default value
	if avgAmount <= 0 {
		if paymentMethod == model.MicroBitcoin {
			avgAmount = 0.001 // Default average for MBC
		} else {
			avgAmount = 1 // Default average for other methods
		}
	}

	// Calculate score: (user's amount / average amount) * 100, max 100
	score := int((float64(amount) / avgAmount) * 100)

	// // Cap score at 100
	// if score > 100 {
	// 	score = 100
	// }

	return score
}

// GetLastWeekDataMicroBitcoinScore gets MBC data for the last week only
func GetLastWeekDataMicroBitcoinScoreKelas(db *mongo.Database, phoneNumber string, usedIDs []primitive.ObjectID) (resultid []primitive.ObjectID, activityScore model.ActivityScore, err error) {
	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful MicroBitcoin payments for this user from the last week
	filter := bson.M{
		"_id":           bson.M{"$nin": usedIDs},
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.MicroBitcoin,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return nil, activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalMBC float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return nil, activityScore, err
	}

	// Sum up the total MBC amount
	for _, payment := range payments {
		resultid = append(resultid, payment.ID)
		totalMBC += float32(payment.Amount)
	}

	// Calculate blockchain score based on total MBC
	blockchainScore := calculateBlockchainScore(db, totalMBC, model.MicroBitcoin)

	// Calculate MBC points
	mbcPoints := calculateMBCPoints(db, totalMBC)

	// Set the activity score values
	activityScore.MBC = totalMBC
	activityScore.MBCPoints = mbcPoints
	activityScore.BlockChain = blockchainScore

	return resultid, activityScore, nil
}

// GetLastWeekDataRavencoinScoreKelasAI gets RVN data for the last week only for KelasAI
func GetLastWeekDataRavencoinScoreKelas(db *mongo.Database, phoneNumber string, usedIDs []primitive.ObjectID) (resultid []primitive.ObjectID, activityScore model.ActivityScore, err error) {
	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful Ravencoin payments for this user from the last week
	filter := bson.M{
		"_id":           bson.M{"$nin": usedIDs},
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.Ravencoin,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return nil, activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRVN float32 = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return nil, activityScore, err
	}

	// Sum up the total RVN amount and collect IDs
	for _, payment := range payments {
		resultid = append(resultid, payment.ID)
		totalRVN += float32(payment.Amount)
	}

	// Calculate Ravencoin points
	ravencoinPoints := calculateRavencoinPoints(db, totalRVN)

	// Calculate Ravencoin score using the new function
	ravencoinScore := calculateRavencoinScore(db, totalRVN)

	// Set the activity score values
	activityScore.RVN = totalRVN
	activityScore.RavencoinPoints = ravencoinPoints
	// Add the Ravencoin score to the Blockchain field
	activityScore.BlockChain = ravencoinScore

	return resultid, activityScore, nil
}

// GetLastWeekDataQRISScore gets QRIS data for the last week only
func GetLastWeekDataQRISScoreKelas(db *mongo.Database, phoneNumber string, usedIDs []primitive.ObjectID) (resultid []primitive.ObjectID, activityScore model.ActivityScore, err error) {
	// Calculate the date one week ago
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Query successful QRIS payments for this user from the last week
	filter := bson.M{
		"_id":           bson.M{"$nin": usedIDs},
		"phoneNumber":   phoneNumber,
		"paymentMethod": model.QRIS,
		"status":        "success",
		"timestamp": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	cursor, err := db.Collection("crowdfundingorders").Find(context.Background(), filter)
	if err != nil {
		return nil, activityScore, err
	}
	defer cursor.Close(context.Background())

	// Process payments
	var totalRupiah int = 0
	var payments []model.CrowdfundingOrder
	if err = cursor.All(context.Background(), &payments); err != nil {
		return nil, activityScore, err
	}

	// Sum up the total QRIS amount
	for _, payment := range payments {
		resultid = append(resultid, payment.ID)
		totalRupiah += int(payment.Amount)
	}

	// Calculate QRIS score based on total amount
	qrisScore := calculateQRISScore(db, totalRupiah)

	// Calculate QRIS points
	qrisPoints := calculateQRISPoints(db, totalRupiah)

	// Set the activity score values
	activityScore.Rupiah = totalRupiah
	activityScore.QRISPoints = qrisPoints
	activityScore.QRIS = qrisScore

	return resultid, activityScore, nil
}
