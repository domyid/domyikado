package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MerchCoinOrder struct to store payment data
type MerchCoinOrder struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	OrderID    string             `json:"orderId" bson:"orderId"`
	WonpayCode string             `json:"wonpayCode" bson:"wonpayCode"`
	Timestamp  time.Time          `json:"timestamp" bson:"timestamp"`
	Status     string             `json:"status" bson:"status"`
	TxID       string             `json:"txid,omitempty" bson:"txid,omitempty"`
	Amount     float64            `json:"amount,omitempty" bson:"amount,omitempty"`
}

// MerchCoinQueue struct to manage payment processing
type MerchCoinQueue struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	IsProcessing   bool               `json:"isProcessing" bson:"isProcessing"`
	CurrentOrderID string             `json:"currentOrderId" bson:"currentOrderId"`
	ExpiryTime     time.Time          `json:"expiryTime" bson:"expiryTime"`
}

// MerchCoinCreateOrderRequest represents the request body for creating an order
type MerchCoinCreateOrderRequest struct {
	WonpayCode string `json:"wonpayCode"`
}

// MerchCoinConfirmRequest represents the request body for manually confirming a payment
type MerchCoinConfirmRequest struct {
	TxID   string  `json:"txid"`
	Amount float64 `json:"amount"`
}

// MerchCoinSimulateRequest represents the request body for simulating a payment
type MerchCoinSimulateRequest struct {
	TxID   string  `json:"txid"`
	Amount float64 `json:"amount"`
}

// MerchCoinNotificationRequest for receiving notification text from payment gateway
type MerchCoinNotificationRequest struct {
	NotificationText string `json:"notification_text"`
}

// MerchCoinPaymentResponse struct for API responses
type MerchCoinPaymentResponse struct {
	Success       bool      `json:"success"`
	Message       string    `json:"message,omitempty"`
	OrderID       string    `json:"orderId,omitempty"`
	ExpiryTime    time.Time `json:"expiryTime,omitempty"`
	QRImageURL    string    `json:"qrImageUrl,omitempty"`
	QueueStatus   bool      `json:"queueStatus,omitempty"`
	Status        string    `json:"status,omitempty"`
	IsProcessing  bool      `json:"isProcessing,omitempty"`
	WalletAddress string    `json:"walletAddress,omitempty"`

	// Transaction verification steps
	Step1Complete bool    `json:"step1Complete,omitempty"`
	Step2Complete bool    `json:"step2Complete,omitempty"` // Added this field
	Step3Complete bool    `json:"step3Complete,omitempty"`
	TxID          string  `json:"txid,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
}

// MerchCoinPaymentTotal struct to track total payments
type MerchCoinPaymentTotal struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TotalAmount float64            `json:"totalAmount" bson:"totalAmount"`
	Count       int                `json:"count" bson:"count"`
	LastUpdated time.Time          `json:"lastUpdated" bson:"lastUpdated"`
}

// MerchCoin API response structures

// MerchCoinMempoolResponse represents the response from the mempool API
type MerchCoinMempoolResponse struct {
	Error  *string                `json:"error"`
	ID     string                 `json:"id"`
	Result MerchCoinMempoolResult `json:"result"`
}

type MerchCoinMempoolResult struct {
	Tx      []MerchCoinMempoolTx `json:"tx"`
	TxCount int                  `json:"txcount"`
}

type MerchCoinMempoolTx struct {
	Index     int    `json:"index"`
	Satoshis  int64  `json:"satoshis"`
	Timestamp int64  `json:"timestamp"`
	TxID      string `json:"txid"`
}

// MerchCoinHistoryResponse represents the response from the transaction history API
type MerchCoinHistoryResponse struct {
	Error  *string                `json:"error"`
	ID     string                 `json:"id"`
	Result MerchCoinHistoryResult `json:"result"`
}

type MerchCoinHistoryResult struct {
	Tx      []string `json:"tx"`
	TxCount int      `json:"txcount"`
}

// MerchCoinTransactionResponse represents the response from the transaction details API
type MerchCoinTransactionResponse struct {
	Error  *string                    `json:"error"`
	ID     string                     `json:"id"`
	Result MerchCoinTransactionResult `json:"result"`
}

type MerchCoinTransactionResult struct {
	Amount        int64                      `json:"amount"`
	BlockHash     string                     `json:"blockhash"`
	BlockTime     int64                      `json:"blocktime"`
	Confirmations int                        `json:"confirmations"`
	Hash          string                     `json:"hash"`
	Height        int                        `json:"height"`
	Hex           string                     `json:"hex"`
	LockTime      int                        `json:"locktime"`
	Size          int                        `json:"size"`
	Time          int64                      `json:"time"`
	TxID          string                     `json:"txid"`
	Version       int                        `json:"version"`
	Vin           []MerchCoinTransactionVin  `json:"vin"`
	Vout          []MerchCoinTransactionVout `json:"vout"`
	VSize         int                        `json:"vsize"`
	Weight        int                        `json:"weight"`
}

type MerchCoinTransactionVin struct {
	ScriptPubKey MerchCoinScriptPubKey `json:"scriptPubKey"`
	ScriptSig    MerchCoinScriptSig    `json:"scriptSig"`
	Sequence     int64                 `json:"sequence"`
	TxID         string                `json:"txid"`
	Value        int64                 `json:"value"`
	Vout         int                   `json:"vout"`
}

type MerchCoinScriptPubKey struct {
	Address   string   `json:"address"`
	Addresses []string `json:"addresses"`
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	Type      string   `json:"type"`
}

type MerchCoinScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type MerchCoinTransactionVout struct {
	N            int                   `json:"n"`
	ScriptPubKey MerchCoinScriptPubKey `json:"scriptPubKey"`
	Value        int64                 `json:"value"`
}
