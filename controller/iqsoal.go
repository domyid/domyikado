package controller

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"go.mongodb.org/mongo-driver/bson"
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

// GetOneIqQuestion retrieves a single IQ question from the MongoDB collection by ID
func GetOneIqQuestion(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID dari URL
	pathSegments := strings.Split(r.URL.Path, "/")
	id := pathSegments[len(pathSegments)-1]
	// Filter untuk mendapatkan soal IQ berdasarkan ID
	filter := bson.M{
		"id":         id,
		"deleted_at": bson.M{"$exists": false},
	}

	var iqQuestion SoalIQ
	err := config.Mongoconn.Collection("iqquestion").FindOne(context.Background(), filter).Decode(&iqQuestion)
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
