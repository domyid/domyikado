package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Order struct to store payment data
type Order struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	OrderID   string             `json:"orderId" bson:"orderId"`
	Name      string             `json:"name" bson:"name"`
	Amount    float64            `json:"amount" bson:"amount"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
	Status    string             `json:"status" bson:"status"`
}

// Queue struct to manage payment processing
type Queue struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	IsProcessing   bool               `json:"isProcessing" bson:"isProcessing"`
	CurrentOrderID string             `json:"currentOrderId" bson:"currentOrderId"`
	ExpiryTime     time.Time          `json:"expiryTime" bson:"expiryTime"`
}

// CreateOrderRequest represents the request body for creating an order
type CreateOrderRequest struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// ConfirmByAmountRequest structure for payment confirmation by amount
type ConfirmByAmountRequest struct {
	Amount float64 `json:"amount"`
}

// NotificationRequest for receiving notification text from MacroDroid
type NotificationRequest struct {
	NotificationText string `json:"notification_text"`
}

// PaymentResponse struct for API responses
type PaymentResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message,omitempty"`
	OrderID      string    `json:"orderId,omitempty"`
	ExpiryTime   time.Time `json:"expiryTime,omitempty"`
	QRISImageURL string    `json:"qrisImageUrl,omitempty"`
	QueueStatus  bool      `json:"queueStatus,omitempty"`
	Status       string    `json:"status,omitempty"`
	IsProcessing bool      `json:"isProcessing,omitempty"`
}
