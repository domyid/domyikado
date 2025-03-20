package controller

import (
	"bytes"
	"context"
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
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// Discord webhook URL for logging
	CrowdfundingDiscordWebhookURL = "https://discord.com/api/webhooks/1348044639818485790/DOsYYebYjrTN48wZVDOPrO4j20X5J3pMAbOdPOUkrJuiXk5niqOjV9ZZ2r06th0jXMhh"
	MicroBitcoinWalletAddress     = "BXheTnryBeec7Ere3zsuRmWjB1LiyCFpec"

	// Expiry times for different payment methods
	QRISExpirySeconds         = 300 // 5 minutes
	MicroBitcoinExpirySeconds = 900 // 15 minutes
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
			TotalQRISAmount:    0,
			QRISCount:          0,
			TotalBitcoinAmount: 0,
			BitcoinCount:       0,
			TotalAmount:        0,
			TotalCount:         0,
			LastUpdated:        time.Now(),
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
	} else {
		return fmt.Sprintf("%f MBC", amount)
	}
}

// Extract user info from token
func extractUserInfoFromToken(r *http.Request) (phoneNumber, name, npm string, err error) {
	// Get login token from header
	token := at.GetLoginFromHeader(r)
	if token == "" {
		return "", "", "", errors.New("token not found in header")
	}

	// Decode token
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid token: %v", err)
	}

	// Extract phone number from payload
	phoneNumber = payload.Id
	if phoneNumber == "" {
		return "", "", "", errors.New("phone number not found in token")
	}

	// Extract name from payload alias
	name = payload.Alias

	// Get user data from MongoDB to get NPM
	var user model.Userdomyikado
	err = config.Mongoconn.Collection("user").FindOne(context.Background(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		// User not found, but we still have phone number and name
		return phoneNumber, name, "", nil
	}

	// If we found the user, get the NPM
	return phoneNumber, user.Name, user.NPM, nil
}

// GetUserInfo returns the user information extracted from the authentication token
func GetUserInfo(w http.ResponseWriter, r *http.Request) {
	phoneNumber, name, npm, err := extractUserInfoFromToken(r)
	if err != nil {
		at.WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Authentication failed: " + err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"phoneNumber": phoneNumber,
		"name":        name,
		"npm":         npm,
	})
}

// 2
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
	//3
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

	// For already processed MicroBitcoin payments
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

// 4
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
			TotalQRISAmount:    0,
			QRISCount:          0,
			TotalBitcoinAmount: 0,
			BitcoinCount:       0,
			TotalAmount:        0,
			TotalCount:         0,
			LastUpdated:        time.Now(),
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
