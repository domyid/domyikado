// package controller

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"
// 	"log"
// 	"net/http"
// 	"strconv"
// 	"time"

// 	"github.com/gocroot/config"
// 	"github.com/gocroot/helper/at"
// 	"github.com/gocroot/helper/atdb"
// 	"github.com/gocroot/helper/watoken"
// 	"github.com/gocroot/model"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// const (
// 	apiBaseURL              = "https://microbitcoinorg.github.io/api"
// 	blockchainInfoCollection = "merch_blockchain_info"
// 	blocksCollection        = "merch_blocks"
// 	transactionsCollection  = "merch_transactions"
// 	syncStatusCollection    = "merch_microbitcoin_sync_status"
// 	statsCollection         = "merch_microbitcoin_stats"
// )

// // GetBlockchainInfo retrieves blockchain information and returns it
// func GetBlockchainInfo(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get the latest blockchain info
// 	var blockchainInfo model.BlockchainInfo
// 	opts := options.FindOne().SetSort(bson.M{"fetched_at": -1})
// 	err = config.Mongoconn.Collection(blockchainInfoCollection).FindOne(r.Context(), bson.M{}, opts).Decode(&blockchainInfo)
	
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to retrieve blockchain info"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, blockchainInfo)
// }

// // GetLatestBlocks retrieves the latest blocks
// func GetLatestBlocks(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Parse limit parameter or default to 10
// 	limitStr := r.URL.Query().Get("limit")
// 	limit := 10
// 	if limitStr != "" {
// 		limit, err = strconv.Atoi(limitStr)
// 		if err != nil || limit < 1 {
// 			limit = 10
// 		}
// 	}

// 	// Get the latest blocks
// 	opts := options.Find().SetSort(bson.M{"height": -1}).SetLimit(int64(limit))
// 	cursor, err := config.Mongoconn.Collection(blocksCollection).Find(r.Context(), bson.M{}, opts)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to retrieve blocks"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}
// 	defer cursor.Close(r.Context())

// 	var blocks []model.Block
// 	if err = cursor.All(r.Context(), &blocks); err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to decode blocks"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, blocks)
// }

// // GetBlockByHeight retrieves a block by its height
// func GetBlockByHeight(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Parse height parameter
// 	heightStr := at.GetParam(r)
// 	height, err := strconv.Atoi(heightStr)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Invalid block height"
// 		respn.Response = "Block height must be a valid integer"
// 		at.WriteJSON(w, http.StatusBadRequest, respn)
// 		return
// 	}

// 	// Find the block
// 	var block model.Block
// 	err = config.Mongoconn.Collection(blocksCollection).FindOne(r.Context(), bson.M{"height": height}).Decode(&block)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Block not found"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusNotFound, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, block)
// }

// // GetBlockByHash retrieves a block by its hash
// func GetBlockByHash(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get hash from query parameter
// 	hash := r.URL.Query().Get("hash")
// 	if hash == "" {
// 		var respn model.Response
// 		respn.Status = "Error: Missing hash parameter"
// 		respn.Response = "Block hash must be provided"
// 		at.WriteJSON(w, http.StatusBadRequest, respn)
// 		return
// 	}

// 	// Find the block
// 	var block model.Block
// 	err = config.Mongoconn.Collection(blocksCollection).FindOne(r.Context(), bson.M{"hash": hash}).Decode(&block)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Block not found"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusNotFound, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, block)
// }

// // GetTransactionByID retrieves a transaction by its ID
// func GetTransactionByID(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get txid from parameter
// 	txid := at.GetParam(r)
// 	if txid == "" {
// 		var respn model.Response
// 		respn.Status = "Error: Missing transaction ID"
// 		respn.Response = "Transaction ID must be provided"
// 		at.WriteJSON(w, http.StatusBadRequest, respn)
// 		return
// 	}

// 	// Find the transaction
// 	var tx model.Transaction
// 	err = config.Mongoconn.Collection(transactionsCollection).FindOne(r.Context(), bson.M{"txid": txid}).Decode(&tx)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Transaction not found"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusNotFound, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, tx)
// }

// // GetBlockTransactions retrieves all transactions for a block
// func GetBlockTransactions(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get block hash from query parameter
// 	blockHash := r.URL.Query().Get("blockhash")
// 	if blockHash == "" {
// 		var respn model.Response
// 		respn.Status = "Error: Missing block hash parameter"
// 		respn.Response = "Block hash must be provided"
// 		at.WriteJSON(w, http.StatusBadRequest, respn)
// 		return
// 	}

// 	// Find transactions for this block
// 	cursor, err := config.Mongoconn.Collection(transactionsCollection).Find(r.Context(), bson.M{"blockhash": blockHash})
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to retrieve transactions"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}
// 	defer cursor.Close(r.Context())

// 	var transactions []model.Transaction
// 	if err = cursor.All(r.Context(), &transactions); err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to decode transactions"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, transactions)
// }

// // GetMicroBitcoinStats retrieves statistics about the MicroBitcoin data
// func GetMicroBitcoinStats(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get the latest stats
// 	var stats model.MicroBitcoinStats
// 	opts := options.FindOne().SetSort(bson.M{"last_updated": -1})
// 	err = config.Mongoconn.Collection(statsCollection).FindOne(r.Context(), bson.M{}, opts).Decode(&stats)
	
// 	if err != nil {
// 		// If stats don't exist, generate them
// 		stats, err = generateAndSaveStats(r.Context())
// 		if err != nil {
// 			var respn model.Response
// 			respn.Status = "Error: Failed to generate stats"
// 			respn.Response = err.Error()
// 			at.WriteJSON(w, http.StatusInternalServerError, respn)
// 			return
// 		}
// 	}

// 	at.WriteJSON(w, http.StatusOK, stats)
// }

// // SyncMicroBitcoinData syncs data from the MicroBitcoin API
// func SyncMicroBitcoinData(w http.ResponseWriter, r *http.Request) {
// 	// Only admin can trigger a sync
// 	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Token Tidak Valid"
// 		respn.Info = at.GetSecretFromHeader(r)
// 		respn.Location = "Decode Token Error"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusForbidden, respn)
// 		return
// 	}

// 	// Check if user is admin (assuming admin phone number is stored in configuration)
// 	if payload.Id != config.PhoneNumber {
// 		var respn model.Response
// 		respn.Status = "Error: Tidak berhak akses"
// 		respn.Response = "Only admin can trigger data sync"
// 		at.WriteJSON(w, http.StatusForbidden, respn)
// 		return
// 	}

// 	// Check if sync is already in progress
// 	var syncStatus model.MicroBitcoinSyncStatus
// 	err = config.Mongoconn.Collection(syncStatusCollection).FindOne(r.Context(), bson.M{}).Decode(&syncStatus)
// 	if err == nil && syncStatus.SyncInProgress {
// 		var respn model.Response
// 		respn.Status = "Info: Sync already in progress"
// 		respn.Response = "Sync started at " + syncStatus.LastSyncedTime.Format(time.RFC3339)
// 		at.WriteJSON(w, http.StatusConflict, respn)
// 		return
// 	}

// 	// Start sync in background
// 	go startSync()

// 	var respn model.Response
// 	respn.Status = "Success"
// 	respn.Response = "MicroBitcoin data sync started"
// 	at.WriteJSON(w, http.StatusOK, respn)
// }

// // Helper functions

// // startSync starts syncing MicroBitcoin data
// func startSync() {
// 	ctx := context.Background()
// 	startTime := time.Now()

// 	// Update sync status to in progress
// 	syncStatus := model.MicroBitcoinSyncStatus{
// 		LastSyncedTime: time.Now(),
// 		SyncInProgress: true,
// 	}

// 	// Get the current sync status to update it
// 	var existingStatus model.MicroBitcoinSyncStatus
// 	err := config.Mongoconn.Collection(syncStatusCollection).FindOne(ctx, bson.M{}).Decode(&existingStatus)
// 	if err == nil {
// 		// Update existing status
// 		syncStatus.ID = existingStatus.ID
// 		syncStatus.LastSyncedBlock = existingStatus.LastSyncedBlock
// 		syncStatus.TotalBlocksSynced = existingStatus.TotalBlocksSynced
// 		syncStatus.TotalTxSynced = existingStatus.TotalTxSynced
// 	}

// 	// Save the sync status
// 	opts := options.Update().SetUpsert(true)
// 	_, err = config.Mongoconn.Collection(syncStatusCollection).UpdateOne(
// 		ctx, 
// 		bson.M{"_id": syncStatus.ID},
// 		bson.M{"$set": syncStatus},
// 		opts,
// 	)
// 	if err != nil {
// 		log.Printf("Failed to update sync status: %v", err)
// 		return
// 	}

// 	// Fetch and save blockchain info
// 	err = fetchAndSaveBlockchainInfo(ctx)
// 	if err != nil {
// 		log.Printf("Failed to fetch blockchain info: %v", err)
// 		// Update sync status to not in progress
// 		updateSyncStatus(ctx, false, 0, syncStatus.LastSyncedBlock, syncStatus.TotalBlocksSynced, syncStatus.TotalTxSynced, 0)
// 		return
// 	}

// 	// Get latest block height
// 	latestHeight, err := getLatestBlockHeight()
// 	if err != nil {
// 		log.Printf("Failed to get latest block height: %v", err)
// 		// Update sync status to not in progress
// 		updateSyncStatus(ctx, false, 0, syncStatus.LastSyncedBlock, syncStatus.TotalBlocksSynced, syncStatus.TotalTxSynced, 0)
// 		return
// 	}

// 	// Determine starting block (from last synced block + 1 or from the latest - 10)
// 	startBlock := syncStatus.LastSyncedBlock + 1
// 	if startBlock == 0 || startBlock > latestHeight {
// 		startBlock = latestHeight - 10
// 		if startBlock < 0 {
// 			startBlock = 0
// 		}
// 	}

// 	// Sync blocks and transactions
// 	blocksSynced := 0
// 	txSynced := 0
// 	for height := startBlock; height <= latestHeight; height++ {
// 		block, err := fetchBlock(height)
// 		if err != nil {
// 			log.Printf("Error fetching block at height %d: %v", height, err)
// 			continue
// 		}

// 		// Save block
// 		err = saveBlockToMongoDB(ctx, block)
// 		if err != nil {
// 			log.Printf("Error saving block at height %d: %v", height, err)
// 			continue
// 		}
// 		blocksSynced++

// 		// Fetch and save transactions (limit to 10 per block for example)
// 		maxTx := 10
// 		if len(block.Tx) < maxTx {
// 			maxTx = len(block.Tx)
// 		}

// 		for i := 0; i < maxTx; i++ {
// 			tx, err := fetchTransaction(block.Tx[i])
// 			if err != nil {
// 				log.Printf("Error fetching transaction %s: %v", block.Tx[i], err)
// 				continue
// 			}

// 			// Save transaction
// 			err = saveTransactionToMongoDB(ctx, tx)
// 			if err != nil {
// 				log.Printf("Error saving transaction %s: %v", tx.TxID, err)
// 				continue
// 			}
// 			txSynced++
// 		}

// 		// Update the sync status periodically
// 		if height%10 == 0 {
// 			updateSyncStatus(
// 				ctx, 
// 				true, 
// 				height, 
// 				height, 
// 				syncStatus.TotalBlocksSynced+blocksSynced, 
// 				syncStatus.TotalTxSynced+txSynced,
// 				0,
// 			)
// 		}
// 	}

// 	// Generate new stats
// 	_, err = generateAndSaveStats(ctx)
// 	if err != nil {
// 		log.Printf("Failed to generate stats: %v", err)
// 	}

// 	// Update sync status one final time
// 	duration := time.Since(startTime).Seconds()
// 	updateSyncStatus(
// 		ctx, 
// 		false, 
// 		latestHeight, 
// 		latestHeight, 
// 		syncStatus.TotalBlocksSynced+blocksSynced, 
// 		syncStatus.TotalTxSynced+txSynced,
// 		duration,
// 	)
// }

// // updateSyncStatus updates the sync status in the database
// func updateSyncStatus(ctx context.Context, inProgress bool, currentBlock, lastSyncedBlock, totalBlocksSynced, totalTxSynced int, duration float64) {
// 	// Find existing status first
// 	var syncStatus model.MicroBitcoinSyncStatus
// 	err := config.Mongoconn.Collection(syncStatusCollection).FindOne(ctx, bson.M{}).Decode(&syncStatus)
	
// 	if err != nil {
// 		// Create a new status if none exists
// 		syncStatus = model.MicroBitcoinSyncStatus{
// 			SyncInProgress:    inProgress,
// 			LastSyncedBlock:   lastSyncedBlock,
// 			LastSyncedTime:    time.Now(),
// 			TotalBlocksSynced: totalBlocksSynced,
// 			TotalTxSynced:     totalTxSynced,
// 			LastSyncDuration:  duration,
// 		}
// 		_, err = config.Mongoconn.Collection(syncStatusCollection).InsertOne(ctx, syncStatus)
// 	} else {
// 		// Update existing status
// 		update := bson.M{
// 			"$set": bson.M{
// 				"sync_in_progress":    inProgress,
// 				"last_synced_block":   lastSyncedBlock,
// 				"last_synced_time":    time.Now(),
// 				"total_blocks_synced": totalBlocksSynced,
// 				"total_tx_synced":     totalTxSynced,
// 			},
// 		}
		
// 		// Only update duration if provided
// 		if duration > 0 {
// 			update["$set"].(bson.M)["last_sync_duration"] = duration
// 		}
		
// 		_, err = config.Mongoconn.Collection(syncStatusCollection).UpdateOne(
// 			ctx,
// 			bson.M{"_id": syncStatus.ID},
// 			update,
// 		)
// 	}
	
// 	if err != nil {
// 		log.Printf("Failed to update sync status: %v", err)
// 	}
// }

// // generateAndSaveStats generates stats about MicroBitcoin data and saves them
// func generateAndSaveStats(ctx context.Context) (model.MicroBitcoinStats, error) {
// 	var stats model.MicroBitcoinStats
	
// 	// Count total blocks
// 	blockCount, err := config.Mongoconn.Collection(blocksCollection).CountDocuments(ctx, bson.M{})
// 	if err != nil {
// 		return stats, fmt.Errorf("failed to count blocks: %v", err)
// 	}
	
// 	// Count total transactions
// 	txCount, err := config.Mongoconn.Collection(transactionsCollection).CountDocuments(ctx, bson.M{})
// 	if err != nil {
// 		return stats, fmt.Errorf("failed to count transactions: %v", err)
// 	}
	
// 	// Calculate average block size
// 	cursor, err := config.Mongoconn.Collection(blocksCollection).Find(ctx, bson.M{})
// 	if err != nil {
// 		return stats, fmt.Errorf("failed to fetch blocks: %v", err)
// 	}
// 	defer cursor.Close(ctx)
	
// 	var blocks []model.Block
// 	if err = cursor.All(ctx, &blocks); err != nil {
// 		return stats, fmt.Errorf("failed to decode blocks: %v", err)
// 	}
	
// 	var totalSize int64
// 	var totalTx int
// 	for _, block := range blocks {
// 		totalSize += int64(block.Size)
// 		totalTx += len(block.Tx)
// 	}
	
// 	var avgBlockSize float64
// 	var avgTxPerBlock float64
	
// 	if blockCount > 0 {
// 		avgBlockSize = float64(totalSize) / float64(blockCount)
// 		avgTxPerBlock = float64(totalTx) / float64(blockCount)
// 	}
	
// 	// Populate stats
// 	stats = model.MicroBitcoinStats{
// 		TotalBlocks:               int(blockCount),
// 		TotalTransactions:         int(txCount),
// 		AverageBlockSize:          avgBlockSize,
// 		AverageTransactionsPerBlock: avgTxPerBlock,
// 		LastUpdated:               time.Now(),
// 	}
	
// 	// Save the stats
// 	opts := options.Update().SetUpsert(true)
// 	_, err = config.Mongoconn.Collection(statsCollection).UpdateOne(
// 		ctx,
// 		bson.M{},
// 		bson.M{"$set": stats},
// 		opts,
// 	)
// 	if err != nil {
// 		return stats, fmt.Errorf("failed to save stats: %v", err)
// 	}
	
// 	return stats, nil
// }

// // fetchAndSaveBlockchainInfo fetches blockchain info and saves it to MongoDB
// func fetchAndSaveBlockchainInfo(ctx context.Context) error {
// 	url := fmt.Sprintf("%s/getblockchaininfo", apiBaseURL)
	
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return fmt.Errorf("error fetching blockchain info: %v", err)
// 	}
// 	defer resp.Body.Close()
	
// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return fmt.Errorf("error reading response body: %v", err)
// 	}
	
// 	var blockchainInfo model.BlockchainInfo
// 	err = json.Unmarshal(body, &blockchainInfo)
// 	if err != nil {
// 		return fmt.Errorf("error unmarshaling blockchain info: %v", err)
// 	}
	
// 	blockchainInfo.FetchedAt = time.Now()
	
// 	_, err = config.Mongoconn.Collection(blockchainInfoCollection).InsertOne(ctx, blockchainInfo)
// 	if err != nil {
// 		return fmt.Errorf("error saving blockchain info: %v", err)
// 	}
	
// 	return nil
// }

// // getLatestBlockHeight gets the latest block height from the API
// func getLatestBlockHeight() (int, error) {
// 	url := fmt.Sprintf("%s/getblockcount", apiBaseURL)
	
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return 0, fmt.Errorf("error fetching block count: %v", err)
// 	}
// 	defer resp.Body.Close()
	
// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return 0, fmt.Errorf("error reading response body: %v", err)
// 	}
	
// 	var blockHeight int
// 	err = json.Unmarshal(body, &blockHeight)
// 	if err != nil {
// 		return 0, fmt.Errorf("error unmarshaling block height: %v", err)
// 	}
	
// 	return blockHeight, nil
// }

// // fetchBlock fetches a block by its height from the API
// func fetchBlock(height int) (*model.Block, error) {
// 	// First, get the block hash from height
// 	hashURL := fmt.Sprintf("%s/getblockhash?height=%d", apiBaseURL, height)
	
// 	resp, err := http.Get(hashURL)
// 	if err != nil {
// 		return nil, fmt.Errorf("error fetching block hash: %v", err)
// 	}
// 	defer resp.Body.Close()
	
// 	hashBody, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading hash response body: %v", err)
// 	}
	
// 	var blockHash string
// 	err = json.Unmarshal(hashBody, &blockHash)
// 	if err != nil {
// 		return nil, fmt.Errorf("error unmarshaling block hash: %v", err)
// 	}
	
// 	// Then, get the block details by hash
// 	blockURL := fmt.Sprintf("%s/getblock?hash=%s", apiBaseURL, blockHash)
	
// 	blockResp, err := http.Get(blockURL)
// 	if err != nil {
// 		return nil, fmt.Errorf("error fetching block details: %v", err)
// 	}
// 	defer blockResp.Body.Close()
	
// 	blockBody, err := ioutil.ReadAll(blockResp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading block response body: %v", err)
// 	}
	
// 	var block model.Block
// 	err = json.Unmarshal(blockBody, &block)
// 	if err != nil {
// 		return nil, fmt.Errorf("error unmarshaling block details: %v", err)
// 	}
	
// 	block.FetchedAt = time.Now()
	
// 	return &block, nil
// }

// // saveBlockToMongoDB saves a block to MongoDB
// func saveBlockToMongoDB(ctx context.Context, block *model.Block) error {
// 	collection := config.Mongoconn.Collection(blocksCollection)
	
// 	// Check if block already exists
// 	filter := bson.M{"hash": block.Hash}
// 	count, err := collection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		return fmt.Errorf("error checking if block exists: %v", err)
// 	}
	
// 	if count > 0 {
// 		// Update if already exists
// 		update := bson.M{"$set": block}
// 		_, err = collection.UpdateOne(ctx, filter, update)
// 		if err != nil {
// 			return fmt.Errorf("error updating block: %v", err)
// 		}
// 		return nil
// 	}
	
// 	// Insert if it doesn't exist
// 	_, err = collection.InsertOne(ctx, block)
// 	if err != nil {
// 		return fmt.Errorf("error inserting block: %v", err)
// 	}
	
// 	return nil
// }

// // fetchTransaction fetches a transaction by its ID from the API
// func fetchTransaction(txID string) (*model.Transaction, error) {
// 	url := fmt.Sprintf("%s/getrawtransaction?txid=%s&verbose=1", apiBaseURL, txID)
	
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return nil, fmt.Errorf("error fetching transaction: %v", err)
// 	}
// 	defer resp.Body.Close()
	
// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading response body: %v", err)
// 	}
	
// 	var tx model.Transaction
// 	err = json.Unmarshal(body, &tx)
// 	if err != nil {
// 		return nil, fmt.Errorf("error unmarshaling transaction: %v", err)
// 	}
	
// 	tx.FetchedAt = time.Now()
	
// 	return &tx, nil
// }

// // saveTransactionToMongoDB saves a transaction to MongoDB
// func saveTransactionToMongoDB(ctx context.Context, tx *model.Transaction) error {
// 	collection := config.Mongoconn.Collection(transactionsCollection)
	
// 	// Check if transaction already exists
// 	filter := bson.M{"txid": tx.TxID}
// 	count, err := collection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		return fmt.Errorf("error checking if transaction exists: %v", err)
// 	}
	
// 	if count > 0 {
// 		// Update if already exists
// 		update := bson.M{"$set": tx}
// 		_, err = collection.UpdateOne(ctx, filter, update)
// 		if err != nil {
// 			return fmt.Errorf("error updating transaction: %v", err)
// 		}
// 		return nil
// 	}
	
// 	// Insert if it doesn't exist
// 	_, err = collection.InsertOne(ctx, tx)
// 	if err != nil {
// 		return fmt.Errorf("error inserting transaction: %v", err)
// 	}
	
// 	return nil
// }

// // GetSyncStatus retrieves the current sync status
// func GetSyncStatus(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Get sync status
// 	var syncStatus model.MicroBitcoinSyncStatus
// 	err = config.Mongoconn.Collection(syncStatusCollection).FindOne(r.Context(), bson.M{}).Decode(&syncStatus)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to retrieve sync status"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, syncStatus)
// }

// // SearchTransactions searches for transactions by various criteria
// func SearchTransactions(w http.ResponseWriter, r *http.Request) {
// 	// Authorize the request
// 	_, err := watoken.ParseToken(w, r)
// 	if err != nil {
// 		return
// 	}

// 	// Parse query parameters
// 	query := bson.M{}

// 	// Check for address parameter
// 	address := r.URL.Query().Get("address")
// 	if address != "" {
// 		query["vout.scriptPubKey.addresses"] = address
// 	}

// 	// Check for time range parameters
// 	timeStartStr := r.URL.Query().Get("timeStart")
// 	timeEndStr := r.URL.Query().Get("timeEnd")

// 	if timeStartStr != "" || timeEndStr != "" {
// 		timeQuery := bson.M{}

// 		if timeStartStr != "" {
// 			timeStart, err := strconv.ParseInt(timeStartStr, 10, 64)
// 			if err == nil {
// 				timeQuery["$gte"] = timeStart
// 			}
// 		}

// 		if timeEndStr != "" {
// 			timeEnd, err := strconv.ParseInt(timeEndStr, 10, 64)
// 			if err == nil {
// 				timeQuery["$lte"] = timeEnd
// 			}
// 		}

// 		if len(timeQuery) > 0 {
// 			query["time"] = timeQuery
// 		}
// 	}

// 	// Limit results
// 	limitStr := r.URL.Query().Get("limit")
// 	limit := 100 // Default limit
// 	if limitStr != "" {
// 		parsedLimit, err := strconv.Atoi(limitStr)
// 		if err == nil && parsedLimit > 0 {
// 			limit = parsedLimit
// 		}
// 	}

// 	// Get transactions
// 	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.M{"time": -1})
// 	cursor, err := config.Mongoconn.Collection(transactionsCollection).Find(r.Context(), query, opts)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to search transactions"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}
// 	defer cursor.Close(r.Context())

// 	var transactions []model.Transaction
// 	if err = cursor.All(r.Context(), &transactions); err != nil {
// 		var respn model.Response
// 		respn.Status = "Error: Failed to decode transactions"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(w, http.StatusOK, transactions)
// }