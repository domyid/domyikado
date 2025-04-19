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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging
	CrowdfundingDiscordWebhookURL = "https://discord.com/api/webhooks/1348044639818485790/DOsYYebYjrTN48wZVDOPrO4j20X5J3pMAbOdPOUkrJuiXk5niqOjV9ZZ2r06th0jXMhh"
	MicroBitcoinWalletAddress     = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"
	RavencoinWalletAddress        = "RKJpSmjTq5MPDaBx2ubTx1msVB2uZcKA5j"

	// Expiry times for different payment methods
	QRISExpirySeconds         = 300 // 5 minutes
	MicroBitcoinExpirySeconds = 900 // 15 minutes
	RavencoinExpirySeconds    = 900 // 15 minutes
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
func extractUserInfoFromToken(r *http.Request) (phoneNumber, name, npm string, err error) {
	// Get login token from header - gunakan 'login' bukan 'Authorization'
	token := at.GetLoginFromHeader(r)
	if token == "" {
		return "", "", "", errors.New("token not found in header")
	}

	// Decode token menggunakan metode yang sama dengan iqsoal.go
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid token: %v", err)
	}

	// Extract phone number from payload
	phoneNumber = payload.Id
	if phoneNumber == "" {
		return "", "", "", errors.New("phone number not found in token")
	}

	// Debugging - tambahkan log seperti pada iqsoal.go
	fmt.Println("✅ Phonenumber dari Token:", phoneNumber)

	// Cari data user di koleksi `user` berdasarkan `phonenumber`
	userCollection := config.Mongoconn.Collection("user")
	var user model.Userdomyikado
	err = userCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		// User not found, tetapi kita masih punya phoneNumber dan menggunakan alias dari token
		return phoneNumber, payload.Alias, "", nil
	}

	// Jika user ditemukan, gunakan data dari database
	return user.PhoneNumber, user.Name, user.NPM, nil
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
			"success":     true,
			"phoneNumber": phoneNumber,
			"name":        payload.Alias,
			"npm":         "",
		})
		return
	}

	// Jika user ditemukan, berikan data lengkap
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"phoneNumber": user.PhoneNumber,
		"name":        user.Name,
		"npm":         user.NPM,
	})
}

// CheckQueueStatus checks if there's an active payment in the queue
func CheckQueueStatus(w http.ResponseWriter, r *http.Request) {
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

	// Extract user info from token
	phoneNumber, name, npm, err := extractUserInfoFromToken(r)
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
	phoneNumber, name, npm, err := extractUserInfoFromToken(r)
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

	// Log successful order creation
	sendCrowdfundingDiscordEmbed(
		"🛒 New MicroBitcoin Order Created",
		"A new MicroBitcoin payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: name, Inline: true},
			{Name: "Phone", Value: phoneNumber, Inline: true},
			{Name: "NPM", Value: npm, Inline: true},
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
			log.Printf("Error checking Ravencoin address: %v", err)
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
			log.Printf("Found Ravencoin transaction: %s with amount: %f", txid, amount)
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
	// Extract user info from token
	phoneNumber, _, _, err := extractUserInfoFromToken(r)
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
	phoneNumber, name, npm, err := extractUserInfoFromToken(r)
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
		at.WriteJSON(w, http.StatusInternalServerError, model.CrowdfundingPaymentResponse{
			Success: false,
			Message: "Error updating queue",
		})
		return
	}

	// Log successful order creation
	sendCrowdfundingDiscordEmbed(
		"🛒 New Ravencoin Order Created",
		"A new Ravencoin payment order has been created.",
		ColorBlue,
		[]DiscordEmbedField{
			{Name: "Order ID", Value: orderID, Inline: true},
			{Name: "Customer", Value: name, Inline: true},
			{Name: "Phone", Value: phoneNumber, Inline: true},
			{Name: "NPM", Value: npm, Inline: true},
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

	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		OrderID:       orderID,
		ExpiryTime:    expiryTime,
		QRImageURL:    "ravencoin.png",
		WalletAddress: RavencoinWalletAddress,
		PaymentMethod: model.Ravencoin,
	})
}

// Step 1: Check Ravencoin address for unconfirmed transactions
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

	// Check if there are unconfirmed transactions
	if addressResp.UnconfirmedTxs > 0 {
		// Get the latest transaction ID (first in the list)
		if len(addressResp.Txids) > 0 {
			latestTxid := addressResp.Txids[0]

			// Parse unconfirmed balance
			unconfirmedBalance, err := strconv.ParseFloat(addressResp.UnconfirmedBalance, 64)
			if err != nil {
				log.Printf("Error parsing unconfirmed balance: %v", err)
				return true, latestTxid, 0, nil
			}

			// Convert satoshis to RVN (divide by 100,000,000)
			unconfirmedBalance = unconfirmedBalance / 100000000

			// Log the details for debugging
			log.Printf("Found unconfirmed transaction. TxID: %s, Amount: %f RVN", latestTxid, unconfirmedBalance)

			return true, latestTxid, unconfirmedBalance, nil
		}
	}

	// No unconfirmed transactions found
	log.Printf("No unconfirmed transactions found for address: %s", RavencoinWalletAddress)
	return false, "", 0, nil
}

// Step 2: Check if transaction is in the blockchain history
func checkRavencoinTxHistory(txid string) (bool, error) {
	// Log the search attempt
	log.Printf("Checking Ravencoin history for transaction: %s", txid)

	// API URL for checking transaction history
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

	// Check if the txid is in the list of txids
	for _, id := range addressResp.Txids {
		if id == txid {
			log.Printf("Transaction %s found in history", txid)
			return true, nil
		}
	}

	log.Printf("Transaction %s not found in history", txid)
	return false, nil
}

// Step 3: Verify transaction details and get the actual amount
func checkRavencoinTxDetails(txid string) (bool, float64, error) {
	// Log the verification attempt
	log.Printf("Verifying Ravencoin transaction details for: %s", txid)

	// API URL for getting transaction details
	url := "https://blockbook.ravencoin.org/api/v2/tx/" + txid

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
				log.Printf("Found output to our address with amount: %f RVN", amount)
				break
			}
		}
		if amount > 0 {
			break
		}
	}

	// Transaction is valid if we found our address with some value
	if amount > 0 {
		log.Printf("Verified transaction %s with amount: %f RVN", txid, amount)
		return true, amount, nil
	}

	log.Printf("Could not verify transaction %s or amount is 0", txid)
	return false, 0, nil
}

// CheckRavencoinStep2Handler checks transaction history after 7-minute wait
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

	// Log the check
	log.Printf("Checking Ravencoin Step 2 for order %s, txid %s", orderID, txid)

	// Step 2: Check history with the txid from step 1
	historyStatus, err := checkRavencoinTxHistory(txid)
	if err != nil {
		log.Printf("Error in Ravencoin Step 2 check: %v", err)
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
		// Transaction found in history, step 2 complete
		log.Printf("Ravencoin Step 2 complete for order %s", orderID)
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Transaction found in history, proceed to final verification.",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// Transaction not found in history
	log.Printf("Ravencoin Step 2 not complete yet for order %s", orderID)
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "pending",
		Message:       "Transaction not found in history yet. Please wait.",
		Step1Complete: true,
		Step2Complete: false,
		Step3Complete: false,
		TxID:          txid,
		PaymentMethod: model.Ravencoin,
	})
}

// CheckRavencoinStep3Handler finalizes the payment after the step 2 delay
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

	// Log the check
	log.Printf("Checking Ravencoin Step 3 for order %s, txid %s", orderID, txid)

	// Step 3: Verify transaction details and get the actual amount
	txDetails, actualAmount, err := checkRavencoinTxDetails(txid)
	if err != nil {
		log.Printf("Error in Ravencoin Step 3 check: %v", err)
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
		log.Printf("Ravencoin Step 3 verification failed for order %s", orderID)
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
	log.Printf("Ravencoin payment verified for order %s, amount: %f RVN", orderID, actualAmount)
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
		log.Printf("Error updating Ravencoin order status: %v", err)
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
	log.Printf("Ravencoin Step 3 complete for order %s", orderID)
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
