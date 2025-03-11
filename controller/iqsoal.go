package controller

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Struct SoalIQ sesuai dengan koleksi questioniq
type SoalIQ struct {
	ID        string `json:"id"`
	Question  string `json:"question"`
	Image     string `json:"image"`
	AnswerKey string `json:"answer_key"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	DeletedAt string `json:"deleted_at"`
}

// GetRandomIqQuestion retrieves a single random IQ question from the MongoDB collection
func GetRandomIqQuestion(w http.ResponseWriter, r *http.Request) {
	// Filter untuk mendapatkan soal IQ yang belum dihapus
	filter := bson.M{
		"deleted_at": bson.M{"$exists": false}, // Pastikan soal belum dihapus
	}

	// Query MongoDB untuk mendapatkan total jumlah soal yang memenuhi filter
	count, err := config.Mongoconn.Collection("questioniq").CountDocuments(context.Background(), filter)
	if err != nil {
		log.Printf("Error counting IQ questions: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"message": "Error counting IQ questions",
			"error":   err.Error(),
		})
		return
	}

	// Jika tidak ada soal, kembalikan respons 404
	if count == 0 {
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Tidak ada soal IQ yang tersedia.",
		})
		return
	}

	// Generate index acak
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(int(count))

	// Query MongoDB untuk mendapatkan satu soal secara acak
	findOptions := options.FindOne().SetSkip(int64(randomIndex))
	var iqQuestion SoalIQ
	err = config.Mongoconn.Collection("questioniq").FindOne(context.Background(), filter, findOptions).Decode(&iqQuestion)

	if err != nil {
		log.Printf("Error querying random IQ question: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"message": "Error querying random IQ question",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, iqQuestion)
}
