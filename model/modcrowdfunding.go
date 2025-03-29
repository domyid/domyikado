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
	Ravencoin    PaymentMethod = "ravencoin"
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
	TxID          string             `json:"txid,omitempty" bson:"txid,omitempty"`                   // Used for crypto payments
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
	TotalRavencoinAmount float64            `json:"totalRavencoinAmount" bson:"totalRavencoinAmount"`
	RavencoinCount       int                `json:"ravencoinCount" bson:"ravencoinCount"`
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
	// Similar to MicroBitcoin, empty struct for now
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

	// Crypto specific fields
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
// RavencoinAccountResponse represents the response from the nanopool account API
type RavencoinAccountResponse struct {
	Status bool                 `json:"status"`
	Data   RavencoinAccountData `json:"data"`
	Error  string               `json:"error,omitempty"`
}

type RavencoinAccountData struct {
	Balance     float64            `json:"balance"`
	UnConfirmed float64            `json:"unconfirmed"`
	Hashrate    float64            `json:"hashrate"`
	Workers     []RavencoinWorker  `json:"workers"`
	Payments    []RavencoinPayment `json:"payments"`
}

type RavencoinWorker struct {
	ID        string  `json:"id"`
	Hashrate  float64 `json:"hashrate"`
	LastShare int64   `json:"lastshare"`
}

type RavencoinPayment struct {
	Date      int64   `json:"date"`
	TxHash    string  `json:"txHash"`
	Amount    float64 `json:"amount"`
	Confirmed bool    `json:"confirmed"`
}

// RavencoinTransactionResponse represents the transaction check response
type RavencoinTransactionResponse struct {
	Status bool                     `json:"status"`
	Data   RavencoinTransactionData `json:"data"`
	Error  string                   `json:"error,omitempty"`
}

type RavencoinTransactionData struct {
	Hash          string    `json:"hash"`
	BlockNumber   int       `json:"blockNumber"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	Value         float64   `json:"value"`
	Time          time.Time `json:"time"`
	Confirmations int       `json:"confirmations"`
}
