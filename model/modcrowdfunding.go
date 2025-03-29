package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PaymentMethod represents the type of payment method used
type PaymentMethod string

const (
	QRIS         PaymentMethod = "qris"
	MicroBitcoin PaymentMethod = "microbitcoin"
	Ravencoin    PaymentMethod = "ravencoin" // Added Ravencoin payment method
)

// CrowdfundingOrder struct to store unified payment data
type CrowdfundingOrder struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	OrderID       string             `json:"orderId" bson:"orderId"`
	Name          string             `json:"name" bson:"name"`
	PhoneNumber   string             `json:"phoneNumber" bson:"phoneNumber"`
	NPM           string             `json:"npm,omitempty" bson:"npm,omitempty"`
	Amount        float64            `json:"amount" bson:"amount"`
	PaymentMethod PaymentMethod      `json:"paymentMethod" bson:"paymentMethod"`
	WonpayCode    string             `json:"wonpayCode,omitempty" bson:"wonpayCode,omitempty"`       // Used for MicroBitcoin
	WalletAddress string             `json:"walletAddress,omitempty" bson:"walletAddress,omitempty"` // Used for MicroBitcoin and Ravencoin
	TxID          string             `json:"txid,omitempty" bson:"txid,omitempty"`                   // Used for MicroBitcoin and Ravencoin
	Timestamp     time.Time          `json:"timestamp" bson:"timestamp"`
	ExpiryTime    time.Time          `json:"expiryTime" bson:"expiryTime"`
	Status        string             `json:"status" bson:"status"` // pending, success, failed
	UpdatedAt     time.Time          `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

// CrowdfundingQueue struct to manage payment processing
type CrowdfundingQueue struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	IsProcessing   bool               `json:"isProcessing" bson:"isProcessing"`
	CurrentOrderID string             `json:"currentOrderId" bson:"currentOrderId"`
	PaymentMethod  PaymentMethod      `json:"paymentMethod" bson:"paymentMethod"`
	ExpiryTime     time.Time          `json:"expiryTime" bson:"expiryTime"`
}

// CrowdfundingTotal struct to track total payments
type CrowdfundingTotal struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TotalQRISAmount      float64            `json:"totalQRISAmount" bson:"totalQRISAmount"`
	QRISCount            int                `json:"qrisCount" bson:"qrisCount"`
	TotalBitcoinAmount   float64            `json:"totalBitcoinAmount" bson:"totalBitcoinAmount"`
	BitcoinCount         int                `json:"bitcoinCount" bson:"bitcoinCount"`
	TotalRavencoinAmount float64            `json:"totalRavencoinAmount" bson:"totalRavencoinAmount"` // Added for Ravencoin
	RavencoinCount       int                `json:"ravencoinCount" bson:"ravencoinCount"`             // Added for Ravencoin
	TotalAmount          float64            `json:"totalAmount" bson:"totalAmount"`
	TotalCount           int                `json:"totalCount" bson:"totalCount"`
	LastUpdated          time.Time          `json:"lastUpdated" bson:"lastUpdated"`
}

// CreateOrderRequest represents the request body for creating a QRIS order
type CreateQRISOrderRequest struct {
	Amount float64 `json:"amount"`
}

// CreateMicroBitcoinOrderRequest represents the request body for creating a MicroBitcoin order
type CreateMicroBitcoinOrderRequest struct {
	WonpayCode string `json:"wonpayCode"`
}

// CreateRavencoinOrderRequest represents the request body for creating a Ravencoin order
type CreateRavencoinOrderRequest struct {
	// Currently empty as we don't need special parameters for Ravencoin orders
}

// NotificationRequest for receiving notification text from payment gateway
type NotificationRequest struct {
	NotificationText string `json:"notification_text"`
}

// Make sure your struct definition includes both fields for compatibility
type CrowdfundingPaymentResponse struct {
	Success       bool          `json:"success"`
	Message       string        `json:"message,omitempty"`
	OrderID       string        `json:"orderId,omitempty"`
	ExpiryTime    time.Time     `json:"expiryTime,omitempty"`
	QRISImageURL  string        `json:"qrisImageUrl,omitempty"`
	QRImageURL    string        `json:"qrImageUrl,omitempty"` // Added for backward compatibility
	QueueStatus   bool          `json:"queueStatus,omitempty"`
	Status        string        `json:"status,omitempty"`
	IsProcessing  bool          `json:"isProcessing,omitempty"`
	PaymentMethod PaymentMethod `json:"paymentMethod,omitempty"`

	// Bitcoin and Ravencoin specific fields
	WalletAddress string  `json:"walletAddress,omitempty"`
	Step1Complete bool    `json:"step1Complete,omitempty"`
	Step2Complete bool    `json:"step2Complete,omitempty"`
	Step3Complete bool    `json:"step3Complete,omitempty"`
	TxID          string  `json:"txid,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
}

// MicroBitcoin API response structures
// MicroBitcoinMempoolResponse represents the response from the mempool API
type MicroBitcoinMempoolResponse struct {
	Error  *string                   `json:"error"`
	ID     string                    `json:"id"`
	Result MicroBitcoinMempoolResult `json:"result"`
}

type MicroBitcoinMempoolResult struct {
	Tx      []MicroBitcoinMempoolTx `json:"tx"`
	TxCount int                     `json:"txcount"`
}

type MicroBitcoinMempoolTx struct {
	Index     int    `json:"index"`
	Satoshis  int64  `json:"satoshis"`
	Timestamp int64  `json:"timestamp"`
	TxID      string `json:"txid"`
}

// MicroBitcoinHistoryResponse represents the response from the transaction history API
type MicroBitcoinHistoryResponse struct {
	Error  *string                   `json:"error"`
	ID     string                    `json:"id"`
	Result MicroBitcoinHistoryResult `json:"result"`
}

type MicroBitcoinHistoryResult struct {
	Tx      []string `json:"tx"`
	TxCount int      `json:"txcount"`
}

// MicroBitcoinTransactionResponse represents the response from the transaction details API
type MicroBitcoinTransactionResponse struct {
	Error  *string                       `json:"error"`
	ID     string                        `json:"id"`
	Result MicroBitcoinTransactionResult `json:"result"`
}

type MicroBitcoinTransactionResult struct {
	Amount        int64                         `json:"amount"`
	BlockHash     string                        `json:"blockhash"`
	BlockTime     int64                         `json:"blocktime"`
	Confirmations int                           `json:"confirmations"`
	Hash          string                        `json:"hash"`
	Height        int                           `json:"height"`
	Hex           string                        `json:"hex"`
	LockTime      int                           `json:"locktime"`
	Size          int                           `json:"size"`
	Time          int64                         `json:"time"`
	TxID          string                        `json:"txid"`
	Version       int                           `json:"version"`
	Vin           []MicroBitcoinTransactionVin  `json:"vin"`
	Vout          []MicroBitcoinTransactionVout `json:"vout"`
	VSize         int                           `json:"vsize"`
	Weight        int                           `json:"weight"`
}

type MicroBitcoinTransactionVin struct {
	ScriptPubKey MicroBitcoinScriptPubKey `json:"scriptPubKey"`
	ScriptSig    MicroBitcoinScriptSig    `json:"scriptSig"`
	Sequence     int64                    `json:"sequence"`
	TxID         string                   `json:"txid"`
	Value        int64                    `json:"value"`
	Vout         int                      `json:"vout"`
}

type MicroBitcoinScriptPubKey struct {
	Address   string   `json:"address"`
	Addresses []string `json:"addresses"`
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	Type      string   `json:"type"`
}

type MicroBitcoinScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type MicroBitcoinTransactionVout struct {
	N            int                      `json:"n"`
	ScriptPubKey MicroBitcoinScriptPubKey `json:"scriptPubKey"`
	Value        int64                    `json:"value"`
}

// Ravencoin API response structures
// RavencoinAddressResponse represents the response from the address API
type RavencoinAddressResponse struct {
	Page               int      `json:"page"`
	TotalPages         int      `json:"totalPages"`
	ItemsOnPage        int      `json:"itemsOnPage"`
	Address            string   `json:"address"`
	Balance            string   `json:"balance"`
	TotalReceived      string   `json:"totalReceived"`
	TotalSent          string   `json:"totalSent"`
	UnconfirmedBalance string   `json:"unconfirmedBalance"`
	UnconfirmedTxs     int      `json:"unconfirmedTxs"`
	Txs                int      `json:"txs"`
	Txids              []string `json:"txids"`
}

// RavencoinTransactionResponse represents the response from the transaction details API
type RavencoinTransactionResponse struct {
	Txid          string                     `json:"txid"`
	Version       int                        `json:"version"`
	Vin           []RavencoinTransactionVin  `json:"vin"`
	Vout          []RavencoinTransactionVout `json:"vout"`
	BlockHash     string                     `json:"blockhash"`
	BlockHeight   int                        `json:"blockheight"`
	Confirmations int                        `json:"confirmations"`
	Time          int64                      `json:"time"`
	BlockTime     int64                      `json:"blocktime"`
	ValueOut      string                     `json:"valueOut"`
	ValueIn       string                     `json:"valueIn"`
	Fees          string                     `json:"fees"`
	Hex           string                     `json:"hex"`
}

type RavencoinTransactionVin struct {
	Txid      string             `json:"txid"`
	Vout      int                `json:"vout"`
	Sequence  int64              `json:"sequence"`
	N         int                `json:"n"`
	ScriptSig RavencoinScriptSig `json:"scriptSig"`
	Addresses []string           `json:"addresses"`
	Value     string             `json:"value"`
}

type RavencoinScriptSig struct {
	Hex string `json:"hex"`
	Asm string `json:"asm"`
}

type RavencoinTransactionVout struct {
	Value        string                `json:"value"`
	N            int                   `json:"n"`
	ScriptPubKey RavencoinScriptPubKey `json:"scriptPubKey"`
	Spent        bool                  `json:"spent"`
}

type RavencoinScriptPubKey struct {
	Hex       string   `json:"hex"`
	Addresses []string `json:"addresses"`
}
