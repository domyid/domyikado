package controller

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
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
	var iqQuestions []SoalIQ
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

	//Iterasi cursor
	for cursor.Next(context.Background()) {
		var iqQuestion SoalIQ
		err := cursor.Decode(&iqQuestion)
		if err != nil {
			log.Printf("Error decoding IQ questions: %v", err)
			at.WriteJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Error decoding IQ questions",
				"error":   err.Error(),
			})
			return
		}
		iqQuestions = append(iqQuestions, iqQuestion)
	}
	if len(iqQuestions) == 0 {
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Tidak ada soal IQ yang tersedia.",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, iqQuestions)
}

// GetOneIqQuestion retrieves a single IQ question from the MongoDB collection by ID
func GetOneIqQuestion(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID dari URL
	pathSegments := strings.Split(r.URL.Path, "/")
	id := pathSegments[len(pathSegments)-1]

	// Filter untuk mendapatkan soal IQ berdasarkan ID
	filter := bson.M{
		"id":         id,
		"deleted_at": bson.M{"$exists": false}, // Pastikan soal belum dihapus
	}

	// GetOneDoc dari helper atdb
	iqQuestion, err := atdb.GetOneDoc[SoalIQ](config.Mongoconn, "questioniq", filter)
	if err != nil {
		log.Printf("Error querying IQ question with ID %s: %v", id, err)
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Soal IQ tidak ditemukan.",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, iqQuestion)
}
