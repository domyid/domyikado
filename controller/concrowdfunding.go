package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging
	CrowdfundingDiscordWebhookURL = "https://discord.com/api/webhooks/1348044639818485790/DOsYYebYjrTN48wZVDOPrO4j20X5J3pMAbOdPOUkrJuiXk5niqOjV9ZZ2r06th0jXMhh"
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
			TotalRavencoinAmount: 0, // Add for Ravencoin
			RavencoinCount:       0, // Add for Ravencoin
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
	} else {
		// Default format
		return strconv.FormatFloat(amount, 'f', 2, 64)
	}
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
	expiryTime := time.Now().Add(model.QRISExpirySeconds * time.Second)

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
		time.Sleep(model.QRISExpirySeconds * time.Second)

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
	expiryTime := time.Now().Add(model.MicroBitcoinExpirySeconds * time.Second)

	// Create new order in database
	newOrder := model.CrowdfundingOrder{
		OrderID:       orderID,
		Name:          name,
		PhoneNumber:   phoneNumber,
		NPM:           npm,
		WalletAddress: model.MicroBitcoinWalletAddress,
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
		time.Sleep(model.MicroBitcoinExpirySeconds * time.Second)

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
		WalletAddress: model.MicroBitcoinWalletAddress,
		PaymentMethod: model.MicroBitcoin,
	})
}

// CreateRavencoinOrder creates a new Ravencoin payment order
func CreateRavencoinOrder(w http.ResponseWriter, r *http.Request) {
	var request model.CreateRavencoinOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Request",
			"Failed to process create Ravencoin order request.",
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

	// Validate amount if provided
	if request.Amount <= 0 {
		sendCrowdfundingDiscordEmbed(
			"🔴 Error: Invalid Order Parameters",
			"Ravencoin order creation failed due to invalid amount.",
			ColorRed,
			[]DiscordEmbedField{
				{Name: "Name", Value: name, Inline: true},
				{Name: "Phone", Value: phoneNumber, Inline: true},
				{Name: "Amount", Value: strconv.FormatFloat(request.Amount, 'f', 8, 64), Inline: true},
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
	expiryTime := time.Now().Add(model.RavencoinExpirySeconds * time.Second)

	// Create new order in database
	newOrder := model.CrowdfundingOrder{
		OrderID:       orderID,
		Name:          name,
		PhoneNumber:   phoneNumber,
		NPM:           npm,
		Amount:        request.Amount,
		WalletAddress: model.RavencoinWalletAddress,
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
			{Name: "Amount", Value: fmt.Sprintf("%f RVN", request.Amount), Inline: true},
			{Name: "Expires", Value: expiryTime.Format("15:04:05"), Inline: true},
			{Name: "Status", Value: "Pending", Inline: true},
		},
	)

	// Set up expiry timer
	go func() {
		time.Sleep(model.RavencoinExpirySeconds * time.Second)

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
					{Name: "Amount", Value: fmt.Sprintf("%f RVN", newOrder.Amount), Inline: true},
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
		WalletAddress: model.RavencoinWalletAddress,
		PaymentMethod: model.Ravencoin,
		Amount:        request.Amount,
	})
}

// CheckRavencoinTransaction checks with Nanopool API if a transaction occurred
func CheckRavencoinTransaction(walletAddress string) (bool, string, float64, error) {
	// API URL for checking transactions
	url := fmt.Sprintf("https://api.nanopool.org/v1/rvn/payments/%s", walletAddress)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", 0, err
	}
	defer resp.Body.Close()

	// Parse response
	var txResp model.RavencoinTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return false, "", 0, err
	}

	// Check for errors in API response
	if !txResp.Status {
		return false, "", 0, errors.New("API error: " + txResp.Error)
	}

	// Check if there are any transactions
	if len(txResp.Data) > 0 {
		// Get the most recent transaction
		latestTx := txResp.Data[0]

		// Check if it has required confirmations
		if latestTx.Confirmations >= model.RavencoinMinConfirmations {
			return true, latestTx.TxID, latestTx.Amount, nil
		}
	}

	return false, "", 0, nil
}

// CheckRavencoinMempool checks for pending transactions in the Ravencoin network
func CheckRavencoinMempool(walletAddress string) (bool, string, error) {
	// API URL for checking mempool
	url := fmt.Sprintf("https://api.nanopool.org/v1/rvn/payments_unconfirmed/%s", walletAddress)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	// Parse response
	var mempoolResp model.RavencoinAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&mempoolResp); err != nil {
		return false, "", err
	}

	// Check for errors in API response
	if !mempoolResp.Status {
		return false, "", errors.New("API error: " + mempoolResp.Error)
	}

	// If data is an array and has elements, there are pending transactions
	if data, ok := mempoolResp.Data.([]interface{}); ok && len(data) > 0 {
		// Extract txid from first transaction
		if tx, ok := data[0].(map[string]interface{}); ok {
			if txid, ok := tx["txid"].(string); ok {
				return true, txid, nil
			}
		}
	}

	return false, "", nil
}

// CheckRavencoinStep2Handler checks transaction confirmation status
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

	// Check transaction status with Nanopool API
	isConfirmed, _, amount, err := CheckRavencoinTransaction(order.WalletAddress)
	if err != nil {
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "Error checking transaction status: " + err.Error(),
			Step1Complete: true,
			Step2Complete: false,
			Step3Complete: false,
			TxID:          txid,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	if isConfirmed {
		// Transaction confirmed, complete the payment process

		// Update order status to success
		_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
			context.Background(),
			bson.M{"orderId": orderID},
			bson.M{"$set": bson.M{
				"status":    "success",
				"txid":      txid,
				"amount":    amount,
				"updatedAt": time.Now(),
			}},
		)
		if err != nil {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Transaction confirmed but error updating order status: " + err.Error(),
				Step1Complete: true,
				Step2Complete: true,
				Step3Complete: false,
				TxID:          txid,
				Amount:        amount,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		// Update payment totals
		updateCrowdfundingTotal(amount, model.Ravencoin)

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
				{Name: "Amount", Value: fmt.Sprintf("%f RVN", amount), Inline: true},
			},
		)

		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "success",
			Message:       "Payment confirmed successfully!",
			Step1Complete: true,
			Step2Complete: true,
			Step3Complete: true,
			TxID:          txid,
			Amount:        amount,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// Transaction not yet confirmed
	at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
		Success:       true,
		Status:        "pending",
		Message:       "Transaction found but waiting for confirmations. Please wait.",
		Step1Complete: true,
		Step2Complete: false,
		Step3Complete: false,
		TxID:          txid,
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

	// For Ravencoin payments, if the payment is pending, check mempool
	if order.PaymentMethod == model.Ravencoin && order.Status == "pending" {
		// Check if there's a transaction in the mempool
		mempoolStatus, mempoolTxid, err := CheckRavencoinMempool(order.WalletAddress)

		if err != nil {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking mempool failed: " + err.Error(),
				Step1Complete: false,
				Step2Complete: false,
				TxID:          "",
				PaymentMethod: model.Ravencoin,
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
				TxID:          mempoolTxid,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		// If not in mempool, check for confirmed transactions
		isConfirmed, txid, amount, err := CheckRavencoinTransaction(order.WalletAddress)
		if err != nil {
			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "pending",
				Message:       "Checking confirmed transactions failed: " + err.Error(),
				Step1Complete: false,
				Step2Complete: false,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		if isConfirmed && txid != "" {
			// Transaction is confirmed, update order
			_, err = config.Mongoconn.Collection("crowdfundingorders").UpdateOne(
				context.Background(),
				bson.M{"orderId": orderID},
				bson.M{"$set": bson.M{
					"status":    "success",
					"txid":      txid,
					"amount":    amount,
					"updatedAt": time.Now(),
				}},
			)
			if err != nil {
				at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
					Success:       true,
					Status:        "pending",
					Message:       "Transaction confirmed but error updating order status: " + err.Error(),
					Step1Complete: true,
					Step2Complete: true,
					TxID:          txid,
					Amount:        amount,
					PaymentMethod: model.Ravencoin,
				})
				return
			}

			// Update payment totals
			updateCrowdfundingTotal(amount, model.Ravencoin)

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
			}

			// Log successful payment
			sendCrowdfundingDiscordEmbed(
				"✅ Ravencoin Payment Successful",
				"A Ravencoin payment has been confirmed automatically.",
				ColorGreen,
				[]DiscordEmbedField{
					{Name: "Order ID", Value: orderID, Inline: true},
					{Name: "Customer", Value: order.Name, Inline: true},
					{Name: "Transaction ID", Value: txid, Inline: true},
					{Name: "Amount", Value: fmt.Sprintf("%f RVN", amount), Inline: true},
				},
			)

			at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
				Success:       true,
				Status:        "success",
				Message:       "Payment confirmed successfully!",
				Step1Complete: true,
				Step2Complete: true,
				TxID:          txid,
				Amount:        amount,
				PaymentMethod: model.Ravencoin,
			})
			return
		}

		// No transaction found yet
		at.WriteJSON(w, http.StatusOK, model.CrowdfundingPaymentResponse{
			Success:       true,
			Status:        "pending",
			Message:       "No transaction found yet. Please make the payment or wait if you've already sent it.",
			Step1Complete: false,
			Step2Complete: false,
			PaymentMethod: model.Ravencoin,
		})
		return
	}

	// For already processed payments (MicroBitcoin or Ravencoin)
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
	url := "https://api.mbc.wiki/mempool/" + model.MicroBitcoinWalletAddress

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
	url := "https://api.mbc.wiki/history/" + model.MicroBitcoinWalletAddress

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
			if addr == model.MicroBitcoinWalletAddress {
				amount = vout.Value
				break
			}
		}
		// Alternative check using the single address field
		if vout.ScriptPubKey.Address == model.MicroBitcoinWalletAddress {
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
