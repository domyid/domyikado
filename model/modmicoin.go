package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MerchCoinPayment represents a payment made via MicroBitcoin
type MerchCoinPayment struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	OrderID      string             `bson:"orderid" json:"orderid"`
	SenderWallet string             `bson:"senderwallet" json:"senderwallet"`
	Amount       float64            `bson:"amount,omitempty" json:"amount,omitempty"`
	Status       string             `bson:"status" json:"status"` // pending, success, failed
	TxHash       string             `bson:"txhash,omitempty" json:"txhash,omitempty"`
	CreatedAt    time.Time          `bson:"createdat" json:"createdat"`
	UpdatedAt    time.Time          `bson:"updatedat,omitempty" json:"updatedat,omitempty"`
	ExpiryTime   time.Time          `bson:"expirytime" json:"expirytime"`
	Notes        string             `bson:"notes,omitempty" json:"notes,omitempty"`
}

// MerchCoinOrderRequest represents a request to create a new MicroBitcoin payment order
type MerchCoinOrderRequest struct {
	WalletCode string  `json:"walletcode"`
	Amount     float64 `json:"amount,omitempty"`
}

// MerchCoinOrderResponse represents the response when creating a new MicroBitcoin payment order
type MerchCoinOrderResponse struct {
	Success    bool      `json:"success"`
	Message    string    `json:"message,omitempty"`
	OrderID    string    `json:"orderId,omitempty"`
	WalletCode string    `json:"walletCode,omitempty"`
	ExpiryTime time.Time `json:"expiryTime,omitempty"`
	QRImageURL string    `json:"qrImageUrl,omitempty"`
}

// MerchCoinPaymentStatusResponse represents the response for a payment status check
type MerchCoinPaymentStatusResponse struct {
	Success       bool      `json:"success"`
	OrderID       string    `json:"orderId,omitempty"`
	Status        string    `json:"status,omitempty"` // pending, success, failed
	Message       string    `json:"message,omitempty"`
	WalletCode    string    `json:"walletCode,omitempty"`
	Amount        float64   `json:"amount,omitempty"`
	TxHash        string    `json:"txHash,omitempty"`
	CreatedAt     time.Time `json:"createdAt,omitempty"`
	ProcessedAt   time.Time `json:"processedAt,omitempty"`
	RemainingTime int       `json:"remainingTime,omitempty"`
}

// MerchCoinQueueStatusResponse represents the response for the queue status check
type MerchCoinQueueStatusResponse struct {
	Success      bool      `json:"success"`
	IsProcessing bool      `json:"isProcessing"`
	OrderID      string    `json:"orderId,omitempty"`
	WalletCode   string    `json:"walletCode,omitempty"`
	ExpiryTime   time.Time `json:"expiryTime,omitempty"`
	Message      string    `json:"message,omitempty"`
}

// MerchCoinPaymentNotification represents an incoming notification about a payment
type MerchCoinPaymentNotification struct {
	TxHash       string  `json:"txHash"`
	SenderWallet string  `json:"senderWallet"`
	Amount       float64 `json:"amount"`
}

// MerchCoinPaymentTotalsResponse represents statistics about payments
type MerchCoinPaymentTotalsResponse struct {
	Success     bool      `json:"success"`
	TotalAmount float64   `json:"totalAmount"`
	Count       int       `json:"count"`
	LastUpdated time.Time `json:"lastUpdated,omitempty"`
}

// MerchCoinTxAPIResponse represents the response from the MicroBitcoin API
type MerchCoinTxAPIResponse struct {
	Status  string                 `json:"status"`
	Txs     []MerchCoinTransaction `json:"txs"`
	Message string                 `json:"message,omitempty"`
}

// MerchCoinTransaction represents a transaction from the MicroBitcoin API
type MerchCoinTransaction struct {
	TxID         string    `json:"txid"`
	BlockHeight  int       `json:"blockheight"`
	Time         time.Time `json:"time"`
	Amount       float64   `json:"amount"`
	Fee          float64   `json:"fee"`
	SenderAddr   string    `json:"senderaddr"`
	ReceiverAddr string    `json:"receiveraddr"`
}

// MerchCoinSimulatePaymentRequest represents a request to simulate a payment
type MerchCoinSimulatePaymentRequest struct {
	OrderID      string  `json:"orderId"`
	SenderWallet string  `json:"senderWallet"`
	Amount       float64 `json:"amount"`
}
