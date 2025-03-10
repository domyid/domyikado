package controller

import (
	"context"
	"log"
	"net/http"

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

// GetIqQuestions retrieves all IQ questions from the MongoDB collection
func GetIqQuestions(w http.ResponseWriter, r *http.Request) {
	// Filter untuk mendapatkan soal IQ yang belum dihapus
	filter := bson.M{
		"deleted_at": bson.M{"$exists": false}, // Pastikan soal belum dihapus
	}

	// Query MongoDB
	cursor, err := config.Mongoconn.Collection("questioniq").Find(context.Background(), filter, options.Find())
	if err != nil {
		log.Printf("Error querying IQ questions: %v", err)
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"message": "Error retrieving IQ questions",
			"error":   err.Error(),
		})
		return
	}
	defer cursor.Close(context.Background())

	var iqQuestions []SoalIQ
	for cursor.Next(context.Background()) {
		var question SoalIQ
		if err := cursor.Decode(&question); err != nil {
			log.Printf("Error decoding IQ question: %v", err)
			at.WriteJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Error decoding IQ question",
				"error":   err.Error(),
			})
			return
		}
		iqQuestions = append(iqQuestions, question)
	}

	if len(iqQuestions) == 0 {
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Tidak ada soal IQ yang tersedia.",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, iqQuestions)
}
