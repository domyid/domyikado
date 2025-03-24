package controller

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

// GetOnePreTestQuestion retrieves a single Pre Test question from the MongoDB collection by ID
func GetOnePreTestQuestion(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID dari URL
	pathSegments := strings.Split(r.URL.Path, "/")
	id := pathSegments[len(pathSegments)-1]

	// Filter untuk mendapatkan soal Pre Test berdasarkan ID
	filter := bson.M{
		"id": id,
		"$or": []bson.M{
			{"deleted_at": nil},
			{"deleted_at": bson.M{"$exists": false}},
		},
	}

	var PreTestQuestion model.PreTestQuestion
	err := config.Mongoconn.Collection("pretestquestion").FindOne(context.Background(), filter).Decode(&PreTestQuestion)
	if err != nil {
		log.Printf("Error querying Pre Test question with ID %s: %v", id, err)
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Soal Pre Test tidak ditemukan.",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, PreTestQuestion)
}
