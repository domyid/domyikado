package report

import (
	"context"
	"time"

	"github.com/gocroot/config"
	"go.mongodb.org/mongo-driver/bson"
)

type IqScoreReport struct {
	Name      string    `json:"name,omitempty" bson:"name,omitempty"`
	Score     string    `json:"score" bson:"score"`
	IQ        float64   `json:"iq" bson:"iq"`
	WaGroupID string    `bson:"wagroupid" json:"wagroupid"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

type IqResponse struct {
	Success bool            `json:"success"`
	Data    []IqScoreReport `json:"data"`
	Message string          `json:"message,omitempty"`
}

// Fungsi untuk mengambil data dari MongoDB
func FetchIqScores() ([]IqScoreReport, error) {
	collection := config.Mongoconn.Collection("iqscore")

	// Ambil semua data dari koleksi iqscore
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	// Menyimpan hasil dalam slice
	var scores []IqScoreReport
	if err = cursor.All(context.TODO(), &scores); err != nil {
		return nil, err
	}

	return scores, nil
}
