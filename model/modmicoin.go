// package model

// import (
// 	"time"

// 	"go.mongodb.org/mongo-driver/bson/primitive"
// )

// // BlockchainInfo represents blockchain metadata
// type BlockchainInfo struct {
// 	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	Chain               string             `json:"chain" bson:"chain"`
// 	Blocks              int                `json:"blocks" bson:"blocks"`
// 	Headers             int                `json:"headers" bson:"headers"`
// 	Bestblockhash       string             `json:"bestblockhash" bson:"bestblockhash"`
// 	Difficulty          float64            `json:"difficulty" bson:"difficulty"`
// 	Mediantime          int64              `json:"mediantime" bson:"mediantime"`
// 	Verificationprogress float64           `json:"verificationprogress" bson:"verificationprogress"`
// 	Chainwork           string             `json:"chainwork" bson:"chainwork"`
// 	Size_on_disk        int64              `json:"size_on_disk" bson:"size_on_disk"`
// 	Pruned              bool               `json:"pruned" bson:"pruned"`
// 	FetchedAt           time.Time          `bson:"fetched_at" json:"fetched_at"`
// }

// // Block represents a single block in the blockchain
// type Block struct {
// 	ID                primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	Hash              string             `json:"hash" bson:"hash"`
// 	Confirmations     int                `json:"confirmations" bson:"confirmations"`
// 	StrippedSize      int                `json:"strippedsize" bson:"strippedsize"`
// 	Size              int                `json:"size" bson:"size"`
// 	Weight            int                `json:"weight" bson:"weight"`
// 	Height            int                `json:"height" bson:"height"`
// 	Version           int                `json:"version" bson:"version"`
// 	VersionHex        string             `json:"versionHex" bson:"versionHex"`
// 	MerkleRoot        string             `json:"merkleroot" bson:"merkleroot"`
// 	Tx                []string           `json:"tx" bson:"tx"`
// 	Time              int64              `json:"time" bson:"time"`
// 	MedianTime        int64              `json:"mediantime" bson:"mediantime"`
// 	Nonce             int                `json:"nonce" bson:"nonce"`
// 	Bits              string             `json:"bits" bson:"bits"`
// 	Difficulty        float64            `json:"difficulty" bson:"difficulty"`
// 	Chainwork         string             `json:"chainwork" bson:"chainwork"`
// 	NTx               int                `json:"nTx" bson:"nTx"`
// 	PreviousBlockHash string             `json:"previousblockhash" bson:"previousblockhash"`
// 	NextBlockHash     string             `json:"nextblockhash,omitempty" bson:"nextblockhash,omitempty"`
// 	FetchedAt         time.Time          `bson:"fetched_at" json:"fetched_at"`
// }

// // Transaction represents a cryptocurrency transaction
// type Transaction struct {
// 	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	TxID          string             `json:"txid" bson:"txid"`
// 	Hash          string             `json:"hash" bson:"hash"`
// 	Version       int                `json:"version" bson:"version"`
// 	Size          int                `json:"size" bson:"size"`
// 	VSize         int                `json:"vsize" bson:"vsize"`
// 	Weight        int                `json:"weight" bson:"weight"`
// 	LockTime      int                `json:"locktime" bson:"locktime"`
// 	Vin           []TxInput          `json:"vin" bson:"vin"`
// 	Vout          []TxOutput         `json:"vout" bson:"vout"`
// 	Hex           string             `json:"hex" bson:"hex"`
// 	BlockHash     string             `json:"blockhash,omitempty" bson:"blockhash,omitempty"`
// 	Confirmations int                `json:"confirmations,omitempty" bson:"confirmations,omitempty"`
// 	Time          int64              `json:"time,omitempty" bson:"time,omitempty"`
// 	BlockTime     int64              `json:"blocktime,omitempty" bson:"blocktime,omitempty"`
// 	FetchedAt     time.Time          `bson:"fetched_at" json:"fetched_at"`
// }

// // TxInput represents an input in a transaction
// type TxInput struct {
// 	TxID      string `json:"txid,omitempty" bson:"txid,omitempty"`
// 	Vout      int    `json:"vout,omitempty" bson:"vout,omitempty"`
// 	ScriptSig struct {
// 		Asm string `json:"asm" bson:"asm"`
// 		Hex string `json:"hex" bson:"hex"`
// 	} `json:"scriptSig,omitempty" bson:"scriptSig,omitempty"`
// 	Coinbase string   `json:"coinbase,omitempty" bson:"coinbase,omitempty"`
// 	Sequence int64    `json:"sequence" bson:"sequence"`
// 	Witness  []string `json:"witness,omitempty" bson:"witness,omitempty"`
// }

// // TxOutput represents an output in a transaction
// type TxOutput struct {
// 	Value        float64 `json:"value" bson:"value"`
// 	N            int     `json:"n" bson:"n"`
// 	ScriptPubKey struct {
// 		Asm       string   `json:"asm" bson:"asm"`
// 		Hex       string   `json:"hex" bson:"hex"`
// 		ReqSigs   int      `json:"reqSigs,omitempty" bson:"reqSigs,omitempty"`
// 		Type      string   `json:"type" bson:"type"`
// 		Addresses []string `json:"addresses,omitempty" bson:"addresses,omitempty"`
// 	} `json:"scriptPubKey" bson:"scriptPubKey"`
// }

// // MicroBitcoinSyncStatus tracks the sync status of the MicroBitcoin data
// type MicroBitcoinSyncStatus struct {
// 	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	LastSyncedBlock    int                `json:"last_synced_block" bson:"last_synced_block"`
// 	LastSyncedTime     time.Time          `json:"last_synced_time" bson:"last_synced_time"`
// 	SyncInProgress     bool               `json:"sync_in_progress" bson:"sync_in_progress"`
// 	TotalBlocksSynced  int                `json:"total_blocks_synced" bson:"total_blocks_synced"`
// 	TotalTxSynced      int                `json:"total_tx_synced" bson:"total_tx_synced"`
// 	LastSyncDuration   float64            `json:"last_sync_duration" bson:"last_sync_duration"`
// }

// // MicroBitcoinStats represents statistics about MicroBitcoin data
// type MicroBitcoinStats struct {
// 	ID                      primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	TotalBlocks             int                `json:"total_blocks" bson:"total_blocks"`
// 	TotalTransactions       int                `json:"total_transactions" bson:"total_transactions"`
// 	AverageBlockSize        float64            `json:"average_block_size" bson:"average_block_size"`
// 	AverageTransactionsPerBlock float64        `json:"average_transactions_per_block" bson:"average_transactions_per_block"`
// 	LastUpdated             time.Time          `json:"last_updated" bson:"last_updated"`
// }