package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MerchCoinOrder stores payment order data
type MerchCoinOrder struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	OrderID   string             `json:"orderId" bson:"orderId"`
	Name      string             `json:"name" bson:"name"`
	Amount    float64            `json:"amount" bson:"amount"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
	Status    string             `json:"status" bson:"status"`
}

// MerchCoinQueue manages the payment processing queue
type MerchCoinQueue struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	IsProcessing  bool               `json:"isProcessing" bson:"isProcessing"`
	CurrentOrderID string            `json:"currentOrderId" bson:"currentOrderId"`
	ExpiryTime    time.Time          `json:"expiryTime" bson:"expiryTime"`
}

// MerchCoinTotal tracks payment statistics
type MerchCoinTotal struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TotalAmount float64            `json:"totalAmount" bson:"totalAmount"`
	Count       int                `json:"count" bson:"count"`
	LastUpdated time.Time          `json:"lastUpdated" bson:"lastUpdated"`
}

// MerchCoinCreateRequest is the request body for creating an order
type MerchCoinCreateRequest struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// MerchCoinNotification is the request body for payment notifications
type MerchCoinNotification struct {
	NotificationText string `json:"notification_text"`
	TransactionID    string `json:"transaction_id"`
}

// MerchCoinResponse is the API response format
type MerchCoinResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message,omitempty"`
	OrderID      string    `json:"orderId,omitempty"`
	ExpiryTime   time.Time `json:"expiryTime,omitempty"`
	QRImageURL   string    `json:"qrImageUrl,omitempty"`
	QueueStatus  bool      `json:"queueStatus,omitempty"`
	Status       string    `json:"status,omitempty"`
	IsProcessing bool      `json:"isProcessing,omitempty"`
}